---
translates: installing
title: Installation
description: Comment installer venom.
fingerprint: 93d69cbe8c61d9eee2f2f904a568c6e04dc04f75ad107145a0ef688e8851a310
drafted_by: claude
drafted_on: 2026-11-16
status: reviewed
reviewed_by: Christian Stephan
reviewed_on: 2026-11-16
reviewed_fingerprint: 93d69cbe8c61d9eee2f2f904a568c6e04dc04f75ad107145a0ef688e8851a310
---
# Installation {#installing}

## Installer depuis les binaires {#install-from-binaries}

Chaque release de ce fork est publiée sur https://github.com/pluries/venom/releases/latest. Elle contient un exécutable par plateforme :
`venom.linux-amd64`, `venom.linux-arm64`, `venom.linux-arm`, `venom.linux-386`, `venom.darwin-amd64`, `venom.darwin-arm64`,
`venom.windows-amd64.exe`, `venom.freebsd-amd64` et `venom.openbsd-amd64`, ainsi que la liste de sommes de contrôle `SHA256SUMS` et sa signature `SHA256SUMS.p7s`.

Exemple pour Linux :

```bash
$ curl -L -o venom https://github.com/pluries/venom/releases/latest/download/venom.linux-amd64
$ chmod +x venom && sudo mv venom /usr/local/bin/venom
$ venom -h
```

### Vérifier le téléchargement {#verify-the-download}

La liste de sommes de contrôle est signée avec un certificat de signature de code Certum, et l'exécutable Windows porte en plus une signature Authenticode.
Il vous faut `sha256sum` et `openssl`, rien d'autre :

```bash
$ curl -LO https://github.com/pluries/venom/releases/latest/download/SHA256SUMS
$ curl -LO https://github.com/pluries/venom/releases/latest/download/SHA256SUMS.p7s
$ sha256sum -c SHA256SUMS --ignore-missing
$ openssl cms -verify -inform DER -in SHA256SUMS.p7s -content SHA256SUMS -binary -purpose any
```

La première vérification compare votre fichier à la liste, la seconde prouve que la liste a été signée par le certificat du mainteneur.
