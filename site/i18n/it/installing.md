---
translates: installing
title: Installazione
description: Come installare venom.
fingerprint: 93d69cbe8c61d9eee2f2f904a568c6e04dc04f75ad107145a0ef688e8851a310
drafted_by: claude
drafted_on: 2026-11-16
status: reviewed
reviewed_by: Christian Stephan
reviewed_on: 2026-11-16
reviewed_fingerprint: 93d69cbe8c61d9eee2f2f904a568c6e04dc04f75ad107145a0ef688e8851a310
---
# Installazione {#installing}

## Installa dai binari {#install-from-binaries}

Ogni release di questo fork è pubblicata su https://github.com/pluries/venom/releases/latest. Contiene un eseguibile per ogni piattaforma:
`venom.linux-amd64`, `venom.linux-arm64`, `venom.linux-arm`, `venom.linux-386`, `venom.darwin-amd64`, `venom.darwin-arm64`,
`venom.windows-amd64.exe`, `venom.freebsd-amd64` e `venom.openbsd-amd64`, più l'elenco dei checksum `SHA256SUMS` e la sua firma `SHA256SUMS.p7s`.

Esempio per Linux:

```bash
$ curl -L -o venom https://github.com/pluries/venom/releases/latest/download/venom.linux-amd64
$ chmod +x venom && sudo mv venom /usr/local/bin/venom
$ venom -h
```

### Verifica il download {#verify-the-download}

L'elenco dei checksum è firmato con un certificato di firma del codice Certum, e anche l'eseguibile per Windows porta una firma Authenticode.
Ti servono `sha256sum` e `openssl`, nient'altro:

```bash
$ curl -LO https://github.com/pluries/venom/releases/latest/download/SHA256SUMS
$ curl -LO https://github.com/pluries/venom/releases/latest/download/SHA256SUMS.p7s
$ sha256sum -c SHA256SUMS --ignore-missing
$ openssl cms -verify -inform DER -in SHA256SUMS.p7s -content SHA256SUMS -binary -purpose any
```

Il primo controllo confronta il tuo file con l'elenco, il secondo dimostra che l'elenco è stato firmato dal certificato del maintainer.
