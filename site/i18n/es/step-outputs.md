---
translates: step-outputs
title: Usar la salida de un paso de prueba como entrada de otro paso
description: Cómo un paso pasa su resultado al siguiente.
fingerprint: 0b48c4fa6f51fc685b64e68b3c67da6261ca7863c29554e824ea525eac5d0679
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# Usar la salida de un paso de prueba como entrada de otro paso {#use-outputs-from-a-test-step-as-input-of-another-test-step}

Para poder reutilizar una propiedad de un paso de prueba en un caso de prueba o un paso posterior, debe extraer la variable, como en el siguiente ejemplo. 

Tras la ejecución del primer paso, `venom` extrae un valor mediante la expresión regular `foo with a ([a-z]+) here` del contenido de la propiedad `result.systemout` devuelta por el `executor`.
Esta variable puede reutilizarse después en otra prueba, con el nombre `testA.myvariable`, donde `testA` corresponde al nombre del caso de prueba. También se puede indicar un valor por defecto para el caso en que la variable no pueda extraerse de la salida, algo habitual al analizar una salida JSON.

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
