## Venom Executor

### Write your executor

An executor has to implement this interface

```go

// Executor executes a testStep.
type Executor interface {
	// Run runs a Test Step
	Run(ctx context.Content, TestStep) (interface{}, error)
}
```

Example

```go


// Name of executor
const Name = "myexecutor"

// New returns a new Executor
func New() venom.Executor {
	return &Executor{}
}

// Executor struct
type Executor struct {
	Command string `json:"command,omitempty" yaml:"command,omitempty"`
}

// Result represents a step result
type Result struct {
	Code        int    `json:"code,omitempty" yaml:"code,omitempty"`
	Command     string `json:"command,omitempty" yaml:"command,omitempty"`
	Systemout   string   `json:"systemout,omitempty" yaml:"systemout,omitempty"` // put in testcase.Systemout by venom if present
	Systemerr   string   `json:"systemerr,omitempty" yaml:"systemerr,omitempty"` // put in testcase.Systemerr by venom if present
}

// GetDefaultAssertions returns the default assertions for this executor
// Optional
func (Executor) GetDefaultAssertions() *venom.StepAssertions {
	return &venom.StepAssertions{Assertions: []venom.Assertion{"result.code ShouldEqual 0"}}
}

// Run executes TestStep
func (Executor)	Run(ctx context.Context, step venom.TestStep) (interface{}, error) {
	// transform step to Executor Instance
	var e Executor
	if err := mapstructure.Decode(step, &e); err != nil {
		return nil, err
	}

	// to something with e.Command here...
	//...

	systemout := "foo"
	outputCode := 0

	// prepare result
	r := Result{
		Code:    outputCode, // return Output Code
		Command: e.Command, // return Command executed
		Systemout: systemout, // return Output string
	}

	return r, nil
}

```

Feel free to open a Pull Request with your executors.

### TLS

Six executors build their own TLS connection: http, grpc, kafka, imap, smtp and ovhapi. They read their TLS
settings the same way. Each executor keeps its own switch and its own default.

| Executor | TLS is on when | Skip the server check | Root CA | Client certificate and key | Server certificate checked by default |
|---|---|---|---|---|---|
| http | `url` starts with `https://` | `ignore_verify_ssl: true` | `tls_root_ca` | `tls_client_cert`, `tls_client_key` | yes |
| grpc | `tls_root_ca` is set | `ignore_verify_ssl: true` | `tls_root_ca` | `tls_client_cert`, `tls_client_key` | yes |
| kafka | `with_tls: true` (or `withTLS`) | `insecure_tls: true` (or `insecureTLS`) | `tls_root_ca` | `tls_client_cert`, `tls_client_key` | yes |
| imap | `auth.withtls: true` | leave `auth.verifytls` unset or `false` | `auth.tls_root_ca` | `auth.tls_client_cert`, `auth.tls_client_key` | no: set `auth.verifytls: true` |
| smtp | `withtls: true` | leave `verifytls` unset or `false` | `tls_root_ca` | `tls_client_cert`, `tls_client_key` | no: set `verifytls: true` |
| ovhapi | `endpoint` is `https://` (the named endpoints, such as `ovh-eu`, are) | none | `tlsRootCA` or `tls_root_ca`, else the `ovh.tlsRootCA` variable | none: the OVH API does not use one | yes |

The rules they share:

- A root CA, a client certificate or a key is either the PEM text itself or the path of a PEM file. A relative path
  is read from the test suite's directory. Several CAs can be concatenated in one root CA.
- The root CA is trusted on top of the system roots, so a server signed by a private CA is verified without skipping
  the check.
- Skipping the server check makes the root CA irrelevant, but the client certificate is still sent.
- A client certificate without its key, or a key without its certificate, fails the step with an error that names the
  missing setting. This holds even when TLS is off.
- On grpc, kafka, imap and smtp, a root CA or a client certificate given while TLS is off is not used: the connection
  stays plaintext and venom logs a warning. On http and ovhapi the URL decides: with an `http://` URL venom logs a
  warning too, which says that a redirect to `https://` or an `https://` proxy (the step's `proxy`, `ovh.proxy` or
  `HTTPS_PROXY`/`HTTP_PROXY`) still uses them (http follows redirects unless `no_follow_redirect` is set). An ovhapi endpoint name such as `ovh-eu` is an `https://` URL and never warns.
- kafka reads both spellings of its two switches: `with_tls` or `withTLS`, `insecure_tls` or `insecureTLS`. Either
  one turns its switch on.
- ovhapi refuses a step that sets both `tlsRootCA` and `tls_root_ca`.

Out of scope: the executors whose TLS comes from a connection string or a driver (sql, mongo, redis, amqp, rabbitmq,
mqtt, couchbase) and the web executor, which always ignores HTTPS certificate errors (see its README).

`executors/tls_conformance_test.go` runs the same cases through all six executors against one throwaway CA: a trusted
CA, a CA that did not sign the server certificate, a skipped check with a client certificate, and a certificate
without its key.
