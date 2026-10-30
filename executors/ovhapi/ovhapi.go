package ovhapi

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mitchellh/mapstructure"
	"github.com/ovh/go-ovh/ovh"

	"github.com/ovh/venom"
	"github.com/ovh/venom/executors/internal/tlsconfig"
)

// Name of executor
const Name = "ovhapi"

// New returns a new Executor
func New() venom.Executor {
	return &Executor{}
}

// Headers represents header HTTP for Request
type Headers map[string]string

// Executor struct. Json and yaml descriptor are used for json output
type Executor struct {
	Endpoint          string   `json:"endpoint" yaml:"endpoint"`
	ApplicationKey    string   `json:"applicationKey" yaml:"applicationKey"`
	ApplicationSecret string   `json:"applicationSecret" yaml:"applicationSecret"`
	ConsumerKey       string   `json:"consumerKey" yaml:"consumerKey"`
	ClientID          string   `json:"clientID" yaml:"clientID"`
	ClientSecret      string   `json:"clientSecret" yaml:"clientSecret"`
	NoAuth            *bool    `json:"noAuth" yaml:"noAuth"`
	Headers           Headers  `json:"headers" yaml:"headers"`
	Resolve           []string `json:"resolve" yaml:"resolve"`
	Proxy             string   `json:"proxy" yaml:"proxy"`
	TLSRootCA         string   `json:"tlsRootCA" yaml:"tlsRootCA"`
	// TLSRootCASnake is the same setting under its ADR-0009 name: setting both is an error.
	TLSRootCASnake string `json:"tls_root_ca" yaml:"tls_root_ca" mapstructure:"tls_root_ca"`

	Method   string `json:"method" yaml:"method"`
	Path     string `json:"path" yaml:"path"`
	Body     string `json:"body" yaml:"body"`
	BodyFile string `json:"bodyFile" yaml:"bodyFile"`
}

// Result represents a step result. Json and yaml descriptor are used for json output
type Result struct {
	TimeSeconds float64     `json:"timeseconds,omitempty" yaml:"timeseconds,omitempty"`
	StatusCode  int         `json:"statuscode,omitempty" yaml:"statuscode,omitempty"`
	Body        string      `json:"body,omitempty" yaml:"body,omitempty"`
	BodyJSON    interface{} `json:"bodyjson,omitempty" yaml:"bodyjson,omitempty"`
	Err         string      `json:"err,omitempty" yaml:"err,omitempty"`
	Headers     Headers     `json:"headers" yaml:"headers"`
}

// ZeroValueResult return an empty implementation of this executor result
func (Executor) ZeroValueResult() interface{} {
	return Result{}
}

// GetDefaultAssertions return default assertions for this executor
func (Executor) GetDefaultAssertions() *venom.StepAssertions {
	return &venom.StepAssertions{Assertions: []venom.Assertion{"result.statuscode ShouldEqual 200"}}
}

// cloneDefaultTransport clones http.DefaultTransport as a *http.Transport we can customize
// (TLS root CA, DNS resolve overrides, proxy). http.DefaultTransport is *http.Transport in every
// stdlib version this module builds against, but the assertion is done in the safe two-value form
// so a future stdlib change degrades to a fresh transport instead of panicking.
func cloneDefaultTransport() *http.Transport {
	tr, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		return &http.Transport{}
	}
	return tr.Clone()
}

// warnf is venom.Warning; a test replaces it.
var warnf = venom.Warning

// Run execute TestStep
func (Executor) Run(ctx context.Context, step venom.TestStep) (interface{}, error) {
	// transform step to Executor Instance
	var e Executor
	if err := mapstructure.Decode(step, &e); err != nil {
		return nil, err
	}

	// Get context
	e.setDefaultsFromContext(ctx)
	noAuth := *(e.NoAuth)

	headers := venom.StringMapStringVarFromCtx(ctx, "ovh.headers")
	workdir := venom.StringVarFromCtx(ctx, "venom.testsuite.workdir")

	// set default values
	if e.Method == "" {
		e.Method = "GET"
	}

	// init result
	r := Result{}

	start := time.Now()

	// prepare ovh api client
	client, err := e.newClient(ctx)
	if err != nil {
		return nil, err
	}

	tr, err := e.customTransport(ctx, workdir)
	if err != nil {
		return nil, err
	}
	if tr != nil {
		client.Client.Transport = tr
	}

	// get request body from file or from field
	requestBody, err := e.getRequestBody(workdir)
	if err != nil {
		return nil, err
	}

	req, err := e.newRequest(client, requestBody, !noAuth, headers)
	if err != nil {
		return nil, err
	}

	// do api call

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	r.Headers = responseHeaders(resp)

	res, err := r.decodeResponse(client, resp)
	if err != nil {
		return nil, err
	}

	elapsed := time.Since(start)
	r.TimeSeconds = elapsed.Seconds()

	if err := r.setBody(res); err != nil {
		return nil, err
	}

	return r, nil
}

// setDefaultsFromContext fills every connection attribute the step left empty from the matching
// ovh.* variable of the context.
func (e *Executor) setDefaultsFromContext(ctx context.Context) {
	if e.Endpoint == "" {
		e.Endpoint = venom.StringVarFromCtx(ctx, "ovh.endpoint")
	}
	if e.ApplicationKey == "" {
		e.ApplicationKey = venom.StringVarFromCtx(ctx, "ovh.applicationKey")
	}
	if e.ApplicationSecret == "" {
		e.ApplicationSecret = venom.StringVarFromCtx(ctx, "ovh.applicationSecret")
	}
	if e.ConsumerKey == "" {
		e.ConsumerKey = venom.StringVarFromCtx(ctx, "ovh.consumerKey")
	}
	if e.ClientID == "" {
		e.ClientID = venom.StringVarFromCtx(ctx, "ovh.clientID")
	}
	if e.ClientSecret == "" {
		e.ClientSecret = venom.StringVarFromCtx(ctx, "ovh.clientSecret")
	}
	if e.NoAuth == nil {
		noauth := venom.BoolVarFromCtx(ctx, "ovh.noAuth")
		e.NoAuth = &noauth
	}
	if len(e.Resolve) == 0 {
		e.Resolve = venom.StringSliceVarFromCtx(ctx, "ovh.resolve")
	}
	if e.Proxy == "" {
		e.Proxy = venom.StringVarFromCtx(ctx, "ovh.proxy")
	}
	if e.TLSRootCA == "" && e.TLSRootCASnake == "" {
		e.TLSRootCA = venom.StringVarFromCtx(ctx, "ovh.tlsRootCA")
	}
}

// newClient builds an OAuth2 client when a client id and secret are both set, else an
// application-key client (a consumer key without an application key is not an error here).
func (e Executor) newClient(ctx context.Context) (*ovh.Client, error) {
	if e.ClientID != "" && e.ClientSecret != "" {
		venom.Debug(ctx, "creating OAuth2 client")
		return ovh.NewOAuth2Client(
			e.Endpoint,
			e.ClientID,
			e.ClientSecret,
		)
	}
	return ovh.NewClient(
		e.Endpoint,
		e.ApplicationKey,
		e.ApplicationSecret,
		e.ConsumerKey,
	)
}

// customTransport returns a clone of the default transport carrying the TLS root CA, the resolve
// overrides and the proxy the step sets, or nil when it sets none of them.
func (e Executor) customTransport(ctx context.Context, workdir string) (*http.Transport, error) {
	var tr *http.Transport

	rootCAs, err := e.rootCAs(ctx, workdir)
	if err != nil {
		return nil, err
	}
	if rootCAs != nil {
		tr = cloneDefaultTransport()
		if tr.TLSClientConfig == nil {
			tr.TLSClientConfig = &tls.Config{}
		}
		tr.TLSClientConfig.RootCAs = rootCAs
	}

	if len(e.Resolve) > 0 {
		if tr == nil {
			tr = cloneDefaultTransport()
		}
		tr.DialContext = resolveDialContext(e.Resolve)
	}

	if e.Proxy != "" {
		proxyURL, err := url.Parse(e.Proxy)
		if err != nil {
			return nil, err
		}

		if tr == nil {
			tr = cloneDefaultTransport()
		}

		tr.Proxy = http.ProxyURL(proxyURL)
	}

	return tr, nil
}

// rootCAs returns the system pool plus the step's root CA (tlsRootCA or tls_root_ca, else the
// ovh.tlsRootCA variable), given as PEM or as a file path relative to workdir; nil when none is set.
func (e Executor) rootCAs(ctx context.Context, workdir string) (*x509.CertPool, error) {
	if e.TLSRootCA != "" && e.TLSRootCASnake != "" {
		return nil, errors.New("tlsRootCA and tls_root_ca are both set: keep one")
	}
	rootCA := e.TLSRootCA
	if rootCA == "" {
		rootCA = e.TLSRootCASnake
	}
	if rootCA == "" {
		return nil, nil
	}
	e.warnPlainHTTPEndpoint(ctx)

	caPEM, err := tlsconfig.ReadPEM(workdir, rootCA, "TLSRootCA")
	if err != nil {
		return nil, err
	}
	pool, err := tlsconfig.RootCAs(ctx, caPEM)
	if err != nil {
		return nil, fmt.Errorf("TLSRootCA: %w", err)
	}
	return pool, nil
}

// plainHTTPScheme is the scheme of a request that opens no TLS connection of its own.
const plainHTTPScheme = "http://"

// warnPlainHTTPEndpoint warns, once per step, that a root CA only serves TLS connections (a followed redirect to
// https:// or an https:// proxy included) when the endpoint is http:// (any case). An alias such as ovh-eu resolves to https:// inside go-ovh
// and never warns (ADR-0009, #2385).
func (e Executor) warnPlainHTTPEndpoint(ctx context.Context) {
	if len(e.Endpoint) >= len(plainHTTPScheme) && strings.EqualFold(e.Endpoint[:len(plainHTTPScheme)], plainHTTPScheme) {
		warnf(ctx, "ovhapi: tlsRootCA applies only to TLS connections: the endpoint is http:// (it still serves a redirect to https:// or an https:// proxy)")
	}
}

// resolveDialContext dials addr through the resolve overrides. The entries are only checked
// here, at dial time: one malformed entry fails every dial.
func resolveDialContext(resolve []string) func(ctx context.Context, network, addr string) (net.Conn, error) {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		// resolve can contains foo.com:443:127.0.0.1
		for _, r := range resolve {
			tuple := strings.Split(r, ":")
			if len(tuple) != 3 {
				return nil, fmt.Errorf("invalid value for resolve attribute: %v", resolve)
			}
			if addr == tuple[0]+":"+tuple[1] {
				addr = tuple[2] + ":" + tuple[1]
			}
		}

		dialer := &net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
			// DualStack removed: it has been a no-op since Go 1.12 — Fast Fallback (Happy
			// Eyeballs) is on by default (SA1019, #2028).
		}
		return dialer.DialContext(ctx, network, addr)
	}
}

// newRequest builds the request (signed when needAuth) with the context headers, then the step
// headers over them; a Host header also sets the request host.
func (e Executor) newRequest(client *ovh.Client, requestBody interface{}, needAuth bool, headers map[string]string) (*http.Request, error) {
	req, err := client.NewRequest(e.Method, e.Path, requestBody, needAuth)
	if err != nil {
		return nil, err
	}

	for key, value := range headers {
		req.Header.Set(key, value)
	}

	if e.Headers != nil {
		for key := range e.Headers {
			req.Header.Set(key, e.Headers[key])
		}
	}

	if h := req.Header.Get("Host"); h != "" {
		req.Host = h
	}

	return req, nil
}

// responseHeaders joins each response header's values with commas.
func responseHeaders(resp *http.Response) Headers {
	headers := Headers{}
	for k, v := range resp.Header {
		headers[k] = strings.Join(v, ",")
	}
	return headers
}

// decodeResponse decodes the response body and sets the status: every 2xx is reported as 200, an
// OVH API error as its code and message; any other decoding error fails the step.
func (r *Result) decodeResponse(client *ovh.Client, resp *http.Response) (*interface{}, error) {
	res := new(interface{})
	if err := client.UnmarshalResponse(resp, res); err != nil {
		apiError, ok := err.(*ovh.APIError)
		if !ok {
			return nil, err
		}
		r.StatusCode = apiError.Code
		r.Err = apiError.Message
	} else {
		r.StatusCode = 200
	}
	return res, nil
}

// setBody adds the decoded response to the result body. res is a *interface{} from
// new(interface{}) and is never nil itself; what matters is whether go-ovh actually decoded a body
// into *res. On an API error UnmarshalResponse returns before touching it, so *res stays nil and
// this is skipped — before #2028 it ran unconditionally and left Body as the literal string "null".
func (r *Result) setBody(res *interface{}) error {
	if *res == nil {
		return nil
	}
	r.BodyJSON = *res
	bb, err := json.Marshal(res)
	if err != nil {
		return err
	}
	r.Body = string(bb)
	return nil
}

func (e Executor) getRequestBody(workdir string) (res interface{}, err error) {
	var bytes []byte
	if e.Body != "" {
		bytes = []byte(e.Body)
	} else if e.BodyFile != "" {
		path := filepath.Join(workdir, e.BodyFile)
		if _, err = os.Stat(path); !os.IsNotExist(err) {
			bytes, err = os.ReadFile(path)
			if err != nil {
				return nil, err
			}
		}
	}
	if len(bytes) > 0 {
		res = new(interface{})
		err = venom.JSONUnmarshal(bytes, res)
		return
	}
	return nil, nil
}
