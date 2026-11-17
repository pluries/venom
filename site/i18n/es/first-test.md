---
translates: first-test
title: Escriba y ejecute su primera suite de pruebas
description: Un recorrido más largo por una primera suite de pruebas.
fingerprint: fda7e71e5e326bc03d72f8c9ef6f8b95feabec80763f205ae7362a10a3a571d1
drafted_by: claude
drafted_on: 2026-11-16
status: reviewed
reviewed_by: Christian Stephan
reviewed_on: 2026-11-16
reviewed_fingerprint: fda7e71e5e326bc03d72f8c9ef6f8b95feabec80763f205ae7362a10a3a571d1
---
# Escriba y ejecute su primera suite de pruebas {#write-and-run-your-first-test-suite}

Para entender cómo funciona Venom, creemos y ejecutemos juntos una primera suite de pruebas.

Las primeras aserciones que haremos en esta suite de pruebas consisten en comprobar si el sitio que queremos probar (una API REST pública, por ejemplo la API de OVHcloud):
- es accesible (responde con un código de estado 200)
- responde en menos de 5 segundos
- devuelve una respuesta válida (en formato JSON)

Primero, cree su suite de pruebas en un archivo llamado, por ejemplo, `testsuite.yml`.
Ábralo en su editor o IDE favorito y complételo con este contenido:

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

Luego, ejecute su suite de pruebas con el siguiente comando:

```bash
$ venom run 

 • APIIntegrationTest (testsuite.yml)
 	• GET-http-testcase-with-5-seconds-timeout SUCCESS
```

Ha escrito y ejecutado su primera suite de pruebas con el executor HTTP. :)
