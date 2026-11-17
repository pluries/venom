---
translates: http
title: HTTP
description: So verwenden Sie den HTTP-executor in einer venom-Test-Suite.
fingerprint: 859e45938b22aae5cb32f1766961507301c17073180a5535014e3adc8f77cecb
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# Venom - Executor HTTP {#venom---executor-http}

Schritt zum Ausführen einer HTTP-Anfrage

## Input {#input}
In Ihrer YAML-Datei können Sie Folgendes verwenden:

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

`multipart_form` sendet den Request-Body als `multipart/form-data`. Es lässt sich nicht mit `body` oder `bodyFile` kombinieren.

Jeder Schlüssel ist ein Formularfeld. Ein Wert ist entweder ein einzelner String oder eine Liste von Strings,
um denselben Feldnamen einmal pro Eintrag zu senden (z. B. ein wiederholter Teil `files` für einen Upload mehrerer Dateien).

Ein Wert, der mit `@` beginnt, ist der Pfad einer hochzuladenden Datei, aufgelöst vom aktuellen
Arbeitsverzeichnis aus. Hängen Sie `;type=<content-type>` an, um den Content-Type dieses Teils zu setzen;
er ist standardmäßig `application/octet-stream`. Das entspricht `-F field=@path;type=...` von curl.

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
- result.timeseconds: Ausführungsdauer
- result.request.method: HTTP-Methode der Anfrage
- result.request.url: HTTP-URL der Anfrage
- result.request.body: Inhalt des Bodys als String
- result.request.form: HTTP-Formular-Map
- result.request.post_form: HTTP-Post-Formular-Map
- result.err: falls vorhanden, enthält dieses Feld den Fehler
- result.body: Body der HTTP-Antwort
- result.bodyjson: Body der HTTP-Antwort, wenn es sich um JSON handelt. Auf JSON-Daten greifen Sie zum Beispiel mit result.bodyjson.yourkey zu.
- result.headers: Header der HTTP-Antwort
- result.statuscode: Statuscode der HTTP-Antwort

### JSON-Schlüssel {#json-keys}

JSON-Schlüssel werden automatisch in Kleinbuchstaben umgewandelt (verwenden Sie z. B. `result.bodyjson.yourkey`, nicht
`result.bodyjson.YourKey`).

Enthält ein JSON-Schlüssel Sonderzeichen, werden sie außerdem in Unterstriche umgewandelt.

### JSON-Arrays {#json-arrays}

Enthält eine HTTP-Antwort ein JSON-Array, müssen Sie die folgende Syntax verwenden,
um auf einen bestimmten Schlüssel eines Arrays zuzugreifen: `result.bodyjson.array_name.array_name_index_in_array.key`

Beispiel, wenn Sie den Wert des Schlüssels `path` des *zweiten* Elements im Array `apis` abrufen möchten: `result.bodyjson.apis.apis1.path`


## Default assertion {#default-assertion}

```yaml
result.statuscode ShouldEqual 200
```

## Cookies {#cookies}

Cookies werden automatisch behandelt, wenn innerhalb eines einzelnen Schritts einer Weiterleitung gefolgt wird.
Zwischen aufeinanderfolgenden Schritten werden sie nicht unterstützt.
