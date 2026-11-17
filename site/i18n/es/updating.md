---
translates: updating
title: Actualización
description: Cómo actualizar venom.
fingerprint: a0c3a4ff4fa192302942a56576ce1f0f240ffb1450dbbc780aeb963709181873
drafted_by: claude
drafted_on: 2026-11-16
status: reviewed
reviewed_by: Christian Stephan
reviewed_on: 2026-11-16
reviewed_fingerprint: a0c3a4ff4fa192302942a56576ce1f0f240ffb1450dbbc780aeb963709181873
---
# Actualización {#updating}

Este fork no tiene un comando `venom update`. El comando de upstream descargaba la última versión del proyecto original
sin comprobar ninguna suma de verificación ni firma (#2296). Para actualizar, descargue el nuevo ejecutable desde
https://github.com/pluries/venom/releases/latest y [verifíquelo](#verify-the-download), o compile usted mismo la etiqueta (`go build ./cmd/venom`).
Después compruebe la versión:

```bash
$ venom version
```
