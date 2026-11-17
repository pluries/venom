---
translates: updating
title: Aktualisierung
description: So aktualisieren Sie venom.
fingerprint: a0c3a4ff4fa192302942a56576ce1f0f240ffb1450dbbc780aeb963709181873
drafted_by: claude
drafted_on: 2026-11-16
status: reviewed
reviewed_by: Christian Stephan
reviewed_on: 2026-11-16
reviewed_fingerprint: a0c3a4ff4fa192302942a56576ce1f0f240ffb1450dbbc780aeb963709181873
---
# Aktualisierung {#updating}

Dieser Fork hat keinen Befehl `venom update`. Der Upstream-Befehl lud das neueste Release des Upstream-Projekts herunter,
ohne eine Prüfsumme oder Signatur zu prüfen (#2296). Zum Aktualisieren laden Sie die neue ausführbare Datei von
https://github.com/pluries/venom/releases/latest herunter und [prüfen sie](#verify-the-download) oder bauen den Tag selbst (`go build ./cmd/venom`).
Prüfen Sie danach die Version:

```bash
$ venom version
```
