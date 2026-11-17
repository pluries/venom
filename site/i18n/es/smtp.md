---
translates: smtp
title: SMTP
description: Cómo usar el executor SMTP en una suite de pruebas de venom.
fingerprint: abcd87d7bf80d280db5eadb6c6eadc1b8cee31c1beb4c1e0abe1ccc6e0654d52
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# Venom - Executor SMTP {#venom---executor-smtp}

Paso para enviar correo por SMTP

Caso de uso: ¿su software tiene que revisar correos para hacer algo con ellos?
Venom puede enviar un correo y después ejecutar pruebas sobre su software.

## Entrada {#input}

```yaml
name: TestSuite with SMTP Steps
testcases:
- name: TestCase SMTP
  steps:
  - type: smtp
    withtls: false
    verifytls: false # Optional, used with withtls: true (see TLS below)
    host: localhost
    port: 25 # 465 if using TLS
    user: yourSMTPUsername # Optional, only works with TLS
    password: yourSMTPPassword # Optional, only works with TLS
    from: venom@smtp.net
    to: destinationa@yourdomain.com,destinationb@yourdomain.com
    subject: Title of mail
    body: Body of mail
    tls_root_ca: ca.pem # Optional, used with withtls: true and verifytls: true (see TLS below)
    tls_client_cert: client.pem # Optional, used with withtls: true, together with tls_client_key
    tls_client_key: client.key # Optional, used with withtls: true, together with tls_client_cert
    assertions:
    - result.err ShouldBeEmpty
```

### TLS {#tls}

| Parámetro | Descripción | Valor por defecto |
|---|---|---|
| withtls | Opcional. Conecta por TLS desde el primer byte (normalmente el puerto 465) | false |
| verifytls | Opcional, se usa con `withtls: true`. `false`: no se comprueba el certificado del servidor, por lo que se acepta un servidor de pruebas con certificado autofirmado. `true`: el certificado debe encadenarse a una raíz del sistema o a `tls_root_ca` y ser válido para `host`; de lo contrario, el paso falla | false |
| tls_root_ca | Opcional, se usa con `withtls: true`. Certificados de CA en los que se confía además de las raíces del sistema, de modo que `verifytls: true` acepte un servidor firmado por una CA privada. Pueden concatenarse varias CA. Solo se usa para verificar cuando `verifytls: true` | |
| tls_client_cert | Opcional, se usa con `withtls: true`. El certificado de cliente (TLS mutuo), primero el certificado final y después los intermedios. Se envía aunque `verifytls` sea `false`. Necesita `tls_client_key`: un certificado sin su clave hace fallar el paso | |
| tls_client_key | Opcional, se usa con `withtls: true`. La clave privada de `tls_client_cert`. Necesita `tls_client_cert`: una clave sin su certificado hace fallar el paso | |

`tls_root_ca`, `tls_client_cert` y `tls_client_key` admiten el contenido PEM en sí o la ruta de un archivo PEM, relativa
al directorio de la suite de pruebas. Sin `withtls: true` no se usan: la conexión sigue en texto plano y se registra una advertencia
(un certificado sin su clave, o a la inversa, sigue haciendo fallar el paso).

La variable de entorno `SSL_CERT_FILE` o `SSL_CERT_DIR` puede indicar a venom otro conjunto de raíces del sistema.

## Salida {#output}

Nada, salvo result.err si hay un error.

## Aserción por defecto {#default-assertion}

```yaml
result.err ShouldBeEmpty
```
