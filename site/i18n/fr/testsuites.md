---
translates: testsuites
title: Suites de tests
description: Comment écrire une suite de tests et ses cas de test.
fingerprint: 692000e4f0fd4c5e632c7755229552c8524f88d83321a5f7cb777257e8c3018a
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# Suites de tests {#testsuites}

Une suite de tests est un ensemble de cas de test destinés à tester un programme afin de montrer qu'il présente un ensemble précis de comportements.
Un cas de test est la spécification des entrées, des conditions d'exécution, de la procédure de test et des résultats attendus qui définissent un test unique, exécuté pour atteindre un objectif de test logiciel donné, par exemple exercer un chemin particulier du programme ou vérifier la conformité à une exigence précise.

Dans `venom`, les testcases d'une testsuite sont exécutés séquentiellement. Chaque testcase est un ensemble ordonné d'étapes. Chaque étape repose sur un `executor` qui apporte un type de comportement spécifique.

Dans `venom`, une testsuite s'écrit dans un seul fichier `YAML` qui respecte la structure suivante :

```yaml

name: Title of TestSuite
description: A detailed description of the TestSuite, in markdown.
testcases:
- name: TestCase with default value, exec cmd. Check if exit code != 1
  steps:
  - script: echo 'foo'
    type: exec

- name: Title of First TestCase
  steps:
  - script: echo 'foo'
    assertions:
    - result.code ShouldEqual 0
  - script: echo 'bar'
    assertions:
    - result.systemout ShouldNotContainSubstring foo
    - result.timeseconds ShouldBeLessThan 1

- name: GET http testcase, with 5 seconds timeout
  steps:
  - type: http
    method: GET
    url: https://eu.api.ovh.com/1.0/
    timeout: 5
    assertions:
    - result.body ShouldContainSubstring /dedicated/server
    - result.body ShouldContainSubstring /ipLoadbalancing
    - result.statuscode ShouldEqual 200
    - result.timeseconds ShouldBeLessThan 1

- name: Test with retries and delay in seconds between each try
  steps:
  - type: http
    method: GET
    url: https://eu.api.ovh.com/1.0/
    retry: 3
    retry_if: # (optional, lets you early break unrecoverable errors)
    - result.statuscode ShouldNotEqual 403
    delay: 2
    assertions:
    - result.statuscode ShouldEqual 200

```

### Formulaire HTTP multipart {#http-multipart-form}

`multipart_form` envoie le corps de la requête en `multipart/form-data`. Il ne peut pas être combiné avec `body` ni avec `bodyFile`. Voir l'[executor HTTP](https://github.com/ovh/venom/tree/master/executors/http) pour la liste complète des champs.

Chaque clé est un champ du formulaire. Une valeur est soit une simple chaîne, soit une liste de chaînes
pour envoyer le même nom de champ une fois par élément (par exemple une partie `files` répétée pour un envoi de plusieurs fichiers).

Une valeur qui commence par `@` est le chemin d'un fichier à envoyer, résolu depuis le répertoire
de travail courant. Ajoutez `;type=<content-type>` pour fixer le Content-Type de cette partie ;
il vaut `application/octet-stream` par défaut. Cela reprend le `-F field=@path;type=...` de curl.

```yaml
- name: upload files
  steps:
  - type: http
    method: POST
    url: https://httpbin.org/post
    multipart_form:
      description: "a plain text field"
      tags:
        - "first"
        - "second"
      files:
        - "@./testdata/report.pdf;type=application/pdf"
        - "@./testdata/notes.txt;type=text/plain"
    assertions:
    - result.statuscode ShouldEqual 200
```
