---
translates: advanced
title: Uso avanzado
description: Opciones para suites más grandes.
fingerprint: a7992da938eb488d1fc5fa9a37e1159081f4a0bf684768a8c2d34b4bb13924ff
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# Uso avanzado {#advanced-usage}

## Depurar sus suites de pruebas {#debug-your-testsuites}

Se genera un archivo *venom.log* con cada comando `venom run`.

Hay dos formas de depurar una suite de pruebas:
 - usar el indicador `-v` en el binario de venom.
   - `$ venom run -v test.yml` mostrará detalles de cada paso
   - `$ venom run -vv test.yml` generará archivos *dump.json* para cada paso de prueba.
   - `$ venom run -vvv test.yml` generará archivos *pprof*, un perfil de CPU y uno de memoria por
     archivo de prueba, con los nombres `pprof_cpu_profile_<file>.prof` y `pprof_mem_profile_<file>.prof` en el
     directorio de salida. Ábralos con `go tool pprof`. El perfilado es por archivo para poder ver qué
     suite es lenta, y por eso `-vvv` no puede combinarse con `--parallel 2` o más: el perfilador de CPU
     de Go abarca todo el proceso.
 - usar la palabra clave `info` en su paso de prueba:
archivo `test.yml`:
```yml
name: Exec testsuite
testcases:
- name: testA
  steps:
  - type: exec
    script: echo 'foo with a bar here'
    info:
      - this a first info
      - and a second...
- name: cat json
  steps:
  - script: cat exec/testa.json
    info: "the value of result.systemoutjson is {{.result.systemoutjson}}"
    assertions:
    - result.systemoutjson.foo ShouldContainSubstring bar
```

```bash
$ venom run test.yml

# output:

 • Exec testsuite (exec.yml)
  • testA SUCCESS
    [info] this a first info (exec.yml:8)
    [info] and a second... (exec.yml:9)
  • testB SUCCESS
  • sleep 1 SUCCESS
  • cat json SUCCESS
    [info] the value of result.systemoutjson is map[foo:bar] (exec.yml:34)
```

## Omitir casos de prueba y pasos de prueba {#skip-testcase-and-teststeps}

Es posible omitir un `testcase` según algunas `assertions`. Por ejemplo, el siguiente ejemplo omitirá el último caso de prueba.

```yaml
name: "Skip testsuite"
vars:
  foo: bar

testcases:
- name: init
  steps:
  - type: exec
    script: echo {{.foo}}
    assertions:
    - result.code ShouldEqual 0
    - result.systemout ShouldContainSubstring bar

- name: do-not-skip-this
  skip: 
  - foo ShouldNotBeEmpty
  steps:
  - type: exec
    script: exit 0

- name: skip-this
  skip: 
    - foo ShouldBeEmpty
  steps:
  - type: exec
    script: command_not_found
    assertions:
    - result.code ShouldEqual 0

```

Una instrucción `skip` también puede colocarse a nivel de paso para ejecutar parcialmente un caso de prueba. Si una condición del bloque skip no se cumple, el paso se omite.

Si se omiten todos los pasos de un caso de prueba, el propio caso de prueba también se considerará "omitido" en lugar de "superado" o "fallido".

```yaml
name: "Skip testsuite"
vars:
  foo: bar

testcases:
- name: skip-one-of-these
  steps:
  - name: do-not-skip-this
    type: exec
    script: exit 0
    assertions:
    - result.code ShouldEqual 0
    skip:
    - foo ShouldNotBeEmpty
  - name: skip-this
    type: exec
    script: exit 1
    assertions:
    - result.code ShouldEqual 0
    skip:
    - foo ShouldBeEmpty

```

## Iterar sobre datos {#iterating-over-data}

Es posible iterar sobre datos mediante el atributo `range`.

Se admiten los siguientes tipos de datos, y cada uno expone las variables contextuales `.index`, `.key` y `.value`:

- Un array en el que se itera sobre cada valor (`[]interface{}`)
  - `.index`/`.key`: índice de la iteración actual
  - `.value`: valor del elemento de la iteración actual
- Un mapa en el que se itera sobre cada clave (`map[string]interface{}`)
  - `.index`: índice de la iteración actual
  - `.key`: clave del elemento de la iteración actual
  - `.value`: valor del elemento de la iteración actual
- Un entero para ejecutar el paso de destino `n` veces (`int`)
  - `.index`/`.key`/`.value`: índice de la iteración actual
- Una cadena con plantilla que da como resultado uno de los tipos anteriores (`string`)
  - Puede heredarse de un archivo de variables o interpolarse a partir del resultado de un paso anterior

Por ejemplo, el siguiente ejemplo itera sobre un array de dos elementos que contienen mapas:
```yaml
- name: range with hardcoded array
  steps:
  - type: exec
    range:
      - actual: hello
        expected: hello
      - actual: world
        expected: world
    script: echo "{{.value.actual}}"
    assertions:
    - result.code ShouldEqual 0
    - result.systemout ShouldEqual "{{.value.expected}}"
```

Hay más ejemplos disponibles en [`tests/ranged.yml`](/tests/ranged.yml).
