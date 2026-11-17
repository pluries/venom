---
translates: plugins
title: Plugins
description: Cómo compilar y cargar un executor de venom como plugin de Go.
fingerprint: 11ab477beeedc9a3ac11b44c97bd1e2a494af335641b1d51535992e059c9782a
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
## Executor de Venom {#venom-executor}

### Escriba su executor {#write-your-executor}

Cree un archivo `executor/plugins/hello/hello.go`

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

`GetDefaultAssertions` debe devolver el puntero `*venom.StepAssertions`: venom ignora la forma por valor sin avisar, así que
sus aserciones por defecto nunca se comprobarían, y la línea `var _ interface{…}` hace que el compilador la rechace.

Compile venom y el plugin:

```bash
$ make build
$ make plugins
```

**Recompile un plugin cada vez que cambie venom o Go.** Un plugin de Go solo lo carga un binario compilado con la misma
versión de Go (el parche exacto, por ejemplo 1.27.3) y las mismas versiones de venom y de todos los módulos que comparten. Un plugin
compilado con otro parche de Go, o contra otro venom, se rechaza. El paso falla entonces con
`user executor "hello" not found - loaded executors are: [...] (loading it as a plugin failed: unable to load plugin "hello".so: plugin.Open("lib/hello"): <reason>)`,
donde el motivo es el que indicó Go (por ejemplo `plugin was built with a different version of package ...`). Si no existe ningún
`lib/hello.so`, el paso falla con el mensaje más corto, sin el paréntesis.
Por eso, después de actualizar venom, actualizar Go o cambiar la versión de una dependencia, ejecute de nuevo `make build` y `make plugins` y distribuya el nuevo
`.so` con el nuevo binario. El repositorio lo prueba con `make run-test-plugin` en `tests/`.

Cree un archivo de suite de pruebas `test.yml`:

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

Ejecute venom:

```
$ ./venom run test.yml
```

Puede abrir una Pull Request con sus executors.
