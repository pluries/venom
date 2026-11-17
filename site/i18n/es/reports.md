---
translates: reports
title: Exportar el informe de pruebas
description: Cómo exportar los resultados en JSON, XML o HTML.
fingerprint: ddb2c03f51333e036451a070d587b91fd37fddc6fe592bb51e66a08e1e6e6454
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# Exportar el informe de pruebas {#export-tests-report}

Puede exportar los resultados de su suite de pruebas como un informe en varios formatos disponibles: xUnit (XML), JSON, YAML, TAP.

Puede indicar el directorio de salida con el indicador `--output-dir` y el formato con el indicador `--format` (XML de forma predeterminada):

```bash
$ venom run --format=xml --output-dir="."

# html export
$ venom run --output-dir="." --html-report
```

Los informes exportados en XML pueden visualizarse con un visor xUnit/jUnit, por ejemplo directamente en su plataforma de CI/CD favorita, para ver los resultados ejecución tras ejecución.
