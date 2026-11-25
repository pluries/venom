---
translates: verify-release
title: Verificare una release
description: Verifica che una release sia firmata con il certificato del maintainer.
fingerprint: 11141e8943d763bea06a3462db682b51480988aa7bd53a6b320de816db989244
drafted_by: claude
drafted_on: 2026-11-24
status: machine-drafted
---
Ogni release di questo fork è firmata e puoi verificare la firma senza fidarti della pagina di download. Questa pagina mostra come provare che una release è del maintainer: che l'elenco dei checksum è firmato, che il firmatario è il certificato del maintainer e che il tuo file è nell'elenco. Servono `openssl` 3 o successivo, `sha256sum` e `curl`. LibreSSL, fornito con macOS, non può verificare il timestamp della firma: installa prima OpenSSL 3 su quel sistema.

Scarica l'elenco dei checksum, la sua firma e l'eseguibile che vuoi:

```bash
$ curl -LO https://github.com/pluries/venom/releases/latest/download/SHA256SUMS
$ curl -LO https://github.com/pluries/venom/releases/latest/download/SHA256SUMS.p7s
$ curl -LO https://github.com/pluries/venom/releases/latest/download/venom.linux-amd64
```

## Verificare il firmatario {#check-the-signer}

Estrai il certificato che ha firmato l'elenco e stampa la sua impronta SHA-256:

```bash
$ openssl cms -verify -inform DER -in SHA256SUMS.p7s -content SHA256SUMS -binary -noverify -signer signer.pem -out /dev/null
$ openssl x509 -in signer.pem -outform DER | sha256sum
```

L'impronta del firmatario di ogni release da `v1.5.0-pluries.2` in poi è:

```
42632819c286ec58a829fea06323c5e53772982d7eaff3d2756bd997527a54f2
```

Se il risultato è diverso, fermati: l'elenco non è stato firmato con il certificato del maintainer.

## Verificare l'elenco e il file {#check-the-list-and-the-file}

```bash
$ openssl cms -verify -inform DER -in SHA256SUMS.p7s -content SHA256SUMS -binary -purpose any
$ sha256sum -c SHA256SUMS --ignore-missing
```

Il primo comando prova che l'elenco è firmato e non è stato modificato. Il secondo prova che il tuo eseguibile è quello dell'elenco.

## Lasciare fare alla skill venom-pluries {#let-the-venom-pluries-skill-do-it}

La skill `venom-pluries` per Claude Code contiene uno script che esegue tutti questi controlli, confronta il firmatario con l'impronta qui sopra e rifiuta di installare qualsiasi cosa se un controllo fallisce. Usala quando aggiungi venom a un progetto.
