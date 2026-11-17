---
translates: reports
title: Testbericht exportieren
description: So exportieren Sie Ergebnisse als JSON, XML oder HTML.
fingerprint: ddb2c03f51333e036451a070d587b91fd37fddc6fe592bb51e66a08e1e6e6454
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# Testbericht exportieren {#export-tests-report}

Sie können die Ergebnisse Ihrer Testsuites als Bericht in mehreren verfügbaren Formaten exportieren: xUnit (XML), JSON, YAML, TAP.

Mit dem Flag `--output-dir` legen Sie das Ausgabeverzeichnis fest, mit dem Flag `--format` das Format (standardmäßig XML):

```bash
$ venom run --format=xml --output-dir="."

# html export
$ venom run --output-dir="." --html-report
```

Berichte im XML-Format lassen sich mit einem xUnit/jUnit-Viewer anzeigen, zum Beispiel direkt in Ihrem bevorzugten CI/CD-System, um die Ergebnisse von Lauf zu Lauf zu verfolgen.
