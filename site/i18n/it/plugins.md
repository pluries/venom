---
translates: plugins
title: Plugin
description: Come compilare e caricare un executor di venom come plugin Go.
fingerprint: 11ab477beeedc9a3ac11b44c97bd1e2a494af335641b1d51535992e059c9782a
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
## Venom Executor {#venom-executor}

### Scrivi il tuo executor {#write-your-executor}

Crea un file `executor/plugins/hello/hello.go`

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

`GetDefaultAssertions` deve restituire il puntatore `*venom.StepAssertions`: venom ignora la forma per valore senza dire nulla, quindi
le tue assertion predefinite non verrebbero mai controllate, e la riga `var _ interface{…}` fa sì che il compilatore la rifiuti.

Compila venom e il plugin:

```bash
$ make build
$ make plugins
```

**Ricompila un plugin ogni volta che cambiano venom o Go.** Un plugin Go viene caricato solo da un binario compilato con la stessa
versione di Go (la patch esatta, per esempio 1.27.3) e con le stesse versioni di venom e di ogni modulo che condividono. Un plugin
compilato con un'altra patch di Go, o contro un altro venom, viene rifiutato. Lo step fallisce allora con
`user executor "hello" not found - loaded executors are: [...] (loading it as a plugin failed: unable to load plugin "hello".so: plugin.Open("lib/hello"): <reason>)`,
dove il motivo è quello dato da Go (per esempio `plugin was built with a different version of package ...`). Se non esiste
alcun `lib/hello.so`, lo step fallisce con il messaggio più breve, senza la parte tra parentesi.
Quindi, dopo aver aggiornato venom, aggiornato Go o cambiato la versione di una dipendenza, esegui di nuovo `make build` e `make plugins` e distribuisci il nuovo
`.so` insieme al nuovo binario. Il repository lo verifica con `make run-test-plugin` in `tests/`.

Crea una test suite nel file `test.yml`:

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

Esegui venom:

```
$ ./venom run test.yml
```

Sentiti libero di aprire una Pull Request con i tuoi executor.
