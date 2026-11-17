---
translates: testsuites
title: Suites de pruebas
description: Cómo se escriben una suite de pruebas y sus casos de prueba.
fingerprint: 692000e4f0fd4c5e632c7755229552c8524f88d83321a5f7cb777257e8c3018a
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# Suites de pruebas {#testsuites}

Una suite de pruebas es un conjunto de casos de prueba destinados a probar un programa de software para demostrar que presenta un conjunto específico de comportamientos.
Un caso de prueba es una especificación de las entradas, las condiciones de ejecución, el procedimiento de prueba y los resultados esperados que definen una única prueba que se ejecuta para alcanzar un objetivo concreto de prueba de software, como ejercitar una ruta determinada del programa o verificar el cumplimiento de un requisito específico.

En `venom` los casos de prueba (testcases) se ejecutan de forma secuencial dentro de una suite de pruebas (testsuite). Cada caso de prueba es un conjunto ordenado de pasos. Cada paso se basa en un `executor` que habilita un tipo específico de comportamiento.

En `venom` una suite de pruebas se escribe en un único archivo `YAML` que respeta la siguiente estructura:

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

### Formulario multipart HTTP {#http-multipart-form}

`multipart_form` envía el cuerpo de la solicitud como `multipart/form-data`. No se puede combinar con `body` ni con `bodyFile`. Consulte el [executor HTTP](https://github.com/ovh/venom/tree/master/executors/http) para ver la lista completa de campos.

Cada clave es un campo del formulario. Un valor es una única cadena o una lista de cadenas
para enviar el mismo nombre de campo una vez por elemento (por ejemplo, una parte `files` repetida para una carga de varios archivos).

Un valor que empieza por `@` es la ruta de un archivo que se va a cargar, resuelta desde el directorio
de trabajo actual. Añada `;type=<content-type>` para fijar el Content-Type de esa parte;
por defecto es `application/octet-stream`. Esto replica el `-F field=@path;type=...` de curl.

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
