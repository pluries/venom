---
translates: docker
title: Image Docker
description: Comment exécuter venom depuis une image Docker.
fingerprint: ba838efdb064cdb08914baa943a79efe02f608659459b8827883285d36c7e84f
drafted_by: claude
drafted_on: 2026-11-16
status: reviewed
reviewed_by: Christian Stephan
reviewed_on: 2026-11-16
reviewed_fingerprint: ba838efdb064cdb08914baa943a79efe02f608659459b8827883285d36c7e84f
---
# Image Docker {#docker-image}

Au lieu d'installer (et de mettre à jour) Venom en local, vous pouvez le lancer sous forme d'image Docker avec les commandes suivantes.
L'image de ce fork est `ghcr.io/pluries/venom`.

Dans l'hypothèse où vos testsuites se trouvent dans le dossier `./tests` de votre répertoire courant et votre bibliothèque de tests sous `./tests/lib`, les résultats seront disponibles dans le dossier `results`.

```bash
$ mkdir -p results
$ docker run --mount type=bind,source=$(pwd)/tests,target=/workdir/tests --mount type=bind,source=$(pwd)/results,target=/workdir/results ghcr.io/pluries/venom:latest
```

Chaque release a aussi sa propre étiquette d'image, par exemple `ghcr.io/pluries/venom:v1.5.0-pluries.1`. Utilisez une étiquette de release dans un pipeline pour que l'image ne change pas sous vos pieds.
