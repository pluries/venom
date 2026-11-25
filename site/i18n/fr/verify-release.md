---
translates: verify-release
title: Vérifier une version
description: Vérifiez qu'une version est signée par le certificat du mainteneur.
fingerprint: 11141e8943d763bea06a3462db682b51480988aa7bd53a6b320de816db989244
drafted_by: claude
drafted_on: 2026-11-24
status: machine-drafted
---
Chaque version de ce fork est signée, et vous pouvez vérifier la signature sans faire confiance à la page de téléchargement. Cette page montre comment prouver qu'une version est bien celle du mainteneur : que la liste des sommes de contrôle est signée, que le signataire est le certificat du mainteneur et que votre fichier figure sur la liste. Il vous faut `openssl` 3 ou plus récent, `sha256sum` et `curl`. LibreSSL, fourni avec macOS, ne peut pas vérifier l'horodatage de la signature : installez d'abord OpenSSL 3 sur ce système.

Téléchargez la liste des sommes de contrôle, sa signature et l'exécutable voulu :

```bash
$ curl -LO https://github.com/pluries/venom/releases/latest/download/SHA256SUMS
$ curl -LO https://github.com/pluries/venom/releases/latest/download/SHA256SUMS.p7s
$ curl -LO https://github.com/pluries/venom/releases/latest/download/venom.linux-amd64
```

## Vérifier le signataire {#check-the-signer}

Extrayez le certificat qui a signé la liste et affichez son empreinte SHA-256 :

```bash
$ openssl cms -verify -inform DER -in SHA256SUMS.p7s -content SHA256SUMS -binary -noverify -signer signer.pem -out /dev/null
$ openssl x509 -in signer.pem -outform DER | sha256sum
```

L'empreinte du signataire de chaque version à partir de `v1.5.0-pluries.2` est :

```
42632819c286ec58a829fea06323c5e53772982d7eaff3d2756bd997527a54f2
```

Si votre résultat diffère, arrêtez-vous : la liste n'a pas été signée par le certificat du mainteneur.

## Vérifier la liste et le fichier {#check-the-list-and-the-file}

```bash
$ openssl cms -verify -inform DER -in SHA256SUMS.p7s -content SHA256SUMS -binary -purpose any
$ sha256sum -c SHA256SUMS --ignore-missing
```

La première commande prouve que la liste est signée et n'a pas été modifiée. La seconde prouve que votre exécutable est celui de la liste.

## Laisser le skill venom-pluries le faire {#let-the-venom-pluries-skill-do-it}

Le skill `venom-pluries` pour Claude Code contient un script qui exécute toutes ces vérifications, compare le signataire à l'empreinte ci-dessus et refuse d'installer quoi que ce soit en cas d'échec. Utilisez-le pour ajouter venom à un projet.
