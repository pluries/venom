---
translates: updating
title: Aggiornamento
description: Come aggiornare venom.
fingerprint: a0c3a4ff4fa192302942a56576ce1f0f240ffb1450dbbc780aeb963709181873
drafted_by: claude
drafted_on: 2026-11-16
status: reviewed
reviewed_by: Christian Stephan
reviewed_on: 2026-11-16
reviewed_fingerprint: a0c3a4ff4fa192302942a56576ce1f0f240ffb1450dbbc780aeb963709181873
---
# Aggiornamento {#updating}

Questo fork non ha il comando `venom update`. Il comando originale scaricava l'ultima release del progetto originale
senza verificare né un checksum né una firma (#2296). Per aggiornare, scarica il nuovo eseguibile da
https://github.com/pluries/venom/releases/latest e [verificalo](#verify-the-download), oppure compila tu stesso il tag (`go build ./cmd/venom`).
Poi controlla la versione:

```bash
$ venom version
```
