---
translates: smtp
title: SMTP
description: Den SMTP-executor in einer venom-Test-Suite verwenden.
fingerprint: abcd87d7bf80d280db5eadb6c6eadc1b8cee31c1beb4c1e0abe1ccc6e0654d52
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# Venom - Executor SMTP {#venom---executor-smtp}

Schritt zum Senden per SMTP

Anwendungsfall: Ihre Software muss Mails prüfen, um etwas mit ihnen zu tun?
Venom kann eine Mail senden und anschließend einige Tests gegen Ihre Software ausführen.

## Eingabe {#input}

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

| Parameter | Beschreibung | Standardwert |
|---|---|---|
| withtls | Optional. Verbindung von Beginn an über TLS aufbauen (meist Port 465) | false |
| verifytls | Optional, zusammen mit `withtls: true` verwendet. `false`: Das Serverzertifikat wird nicht geprüft, sodass ein selbstsignierter Testserver akzeptiert wird. `true`: Das Zertifikat muss zu einem System-Root oder zu `tls_root_ca` führen und für `host` gültig sein, sonst schlägt der Schritt fehl | false |
| tls_root_ca | Optional, zusammen mit `withtls: true` verwendet. CA-Zertifikate, denen zusätzlich zu den System-Roots vertraut wird, damit `verifytls: true` einen Server akzeptiert, der von einer privaten CA signiert wurde. Mehrere CAs können aneinandergehängt werden. Es dient nur dann zur Prüfung, wenn `verifytls: true` gilt | |
| tls_client_cert | Optional, zusammen mit `withtls: true` verwendet. Das Client-Zertifikat (Mutual TLS), zuerst das Blattzertifikat, danach Zwischenzertifikate. Es wird auch gesendet, wenn `verifytls` auf `false` steht. Es braucht `tls_client_key`: Ein Zertifikat ohne seinen Schlüssel lässt den Schritt fehlschlagen | |
| tls_client_key | Optional, zusammen mit `withtls: true` verwendet. Der private Schlüssel von `tls_client_cert`. Er braucht `tls_client_cert`: Ein Schlüssel ohne sein Zertifikat lässt den Schritt fehlschlagen | |

`tls_root_ca`, `tls_client_cert` und `tls_client_key` nehmen den PEM-Inhalt selbst oder den Pfad zu einer PEM-Datei entgegen, relativ
zum Verzeichnis der Test-Suite. Ohne `withtls: true` werden sie nicht verwendet: Die Verbindung bleibt unverschlüsselt, und eine Warnung
wird protokolliert (ein Zertifikat ohne seinen Schlüssel oder umgekehrt lässt den Schritt dennoch fehlschlagen).

Über die Umgebungsvariable `SSL_CERT_FILE` oder `SSL_CERT_DIR` lässt sich venom auf einen anderen Satz von System-Roots verweisen.

## Ausgabe {#output}

Nichts, außer result.err, wenn ein Fehler auftritt.

## Standard-Assertion {#default-assertion}

```yaml
result.err ShouldBeEmpty
```
