---
translates: plugins
title: Plugins
description: Comment compiler et charger un executor venom sous forme de plugin Go.
fingerprint: 11ab477beeedc9a3ac11b44c97bd1e2a494af335641b1d51535992e059c9782a
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
## Executor venom {#venom-executor}

### Écrire votre executor {#write-your-executor}

Créez un fichier `executor/plugins/hello/hello.go`

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

`GetDefaultAssertions` doit renvoyer le pointeur `*venom.StepAssertions` : venom ignore sans un mot la forme par valeur, donc
vos assertions par défaut ne seraient jamais vérifiées, et la ligne `var _ interface{…}` fait en sorte que le compilateur la refuse.

Compilez venom et le plugin :

```bash
$ make build
$ make plugins
```

**Recompilez un plugin chaque fois que venom ou Go change.** Un plugin Go n'est chargé que par un binaire compilé avec la même
version de Go (le patch exact, par exemple 1.27.3) et les mêmes versions de venom et de chaque module qu'ils partagent. Un plugin
compilé avec un autre patch de Go, ou contre un autre venom, est refusé. L'étape échoue alors avec
`user executor "hello" not found - loaded executors are: [...] (loading it as a plugin failed: unable to load plugin "hello".so: plugin.Open("lib/hello"): <reason>)`,
où la raison est celle que Go a donnée (par exemple `plugin was built with a different version of package ...`). Sans aucun
`lib/hello.so`, l'étape échoue avec le message plus court, sans la parenthèse.
Après une mise à jour de venom, une mise à jour de Go ou un changement de version d'une dépendance, relancez donc `make build` et `make plugins`, et livrez le nouveau
`.so` avec le nouveau binaire. Le dépôt teste cela avec `make run-test-plugin` dans `tests/`.

Créez une testsuite `test.yml` :

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

Lancez venom :

```
$ ./venom run test.yml
```

N'hésitez pas à ouvrir une Pull Request avec vos executors.
