---
translates: variables
title: Variables
description: Cómo definir y usar variables en una suite.
fingerprint: 571c5e16517c3ed7ec39bc40546cba3ab0da6e9c0ebd3a0e1e59d6875364590b
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# Variables {#variables}

### Variables de la suite de pruebas {#testsuite-variables}

Puede definir variables a nivel de `testsuite`.

```yaml
name: myTestSuite
vars:
  foo: foo
  biz:
    bar: bar
  aString: '{"foo": "bar"}'

testcases:
- name: first-test-case
  steps:
  - type: exec
    script: echo '{{.foo}} {{.biz.bar}}'
    assertions:
    - result.code ShouldEqual 0
    - result.systemout ShouldEqual "foo bar"

- name: foobar
  steps:
  - script: echo '{{.aString}}'
    info: value of aString is {{.aString}}
    assertions:
    - result.systemoutjson.foo ShouldEqual bar
...
```

Cada variable de usuario utilizada en la suite de pruebas debe declararse en esta sección. Puede sobrescribir su valor en tiempo de ejecución de varias maneras:
- De forma individual, con la opción de línea de comandos `--var`.
- En archivos de definición de variables, indicados en la línea de comandos con `--var-from-file`.
- Como variables de entorno.



### Hacer fallar un paso por una variable desconocida {#fail-a-step-on-an-unknown-variable}

Por defecto, venom deja tal cual está escrito en el paso un `{{.name}}` que no conoce, de modo que una variable mal escrita como
`{{.nmae}}` se envía como texto y la prueba puede pasar. Con `--strict-variables`, un paso que nombra una variable desconocida
falla antes de ejecutarse, y el mensaje nombra el paso y la variable:

```
step #2: unknown variable(s) "nmae"
```

El ajuste está desactivado por defecto y se aplica a toda la ejecución. Actívelo con `--strict-variables`,
`VENOM_STRICT_VARIABLES=true` o `strict_variables: true` en `.venomrc`.

Nunca se notifican estas: las variables definidas por un paso anterior o por `vars`, las variables de rango (`key`, `value`, `index`),
las variables `venom.*`, las expresiones que usan `default` o `ternary`, y `{{.result...}}` (venom las resuelve una vez ejecutado el
paso, en `info`, en las aserciones y en las variables extraídas). Los nombres de helpers desconocidos no se comprueban.

Un límite: venom primero lee el archivo completo y evalúa, a nivel de paso, toda expresión formada únicamente por letras, dígitos, `.`, `-`, `_`,
`"`, `|` y espacios. Una expresión con cualquier otro carácter, como `{{ printf "%s" .a }}` o
`{{.url | trimSuffix "/"}}`, se evalúa al leer el archivo, antes de ejecutar los pasos, por lo que el modo estricto no la ve
y una variable mal escrita en ella sigue dando un valor vacío o incorrecto sin ningún error.

Un paso que necesita que el texto `{{.something}}` llegue sin cambios a otra herramienta, como `docker ... --format '{{.ID}}'`,
falla en modo estricto. Venom no tiene escape para las llaves dobles literales, así que deje el ajuste desactivado en una suite así.

### Helpers de variables {#variable-helpers}

Helpers disponibles y algunos ejemplos:

- `abbrev`
- `abbrevboth`
- `trunc`
- `trim`
- `upper`: {{.myvar | upper}}
- `lower`: {{.myvar | lower}}
- `title`
- `untitle`
- `substr`
- `repeat`
- `trimall`
- `trimAll`
- `trimSuffix`
- `trimPrefix`
- `nospace`
- `initials`
- `randAlphaNum`
- `randAlpha`
- `randASCII`
- `randNumeric`
- `swapcase`
- `shuffle`
- `snakecase`
- `camelcase`
- `quote`
- `squote`
- `indent`
- `nindent`
- `replace`: {{.myvar | replace "_" "."}}
- `plural`
- `default`: {{.myvar | default ""}}
- `empty`
- `coalesce`
- `toJSON`
- `toPrettyJSON`
- `b64enc`
- `b64dec` {{.result.bodyjson | b64enc}}
- `escape`: sustituye ‘_‘, ‘/’, ‘.’ por ‘-’

Hay más ejemplos [aquí](https://github.com/ovh/venom/tree/master/variable_helpers.md)
