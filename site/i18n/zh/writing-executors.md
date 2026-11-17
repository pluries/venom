---
translates: writing-executors
title: 编写 executor
description: 如何编写新的 venom executor。
fingerprint: 5657ec73882903ce4f26906bdc21e94ded3d4d9616874ddcc1f4496b8bc206c1
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
## Venom Executor {#venom-executor}

### 编写您的 executor {#write-your-executor}

executor 必须实现这个接口

```go

// Executor executes a testStep.
type Executor interface {
	// Run runs a Test Step
	Run(ctx context.Content, TestStep) (interface{}, error)
}
```

示例

```go


// Name of executor
const Name = "myexecutor"

// New returns a new Executor
func New() venom.Executor {
	return &Executor{}
}

// Executor struct
type Executor struct {
	Command string `json:"command,omitempty" yaml:"command,omitempty"`
}

// Result represents a step result
type Result struct {
	Code        int    `json:"code,omitempty" yaml:"code,omitempty"`
	Command     string `json:"command,omitempty" yaml:"command,omitempty"`
	Systemout   string   `json:"systemout,omitempty" yaml:"systemout,omitempty"` // put in testcase.Systemout by venom if present
	Systemerr   string   `json:"systemerr,omitempty" yaml:"systemerr,omitempty"` // put in testcase.Systemerr by venom if present
}

// GetDefaultAssertions returns the default assertions for this executor
// Optional
func (Executor) GetDefaultAssertions() *venom.StepAssertions {
	return &venom.StepAssertions{Assertions: []venom.Assertion{"result.code ShouldEqual 0"}}
}

// Run executes TestStep
func (Executor)	Run(ctx context.Context, step venom.TestStep) (interface{}, error) {
	// transform step to Executor Instance
	var e Executor
	if err := mapstructure.Decode(step, &e); err != nil {
		return nil, err
	}

	// to something with e.Command here...
	//...

	systemout := "foo"
	outputCode := 0

	// prepare result
	r := Result{
		Code:    outputCode, // return Output Code
		Command: e.Command, // return Command executed
		Systemout: systemout, // return Output string
	}

	return r, nil
}

```

欢迎您为您的 executor 提交 Pull Request。

### TLS {#tls}

有六个 executor 会自行建立 TLS 连接：http、grpc、kafka、imap、smtp 和 ovhapi。它们读取 TLS
设置的方式相同。每个 executor 保留各自的开关和各自的默认值。

| Executor | 何时启用 TLS | 跳过服务器检查 | 根 CA | 客户端证书和密钥 | 默认是否检查服务器证书 |
|---|---|---|---|---|---|
| http | `url` 以 `https://` 开头 | `ignore_verify_ssl: true` | `tls_root_ca` | `tls_client_cert`, `tls_client_key` | 是 |
| grpc | 设置了 `tls_root_ca` | `ignore_verify_ssl: true` | `tls_root_ca` | `tls_client_cert`, `tls_client_key` | 是 |
| kafka | `with_tls: true`（或 `withTLS`） | `insecure_tls: true`（或 `insecureTLS`） | `tls_root_ca` | `tls_client_cert`, `tls_client_key` | 是 |
| imap | `auth.withtls: true` | 不设置 `auth.verifytls`，或设为 `false` | `auth.tls_root_ca` | `auth.tls_client_cert`, `auth.tls_client_key` | 否：请设置 `auth.verifytls: true` |
| smtp | `withtls: true` | 不设置 `verifytls`，或设为 `false` | `tls_root_ca` | `tls_client_cert`, `tls_client_key` | 否：请设置 `verifytls: true` |
| ovhapi | `endpoint` 是 `https://`（具名端点，例如 `ovh-eu`，就是） | 无 | `tlsRootCA` 或 `tls_root_ca`，否则使用变量 `ovh.tlsRootCA` | 无：OVH API 不使用客户端证书 | 是 |

它们共有的规则：

- 根 CA、客户端证书或密钥，要么是 PEM 文本本身，要么是 PEM 文件的路径。相对路径
  从测试套件所在的目录读取。可以把多个 CA 拼接在同一个根 CA 中。
- 根 CA 会在系统根证书之外额外受到信任，因此由私有 CA 签发的服务器无需跳过检查即可通过验证。
- 跳过服务器检查会使根 CA 不再起作用，但客户端证书仍然会被发送。
- 只有客户端证书而没有密钥，或者只有密钥而没有证书，该步骤会失败，并给出指明缺失设置的错误。
  即使 TLS 处于关闭状态，这一点也同样成立。
- 在 grpc、kafka、imap 和 smtp 上，TLS 关闭时给出的根 CA 或客户端证书不会被使用：连接保持明文，venom 会记录一条警告。在 http 和 ovhapi 上由 URL 决定：使用 `http://` URL 时 venom 同样会记录一条警告，其中说明，重定向到 `https://`，或使用 `https://` 代理（步骤的 `proxy`、`ovh.proxy` 或
  `HTTPS_PROXY`/`HTTP_PROXY`）时，它们仍然会被使用（除非设置了 `no_follow_redirect`，否则 http 会跟随重定向）。像 `ovh-eu` 这样的 ovhapi 端点名称是一个 `https://` URL，永远不会产生警告。
- kafka 两个开关的两种写法都能读取：`with_tls` 或 `withTLS`，`insecure_tls` 或 `insecureTLS`。任何
  一种写法都会打开对应的开关。
- ovhapi 会拒绝同时设置了 `tlsRootCA` 和 `tls_root_ca` 的步骤。

不在范围内：TLS 来自连接字符串或驱动的 executor（sql、mongo、redis、amqp、rabbitmq、
mqtt、couchbase），以及始终忽略 HTTPS 证书错误的 web executor（参见其 README）。

`executors/tls_conformance_test.go` 针对同一个临时 CA，让这六个 executor 都运行相同的用例：受信任的
CA、没有签发服务器证书的 CA、带客户端证书的跳过检查，以及没有密钥的证书。
