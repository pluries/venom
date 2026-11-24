# Example: testing a small gRPC service with Venom

A tiny gRPC server (`main.go`) that serves the standard health service, `grpc.health.v1.Health`, with server reflection on, and a
first suite for it (`tests/venom/first.yml`). Venom's grpc executor reaches a server through reflection, so the server must have
it enabled.

Run it (from this folder):

```bash
go run .                                   # the service, on 127.0.0.1:50051 (plaintext)
venom run tests/venom/first.yml --var grpc_address=127.0.0.1:50051
```

The first test checks that `Check` with an empty service name answers `SERVING`; the second checks that an unknown service name
answers the gRPC status `NotFound` (code 5).
