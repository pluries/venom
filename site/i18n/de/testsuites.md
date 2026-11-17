---
translates: testsuites
title: Test-Suites
description: Wie eine Test-Suite und ihre Testfälle geschrieben werden.
fingerprint: 692000e4f0fd4c5e632c7755229552c8524f88d83321a5f7cb777257e8c3018a
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# Test-Suites {#testsuites}

Eine Test-Suite ist eine Sammlung von Testfällen, mit denen ein Softwareprogramm geprüft wird, um zu zeigen, dass es ein festgelegtes Verhalten aufweist.
Ein Testfall ist die Beschreibung der Eingaben, der Ausführungsbedingungen, des Testverfahrens und der erwarteten Ergebnisse, die einen einzelnen Test festlegen, der ein bestimmtes Testziel erreichen soll, zum Beispiel einen bestimmten Programmpfad zu durchlaufen oder die Einhaltung einer bestimmten Anforderung zu prüfen.

In `venom` werden die Testcases innerhalb einer Testsuite nacheinander ausgeführt. Jeder Testcase ist eine geordnete Folge von Schritten. Jeder Schritt basiert auf einem `executor`, der ein bestimmtes Verhalten ermöglicht.

In `venom` wird eine Testsuite in einer einzigen `YAML`-Datei geschrieben, die der folgenden Struktur entspricht:

```yaml

name: Title of TestSuite
description: A detailed description of the TestSuite, in markdown.
testcases:
- name: TestCase with default value, exec cmd. Check if exit code != 1
  steps:
  - script: echo 'foo'
    type: exec

- name: Title of First TestCase
  steps:
  - script: echo 'foo'
    assertions:
    - result.code ShouldEqual 0
  - script: echo 'bar'
    assertions:
    - result.systemout ShouldNotContainSubstring foo
    - result.timeseconds ShouldBeLessThan 1

- name: GET http testcase, with 5 seconds timeout
  steps:
  - type: http
    method: GET
    url: https://eu.api.ovh.com/1.0/
    timeout: 5
    assertions:
    - result.body ShouldContainSubstring /dedicated/server
    - result.body ShouldContainSubstring /ipLoadbalancing
    - result.statuscode ShouldEqual 200
    - result.timeseconds ShouldBeLessThan 1

- name: Test with retries and delay in seconds between each try
  steps:
  - type: http
    method: GET
    url: https://eu.api.ovh.com/1.0/
    retry: 3
    retry_if: # (optional, lets you early break unrecoverable errors)
    - result.statuscode ShouldNotEqual 403
    delay: 2
    assertions:
    - result.statuscode ShouldEqual 200

```

### HTTP-Multipart-Formular {#http-multipart-form}

`multipart_form` sendet den Request-Body als `multipart/form-data`. Es kann nicht mit `body` oder `bodyFile` kombiniert werden. Die vollständige Liste der Felder finden Sie im [HTTP executor](https://github.com/ovh/venom/tree/master/executors/http).

Jeder Schlüssel ist ein Formularfeld. Ein Wert ist entweder ein einzelner String oder eine Liste von Strings,
mit der derselbe Feldname einmal pro Eintrag gesendet wird (zum Beispiel ein wiederholter Teil `files` für einen Upload mehrerer Dateien).

Ein Wert, der mit `@` beginnt, ist der Pfad einer hochzuladenden Datei, ausgehend vom aktuellen
Arbeitsverzeichnis. Hängen Sie `;type=<content-type>` an, um den Content-Type dieses Teils festzulegen;
Standard ist `application/octet-stream`. Das entspricht dem Aufruf `-F field=@path;type=...` von curl.

```yaml
- name: upload files
  steps:
  - type: http
    method: POST
    url: https://httpbin.org/post
    multipart_form:
      description: "a plain text field"
      tags:
        - "first"
        - "second"
      files:
        - "@./testdata/report.pdf;type=application/pdf"
        - "@./testdata/notes.txt;type=text/plain"
    assertions:
    - result.statuscode ShouldEqual 200
```
