---
translates: installing
title: Installation
description: So installieren Sie venom.
fingerprint: 6b24ed9fedcdd72c1c379a6a9fa397e4c41b8332cfcf676a4cae3eca8d6e997e
drafted_by: claude
drafted_on: 2026-11-24
status: reviewed
reviewed_by: Christian Stephan
reviewed_on: 2026-11-25
reviewed_fingerprint: 6b24ed9fedcdd72c1c379a6a9fa397e4c41b8332cfcf676a4cae3eca8d6e997e
---
# Installation {#installing}

## Aus Binärdateien installieren {#install-from-binaries}

Jedes Release dieses Forks wird unter https://github.com/pluries/venom/releases/latest veröffentlicht. Es enthält eine ausführbare Datei pro Plattform:
`venom.linux-amd64`, `venom.linux-arm64`, `venom.linux-arm`, `venom.linux-386`, `venom.darwin-amd64`, `venom.darwin-arm64`,
`venom.linux-ppc64le`, `venom.windows-amd64.exe`, `venom.freebsd-amd64` und `venom.openbsd-amd64`, dazu die Prüfsummenliste `SHA256SUMS`, ihre Signatur `SHA256SUMS.p7s` und das Zeitstempel-Token dieser Signatur, `SHA256SUMS.p7s.tsr`.

Beispiel für Linux:

```bash
$ curl -L -o venom https://github.com/pluries/venom/releases/latest/download/venom.linux-amd64
$ chmod +x venom && sudo mv venom /usr/local/bin/venom
$ venom -h
```

### Den Download prüfen {#verify-the-download}

Die Prüfsummenliste ist mit einem Code-Signing-Zertifikat von Certum signiert, und die Windows-Datei trägt zusätzlich eine Authenticode-Signatur.
Sie brauchen `sha256sum` und `openssl`, sonst nichts:

```bash
$ curl -LO https://github.com/pluries/venom/releases/latest/download/SHA256SUMS
$ curl -LO https://github.com/pluries/venom/releases/latest/download/SHA256SUMS.p7s
$ sha256sum -c SHA256SUMS --ignore-missing
$ openssl cms -verify -inform DER -in SHA256SUMS.p7s -content SHA256SUMS -binary -purpose any
```

Die erste Prüfung vergleicht Ihre Datei mit der Liste, die zweite beweist, dass die Liste mit dem Zertifikat der Maintainerin oder des Maintainers signiert wurde.
