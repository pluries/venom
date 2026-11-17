---
translates: exec
title: Exec
description: Cómo usar el executor Exec en una suite de pruebas de venom.
fingerprint: fc1f5ab5e0397848ca237c08b3e7fd811ce670d0c6a309363d41c0481b77f07e
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# Venom - Executor Exec {#venom---executor-exec}

Paso que se utiliza para ejecutar un script

El script se ejecuta con los permisos del usuario que ejecuta venom. No incluya en él el resultado de un paso ni ningún valor no confiable sin entrecomillarlo: consulte Trust model en el [README raíz](../../README.md#trust-model).


## Input {#input}

Ejemplo

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

Script de varias líneas:

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

Entrada:

```yaml
name: Title of TestSuite
testcases:
- name: with stdin
  steps:
  - type: exec
    stdin: Foo
    script: cat
```

Comando explícito (sin shell):

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

- result.timeseconds: duración de la ejecución
- result.err: si existe, este campo contiene los detalles del error
- result.systemout: salida estándar del script ejecutado
- result.systemoutjson: salida estándar del script ejecutado, analizada como un objeto JSON
- result.systemerr: salida de error del script ejecutado
- result.systemerrjson: salida de error del script ejecutado, analizada como un objeto JSON
- result.code: código de salida

## Default assertion {#default-assertion}

```yaml
result.code ShouldEqual 0
```

