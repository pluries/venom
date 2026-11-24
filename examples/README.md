# Examples

Small, self-contained examples for Venom Pluries. Each folder is its own Go module (so it is not part of the venom build) with the
service under test, a first suite, and a README that says how to run it.

| Folder | What it shows |
|---|---|
| `http-api/` | A first suite for an HTTP API: variables, a secret, a read-only check, a check that needs a token |
| `grpc-service/` | A first suite for a gRPC service with reflection |

The data in them is made up. To set venom up in your own project, see the `venom-pluries` skill in `skills/venom-pluries/`.
