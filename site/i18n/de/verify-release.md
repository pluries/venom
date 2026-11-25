---
translates: verify-release
title: Ein Release prüfen
description: Prüfen Sie, ob ein Release mit dem Zertifikat des Maintainers signiert ist.
fingerprint: 11141e8943d763bea06a3462db682b51480988aa7bd53a6b320de816db989244
drafted_by: claude
drafted_on: 2026-11-24
status: machine-drafted
---
Jedes Release dieses Forks ist signiert, und Sie können die Signatur prüfen, ohne der Download-Seite zu vertrauen. Diese Seite zeigt, wie Sie belegen, dass ein Release vom Maintainer stammt: dass die Prüfsummenliste signiert ist, dass der Unterzeichner das Zertifikat des Maintainers ist und dass Ihre Datei auf der Liste steht. Sie brauchen `openssl` 3 oder neuer, `sha256sum` und `curl`. Das mit macOS gelieferte LibreSSL kann den Zeitstempel der Signatur nicht prüfen, installieren Sie dort zuerst OpenSSL 3.

Laden Sie die Prüfsummenliste, ihre Signatur und die gewünschte ausführbare Datei herunter:

```bash
$ curl -LO https://github.com/pluries/venom/releases/latest/download/SHA256SUMS
$ curl -LO https://github.com/pluries/venom/releases/latest/download/SHA256SUMS.p7s
$ curl -LO https://github.com/pluries/venom/releases/latest/download/venom.linux-amd64
```

## Den Unterzeichner prüfen {#check-the-signer}

Extrahieren Sie das Zertifikat, das die Liste signiert hat, und geben Sie seinen SHA-256-Fingerabdruck aus:

```bash
$ openssl cms -verify -inform DER -in SHA256SUMS.p7s -content SHA256SUMS -binary -noverify -signer signer.pem -out /dev/null
$ openssl x509 -in signer.pem -outform DER | sha256sum
```

Der Fingerabdruck des Unterzeichners jedes Releases ab `v1.5.0-pluries.2` lautet:

```
42632819c286ec58a829fea06323c5e53772982d7eaff3d2756bd997527a54f2
```

Weicht Ihr Ergebnis ab, brechen Sie ab: Die Liste wurde nicht mit dem Zertifikat des Maintainers signiert.

## Die Liste und die Datei prüfen {#check-the-list-and-the-file}

```bash
$ openssl cms -verify -inform DER -in SHA256SUMS.p7s -content SHA256SUMS -binary -purpose any
$ sha256sum -c SHA256SUMS --ignore-missing
```

Der erste Befehl belegt, dass die Liste signiert und unverändert ist. Der zweite belegt, dass Ihre ausführbare Datei die auf der Liste ist.

## Das Skill venom-pluries übernehmen lassen {#let-the-venom-pluries-skill-do-it}

Das Skill `venom-pluries` für Claude Code enthält ein Skript, das alle diese Prüfungen ausführt, den Unterzeichner mit dem obigen Fingerabdruck vergleicht und nichts installiert, wenn eine Prüfung fehlschlägt. Verwenden Sie es, wenn Sie venom zu einem Projekt hinzufügen.
