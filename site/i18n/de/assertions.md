---
translates: assertions
title: Assertions
description: Die Assertions, mit denen Sie das Ergebnis eines Schritts prüfen.
fingerprint: e848fd058b87a06a7e564aacd53bf6df2867aceff122501a2ebea797b0529545
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# Assertions {#assertions}

### Schlüsselwörter {#keywords}

* ShouldEqual - [Beispiel](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldEqual.yml)
* ShouldNotEqual - [Beispiel](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldNotEqual.yml)
* ShouldAlmostEqual - [Beispiel](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldAlmostEqual.yml)
* ShouldNotAlmostEqual - [Beispiel](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldNotAlmostEqual.yml)
* ShouldBeNil - [Beispiel](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldBeNil.yml)
* ShouldNotBeNil - [Beispiel](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldNotBeNil.yml)
* ShouldBeTrue - [Beispiel](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldBeTrue.yml)
* ShouldBeFalse - [Beispiel](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldBeFalse.yml)
* ShouldBeZeroValue - [Beispiel](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldBeZeroValue.yml)
* ShouldBeGreaterThan - [Beispiel](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldBeGreaterThan.yml)
* ShouldBeGreaterThanOrEqualTo - [Beispiel](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldBeGreaterThanOrEqualTo.yml)
* ShouldBeLessThan - [Beispiel](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldBeLessThan.yml)
* ShouldBeLessThanOrEqualTo - [Beispiel](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldBeLessThanOrEqualTo.yml)
* ShouldBeBetween - [Beispiel](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldBeBetween.yml)
* ShouldNotBeBetween - [Beispiel](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldNotBeBetween.yml)
* ShouldBeBetweenOrEqual - [Beispiel](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldBeBetweenOrEqual.yml)
* ShouldNotBeBetweenOrEqual - [Beispiel](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldNotBeBetweenOrEqual.yml)
* ShouldContain - [Beispiel](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldContain.yml)
* ShouldNotContain - [Beispiel](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldNotContain.yml)
* ShouldJSONContain - [Beispiel](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldJSONContain.yml)
* ShouldNotJSONContain - [Beispiel](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldNotJSONContain.yml)
* ShouldJSONContainWithKey - [Beispiel](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldJSONContainWithKey.yml)
* ShouldJSONContainAllWithKey - [Beispiel](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldJSONContainAllWithKey.yml)
* ShouldNotJSONContainWithKey - [Beispiel](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldNotJSONContainWithKey.yml)
* ShouldContainKey - [Beispiel](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldContainKey.yml)
* ShouldNotContainKey - [Beispiel](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldNotContainKey.yml)
* ShouldBeIn - [Beispiel](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldBeIn.yml)
* ShouldNotBeIn - [Beispiel](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldNotBeIn.yml)
* ShouldBeEmpty - [Beispiel](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldBeEmpty.yml)
* ShouldNotBeEmpty - [Beispiel](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldNotBeEmpty.yml)
* ShouldBeArray - [Beispiel](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldBeArray.yml)
* ShouldBeMap - [Beispiel](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldBeMap.yml)
* ShouldHaveLength - [Beispiel](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldHaveLength.yml)
* ShouldStartWith - [Beispiel](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldStartWith.yml)
* ShouldNotStartWith - [Beispiel](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldNotStartWith.yml)
* ShouldEndWith - [Beispiel](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldEndWith.yml)
* ShouldNotEndWith - [Beispiel](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldNotEndWith.yml)
* ShouldBeBlank - [Beispiel](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldBeBlank.yml)
* ShouldNotBeBlank - [Beispiel](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldNotBeBlank.yml)
* ShouldContainSubstring - [Beispiel](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldContainSubstring.yml)
* ShouldNotContainSubstring - [Beispiel](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldNotContainSubstring.yml)
* ShouldEqualTrimSpace - [Beispiel](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldEqualTrimSpace.yml)
* ShouldNotExist - [Beispiel](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldNotExist.yml)
* ShouldHappenBefore - [Beispiel](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldHappenBefore.yml)
* ShouldHappenOnOrBefore - [Beispiel](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldHappenOnOrBefore.yml)
* ShouldHappenAfter - [Beispiel](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldHappenAfter.yml)
* ShouldHappenOnOrAfter - [Beispiel](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldHappenOnOrAfter.yml)
* ShouldHappenBetween - [Beispiel](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldHappenBetween.yml)
* ShouldTimeEqual - [Beispiel](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldTimeEqual.yml)
* ShouldMatchRegex - [Beispiel](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldMatchRegex.yml)
* ShouldJSONEqual - [Beispiel](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldJSONEqual.yml)
* ShouldNotJSONEqual - [Beispiel](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldNotJSONEqual.yml)


#### `Must`-Schlüsselwörter {#must-keywords}

Alle oben genannten Assertion-Schlüsselwörter gibt es auch mit einem `Must`-Gegenstück. Damit lässt sich eine zwingend zu erfüllende Assertion erstellen, die verhindert, dass Testfälle (und benutzerdefinierte Executors) die übrigen Schritte ausführen.

Beispiel:
```yml
- steps:
  - type: exec
    script: exit 1
    assertions:
      - result.code MustEqual 0
  # Remaining steps in this context will not be executed
```

#### Benutzerdefinierte Assertions {#user-assertions}

Sie können die [Syntax benutzerdefinierter Executors](#user-defined-executors) verwenden und dem Namen Ihres Executors das Präfix `Should` voranstellen, um ein neues Schlüsselwort für eine „benutzerdefinierte Assertion“ zu erstellen. 

Beispiel:
```yml
# lib/ShouldBeHttp2XX.yml
executor: ShouldBeHttp2XX
steps:
  - assertions:
    - a ShouldBeGreaterThanOrEqualTo 200
    - a ShouldBeLessThan 300
```

Sie können wie bei einem gewöhnlichen benutzerdefinierten Executor auch zusätzliche Schritte einbinden.

Benutzerdefinierte Assertions werden in einem vollständig sauberen Kontext ausgeführt, der nur die folgenden Variablen enthält:
- `a`: der linke Operand
- `b`: der (erste) rechte Operand
- `argv`: die rechten Operanden

Wenn Sie zur `input`-Syntax benutzerdefinierter Executors kompatibel bleiben müssen, können Sie `argv` als Standardwert verwenden und über die reguläre `input.*`-Syntax darauf zugreifen. 
```yaml
input:
  test: "{{.argv.argv1}}"
```

Um eine registrierte benutzerdefinierte Assertion aufzurufen, verwenden Sie einfach ihren registrierten Namen im Array `assertions`, genau wie jedes eingebaute Assertion-Schlüsselwort.

```yml
# test.yml
testcases:
- steps:
  - type: http
    url: https://example.com
    assertions:
    - result.statuscode ShouldBeHttp2XX
```

### Logische Operatoren verwenden {#using-logical-operators}

Assertions werden implizit mit dem Operator `and` verknüpft. Für komplexe Assertions können Sie aber auch andere logische Operatoren verwenden.

Unterstützt werden die Operatoren `and`, `or` und `xor`.

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

Weitere Beispiele finden Sie in [`tests/assertions_operators.yml`](/tests/assertions_operators.yml).
