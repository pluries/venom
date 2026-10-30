## Venom Executor

### Write your executor

Create a file `executor/plugins/hello/hello.go`

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

`GetDefaultAssertions` must return the pointer `*venom.StepAssertions`: venom ignores the value form without a word, so
your default assertions would never be checked, and the `var _ interface{…}` line makes the compiler refuse it.

Build venom and plugin:

```bash
$ make build
$ make plugins
```

**Rebuild a plugin whenever venom or Go changes.** A Go plugin is only loaded by a binary that was built with the same
Go version (the exact patch, for example 1.27.3) and the same versions of venom and of every module they share. A plugin
built on another Go patch, or against another venom, is refused. The step then fails with
`user executor "hello" not found - loaded executors are: [...] (loading it as a plugin failed: unable to load plugin "hello".so: plugin.Open("lib/hello"): <reason>)`,
where the reason is the one Go gave (for example `plugin was built with a different version of package ...`). Without a
`lib/hello.so` at all the step fails with the shorter message, without the parenthesis.
So after you upgrade venom, upgrade Go, or bump a dependency, run `make build` and `make plugins` again and ship the new
`.so` with the new binary. The repository tests this with `make run-test-plugin` in `tests/`.

Create a file `test.yml` testsuite:

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

Run venom:

```
$ ./venom run test.yml
```

Feel free to open a Pull Request with your executors.