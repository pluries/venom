---
translates: testsuites
title: TestSuites
description: Come si scrivono una test suite e i suoi test case.
fingerprint: 692000e4f0fd4c5e632c7755229552c8524f88d83321a5f7cb777257e8c3018a
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# TestSuites {#testsuites}

Una test suite è un insieme di test case pensati per mettere alla prova un programma e mostrare che ha un determinato insieme di comportamenti.
Un test case è una specifica degli input, delle condizioni di esecuzione, della procedura di test e dei risultati attesi che definiscono un singolo test da eseguire per raggiungere un preciso obiettivo di test del software, come esercitare un particolare percorso del programma o verificare la conformità a un requisito specifico.

In `venom` i testcase vengono eseguiti in sequenza all'interno di una testsuite. Ogni testcase è un insieme ordinato di step. Ogni step si basa su un `executor` che abilita un tipo specifico di comportamento.

In `venom` una testsuite si scrive in un unico file `YAML` che rispetta la struttura seguente:

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

### Form multipart HTTP {#http-multipart-form}

`multipart_form` invia il corpo della richiesta come `multipart/form-data`. Non può essere combinato con `body` né con `bodyFile`. Per l'elenco completo dei campi vedi l'[executor HTTP](https://github.com/ovh/venom/tree/master/executors/http).

Ogni chiave è un campo del form. Un valore è una singola stringa, oppure un elenco di stringhe
per inviare lo stesso nome di campo una volta per ogni elemento (ad esempio una parte `files` ripetuta per il caricamento di più file).

Un valore che inizia con `@` è il percorso di un file da caricare, risolto a partire dalla
directory di lavoro corrente. Aggiungi `;type=<content-type>` per impostare il Content-Type di quella parte;
il valore predefinito è `application/octet-stream`. Il comportamento ricalca il `-F field=@path;type=...` di curl.

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
