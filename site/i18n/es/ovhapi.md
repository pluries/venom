---
translates: ovhapi
title: OVHAPI
description: Cómo usar el executor OVHAPI en una suite de pruebas de venom.
fingerprint: 33b8b8e3e61e28e515ea5ca5ac5d85740f605d690b373504f4afcf4ea16ade0e
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# Venom - Executor OVHAPI {#venom---executor-ovhapi}

## Paso para probar la API de OVH {#step-to-test-ovh-api}

Caso de uso: su software necesita hacer llamadas a la API de OVH.<br>
Necesitará credenciales de OVH para hacer llamadas a la API. Puede usar la autenticación con claves de aplicación o OAuth2.

Para usar la autenticación con claves de aplicación, siga este tutorial: <br>
EN: https://docs.ovh.com/gb/en/customer/first-steps-with-ovh-api/

Para usar OAuth2, siga este tutorial: <br>
EN: https://help.ovhcloud.com/csm/en-manage-service-account?id=kb_article_view&sysparm_article=KB0059343

## Entrada {#input}

Están disponibles los siguientes parámetros:

| Parámetro         | Descripción                                                      | Valor por defecto |
|-------------------|------------------------------------------------------------------|---------------|
| endpoint          | Opcional                                                         | ovh-eu        |
| applicationKey    | Opcional si `noAuth`, obligatorio con la autenticación por claves de aplicación |               |
| applicationSecret | Opcional si `noAuth`, obligatorio con la autenticación por claves de aplicación |               |
| consumerKey       | Opcional si `noAuth`, obligatorio con la autenticación por claves de aplicación |               |
| clientID          | Opcional si `noAuth`, obligatorio con OAuth2                     |               |
| clientSecret      | Opcional si `noAuth`, obligatorio con OAuth2                     |               |
| noAuth            | Opcional                                                         |               |
| headers           | Opcional                                                         |               |
| resolve           | Opcional                                                         |               |
| proxy             | Opcional                                                         |               |
| tlsRootCA         | Opcional, véase [CA raíz privada](#private-root-ca)              |               |
| tls_root_ca       | Opcional, otro nombre de `tlsRootCA`                             |               |

| Parámetro | Descripción | Valor por defecto |
|-----------|-------------|---------------|
| method    | Opcional    | GET           |
| path      | Obligatorio |               |
| body      | Opcional    |               |
| bodyFile  | Opcional    |               |
 
El primer grupo de parámetros también puede definirse dentro de las variables de Venom así:

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

### CA raíz privada {#private-root-ca}

`tlsRootCA` y `tls_root_ca` son dos nombres de un mismo ajuste: una CA en la que el executor confía además de las del sistema,
para una API detrás de un certificado privado. Defina uno de los dos; un paso que defina ambos falla antes de cualquier llamada.
La variable `ovh.tlsRootCA`, la única variable para ambos nombres, solo se usa cuando el paso no define ninguno de los dos.

El valor es el texto PEM del certificado, o la ruta de un archivo PEM. Una ruta relativa se lee desde la
carpeta de la suite de pruebas. La variable `ovh.tlsRootCA` también acepta una ruta.

La CA raíz solo sirve para las conexiones TLS. Con un endpoint `http://` el paso registra una advertencia; una redirección a
`https://` o un proxy `https://` sigue usando la CA. Un nombre de endpoint como `ovh-eu` es una URL `https://` y no genera advertencia.

```yaml
- type: ovhapi
  endpoint: 'https://api.example.internal/1.0'
  tls_root_ca: certs/internal-ca.pem
  noAuth: true
  path: /me
```

## Ejemplo de una suite de pruebas __ovhapi__ {#example-of-an-__ovhapi__-testsuite}

### Con autenticación por claves de aplicación {#using-app-keys-authentication}

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

## Salida {#output}

Están disponibles los siguientes campos de salida:

| Campo              | Descripción                          |
|--------------------|--------------------------------------|
| result.executor    |                                      |
| result.timeseconds | Duración de la ejecución             |
| result.statuscode  | Código de estado de la respuesta HTTP |
| result.body        | Cuerpo de la respuesta HTTP          |
| result.bodyjson    | Cuerpo de la respuesta HTTP si es JSON |
| result.err         | Mensaje de error, si existe          |

Tenga en cuenta que puede acceder a los datos JSON como `result.bodyjson.yourkey`, por ejemplo.

## Aserción por defecto {#default-assertion}

```yaml
result.statuscode ShouldEqual 200
```
