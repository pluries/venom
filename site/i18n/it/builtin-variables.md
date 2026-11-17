---
translates: builtin-variables
title: Variabili integrate di venom
description: Le variabili che venom imposta per te.
fingerprint: 57868302669e41f5bae786f964d1e4ad3735ca9dcc92970c85f06a769329fbae
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# Variabili integrate di venom {#builtin-venom-variables}

```yaml
name: MyTestSuite
testcases:
- name: testA
  steps:
  - type: exec
    script: echo '{{.venom.testsuite}} {{.venom.testsuite.filename}} {{.venom.testcase}} {{.venom.teststep.number}} {{.venom.datetime}} {{.venom.timestamp}} {{.venom.testcase.totalSteps}} {{.venom.testsuite.totalSteps}}'
    # will display something as: MyTestSuite MyTestSuiteWithVenomBuiltinVar.yml testA 0 2018-08-05T21:38:24+02:00 1533497904 3 5

```

Variabili integrate:

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


### Variabili segrete {#secrets-variables}

Esempio:

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

Il valore `this-value-is-secret` non verrà stampato nella console, in `venom.log` né nei file `...dump.json`.

#### Cosa viene nascosto e cosa no {#what-is-hidden-and-what-is-not}

Venom nasconde un segreto in base al suo valore, ovunque l'esecuzione lo stamperebbe o lo scriverebbe: console, `venom.log`, report e file `...dump.json`, nell'output degli step, nei nomi, nei testi delle assertion e nei messaggi di errore.

Nascosto:

- ogni valore dichiarato in `secrets:` (nella suite) o in `secrets` di `.venomrc`;
- la forma base64 di quel valore e, per `basic_auth_password`, il base64 di `user:password`;
- la forma slug che venom gli dà dentro il nome di un test case (solo quando è lunga almeno 4 caratteri);
- le forme codificate per URL: query (`a+b%21`) e percorso (`a%20b%21`), quando differiscono dal valore e sono lunghe almeno 4 caratteri.

Non nascosto:

- un valore che non hai dichiarato, anche quando proviene da una fonte segreta;
- qualsiasi altra trasformazione: esadecimale, entità HTML, un hash, una firma, un valore spezzato in più parti o unito ad altro testo, una forma con escape JSON.

Quando uno step può stampare un segreto in un'altra forma, evita di stamparlo, oppure dichiara in `secrets:` anche il valore trasformato.
