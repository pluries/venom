---
translates: advanced
title: Fortgeschrittene Nutzung
description: Optionen für größere Suites.
fingerprint: a7992da938eb488d1fc5fa9a37e1159081f4a0bf684768a8c2d34b4bb13924ff
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# Fortgeschrittene Nutzung {#advanced-usage}

## Ihre Testsuites debuggen {#debug-your-testsuites}

Bei jedem Befehl `venom run` wird eine Datei *venom.log* erzeugt.

Es gibt zwei Möglichkeiten, eine Testsuite zu debuggen:
 - das Flag `-v` der venom-Binärdatei verwenden.
   - `$ venom run -v test.yml` gibt Details zu jedem Schritt aus.
   - `$ venom run -vv test.yml` erzeugt für jeden Testschritt *dump.json*-Dateien.
   - `$ venom run -vvv test.yml` erzeugt *pprof*-Dateien, ein CPU-Profil und ein Speicherprofil pro
     Testdatei, benannt `pprof_cpu_profile_<file>.prof` und `pprof_mem_profile_<file>.prof` im
     Ausgabeverzeichnis. Öffnen Sie sie mit `go tool pprof`. Das Profiling erfolgt pro Datei, damit Sie sehen, welche
     Suite langsam ist; deshalb lässt sich `-vvv` nicht mit `--parallel 2` oder mehr kombinieren: Der CPU-Profiler
     von Go arbeitet prozessweit.
 - das Schlüsselwort `info` in Ihrem Testschritt verwenden:
Datei `test.yml`:
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

## Testfälle und Testschritte überspringen {#skip-testcase-and-teststeps}

Ein `testcase` lässt sich abhängig von bestimmten `assertions` überspringen. Das folgende Beispiel überspringt zum Beispiel den letzten Testfall.

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

Eine `skip`-Anweisung kann auch auf Schrittebene stehen, um einen Testfall nur teilweise auszuführen. Ist eine Bedingung des skip-Blocks nicht erfüllt, wird der Schritt übersprungen.

Werden alle Schritte eines Testfalls übersprungen, gilt der Testfall selbst ebenfalls als „übersprungen“ und nicht als „bestanden“ oder „fehlgeschlagen“.

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

## Über Daten iterieren {#iterating-over-data}

Mit dem Attribut `range` lässt sich über Daten iterieren.

Die folgenden Datentypen werden unterstützt; jeder stellt die Kontextvariablen `.index`, `.key` und `.value` bereit:

- Ein Array, dessen Werte nacheinander durchlaufen werden (`[]interface{}`)
  - `.index`/`.key`: Index der aktuellen Iteration
  - `.value`: Wert des aktuellen Elements
- Eine Map, deren Schlüssel nacheinander durchlaufen werden (`map[string]interface{}`)
  - `.index`: Index der aktuellen Iteration
  - `.key`: Schlüssel des aktuellen Elements
  - `.value`: Wert des aktuellen Elements
- Eine Ganzzahl, mit der der Zielschritt `n`-mal ausgeführt wird (`int`)
  - `.index`/`.key`/`.value`: Index der aktuellen Iteration
- Eine Template-Zeichenkette, die zu einem der obigen Typen aufgelöst wird (`string`)
  - Sie kann entweder aus einer Variablendatei übernommen oder aus dem Ergebnis eines vorherigen Schritts interpoliert werden

Das folgende Beispiel iteriert zum Beispiel über ein Array aus zwei Elementen, die Maps enthalten:
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

Weitere Beispiele finden Sie in [`tests/ranged.yml`](/tests/ranged.yml).
