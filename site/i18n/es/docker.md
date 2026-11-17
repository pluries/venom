---
translates: docker
title: Imagen Docker
description: Cómo ejecutar venom desde una imagen Docker.
fingerprint: ba838efdb064cdb08914baa943a79efe02f608659459b8827883285d36c7e84f
drafted_by: claude
drafted_on: 2026-11-16
status: reviewed
reviewed_by: Christian Stephan
reviewed_on: 2026-11-16
reviewed_fingerprint: ba838efdb064cdb08914baa943a79efe02f608659459b8827883285d36c7e84f
---
# Imagen Docker {#docker-image}

En lugar de instalar (y actualizar) Venom localmente, puede iniciarlo como una imagen Docker con los siguientes comandos.
La imagen de este fork es `ghcr.io/pluries/venom`.

Suponiendo que sus suites de pruebas (testsuites) están en el directorio `./tests` de su directorio actual y que su biblioteca de pruebas está en `./tests/lib`, los resultados estarán disponibles en el directorio `results`.

```bash
$ mkdir -p results
$ docker run --mount type=bind,source=$(pwd)/tests,target=/workdir/tests --mount type=bind,source=$(pwd)/results,target=/workdir/results ghcr.io/pluries/venom:latest
```

Cada versión tiene además su propia etiqueta de imagen, por ejemplo `ghcr.io/pluries/venom:v1.5.0-pluries.1`. Use una etiqueta de versión en un pipeline para que la imagen no cambie sin que usted lo note.
