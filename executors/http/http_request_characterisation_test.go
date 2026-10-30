package http

import (
	"context"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ovh/venom"
)

// Characterisation tests for getRequest and TLSOptions (#2031, session C). See http_characterisation_test.go.

// varsCtx builds the context an executor receives from GetExecutorRunner: ContextKey("vars") lists the variable names,
// one ContextKey("var.<name>") holds each value (the pattern of TestInterpolation_Of_String), plus the suite workdir.
func varsCtx(workdir string, vars map[string]string) context.Context {
	ctx := httpWorkdirCtx(workdir)
	keys := make([]string, 0, len(vars))
	for k, v := range vars {
		keys = append(keys, k)
		ctx = context.WithValue(ctx, venom.ContextKey("var."+k), v)
	}
	return context.WithValue(ctx, venom.ContextKey("vars"), keys)
}

func writeFixture(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600)) //nolint:gosec // G703: dir is a t.TempDir() and name a literal of the test
	return path
}

func requestBody(t *testing.T, req *http.Request) string {
	t.Helper()
	b, err := io.ReadAll(req.Body)
	require.NoError(t, err)
	require.NoError(t, req.Body.Close())
	return string(b)
}

func TestGetRequestReadsTheBodyFromItsSource(t *testing.T) {
	dir := t.TempDir()
	plain := writeFixture(t, dir, "plain.txt", "plain")
	writeFixture(t, dir, "tpl.json", `{"name":"{{.fullName}}"}`)
	ctx := varsCtx(dir, map[string]string{"fullName": "{{.name}} test", "name": "123"})

	cases := []struct {
		name string
		e    Executor
		want string
	}{
		{"no body source sends an empty body", Executor{}, ""},
		{"body is sent as it is, not interpolated here", Executor{Body: "{{.name}}"}, "{{.name}}"},
		{"body wins over bodyfile", Executor{Body: "inline", BodyFile: plain}, "inline"},
		{"an absolute bodyfile", Executor{BodyFile: plain}, "plain"},
		{"a relative bodyfile is read from the workdir", Executor{BodyFile: "plain.txt"}, "plain"},
		{"a missing bodyfile sends an empty body without error (D77)", Executor{BodyFile: "nope.txt"}, ""},
		{"a bodyfile is interpolated until no variable is left", Executor{BodyFile: "tpl.json"}, `{"name":"123 test"}`},
		{"a preserved bodyfile is sent raw", Executor{BodyFile: "tpl.json", PreserveBodyFile: true}, `{"name":"{{.fullName}}"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.e.URL = "http://h.test"
			req, err := tc.e.getRequest(ctx, dir)
			require.NoError(t, err)
			assert.Equal(t, tc.want, requestBody(t, req))
			assert.Empty(t, req.Header)
		})
	}
}

func TestGetRequestReportsBodyFileInterpolationErrors(t *testing.T) {
	dir := t.TempDir()
	ctx := varsCtx(dir, map[string]string{"z": "1"})
	cases := []struct {
		name    string
		content string
		want    string
	}{
		{"an unresolved variable", `{"a":"{{.missing}}"}`, "unable to interpolate file due to unresolved variables {{.missing}}"},
		{"a known variable next to an unknown one", "{{.z}}{{.missing}}", "unable to interpolate file due to unresolved variables {{.missing}}"},
		{"two unresolved variables on one line are one match (D78)", "{{.a}} and {{.b}}", "unable to interpolate file due to unresolved variables {{.a}} and {{.b}}"},
		{"unresolved variables on two lines are listed", "{{.a}}\n{{.b}}", "unable to interpolate file due to unresolved variables {{.a}},{{.b}}"},
		{"a template error names the url, not the file (D79)", "{{.z}", `unable to interpolate file http://h.test/p: invalid template format "{{.z}": template: input:1: bad character U+007D '}'`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			writeFixture(t, dir, "body.txt", tc.content)
			req, err := Executor{URL: "http://h.test", Path: "/p", BodyFile: "body.txt"}.getRequest(ctx, dir)
			require.EqualError(t, err, tc.want)
			assert.Nil(t, req)
		})
	}
}

type formPart struct {
	filename string
	content  string
}

func readMultipart(t *testing.T, req *http.Request) map[string]formPart {
	t.Helper()
	mediaType, params, err := mime.ParseMediaType(req.Header.Get("Content-Type"))
	require.NoError(t, err)
	require.Equal(t, "multipart/form-data", mediaType)
	parts := map[string]formPart{}
	reader := multipart.NewReader(req.Body, params["boundary"])
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			return parts
		}
		require.NoError(t, err)
		b, err := io.ReadAll(part)
		require.NoError(t, err)
		parts[part.FormName()] = formPart{filename: part.FileName(), content: string(b)}
	}
}

func TestGetRequestBuildsAMultipartForm(t *testing.T) {
	dir := t.TempDir()
	upload := writeFixture(t, dir, "upload.txt", "file content")
	writeFixture(t, dir, "rel.txt", "relative content")
	missing := filepath.Join(dir, "nope.txt")

	e := Executor{URL: "http://h.test", MultipartForm: map[string]interface{}{
		"field":    "value",
		"upload":   "@" + upload,
		"missing":  "@" + missing,
		"relative": "@rel.txt",
	}}
	req, err := e.getRequest(httpWorkdirCtx(dir), dir)
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(req.Header.Get("Content-Type"), "multipart/form-data; boundary="))
	assert.Equal(t, map[string]formPart{
		"field":   {content: "value"},
		"upload":  {filename: "upload.txt", content: "file content"},
		"missing": {content: "@" + missing},
		// An @file is looked up from the process working directory, not the suite workdir (D80).
		"relative": {content: "@rel.txt"},
	}, readMultipart(t, req))
}

func TestGetRequestRejectsConflictingOrMalformedInput(t *testing.T) {
	form := map[string]interface{}{"a": "b"}
	cases := []struct {
		name string
		e    Executor
		want string
	}{
		{"body and multipart_form", Executor{Body: "x", MultipartForm: form}, "can only use one of 'body', 'body_file' and 'multipart_form'"},
		{"bodyfile and multipart_form", Executor{BodyFile: "x", MultipartForm: form}, "can only use one of 'body', 'body_file' and 'multipart_form'"},
		{"a multipart_form that is not a map", Executor{MultipartForm: "a=b"}, "string 'multipart_form' should be a map"},
		{"a map of strings is not the expected map type", Executor{MultipartForm: map[string]string{"a": "b"}}, "map[string]string 'multipart_form' should be a map"},
		{"a multipart_form value that is not a string", Executor{MultipartForm: map[string]interface{}{"a": 1}}, "'multipart_form' values must be a string or a list of strings, got int"},
		{"query parameters on an unparsable url", Executor{URL: "http://[::1", QueryParameters: map[string]string{"a": "b"}}, `unable to parse url http://[::1: parse "http://[::1": missing ']' in host`},
		{"an unparsable url", Executor{URL: "http://[::1"}, `parse "http://[::1": missing ']' in host`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req, err := tc.e.getRequest(context.Background(), "")
			require.EqualError(t, err, tc.want)
			assert.Nil(t, req)
		})
	}
}

func TestGetRequestBuildsTheURLMethodAndAuthentication(t *testing.T) {
	cases := []struct {
		name       string
		e          Executor
		wantMethod string
		wantURL    string
		wantAuth   string
	}{
		{"url and path are concatenated without a separator", Executor{URL: "http://h.test", Path: "p"}, "GET", "http://h.testp", ""},
		{"the method is kept as given", Executor{Method: "delete", URL: "http://h.test/x"}, "delete", "http://h.test/x", ""},
		{"query parameters are sorted and escaped", Executor{URL: "http://h.test/x", QueryParameters: map[string]string{"z": "a&b", "a": "é"}}, "GET", "http://h.test/x?a=%C3%A9&z=a%26b", ""},
		{"query parameters replace the url query (D74)", Executor{URL: "http://h.test/x?old=1#frag", QueryParameters: map[string]string{"a": "1"}}, "GET", "http://h.test/x?a=1#frag", ""},
		{"a user alone sets basic auth", Executor{URL: "http://h.test", BasicAuthUser: "user"}, "GET", "http://h.test", "Basic dXNlcjo="},
		{"a password alone sets basic auth", Executor{URL: "http://h.test", BasicAuthPassword: "synthetic"}, "GET", "http://h.test", "Basic OnN5bnRoZXRpYw=="},
		{"user and password", Executor{URL: "http://h.test", BasicAuthUser: "u", BasicAuthPassword: "synthetic"}, "GET", "http://h.test", "Basic dTpzeW50aGV0aWM="},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req, err := tc.e.getRequest(context.Background(), "")
			require.NoError(t, err)
			assert.Equal(t, tc.wantMethod, req.Method)
			assert.Equal(t, tc.wantURL, req.URL.String())
			assert.Equal(t, tc.wantAuth, req.Header.Get("Authorization"))
		})
	}
}

// tlsKinds applies each option to a fresh transport and names what it configured.
func tlsKinds(opts []func(*http.Transport) error) []string {
	kinds := []string{}
	for _, o := range opts {
		tr := &http.Transport{}
		if err := o(tr); err != nil {
			kinds = append(kinds, "error: "+err.Error())
			continue
		}
		switch {
		case tr.TLSClientConfig == nil:
			kinds = append(kinds, "no tls")
		case tr.TLSClientConfig.InsecureSkipVerify:
			kinds = append(kinds, "insecure")
		case tr.TLSClientConfig.RootCAs != nil:
			kinds = append(kinds, "root-ca")
		case len(tr.TLSClientConfig.Certificates) == 1:
			kinds = append(kinds, "client-cert")
		default:
			kinds = append(kinds, "other")
		}
	}
	return kinds
}

// clientPair writes an openssl-generated key and certificate (synthetic, like the neighbouring TLS tests) into dir
// and returns their PEM contents.
func clientPair(t *testing.T, dir string) (certPEM, keyPEM string) {
	t.Helper()
	keyFile, certFile := generateClientFile(t)
	t.Cleanup(func() {
		os.Remove(keyFile)
		os.Remove(certFile)
	})
	certBytes, err := os.ReadFile(certFile)
	require.NoError(t, err)
	keyBytes, err := os.ReadFile(keyFile)
	require.NoError(t, err)
	writeFixture(t, dir, "client.crt", string(certBytes))
	writeFixture(t, dir, "client.key", string(keyBytes))
	return string(certBytes), string(keyBytes)
}

func TestTLSOptionsBuildsOneOptionPerSetting(t *testing.T) {
	dir := t.TempDir()
	certPEM, keyPEM := clientPair(t, dir)
	caPEM, err := os.ReadFile(digicertRootCA)
	require.NoError(t, err)
	caAbs := writeFixture(t, dir, "ca.crt", string(caPEM))
	certAbs, keyAbs := filepath.Join(dir, "client.crt"), filepath.Join(dir, "client.key")

	cases := []struct {
		name string
		e    Executor
		want []string
	}{
		{"nothing set", Executor{}, []string{}},
		{"ignore_verify_ssl", Executor{IgnoreVerifySSL: true}, []string{"insecure"}},
		{"a root CA file", Executor{TLSRootCA: caAbs}, []string{"root-ca"}},
		{"a root CA relative to the workdir", Executor{TLSRootCA: "ca.crt"}, []string{"root-ca"}},
		{"a root CA as PEM content", Executor{TLSRootCA: string(caPEM)}, []string{"root-ca"}},
		{"a root CA that is not PEM fails only when applied (D81)", Executor{TLSRootCA: "typo.crt"}, []string{"error: WithTLSRootCA: failed to add a certificate to the cert pool"}},
		{"a client pair from files", Executor{TLSClientCert: certAbs, TLSClientKey: keyAbs}, []string{"client-cert"}},
		{"a client pair relative to the workdir", Executor{TLSClientCert: "client.crt", TLSClientKey: "client.key"}, []string{"client-cert"}},
		{"a client pair as PEM content", Executor{TLSClientCert: certPEM, TLSClientKey: keyPEM}, []string{"client-cert"}},
		{"everything, in a fixed order", Executor{IgnoreVerifySSL: true, TLSRootCA: caAbs, TLSClientCert: certAbs, TLSClientKey: keyAbs}, []string{"insecure", "root-ca", "client-cert"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			opts, err := tc.e.TLSOptions(httpWorkdirCtx(dir))
			require.NoError(t, err)
			assert.Equal(t, tc.want, tlsKinds(opts))
		})
	}
}

func TestTLSOptionsReportsUnreadableAndInvalidMaterial(t *testing.T) {
	dir := t.TempDir()
	certPEM, keyPEM := clientPair(t, dir)
	unreadable := t.TempDir() // a directory: os.Stat succeeds, os.ReadFile fails

	cases := []struct {
		name       string
		e          Executor
		wantPrefix string
	}{
		{"an unreadable root CA", Executor{TLSRootCA: unreadable}, "unable to read TLSRootCA from file " + unreadable},
		{"an unreadable client certificate", Executor{TLSClientCert: unreadable, TLSClientKey: keyPEM}, "unable to read TLSClientCert from file " + unreadable},
		{"an unreadable client key", Executor{TLSClientCert: certPEM, TLSClientKey: unreadable}, "unable to read TLSClientKey from file " + unreadable},
		{"certificate and key swapped", Executor{TLSClientCert: keyPEM, TLSClientKey: certPEM}, "failed to parse x509 mTLS certificate or key: tls: "},
		{"a missing certificate path is read as content (D81)", Executor{TLSClientCert: "missing.crt", TLSClientKey: keyPEM}, "failed to parse x509 mTLS certificate or key: tls: failed to find any PEM data in certificate input"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			opts, err := tc.e.TLSOptions(httpWorkdirCtx(dir))
			require.Error(t, err)
			assert.True(t, strings.HasPrefix(err.Error(), tc.wantPrefix), err.Error())
			assert.Nil(t, opts)
		})
	}
}

// A certificate without its key, or a key without its certificate, fails and names the missing setting (#2369,
// ADR-0009 Precedence 2). It used to be dropped silently (D83).
func TestTLSOptionsRefusesALoneCertificateOrKey(t *testing.T) {
	dir := t.TempDir()
	clientPair(t, dir)
	certAbs, keyAbs := filepath.Join(dir, "client.crt"), filepath.Join(dir, "client.key")

	cases := []struct {
		name string
		e    Executor
		want string
	}{
		{"a certificate without its key", Executor{TLSClientCert: certAbs}, "tls_client_cert is set without tls_client_key"},
		{"a key without its certificate", Executor{TLSClientKey: keyAbs}, "tls_client_key is set without tls_client_cert"},
		{"a certificate without its key, verification skipped", Executor{IgnoreVerifySSL: true, TLSClientCert: certAbs}, "tls_client_cert is set without tls_client_key"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			opts, err := tc.e.TLSOptions(httpWorkdirCtx(dir))
			require.EqualError(t, err, tc.want)
			assert.Nil(t, opts)
		})
	}
}
