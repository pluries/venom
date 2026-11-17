---
translates: ovhapi
title: OVHAPI
description: Come usare l'executor OVHAPI in una test suite di venom.
fingerprint: 33b8b8e3e61e28e515ea5ca5ac5d85740f605d690b373504f4afcf4ea16ade0e
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# Venom - Executor OVHAPI {#venom---executor-ovhapi}

## Step per testare l'API OVH {#step-to-test-ovh-api}

Caso d'uso: il tuo software deve effettuare chiamate all'API OVH.<br>
Ti servono credenziali OVH per effettuare le chiamate. Puoi usare l'autenticazione con app keys oppure OAuth2.

Per usare l'autenticazione con app keys, segui questo tutorial: <br>
EN: https://docs.ovh.com/gb/en/customer/first-steps-with-ovh-api/

Per usare OAuth2, segui questo tutorial: <br>
EN: https://help.ovhcloud.com/csm/en-manage-service-account?id=kb_article_view&sysparm_article=KB0059343

## Input {#input}

Sono disponibili i seguenti parametri:

| Parameter         | Description                                                      | Default Value |
|-------------------|------------------------------------------------------------------|---------------|
| endpoint          | Facoltativo                                                      | ovh-eu        |
| applicationKey    | Facoltativo con `noAuth`, obbligatorio con l'autenticazione app keys |               |
| applicationSecret | Facoltativo con `noAuth`, obbligatorio con l'autenticazione app keys |               |
| consumerKey       | Facoltativo con `noAuth`, obbligatorio con l'autenticazione app keys |               |
| clientID          | Facoltativo con `noAuth`, obbligatorio con OAuth2                |               |
| clientSecret      | Facoltativo con `noAuth`, obbligatorio con OAuth2                |               |
| noAuth            | Facoltativo                                                      |               |
| headers           | Facoltativo                                                      |               |
| resolve           | Facoltativo                                                      |               |
| proxy             | Facoltativo                                                      |               |
| tlsRootCA         | Facoltativo, vedi [CA radice privata](#private-root-ca)          |               |
| tls_root_ca       | Facoltativo, altro nome di `tlsRootCA`                           |               |

| Parameter | Description | Default Value |
|-----------|-------------|---------------|
| method    | Facoltativo | GET           |
| path      | Obbligatorio |              |
| body      | Facoltativo |               |
| bodyFile  | Facoltativo |               |
 
Il primo gruppo di parametri può anche essere definito nelle variabili di Venom, così:

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

### CA radice privata {#private-root-ca}

`tlsRootCA` e `tls_root_ca` sono due nomi della stessa impostazione: una CA di cui l'executor si fida in aggiunta a quelle di sistema,
per un'API dietro un certificato privato. Impostane uno solo; uno step che li imposta entrambi fallisce prima di qualsiasi chiamata.
La variabile `ovh.tlsRootCA`, l'unica variabile per entrambi i nomi, viene usata solo quando lo step non imposta nessuno dei due.

Il valore è il testo PEM del certificato, oppure il percorso di un file PEM. Un percorso relativo viene letto a partire dalla
cartella della test suite. Anche la variabile `ovh.tlsRootCA` accetta un percorso.

La CA radice serve solo per le connessioni TLS. Con un endpoint `http://` lo step registra un avviso; un reindirizzamento verso
`https://` o un proxy `https://` usa comunque la CA. Un nome di endpoint come `ovh-eu` è un URL `https://` e non genera avvisi.

```yaml
- type: ovhapi
  endpoint: 'https://api.example.internal/1.0'
  tls_root_ca: certs/internal-ca.pem
  noAuth: true
  path: /me
```

## Esempio di test suite __ovhapi__ {#example-of-an-__ovhapi__-testsuite}

### Con autenticazione app keys {#using-app-keys-authentication}

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

### Con OAuth2 {#using-oauth2}

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

## Output {#output}

Sono disponibili i seguenti campi di output:

| Field              | Description                          |
|--------------------|--------------------------------------|
| result.executor    |                                      |
| result.timeseconds | Durata dell'esecuzione               |
| result.statuscode  | Codice di stato della risposta HTTP  |
| result.body        | Corpo della risposta HTTP            |
| result.bodyjson    | Corpo della risposta HTTP se è JSON  |
| result.err         | Messaggio di errore, se presente     |

Nota che puoi accedere ai dati JSON come `result.bodyjson.yourkey`, per esempio.

## Assertion predefinita {#default-assertion}

```yaml
result.statuscode ShouldEqual 200
```
