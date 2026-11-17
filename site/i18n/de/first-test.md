---
translates: first-test
title: Die erste Test-Suite schreiben und ausführen
description: Eine ausführlichere Anleitung zur ersten Test-Suite.
fingerprint: fda7e71e5e326bc03d72f8c9ef6f8b95feabec80763f205ae7362a10a3a571d1
drafted_by: claude
drafted_on: 2026-11-16
status: reviewed
reviewed_by: Christian Stephan
reviewed_on: 2026-11-16
reviewed_fingerprint: fda7e71e5e326bc03d72f8c9ef6f8b95feabec80763f205ae7362a10a3a571d1
---
# Die erste Test-Suite schreiben und ausführen {#write-and-run-your-first-test-suite}

Um zu verstehen, wie Venom arbeitet, erstellen und führen wir gemeinsam eine erste Testsuite aus.

Mit den ersten Assertions in dieser Testsuite prüfen wir, ob die Website, die wir testen wollen (zum Beispiel eine öffentliche REST-API wie die OVHcloud API):
- erreichbar ist (mit dem Statuscode 200 antwortet)
- in weniger als 5 Sekunden antwortet
- eine gültige Antwort liefert (im JSON-Format)

Erstellen Sie zuerst Ihre Testsuite in einer Datei, zum Beispiel mit dem Namen `testsuite.yml`.
Öffnen Sie sie in Ihrem bevorzugten Editor oder Ihrer IDE und füllen Sie sie mit diesem Inhalt:

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

Führen Sie Ihre Testsuite dann mit dem folgenden Befehl aus:

```bash
$ venom run 

 • APIIntegrationTest (testsuite.yml)
 	• GET-http-testcase-with-5-seconds-timeout SUCCESS
```

Sie haben Ihre erste Testsuite mit dem HTTP-Executor geschrieben und ausgeführt! :)
