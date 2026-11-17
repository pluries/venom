---
translates: advanced
title: Uso avanzato
description: Opzioni per le suite più grandi.
fingerprint: a7992da938eb488d1fc5fa9a37e1159081f4a0bf684768a8c2d34b4bb13924ff
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# Uso avanzato {#advanced-usage}

## Fare il debug delle tue testsuite {#debug-your-testsuites}

Un file *venom.log* viene generato a ogni comando `venom run`.

Ci sono due modi per fare il debug di una testsuite:
 - usa il flag `-v` sul binario venom.
   - `$ venom run -v test.yml` mostra i dettagli di ogni step
   - `$ venom run -vv test.yml` genera dei file *dump.json* per ogni teststep.
   - `$ venom run -vvv test.yml` genera dei file *pprof*, un profilo CPU e uno di memoria per
     ogni file di test, chiamati `pprof_cpu_profile_<file>.prof` e `pprof_mem_profile_<file>.prof` nella
     directory di output. Aprili con `go tool pprof`. Il profiling è per file, così puoi vedere quale
     suite è lenta, ed è per questo che `-vvv` non può essere combinato con `--parallel 2` o più: il profiler CPU
     di Go è a livello di processo.
 - usa la parola chiave `info` nel tuo teststep:
file `test.yml`:
```yml
name: Exec testsuite
testcases:
- name: testA
  steps:
  - type: exec
    script: echo 'foo with a bar here'
    info:
      - this a first info
      - and a second...
- name: cat json
  steps:
  - script: cat exec/testa.json
    info: "the value of result.systemoutjson is {{.result.systemoutjson}}"
    assertions:
    - result.systemoutjson.foo ShouldContainSubstring bar
```

```bash
$ venom run test.yml

# output:

 • Exec testsuite (exec.yml)
  • testA SUCCESS
    [info] this a first info (exec.yml:8)
    [info] and a second... (exec.yml:9)
  • testB SUCCESS
  • sleep 1 SUCCESS
  • cat json SUCCESS
    [info] the value of result.systemoutjson is map[foo:bar] (exec.yml:34)
```

## Saltare testcase e teststep {#skip-testcase-and-teststeps}

È possibile saltare un `testcase` in base ad alcune `assertions`. Ad esempio, l'esempio seguente salta l'ultimo testcase.

```yaml
name: "Skip testsuite"
vars:
  foo: bar

testcases:
- name: init
  steps:
  - type: exec
    script: echo {{.foo}}
    assertions:
    - result.code ShouldEqual 0
    - result.systemout ShouldContainSubstring bar

- name: do-not-skip-this
  skip: 
  - foo ShouldNotBeEmpty
  steps:
  - type: exec
    script: exit 0

- name: skip-this
  skip: 
    - foo ShouldBeEmpty
  steps:
  - type: exec
    script: command_not_found
    assertions:
    - result.code ShouldEqual 0

```

Un'istruzione `skip` può anche essere posta a livello di step per eseguire solo in parte un testcase. Se una condizione del blocco skip non è vera, lo step viene saltato.

Se tutti gli step di un testcase vengono saltati, anche il testcase stesso è considerato "saltato" invece che "superato"/"fallito".

```yaml
name: "Skip testsuite"
vars:
  foo: bar

testcases:
- name: skip-one-of-these
  steps:
  - name: do-not-skip-this
    type: exec
    script: exit 0
    assertions:
    - result.code ShouldEqual 0
    skip:
    - foo ShouldNotBeEmpty
  - name: skip-this
    type: exec
    script: exit 1
    assertions:
    - result.code ShouldEqual 0
    skip:
    - foo ShouldBeEmpty

```

## Iterare sui dati {#iterating-over-data}

È possibile iterare sui dati con l'attributo `range`.

Sono supportati i tipi di dati seguenti, ognuno dei quali espone le variabili contestuali `.index`, `.key` e `.value`:

- Un array in cui si itera su ogni valore (`[]interface{}`)
  - `.index`/`.key`: indice dell'iterazione corrente
  - `.value`: valore dell'elemento dell'iterazione corrente
- Una map in cui si itera su ogni chiave (`map[string]interface{}`)
  - `.index`: indice dell'iterazione corrente
  - `.key`: chiave dell'elemento dell'iterazione corrente
  - `.value`: valore dell'elemento dell'iterazione corrente
- Un intero per eseguire lo step di destinazione `n` volte (`int`)
  - `.index`/`.key`/`.value`: indice dell'iterazione corrente
- Una stringa con template che produce uno dei tipi precedenti (`string`)
  - Può essere ereditata da un file di variabili oppure interpolata dal risultato di uno step precedente

Ad esempio, l'esempio seguente itera su un array di due elementi che contengono delle map:
```yaml
- name: range with hardcoded array
  steps:
  - type: exec
    range:
      - actual: hello
        expected: hello
      - actual: world
        expected: world
    script: echo "{{.value.actual}}"
    assertions:
    - result.code ShouldEqual 0
    - result.systemout ShouldEqual "{{.value.expected}}"
```

Altri esempi sono disponibili in [`tests/ranged.yml`](/tests/ranged.yml).
