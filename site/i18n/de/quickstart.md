---
translates: quickstart
title: Schnellstart
description: Venom installieren, eine Suite schreiben und ausführen.
fingerprint: ff09c7857b2e31620d2ae8d74003a0e186283bb52cf0d3334055d5d020980dc4
drafted_by: claude
drafted_on: 2026-11-24
status: reviewed
reviewed_by: Christian Stephan
reviewed_on: 2026-11-25
reviewed_fingerprint: ff09c7857b2e31620d2ae8d74003a0e186283bb52cf0d3334055d5d020980dc4
---
Diese Seite führt Sie von Null zu einer erfolgreichen Test-Suite. Sie brauchen ein Terminal unter Linux, macOS oder Windows.

## Installieren {#install}

Laden Sie die ausführbare Datei des neuesten Releases für Ihre Plattform herunter. Das ist das Linux-Beispiel. Die Seite [Installation](installing.html) listet alle Plattformen, und die Seite [Ein Release prüfen](verify-release.html) zeigt, wie Sie die Signatur prüfen:

```
curl -L -o venom https://github.com/pluries/venom/releases/latest/download/venom.linux-amd64
chmod +x venom
```

Die Befehle schreiben eine ausführbare `venom`-Datei in den aktuellen Ordner.

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
