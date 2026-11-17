---
translates: cli-usage
title: Verwendung der CLI
description: Die Befehle von venom und ihre Optionen.
fingerprint: 02ee0f4bbd022eb88342dc24560b249d66c272730ab0c8285af92241332fa21a
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# Verwendung der CLI {#cli-usage}

Die `venom`-CLI besteht aus mehreren Befehlen:

```bash
$ venom -h
Venom - RUN Integration Tests

Usage:
  venom [command]

Available Commands:
  help        Help about any command
  run         Run Tests
  version     Display Version of venom: venom version

Flags:
  -h, --help   help for venom

Use "venom [command] --help" for more information about a command.
```

Die Hilfe zu einem Befehl zeigen Sie mit `venom [command] -h` an:

```bash
$ venom run -h

run integration tests

Usage:
  venom run [flags]

Examples:
  Run all testsuites containing in files ending with *.yml or *.yaml: venom run
  Run a single testsuite: venom run mytestfile.yml
  Run a single testsuite and export the result in JSON format in test/ folder: venom run mytestfile.yml --format=json --output-dir=test
  Run a single testsuite and export the result in XML and HTML formats in test/ folder: venom run mytestfile.yml --format=xml --output-dir=test --html-report
  Run a single testsuite and specify a variable: venom run mytestfile.yml --var="foo=bar"
  Run a single testsuite and load all variables from a file: venom run mytestfile.yml --var-from-file variables.yaml
  Run all testsuites containing in files ending with *.yml or *.yaml with verbosity: VENOM_VERBOSE=2 venom run

  Notice that variables initialized with -var-from-file argument can be overrided with -var argument

  More info: https://github.com/ovh/venom

Flags:
      --format string           --format:json, tap, xml, yaml (default "xml")
  -h, --help                    help for run
      --html-report             Generate HTML Report
      --lib-dir string          Lib Directory: can contain user executors. example:/etc/venom/lib:$HOME/venom.d/lib
      --output-dir string       Output Directory: create tests results file inside this directory
      --parallel int            Number of test files to run at the same time. 1 (default) runs them one after another (default 1)
      --stop-on-failure         Stop running Test Suite on first Test Case failure
      --strict-variables        Fail a step that names an unknown variable
      --var stringArray         --var cds='cds -f config.json' --var cds2='cds -f config.json'
      --var-from-file strings   --var-from-file filename.yaml --var-from-file filename2.yaml: yaml, must contains a dictionary
  -v, --verbose count           verbose. -v (INFO level in venom.log file), -vv to very verbose (DEBUG level) and -vvv to very verbose with CPU Profiling
```

## Testsuites in einer bestimmten Reihenfolge ausführen {#run-test-suites-in-a-specific-order}

- `venom run 01_foo.yml 02_foo.yml` führt 01 vor 02 aus.
- `venom run 02_foo.yml 01_foo.yml` führt 02 vor 01 aus.

Wenn Sie viele Testsuite-Dateien sortieren möchten, können Sie Standardbefehle verwenden, zum Beispiel:

```bash
venom run `find . -type f -name "*.yml"|sort`
```

## Mehrere Testdateien gleichzeitig ausführen {#run-several-test-files-at-the-same-time}

Standardmäßig führt `venom` jeweils eine Testdatei aus. `--parallel N` führt bis zu `N` davon gleichzeitig aus:

```bash
$ venom run --parallel 4
```

Integrationstests verbringen die meiste Zeit damit, auf andere Systeme zu warten, und genau diese Wartezeit
wird dadurch aufgeteilt. Bei acht Dateien, die jeweils warten, wurde gemessen: **3,4× schneller mit `--parallel 4`, 5,6× mit
`--parallel 8`**.

Die Einheit ist die **Datei**: Die Testfälle innerhalb einer Datei und die Schritte innerhalb eines Testfalls laufen weiterhin
nacheinander, weil ein Schritt seine Werte an den nächsten weitergibt. Eine einzelne Datei mit
`--parallel 4` auszuführen ändert daher nichts.

**Wie viel Sie gewinnen, hängt davon ab, ob Ihre Dateien tatsächlich warten.** Etwa 0,35 s jedes Laufs sind
fest — Start, Parsen, Schreiben der Berichte — und keine Worker-Anzahl beseitigt sie. Bei den eigenen
`tests/`-Suites von venom, die kurz sind und meist nicht warten, ist `--parallel 4` nur etwa 1,2× schneller.
Einen echten Gewinn dürfen Sie erwarten, wenn Ihre Dateien auf einen Dienst warten, und einen kleinen, wenn nicht.

Zwei Dinge sollten Sie wissen:

- **Jede Konsolenzeile ist mit der Datei gekennzeichnet, aus der sie stammt**, sodass die Ausgabe von gleichzeitig
  laufenden Dateien lesbar bleibt:

  ```
  [exec.yml]   • exec testsuite (tests/exec.yml)
  [vars.yml]   • vars testsuite (tests/vars.yml)
  [exec.yml]   	• stdin PASS
  [vars.yml]   	• set PASS
  ```

  Zeilen aus verschiedenen Dateien werden durchmischt — sie werden nicht gruppiert —, aber jede nennt ihren Urheber,
  und eine Zeile wird nie zwischen zwei Dateien aufgeteilt. Eine einzelne Datei verfolgen Sie mit `grep`:
  `venom run --parallel 4 | grep '^\[exec.yml'`. Mit `--parallel 1` gibt es überhaupt keine Kennzeichnung, und die
  Ausgabe ist genau so wie immer. Die Logdatei bleibt in beiden Fällen unberührt: Jede Zeile
  darin ist bereits mit ihrer Testdatei gekennzeichnet.
- **`--stop-on-failure` stoppt den gesamten Lauf.** Schlägt eine Testdatei fehl, wird keine weitere Datei gestartet;
  die bereits laufenden Dateien werden zu Ende geführt, sodass jede gestartete Datei weiterhin ein Ergebnis meldet.
  Dateien, die nie gestartet wurden, werden als übersprungen gemeldet. In früheren Versionen hat dieses Flag nur die
  übrigen Testfälle der fehlgeschlagenen Datei übersprungen, und der Lauf wurde mit der nächsten Datei fortgesetzt.

`-vvv` lässt sich nicht mit `--parallel 2` oder mehr kombinieren: Das CPU-Profiling ist prozessweit und kann daher
kein eigenes Profil pro gleichzeitig laufender Datei erzeugen. Der Lauf bricht beim Start ab und nennt den Grund.

## Globstar-Unterstützung {#globstar-support}

Die `venom`-CLI unterstützt Globstar:

```
$ venom run ./foo/b*/**/z*.yml
```

## Variablen {#variables}

Um einzelne Variablen auf der Kommandozeile anzugeben, verwenden Sie die Option `--var`, wenn Sie den Befehl `venom run` ausführen:

```bash
$ venom run --var="foo=bar"
$ venom run --var='foo_list=["biz","buz"]'
$ venom run --var='foo={"biz":"bar","biz":"barr"}'
```

Die Option `--var` kann in einem einzigen Befehl mehrfach verwendet werden.

### Variablendefinitionsdateien {#variable-definitions-files}

Wenn Sie viele Variablen setzen möchten, ist es bequemer, ihre Werte in einer Variablendefinitionsdatei anzugeben. Diese Datei ist ein YAML-Dictionary. Sie geben die Datei auf der Kommandozeile mit `--var-from-file` an:

```bash
venom run --var-from-file variables.yaml
```

### Umgebungsvariablen {#environment-variables}

Als Rückfallebene für die anderen Arten, Variablen zu definieren, durchsucht das Werkzeug `venom` die Umgebung seines eigenen Prozesses nach Umgebungsvariablen mit dem Namen `VENOM_VAR_`, gefolgt vom Namen einer deklarierten Variable.

```bash
$ export VENOM_VAR_foo=bar
$ venom run *.yml
```

Sie können die Umgebungsvariable auch definieren und Ihre Testsuite in einer Zeile ausführen:

```bash
$ VENOM_VAR_foo=bar venom run *.yml
```

## Argumente {#arguments}

Sie können Argumente auf der Kommandozeile über den Namen des Flags definieren.

Die Flags werden in der Ausgabe des Hilfebefehls aufgelistet.

Liste der verfügbaren Flags für den Befehl `venom run`:

```
Flags:
      --format string           --format:json, tap, xml, yaml (default "xml")
  -h, --help                    help for run
      --html-report             Generate HTML Report
      --lib-dir string          Lib Directory: can contain user executors. example:/etc/venom/lib:$HOME/venom.d/lib
      --output-dir string       Output Directory: create tests results file inside this directory
      --parallel int            Number of test files to run at the same time. 1 (default) runs them one after another (default 1)
      --stop-on-failure         Stop running Test Suite on first Test Case failure
      --strict-variables        Fail a step that names an unknown variable
      --var stringArray         --var cds='cds -f config.json' --var cds2='cds -f config.json'
      --var-from-file strings   --var-from-file filename.yaml --var-from-file filename2.yaml: yaml, must contains a dictionary
  -v, --verbose count           verbose. -vv to very verbose and -vvv to very verbose with CPU Profiling
```

### Argumente über Umgebungsvariablen definieren {#define-arguments-with-environment-variables}

Sie können die Argumente auch über Umgebungsvariablen definieren:

```bash
# is the same as
VENOM_FORMAT=json venom run my-test-suite.yml

# is equivalent to
venom run my-test-suite.yml --format=json
```

Flags und ihre Entsprechung bei der Verwendung von Umgebungsvariablen:

- Das Flag `--format="json"` entspricht der Umgebungsvariable `VENOM_FORMAT="json"`
- Das Flag `--lib-dir="/etc/venom/lib:$HOME/venom.d/lib"` entspricht der Umgebungsvariable `VENOM_LIB_DIR="/etc/venom/lib"`
- Das Flag `--output-dir="test-results"` entspricht der Umgebungsvariable `VENOM_OUTPUT_DIR="test-results"`
- Das Flag `--stop-on-failure` entspricht der Umgebungsvariable `VENOM_STOP_ON_FAILURE=true`
- Das Flag `--strict-variables` entspricht der Umgebungsvariable `VENOM_STRICT_VARIABLES=true`
- Das Flag `--var foo=bar` entspricht der Umgebungsvariable `VENOM_VAR_foo='bar'`
- Das Flag `--var-from-file fileA.yml fileB.yml` entspricht der Umgebungsvariable `VENOM_VAR_FROM_FILE="fileA.yml fileB.yml"`
- Das Flag `--parallel 4` entspricht der Umgebungsvariable `VENOM_PARALLEL=4`
- Das Flag `-v` entspricht der Umgebungsvariable `VENOM_VERBOSE=1`
- Das Flag `-vv` entspricht der Umgebungsvariable `VENOM_VERBOSE=2`

Mit der Umgebungsvariable `NO_COLOR=1` lassen sich die Farben in der Ausgabe abschalten.

## Eine Konfigurationsdatei verwenden {#use-a-configuration-file}

Sie können die Einstellungen von Venom in einer Konfigurationsdatei `.venomrc` festlegen. Diese Konfigurationsdatei sollte im aktuellen Verzeichnis oder im `home`-Verzeichnis liegen.

```yml
variables: 
  - foo=bar
variables_files:
  - my_var_file.yaml
stop_on_failure: true
strict_variables: true
format: xml
output_dir: output
lib_dir: lib
verbosity: 3
parallel: 4
```

Beachten Sie, dass die Flags der Kommandozeile die Konfigurationsdatei überschreiben. Die Konfigurationsdatei überschreibt die Umgebungsvariablen.

## Vertrauensmodell {#trust-model}

Venom hat keine Sandbox. Eine Test-Suite ist Code und läuft mit den Rechten des Benutzers, der venom ausführt.

- Test-Suites, benutzerdefinierte Executors in `lib/` und Go-Plugins (`lib/*.so`) laufen alle mit Ihren Rechten. Führen Sie nur eine Suite aus, die Sie auch als Skript ausführen würden.
- Setzen Sie das Ergebnis eines Schritts oder einen anderen Wert, den Sie nicht kontrollieren, niemals ohne Quoting in ein `script:` oder in SQL-Text ein. Ein Schrittergebnis kann beliebigen Inhalt haben, und venom maskiert es weder für eine Shell noch für SQL.
- Führen Sie in der CI keine Suites aus ungeprüften Forks oder Pull Requests aus, und stellen Sie solchen Läufen keine Secrets zur Verfügung.
- Ein in `secrets` oder in `.venomrc` deklariertes Secret wird in Berichten, Logs und der Konsole verborgen. Verborgen werden nur die deklarierten Werte.
- Die Executors `ssh`, `imap` und `smtp` prüfen die Identität des Servers standardmäßig nicht. Venom schreibt in diesem Fall einmal pro Lauf eine Warnung ins Log. Um die Prüfung einzuschalten, setzen Sie `knownhosts` oder `hostkey` bei einem `ssh`-Schritt sowie `verifytls: true` (unter `auth:`, bei `imap`) zusammen mit `withtls: true` bei einem `imap`- oder `smtp`-Schritt.
- Der Executor `web` ignoriert HTTPS-Zertifikatfehler standardmäßig. Setzen Sie die Variable `verifyTls` auf `true`, damit der Browser ein ungültiges Zertifikat ablehnt. Siehe [executors/web](executors/web/README.md).
