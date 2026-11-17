---
translates: step-outputs
title: Utiliser la sortie d'une étape de test comme entrée d'une autre étape
description: Comment une étape transmet son résultat à la suivante.
fingerprint: 0b48c4fa6f51fc685b64e68b3c67da6261ca7863c29554e824ea525eac5d0679
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# Utiliser la sortie d'une étape de test comme entrée d'une autre étape {#use-outputs-from-a-test-step-as-input-of-another-test-step}

Pour réutiliser une propriété d'une étape dans un testcase ou une étape suivante, vous devez extraire la variable, comme dans l'exemple ci-dessous. 

Après l'exécution de la première étape, `venom` extrait une valeur au moyen de l'expression régulière `foo with a ([a-z]+) here` à partir du contenu de la propriété `result.systemout` renvoyée par l'`executor`.
Cette variable peut ensuite être réutilisée dans un autre test, sous le nom `testA.myvariable`, où `testA` correspond au nom du testcase. Une valeur par défaut peut aussi être fournie si la variable ne peut pas être extraite de la sortie, ce qui arrive couramment lors de l'analyse d'une sortie JSON.

```yaml
name: MyTestSuite
testcases:
- name: testA
  steps:
  - type: exec
    script: echo 'foo with a bar here'
    vars:
      myvariable:
        from: result.systemout
        regex: foo with a ([a-z]+) here
        default: "somevalue"

- name: testB
  steps:
  - type: exec
    script: echo {{.testA.myvariable}}
    assertions:
    - result.code ShouldEqual 0
    - result.systemout ShouldContainSubstring bar
```
