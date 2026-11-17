---
translates: http
title: HTTP
description: Come usare l'executor HTTP in una test suite di venom.
fingerprint: 859e45938b22aae5cb32f1766961507301c17073180a5535014e3adc8f77cecb
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# Venom - Executor HTTP {#venom---executor-http}

Step per eseguire una richiesta HTTP

## Input {#input}
Nel tuo file YAML puoi usare:

```yaml
  - method (optional), default value: GET
  - url mandatory
  - unix_sock (optional)
  - path (optional)
  - query_parameters (optional)
  - body (optional)
  - bodyFile (optional)
  - multipart_form (optional): cannot be combined with body or bodyFile
  - preserve_bodyfile (optional) skip file content interpolation
  - headers (optional)
  - proxy (optional): set to use a proxy server for connection to url
  - resolve (optional): array of custom resolver of host and port pair. example: foo.com:443:127.0.0.1
  - ignore_verify_ssl (optional): set to true if you use a self-signed SSL on remote for example
  - basic_auth_user (optional): username to use for HTTP basic authentication
  - basic_auth_password (optional): password to use for HTTP basic authentication
  - no_follow_redirect (optional): indicates that you don't want to follow Location if server returns a Redirect (301/302/...)
  - skip_body: skip the body and bodyjson result
  - skip_headers: skip the headers result
  - tls_client_cert (optional): a chain of certificates to identify the caller, first certificate in the chain is considered as the leaf, followed by intermediates. Setting it enables mutual TLS authentication, and the certificate is sent even when ignore_verify_ssl is true. Set the PEM content or the path to the PEM file. It needs tls_client_key: a certificate without its key fails the step.
  - tls_client_key (optional): private key corresponding to the certificate. Set the PEM content or the path to the PEM file. It needs tls_client_cert: a key without its certificate fails the step.
  - tls_root_ca (optional): defines additional root CAs to perform the call. Can contain multiple CAs concatenated together. Set the PEM content or the path to the PEM file.
  - tls_root_ca, tls_client_cert and tls_client_key only serve TLS connections. With an `http://` url the step logs a warning; a redirect to `https://` or an `https://` proxy still uses them.

```

```yaml

name: HTTP testsuite
testcases:
- name: get http testcase
  steps:
  - type: http
    method: GET
    url: https://eu.api.ovh.com/1.0/
    assertions:
    - result.body ShouldContainSubstring /dedicated/server
    - result.body ShouldContainSubstring /ipLoadbalancing
    - result.statuscode ShouldEqual 200
    - result.bodyjson.api ShouldBeNil
    - result.bodyjson.apis ShouldNotBeEmpty
    - result.bodyjson.apis.apis0 ShouldNotBeNil
    - result.bodyjson.apis.apis0.path ShouldEqual /allDom

- name: post http multipart
  steps:
  - type: http
    method: POST
    url: https://eu.api.ovh.com/1.0/auth/logout
    multipart_form:
      file: '@./venom.gif'
    assertions:
    - result.statuscode ShouldEqual 401
    vars:
      statuscode:
        from: result.statuscode

- name: post http with body
  steps:
    - type: http
      method: POST
      url: https://httpbin.org/post
      body: |
        {"key": "value"}
      headers:
        Content-Type: application/json
      assertions:
        - result.statuscode ShouldEqual 200
        - result.bodyjson.url ShouldEqual https://httpbin.org/post
      vars:
        statuscode:
          from: result.statuscode
          
- name: post http enhanced assertions
  steps:
  - type: http
    method: POST
    url: https://eu.api.ovh.com/1.0/newAccount/rules
    assertions:
      - result.statuscode ShouldEqual 200
      - result.bodyjson.__Type__ ShouldEqual Array
      # Ensure a minimum of fields are present.
      - result.bodyjson.__Len__ ShouldBeGreaterThanOrEqualTo 8
      # Ensure fields have the right keys.
      - result.bodyjson.bodyjson0 ShouldContainKey fieldName
      - result.bodyjson.bodyjson0 ShouldContainKey mandatory
      - result.bodyjson.bodyjson0 ShouldContainKey regularExpression
      - result.bodyjson.bodyjson0 ShouldContainKey prefix
      - result.bodyjson.bodyjson0 ShouldContainKey examples
      - result.bodyjson.bodyjson0 ShouldNotContainKey lol
      - result.statuscode ShouldNotEqual {{.post-http-multipart.statuscode}}

- name: get http (with options)
  steps:
  - type: http
    method: POST
    url: https://eu.api.ovh.com/1.0
    skip_body: true
    skip_headers: true
    info: request is {{.result.request.method}} {{.result.request.url}} {{.result.request.body}}
    assertions:
      - result.statuscode ShouldEqual 405
      - result.body ShouldBeEmpty
      - result.headers ShouldBeEmpty


```

## Multipart form {#multipart-form}

`multipart_form` invia il corpo della richiesta come `multipart/form-data`. Non può essere combinato con `body` o `bodyFile`.

Ogni chiave è un campo del modulo. Un valore è una singola stringa oppure un elenco di stringhe
per inviare lo stesso nome di campo una volta per ogni elemento (ad esempio una parte `files` ripetuta per il caricamento di più file).

Un valore che inizia con `@` è il percorso di un file da caricare, risolto a partire dalla
directory di lavoro corrente. Aggiungi `;type=<content-type>` per impostare il Content-Type di quella parte;
il valore predefinito è `application/octet-stream`. Rispecchia il `-F field=@path;type=...` di curl.

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

## Output {#output}

```
result.request
result.timeseconds
result.statuscode
result.body
result.bodyjson
result.headers
result.err
```
- result.timeseconds: durata dell'esecuzione
- result.request.method: metodo HTTP della richiesta
- result.request.url: URL HTTP della richiesta
- result.request.body: contenuto del corpo come stringa
- result.request.form: mappa del modulo HTTP
- result.request.post_form: mappa del modulo HTTP inviato con POST
- result.err: se presente, questo campo contiene l'errore
- result.body: corpo della risposta HTTP
- result.bodyjson: corpo della risposta HTTP se è un JSON. Puoi accedere ai dati JSON, ad esempio, come result.bodyjson.yourkey.
- result.headers: header della risposta HTTP
- result.statuscode: status code della risposta HTTP

### Chiavi JSON {#json-keys}

Le chiavi JSON vengono convertite automaticamente in minuscolo (ad esempio usa `result.bodyjson.yourkey`, non
`result.bodyjson.YourKey`).

Inoltre, se una chiave JSON contiene caratteri speciali, questi vengono trasformati in underscore.

### Array JSON {#json-arrays}

Quando una risposta HTTP contiene un array JSON, devi usare la sintassi seguente
per accedere a una chiave specifica di un elemento dell'array: `result.bodyjson.array_name.array_name_index_in_array.key`

Esempio, per ottenere il valore della chiave `path` del *secondo* elemento dell'array `apis`: `result.bodyjson.apis.apis1.path`


## Assertion predefinita {#default-assertion}

```yaml
result.statuscode ShouldEqual 200
```

## Cookie {#cookies}

I cookie sono gestiti automaticamente quando si segue un redirect all'interno di un singolo step.
Non sono supportati tra step successivi.
