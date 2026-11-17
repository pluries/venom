---
translates: builtin-variables
title: Variables intégrées de venom
description: Les variables que venom définit pour vous.
fingerprint: 57868302669e41f5bae786f964d1e4ad3735ca9dcc92970c85f06a769329fbae
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# Variables intégrées de venom {#builtin-venom-variables}

```yaml
name: MyTestSuite
testcases:
- name: testA
  steps:
  - type: exec
    script: echo '{{.venom.testsuite}} {{.venom.testsuite.filename}} {{.venom.testcase}} {{.venom.teststep.number}} {{.venom.datetime}} {{.venom.timestamp}} {{.venom.testcase.totalSteps}} {{.venom.testsuite.totalSteps}}'
    # will display something as: MyTestSuite MyTestSuiteWithVenomBuiltinVar.yml testA 0 2018-08-05T21:38:24+02:00 1533497904 3 5

```

Variables intégrées :

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


### Variables secrètes {#secrets-variables}

Exemple :

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

La valeur `this-value-is-secret` ne sera pas affichée dans votre console, ni dans `venom.log` ni dans les fichiers `...dump.json`.

#### Ce qui est masqué, et ce qui ne l'est pas {#what-is-hidden-and-what-is-not}

Venom masque un secret d'après sa valeur, partout où l'exécution l'afficherait ou l'écrirait : console, `venom.log`, rapports et fichiers `...dump.json`, dans la sortie des étapes, les noms, les textes d'assertions et les messages d'erreur.

Masqués :

- toutes les valeurs déclarées dans `secrets:` (suite) ou dans `secrets` de `.venomrc` ;
- la forme base64 de cette valeur et, pour `basic_auth_password`, le base64 de `user:password` ;
- la forme « slug » que venom lui donne dans le nom d'un cas de test (seulement lorsqu'elle compte au moins 4 caractères) ;
- les formes encodées en URL : requête (`a+b%21`) et chemin (`a%20b%21`), lorsqu'elles diffèrent de la valeur et comptent au moins 4 caractères.

Non masqués :

- une valeur que vous n'avez pas déclarée, même si elle provient d'une source secrète ;
- toute autre transformation : hexadécimal, entités HTML, hachage, signature, valeur découpée en morceaux ou assemblée avec d'autre texte, forme échappée en JSON.

Lorsqu'une étape peut afficher un secret sous une autre forme, évitez de l'afficher, ou déclarez aussi la valeur transformée dans `secrets:`.
