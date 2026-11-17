---
translates: first-test
title: Scrivi ed esegui la tua prima test suite
description: Un percorso più lungo attraverso una prima test suite.
fingerprint: fda7e71e5e326bc03d72f8c9ef6f8b95feabec80763f205ae7362a10a3a571d1
drafted_by: claude
drafted_on: 2026-11-16
status: reviewed
reviewed_by: Christian Stephan
reviewed_on: 2026-11-16
reviewed_fingerprint: fda7e71e5e326bc03d72f8c9ef6f8b95feabec80763f205ae7362a10a3a571d1
---
# Scrivi ed esegui la tua prima test suite {#write-and-run-your-first-test-suite}

Per capire come funziona Venom, creiamo ed eseguiamo insieme una prima testsuite.

Le prime assertion che scriveremo, in questa testsuite, servono a verificare se il sito che vogliamo testare (una REST API pubblica, ad esempio l'API di OVHcloud):
- è raggiungibile (risponde con uno status code 200)
- risponde in meno di 5 secondi
- restituisce una risposta valida (in formato JSON)

Per prima cosa, crea la tua testsuite in un file chiamato ad esempio `testsuite.yml`.
Aprilo nel tuo editor o IDE preferito e riempilo con questo contenuto:

```yaml
name: APIIntegrationTest

vars:
  url: https://eu.api.ovh.com

testcases:
- name: GET http testcase, with 5 seconds timeout
  steps:
  - type: http
    method: GET
    url: {{.url}}/1.0/
    timeout: 5
    assertions:
    - result.statuscode ShouldEqual 200
    - result.timeseconds ShouldBeLessThan 1
    - result.bodyjson ShouldContainKey apis
    - result.body ShouldContainSubstring /dedicated/server
    - result.body ShouldContainSubstring /ipLoadbalancing
```

Poi esegui la tua testsuite con il comando seguente:

```bash
$ venom run 

 • APIIntegrationTest (testsuite.yml)
 	• GET-http-testcase-with-5-seconds-timeout SUCCESS
```

Hai scritto ed eseguito la tua prima testsuite con l'executor HTTP! :)
