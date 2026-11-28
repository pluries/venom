---
translates: skill
title: Usarla da Claude Code
description: Installate la skill venom-pluries perché Claude Code configuri Venom Pluries nel vostro progetto.
fingerprint: d7f89518bebf2fcf0f7a15898d97748d2257db0cefe23caabcf8a0dd26d922b6
drafted_by: claude
drafted_on: 2026-11-27
status: machine-drafted
---
La skill `venom-pluries` permette a Claude Code di configurare Venom Pluries nel vostro progetto. Installa una versione verificata, scrive una prima suite di test, aggiunge il passaggio alla vostra build e controlla le ricette per le pagine web e mobili. Voi approvate ogni comando che vuole eseguire.

## Installarla {#install-it}

Indicate un tag di versione. Non installate mai dal ramo predefinito, che si muove.

```bash
$ npx skills add pluries/venom@skill-v1.0.1 --skill venom-pluries
```

Lo strumento chiede per quale agente installare: scegliete Claude Code. Mette la skill nella cartella `venom-pluries` (sotto `skills`, dentro `.claude`) del vostro progetto e scrive accanto un file `skills-lock.json`. Lo strumento invia dati d'uso anonimi, a meno che prima non impostiate `DISABLE_TELEMETRY=1` (oppure `DO_NOT_TRACK=1`).

Le skill girano con i permessi del vostro agente. Leggete il `SKILL.md` della cartella installata prima di usarla. La skill mostra ogni comando e aspetta la vostra approvazione, e non vi chiede mai di disattivare le approvazioni.

Preferite vedere cosa viene copiato? Clonate il tag e copiate la cartella voi stessi:

```bash
$ git clone --depth 1 --branch skill-v1.0.1 https://github.com/pluries/venom venom-skill
```

Copiate la cartella `skills/venom-pluries/` di quel clone nella cartella `venom-pluries` (sotto `skills`, dentro `.claude`) del vostro progetto, poi eliminate il clone.

## Verificare che ci sia {#check-that-it-is-there}

Avviate Claude Code nel progetto e chiedete quali skill ha: `venom-pluries` è nell'elenco. La riga `Skill version` in cima al suo `SKILL.md` indica la versione, che corrisponde al tag scelto.

## Usarla {#use-it}

Chiedete con parole vostre, per esempio «configura venom in questo progetto e scrivi un primo test», «eseguilo in GitHub Actions» o «controlla la mia ricetta». La skill installa la versione di venom a cui punta (scritta nel suo `SKILL.md` e nel vostro `.venom-version`), controlla somma di controllo, firma e firmatario prima di eseguire qualsiasi cosa, e vi dice cosa ha fatto e cosa no.

## Cosa i manutentori non hanno eseguito {#what-the-maintainers-did-not-run}

Il file GitLab CI non è mai stato eseguito su un vero server GitLab. La skill lo dice, e la prima esecuzione è vostra. Per le ricette mobili la skill non installa né avvia un host di dispositivi: voi fornite un server Appium e un emulatore o un dispositivo.

## Aggiornarla {#update-it}

Installate il nuovo tag nello stesso modo. Se il `SIGNER_SHA256` in `release.env` differisce da quello del vostro progetto, il certificato di firma è stato rinnovato. La skill vi dice allora di copiare i nuovi script, con la differenza mostrata.
