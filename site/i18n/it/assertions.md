---
translates: assertions
title: Assertion
description: Le assertion che puoi usare per verificare il risultato di uno step.
fingerprint: e848fd058b87a06a7e564aacd53bf6df2867aceff122501a2ebea797b0529545
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# Assertion {#assertions}

### Parole chiave {#keywords}

* ShouldEqual - [esempio](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldEqual.yml)
* ShouldNotEqual - [esempio](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldNotEqual.yml)
* ShouldAlmostEqual - [esempio](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldAlmostEqual.yml)
* ShouldNotAlmostEqual - [esempio](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldNotAlmostEqual.yml)
* ShouldBeNil - [esempio](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldBeNil.yml)
* ShouldNotBeNil - [esempio](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldNotBeNil.yml)
* ShouldBeTrue - [esempio](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldBeTrue.yml)
* ShouldBeFalse - [esempio](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldBeFalse.yml)
* ShouldBeZeroValue - [esempio](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldBeZeroValue.yml)
* ShouldBeGreaterThan - [esempio](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldBeGreaterThan.yml)
* ShouldBeGreaterThanOrEqualTo - [esempio](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldBeGreaterThanOrEqualTo.yml)
* ShouldBeLessThan - [esempio](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldBeLessThan.yml)
* ShouldBeLessThanOrEqualTo - [esempio](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldBeLessThanOrEqualTo.yml)
* ShouldBeBetween - [esempio](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldBeBetween.yml)
* ShouldNotBeBetween - [esempio](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldNotBeBetween.yml)
* ShouldBeBetweenOrEqual - [esempio](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldBeBetweenOrEqual.yml)
* ShouldNotBeBetweenOrEqual - [esempio](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldNotBeBetweenOrEqual.yml)
* ShouldContain - [esempio](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldContain.yml)
* ShouldNotContain - [esempio](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldNotContain.yml)
* ShouldJSONContain - [esempio](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldJSONContain.yml)
* ShouldNotJSONContain - [esempio](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldNotJSONContain.yml)
* ShouldJSONContainWithKey - [esempio](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldJSONContainWithKey.yml)
* ShouldJSONContainAllWithKey - [esempio](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldJSONContainAllWithKey.yml)
* ShouldNotJSONContainWithKey - [esempio](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldNotJSONContainWithKey.yml)
* ShouldContainKey - [esempio](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldContainKey.yml)
* ShouldNotContainKey - [esempio](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldNotContainKey.yml)
* ShouldBeIn - [esempio](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldBeIn.yml)
* ShouldNotBeIn - [esempio](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldNotBeIn.yml)
* ShouldBeEmpty - [esempio](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldBeEmpty.yml)
* ShouldNotBeEmpty - [esempio](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldNotBeEmpty.yml)
* ShouldBeArray - [esempio](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldBeArray.yml)
* ShouldBeMap - [esempio](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldBeMap.yml)
* ShouldHaveLength - [esempio](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldHaveLength.yml)
* ShouldStartWith - [esempio](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldStartWith.yml)
* ShouldNotStartWith - [esempio](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldNotStartWith.yml)
* ShouldEndWith - [esempio](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldEndWith.yml)
* ShouldNotEndWith - [esempio](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldNotEndWith.yml)
* ShouldBeBlank - [esempio](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldBeBlank.yml)
* ShouldNotBeBlank - [esempio](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldNotBeBlank.yml)
* ShouldContainSubstring - [esempio](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldContainSubstring.yml)
* ShouldNotContainSubstring - [esempio](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldNotContainSubstring.yml)
* ShouldEqualTrimSpace - [esempio](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldEqualTrimSpace.yml)
* ShouldNotExist - [esempio](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldNotExist.yml)
* ShouldHappenBefore - [esempio](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldHappenBefore.yml)
* ShouldHappenOnOrBefore - [esempio](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldHappenOnOrBefore.yml)
* ShouldHappenAfter - [esempio](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldHappenAfter.yml)
* ShouldHappenOnOrAfter - [esempio](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldHappenOnOrAfter.yml)
* ShouldHappenBetween - [esempio](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldHappenBetween.yml)
* ShouldTimeEqual - [esempio](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldTimeEqual.yml)
* ShouldMatchRegex - [esempio](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldMatchRegex.yml)
* ShouldJSONEqual - [esempio](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldJSONEqual.yml)
* ShouldNotJSONEqual - [esempio](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldNotJSONEqual.yml)

#### Le parole chiave `Must` {#must-keywords}

Tutte le parole chiave delle assertion qui sopra hanno anche una controparte `Must`, che permette di creare un'assertion obbligatoria e impedisce ai test case (e agli executor personalizzati) di eseguire gli step rimanenti.

Esempio:
```yml
- steps:
  - type: exec
    script: exit 1
    assertions:
      - result.code MustEqual 0
  # Remaining steps in this context will not be executed
```

#### Assertion dell'utente {#user-assertions}

È possibile usare la [sintassi degli executor definiti dall'utente](#user-defined-executors) e anteporre `Should` al nome del tuo executor per creare una nuova parola chiave di "assertion dell'utente". 

Esempio:
```yml
# lib/ShouldBeHttp2XX.yml
executor: ShouldBeHttp2XX
steps:
  - assertions:
    - a ShouldBeGreaterThanOrEqualTo 200
    - a ShouldBeLessThan 300
```

Puoi anche includere degli step aggiuntivi, come in un normale executor definito dall'utente.

Le assertion dell'utente vengono eseguite in un contesto completamente pulito, che contiene solo le variabili seguenti:
- `a`: l'operando di sinistra
- `b`: il (primo) operando di destra
- `argv`: gli operandi di destra

Se ti serve la compatibilità con la sintassi `input` degli executor definiti dall'utente, puoi usare `argv` come valore predefinito e accedervi tramite la normale sintassi `input.*`. 
```yaml
input:
  test: "{{.argv.argv1}}"
```

Per chiamare un'assertion dell'utente registrata, basta usare il suo nome registrato nell'array `assertions`, come qualsiasi parola chiave di assertion integrata.

```yml
# test.yml
testcases:
- steps:
  - type: http
    url: https://example.com
    assertions:
    - result.statuscode ShouldBeHttp2XX
```

### Usare gli operatori logici {#using-logical-operators}

Mentre le assertion usano implicitamente l'operatore `and`, è possibile usare altri operatori logici per scrivere assertion complesse.

Gli operatori supportati sono `and`, `or` e `xor`.

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

Altri esempi sono disponibili in [`tests/assertions_operators.yml`](/tests/assertions_operators.yml).

