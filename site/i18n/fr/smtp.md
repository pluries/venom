---
translates: smtp
title: SMTP
description: Comment utiliser l'executor SMTP dans une testsuite venom.
fingerprint: abcd87d7bf80d280db5eadb6c6eadc1b8cee31c1beb4c1e0abe1ccc6e0654d52
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# Venom - Executor SMTP {#venom---executor-smtp}

Étape pour envoyer des mails via SMTP

Cas d'usage : votre logiciel doit vérifier des mails pour en faire quelque chose ?
Venom peut envoyer un mail, puis exécuter des tests sur votre logiciel.

## Entrée {#input}

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

| Paramètre | Description | Valeur par défaut |
|---|---|---|
| withtls | Facultatif. Se connecte en TLS dès le premier octet (en général sur le port 465) | false |
| verifytls | Facultatif, utilisé avec `withtls: true`. `false` : le certificat du serveur n'est pas vérifié, donc un serveur de test auto-signé est accepté. `true` : le certificat doit s'enchaîner à une racine du système ou à `tls_root_ca` et être valide pour `host`, sinon l'étape échoue | false |
| tls_root_ca | Facultatif, utilisé avec `withtls: true`. Certificats de CA reconnus en plus des racines du système, pour que `verifytls: true` accepte un serveur signé par une CA privée. Plusieurs CA peuvent être concaténées. Il ne sert à vérifier que si `verifytls: true` | |
| tls_client_cert | Facultatif, utilisé avec `withtls: true`. Le certificat client (TLS mutuel), le certificat final en premier, suivi des intermédiaires. Il est envoyé même si `verifytls` vaut `false`. Il exige `tls_client_key` : un certificat sans sa clé fait échouer l'étape | |
| tls_client_key | Facultatif, utilisé avec `withtls: true`. La clé privée de `tls_client_cert`. Elle exige `tls_client_cert` : une clé sans son certificat fait échouer l'étape | |

`tls_root_ca`, `tls_client_cert` et `tls_client_key` prennent le contenu PEM lui-même ou le chemin d'un fichier PEM, relatif
au répertoire de la testsuite. Sans `withtls: true`, ils ne sont pas utilisés : la connexion reste en clair et un avertissement
est journalisé (un certificat sans sa clé, ou l'inverse, fait quand même échouer l'étape).

La variable d'environnement `SSL_CERT_FILE` ou `SSL_CERT_DIR` permet d'indiquer à venom un autre jeu de racines système.

## Sortie {#output}

Rien, sauf result.err en cas d'erreur.

## Assertion par défaut {#default-assertion}

```yaml
result.err ShouldBeEmpty
```
