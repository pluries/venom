---
translates: docker
title: Immagine Docker
description: Come eseguire venom da un'immagine Docker.
fingerprint: ba838efdb064cdb08914baa943a79efe02f608659459b8827883285d36c7e84f
drafted_by: claude
drafted_on: 2026-11-16
status: reviewed
reviewed_by: Christian Stephan
reviewed_on: 2026-11-16
reviewed_fingerprint: ba838efdb064cdb08914baa943a79efe02f608659459b8827883285d36c7e84f
---
# Immagine Docker {#docker-image}

Invece di installare (e aggiornare) Venom in locale, puoi avviarlo come immagine Docker con i comandi seguenti.
L'immagine di questo fork è `ghcr.io/pluries/venom`.

Supponendo che le tue testsuite siano nella cartella `./tests` della directory corrente e che la tua libreria di test sia in `./tests/lib`, i risultati saranno disponibili nella cartella `results`.

```bash
$ mkdir -p results
$ docker run --mount type=bind,source=$(pwd)/tests,target=/workdir/tests --mount type=bind,source=$(pwd)/results,target=/workdir/results ghcr.io/pluries/venom:latest
```

Ogni release ha anche un proprio tag di immagine, ad esempio `ghcr.io/pluries/venom:v1.5.0-pluries.1`. Usa un tag di release in una pipeline, così l'immagine non cambia sotto i tuoi piedi.
