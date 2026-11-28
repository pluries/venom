---
translates: skill
title: Aus Claude Code nutzen
description: Installieren Sie den Skill venom-pluries, damit Claude Code Venom Pluries in Ihrem Projekt einrichtet.
fingerprint: ee9619c51479fe441be081d380614b2cd7bc62c1c2f86e5b8c338448a471a5ba
drafted_by: claude
drafted_on: 2026-11-27
status: machine-drafted
---
Der Skill `venom-pluries` lässt Claude Code Venom Pluries in Ihrem Projekt einrichten. Er installiert eine geprüfte Version, schreibt eine erste Testsuite, fügt den Schritt zu Ihrem Build hinzu und prüft Rezepte für Web- und Mobilseiten. Sie genehmigen jeden Befehl, den er ausführen will.

## Installieren {#install-it}

Nennen Sie ein Release-Tag. Installieren Sie nie vom Standardzweig, der sich ständig ändert.

```bash
$ npx skills add pluries/venom@skill-v1.0.0 --skill venom-pluries
```

Das Werkzeug fragt, für welchen Agenten installiert werden soll; wählen Sie Claude Code. Es legt den Skill im Ordner `venom-pluries` (unter `skills`, innerhalb von `.claude`) Ihres Projekts ab und schreibt daneben eine Datei `skills-lock.json`. Das Werkzeug sendet anonyme Nutzungsdaten, außer Sie setzen vorher `DISABLE_TELEMETRY=1` (oder `DO_NOT_TRACK=1`).

Skills laufen mit den Rechten Ihres Agenten. Lesen Sie die `SKILL.md` im installierten Ordner, bevor Sie ihn benutzen. Der Skill zeigt jeden Befehl und wartet auf Ihre Genehmigung; er bittet Sie nie, Genehmigungen abzuschalten.

Möchten Sie sehen, was kopiert wird? Klonen Sie das Tag und kopieren Sie den Ordner selbst:

```bash
$ git clone --depth 1 --branch skill-v1.0.0 https://github.com/pluries/venom venom-skill
```

Kopieren Sie den Ordner `skills/venom-pluries/` dieses Klons in den Ordner `venom-pluries` (unter `skills`, innerhalb von `.claude`) Ihres Projekts und löschen Sie danach den Klon.

## Prüfen, dass er da ist {#check-that-it-is-there}

Starten Sie Claude Code im Projekt und fragen Sie, welche Skills es hat: `venom-pluries` steht in der Liste. Die Zeile `Skill version` oben in seiner `SKILL.md` nennt die Version; sie entspricht dem gewählten Tag.

## Benutzen {#use-it}

Fragen Sie mit eigenen Worten, zum Beispiel „richte venom in diesem Projekt ein und schreibe einen ersten Test“, „führe es in GitHub Actions aus“ oder „prüfe mein Rezept“. Der Skill installiert die Venom-Version, auf die er zielt (steht in seiner `SKILL.md` und in Ihrer `.venom-version`), prüft Prüfsumme, Signatur und Unterzeichner, bevor er etwas ausführt, und sagt Ihnen, was er getan und was er nicht getan hat.

## Was die Maintainer nicht ausgeführt haben {#what-the-maintainers-did-not-run}

Die GitLab-CI-Datei wurde nie auf einem echten GitLab-Server ausgeführt. Der Skill sagt das, und der erste Lauf ist Ihrer. Für mobile Rezepte installiert oder startet der Skill keinen Geräte-Host: Sie stellen einen Appium-Server und einen Emulator oder ein Gerät bereit.

## Aktualisieren {#update-it}

Installieren Sie das neue Tag auf dieselbe Weise. Weicht das `SIGNER_SHA256` in `release.env` von dem Ihres Projekts ab, wurde das Signaturzertifikat erneuert. Der Skill sagt Ihnen dann, die neuen Skripte zu kopieren, mit angezeigtem Unterschied.
