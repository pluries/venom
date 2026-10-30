package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/textproto"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/mitchellh/mapstructure"
	"github.com/ovh/venom"
	"github.com/ovh/venom/executors/internal/tlsconfig"
	"github.com/ovh/venom/interpolate"
)

// Name of executor
const Name = "http"

// New returns a new Executor
func New() venom.Executor {
	return &Executor{}
}

// Headers represents header HTTP for Request
type Headers map[string]string

// Executor struct. Json and yaml descriptor are used for json output
type Executor struct {
	Method            string            `json:"method" yaml:"method"`
	URL               string            `json:"url" yaml:"url"`
	Path              string            `json:"path" yaml:"path"`
	QueryParameters   map[string]string `json:"query_parameters" yaml:"query_parameters" mapstructure:"query_parameters"`
	Body              string            `json:"body" yaml:"body"`
	BodyFile          string            `json:"bodyfile" yaml:"bodyfile"`
	PreserveBodyFile  bool              `json:"preserve_bodyfile" yaml:"preserve_bodyfile" mapstructure:"preserve_bodyfile"`
	MultipartForm     interface{}       `json:"multipart_form" yaml:"multipart_form"`
	Headers           Headers           `json:"headers" yaml:"headers"`
	IgnoreVerifySSL   bool              `json:"ignore_verify_ssl" yaml:"ignore_verify_ssl" mapstructure:"ignore_verify_ssl"`
	BasicAuthUser     string            `json:"basic_auth_user" yaml:"basic_auth_user" mapstructure:"basic_auth_user"`
	BasicAuthPassword string            `json:"basic_auth_password" yaml:"basic_auth_password" mapstructure:"basic_auth_password"`
	SkipHeaders       bool              `json:"skip_headers" yaml:"skip_headers" mapstructure:"skip_headers"`
	SkipBody          bool              `json:"skip_body" yaml:"skip_body" mapstructure:"skip_body"`
	Proxy             string            `json:"proxy" yaml:"proxy" mapstructure:"proxy"`
	Resolve           []string          `json:"resolve" yaml:"resolve" mapstructure:"resolve"`
	NoFollowRedirect  bool              `json:"no_follow_redirect" yaml:"no_follow_redirect" mapstructure:"no_follow_redirect"`
	UnixSock          string            `json:"unix_sock" yaml:"unix_sock" mapstructure:"unix_sock"`
	TLSClientCert     string            `json:"tls_client_cert" yaml:"tls_client_cert" mapstructure:"tls_client_cert"`
	TLSClientKey      string            `json:"tls_client_key" yaml:"tls_client_key" mapstructure:"tls_client_key"`
	TLSRootCA         string            `json:"tls_root_ca" yaml:"tls_root_ca" mapstructure:"tls_root_ca"`
}

// Result represents a step result. Json and yaml descriptor are used for json output
type Result struct {
	TimeSeconds float64     `json:"timeseconds,omitempty" yaml:"timeseconds,omitempty"`
	StatusCode  int         `json:"statuscode,omitempty" yaml:"statuscode,omitempty"`
	Request     HTTPRequest `json:"request,omitempty" yaml:"request,omitempty"`
	Body        string      `json:"body,omitempty" yaml:"body,omitempty"`
	BodyJSON    interface{} `json:"bodyjson,omitempty" yaml:"bodyjson,omitempty"`
	Headers     Headers     `json:"headers,omitempty" yaml:"headers,omitempty"`
	Err         string      `json:"err,omitempty" yaml:"err,omitempty"`
	Systemout   string      `json:"systemout,omitempty" yaml:"systemout,omitempty"`
}

type HTTPRequest struct { //nolint:revive // exported name kept for module compatibility (plugins, library users, upstream merges)
	Method   string      `json:"method,omitempty"`
	URL      string      `json:"url,omitempty"`
	Header   http.Header `json:"header,omitempty"`
	Body     string      `json:"body,omitempty"`
	Form     url.Values  `json:"form,omitempty"`
	PostForm url.Values  `json:"post_form,omitempty"`
}

// ZeroValueResult return an empty implementation of this executor result
func (Executor) ZeroValueResult() interface{} {
	return Result{}
}

// GetDefaultAssertions return default assertions for this executor
func (Executor) GetDefaultAssertions() *venom.StepAssertions {
	return &venom.StepAssertions{Assertions: []venom.Assertion{"result.statuscode ShouldEqual 200"}}
}

// Run execute TestStep
func (Executor) Run(ctx context.Context, step venom.TestStep) (interface{}, error) {
	// transform step to Executor Instance
	var e Executor
	if err := mapstructure.Decode(step, &e); err != nil {
		return nil, err
	}

	// dirty: mapstructure doesn't like decoding map[interface{}]interface{}, let's force manually
	e.MultipartForm = step["multipart_form"]

	result := Result{}

	workdir := venom.StringVarFromCtx(ctx, "venom.testsuite.workdir")

	req, err := e.getRequest(ctx, workdir)
	if err != nil {
		return nil, err
	}

	e.setHeaders(req)

	client, err := e.newClient(ctx)
	if err != nil {
		return nil, err
	}

	cReq := req.Clone(ctx)
	result.Request, err = captureRequest(cReq, req)
	if err != nil {
		return nil, err
	}
	if cReq.Body != nil {
		defer cReq.Body.Close()
	}

	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		result.Err = err.Error()
		return result, err
	}
	elapsed := time.Since(start)
	result.TimeSeconds = elapsed.Seconds()

	if resp.Body != nil {
		defer resp.Body.Close()

		if err := e.readResponseBody(resp, &result); err != nil {
			return nil, err
		}
	}

	if !e.SkipHeaders {
		result.Headers = responseHeaders(resp.Header)
	}

	requestContentType := result.Request.Header.Get("Content-Type")
	// if PreserveBodyFile == true, the body is not interpolated.
	// So, no need to keep it in request here (to re-inject it in vars)
	// this will avoid to be interpolated after in vars too.
	if e.PreserveBodyFile || !isContentTypeSupported(requestContentType) {
		result.Request.Body = ""
	}

	result.StatusCode = resp.StatusCode
	return result, nil
}

// setHeaders applies the step headers to the request; a "host" header also sets req.Host.
func (e Executor) setHeaders(req *http.Request) {
	for k, v := range e.Headers {
		req.Header.Set(k, v)
		if strings.ToLower(k) == "host" {
			req.Host = v
		}
	}
}

// newClient builds the HTTP client of the step: its transport, a per-call cookie jar and the redirect policy.
func (e Executor) newClient(ctx context.Context) (*http.Client, error) {
	tr, err := e.newTransport(ctx)
	if err != nil {
		return nil, err
	}

	// cookie jar can be used with redirects in the same call
	// this doesn't support cross steps cookies.
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}

	client := &http.Client{Transport: tr, Jar: jar}
	if e.NoFollowRedirect {
		client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		}
	}
	return client, nil
}

// newTransport builds the transport: proxy from the environment, TLS options, the resolve or unix_sock dialer,
// then the step's explicit proxy.
func (e Executor) newTransport(ctx context.Context) (*http.Transport, error) {
	var opts []func(*http.Transport) error
	opts = append(opts, WithProxyFromEnv())

	tlsOptions, err := e.TLSOptions(ctx)
	if err != nil {
		return nil, err
	}
	opts = append(opts, tlsOptions...)

	tr, err := GetTransport(opts...)
	if err != nil {
		return nil, err
	}

	if len(e.Resolve) > 0 && len(e.UnixSock) > 0 {
		return nil, fmt.Errorf("you can't use resolve and unix_sock attributes in the same time")
	}

	e.setDialer(tr)

	if len(e.Proxy) > 0 {
		proxyURL, err := url.Parse(e.Proxy)
		if err != nil {
			return nil, err
		}
		tr.Proxy = http.ProxyURL(proxyURL)
	}
	return tr, nil
}

// setDialer replaces the transport's dialer when the step asks for resolve tuples or a unix socket.
func (e Executor) setDialer(tr *http.Transport) {
	if len(e.Resolve) > 0 {
		tr.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
			// resolve can contains foo.com:443:127.0.0.1
			for _, r := range e.Resolve {
				tuple := strings.Split(r, ":")
				if len(tuple) != 3 {
					return nil, fmt.Errorf("invalid value for resolve attribute: %v", e.Resolve)
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
	} else if len(e.UnixSock) > 0 {
		tr.DialContext = func(_ context.Context, _, _ string) (net.Conn, error) {
			return net.DialUnix("unix", nil, &net.UnixAddr{
				Name: e.UnixSock,
				Net:  "unix",
			})
		}
	}
}

// captureRequest records what is sent, read from a clone of the request (cReq) so the body can be re-read;
// the URL is the one of the original request. The caller owns the closing of cReq.Body.
func captureRequest(cReq, req *http.Request) (HTTPRequest, error) {
	var r HTTPRequest
	r.Method = cReq.Method
	r.URL = req.URL.String()
	if cReq.Body != nil {
		body, err := cReq.GetBody()
		if err != nil {
			return HTTPRequest{}, err
		}
		btes, err := io.ReadAll(body)
		if err != nil {
			return HTTPRequest{}, err
		}
		r.Body = string(btes)
	}
	r.Header = cReq.Header
	r.Form = cReq.Form
	r.PostForm = cReq.PostForm
	return r, nil
}

// readResponseBody fills body, bodyjson and systemout, unless skip_body is set or the content type is not text.
func (e Executor) readResponseBody(resp *http.Response, result *Result) error {
	if e.SkipBody || !isBodySupported(resp) {
		return nil
	}

	bb, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	result.Body = string(bb)

	if isBodyJSONSupported(resp) {
		var m interface{}
		decoder := json.NewDecoder(strings.NewReader(string(bb)))
		decoder.UseNumber()
		if err := decoder.Decode(&m); err == nil {
			result.BodyJSON = m
		}
	}

	result.Systemout = buildResultInfo(result, resp)
	return nil
}

// responseHeaders flattens the response headers: Set-Cookie values joined with "; ", the others with ", ".
func responseHeaders(header http.Header) Headers {
	headers := make(map[string]string)
	for k, v := range header {
		if strings.ToLower(k) == "set-cookie" {
			headers[k] = strings.Join(v, "; ")
		} else {
			headers[k] = strings.Join(v, ", ")
		}
	}
	return headers
}

// getRequest returns the request correctly set for the current executor
func (e Executor) getRequest(ctx context.Context, workdir string) (*http.Request, error) {
	path := fmt.Sprintf("%s%s", e.URL, e.Path)
	method := e.Method
	if method == "" {
		method = "GET"
	}
	if (e.Body != "" || e.BodyFile != "") && e.MultipartForm != nil {
		return nil, fmt.Errorf("can only use one of 'body', 'body_file' and 'multipart_form'")
	}

	body, multipartContentType, err := e.requestBody(ctx, workdir, path)
	if err != nil {
		return nil, err
	}

	path, err = e.withQueryParameters(path)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(method, path, body)
	if err != nil {
		return nil, err
	}

	if len(e.BasicAuthUser) > 0 || len(e.BasicAuthPassword) > 0 {
		req.SetBasicAuth(e.BasicAuthUser, e.BasicAuthPassword)
	}

	if multipartContentType != "" {
		req.Header.Set("Content-Type", multipartContentType)
	}
	return req, err
}

// requestBody returns the body from its one source (body, bodyfile or multipart_form; empty when none) and, for a
// multipart form only, its Content-Type. path (the URL) names the request in the bodyfile error
// messages.
func (e Executor) requestBody(ctx context.Context, workdir, path string) (*bytes.Buffer, string, error) {
	switch {
	case e.Body != "":
		return bytes.NewBuffer([]byte(e.Body)), "", nil
	case e.BodyFile != "":
		body, err := e.bodyFromFile(ctx, workdir, path)
		return body, "", err
	case e.MultipartForm != nil:
		return e.multipartBody()
	}
	return &bytes.Buffer{}, "", nil
}

// bodyFromFile reads bodyfile, relative to the workdir, and interpolates it unless preserve_bodyfile is set.
// A missing file gives an empty body, not an error.
func (e Executor) bodyFromFile(ctx context.Context, workdir, path string) (*bytes.Buffer, error) {
	bodyfilePath := e.BodyFile
	if !filepath.IsAbs(e.BodyFile) {
		// Only join with the workdir with relative path
		bodyfilePath = filepath.Join(workdir, e.BodyFile)
	}
	if _, err := os.Stat(bodyfilePath); os.IsNotExist(err) {
		return &bytes.Buffer{}, nil
	}
	temp, err := os.ReadFile(bodyfilePath)
	if err != nil {
		return nil, err
	}
	if e.PreserveBodyFile {
		return bytes.NewBuffer(temp), nil
	}
	str, err := interpolateBodyFile(ctx, string(temp), path)
	if err != nil {
		return nil, err
	}
	return bytes.NewBufferString(str), nil
}

// interpolateBodyFile interpolates the file content with the context variables until no "{{." is left, and fails
// once a pass changes nothing after more passes than there are variables.
func interpolateBodyFile(ctx context.Context, str, path string) (string, error) {
	h := venom.AllVarsFromCtx(ctx)
	vars, err := venom.DumpStringPreserveCase(h)
	if err != nil {
		return "", fmt.Errorf("unable to dump context vars for file %s: %v", path, err)
	}
	upperlimit := len(vars)
	counter := 0
	for strings.Contains(str, "{{.") {
		stemp, err := interpolate.Do(str, vars)
		if err != nil {
			return "", fmt.Errorf("unable to interpolate file %s: %v", path, err)
		}
		if strings.Compare(str, stemp) == 0 && counter > upperlimit {
			r := regexp.MustCompile(`{{\..*}}`)
			return "", fmt.Errorf("unable to interpolate file due to unresolved variables %s", strings.Join(r.FindAllString(str, -1), ","))
		}
		str = stemp
		counter++
	}
	return str, nil
}

// multipartBody encodes multipart_form and returns the body with its Content-Type (which carries the boundary).
func (e Executor) multipartBody() (*bytes.Buffer, string, error) {
	form, ok := e.MultipartForm.(map[string]interface{})
	if !ok {
		return nil, "", fmt.Errorf("%T 'multipart_form' should be a map", e.MultipartForm)
	}
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	for key, v := range form {
		values, err := formValues(v)
		if err != nil {
			return nil, "", err
		}
		for _, value := range values {
			if err := writeFormPart(writer, key, value); err != nil {
				return nil, "", err
			}
		}
	}
	if err := writer.Close(); err != nil {
		return nil, "", err
	}
	return body, writer.FormDataContentType(), nil
}

// formValues returns the values of one multipart_form key: a single string, or a list of strings to send the same
// field name several times (e.g. a repeated "files" part for a multi-file upload).
func formValues(v interface{}) ([]string, error) {
	switch t := v.(type) {
	case string:
		return []string{t}, nil
	case []interface{}:
		values := make([]string, 0, len(t))
		for _, item := range t {
			value, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("'multipart_form' list values must be strings, got %T", item)
			}
			values = append(values, value)
		}
		return values, nil
	default:
		return nil, fmt.Errorf("'multipart_form' values must be a string or a list of strings, got %T", v)
	}
}

// writeFormPart writes one form value: "@<path>" of an existing file (looked up from the process cwd) as a file
// part, anything else, including "@<path>" of a missing file, as a plain field. "@<path>;type=<mime>" sets the
// file part's Content-Type (curl's syntax); the default is application/octet-stream.
func writeFormPart(writer *multipart.Writer, key, value string) error {
	// todo: how can we be sure the @ is not the value we wanted to use ?
	if strings.HasPrefix(value, "@") {
		filePath, contentType := splitFileType(value[1:])
		if _, err := os.Stat(filePath); !os.IsNotExist(err) {
			header := textproto.MIMEHeader{}
			header.Set("Content-Disposition", multipart.FileContentDisposition(key, filepath.Base(filePath)))
			header.Set("Content-Type", contentType)
			part, err := writer.CreatePart(header)
			if err != nil {
				return err
			}
			return writeFile(part, filePath)
		}
	}
	return writer.WriteField(key, value)
}

// splitFileType splits a trailing ";type=<mime>" off a file path; without one the type is application/octet-stream.
func splitFileType(s string) (path, contentType string) {
	if idx := strings.LastIndex(s, ";type="); idx != -1 {
		return s[:idx], s[idx+len(";type="):]
	}
	return s, "application/octet-stream"
}

// withQueryParameters replaces the query of path by query_parameters, when there are any.
func (e Executor) withQueryParameters(path string) (string, error) {
	if len(e.QueryParameters) == 0 {
		return path, nil
	}
	baseURL, err := url.Parse(path)
	if err != nil {
		return "", fmt.Errorf("unable to parse url %s: %v", path, err)
	}
	params := url.Values{}
	for k, v := range e.QueryParameters {
		params.Add(k, v)
	}
	baseURL.RawQuery = params.Encode()
	return baseURL.String(), nil
}

// writeFile writes the content of the file to an io.Writer
func writeFile(part io.Writer, filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = io.Copy(part, file)
	return err
}

func parseContentType(contentType string) string {
	parsed, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return contentType
	}
	return parsed
}

// given https://developer.mozilla.org/fr/docs/Web/HTTP/Basics_of_HTTP/MIME_types/Common_types
func isBodySupported(resp *http.Response) bool {
	contentType := resp.Header.Get("Content-Type")
	return isContentTypeSupported(contentType)
}

func isContentTypeSupported(contentType string) bool {
	contentType = parseContentType(contentType)
	switch {
	case strings.HasSuffix(contentType, "+json"):
		return true
	case strings.HasPrefix(contentType, "image/"), strings.HasPrefix(contentType, "audio/"), strings.HasPrefix(contentType, "video/"),
		strings.HasPrefix(contentType, "font/"), strings.HasPrefix(contentType, "application/vnd."):
		return false
	case strings.HasPrefix(contentType, "application/"):
		x := strings.SplitN(contentType, "/", 2)[1]
		switch x {
		case "octet-stream", "x-abiword", "vnd.amazon.ebook", "x-bzip", "x-bzip2", "x-csh", "msword", "epub+zip", "java-archive", "ogg", "pdf",
			"x-rar-compressed", "rtf", "x-sh", "x-shockwave-flash", "x-tar", "zip", "x-7z-compressed":
			return false
		}
	case strings.Contains(contentType, "multipart/form-data"):
		return false
	}
	return true
}

func isBodyJSONSupported(resp *http.Response) bool {
	contentType := parseContentType(resp.Header.Get("Content-Type"))
	return strings.Contains(contentType, "application/json") || strings.HasSuffix(contentType, "+json")
}

// warnf is venom.Warning; a test replaces it.
var warnf = venom.Warning

// plainHTTPScheme is the scheme of a request that opens no TLS connection of its own.
const plainHTTPScheme = "http://"

// warnUnusedTLS warns, once per step, when tls_root_ca, tls_client_cert or tls_client_key is set and the url is http://
// (any case): those settings only serve TLS connections, which an http:// url still opens for a followed redirect to
// https:// or through an https:// proxy (ADR-0009, #2385).
func (e Executor) warnUnusedTLS(ctx context.Context) {
	if e.TLSRootCA == "" && e.TLSClientCert == "" && e.TLSClientKey == "" {
		return
	}
	if len(e.URL) >= len(plainHTTPScheme) && strings.EqualFold(e.URL[:len(plainHTTPScheme)], plainHTTPScheme) {
		warnf(ctx, "http: tls_root_ca, tls_client_cert and tls_client_key apply only to TLS connections: the url is http:// (they still serve a redirect to https:// or an https:// proxy)")
	}
}

func (e Executor) TLSOptions(ctx context.Context) ([]func(*http.Transport) error, error) {
	e.warnUnusedTLS(ctx)

	var opts []func(*http.Transport) error

	if e.IgnoreVerifySSL {
		opts = append(opts, WithTLSInsecureSkipVerify(true))
	}

	workdir := venom.StringVarFromCtx(ctx, "venom.testsuite.workdir")

	if e.TLSRootCA != "" {
		TLSRootCA, err := tlsconfig.ReadPEM(workdir, e.TLSRootCA, "TLSRootCA")
		if err != nil {
			return nil, err
		}
		opts = append(opts, WithTLSRootCA(ctx, TLSRootCA))
	}

	cert, err := tlsconfig.ClientCertificate(workdir, e.TLSClientCert, e.TLSClientKey)
	if err != nil {
		return nil, err
	}
	if cert != nil {
		opts = append(opts, WithTLSClientAuth(*cert))
	}

	return opts, nil
}

func buildResultInfo(result *Result, resp *http.Response) string {
	var body string
	if result.BodyJSON != nil {
		jsonString, err := json.Marshal(result.BodyJSON)
		if err != nil {
			return fmt.Sprintf("Error marshaling JSON: %s", err.Error())
		}
		body = string(jsonString)
	} else {
		body = result.Body
	}

	return fmt.Sprintf(`===== Result Info =====
		Method:     %s
		URL:        %s
		Body:       %s
		======================`,
		resp.Request.Method,
		resp.Request.URL,
		body)
}
