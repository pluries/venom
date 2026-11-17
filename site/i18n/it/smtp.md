---
translates: smtp
title: SMTP
description: Come usare l'executor SMTP in una test suite di venom.
fingerprint: abcd87d7bf80d280db5eadb6c6eadc1b8cee31c1beb4c1e0abe1ccc6e0654d52
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# Venom - Executor SMTP {#venom---executor-smtp}

Step per inviare email via SMTP

Caso d'uso: il tuo software deve controllare le email per farci qualcosa?
Venom può inviare un'email ed eseguire poi dei test sul tuo software.

## Input {#input}

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

| Parametro | Descrizione | Valore predefinito |
|---|---|---|
| withtls | Facoltativo. Si connette in TLS fin dal primo byte (di solito porta 465) | false |
| verifytls | Facoltativo, usato con `withtls: true`. `false`: il certificato del server non viene controllato, quindi un server di test con certificato autofirmato viene accettato. `true`: il certificato deve risalire a una radice di sistema o a `tls_root_ca` ed essere valido per `host`, altrimenti lo step fallisce | false |
| tls_root_ca | Facoltativo, usato con `withtls: true`. Certificati CA di cui ci si fida in aggiunta alle radici di sistema, così che `verifytls: true` accetti un server firmato da una CA privata. Si possono concatenare più CA. Viene usato per la verifica solo quando `verifytls: true` | |
| tls_client_cert | Facoltativo, usato con `withtls: true`. Il certificato client (mutual TLS), prima la foglia, poi gli intermedi. Viene inviato anche quando `verifytls` è `false`. Richiede `tls_client_key`: un certificato senza la sua chiave fa fallire lo step | |
| tls_client_key | Facoltativo, usato con `withtls: true`. La chiave privata di `tls_client_cert`. Richiede `tls_client_cert`: una chiave senza il suo certificato fa fallire lo step | |

`tls_root_ca`, `tls_client_cert` e `tls_client_key` accettano il contenuto PEM stesso oppure il percorso di un file PEM, relativo
alla directory della test suite. Senza `withtls: true` non vengono usati: la connessione resta in chiaro e viene scritto un avviso
nel log (un certificato senza la sua chiave, o il contrario, fa comunque fallire lo step).

La variabile d'ambiente `SSL_CERT_FILE` o `SSL_CERT_DIR` può far puntare venom a un altro insieme di radici di sistema.

## Output {#output}

Niente, tranne result.err in caso di errore.

## Assertion predefinita {#default-assertion}

```yaml
result.err ShouldBeEmpty
```
