---
translates: reports
title: Esportare il report dei test
description: Come esportare i risultati in JSON, XML o HTML.
fingerprint: ddb2c03f51333e036451a070d587b91fd37fddc6fe592bb51e66a08e1e6e6454
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# Esportare il report dei test {#export-tests-report}

Puoi esportare i risultati delle tue testsuite come report in diversi formati disponibili: xUnit (XML), JSON, YAML, TAP.

Puoi indicare la directory di output con il flag `--output-dir` e il formato con il flag `--format` (XML per impostazione predefinita):

```bash
$ venom run --format=xml --output-dir="."

# html export
$ venom run --output-dir="." --html-report
```

I report esportati in XML si possono visualizzare con un viewer xUnit/jUnit, direttamente nel tuo stack CI/CD preferito, per esempio per vedere i risultati esecuzione dopo esecuzione.
