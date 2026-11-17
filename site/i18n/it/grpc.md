---
translates: grpc
title: Grpc
description: Come usare l'executor Grpc in una test suite di venom.
fingerprint: bc75454d8a0a9263d05f80a569e9e4373069e488aef05a31e37fabcb31c2c402
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# Venom - Executor Grpc {#venom---executor-grpc}

Step per eseguire una richiesta gRPC

Si basa su `grpcurl`, vedi [grpcurl](https://github.com/fullstorydev/grpcurl) per maggiori informazioni.
Questo executor si appoggia alla server reflection di gRPC, che deve essere abilitata sul server come descritto
[qui](https://github.com/grpc/grpc/blob/master/doc/server-reflection.md).
La server reflection di gRPC non è abilitata per impostazione predefinita e non è implementata da tutte le librerie gRPC:
assicurati che la libreria scelta supporti la reflection prima di scrivere test con questo executor.
La server reflection di gRPC inoltre non funziona correttamente con `gogo/protobuf`: grpc/grpc-go#1873

## Test {#tests}

I risultati dei test vengono interpretati come JSON e salvati in `systemoutjson`. I codici di stato corrispondono
ai codici di stato ufficiali di gRPC.
Il significato dei singoli codici di ritorno è descritto [qui](https://github.com/grpc/grpc/blob/master/doc/statuscodes.md).

## Input {#input}

Nel tuo file YAML puoi usare:

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

Esempio:

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

Esempio TLS:

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

Esempio di TLS reciproco (mutual TLS):

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

## Output {#output}

```yaml
executor
systemout
systemerr
err
code
timeseconds
```

- result.timeseconds: durata dell'esecuzione
- result.executor.executor.script: script eseguito
- result.err: se presente, questo campo contiene l'errore
- result.systemout: standard output dello script eseguito
- result.systemerr: output di errore dello script eseguito
- result.code: codice di uscita
