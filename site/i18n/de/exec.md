---
translates: exec
title: Exec
description: So verwenden Sie den Exec-executor in einer venom-Test-Suite.
fingerprint: fc1f5ab5e0397848ca237c08b3e7fd811ce670d0c6a309363d41c0481b77f07e
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# Venom - Executor Exec {#venom---executor-exec}

Schritt zum Ausführen eines Skripts

Das Skript läuft mit den Rechten des Benutzers, der venom ausführt. Setzen Sie das Ergebnis eines Schritts oder einen nicht vertrauenswürdigen Wert nie ohne Quoting in das Skript ein: siehe Trust model im [Root-README](../../README.md#trust-model).


## Input {#input}

Beispiel

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

Mehrzeiliges Skript:

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

Eingabe:

```yaml
name: Title of TestSuite
testcases:
- name: with stdin
  steps:
  - type: exec
    stdin: Foo
    script: cat
```

Expliziter Befehl (ohne Shell):

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

- result.timeseconds: Dauer der Ausführung
- result.err: falls vorhanden, enthält dieses Feld Fehlerdetails
- result.systemout: Standardausgabe des ausgeführten Skripts
- result.systemoutjson: Standardausgabe des ausgeführten Skripts, als JSON-Objekt geparst
- result.systemerr: Fehlerausgabe des ausgeführten Skripts
- result.systemerrjson: Fehlerausgabe des ausgeführten Skripts, als JSON-Objekt geparst
- result.code: Exit-Code

## Default assertion {#default-assertion}

```yaml
result.code ShouldEqual 0
```
