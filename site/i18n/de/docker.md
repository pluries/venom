---
translates: docker
title: Docker-Image
description: So führen Sie venom aus einem Docker-Image aus.
fingerprint: ba838efdb064cdb08914baa943a79efe02f608659459b8827883285d36c7e84f
drafted_by: claude
drafted_on: 2026-11-16
status: reviewed
reviewed_by: Christian Stephan
reviewed_on: 2026-11-16
reviewed_fingerprint: ba838efdb064cdb08914baa943a79efe02f608659459b8827883285d36c7e84f
---
# Docker-Image {#docker-image}

Statt Venom lokal zu installieren (und zu aktualisieren), können Sie es mit den folgenden Befehlen als Docker-Image starten.
Das Image dieses Forks heißt `ghcr.io/pluries/venom`.

Angenommen, Ihre Testsuites liegen im Verzeichnis `./tests` Ihres aktuellen Verzeichnisses und Ihre Testbibliothek unter `./tests/lib`, dann stehen die Ergebnisse im Verzeichnis `results` bereit.

```bash
$ mkdir -p results
$ docker run --mount type=bind,source=$(pwd)/tests,target=/workdir/tests --mount type=bind,source=$(pwd)/results,target=/workdir/results ghcr.io/pluries/venom:latest
```

Jedes Release hat außerdem ein eigenes Image-Tag, zum Beispiel `ghcr.io/pluries/venom:v1.5.0-pluries.1`. Verwenden Sie in einer Pipeline ein Release-Tag, damit sich das Image nicht unter Ihnen ändert.
