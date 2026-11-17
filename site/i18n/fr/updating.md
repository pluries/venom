---
translates: updating
title: Mise à jour
description: Comment mettre à jour venom.
fingerprint: a0c3a4ff4fa192302942a56576ce1f0f240ffb1450dbbc780aeb963709181873
drafted_by: claude
drafted_on: 2026-11-16
status: reviewed
reviewed_by: Christian Stephan
reviewed_on: 2026-11-16
reviewed_fingerprint: a0c3a4ff4fa192302942a56576ce1f0f240ffb1450dbbc780aeb963709181873
---
# Mise à jour {#updating}

Ce fork n'a pas de commande `venom update`. La commande d'origine téléchargeait la dernière release du projet amont
sans vérifier de somme de contrôle ni de signature (#2296). Pour mettre à jour, téléchargez le nouvel exécutable depuis
https://github.com/pluries/venom/releases/latest et [vérifiez-le](#verify-the-download), ou compilez vous-même le tag (`go build ./cmd/venom`).
Vérifiez ensuite la version :

```bash
$ venom version
```
