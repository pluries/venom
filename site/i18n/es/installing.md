---
translates: installing
title: Instalación
description: Cómo instalar venom.
fingerprint: 6b24ed9fedcdd72c1c379a6a9fa397e4c41b8332cfcf676a4cae3eca8d6e997e
drafted_by: claude
drafted_on: 2026-11-24
status: reviewed
reviewed_by: Christian Stephan
reviewed_on: 2026-11-25
reviewed_fingerprint: 6b24ed9fedcdd72c1c379a6a9fa397e4c41b8332cfcf676a4cae3eca8d6e997e
---
# Instalación {#installing}

## Instalar desde binarios {#install-from-binaries}

Cada versión de este fork se publica en https://github.com/pluries/venom/releases/latest. Contiene un ejecutable por plataforma:
`venom.linux-amd64`, `venom.linux-arm64`, `venom.linux-arm`, `venom.linux-386`, `venom.darwin-amd64`, `venom.darwin-arm64`,
`venom.linux-ppc64le`, `venom.windows-amd64.exe`, `venom.freebsd-amd64` y `venom.openbsd-amd64`, además de la lista de sumas de verificación `SHA256SUMS`, su firma `SHA256SUMS.p7s` y el token de marca de tiempo de esa firma, `SHA256SUMS.p7s.tsr`.

Ejemplo para Linux:

```bash
$ curl -L -o venom https://github.com/pluries/venom/releases/latest/download/venom.linux-amd64
$ chmod +x venom && sudo mv venom /usr/local/bin/venom
$ venom -h
```

### Verificar la descarga {#verify-the-download}

La lista de sumas de verificación está firmada con un certificado de firma de código de Certum, y el ejecutable de Windows lleva además una firma Authenticode.
Solo necesita `sha256sum` y `openssl`:

```bash
$ curl -LO https://github.com/pluries/venom/releases/latest/download/SHA256SUMS
$ curl -LO https://github.com/pluries/venom/releases/latest/download/SHA256SUMS.p7s
$ sha256sum -c SHA256SUMS --ignore-missing
$ openssl cms -verify -inform DER -in SHA256SUMS.p7s -content SHA256SUMS -binary -purpose any
```

La primera comprobación compara su archivo con la lista; la segunda demuestra que la lista fue firmada con el certificado del mantenedor.
