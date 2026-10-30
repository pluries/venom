package smtp

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/mail"
	"net/smtp"
	"strings"
	"sync"
	"time"

	"github.com/mitchellh/mapstructure"

	"github.com/ovh/venom"
	"github.com/ovh/venom/executors/internal/tlsconfig"
)

// Name for test smtp
const Name = "smtp"

// New returns a new Test Exec
func New() venom.Executor {
	return &Executor{}
}

// Executor represents a Test Exec
type Executor struct {
	WithTLS bool `json:"withtls,omitempty" yaml:"withtls,omitempty"`
	// VerifyTLS checks the server certificate against the system roots and Host when WithTLS is set. Unset keeps the
	// historical behaviour: the certificate is not verified.
	VerifyTLS bool   `json:"verifytls,omitempty" yaml:"verifytls,omitempty"`
	Host      string `json:"host,omitempty" yaml:"host,omitempty"`
	Port      string `json:"port,omitempty" yaml:"port,omitempty"`
	User      string `json:"user,omitempty" yaml:"user,omitempty"`
	Password  string `json:"password,omitempty" yaml:"password,omitempty"`
	To        string `json:"to,omitempty" yaml:"to,omitempty"`
	From      string `json:"from,omitempty" yaml:"from,omitempty"`
	Subject   string `json:"subject,omitempty" yaml:"subject,omitempty"`
	Body      string `json:"body,omitempty" yaml:"body,omitempty"`
	// TLSRootCA is trusted on top of the system roots when VerifyTLS is set; TLSClientCert and TLSClientKey are the
	// client certificate, sent whether VerifyTLS is set or not. Each is PEM text or a file path relative to the suite
	// (ADR-0009). They are used only with WithTLS.
	TLSRootCA     string `json:"tls_root_ca" yaml:"tls_root_ca" mapstructure:"tls_root_ca"`
	TLSClientCert string `json:"tls_client_cert" yaml:"tls_client_cert" mapstructure:"tls_client_cert"`
	TLSClientKey  string `json:"tls_client_key" yaml:"tls_client_key" mapstructure:"tls_client_key"`
}

// Result represents a step result
type Result struct {
	Err         string  `json:"err,omitempty" yaml:"error"`
	TimeSeconds float64 `json:"timeseconds,omitempty" yaml:"timeSeconds,omitempty"`
}

// ZeroValueResult return an empty implementation of this executor result
func (Executor) ZeroValueResult() interface{} {
	return Result{}
}

// GetDefaultAssertions return default assertions for type exec
func (Executor) GetDefaultAssertions() *venom.StepAssertions {
	return &venom.StepAssertions{Assertions: []venom.Assertion{"result.err ShouldBeEmpty"}}
}

// Run execute TestStep of type exec
func (Executor) Run(ctx context.Context, step venom.TestStep) (interface{}, error) {
	var e Executor
	if err := mapstructure.Decode(step, &e); err != nil {
		return nil, err
	}

	start := time.Now()

	result := Result{}
	err := e.sendEmail(ctx)
	if err != nil {
		result.Err = err.Error()
		return result, err
	}

	elapsed := time.Since(start)
	result.TimeSeconds = elapsed.Seconds()

	return result, nil
}

func (e *Executor) sendEmail(ctx context.Context) error {
	if e.To == "" {
		return fmt.Errorf("invalid To")
	}
	if e.From == "" {
		return fmt.Errorf("invalid From")
	}

	mailFrom := mail.Address{
		Name:    "",
		Address: e.From,
	}
	mailTo := mail.Address{
		Name:    "",
		Address: e.To,
	}

	message := e.message()

	c, closeOnReturn, err := e.connect(ctx)
	if err != nil {
		return err
	}
	if closeOnReturn {
		defer c.Close()
	}

	if err := e.authenticate(c); err != nil {
		return err
	}

	return deliver(ctx, c, mailFrom.Address, mailTo.Address, message)
}

// message builds the headers and the body of the mail.
func (e *Executor) message() string {
	// Setup headers
	headers := make(map[string]string)
	headers["From"] = e.From
	headers["To"] = e.To
	headers["Subject"] = e.Subject
	headers["Content-Type"] = "text/plain; charset=utf-8"

	// Setup message
	message := ""
	for k, v := range headers {
		message += fmt.Sprintf("%s: %s\r\n", k, v)
	}
	message += "\r\n" + e.Body
	return message
}

// warnf is venom.Warn; a test replaces it. insecureWarning limits the TLS warning to one per run (#2298).
var (
	warnf           = venom.Warn
	insecureWarning sync.Once
)

// resetInsecureWarning re-arms the warning, for tests only.
func resetInsecureWarning() { insecureWarning = sync.Once{} }

// warnInsecureTLS says once per run that a TLS connection does not verify the server certificate. Behaviour unchanged.
func (e *Executor) warnInsecureTLS(ctx context.Context) {
	if e.WithTLS && !e.VerifyTLS {
		insecureWarning.Do(func() {
			warnf(ctx, "smtp: the server's TLS certificate is not verified (set verifytls: true to verify it)")
		})
	}
}

// checkTLSSettings refuses half a client pair, whether WithTLS is set or not, and warns when TLS settings are given
// without WithTLS: they are not used and the connection stays plaintext (ADR-0009 Precedence 2 and 3).
func (e *Executor) checkTLSSettings(ctx context.Context) error {
	if err := tlsconfig.CheckClientPair(e.TLSClientCert, e.TLSClientKey); err != nil {
		return err
	}
	if !e.WithTLS && (e.TLSRootCA != "" || e.TLSClientCert != "") {
		warnf(ctx, "smtp: tls_root_ca, tls_client_cert and tls_client_key are not used without withtls")
	}
	return nil
}

// connect opens the SMTP session, over TLS when asked. The boolean says whether the caller closes the
// client on return: only a plain connection is closed that way, a TLS one is left to Quit.
func (e *Executor) connect(ctx context.Context) (*smtp.Client, bool, error) {
	if err := e.checkTLSSettings(ctx); err != nil {
		return nil, false, err
	}

	e.warnInsecureTLS(ctx)
	servername := fmt.Sprintf("%s:%s", e.Host, e.Port)
	venom.Info(ctx, "Connecting to %s", servername)

	if e.WithTLS {
		return e.dialTLS(ctx, servername)
	}

	c, err := smtp.Dial(servername)
	if err != nil {
		return nil, false, fmt.Errorf("smtp dial error: %w", err)
	}
	return c, true, nil
}

// dialTLS opens a TLS session to servername with the configuration of the shared tlsconfig package (ADR-0009):
// VerifyTLS decides whether the server certificate is checked, against the system roots plus TLSRootCA and for Host;
// the client certificate is sent either way.
func (e *Executor) dialTLS(ctx context.Context, servername string) (*smtp.Client, bool, error) {
	cfg, err := tlsconfig.New(ctx, tlsconfig.Settings{
		Workdir:            venom.StringVarFromCtx(ctx, "venom.testsuite.workdir"),
		RootCA:             e.TLSRootCA,
		ClientCert:         e.TLSClientCert,
		ClientKey:          e.TLSClientKey,
		InsecureSkipVerify: !e.VerifyTLS,
	})
	if err != nil {
		return nil, false, err
	}
	cfg.ServerName = e.Host

	conn, err := tls.Dial("tcp", servername, cfg)
	if err != nil {
		return nil, false, fmt.Errorf("tls dial error: %w", err)
	}

	c, err := smtp.NewClient(conn, e.Host)
	if err != nil {
		return nil, false, err
	}
	return c, false, nil
}

// authenticate logs in with PLAIN auth when both a user and a password are set.
func (e *Executor) authenticate(c *smtp.Client) error {
	if e.User != "" && e.Password != "" {
		auth := smtp.PlainAuth("", e.User, e.Password, e.Host)
		if err := c.Auth(auth); err != nil {
			return fmt.Errorf("failed to authenticate: %w", err)
		}
	}
	return nil
}

// deliver sends the message from one sender to each comma-separated recipient, then ends the session.
func deliver(ctx context.Context, c *smtp.Client, from, to, message string) error {
	if err := c.Mail(from); err != nil {
		return err
	}

	for _, toaddr := range strings.Split(to, ",") {
		if err := c.Rcpt(toaddr); err != nil {
			return fmt.Errorf("%s: %v", toaddr, err)
		}
	}

	w, err := c.Data()
	if err != nil {
		return err
	}

	if _, err := w.Write([]byte(message)); err != nil {
		return err
	}

	if err := w.Close(); err != nil {
		return err
	}

	venom.Info(ctx, "mail sent to %s", to)

	return c.Quit()
}
