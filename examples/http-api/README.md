# Example: testing a small HTTP API with Venom

A tiny HTTP API (`main.go`, two endpoints, no database) and the first suite that the `venom-pluries` skill writes for it
(`tests/venom/first.yml`). Use it to see what a first suite looks like, or to try venom before you write your own.

- `GET /health` answers `200` and `{"status":"ok"}`.
- `GET /items` needs `Authorization: Bearer <token>` and answers a list of two items, `401` without it.

Run it (from this folder):

```bash
go run .                                   # the API, on 127.0.0.1:8080; the token is API_TOKEN, default sample-token
export VENOM_VAR_token=sample-token        # PowerShell: $env:VENOM_VAR_token = "sample-token"
venom run tests/venom/first.yml --var base_url=http://127.0.0.1:8080
```

The suite takes the address and the token as variables, declares the token under `secrets:` so reports hide it, and uses only
read calls. `.venomrc` turns on `strict_variables`, so a typo in a variable name fails the step. The token here is a made-up
value for the example; never write a real credential into a suite.
