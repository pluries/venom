---
translates: ovhapi
title: OVHAPI
description: Den OVHAPI-executor in einer venom-Test-Suite verwenden.
fingerprint: 33b8b8e3e61e28e515ea5ca5ac5d85740f605d690b373504f4afcf4ea16ade0e
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# Venom - Executor OVHAPI {#venom---executor-ovhapi}

## Schritt zum Testen der OVH-API {#step-to-test-ovh-api}

Anwendungsfall: Ihre Software muss Aufrufe an die OVH-API senden.<br>
Für diese Aufrufe benötigen Sie OVH-Zugangsdaten. Sie können entweder die Authentifizierung mit App-Keys oder OAuth2 verwenden.

Für die Authentifizierung mit App-Keys folgen Sie bitte dieser Anleitung: <br>
EN: https://docs.ovh.com/gb/en/customer/first-steps-with-ovh-api/

Für OAuth2 folgen Sie bitte dieser Anleitung: <br>
EN: https://help.ovhcloud.com/csm/en-manage-service-account?id=kb_article_view&sysparm_article=KB0059343

## Eingabe {#input}

Die folgenden Parameter stehen zur Verfügung:

| Parameter         | Beschreibung                                                      | Standardwert  |
|-------------------|-------------------------------------------------------------------|---------------|
| endpoint          | Optional                                                          | ovh-eu        |
| applicationKey    | Optional bei `noAuth`, Pflicht bei Authentifizierung mit App-Keys |               |
| applicationSecret | Optional bei `noAuth`, Pflicht bei Authentifizierung mit App-Keys |               |
| consumerKey       | Optional bei `noAuth`, Pflicht bei Authentifizierung mit App-Keys |               |
| clientID          | Optional bei `noAuth`, Pflicht bei OAuth2                         |               |
| clientSecret      | Optional bei `noAuth`, Pflicht bei OAuth2                         |               |
| noAuth            | Optional                                                          |               |
| headers           | Optional                                                          |               |
| resolve           | Optional                                                          |               |
| proxy             | Optional                                                          |               |
| tlsRootCA         | Optional, siehe [Private Root-CA](#private-root-ca)               |               |
| tls_root_ca       | Optional, ein anderer Name für `tlsRootCA`                        |               |

| Parameter | Beschreibung | Standardwert |
|-----------|--------------|--------------|
| method    | Optional     | GET          |
| path      | Pflicht      |              |
| body      | Optional     |              |
| bodyFile  | Optional     |              |
 
Die erste Gruppe von Parametern kann auch in Venom-Variablen wie folgt definiert werden:

```yaml
vars:
  ovh.endpoint: ovh-eu
  ovh.applicationKey: foo
  ovh.applicationSecret: foo
  ovh.clientID: foo
  ovh.clientSecret: foo
  ovh.consumerKey: foo
  ovh.noAuth: false
  ovh.headers:
    x-foo: foo
  ovh.resolve:
  - example.org:443:127.0.0.1
  ovh.proxy: localhost:8000
  ovh.tlsRootCA: |-
    -----BEGIN CERTIFICATE-----
    MIIF3jCCA8agAwIBAgIQAf1tMPyjylGoG7xkDjUDLTANBgkqhkiG9w0BAQwFADCB
    ...
    -----END CERTIFICATE-----
```

### Private Root-CA {#private-root-ca}

`tlsRootCA` und `tls_root_ca` sind zwei Namen für dieselbe Einstellung: eine CA, der der executor zusätzlich zu den Systemzertifikaten vertraut,
für eine API hinter einem privaten Zertifikat. Setzen Sie nur einen der beiden Namen; ein Schritt, der beide setzt, schlägt vor jedem Aufruf fehl.
Die Variable `ovh.tlsRootCA`, die eine Variable für beide Namen, wird nur verwendet, wenn der Schritt keinen von beiden setzt.

Der Wert ist der PEM-Text des Zertifikats oder der Pfad einer PEM-Datei. Ein relativer Pfad wird ab dem
Verzeichnis der Test-Suite gelesen. Auch die Variable `ovh.tlsRootCA` akzeptiert einen Pfad.

Die Root-CA dient nur TLS-Verbindungen. Bei einem `http://`-Endpoint schreibt der Schritt eine Warnung ins Log; eine Weiterleitung auf
`https://` oder ein `https://`-Proxy verwendet die CA weiterhin. Ein Endpoint-Name wie `ovh-eu` ist eine `https://`-URL und löst keine Warnung aus.

```yaml
- type: ovhapi
  endpoint: 'https://api.example.internal/1.0'
  tls_root_ca: certs/internal-ca.pem
  noAuth: true
  path: /me
```

## Beispiel einer __ovhapi__-TestSuite {#example-of-an-__ovhapi__-testsuite}

### Authentifizierung mit App-Keys {#using-app-keys-authentication}

```yaml
name: Title of TestSuite
testcases:
- name: me
  steps:
  - type: ovhapi
    endpoint: 'ovh-eu'
    applicationKey: 'APPLICATION_KEY'
    applicationSecret: 'APPLICATION_SECRET'
    consumerKey: 'CONSUMER_KEY'
    method: GET
    path: /me
    retry: 3
    delay: 2
    assertions:
    - result.statuscode ShouldEqual 200
    - result.bodyjson.nichandle ShouldContainSubstring MY_NICHANDLE
```

### Authentifizierung mit OAuth2 {#using-oauth2}

```yaml
name: Title of TestSuite
testcases:
- name: me
  steps:
  - type: ovhapi
    endpoint: 'ovh-eu'
    clientID: 'CLIENT_ID'
    clientSecret: 'CLIENT_SECRET'
    method: GET
    path: /me
    retry: 3
    delay: 2
    assertions:
    - result.statuscode ShouldEqual 200
    - result.bodyjson.nichandle ShouldContainSubstring MY_NICHANDLE
```

## Ausgabe {#output}

Die folgenden Ausgabefelder stehen zur Verfügung:

| Feld               | Beschreibung                            |
|--------------------|-----------------------------------------|
| result.executor    |                                         |
| result.timeseconds | Ausführungszeit                         |
| result.statuscode  | Statuscode der HTTP-Antwort             |
| result.body        | Body der HTTP-Antwort                   |
| result.bodyjson    | Body der HTTP-Antwort, wenn er JSON ist |
| result.err         | Fehlermeldung, falls vorhanden          |

Beachten Sie, dass Sie auf JSON-Daten zum Beispiel als `result.bodyjson.yourkey` zugreifen können.

## Standard-Assertion {#default-assertion}

```yaml
result.statuscode ShouldEqual 200
```
