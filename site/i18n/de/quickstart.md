---
translates: quickstart
title: Schnellstart
description: Venom installieren, eine Suite schreiben und ausführen.
fingerprint: 86cf4090a52b3ea8d391e1a37a1038fbbf6b86519dd9a23afd4e7391ceb9b402
drafted_by: claude
drafted_on: 2026-11-16
status: reviewed
reviewed_by: Christian Stephan
reviewed_on: 2026-11-16
reviewed_fingerprint: 86cf4090a52b3ea8d391e1a37a1038fbbf6b86519dd9a23afd4e7391ceb9b402
---
Diese Seite führt Sie von Null zu einer erfolgreichen Test-Suite. Sie brauchen Go und einen Klon des venom-Repositorys.

## Installieren {#install}

Bauen Sie das Kommandozeilenwerkzeug aus einem Klon des Repositorys:

```
go build -o venom ./cmd/venom
```

Der Befehl schreibt eine `venom`-Binärdatei in den aktuellen Ordner. Sobald ein Release existiert, können Sie stattdessen dessen Binärdatei herunterladen, statt sie zu bauen.

## Eine Suite schreiben {#write-a-suite}

Eine Suite ist eine YAML-Datei. Diese hier hat einen Testfall mit einem Schritt. Der Schritt führt einen Shell-Befehl aus, und zwei Assertions prüfen sein Ergebnis:

````yaml
name: Quickstart suite
testcases:
  - name: say hello
    steps:
      - type: exec
        script: echo "hello venom"
        assertions:
          - result.systemout ShouldContainSubstring hello
          - result.code ShouldEqual 0
````

- `type: exec` führt das `script` in einer Shell aus.
- `result.systemout ShouldContainSubstring hello` prüft, dass die Ausgabe das Wort `hello` enthält.
- `result.code ShouldEqual 0` prüft, dass der Befehl ohne Fehler beendet wurde.

## Ausführen {#run-it}

Speichern Sie die Datei als `quickstart.yml` und führen Sie sie aus:

```
./venom run quickstart.yml
```

## Das Ergebnis lesen {#read-the-result}

Venom gibt jeden Testfall mit seinem Status aus. Wenn alle Assertions erfüllt sind, lautet die letzte Zeile:

```
final status: PASS
```

Eine fehlgeschlagene Assertion wird mit dem Schritt, der Assertion und dem von venom gefundenen Wert ausgegeben. Der Exit-Code ist ungleich null, wenn eine Suite fehlschlägt.

Venom ist ein unabhängiger Fork von [ovh/venom](https://github.com/ovh/venom). Lesen Sie als Nächstes den [Überblick](overview.html) oder die Seite über [Test-Suites](testsuites.html).
