---
translates: http
title: HTTP
description: Comment utiliser l'executor HTTP dans une suite de tests venom.
fingerprint: 859e45938b22aae5cb32f1766961507301c17073180a5535014e3adc8f77cecb
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# Venom - Executor HTTP {#venom---executor-http}

Étape pour exécuter une requête HTTP

## Input {#input}
Dans votre fichier yaml, vous pouvez utiliser :

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

`multipart_form` envoie le corps de la requête en `multipart/form-data`. Il ne peut pas être combiné avec `body` ni `bodyFile`.

Chaque clé est un champ de formulaire. Une valeur est soit une simple chaîne, soit une liste de chaînes
pour envoyer le même nom de champ une fois par élément (par exemple une partie `files` répétée pour un envoi de plusieurs fichiers).

Une valeur qui commence par `@` est le chemin d'un fichier à envoyer, résolu depuis le répertoire de travail
courant. Ajoutez `;type=<content-type>` pour définir le Content-Type de cette partie ;
il vaut `application/octet-stream` par défaut. Cela reproduit le `-F field=@path;type=...` de curl.

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
- result.timeseconds : durée d'exécution
- result.request.method : méthode HTTP de la requête
- result.request.url : URL HTTP de la requête
- result.request.body : contenu du corps, sous forme de chaîne
- result.request.form : map du formulaire HTTP
- result.request.post_form : map du formulaire HTTP envoyé en POST
- result.err : s'il existe, ce champ contient l'erreur
- result.body : corps de la réponse HTTP
- result.bodyjson : corps de la réponse HTTP s'il s'agit de JSON. Vous accédez aux données JSON par exemple avec result.bodyjson.yourkey.
- result.headers : en-têtes de la réponse HTTP
- result.statuscode : code de statut de la réponse HTTP

### JSON keys {#json-keys}

Les clés JSON sont automatiquement mises en minuscules (par exemple, utilisez `result.bodyjson.yourkey`, et non
`result.bodyjson.YourKey`).

De plus, si une clé JSON contient des caractères spéciaux, ils sont remplacés par des tirets bas.

### JSON arrays {#json-arrays}

Lorsqu'une réponse HTTP contient un tableau JSON, vous devez utiliser la syntaxe suivante
pour accéder à une clé précise d'un élément du tableau : `result.bodyjson.array_name.array_name_index_in_array.key`

Exemple pour obtenir la valeur de la clé `path` du *deuxième* élément du tableau `apis` : `result.bodyjson.apis.apis1.path`


## Default assertion {#default-assertion}

```yaml
result.statuscode ShouldEqual 200
```

## Cookies {#cookies}

Les cookies sont gérés automatiquement lors du suivi d'une redirection au sein d'une même étape.
Ils ne sont pas pris en charge entre des étapes successives.
