---
translates: installing
title: Installation
description: Comment installer venom.
fingerprint: 6b24ed9fedcdd72c1c379a6a9fa397e4c41b8332cfcf676a4cae3eca8d6e997e
drafted_by: claude
drafted_on: 2026-11-24
status: reviewed
reviewed_by: Christian Stephan
reviewed_on: 2026-11-25
reviewed_fingerprint: 6b24ed9fedcdd72c1c379a6a9fa397e4c41b8332cfcf676a4cae3eca8d6e997e
---
# Installation {#installing}

## Installer depuis les binaires {#install-from-binaries}

Chaque release de ce fork est publiée sur https://github.com/pluries/venom/releases/latest. Elle contient un exécutable par plateforme :
`venom.linux-amd64`, `venom.linux-arm64`, `venom.linux-arm`, `venom.linux-386`, `venom.darwin-amd64`, `venom.darwin-arm64`,
`venom.linux-ppc64le`, `venom.windows-amd64.exe`, `venom.freebsd-amd64` et `venom.openbsd-amd64`, ainsi que la liste de sommes de contrôle `SHA256SUMS`, sa signature `SHA256SUMS.p7s` et le jeton d'horodatage de cette signature, `SHA256SUMS.p7s.tsr`.

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
