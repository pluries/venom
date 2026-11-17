---
translates: exec
title: Exec
description: Come usare l'executor Exec in una test suite di venom.
fingerprint: fc1f5ab5e0397848ca237c08b3e7fd811ce670d0c6a309363d41c0481b77f07e
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# Venom - Executor Exec {#venom---executor-exec}

Step usato per eseguire uno script

Lo script gira con i permessi dell'utente che esegue venom. Non inserirvi il risultato di uno step o un valore non attendibile senza metterlo tra apici: vedi Modello di fiducia nel [README principale](../../README.md#trust-model).


## Input {#input}

Esempio

```yaml

name: Title of TestSuite
testcases:
- name: Check if exit code != 1 and echo command response in less than 1s
  steps:
  - type: exec
    script: echo 'foo'
    assertions:
    - result.code ShouldEqual 0
    - result.timeseconds ShouldBeLessThan 1

```

Script su più righe:

```yaml
name: Title of TestSuite
testcases:
- name: multiline script
  steps:
  - type: exec
    script: |
            echo "Foo" \
            echo "Bar"
```

Input:

```yaml
name: Title of TestSuite
testcases:
- name: with stdin
  steps:
  - type: exec
    stdin: Foo
    script: cat
```

Comando esplicito (senza shell):

```yaml
name: Title of TestSuite
testcases:
- name: explicit command
  steps:
  - type: exec
    stdin: "{\"foo\":\"bar\"}"
    command: ["jq", ".foo"]
```

## Output {#output}

```yaml
systemout
systemoutjson
systemerr
systemerrjson
err
code
timeseconds
```

- result.timeseconds: durata dell'esecuzione
- result.err: se presente, questo campo contiene i dettagli dell'errore
- result.systemout: standard output dello script eseguito
- result.systemoutjson: standard output dello script eseguito, interpretato come oggetto JSON
- result.systemerr: output di errore dello script eseguito
- result.systemerrjson: output di errore dello script eseguito, interpretato come oggetto JSON
- result.code: codice di uscita

## Assertion predefinita {#default-assertion}

```yaml
result.code ShouldEqual 0
```
