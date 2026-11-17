---
translates: plugins
title: 插件
description: 如何将 venom executor 构建为 Go 插件并加载。
fingerprint: 11ab477beeedc9a3ac11b44c97bd1e2a494af335641b1d51535992e059c9782a
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
## Venom Executor {#venom-executor}

### 编写您的 executor {#write-your-executor}

创建文件 `executor/plugins/hello/hello.go`

```go
package main

import (
	"C"
	"context"
	"fmt"

	"github.com/mitchellh/mapstructure"

	"github.com/ovh/venom"
)

// Name of the executor
const Name = "hello"

// Plugin var is mandatory, it's used by venom to register the executor
var Plugin = Executor{}

// The engine only takes default assertions in the pointer form and ignores any other silently: refuse it at build time.
var _ interface{ GetDefaultAssertions() *venom.StepAssertions } = Executor{}

// Executor is a venom executor for Hello plugin
type Executor struct {
	Arg string `json:"arg,omitempty" yaml:"arg,omitempty"`
}

// Result represents a step result.
type Result struct {
	Body string `json:"body,omitempty" yaml:"body,omitempty"`
}

// Run implements the venom.Executor interface for Executor.
func (e Executor) Run(ctx context.Context, step venom.TestStep) (interface{}, error) {
	// Transform step to Executor instance.
	if err := mapstructure.Decode(step, &e); err != nil {
		return nil, err
	}

	venom.Debug(ctx, "running plugin Hello with arg %v\n", e.Arg)
	r := Result{Body: fmt.Sprintf("Hello %v", e.Arg)}
	return r, nil
}

// ZeroValueResult return an empty implementation of this executor result
func (e Executor) ZeroValueResult() interface{} {
	return Result{}
}

// GetDefaultAssertions return the default assertions of the executor.
func (e Executor) GetDefaultAssertions() *venom.StepAssertions {
	return &venom.StepAssertions{Assertions: []venom.Assertion{}}
}

```

`GetDefaultAssertions` 必须返回指针 `*venom.StepAssertions`：venom 会悄无声息地忽略值形式，
因此您的默认断言永远不会被检查，而 `var _ interface{…}` 这一行会让编译器拒绝这种写法。

构建 venom 和插件：

```bash
$ make build
$ make plugins
```

**每当 venom 或 Go 发生变化，都要重新构建插件。** Go 插件只能被使用相同 Go 版本（精确到补丁版本，例如 1.27.3）、
并且与插件共享的 venom 和每个模块版本都相同的二进制文件加载。在另一个 Go 补丁版本上构建的插件，或针对另一个 venom 构建的插件，都会被拒绝。此时该步骤会失败，并给出
`user executor "hello" not found - loaded executors are: [...] (loading it as a plugin failed: unable to load plugin "hello".so: plugin.Open("lib/hello"): <reason>)`，
其中 reason 是 Go 给出的原因（例如 `plugin was built with a different version of package ...`）。如果根本没有
`lib/hello.so`，该步骤会以更短的消息失败，不带括号部分。
因此，升级 venom、升级 Go 或更新某个依赖之后，请再次运行 `make build` 和 `make plugins`，并将新的
`.so` 与新的二进制文件一起交付。仓库通过 `make run-test-plugin` 对此进行测试，该命令在 `tests/` 中运行。

创建测试套件文件 `test.yml`：

```yml
name: TestSuite
testcases:
- name: TestAssertions
  steps:
  - type: hello
    arg: world
    assertions:
    - result.body ShouldContainSubstring world
```

运行 venom：

```
$ ./venom run test.yml
```

欢迎您为您的 executor 提交 Pull Request。
