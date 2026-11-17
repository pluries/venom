---
translates: mcp
title: MCP
description: 如何在 venom 测试套件中使用 MCP executor。
fingerprint: 8d470c5d14201b424e787037fbd21d3ce531fda24704df29703bb787038699e0
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# Venom - Executor MCP {#venom---executor-mcp}

用于调用 MCP（Model Context Protocol）服务器的步骤：列出并调用其工具，列出并读取其资源，列出并获取其提示词，并对返回结果进行断言。
可以用它在 venom 测试套件中测试 MCP 服务器，支持 stdio（由 venom 启动服务器）或可流式传输的 HTTP。

自：v1.1.0（计划中）。该 executor 是 v1.0.0 之后的版本中新增的。

验证它能否正常工作：`tests/mcp.yml` 会针对一个参考服务器端到端地运行它（见“运行测试套件”），`tests/realserver/filesystem.yml` 则针对一个真实的第三方服务器运行它（见“用真实服务器试一试”）。

## 输入 {#input}

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

`tools/call` 以 JSON 形式发送参数。`prompts/get` 按协议要求以字符串形式发送参数：字符串保持原样，数字和布尔值会被写成字符串（`3` 变成 `"3"`，`true` 变成 `"true"`），而嵌套的值（映射或列表）则是使用错误，在建立任何连接之前就会报出。

值为 `null` 的提示词参数（`who:` 后面没有值）与嵌套的值一样，会作为使用错误被拒绝。

默认断言：`result.err ShouldBeEmpty`。

## 输出 {#output}

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

### 每个方法会填充哪些键 {#which-keys-each-method-fills}

连接建立之后，每个方法都会填充 `serverinfo`、`capabilities` 和 `protocolversion`（它们来自 initialize 的应答）。`iserror`、`err` 和 `timeseconds` 始终存在。没有值的键不会出现在结果中（空文本、缺失的列表）。下表列出每个方法特有的键。

| 方法 | 填充的键（除上面的通用键之外） |
|---|---|
| `initialize` | 无：通用键就是全部应答 |
| `ping` | 无 |
| `tools/list` | `tools`, `toolsjson` |
| `tools/call` | `content`, `text`, `structured`, `structuredjson`（当工具发送结构化内容时），`iserror` |
| `resources/list` | `resources` |
| `resources/read` | `resources`, `content`, `text` |
| `prompts/list` | `prompts` |
| `prompts/get` | `prompts`, `text`, `description` |

如果连接打开之后交互失败，`err` 会被设置，上面列出的键可能缺失；通用键仍然存在。如果连接本身失败，则只会设置 `err` 和 `timeseconds`。

列表按服务器发送的顺序返回（Go SDK 会按 URI 或名称排序）。可以用 `result.resources.resources0.uri` 或 `result.prompts.prompts1.name` 对其中的一项进行断言。

blob 以 base64 字符串的形式在 `blob` 中返回，此时 `text` 为空。没有大小上限。

内容和消息按服务器发送的原样返回。请在 `secrets:` 中声明必须排除在报告之外的内容：该 executor 从不运行或转发这些数据，引擎的脱敏功能会在报告中隐藏已声明的机密。

## 版本 1 的限制 {#limits-of-version-1}

- 每个步骤一个连接：步骤之间不共享会话。
- 服务器发来的 sampling 和 elicitation 请求会被拒绝。
- 服务器通知会被忽略。
- 不支持资源模板（`resources/templates/list`），它们可能会在后续版本中提供。
- blob 没有大小上限：较大的 blob 会保存在内存中并放入结果。

## 错误 {#errors}

- 写错的步骤（未知的 transport，缺少 `command`、`url`、`tool`、`uri` 或 `prompt`，未知的方法，嵌套的提示词参数）属于 Go
  错误：该步骤会在建立任何连接之前失败。
- 交互过程中出现的一切问题（无法连接、401、超时、JSON-RPC 错误）都会放入
  `result.err`；默认断言随后会以该文本使步骤失败，您也可以改为对它进行断言。
- 工具报告错误（`isError`）时，会把 `result.iserror` 设为 true，并把 `result.err` 设为
  `tool "<name>" returned an error: <text of the server>`。使用默认断言时步骤会失败；如果预期会出现该错误，
  请断言 `result.iserror ShouldBeTrue`。

## 连接、超时、环境 {#connection-timeout-environment}

- 每个步骤一个连接：步骤会连接、运行其方法，然后关闭连接。stdio 服务器为该步骤启动，并在步骤结束时停止。
- 超时：步骤的 `timeout`（引擎层面）有效；没有设置时，该 executor 使用 30 秒。
- stdio 子进程会继承 venom 的全部环境变量。
- 协议：`initialize` 以 2025-11-25 版本发送，除非 `protocol_version` 另有指定，因此 SDK 更改其默认值时，测试套件不会随之改变。

## stdio 的安全说明 {#security-note-for-stdio}

`command` 以测试套件的权限运行，并继承 venom 的全部环境变量，包括令牌。对于第三方服务器，请通过干净的环境启动它：`env -i PATH=... HOME=<tempdir> <server>`。测试套件
`tests/realserver/filesystem.yml` 就是这样做的：

```yaml
    command: [env, -i, "PATH={{.prepare.nodebin}}:/usr/bin:/bin", "HOME={{.prepare.tmpdir}}", npx, -y, "@modelcontextprotocol/server-filesystem@2026.1.14", "{{.prepare.tmpdir}}"]
```

## 机密 {#secrets}

在测试套件的 `secrets:` 中声明令牌，并在 `headers` 中使用它；venom 会在日志和报告中隐藏它。该 executor 从不在其结果中回显这些请求头。

## 示例 {#examples}

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

### 用 `vars:` 保留一个值 {#keep-a-value-with-vars}

结果中的值可以保留下来，供后续的测试用例使用。下面这个步骤来自 `tests/realserver/filesystem.yml`，它保留了服务器名称；之后的测试用例通过 `{{.initialize.servername}}` 读取它：

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

## 运行测试套件 {#run-the-suite}

`tests/mcp.yml` 会把参考服务器（`executors/mcp/internal/refserver`）通过 `go build` 构建到一个全新的
`mktemp -d` 目录中（二进制文件、地址文件和 pid 文件），因此需要 Go 工具链，同时运行两次也不会
冲突。最后一个测试用例会删除该目录。请在 `tests/` 目录下单独运行它：
`venom run mcp.yml`。无需 docker，无需网络。
手动使用的夹具 `tests/failing/mcp-iserror.yml`（工具错误）和 `tests/failing/mcp-resource-unknown.yml`
（未知的资源 URI）用来展示失败信息。每个夹具都会在临时目录中构建自己的服务器，用完即删，
并且预期会失败：`cd tests && venom run failing/mcp-iserror.yml`。

## 用真实服务器试一试 {#try-it-against-a-real-server}

`tests/realserver/filesystem.yml` 通过 stdio 驱动一个真实的第三方服务器 `@modelcontextprotocol/server-filesystem`，版本固定为
2026.1.14。它不在 `make run-test` 之内：它需要网络，并且会运行第三方代码。
请手动运行：

```bash
cd tests && venom run realserver/filesystem.yml
```

它需要 `node`、`npx` 以及 npm 网络（npx 会在运行时下载该软件包）。该测试套件在一个临时目录中工作，里面只有一个合成文件，结束时会将其删除。

记录下来的那次运行通过了（5 个测试用例，8 个步骤）。服务器自报为 `secure-filesystem-server` 0.2.0，
协议为 2025-11-25，`tools/list` 返回了 14 个工具，测试套件使用了 `list_directory` 和
`read_text_file`。
