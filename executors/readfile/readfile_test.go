package readfile

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ovh/venom"
)

func workdirCtx(dir string) context.Context {
	return context.WithValue(context.Background(), venom.ContextKey("var.venom.testsuite.workdir"), dir)
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func run(t *testing.T, dir string, step venom.TestStep) Result {
	t.Helper()
	venom.InitTestLogger(t)
	res, err := New().Run(workdirCtx(dir), step)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	r, ok := res.(Result)
	if !ok {
		t.Fatalf("unexpected result type %T", res)
	}
	return r
}

func TestRunInvalidInput(t *testing.T) {
	venom.InitTestLogger(t)
	if _, err := New().Run(context.Background(), venom.TestStep{"path": []string{"a"}}); err == nil {
		t.Error("expected a decode error")
	}
	if _, err := New().Run(context.Background(), venom.TestStep{}); err == nil || !strings.Contains(err.Error(), "invalid path") {
		t.Errorf("expected invalid path, got %v", err)
	}
}

func TestRunContent(t *testing.T) {
	tests := []struct {
		name, file, content string
		wantJSON            bool
	}{
		{"text", "a.txt", "hello", false},
		{"empty", "e.txt", "", false},
		{"json", "a.json", `{"k": 12}`, true},
		{"yaml", "a.yaml", "k: v\n", true},
		{"yml", "a.yml", "k: v\n", true},
		{"broken yaml", "b.yaml", "k: [unclosed", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			writeFile(t, dir, tt.file, tt.content)
			r := run(t, dir, venom.TestStep{"path": tt.file})
			checkContent(t, r, tt.file, tt.content, tt.wantJSON)
		})
	}
}

func checkContent(t *testing.T, r Result, file, content string, wantJSON bool) {
	t.Helper()
	if r.Err != "" {
		t.Fatalf("unexpected error %q", r.Err)
	}
	if r.Content != content {
		t.Errorf("content = %q", r.Content)
	}
	if r.Size[file] != int64(len(content)) {
		t.Errorf("size = %v", r.Size)
	}
	if len(r.Md5sum[file]) != 32 || r.Mod[file] == "" || r.ModTime[file] == 0 {
		t.Errorf("metadata missing: %+v", r)
	}
	if _, isMap := r.ContentJSON.(map[string]interface{}); isMap != wantJSON {
		t.Errorf("contentjson = %#v", r.ContentJSON)
	}
	if r.TimeSeconds < 0 {
		t.Error("negative time")
	}
}

func TestRunEmptyMd5(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "e.txt", "")
	r := run(t, dir, venom.TestStep{"path": "e.txt"})
	if r.Md5sum["e.txt"] != "d41d8cd98f00b204e9800998ecf8427e" {
		t.Errorf("md5 = %s", r.Md5sum["e.txt"])
	}
}

func TestRunAbsolutePathAndGlob(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "one.txt", "1")
	writeFile(t, dir, "two.txt", "2")
	r := run(t, dir, venom.TestStep{"path": filepath.Join(dir, "*.txt")})
	if r.Err != "" {
		t.Fatalf("unexpected error %q", r.Err)
	}
	if len(r.Md5sum) != 2 || len(r.Content) != 2 {
		t.Errorf("glob result: %+v", r)
	}
}

func TestRunLargeFile(t *testing.T) {
	dir := t.TempDir()
	big := strings.Repeat("x", 2<<20)
	writeFile(t, dir, "big.txt", big)
	r := run(t, dir, venom.TestStep{"path": "big.txt"})
	if r.Err != "" || r.Size["big.txt"] != int64(len(big)) || len(r.Content) != len(big) {
		t.Errorf("large file: err=%q size=%v", r.Err, r.Size)
	}
}

func TestRunErrors(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "sub"), 0o700); err != nil {
		t.Fatal(err)
	}
	tests := []struct{ name, path, want string }{
		{"missing", "nope.txt", "nope.txt"},
		{"bad pattern", "[", "error reading files"},
		{"directory", "sub", "error while reading file"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := run(t, dir, venom.TestStep{"path": tt.path})
			if !strings.Contains(r.Err, tt.want) {
				t.Errorf("err = %q, want %q", r.Err, tt.want)
			}
		})
	}
}

func TestRunUnreadableFile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "locked.txt", "x")
	p := filepath.Join(dir, "locked.txt")
	if err := os.Chmod(p, 0); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(p)
	if err == nil {
		f.Close()
		t.Skip("file mode not enforced on this platform or user")
	}
	r := run(t, dir, venom.TestStep{"path": "locked.txt"})
	if !strings.Contains(r.Err, "error while opening file") {
		t.Errorf("err = %q", r.Err)
	}
}

func TestRelativeNameError(t *testing.T) {
	e := &Executor{Path: "x"}
	// a relative workdir against an absolute match cannot be made relative
	dir := t.TempDir()
	writeFile(t, dir, "x", "1")
	e.Path = filepath.Join(dir, "x")
	if _, err := e.readfile(context.Background(), "relative"); err == nil {
		t.Error("expected relative path error")
	}
}

func TestDefaults(t *testing.T) {
	a := Executor{}.GetDefaultAssertions()
	if len(a.Assertions) != 1 || a.Assertions[0] != "result.err ShouldBeEmpty" {
		t.Errorf("assertions = %v", a.Assertions)
	}
	z, ok := Executor{}.ZeroValueResult().(Result)
	if !ok || z.Md5sum == nil || z.Size == nil || z.ModTime == nil || z.Mod == nil {
		t.Errorf("zero value = %#v", z)
	}
}
