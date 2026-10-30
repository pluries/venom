package grpc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/fullstorydev/grpcurl"
	"github.com/jhump/protoreflect/desc" //nolint:staticcheck // grpcurl.InvocationEventHandler takes *desc.MethodDescriptor; remove when grpcurl moves to protobuf-go (#2035)
	"github.com/jhump/protoreflect/grpcreflect"
	"github.com/mitchellh/mapstructure"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/runtime/protoiface"

	"github.com/ovh/venom"
	"github.com/ovh/venom/executors/internal/tlsconfig"
)

// Name for test exec
const Name = "grpc"

// New returns a new Test Exec
func New() venom.Executor {
	return &Executor{}
}

// Executor represents a Test Exec
type Executor struct {
	URL                  string                 `json:"url" yaml:"url"`
	Service              string                 `json:"service" yaml:"service"`
	Method               string                 `json:"method" yaml:"method"`
	JSONDefaultFields    bool                   `json:"default_fields" yaml:"default_fields"`
	IncludeTextSeparator bool                   `json:"include_text_separator" yaml:"include_text_separator"`
	Data                 map[string]interface{} `json:"data" yaml:"data"`
	Headers              map[string]string      `json:"headers" yaml:"headers"`
	ConnectTimeout       *int64                 `json:"connect_timeout" yaml:"connect_timeout"`
	TLSClientCert        string                 `json:"tls_client_cert" yaml:"tls_client_cert" mapstructure:"tls_client_cert"`
	TLSClientKey         string                 `json:"tls_client_key" yaml:"tls_client_key" mapstructure:"tls_client_key"`
	TLSRootCA            string                 `json:"tls_root_ca" yaml:"tls_root_ca" mapstructure:"tls_root_ca"`
	IgnoreVerifySSL      bool                   `json:"ignore_verify_ssl" yaml:"ignore_verify_ssl" mapstructure:"ignore_verify_ssl"`

	// The three fields below take the snake_case keys the README documents (default_fields, include_text_separator,
	// connect_timeout), which the Go field names above do not reach; either spelling sets the option (#2038). They are
	// decoded only: the json and yaml names belong to the fields above.
	JSONDefaultFieldsSnake    bool   `json:"-" yaml:"-" mapstructure:"default_fields"`
	IncludeTextSeparatorSnake bool   `json:"-" yaml:"-" mapstructure:"include_text_separator"`
	ConnectTimeoutSnake       *int64 `json:"-" yaml:"-" mapstructure:"connect_timeout"`
}

// Result represents a step result
type Result struct {
	Systemout     string        `json:"systemout,omitempty" yaml:"systemout,omitempty"`
	SystemoutJSON interface{}   `json:"systemoutjson,omitempty" yaml:"systemoutjson,omitempty"`
	Systemerr     string        `json:"systemerr,omitempty" yaml:"systemerr,omitempty"`
	SystemerrJSON interface{}   `json:"systemerrjson,omitempty" yaml:"systemerrjson,omitempty"`
	Err           string        `json:"err,omitempty" yaml:"err,omitempty"`
	Code          string        `json:"code,omitempty" yaml:"code,omitempty"`
	Details       []interface{} `json:"details,omitempty" yaml:"details,omitempty"`
	TimeSeconds   float64       `json:"timeseconds,omitempty" yaml:"timeseconds,omitempty"`
}

type customHandler struct {
	formatter grpcurl.Formatter
	target    *Result
	err       error
}

// OnResolveMethod is called with a descriptor of the method that is being invoked.
func (*customHandler) OnResolveMethod(_ *desc.MethodDescriptor) {}

// OnSendHeaders is called with the request metadata that is being sent.
func (*customHandler) OnSendHeaders(metadata.MD) {}

// OnReceiveHeaders is called when response headers have been received.
func (*customHandler) OnReceiveHeaders(_ metadata.MD) {}

// OnReceiveResponse is called for each response message received.
func (c *customHandler) OnReceiveResponse(msg protoiface.MessageV1) {
	res, err := c.formatter(msg)
	if err != nil || c.err != nil {
		c.err = err
		return
	}
	c.target.Systemout = res
}

// OnReceiveTrailers is called when response trailers and final RPC status have been received.
func (c *customHandler) OnReceiveTrailers(stat *status.Status, _ metadata.MD) {
	if err := stat.Err(); err != nil {
		c.target.Systemerr = err.Error()

		// Extract gRPC error details
		for _, d := range stat.Details() {
			jsonBytes, err := json.Marshal(d)
			if err != nil {
				continue
			}
			var detailMap map[string]interface{}
			if err := json.Unmarshal(jsonBytes, &detailMap); err != nil {
				continue
			}
			c.target.Details = append(c.target.Details, detailMap)
		}
	}
	c.target.Code = strconv.Itoa(int(uint32(stat.Code())))
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
	// decode test
	var e Executor
	if err := mapstructure.Decode(step, &e); err != nil {
		return nil, err
	}

	headers := requestHeaders(e.Headers)

	// prepare data
	data, err := json.Marshal(e.Data)
	if err != nil {
		return nil, fmt.Errorf("runGrpcurl: Cannot marshal request data: %s", err)
	}

	result := Result{}
	start := time.Now()

	var cc *grpc.ClientConn
	var descSource grpcurl.DescriptorSource
	var refClient *grpcreflect.Client
	md := grpcurl.MetadataFromHeaders(headers)
	refCtx := metadata.NewOutgoingContext(ctx, md)
	cc, err = e.dial(ctx)
	if err != nil {
		return Result{Err: err.Error()}, fmt.Errorf("grpc dial error: %w", err)
	}
	refClient = grpcreflect.NewClientAuto(refCtx, cc)
	descSource = grpcurl.DescriptorSourceFromServer(ctx, refClient)

	// arrange for the RPCs to be cleanly shutdown
	defer func() {
		if refClient != nil {
			refClient.Reset()
			refClient = nil
		}
		if cc != nil {
			_ = cc.Close()
			cc = nil
		}
	}()

	// prepare request and send
	in := bytes.NewReader(data)
	rf, formatter, err := grpcurl.RequestParserAndFormatterFor(
		grpcurl.FormatJSON,
		descSource,
		e.JSONDefaultFields || e.JSONDefaultFieldsSnake,
		e.IncludeTextSeparator || e.IncludeTextSeparatorSnake,
		in,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to construct request parser and formatter %s", err)
	}

	// prepare custom handler to handle response
	handle := customHandler{
		formatter,
		&result,
		nil,
	}

	// invoke the gRPC
	err = grpcurl.InvokeRPC(ctx, descSource, cc, e.Service+"/"+e.Method, headers, &handle, rf.Next)
	if err != nil {
		return nil, fmt.Errorf("grpcurl.InvokeRPC() failed.\nUrl: %q\nService: %q\nMethod: %q\nData:%v: %v", e.URL, e.Service, e.Method, e.Data, err)
	}

	elapsed := time.Since(start)
	result.TimeSeconds = elapsed.Seconds()

	if handle.err != nil {
		result.Err = handle.err.Error()
	}

	result.SystemoutJSON = parseOutputJSON(result.Systemout)
	result.SystemerrJSON = parseOutputJSON(result.Systemerr)

	return result, nil
}

// requestHeaders turns the step's headers into grpcurl's "name: value" form. The slice starts with
// len(h) empty entries before the appended ones, as it always has.
func requestHeaders(h map[string]string) []string {
	headers := make([]string, len(h))
	for k, v := range h {
		headers = append(headers, fmt.Sprintf("%s: %s", k, v))
	}
	return headers
}

// dialTimeout is connect_timeout in seconds when positive, else 10 seconds.
func (e Executor) dialTimeout() time.Duration {
	seconds := e.ConnectTimeout
	if seconds == nil {
		seconds = e.ConnectTimeoutSnake
	}
	if seconds != nil && *seconds > 0 {
		return time.Duration(*seconds * int64(time.Second))
	}
	return 10 * time.Second
}

// dial connects to e.URL within the dial timeout, over TLS when a root CA is set.
func (e Executor) dial(ctx context.Context) (*grpc.ClientConn, error) {
	ctx, cancel := context.WithTimeout(ctx, e.dialTimeout())
	defer cancel()

	workdir := venom.StringVarFromCtx(ctx, "venom.testsuite.workdir")

	creds, err := e.transportCredentials(ctx, workdir)
	if err != nil {
		return nil, err
	}

	cc, err := grpcurl.BlockingDial(ctx, "tcp", e.URL, creds)
	if err != nil {
		return nil, fmt.Errorf("failed to dial grpc: %w", err)
	}
	return cc, nil
}

// warnf is venom.Warning; a test replaces it.
var warnf = venom.Warning

// transportCredentials returns nil (plaintext) without a root CA, TLS credentials with one, and mutual TLS credentials
// when a client certificate and key are set as well; one without the other is an error. The configuration comes from
// the shared tlsconfig package (ADR-0009): the root CA is trusted on top of the system pool, and the client
// certificate is sent even when ignore_verify_ssl skips the server check. A client pair set without a root CA is not
// used: the call stays plaintext, as it always was, and a warning says so; half a pair fails in every case.
func (e Executor) transportCredentials(ctx context.Context, workdir string) (credentials.TransportCredentials, error) {
	if err := tlsconfig.CheckClientPair(e.TLSClientCert, e.TLSClientKey); err != nil {
		return nil, err
	}
	if e.TLSRootCA == "" {
		if e.TLSClientCert != "" {
			warnf(ctx, "grpc: tls_client_cert and tls_client_key are not used without tls_root_ca: the connection is plaintext")
		}
		return nil, nil
	}

	cfg, err := tlsconfig.New(ctx, tlsconfig.Settings{
		Workdir:            workdir,
		RootCA:             e.TLSRootCA,
		ClientCert:         e.TLSClientCert,
		ClientKey:          e.TLSClientKey,
		InsecureSkipVerify: e.IgnoreVerifySSL,
	})
	if err != nil {
		return nil, err
	}
	return credentials.NewTLS(cfg), nil
}

// parseOutputJSON decodes s as a JSON array, else as a JSON object, else returns nil.
func parseOutputJSON(s string) interface{} {
	var outJSONArray []interface{}
	if err := venom.JSONUnmarshal([]byte(s), &outJSONArray); err == nil {
		return outJSONArray
	}
	outJSONMap := map[string]interface{}{}
	if err := venom.JSONUnmarshal([]byte(s), &outJSONMap); err == nil {
		return outJSONMap
	}
	return nil
}
