---
translates: assertions
title: Aserciones
description: Las aserciones que puede usar para comprobar el resultado de un paso.
fingerprint: e848fd058b87a06a7e564aacd53bf6df2867aceff122501a2ebea797b0529545
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# Aserciones {#assertions}

### Palabras clave {#keywords}

* ShouldEqual - [ejemplo](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldEqual.yml)
* ShouldNotEqual - [ejemplo](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldNotEqual.yml)
* ShouldAlmostEqual - [ejemplo](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldAlmostEqual.yml)
* ShouldNotAlmostEqual - [ejemplo](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldNotAlmostEqual.yml)
* ShouldBeNil - [ejemplo](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldBeNil.yml)
* ShouldNotBeNil - [ejemplo](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldNotBeNil.yml)
* ShouldBeTrue - [ejemplo](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldBeTrue.yml)
* ShouldBeFalse - [ejemplo](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldBeFalse.yml)
* ShouldBeZeroValue - [ejemplo](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldBeZeroValue.yml)
* ShouldBeGreaterThan - [ejemplo](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldBeGreaterThan.yml)
* ShouldBeGreaterThanOrEqualTo - [ejemplo](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldBeGreaterThanOrEqualTo.yml)
* ShouldBeLessThan - [ejemplo](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldBeLessThan.yml)
* ShouldBeLessThanOrEqualTo - [ejemplo](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldBeLessThanOrEqualTo.yml)
* ShouldBeBetween - [ejemplo](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldBeBetween.yml)
* ShouldNotBeBetween - [ejemplo](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldNotBeBetween.yml)
* ShouldBeBetweenOrEqual - [ejemplo](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldBeBetweenOrEqual.yml)
* ShouldNotBeBetweenOrEqual - [ejemplo](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldNotBeBetweenOrEqual.yml)
* ShouldContain - [ejemplo](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldContain.yml)
* ShouldNotContain - [ejemplo](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldNotContain.yml)
* ShouldJSONContain - [ejemplo](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldJSONContain.yml)
* ShouldNotJSONContain - [ejemplo](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldNotJSONContain.yml)
* ShouldJSONContainWithKey - [ejemplo](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldJSONContainWithKey.yml)
* ShouldJSONContainAllWithKey - [ejemplo](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldJSONContainAllWithKey.yml)
* ShouldNotJSONContainWithKey - [ejemplo](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldNotJSONContainWithKey.yml)
* ShouldContainKey - [ejemplo](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldContainKey.yml)
* ShouldNotContainKey - [ejemplo](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldNotContainKey.yml)
* ShouldBeIn - [ejemplo](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldBeIn.yml)
* ShouldNotBeIn - [ejemplo](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldNotBeIn.yml)
* ShouldBeEmpty - [ejemplo](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldBeEmpty.yml)
* ShouldNotBeEmpty - [ejemplo](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldNotBeEmpty.yml)
* ShouldBeArray - [ejemplo](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldBeArray.yml)
* ShouldBeMap - [ejemplo](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldBeMap.yml)
* ShouldHaveLength - [ejemplo](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldHaveLength.yml)
* ShouldStartWith - [ejemplo](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldStartWith.yml)
* ShouldNotStartWith - [ejemplo](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldNotStartWith.yml)
* ShouldEndWith - [ejemplo](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldEndWith.yml)
* ShouldNotEndWith - [ejemplo](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldNotEndWith.yml)
* ShouldBeBlank - [ejemplo](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldBeBlank.yml)
* ShouldNotBeBlank - [ejemplo](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldNotBeBlank.yml)
* ShouldContainSubstring - [ejemplo](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldContainSubstring.yml)
* ShouldNotContainSubstring - [ejemplo](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldNotContainSubstring.yml)
* ShouldEqualTrimSpace - [ejemplo](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldEqualTrimSpace.yml)
* ShouldNotExist - [ejemplo](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldNotExist.yml)
* ShouldHappenBefore - [ejemplo](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldHappenBefore.yml)
* ShouldHappenOnOrBefore - [ejemplo](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldHappenOnOrBefore.yml)
* ShouldHappenAfter - [ejemplo](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldHappenAfter.yml)
* ShouldHappenOnOrAfter - [ejemplo](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldHappenOnOrAfter.yml)
* ShouldHappenBetween - [ejemplo](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldHappenBetween.yml)
* ShouldTimeEqual - [ejemplo](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldTimeEqual.yml)
* ShouldMatchRegex - [ejemplo](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldMatchRegex.yml)
* ShouldJSONEqual - [ejemplo](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldJSONEqual.yml)
* ShouldNotJSONEqual - [ejemplo](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldNotJSONEqual.yml)

#### Palabras clave `Must` {#must-keywords}

Todas las palabras clave de assertions anteriores tienen también una contraparte `Must`, que sirve para crear una aserción cuyo cumplimiento es obligatorio e impedir que los casos de prueba (y los executors personalizados) ejecuten los pasos restantes.

Ejemplo:
```yml
- steps:
  - type: exec
    script: exit 1
    assertions:
      - result.code MustEqual 0
  # Remaining steps in this context will not be executed
```

#### Aserciones de usuario {#user-assertions}

Es posible usar la [sintaxis de los executors definidos por el usuario](#user-defined-executors) y anteponer `Should` al nombre de su executor para crear una nueva palabra clave de "aserción de usuario". 

Ejemplo:
```yml
# lib/ShouldBeHttp2XX.yml
executor: ShouldBeHttp2XX
steps:
  - assertions:
    - a ShouldBeGreaterThanOrEqualTo 200
    - a ShouldBeLessThan 300
```

También puede incluir pasos adicionales, como en un executor definido por el usuario normal.

Las aserciones de usuario se ejecutan en un contexto completamente limpio, que contiene únicamente las siguientes variables:
- `a`: el operando izquierdo
- `b`: el (primer) operando derecho
- `argv`: los operandos derechos

Si necesita ser compatible con la sintaxis `input` de los executors definidos por el usuario, puede usar `argv` como valor predeterminado y acceder a ellos mediante la sintaxis habitual `input.*`. 
```yaml
input:
  test: "{{.argv.argv1}}"
```

Para llamar a una aserción de usuario registrada, basta con usar su nombre registrado en el array `assertions`, igual que cualquier palabra clave de aserción integrada.

```yml
# test.yml
testcases:
- steps:
  - type: http
    url: https://example.com
    assertions:
    - result.statuscode ShouldBeHttp2XX
```

### Uso de operadores lógicos {#using-logical-operators}

Aunque las aserciones usan el operador `and` de forma implícita, es posible usar otros operadores lógicos para realizar aserciones complejas.

Los operadores admitidos son `and`, `or` y `xor`.

```yml
- name: Assertions operators
  steps:
  - script: echo 1
    assertions:
      - or:
        - result.systemoutjson ShouldEqual 1 
        - result.systemoutjson ShouldEqual 2
      # Nested operators
      - or:
        - result.systemoutjson ShouldBeGreaterThanOrEqualTo 1
        - result.systemoutjson ShouldBeLessThanOrEqualTo 1
        - or:
          - result.systemoutjson ShouldEqual 1
```

Hay más ejemplos disponibles en [`tests/assertions_operators.yml`](/tests/assertions_operators.yml).

