---
translates: readfile
title: Read file
description: Den Read-file-executor in einer venom-Test-Suite verwenden.
fingerprint: 8370675caf1915a0b8257d34f10772113674ba996cf21e009ccff6d505434aaa
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# Venom - Executor Read file {#venom---executor-read-file}

Schritt zum Lesen einer Datei

Anwendungsfall: Ihre Anwendung schreibt eine Datei auf die Festplatte. Venom kann prüfen, dass diese Datei erzeugt wird, sie lesen
und ihren Inhalt zurückgeben. Der Inhalt lässt sich in anderen Schritten der Test-Suite verwenden.

Der Pfad kann einen vollständigen Pfad, einen Platzhalter (Wildcard) oder ein Verzeichnis enthalten:

```
- path: /a/full/path/file.txt
- path: afile.txt
- path: *.yml
- path: a_directory/
- path: ./foo/b*/**/z*.txt
```

## Eingabe {#input}

```yaml
name: TestSuite Read File
testcases:
- name: TestCase Read File
  steps:
  - type: readfile
    path: yourfile.txt
    assertions:
    - result.err ShouldBeEmpty
```
```yaml
name: TestSuite Read Yaml File
testcases:
  - name: Read File
    steps:
      - type: readfile
        path: yourfile.yml
        assertions:
          - result.err ShouldBeEmpty
```

## Ausgabe {#output}

```yaml
  result.timeseconds
  result.content
  result.contentjson
  result.size.filename
  result.md5sum.filename
  result.modtime.filename
  result.mod.filename
```

- result.timeseconds: Ausführungsdauer
- result.err: Wenn die Datei nicht existiert, enthält dieses Feld einen Fehler
- result.content: Inhalt der gelesenen Datei
- result.contentjson: Inhalt der gelesenen Datei, wenn sie eine JSON-Datei ist. Auf JSON-Daten können Sie zum Beispiel als result.contentjson.yourkey zugreifen
- result.size.filename: Größe der Datei 'filename'
- result.md5sum.filename: md5 der Datei 'filename'
- result.modtime.filename: Änderungsdatum der Datei 'filename', Beispiel: 1487698253
- result.mod.filename: Rechte an der Datei 'filename', Beispiel: -rw-r--r--

Hinweis: Der Wert von 'filename' entspricht dem Pfad, in dem alle Buchstaben kleingeschrieben und alle '/' durch '_' ersetzt wurden.

## Standard-Assertion {#default-assertion}

```yaml
result.err ShouldBeEmpty
```

## Beispiel {#example}

Datei testa.txt:

```
simple content
multilines
```

Datei testa.json:

```json
{
  "foo": "bar"
}
```

Datei testb.json:

```json
[
  {
    "foo": "bar",
    "foo2": "bar2"
  }
]

```

Venom-Testdatei:

```yaml
name: TestSuite Read File
testcases:
- name: TestCase Read File
  steps:
  - type: readfile
    path: testa.json
    assertions:
      - result.contentjson.foo ShouldEqual bar

  - type: readfile
    path: testb.json
    assertions:
      - result.contentjson.contentjson0.foo2 ShouldEqual bar2

  - type: readfile
    path: testa.txt
    assertions:
      - result.content ShouldContainSubstring multilines
```
