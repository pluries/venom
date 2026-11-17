---
translates: plugins
title: Plugins
description: Einen venom-executor als Go-Plugin bauen und laden.
fingerprint: 11ab477beeedc9a3ac11b44c97bd1e2a494af335641b1d51535992e059c9782a
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
## Venom-Executor {#venom-executor}

### Ihren executor schreiben {#write-your-executor}

Legen Sie eine Datei `executor/plugins/hello/hello.go` an:

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

`GetDefaultAssertions` muss den Zeiger `*venom.StepAssertions` zurückgeben: venom ignoriert die Wertform kommentarlos, sodass
Ihre Standard-Assertions nie geprüft würden, und die Zeile `var _ interface{…}` sorgt dafür, dass der Compiler sie ablehnt.

Venom und das Plugin bauen:

```bash
$ make build
$ make plugins
```

**Bauen Sie ein Plugin neu, sobald sich venom oder Go ändert.** Ein Go-Plugin wird nur von einer Binärdatei geladen, die mit
derselben Go-Version gebaut wurde (genau derselben Patch-Version, zum Beispiel 1.27.3) und mit denselben Versionen von venom und
von jedem Modul, das beide gemeinsam nutzen. Ein Plugin, das mit einem anderen Go-Patch oder gegen ein anderes venom gebaut wurde, wird abgelehnt.
Der Schritt schlägt dann fehl mit
`user executor "hello" not found - loaded executors are: [...] (loading it as a plugin failed: unable to load plugin "hello".so: plugin.Open("lib/hello"): <reason>)`,
wobei der Grund der ist, den Go nannte (zum Beispiel `plugin was built with a different version of package ...`). Gibt es gar keine
`lib/hello.so`, schlägt der Schritt mit der kürzeren Meldung ohne die Klammer fehl.
Führen Sie deshalb nach einem Upgrade von venom, einem Upgrade von Go oder dem Anheben einer Abhängigkeit `make build` und `make plugins` erneut aus und liefern Sie die neue
`.so` mit der neuen Binärdatei aus. Das Repository testet dies mit `make run-test-plugin` in `tests/`.

Legen Sie eine Test-Suite-Datei `test.yml` an:

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

Venom ausführen:

```
$ ./venom run test.yml
```

Eröffnen Sie gern einen Pull Request mit Ihren executors.
