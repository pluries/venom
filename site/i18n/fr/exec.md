---
translates: exec
title: Exec
description: Comment utiliser l'executor Exec dans une suite de tests venom.
fingerprint: fc1f5ab5e0397848ca237c08b3e7fd811ce670d0c6a309363d41c0481b77f07e
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# Venom - Executor Exec {#venom---executor-exec}

Étape qui exécute un script

Le script s'exécute avec les droits de l'utilisateur qui lance venom. Ne placez pas le résultat d'une étape ni une valeur non fiable dans le script sans la mettre entre guillemets : voir Trust model dans le [README racine](../../README.md#trust-model).


## Input {#input}

Exemple

```yaml

name: Title of TestSuite
testcases:
- name: Check if exit code != 1 and echo command response in less than 1s
  steps:
  - type: exec
    script: echo 'foo'
    assertions:
    - result.code ShouldEqual 0
    - result.timeseconds ShouldBeLessThan 1

```

Script sur plusieurs lignes :

```yaml
name: Title of TestSuite
testcases:
- name: multiline script
  steps:
  - type: exec
    script: |
            echo "Foo" \
            echo "Bar"
```

Entrée standard :

```yaml
name: Title of TestSuite
testcases:
- name: with stdin
  steps:
  - type: exec
    stdin: Foo
    script: cat
```

Commande explicite (sans shell) :

```yaml
name: Title of TestSuite
testcases:
- name: explicit command
  steps:
  - type: exec
    stdin: "{\"foo\":\"bar\"}"
    command: ["jq", ".foo"]
```

## Output {#output}

```yaml
systemout
systemoutjson
systemerr
systemerrjson
err
code
timeseconds
```

- result.timeseconds : durée d'exécution
- result.err : s'il existe, ce champ contient le détail de l'erreur
- result.systemout : sortie standard du script exécuté
- result.systemoutjson : sortie standard du script exécuté, analysée comme un objet JSON
- result.systemerr : sortie d'erreur du script exécuté
- result.systemerrjson : sortie d'erreur du script exécuté, analysée comme un objet JSON
- result.code : code de sortie

## Default assertion {#default-assertion}

```yaml
result.code ShouldEqual 0
```
