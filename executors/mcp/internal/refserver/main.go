// Command refserver is a small MCP server that tests/mcp.yml drives through the mcp executor. It is a test
// program, not part of venom: it speaks stdio by default and streamable HTTP with -http.
package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"net"
	"net/http"
	"os"
	"time"
)

func main() {
	log.SetOutput(os.Stderr) // stdout belongs to the protocol in stdio mode
	httpAddr := flag.String("http", "", "serve streamable HTTP on this address (host:port, port 0 allowed); default is stdio")
	addrFile := flag.String("addr-file", "", "write the address the HTTP server listens on to this file")
	idleExit := flag.Duration("idle-exit", 2*time.Minute, "exit after this long, whatever happens (a lifetime cap, not an idle timer), so a failed test run leaves no server behind")
	flag.Parse()

	if err := run(*httpAddr, *addrFile, *idleExit); err != nil {
		log.Fatalf("refserver: %v", err)
	}
}

func run(httpAddr, addrFile string, idleExit time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), idleExit)
	defer cancel()

	if httpAddr == "" {
		return serveStdio(ctx)
	}
	return serveHTTP(ctx, httpAddr, addrFile)
}

func serveStdio(ctx context.Context) error {
	err := newServer().Run(ctx, stdioTransport())
	if errors.Is(err, context.DeadlineExceeded) {
		return nil
	}
	return err
}

func serveHTTP(ctx context.Context, addr, addrFile string) error {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	if addrFile != "" {
		//nolint:gosec // G306: a throwaway address file for the test suite
		if err := os.WriteFile(addrFile, []byte(listener.Addr().String()), 0o644); err != nil {
			return err
		}
	}
	srv := &http.Server{Handler: httpHandler(newServer()), ReadHeaderTimeout: 10 * time.Second}
	go func() {
		<-ctx.Done()
		_ = srv.Close()
	}()
	if err := srv.Serve(listener); !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
