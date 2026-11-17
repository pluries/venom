---
translates: builtin-variables
title: Eingebaute venom-Variablen
description: Die Variablen, die venom für Sie setzt.
fingerprint: 57868302669e41f5bae786f964d1e4ad3735ca9dcc92970c85f06a769329fbae
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# Eingebaute venom-Variablen {#builtin-venom-variables}

```yaml
name: MyTestSuite
testcases:
- name: testA
  steps:
  - type: exec
    script: echo '{{.venom.testsuite}} {{.venom.testsuite.filename}} {{.venom.testcase}} {{.venom.teststep.number}} {{.venom.datetime}} {{.venom.timestamp}} {{.venom.testcase.totalSteps}} {{.venom.testsuite.totalSteps}}'
    # will display something as: MyTestSuite MyTestSuiteWithVenomBuiltinVar.yml testA 0 2018-08-05T21:38:24+02:00 1533497904 3 5

```

Eingebaute Variablen:

* {{.venom.datetime}}
* {{.venom.executable}}
* {{.venom.libdir}}
* {{.venom.outputdir}}
* {{.venom.testcase}}
* {{.venom.testcase.totalSteps}}
* {{.venom.teststep.number}}
* {{.venom.testsuite.name}}
* {{.venom.testsuite.filename}}
* {{.venom.testsuite.filepath}}
* {{.venom.testsuite.shortName}}
* {{.venom.testsuite.workdir}}
* {{.venom.testsuite}}
* {{.venom.testsuite.totalSteps}}
* {{.venom.timestamp}}


### Geheime Variablen {#secrets-variables}

Beispiel:

```yml
name: Your Testsuite
vars:
  foo : this-value-is-secret
secrets:
  - foo
testcases:
- name: myvar_first
  steps:
  - type: exec
    script: "echo myvar {{.foo}}"
```

Der Wert `this-value-is-secret` wird weder in Ihrer Konsole noch in `venom.log` oder in den Dateien `...dump.json` ausgegeben.

#### Was verborgen wird und was nicht {#what-is-hidden-and-what-is-not}

Venom verbirgt ein Geheimnis anhand seines Werts, überall dort, wo der Lauf es ausgeben oder schreiben würde: in der Konsole, in `venom.log`, in den Berichten und in den Dateien `...dump.json`, in der Ausgabe von Schritten, in Namen, in Assertion-Texten und in Fehlermeldungen.

Verborgen werden:

- jeder Wert, der in `secrets:` (Suite) oder in `secrets` der Datei `.venomrc` deklariert ist;
- die Base64-Form dieses Werts, und bei `basic_auth_password` die Base64-Form von `user:password`;
- die Slug-Form, die venom ihm innerhalb eines Testcase-Namens gibt (nur wenn er mindestens 4 Zeichen lang ist);
- die URL-kodierten Formen: Query (`a+b%21`) und Pfad (`a%20b%21`), wenn sie sich vom Wert unterscheiden und mindestens 4 Zeichen lang sind.

Nicht verborgen werden:

- ein Wert, den Sie nicht deklariert haben, auch wenn er aus einer geheimen Quelle stammt;
- jede andere Umwandlung: Hex, HTML-Entities, ein Hash, eine Signatur, ein in Teile zerlegter oder mit anderem Text verbundener Wert, eine JSON-maskierte Form.

Wenn ein Schritt ein Geheimnis in einer anderen Form ausgeben kann, vermeiden Sie die Ausgabe oder deklarieren Sie den umgewandelten Wert zusätzlich in `secrets:`.
