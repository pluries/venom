---
translates: ovhapi
title: OVHAPI
description: Comment utiliser l'executor OVHAPI dans une testsuite venom.
fingerprint: 33b8b8e3e61e28e515ea5ca5ac5d85740f605d690b373504f4afcf4ea16ade0e
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# Venom - Executor OVHAPI {#venom---executor-ovhapi}

## Étape pour tester l'API OVH {#step-to-test-ovh-api}

Cas d'usage : votre logiciel doit effectuer des appels à l'API OVH.<br>
Vous aurez besoin d'identifiants OVH pour appeler l'API. Vous pouvez utiliser l'authentification par clés d'application ou OAuth2.

Pour utiliser l'authentification par clés d'application, suivez ce tutoriel : <br>
EN: https://docs.ovh.com/gb/en/customer/first-steps-with-ovh-api/

Pour utiliser OAuth2, suivez ce tutoriel : <br>
EN: https://help.ovhcloud.com/csm/en-manage-service-account?id=kb_article_view&sysparm_article=KB0059343

## Entrée {#input}

Les paramètres suivants sont disponibles :

| Paramètre         | Description                                                      | Valeur par défaut |
|-------------------|------------------------------------------------------------------|---------------|
| endpoint          | Facultatif                                                       | ovh-eu        |
| applicationKey    | Facultatif avec `noAuth`, obligatoire avec l'authentification par clés d'application |               |
| applicationSecret | Facultatif avec `noAuth`, obligatoire avec l'authentification par clés d'application |               |
| consumerKey       | Facultatif avec `noAuth`, obligatoire avec l'authentification par clés d'application |               |
| clientID          | Facultatif avec `noAuth`, obligatoire avec OAuth2                |               |
| clientSecret      | Facultatif avec `noAuth`, obligatoire avec OAuth2                |               |
| noAuth            | Facultatif                                                       |               |
| headers           | Facultatif                                                       |               |
| resolve           | Facultatif                                                       |               |
| proxy             | Facultatif                                                       |               |
| tlsRootCA         | Facultatif, voir [CA racine privée](#private-root-ca)            |               |
| tls_root_ca       | Facultatif, autre nom de `tlsRootCA`                             |               |

| Paramètre | Description | Valeur par défaut |
|-----------|-------------|---------------|
| method    | Facultatif  | GET           |
| path      | Obligatoire |               |
| body      | Facultatif  |               |
| bodyFile  | Facultatif  |               |
 
Le premier groupe de paramètres peut aussi être défini dans les variables de Venom, comme ceci :

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

### CA racine privée {#private-root-ca}

`tlsRootCA` et `tls_root_ca` sont deux noms pour un même réglage : une CA à laquelle l'executor fait confiance en plus de celles du système,
pour une API derrière un certificat privé. Renseignez-en un seul ; une étape qui définit les deux échoue avant tout appel.
La variable `ovh.tlsRootCA`, la variable unique pour les deux noms, n'est utilisée que si l'étape ne définit ni l'un ni l'autre.

La valeur est le texte PEM du certificat, ou le chemin d'un fichier PEM. Un chemin relatif est lu depuis le
dossier de la testsuite. La variable `ovh.tlsRootCA` accepte aussi un chemin.

La CA racine ne sert que pour les connexions TLS. Avec un endpoint `http://`, l'étape journalise un avertissement ; une redirection vers
`https://` ou un proxy `https://` utilise quand même la CA. Un nom d'endpoint comme `ovh-eu` est une URL `https://` et n'avertit pas.

```yaml
- type: ovhapi
  endpoint: 'https://api.example.internal/1.0'
  tls_root_ca: certs/internal-ca.pem
  noAuth: true
  path: /me
```

## Exemple de testsuite __ovhapi__ {#example-of-an-__ovhapi__-testsuite}

### Avec l'authentification par clés d'application {#using-app-keys-authentication}

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

### Avec OAuth2 {#using-oauth2}

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

## Sortie {#output}

Les champs de sortie suivants sont disponibles :

| Champ              | Description                          |
|--------------------|--------------------------------------|
| result.executor    |                                      |
| result.timeseconds | Durée d'exécution                    |
| result.statuscode  | Code de statut de la réponse HTTP    |
| result.body        | Corps de la réponse HTTP             |
| result.bodyjson    | Corps de la réponse HTTP s'il s'agit de JSON |
| result.err         | Message d'erreur s'il existe         |

Notez que vous pouvez accéder aux données JSON comme `result.bodyjson.yourkey`, par exemple.

## Assertion par défaut {#default-assertion}

```yaml
result.statuscode ShouldEqual 200
```
