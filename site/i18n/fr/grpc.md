---
translates: grpc
title: Grpc
description: Comment utiliser l'executor Grpc dans une testsuite venom.
fingerprint: bc75454d8a0a9263d05f80a569e9e4373069e488aef05a31e37fabcb31c2c402
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# Venom - Executor Grpc {#venom---executor-grpc}

Étape qui exécute une requête gRPC

Basé sur `grpcurl`, voir [grpcurl](https://github.com/fullstorydev/grpcurl) pour plus d'informations.
Cet executor s'appuie sur la réflexion du serveur gRPC, qui doit être activée sur le serveur comme décrit 
[ici](https://github.com/grpc/grpc/blob/master/doc/server-reflection.md). 
La réflexion du serveur gRPC n'est pas activée par défaut et n'est pas implémentée dans toutes les bibliothèques gRPC :
assurez-vous que la bibliothèque de votre choix prend en charge la réflexion avant d'écrire des tests avec cet executor.
La réflexion du serveur gRPC ne fonctionne pas non plus correctement avec `gogo/protobuf` : grpc/grpc-go#1873

## Tests {#tests}

Les résultats des tests sont analysés comme du JSON et enregistrés dans `systemoutjson`. Les codes de statut correspondent 
aux codes de statut officiels de gRPC.
Vous trouverez la signification de chaque code de retour [ici](https://github.com/grpc/grpc/blob/master/doc/statuscodes.md).

## Entrée {#input}

Dans votre fichier YAML, vous pouvez utiliser :

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

Exemple :

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

Exemple TLS :

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

Exemple TLS mutuel :

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

## Sortie {#output}

```yaml
executor
systemout
systemerr
err
code
timeseconds
```

- result.timeseconds : durée d'exécution
- result.executor.executor.script : script exécuté
- result.err : s'il existe, ce champ contient l'erreur
- result.systemout : sortie standard du script exécuté
- result.systemerr : sortie d'erreur du script exécuté
- result.code : code de sortie
