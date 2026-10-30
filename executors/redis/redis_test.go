package redis

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/ovh/venom"
)

// replies are canned RESP answers keyed by upper-case command name.
var replies = map[string]string{
	"PING":   "+PONG\r\n",
	"SET":    "+OK\r\n",
	"GET":    "$5\r\nvalue\r\n",
	"MISS":   "$-1\r\n",
	"INCR":   ":42\r\n",
	"LRANGE": "*3\r\n$1\r\na\r\n$1\r\nb\r\n*1\r\n$1\r\nc\r\n",
	"BOOM":   "-ERR boom\r\n",
}

// readCommand parses one RESP array of bulk strings.
func readCommand(r *bufio.Reader) ([]string, error) {
	line, err := r.ReadString('\n')
	if err != nil {
		return nil, err
	}
	n, err := strconv.Atoi(strings.TrimSpace(line[1:]))
	if err != nil {
		return nil, err
	}
	args := make([]string, 0, n)
	for i := 0; i < n; i++ {
		hdr, err := r.ReadString('\n')
		if err != nil {
			return nil, err
		}
		size, err := strconv.Atoi(strings.TrimSpace(hdr[1:]))
		if err != nil {
			return nil, err
		}
		buf := make([]byte, size+2)
		if _, err := io.ReadFull(r, buf); err != nil {
			return nil, err
		}
		args = append(args, string(buf[:size]))
	}
	return args, nil
}

func serveConn(c net.Conn) {
	defer c.Close()
	r := bufio.NewReader(c)
	for {
		args, err := readCommand(r)
		if err != nil || len(args) == 0 {
			return
		}
		reply, ok := replies[strings.ToUpper(args[0])]
		if !ok {
			reply = "-ERR unknown command\r\n"
		}
		if _, err := c.Write([]byte(reply)); err != nil {
			return
		}
	}
}

// startServer runs an in-process RESP server on loopback and returns its URL.
func startServer(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	var (
		wg    sync.WaitGroup
		mu    sync.Mutex
		conns []net.Conn
	)
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			mu.Lock()
			conns = append(conns, c)
			mu.Unlock()
			wg.Add(1)
			go func() {
				defer wg.Done()
				serveConn(c)
			}()
		}
	}()
	t.Cleanup(func() {
		ln.Close()
		mu.Lock()
		for _, c := range conns {
			c.Close()
		}
		mu.Unlock()
		wg.Wait()
	})
	return "redis://" + ln.Addr().String()
}

func workdirCtx(dir string) context.Context {
	return context.WithValue(context.Background(), venom.ContextKey("var.venom.testsuite.workdir"), dir)
}

func TestRunCommands(t *testing.T) {
	venom.InitTestLogger(t)
	url := startServer(t)
	step := venom.TestStep{
		"dialURL":  url,
		"commands": []string{"PING", "", `SET key "a value"`, "GET key", "MISS", "INCR n", "LRANGE l 0 -1"},
	}
	res, err := New().Run(context.Background(), step)
	if err != nil {
		t.Fatal(err)
	}
	r, ok := res.(Result)
	if !ok || len(r.Commands) != 6 {
		t.Fatalf("result = %#v", res)
	}
	want := []interface{}{"PONG", "OK", "value", "", "", []interface{}{"a", "b", []interface{}{"c"}}}
	for i, w := range want {
		if fmt.Sprint(r.Commands[i].Response) != fmt.Sprint(w) {
			t.Errorf("command %d (%s) response = %#v, want %#v", i, r.Commands[i].Name, r.Commands[i].Response, w)
		}
	}
	if len(r.Commands[1].Args) != 2 || r.Commands[1].Args[1] != "a value" {
		t.Errorf("args = %#v", r.Commands[1].Args)
	}
}

func TestRunDialURLFromContext(t *testing.T) {
	venom.InitTestLogger(t)
	ctx := context.WithValue(context.Background(), venom.ContextKey("var.redis.dialURL"), startServer(t))
	res, err := New().Run(ctx, venom.TestStep{"commands": []string{"PING"}})
	if err != nil {
		t.Fatal(err)
	}
	if r, ok := res.(Result); !ok || len(r.Commands) != 1 {
		t.Errorf("result = %#v", res)
	}
}

func TestRunFromFile(t *testing.T) {
	venom.InitTestLogger(t)
	dir := t.TempDir()
	long := strings.Repeat("z", 100000)
	content := "PING\nSET k " + long + "\n"
	if err := os.WriteFile(filepath.Join(dir, "cmds.txt"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	res, err := New().Run(workdirCtx(dir), venom.TestStep{"dialURL": startServer(t), "path": "cmds.txt"})
	if err != nil {
		t.Fatal(err)
	}
	r, ok := res.(Result)
	if !ok || len(r.Commands) != 2 || fmt.Sprint(r.Commands[1].Args[1]) != long {
		t.Errorf("result = %#v", res)
	}
}

func TestRunErrors(t *testing.T) {
	venom.InitTestLogger(t)
	url := startServer(t)
	tests := []struct {
		name string
		step venom.TestStep
		want string
	}{
		{"bad input", venom.TestStep{"dialURL": []string{"x"}}, ""},
		{"missing url", venom.TestStep{"commands": []string{"PING"}}, "missing dialURL"},
		{"bad scheme", venom.TestStep{"dialURL": "http://127.0.0.1:1"}, "invalid redis URL scheme"},
		{"missing file", venom.TestStep{"dialURL": url, "path": "absent.txt"}, "Failed to load file"},
		{"unbalanced quote", venom.TestStep{"dialURL": url, "commands": []string{`SET "k`}}, ""},
		{"error reply", venom.TestStep{"dialURL": url, "commands": []string{"BOOM"}}, "failed to execute command BOOM"},
		{"unknown command", venom.TestStep{"dialURL": url, "commands": []string{"NOPE x"}}, "failed to execute command NOPE"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := New().Run(workdirCtx(t.TempDir()), tt.step)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("err = %v, want containing %q", err, tt.want)
			}
		})
	}
}

func TestFile2LinesMissing(t *testing.T) {
	if _, err := file2lines(filepath.Join(t.TempDir(), "none")); err == nil {
		t.Error("expected an error")
	}
}

func TestZeroValueResult(t *testing.T) {
	if _, ok := (Executor{}).ZeroValueResult().(Result); !ok {
		t.Error("unexpected zero value type")
	}
}
