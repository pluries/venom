---
translates: step-outputs
title: Usare l'output di uno step come input di un altro step
description: Come uno step passa il proprio risultato allo step successivo.
fingerprint: 0b48c4fa6f51fc685b64e68b3c67da6261ca7863c29554e824ea525eac5d0679
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# Usare l'output di uno step come input di un altro step {#use-outputs-from-a-test-step-as-input-of-another-test-step}

Per riutilizzare una proprietà di un teststep in un testcase o in uno step successivo, devi estrarre la variabile, come nell'esempio seguente. 

Dopo l'esecuzione del primo step, `venom` estrae un valore con l'espressione regolare `foo with a ([a-z]+) here` dal contenuto della proprietà `result.systemout` restituita dall'`executor`.
Questa variabile può poi essere riutilizzata in un altro test, con il nome `testA.myvariable`, dove `testA` corrisponde al nome del testcase. Si può anche indicare un valore predefinito, da usare se la variabile non può essere estratta dall'output, cosa che capita spesso quando si analizza un output JSON.

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
