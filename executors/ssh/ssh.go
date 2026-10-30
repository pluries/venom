package ssh

import (
	"context"
	"fmt"
	"io"
	"os/user"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/mitchellh/mapstructure"
	"golang.org/x/crypto/ssh"

	"github.com/ovh/venom"
	"github.com/ovh/venom/executors/internal/sshconn"
)

// Name for test ssh
const (
	Name       = "ssh"
	sudoprompt = "sudo_venom"
)

// New returns a new Test Exec
func New() venom.Executor {
	return &Executor{}
}

// Executor represents a Test Exec
type Executor struct {
	Host         string `json:"host,omitempty" yaml:"host,omitempty"`
	Command      string `json:"command,omitempty" yaml:"command,omitempty"`
	User         string `json:"user,omitempty" yaml:"user,omitempty"`
	Password     string `json:"password,omitempty" yaml:"password,omitempty"`
	PrivateKey   string `json:"privatekey,omitempty" yaml:"privatekey,omitempty"`
	Sudo         string `json:"sudo,omitempty" yaml:"sudo,omitempty"`
	SudoPassword string `json:"sudopassword,omitempty" yaml:"sudopassword,omitempty"`
	// KnownHosts is the path of an OpenSSH known_hosts file the host's key is checked against; relative to the suite.
	// A pointer, like HostKey, so that a key set to nothing is told from a key not set: the first is an error, not
	// the unchecked default.
	KnownHosts *string `json:"knownhosts,omitempty" yaml:"knownhosts,omitempty"`
	// HostKey is the one public key, in authorized_keys format, the host must present. Exclusive with KnownHosts.
	HostKey *string `json:"hostkey,omitempty" yaml:"hostkey,omitempty"`
}

// Result represents a step result
type Result struct {
	Systemout   string  `json:"systemout,omitempty" yaml:"systemout,omitempty"`
	Systemerr   string  `json:"systemerr,omitempty" yaml:"systemerr,omitempty"`
	Err         string  `json:"err,omitempty" yaml:"err,omitempty"`
	Code        string  `json:"code,omitempty" yaml:"code,omitempty"`
	TimeSeconds float64 `json:"timeseconds,omitempty" yaml:"timeseconds,omitempty"`
}

// ZeroValueResult return an empty implementation of this executor result
func (Executor) ZeroValueResult() interface{} {
	return Result{}
}

// GetDefaultAssertions return default assertions for type exec
func (Executor) GetDefaultAssertions() *venom.StepAssertions {
	return &venom.StepAssertions{Assertions: []venom.Assertion{"result.code ShouldEqual 0"}}
}

// Run execute TestStep of type exec
func (Executor) Run(ctx context.Context, step venom.TestStep) (interface{}, error) {
	var e Executor
	if err := mapstructure.Decode(step, &e); err != nil {
		return nil, err
	}

	if e.Command == "" {
		return nil, fmt.Errorf("invalid command")
	}

	start := time.Now()
	result := Result{}

	client, session, err := e.connect(ctx)
	if err != nil {
		result.Err = err.Error()
	} else {
		defer client.Close()
		stdout := &Buffer{}
		stderr := &Buffer{}

		session.Stderr = stderr
		session.Stdout = stdout
		// StdinPipe's only error cases are session.Stdin already set or the session already started
		// (golang.org/x/crypto/ssh); neither happens here: session.Stdin is never assigned and Run
		// has not been called yet.
		stdin, _ := session.StdinPipe() //nolint:errcheck // verified above: cannot fail at this call site

		// Handle sudo password
		command := e.Command
		quit := make(chan bool)
		if e.Sudo != "" {
			command = "TERM=xterm-mono sudo -S -p " + sudoprompt + " -u " + e.Sudo + " " + command
			if e.SudoPassword == "" {
				e.SudoPassword = e.Password
			}
			go handleSudo(ctx, stdin, stdout, quit, e.SudoPassword)
		}

		if err := session.Run(command); err != nil {
			if exiterr, ok := err.(*ssh.ExitError); ok {
				status := exiterr.ExitStatus()
				result.Code = strconv.Itoa(status)
			} else if _, ok := err.(*ssh.ExitMissingError); ok {
				result.Code = strconv.Itoa(127)
				result.Err = err.Error()
			} else {
				result.Code = strconv.Itoa(137)
				result.Err = err.Error()
			}
		} else {
			result.Code = "0"
		}

		if e.Sudo != "" {
			quit <- true
		}
		result.Systemerr = strings.TrimSpace(stderr.String())
		result.Systemout = strings.TrimSpace(stdout.String())
	}

	elapsed := time.Since(start)
	result.TimeSeconds = elapsed.Seconds()

	return result, nil
}

func handleSudo(ctx context.Context, in io.Writer, out *Buffer, quit chan bool, password string) {
	sudopromptlen := len(sudoprompt)
	for {
		select {
		case <-quit:
			return
		default:
			content := out.String()
			bufferLen := utf8.RuneCountInString(content)

			// Check if we have to enter password
			if bufferLen >= sudopromptlen && strings.Contains(content[bufferLen-sudopromptlen:], sudoprompt) {
				if _, err := in.Write([]byte(password + "\n")); err != nil {
					// Best-effort: session.Run surfaces the failure itself; this warning gives the why.
					// No return: Run always sends on the unbuffered quit, so this loop must stay to receive it.
					venom.Warning(ctx, "sudo: failed to write password to stdin: %v", err)
				}
				out.Truncate(0)
			}
		}
	}
}

// connect opens a session on the step's host, checking the host's key the way the step asks. A host-key setting it
// cannot use (both keys set, a key set but empty, an unreadable file, an unparsable key) is an error before dialing,
// reported like an unreadable privatekey.
func (e Executor) connect(ctx context.Context) (*ssh.Client, *ssh.Session, error) {
	check, err := e.newHostKeyCheck(ctx)
	if err != nil {
		return nil, nil, err
	}
	return connectToHost(e.User, e.Password, e.PrivateKey, e.Host, e.Sudo, check)
}

// warnf is venom.Warn; a test replaces it. The warning is limited to one per run by sshconn (#2298).
var warnf = venom.Warn

// resetInsecureWarning re-arms the warning, for tests only.
func resetInsecureWarning() { sshconn.ResetInsecureWarning() }

// newHostKeyCheck returns the check the step asks for (see sshconn.NewHostKeyCheck).
func (e Executor) newHostKeyCheck(ctx context.Context) (sshconn.HostKeyCheck, error) {
	return sshconn.NewHostKeyCheck(sshconn.HostKeySettings{
		KnownHosts: e.KnownHosts,
		HostKey:    e.HostKey,
		Workdir:    venom.StringVarFromCtx(ctx, "venom.testsuite.workdir"),
		Host:       e.Host,
	}, func(format string, args ...any) { warnf(ctx, format, args...) })
}

func connectToHost(u, pass, key, host, sudo string, check sshconn.HostKeyCheck) (*ssh.Client, *ssh.Session, error) {
	// Default user is current username
	if u == "" {
		osUser, err := user.Current()
		if err != nil {
			return nil, nil, err
		}
		u = osUser.Username
	}

	// If password is set, and we don't have key use it
	var auth []ssh.AuthMethod
	if pass != "" && key == "" {
		auth = []ssh.AuthMethod{ssh.Password(pass)}
	} else {
		// Load the the private key
		key, err := sshconn.PrivateKey(key, "")
		if err != nil {
			return nil, nil, err
		}
		auth = []ssh.AuthMethod{ssh.PublicKeys(key)}
	}

	// Open the tcp connection
	client, err := sshconn.Dial(host, u, auth, check)
	if err != nil {
		return nil, nil, err
	}

	// New ssh session
	session, err := client.NewSession()
	if err != nil {
		client.Close()
		return nil, nil, err
	}

	// Request PTY for sudo cmd
	if sudo != "" {
		modes := ssh.TerminalModes{
			ssh.TTY_OP_ISPEED: 14400, // input speed = 14.4kbaud
			ssh.TTY_OP_OSPEED: 14400, // output speed = 14.4kbaud
		}

		if err := session.RequestPty("xterm", 40, 80, modes); err != nil {
			return nil, nil, err
		}
	}

	return client, session, nil
}
