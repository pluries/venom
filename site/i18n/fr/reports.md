---
translates: reports
title: Exporter le rapport de tests
description: Comment exporter les résultats en JSON, XML ou HTML.
fingerprint: ddb2c03f51333e036451a070d587b91fd37fddc6fe592bb51e66a08e1e6e6454
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# Exporter le rapport de tests {#export-tests-report}

Vous pouvez exporter les résultats de vos suites de tests sous forme de rapport dans plusieurs formats disponibles : xUnit (XML), JSON, YAML, TAP.

Vous pouvez indiquer le répertoire de sortie avec l'option `--output-dir` et le format avec l'option `--format` (XML par défaut) :

```bash
$ venom run --format=xml --output-dir="."

# html export
$ venom run --output-dir="." --html-report
```

Les rapports exportés en XML peuvent être visualisés avec un visualiseur xUnit/jUnit, directement dans votre chaîne CI/CD préférée par exemple, afin de suivre les résultats d'exécution en exécution.
