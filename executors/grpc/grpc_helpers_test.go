package grpc

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/reflection"

	"github.com/ovh/venom"
)

// Test fixtures for the characterisation tests in grpc_test.go: an in-process gRPC server on loopback exposing the
// health service and server reflection. The throw-away PKI is tlstest's, generated at test time (nothing is committed).

const (
	healthService = "grpc.health.v1.Health"
	// servingService is reported SERVING by the fake server's health service.
	servingService = "acme.Serving"
	// zeroStatusService is reported UNKNOWN, the enum's zero value — JSON omits it unless JSONDefaultFields is set.
	zeroStatusService = "acme.ZeroStatus"
	// missingService is never registered with the health service: Check answers NOT_FOUND.
	missingService = "acme.Missing"
)

// fakeServer is a running gRPC server and the incoming metadata its interceptors saw.
type fakeServer struct {
	addr string

	mu       sync.Mutex
	unaryMD  []metadata.MD
	streamMD []metadata.MD
}

func (f *fakeServer) unary(ctx context.Context, req any, _ *grpc.UnaryServerInfo, next grpc.UnaryHandler) (any, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	f.mu.Lock()
	f.unaryMD = append(f.unaryMD, md.Copy())
	f.mu.Unlock()
	return next(ctx, req)
}

func (f *fakeServer) stream(srv any, ss grpc.ServerStream, _ *grpc.StreamServerInfo, next grpc.StreamHandler) error {
	md, _ := metadata.FromIncomingContext(ss.Context())
	f.mu.Lock()
	f.streamMD = append(f.streamMD, md.Copy())
	f.mu.Unlock()
	return next(srv, ss)
}

// seen returns copies of the metadata recorded on unary calls and on streams (the reflection calls).
func (f *fakeServer) seen() (unary, stream []metadata.MD) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]metadata.MD(nil), f.unaryMD...), append([]metadata.MD(nil), f.streamMD...)
}

// startServer serves health and reflection on 127.0.0.1 until the test ends. opts carry the TLS credentials, if any.
func startServer(t *testing.T, opts ...grpc.ServerOption) *fakeServer {
	t.Helper()
	f := &fakeServer{}
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	opts = append(opts, grpc.UnaryInterceptor(f.unary), grpc.StreamInterceptor(f.stream))
	s := grpc.NewServer(opts...)
	hs := health.NewServer()
	hs.SetServingStatus(servingService, healthpb.HealthCheckResponse_SERVING)
	hs.SetServingStatus(zeroStatusService, healthpb.HealthCheckResponse_UNKNOWN)
	healthpb.RegisterHealthServer(s, hs)
	reflection.Register(s)

	served := make(chan struct{})
	go func() {
		defer close(served)
		_ = s.Serve(lis) //nolint:errcheck // Serve only returns once Stop ran at cleanup, or on a listener error the RPCs then report
	}()
	t.Cleanup(func() {
		s.Stop()
		<-served
	})
	f.addr = lis.Addr().String()
	return f
}

// startSilentListener accepts TCP connections and never answers: a gRPC dial to it can only time out.
func startSilentListener(t *testing.T) string {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	var mu sync.Mutex
	var conns []net.Conn
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			c, err := lis.Accept()
			if err != nil {
				return
			}
			mu.Lock()
			conns = append(conns, c)
			mu.Unlock()
		}
	}()
	t.Cleanup(func() {
		lis.Close()
		<-done
		mu.Lock()
		defer mu.Unlock()
		for _, c := range conns {
			c.Close()
		}
	})
	return lis.Addr().String()
}

// writeFile writes content to dir/name and returns the path.
func writeFile(t *testing.T, dir, name string, content []byte) string {
	t.Helper()
	path := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(path, content, 0o600))
	return path
}

// workdirCtx is the context a step gets inside a suite whose workdir is dir.
func workdirCtx(dir string) context.Context {
	return context.WithValue(context.Background(), venom.ContextKey("var.venom.testsuite.workdir"), dir)
}

// grpcRun runs one step and returns its Result; a nil result (decode, marshal or invoke error) is a zero Result.
func grpcRun(ctx context.Context, t *testing.T, step venom.TestStep) (Result, bool, error) {
	t.Helper()
	got, err := Executor{}.Run(ctx, step)
	if got == nil {
		return Result{}, false, err
	}
	res, ok := got.(Result)
	require.Truef(t, ok, "want a Result, got %T: %#v", got, got)
	return res, true, err
}

// healthStep is a Check call on the fake server at addr, extra keys merged in.
func healthStep(addr string, extra venom.TestStep) venom.TestStep {
	step := venom.TestStep{"url": addr, "service": healthService, "method": "Check"}
	for k, v := range extra {
		step[k] = v
	}
	return step
}
