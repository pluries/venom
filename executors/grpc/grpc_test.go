package grpc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/fullstorydev/grpcurl"
	"github.com/mitchellh/mapstructure"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/runtime/protoiface"

	"github.com/ovh/venom"
	"github.com/ovh/venom/executors/internal/tlsconfig/tlstest"
)

// Characterisation tests for the grpc executor (#2031, reopen item C1). They pin what the executor does today against
// an in-process server (grpc_helpers_test.go), odd parts included; nothing here is a fix.

// okResult checks the timing of a Result and zeroes it, so a whole struct can be compared.
func okResult(t *testing.T, res Result) Result {
	t.Helper()
	assert.Greater(t, res.TimeSeconds, 0.0)
	res.TimeSeconds = 0
	return res
}

func TestRun_plaintext(t *testing.T) {
	srv := startServer(t)

	tests := []struct {
		name  string
		extra venom.TestStep
		want  Result
	}{
		{
			name:  "a serving service answers code 0 with the indented JSON of the response",
			extra: venom.TestStep{"data": map[string]interface{}{"service": servingService}},
			want: Result{
				Systemout:     "{\n  \"status\": \"SERVING\"\n}",
				SystemoutJSON: map[string]interface{}{"status": "SERVING"},
				Code:          "0",
			},
		},
		{
			name:  "no data sends an empty request: the server's overall status",
			extra: venom.TestStep{},
			want: Result{
				Systemout:     "{\n  \"status\": \"SERVING\"\n}",
				SystemoutJSON: map[string]interface{}{"status": "SERVING"},
				Code:          "0",
			},
		},
		{
			name:  "a zero-value field is omitted without default_fields",
			extra: venom.TestStep{"data": map[string]interface{}{"service": zeroStatusService}},
			want:  Result{Systemout: "{}", SystemoutJSON: map[string]interface{}{}, Code: "0"},
		},
		{
			// #2038: default_fields, the documented key, is decoded like its field-name spelling below.
			name:  "default_fields emits the zero-value field",
			extra: venom.TestStep{"data": map[string]interface{}{"service": zeroStatusService}, "default_fields": true},
			want: Result{
				Systemout:     "{\n  \"status\": \"UNKNOWN\"\n}",
				SystemoutJSON: map[string]interface{}{"status": "UNKNOWN"},
				Code:          "0",
			},
		},
		{
			name:  "the field-name key jsonDefaultFields emits the zero-value field",
			extra: venom.TestStep{"data": map[string]interface{}{"service": zeroStatusService}, "jsonDefaultFields": true},
			want: Result{
				Systemout:     "{\n  \"status\": \"UNKNOWN\"\n}",
				SystemoutJSON: map[string]interface{}{"status": "UNKNOWN"},
				Code:          "0",
			},
		},
		{
			// The separator only applies to grpcurl's text format.
			name: "includeTextSeparator changes nothing in the JSON format",
			extra: venom.TestStep{
				"data":                 map[string]interface{}{"service": servingService},
				"includeTextSeparator": true,
			},
			want: Result{
				Systemout:     "{\n  \"status\": \"SERVING\"\n}",
				SystemoutJSON: map[string]interface{}{"status": "SERVING"},
				Code:          "0",
			},
		},
		{
			// A non-OK status is a result, not a Run error; Systemerr is not JSON so SystemerrJSON stays nil.
			name:  "an unknown health service is NOT_FOUND in the result",
			extra: venom.TestStep{"data": map[string]interface{}{"service": missingService}},
			want:  Result{Systemerr: "rpc error: code = NotFound desc = unknown service", Code: "5"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, ok, err := grpcRun(context.Background(), t, healthStep(srv.addr, tt.extra))
			require.NoError(t, err)
			require.True(t, ok)
			assert.Equal(t, tt.want, okResult(t, res))
		})
	}
}

func TestRun_invokeErrors(t *testing.T) {
	srv := startServer(t)

	tests := []struct {
		name    string
		step    venom.TestStep
		wantErr string
	}{
		{
			name: "an unknown method",
			step: venom.TestStep{"url": srv.addr, "service": healthService, "method": "Nope"},
			wantErr: fmt.Sprintf("grpcurl.InvokeRPC() failed.\nUrl: %q\nService: %q\nMethod: \"Nope\"\nData:map[]: "+
				"service %q does not include a method named \"Nope\"", srv.addr, healthService, healthService),
		},
		{
			name: "a service the server does not expose",
			step: venom.TestStep{"url": srv.addr, "service": missingService, "method": "Check"},
			wantErr: fmt.Sprintf("grpcurl.InvokeRPC() failed.\nUrl: %q\nService: %q\nMethod: \"Check\"\nData:map[]: "+
				"target server does not expose service %q", srv.addr, missingService, missingService),
		},
		{
			name: "data with a field the request does not have",
			step: healthStep(srv.addr, venom.TestStep{"data": map[string]interface{}{"nope": 1}}),
			wantErr: fmt.Sprintf("grpcurl.InvokeRPC() failed.\nUrl: %q\nService: %q\nMethod: \"Check\"\nData:map[nope:1]: "+
				"error getting request data: message type grpc.health.v1.HealthCheckRequest has no known field named nope",
				srv.addr, healthService),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, ok, err := grpcRun(context.Background(), t, tt.step)
			assert.False(t, ok, "Run returns a nil result")
			require.EqualError(t, err, tt.wantErr)
		})
	}
}

func TestRun_stepErrors(t *testing.T) {
	tests := []struct {
		name    string
		step    venom.TestStep
		wantErr string
	}{
		{
			name:    "a step that does not decode",
			step:    venom.TestStep{"url": 42},
			wantErr: "1 error(s) decoding:\n\n* 'URL' expected type 'string', got unconvertible type 'int', value: '42'",
		},
		{
			name:    "data that cannot be marshalled to JSON",
			step:    venom.TestStep{"url": "127.0.0.1:1", "data": map[string]interface{}{"c": make(chan int)}},
			wantErr: "runGrpcurl: Cannot marshal request data: json: unsupported type: chan int",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, ok, err := grpcRun(context.Background(), t, tt.step)
			assert.False(t, ok, "Run returns a nil result")
			require.ErrorContains(t, err, tt.wantErr)
		})
	}
}

func TestRun_headers(t *testing.T) {
	srv := startServer(t)

	// requestHeaders' leading empty entries are skipped by grpcurl.MetadataFromHeaders; names arrive lower-cased,
	// on the reflection streams as well as on the call.
	res, ok, err := grpcRun(context.Background(), t, healthStep(srv.addr, venom.TestStep{
		"data":    map[string]interface{}{"service": servingService},
		"headers": map[string]string{"X-Tenant": "acme", "x-trace": "t-1"},
	}))
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, "0", res.Code)

	unary, stream := srv.seen()
	require.Len(t, unary, 1)
	assert.Equal(t, []string{"acme"}, unary[0].Get("x-tenant"))
	assert.Equal(t, []string{"t-1"}, unary[0].Get("x-trace"))
	assert.Empty(t, unary[0].Get(""), "no empty header name reaches the server")
	require.NotEmpty(t, stream, "the method is resolved through reflection")
	for _, md := range stream {
		assert.Equal(t, []string{"acme"}, md.Get("x-tenant"))
	}
}

func TestRun_dialFailures(t *testing.T) {
	t.Run("a server that never answers times out after connectTimeout seconds", func(t *testing.T) {
		addr := startSilentListener(t)
		start := time.Now()
		got, err := Executor{}.Run(context.Background(), venom.TestStep{
			"url": addr, "service": healthService, "method": "Check", "connectTimeout": 1,
		})
		elapsed := time.Since(start)

		assert.Equal(t, Result{Err: "failed to dial grpc: context deadline exceeded"}, got)
		require.EqualError(t, err, "grpc dial error: failed to dial grpc: context deadline exceeded")
		assert.ErrorIs(t, err, context.DeadlineExceeded)
		assert.GreaterOrEqual(t, elapsed, 900*time.Millisecond)
		assert.Less(t, elapsed, 5*time.Second, "the 10-second default did not apply")
	})

	t.Run("invalid TLS settings fail the dial before connecting", func(t *testing.T) {
		got, err := Executor{}.Run(context.Background(), venom.TestStep{
			"url": "127.0.0.1:1", "service": healthService, "method": "Check", "tls_root_ca": "not a PEM",
		})
		assert.Equal(t, Result{Err: "TLSRootCA: failed to add a certificate to the cert pool"}, got)
		require.EqualError(t, err, "grpc dial error: TLSRootCA: failed to add a certificate to the cert pool")
	})
}

func TestRun_stepKeys(t *testing.T) {
	// Run decodes the step with mapstructure, which reads mapstructure tags, not the json/yaml ones. Three fields have
	// no mapstructure tag, so the keys the README documents (connect_timeout, default_fields, include_text_separator)
	// reach decode-only alias fields (#2038); the Go field name, in any case, still reaches the field itself.
	tests := []struct {
		name string
		step venom.TestStep
		want Executor
	}{
		{
			name: "documented keys",
			step: venom.TestStep{
				"url": "h:1", "service": "s", "method": "m", "connect_timeout": 3, "default_fields": true,
				"include_text_separator": true, "tls_root_ca": "ca", "tls_client_cert": "c", "tls_client_key": "k",
				"ignore_verify_ssl": true, "headers": map[string]string{"a": "b"}, "data": map[string]interface{}{"d": 1},
			},
			want: Executor{
				URL: "h:1", Service: "s", Method: "m", JSONDefaultFieldsSnake: true, IncludeTextSeparatorSnake: true, ConnectTimeoutSnake: func() *int64 { n := int64(3); return &n }(), TLSRootCA: "ca", TLSClientCert: "c", TLSClientKey: "k",
				IgnoreVerifySSL: true, Headers: map[string]string{"a": "b"}, Data: map[string]interface{}{"d": 1},
			},
		},
		{
			name: "field-name keys",
			step: venom.TestStep{"connectTimeout": 3, "JSONDefaultFields": true, "includetextseparator": true},
			want: Executor{ConnectTimeout: func() *int64 { n := int64(3); return &n }(), JSONDefaultFields: true, IncludeTextSeparator: true},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got Executor
			require.NoError(t, mapstructure.Decode(tt.step, &got))
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestDialTimeout(t *testing.T) {
	seconds := func(n int64) *int64 { return &n }
	tests := []struct {
		name    string
		timeout *int64
		want    time.Duration
	}{
		{name: "unset", timeout: nil, want: 10 * time.Second},
		{name: "zero", timeout: seconds(0), want: 10 * time.Second},
		{name: "negative", timeout: seconds(-3), want: 10 * time.Second},
		{name: "positive", timeout: seconds(3), want: 3 * time.Second},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, Executor{ConnectTimeout: tt.timeout}.dialTimeout())
		})
	}
}

func TestRun_TLS(t *testing.T) {
	pki := tlstest.New(t)
	srv := startServer(t, grpc.Creds(credentials.NewTLS(pki.ServerConfig(false))))
	workdir := t.TempDir()
	caFile := writeFile(t, workdir, "ca.pem", pki.CAPEM)
	serving := venom.TestStep{"data": map[string]interface{}{"service": servingService}}

	merge := func(steps ...venom.TestStep) venom.TestStep {
		out := venom.TestStep{}
		for _, s := range steps {
			for k, v := range s {
				out[k] = v
			}
		}
		return out
	}

	t.Run("the root CA makes the server certificate trusted", func(t *testing.T) {
		// #2368: tls_root_ca used to land in tls.Config.ClientCAs, so it only worked with ignore_verify_ssl.
		for name, ca := range map[string]string{"inline": string(pki.CAPEM), "relative file": "ca.pem", "absolute file": caFile} {
			t.Run(name, func(t *testing.T) {
				res, ok, err := grpcRun(workdirCtx(workdir), t, healthStep(srv.addr, merge(serving, venom.TestStep{"tls_root_ca": ca})))
				require.NoError(t, err)
				require.True(t, ok)
				assert.Equal(t, "0", res.Code)
				assert.Equal(t, map[string]interface{}{"status": "SERVING"}, res.SystemoutJSON)
			})
		}
	})

	t.Run("a root CA that did not sign the server certificate is refused", func(t *testing.T) {
		other := tlstest.New(t)
		got, err := Executor{}.Run(workdirCtx(workdir), healthStep(srv.addr, merge(serving, venom.TestStep{"tls_root_ca": string(other.CAPEM)})))
		// Both test CAs are named "venom test CA", so Go appends which candidate failed: compare the stable prefix.
		const want = "grpc dial error: failed to dial grpc: tls: failed to verify certificate: x509: certificate signed by unknown authority"
		require.Error(t, err)
		assert.True(t, strings.HasPrefix(err.Error(), want), err.Error())
		assert.Equal(t, Result{Err: strings.TrimPrefix(err.Error(), "grpc dial error: ")}, got)
	})

	t.Run("ignore_verify_ssl connects", func(t *testing.T) {
		for name, ca := range map[string]string{"inline": string(pki.CAPEM), "relative file": "ca.pem", "absolute file": caFile} {
			t.Run(name, func(t *testing.T) {
				// The file forms prove the file was read: its path alone is not PEM and would fail with
				// "TLSRootCA: failed to add a certificate to the cert pool".
				res, ok, err := grpcRun(workdirCtx(workdir), t, healthStep(srv.addr, merge(serving, venom.TestStep{"tls_root_ca": ca, "ignore_verify_ssl": true})))
				require.NoError(t, err)
				require.True(t, ok)
				assert.Equal(t, "0", res.Code)
				assert.Equal(t, map[string]interface{}{"status": "SERVING"}, res.SystemoutJSON)
			})
		}
	})

	t.Run("a plaintext step against a TLS server fails the dial", func(t *testing.T) {
		start := time.Now()
		got, err := Executor{}.Run(context.Background(), healthStep(srv.addr, merge(serving, venom.TestStep{"connectTimeout": 1})))
		// The TLS server drops the connection; whether the client reads EOF or a reset depends on timing.
		require.Error(t, err)
		assert.Regexp(t, `^grpc dial error: failed to dial grpc: (EOF|read tcp [0-9.:]+->[0-9.:]+: read: connection reset by peer)$`, err.Error())
		res, ok := got.(Result)
		require.True(t, ok)
		assert.Equal(t, strings.TrimPrefix(err.Error(), "grpc dial error: "), res.Err)
		assert.Less(t, time.Since(start), 900*time.Millisecond, "the server closes the connection: no wait for the timeout")
	})
}

func TestRun_mutualTLS(t *testing.T) {
	pki := tlstest.New(t)
	srv := startServer(t, grpc.Creds(credentials.NewTLS(pki.ServerConfig(true))))
	workdir := t.TempDir()
	writeFile(t, workdir, "client.pem", pki.ClientCertPEM)
	writeFile(t, workdir, "client.key", pki.ClientKeyPEM)

	step := func(extra venom.TestStep) venom.TestStep {
		s := healthStep(srv.addr, venom.TestStep{
			"data":              map[string]interface{}{"service": servingService},
			"tls_root_ca":       string(pki.CAPEM),
			"ignore_verify_ssl": true,
			"connectTimeout":    2,
		})
		for k, v := range extra {
			s[k] = v
		}
		return s
	}

	t.Run("a client certificate and key connect", func(t *testing.T) {
		for name, extra := range map[string]venom.TestStep{
			"inline": {"tls_client_cert": string(pki.ClientCertPEM), "tls_client_key": string(pki.ClientKeyPEM)},
			"files":  {"tls_client_cert": "client.pem", "tls_client_key": "client.key"},
			"files, server certificate verified": {
				"tls_client_cert": "client.pem", "tls_client_key": "client.key", "ignore_verify_ssl": false,
			},
		} {
			t.Run(name, func(t *testing.T) {
				res, ok, err := grpcRun(workdirCtx(workdir), t, step(extra))
				require.NoError(t, err)
				require.True(t, ok)
				assert.Equal(t, "0", res.Code)
			})
		}
	})

	t.Run("without a client certificate the server refuses the connection", func(t *testing.T) {
		got, err := Executor{}.Run(workdirCtx(workdir), step(venom.TestStep{}))
		// Usually the server's TLS alert; now and then the reset that follows it wins (grpcurl waits 50 ms for the
		// alert before closing).
		require.Error(t, err)
		assert.Regexp(t, `^grpc dial error: failed to dial grpc: (remote error: tls: certificate required|read tcp [0-9.:]+->[0-9.:]+: read: connection reset by peer)$`, err.Error())
		res, ok := got.(Result)
		require.True(t, ok)
		assert.Equal(t, strings.TrimPrefix(err.Error(), "grpc dial error: "), res.Err)
	})

	t.Run("a certificate without its key, or the reverse, fails before the dial", func(t *testing.T) {
		// #2369 (ADR-0009 Precedence 2): the lone setting used to be dropped silently (D83) and the server refused the
		// handshake; now the step fails naming the missing setting, and no connection is attempted.
		for name, tc := range map[string]struct {
			extra venom.TestStep
			want  string
		}{
			"certificate only": {venom.TestStep{"tls_client_cert": string(pki.ClientCertPEM)}, "tls_client_cert is set without tls_client_key"},
			"key only":         {venom.TestStep{"tls_client_key": string(pki.ClientKeyPEM)}, "tls_client_key is set without tls_client_cert"},
		} {
			t.Run(name, func(t *testing.T) {
				got, err := Executor{}.Run(workdirCtx(workdir), step(tc.extra))
				require.EqualError(t, err, "grpc dial error: "+tc.want)
				assert.Equal(t, Result{Err: tc.want}, got)
			})
		}
	})
}

func TestTransportCredentials(t *testing.T) {
	pki := tlstest.New(t)
	workdir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(workdir, "adir"), 0o700))
	ca := string(pki.CAPEM)

	t.Run("no root CA is plaintext", func(t *testing.T) {
		var warnings []string
		warnf = func(_ context.Context, format string, args ...interface{}) {
			warnings = append(warnings, fmt.Sprintf(format, args...))
		}
		t.Cleanup(func() { warnf = venom.Warning })

		creds, err := Executor{}.transportCredentials(context.Background(), workdir)
		require.NoError(t, err)
		assert.Nil(t, creds)
		assert.Empty(t, warnings)

		// ADR-0009 Precedence 3: a client pair without tls_root_ca is not used, and a warning says so.
		creds, err = Executor{TLSClientCert: "ignored", TLSClientKey: "ignored"}.transportCredentials(context.Background(), workdir)
		require.NoError(t, err)
		assert.Nil(t, creds)
		assert.Equal(t, []string{"grpc: tls_client_cert and tls_client_key are not used without tls_root_ca: the connection is plaintext"}, warnings)

		// Precedence 2 comes first: a lone certificate or key fails the step, with or without tls_root_ca.
		for name, tc := range map[string]struct {
			e    Executor
			want string
		}{
			"certificate only": {Executor{TLSClientCert: "ignored"}, "tls_client_cert is set without tls_client_key"},
			"key only":         {Executor{TLSClientKey: "ignored"}, "tls_client_key is set without tls_client_cert"},
		} {
			t.Run(name, func(t *testing.T) {
				warnings = nil
				creds, err := tc.e.transportCredentials(context.Background(), workdir)
				require.EqualError(t, err, tc.want)
				assert.Nil(t, creds)
				assert.Empty(t, warnings)
			})
		}
	})

	t.Run("a root CA gives TLS credentials", func(t *testing.T) {
		for name, e := range map[string]Executor{
			"server TLS": {TLSRootCA: ca},
			"mutual TLS": {TLSRootCA: ca, TLSClientCert: string(pki.ClientCertPEM), TLSClientKey: string(pki.ClientKeyPEM)},
		} {
			t.Run(name, func(t *testing.T) {
				creds, err := e.transportCredentials(context.Background(), workdir)
				require.NoError(t, err)
				require.NotNil(t, creds)
				assert.Equal(t, "tls", creds.Info().SecurityProtocol)
			})
		}
	})

	tests := []struct {
		name    string
		e       Executor
		wantErr string
	}{
		{name: "a root CA that is not PEM", e: Executor{TLSRootCA: "garbage"}, wantErr: "TLSRootCA: failed to add a certificate to the cert pool"},
		{
			name:    "a root CA path that cannot be read",
			e:       Executor{TLSRootCA: "adir"},
			wantErr: "unable to read TLSRootCA from file " + filepath.Join(workdir, "adir"),
		},
		{
			name:    "a client certificate path that cannot be read",
			e:       Executor{TLSRootCA: ca, TLSClientCert: "adir", TLSClientKey: string(pki.ClientKeyPEM)},
			wantErr: "unable to read TLSClientCert from file " + filepath.Join(workdir, "adir"),
		},
		{
			name:    "a client key path that cannot be read",
			e:       Executor{TLSRootCA: ca, TLSClientCert: string(pki.ClientCertPEM), TLSClientKey: "adir"},
			wantErr: "unable to read TLSClientKey from file " + filepath.Join(workdir, "adir"),
		},
		{
			name:    "a client certificate without its key",
			e:       Executor{TLSRootCA: ca, TLSClientCert: string(pki.ClientCertPEM)},
			wantErr: "tls_client_cert is set without tls_client_key",
		},
		{
			name:    "a client key without its certificate",
			e:       Executor{TLSRootCA: ca, TLSClientKey: string(pki.ClientKeyPEM)},
			wantErr: "tls_client_key is set without tls_client_cert",
		},
		{
			name:    "a client certificate and key that do not match",
			e:       Executor{TLSRootCA: ca, TLSClientCert: string(pki.ClientCertPEM), TLSClientKey: string(pki.ServerKeyPEM)},
			wantErr: "failed to parse x509 mTLS certificate or key: tls: private key does not match public key",
		},
		{
			name:    "a client pair that is not PEM",
			e:       Executor{TLSRootCA: ca, TLSClientCert: "garbage", TLSClientKey: "garbage"},
			wantErr: "failed to parse x509 mTLS certificate or key: tls: failed to find any PEM data in certificate input",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			creds, err := tt.e.transportCredentials(context.Background(), workdir)
			require.EqualError(t, err, tt.wantErr)
			assert.Nil(t, creds)
		})
	}
}

func TestRequestHeaders(t *testing.T) {
	assert.Empty(t, requestHeaders(nil))
	assert.Empty(t, requestHeaders(map[string]string{}))
	// One leading empty entry per header, then the "name: value" form (as its doc comment says).
	assert.Equal(t, []string{"", "X-Tenant: acme"}, requestHeaders(map[string]string{"X-Tenant": "acme"}))

	got := requestHeaders(map[string]string{"a": "1", "b": "2"})
	require.Len(t, got, 4)
	assert.Equal(t, []string{"", ""}, got[:2])
	assert.ElementsMatch(t, []string{"a: 1", "b: 2"}, got[2:])
}

func TestParseOutputJSON(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want interface{}
	}{
		{name: "an array", in: `[1, "a"]`, want: []interface{}{json.Number("1"), "a"}},
		{name: "an object", in: `{"a": {"b": 2}}`, want: map[string]interface{}{"a": map[string]interface{}{"b": json.Number("2")}}},
		{name: "only the first JSON value is read", in: `{"a": 1} trailing`, want: map[string]interface{}{"a": json.Number("1")}},
		// null decodes into the array without error: the result is a typed nil slice, not nil.
		{name: "null", in: `null`, want: []interface{}(nil)},
		{name: "a scalar", in: `42`, want: nil},
		{name: "a string", in: `"s"`, want: nil},
		{name: "not JSON", in: `rpc error: code = NotFound desc = unknown service`, want: nil},
		{name: "empty", in: ``, want: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, parseOutputJSON(tt.in))
		})
	}
}

func TestCustomHandler_OnReceiveTrailers(t *testing.T) {
	withDetail, err := status.New(codes.FailedPrecondition, "not ready").
		WithDetails(&healthpb.HealthCheckResponse{Status: healthpb.HealthCheckResponse_NOT_SERVING})
	require.NoError(t, err)

	tests := []struct {
		name string
		stat *status.Status
		want Result
	}{
		{name: "OK", stat: status.New(codes.OK, ""), want: Result{Code: "0"}},
		{
			name: "an error without details",
			stat: status.New(codes.Unavailable, "down"),
			want: Result{Systemerr: "rpc error: code = Unavailable desc = down", Code: "14"},
		},
		{
			// Details go through encoding/json, not protojson: the enum is its number.
			name: "an error with details",
			stat: withDetail,
			want: Result{
				Systemerr: "rpc error: code = FailedPrecondition desc = not ready",
				Code:      "9",
				Details:   []interface{}{map[string]interface{}{"status": float64(2)}},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var res Result
			h := customHandler{target: &res}
			h.OnReceiveTrailers(tt.stat, nil)
			assert.Equal(t, tt.want, res)
			assert.NoError(t, h.err)
		})
	}
}

func TestCustomHandler_OnReceiveResponse(t *testing.T) {
	errFormat := errors.New("format failed")
	fail := true
	var formatter grpcurl.Formatter = func(m protoiface.MessageV1) (string, error) {
		if fail {
			return "", errFormat
		}
		return grpcurl.NewJSONFormatter(false, nil)(m)
	}
	msg := &healthpb.HealthCheckResponse{Status: healthpb.HealthCheckResponse_SERVING}

	var res Result
	h := customHandler{formatter: formatter, target: &res}

	h.OnReceiveResponse(msg)
	require.ErrorIs(t, h.err, errFormat)
	assert.Empty(t, res.Systemout)

	// The next good message clears the error without writing its output; the one after writes it.
	fail = false
	h.OnReceiveResponse(msg)
	require.NoError(t, h.err)
	assert.Empty(t, res.Systemout)

	h.OnReceiveResponse(msg)
	require.NoError(t, h.err)
	assert.Equal(t, "{\n  \"status\": \"SERVING\"\n}", res.Systemout)

	// The no-op callbacks of grpcurl.InvocationEventHandler leave the result alone.
	h.OnResolveMethod(nil)
	h.OnSendHeaders(nil)
	h.OnReceiveHeaders(nil)
	assert.Equal(t, Result{Systemout: "{\n  \"status\": \"SERVING\"\n}"}, res)
}

func TestExecutorDefaults(t *testing.T) {
	assert.Equal(t, &Executor{}, New())
	assert.Equal(t, Result{}, Executor{}.ZeroValueResult())
	assert.Equal(t, &venom.StepAssertions{Assertions: []venom.Assertion{"result.code ShouldEqual 0"}}, Executor{}.GetDefaultAssertions())
}

func TestDialTimeout_keys(t *testing.T) {
	five, zero := int64(5), int64(0)
	tests := map[string]struct {
		e    Executor
		want time.Duration
	}{
		"none":                {Executor{}, 10 * time.Second},
		"connect_timeout":     {Executor{ConnectTimeoutSnake: &five}, 5 * time.Second},
		"connectTimeout":      {Executor{ConnectTimeout: &five}, 5 * time.Second},
		"the field name wins": {Executor{ConnectTimeout: &five, ConnectTimeoutSnake: &zero}, 5 * time.Second},
		"zero is the default": {Executor{ConnectTimeoutSnake: &zero}, 10 * time.Second},
	}
	for name, tt := range tests {
		assert.Equal(t, tt.want, tt.e.dialTimeout(), name)
	}
}
