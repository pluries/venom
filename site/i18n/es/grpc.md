---
translates: grpc
title: Grpc
description: Cómo usar el executor Grpc en una suite de pruebas de venom.
fingerprint: bc75454d8a0a9263d05f80a569e9e4373069e488aef05a31e37fabcb31c2c402
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# Venom - Executor Grpc {#venom---executor-grpc}

Paso para ejecutar una petición gRPC

Basado en `grpcurl`, consulte [grpcurl](https://github.com/fullstorydev/grpcurl) para más información.
Este executor depende de la reflexión del servidor gRPC, que debe estar habilitada en el servidor como se describe 
[aquí](https://github.com/grpc/grpc/blob/master/doc/server-reflection.md). 
La reflexión del servidor gRPC no está habilitada por defecto y no está implementada en todas las bibliotecas gRPC,
asegúrese de que la biblioteca que elija admite la reflexión antes de implementar pruebas con este executor.
La reflexión del servidor gRPC tampoco funciona correctamente con `gogo/protobuf`: grpc/grpc-go#1873

## Pruebas {#tests}

Los resultados de las pruebas se interpretan como JSON y se guardan en `systemoutjson`. Los códigos de estado corresponden 
a los códigos de estado oficiales de gRPC.
Puede consultar el significado de cada código de retorno [aquí](https://github.com/grpc/grpc/blob/master/doc/statuscodes.md).

## Entrada {#input}

En su archivo YAML puede usar:

```yaml
  - url mandatory
  - service mandatory: service to call
  - method mandatory: list, describe, or method of the endpoint
  - data optional: data to marshal to JSON and send as a request
  - headers optional: data to send as additional headers
  - connect_timeout optional: The maximum time, in seconds, to wait for connection to be established. Defaults to 10 seconds
  - default_fields optional: whether json formatter should emit default fields
  - include_text_separator optional: when protobuf string formatter is invoked to format multiple messages, all messages after the first one will be prefixed with character (0x1E)
  - tls_client_cert optional: a chain of certificates to identify the caller, first certificate in the chain is considered as the leaf, followed by intermediates. Setting it enables mutual TLS authentication, and the certificate is sent even when ignore_verify_ssl is true. Set the PEM content or the path to the PEM file. It needs tls_client_key: a certificate without its key fails the step. It is not used without tls_root_ca: the connection stays plaintext and a warning is logged.
  - tls_client_key optional: private key corresponding to the certificate. Set the PEM content or the path to the PEM file. It needs tls_client_cert: a key without its certificate fails the step. It is not used without tls_root_ca: the connection stays plaintext and a warning is logged.
  - tls_root_ca optional: root CAs trusted to verify the server certificate, on top of the system ones. Can contain multiple CAs concatenated together. Set the PEM content or the path to the PEM file. The step uses TLS only when this is set; without it the connection is plaintext.
  - ignore_verify_ssl optional: set to true to skip the server certificate check, for example a self-signed certificate that tls_root_ca does not cover
```

Ejemplo:

```yaml

name: Title of TestSuite
testcases:

- name: request GRPC
  steps:
  - type: grpc
    url: serverUrlWithoutHttp:8090
    data:
      foo: bar
    service: coolService.api
    method: GetAllFoos
    assertions:
    - result.code ShouldEqual 0
    - result.systemoutjson.foo ShouldEqual bar
```

Ejemplo TLS:

```yaml

name: Title of TestSuite
testcases:

- name: request GRPC
  steps:
  - type: grpc
    url: serverUrlWithoutHttp:8090
    tls_root_ca: |-
      -----BEGIN CERTIFICATE-----
      ...
      -----END CERTIFICATE-----
    # ignore_verify_ssl: true # only when tls_root_ca and the system CAs do not cover the server certificate
    data:
      foo: bar
    service: coolService.api
    method: GetAllFoos
    assertions:
    - result.code ShouldEqual 0
    - result.systemoutjson.foo ShouldEqual bar
```

Ejemplo de TLS mutuo:

```yaml

name: Title of TestSuite
testcases:

- name: request GRPC
  steps:
  - type: grpc
    url: serverUrlWithoutHttp:8090
    tls_root_ca: |-
      -----BEGIN CERTIFICATE-----
      ...
      -----END CERTIFICATE-----
    tls_client_cert: |-
      -----BEGIN CERTIFICATE-----
      ...
      -----END CERTIFICATE-----
    tls_client_key: |-
      -----BEGIN PRIVATE KEY-----
      ...
      -----END PRIVATE KEY-----
    # ignore_verify_ssl: true # only when tls_root_ca and the system CAs do not cover the server certificate
    data:
      foo: bar
    service: coolService.api
    method: GetAllFoos
    assertions:
    - result.code ShouldEqual 0
    - result.systemoutjson.foo ShouldEqual bar
```

## Salida {#output}

```yaml
executor
systemout
systemerr
err
code
timeseconds
```

- result.timeseconds: duración de la ejecución
- result.executor.executor.script: script ejecutado
- result.err: si existe, este campo contiene el error
- result.systemout: salida estándar del script ejecutado
- result.systemerr: salida de error del script ejecutado
- result.code: código de salida
