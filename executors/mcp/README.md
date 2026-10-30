# Venom - Executor MCP

Step to call an MCP (Model Context Protocol) server: list and call its tools, list and read its resources, list and get its prompts, and assert on the answer.
Use it to test an MCP server from a venom suite, over stdio (venom starts the server) or streamable HTTP.

Since: v1.1.0 (planned). The executor is new in the release after v1.0.0.

Check that it works: `tests/mcp.yml` runs it end to end against a reference server (see "Run the suite"), and
`tests/realserver/filesystem.yml` runs it against a real third-party server (see "Try it against a real server").

## Input

```yaml
  - transport (mandatory): stdio or http
  - command (mandatory with stdio): the server command and its arguments, as a list
  - url (mandatory with http): the endpoint of the server
  - headers (optional, http): map of HTTP headers sent with every request, e.g. Authorization
  - method (mandatory): initialize, ping, tools/list, tools/call, resources/list, resources/read, prompts/list or prompts/get
  - tool (mandatory with tools/call): the name of the tool
  - uri (mandatory with resources/read): the absolute URI of the resource
  - prompt (mandatory with prompts/get): the name of the prompt
  - arguments (optional, tools/call and prompts/get): map of arguments
  - protocol_version (optional), default value: 2025-11-25
```

`tools/call` sends the arguments as JSON. `prompts/get` sends them as strings, as the protocol says: strings are
kept, numbers and booleans are written out (`3` becomes `"3"`, `true` becomes `"true"`), and a nested value (map or
list) is a misuse error before any connection.

A `null` prompt argument (`who:` with no value) is refused as a misuse error, like a nested value.

Default assertion: `result.err ShouldBeEmpty`.

## Output

```yaml
  - result.content: the content blocks of a tool answer (tools/call); the text of each content item of a resource (resources/read), blobs left out
  - result.text: the text blocks of a tool answer, joined with a newline (tools/call); the texts of the resource items, joined (resources/read); the texts of the prompt messages, joined (prompts/get)
  - result.structured: the structured content of a tool answer, as JSON text (tools/call)
  - result.structuredjson: the same, parsed, so you can assert on result.structuredjson.<key>
  - result.tools: the tools, as JSON text (tools/list)
  - result.toolsjson: the same, parsed
  - result.resources: the resources, one entry each with uri, name, mimetype, description (resources/list); the content items, one entry each with uri, mimetype, text, blob (resources/read)
  - result.prompts: the prompts, one entry each with name, description, arguments (prompts/list); the messages, one entry each with role, text (prompts/get)
  - result.description: the description of the prompt (prompts/get)
  - result.serverinfo.name, result.serverinfo.version: what the server announced
  - result.capabilities: the capabilities the server announced
  - result.protocolversion: the protocol version the server answered with
  - result.iserror: true when a tool reported an error (tools/call)
  - result.err: the failure of the step, empty when it worked
  - result.timeseconds: duration of the step
```

### Which keys each method fills

`serverinfo`, `capabilities` and `protocolversion` are filled by every method once the connection is open
(they come from the initialize answer). `iserror`, `err` and `timeseconds` always exist. A key with no value is
absent from the result (an empty text, a missing list). The table shows the keys specific to each method.

| Method | Keys it fills (besides the common ones above) |
|---|---|
| `initialize` | none: the common keys are the whole answer |
| `ping` | none |
| `tools/list` | `tools`, `toolsjson` |
| `tools/call` | `content`, `text`, `structured`, `structuredjson` (when the tool sends structured content), `iserror` |
| `resources/list` | `resources` |
| `resources/read` | `resources`, `content`, `text` |
| `prompts/list` | `prompts` |
| `prompts/get` | `prompts`, `text`, `description` |

If the exchange fails after the connection opened, `err` is set and the keys above may be missing; the common
keys are still there. If the connection itself fails, only `err` and `timeseconds` are set.

Lists come back in the order the server sent them (the Go SDK sorts by URI or name). Assert on an entry with
`result.resources.resources0.uri` or `result.prompts.prompts1.name`.

A blob is returned as a base64 string in `blob`, with `text` empty. There is no size cap.

Contents and messages are returned as the server sent them. Declare in `secrets:` what must stay out of reports:
the executor never runs or forwards that data, and the engine redaction hides declared secrets in reports.

## Limits of version 1

- One connection per step: no session is shared between steps.
- Sampling and elicitation requests from the server are declined.
- Server notifications are ignored.
- Resource templates (`resources/templates/list`) are not supported; they are a possible follow-up.
- No size cap on blobs: a large blob is held in memory and put in the result.

## Errors

- A wrong step (unknown transport, missing `command`, `url`, `tool`, `uri` or `prompt`, unknown method, nested prompt argument) is a Go
  error: the step fails before any connection.
- Everything that goes wrong with the exchange (cannot connect, 401, timeout, JSON-RPC error) is put in
  `result.err`; the default assertion then fails the step with that text, and you can assert on it instead.
- A tool that reports an error (`isError`) sets `result.iserror` to true and `result.err` to
  `tool "<name>" returned an error: <text of the server>`. With the default assertion the step fails; to expect
  the error, assert `result.iserror ShouldBeTrue`.

## Connection, timeout, environment

- One connection per step: the step connects, runs its method and closes. A stdio server is started for the step
  and stopped at its end.
- Timeout: the `timeout` of the step (engine) applies; without one, the executor uses 30 seconds.
- The stdio child inherits the whole environment of venom.
- Protocol: `initialize` is sent with version 2025-11-25 unless `protocol_version` says otherwise, so a suite does
  not change when the SDK moves its default.

## Security note for stdio

`command` runs with the rights of the suite and inherits the whole environment of venom, tokens included. For a
third-party server, start it through a clean environment: `env -i PATH=... HOME=<tempdir> <server>`. The suite
`tests/realserver/filesystem.yml` does this:

```yaml
    command: [env, -i, "PATH={{.prepare.nodebin}}:/usr/bin:/bin", "HOME={{.prepare.tmpdir}}", npx, -y, "@modelcontextprotocol/server-filesystem@2026.1.14", "{{.prepare.tmpdir}}"]
```

## Secrets

Declare a token in the suite `secrets:` and use it in `headers`; venom hides it in logs and reports. The executor
never echoes the headers in its result.

## Examples

```yaml
name: MCP testsuite
testcases:
- name: call a tool over stdio
  steps:
  - type: mcp
    transport: stdio
    command: ["/usr/local/bin/my-mcp-server", "--quiet"]
    method: tools/call
    tool: echo
    arguments:
      message: hello
    assertions:
    - result.err ShouldBeEmpty
    - result.text ShouldContainSubstring hello
    - result.structuredjson.message ShouldEqual hello
```

```yaml
name: MCP testsuite over HTTP
vars:
  token: changeme
secrets:
  - token
testcases:
- name: list the tools
  steps:
  - type: mcp
    transport: http
    url: http://127.0.0.1:8080/mcp
    headers:
      Authorization: Bearer {{.token}}
    method: tools/list
    assertions:
    - result.tools ShouldContainSubstring echo
```

```yaml
name: MCP resources and prompts
testcases:
- name: resources
  steps:
  - type: mcp
    transport: stdio
    command: ["/usr/local/bin/my-mcp-server"]
    method: resources/list
    assertions:
    - result.resources.resources0.uri ShouldEqual venom://text/hello
  - type: mcp
    transport: stdio
    command: ["/usr/local/bin/my-mcp-server"]
    method: resources/read
    uri: venom://text/hello
    assertions:
    - result.text ShouldEqual hello
    - result.content.content0 ShouldEqual hello
- name: prompts
  steps:
  - type: mcp
    transport: stdio
    command: ["/usr/local/bin/my-mcp-server"]
    method: prompts/list
    assertions:
    - result.prompts.prompts0.name ShouldEqual greet
  - type: mcp
    transport: stdio
    command: ["/usr/local/bin/my-mcp-server"]
    method: prompts/get
    prompt: greet
    arguments:
      who: venom
    assertions:
    - result.text ShouldEqual hello venom
    - result.description ShouldEqual A greeting
```

### Keep a value with `vars:`

A result value can be kept for the next test cases. This step, from `tests/realserver/filesystem.yml`, keeps the
server name; later test cases read it as `{{.initialize.servername}}`:

```yaml
- name: initialize
  steps:
  - type: mcp
    transport: stdio
    command: [env, -i, "PATH={{.prepare.nodebin}}:/usr/bin:/bin", "HOME={{.prepare.tmpdir}}", npx, -y, "@modelcontextprotocol/server-filesystem@2026.1.14", "{{.prepare.tmpdir}}"]
    method: initialize
    assertions:
    - result.err ShouldBeEmpty
    - result.serverinfo.name ShouldNotBeEmpty
    vars:
      servername:
        from: result.serverinfo.name
```

## Run the suite

`tests/mcp.yml` builds the reference server (`executors/mcp/internal/refserver`) with `go build` into a fresh
`mktemp -d` directory (binary, address file and pid file), so it needs the Go toolchain, and two runs at once do
not collide. The last test case removes the directory. Run it alone, from the `tests/` directory:
`venom run mcp.yml`. No docker, no network.
The manual fixtures `tests/failing/mcp-iserror.yml` (a tool error) and `tests/failing/mcp-resource-unknown.yml`
(an unknown resource URI) show the failure messages. Each builds its own server in a temporary directory, removes
it, and is expected to fail: `cd tests && venom run failing/mcp-iserror.yml`.

## Try it against a real server

`tests/realserver/filesystem.yml` drives a real third-party server, `@modelcontextprotocol/server-filesystem`, pinned
to version 2026.1.14, over stdio. It sits outside `make run-test`: it needs the network and runs third-party code.
Run it by hand:

```bash
cd tests && venom run realserver/filesystem.yml
```

It needs `node`, `npx` and the npm network (npx downloads the package at run time). The suite works in a temporary
directory with one synthetic file and removes it at the end.

The recorded run passed (5 test cases, 8 steps). The server announced itself as `secure-filesystem-server` 0.2.0,
the protocol was 2025-11-25, `tools/list` returned 14 tools, and the suite used `list_directory` and
`read_text_file`.
