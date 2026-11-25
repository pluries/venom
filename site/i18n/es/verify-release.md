---
translates: verify-release
title: Verificar una versión
description: Compruebe que una versión está firmada con el certificado del mantenedor.
fingerprint: 11141e8943d763bea06a3462db682b51480988aa7bd53a6b320de816db989244
drafted_by: claude
drafted_on: 2026-11-24
status: machine-drafted
---
Cada versión de este fork está firmada, y puede comprobar la firma sin fiarse de la página de descargas. Esta página muestra cómo demostrar que una versión es la del mantenedor: que la lista de sumas de verificación está firmada, que el firmante es el certificado del mantenedor y que su archivo figura en la lista. Necesita `openssl` 3 o posterior, `sha256sum` y `curl`. LibreSSL, que se entrega con macOS, no puede comprobar la marca de tiempo de la firma: instale antes OpenSSL 3 en ese sistema.

Descargue la lista de sumas de verificación, su firma y el ejecutable que quiera:

```bash
$ curl -LO https://github.com/pluries/venom/releases/latest/download/SHA256SUMS
$ curl -LO https://github.com/pluries/venom/releases/latest/download/SHA256SUMS.p7s
$ curl -LO https://github.com/pluries/venom/releases/latest/download/venom.linux-amd64
```

## Comprobar el firmante {#check-the-signer}

Extraiga el certificado que firmó la lista e imprima su huella SHA-256:

```bash
$ openssl cms -verify -inform DER -in SHA256SUMS.p7s -content SHA256SUMS -binary -noverify -signer signer.pem -out /dev/null
$ openssl x509 -in signer.pem -outform DER | sha256sum
```

La huella del firmante de cada versión desde `v1.5.0-pluries.2` es:

```
42632819c286ec58a829fea06323c5e53772982d7eaff3d2756bd997527a54f2
```

Si su resultado es distinto, deténgase: la lista no fue firmada con el certificado del mantenedor.

## Comprobar la lista y el archivo {#check-the-list-and-the-file}

```bash
$ openssl cms -verify -inform DER -in SHA256SUMS.p7s -content SHA256SUMS -binary -purpose any
$ sha256sum -c SHA256SUMS --ignore-missing
```

El primer comando demuestra que la lista está firmada y no ha cambiado. El segundo demuestra que su ejecutable es el de la lista.

## Dejar que lo haga la skill venom-pluries {#let-the-venom-pluries-skill-do-it}

La skill `venom-pluries` para Claude Code incluye un script que ejecuta todas estas comprobaciones, compara el firmante con la huella de arriba y se niega a instalar nada si alguna falla. Úsela cuando añada venom a un proyecto.
