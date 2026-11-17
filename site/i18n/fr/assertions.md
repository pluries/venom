---
translates: assertions
title: Assertions
description: Les assertions utilisables pour vérifier le résultat d'une étape.
fingerprint: e848fd058b87a06a7e564aacd53bf6df2867aceff122501a2ebea797b0529545
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# Assertions {#assertions}

### Mots-clés {#keywords}

* ShouldEqual - [exemple](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldEqual.yml)
* ShouldNotEqual - [exemple](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldNotEqual.yml)
* ShouldAlmostEqual - [exemple](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldAlmostEqual.yml)
* ShouldNotAlmostEqual - [exemple](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldNotAlmostEqual.yml)
* ShouldBeNil - [exemple](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldBeNil.yml)
* ShouldNotBeNil - [exemple](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldNotBeNil.yml)
* ShouldBeTrue - [exemple](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldBeTrue.yml)
* ShouldBeFalse - [exemple](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldBeFalse.yml)
* ShouldBeZeroValue - [exemple](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldBeZeroValue.yml)
* ShouldBeGreaterThan - [exemple](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldBeGreaterThan.yml)
* ShouldBeGreaterThanOrEqualTo - [exemple](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldBeGreaterThanOrEqualTo.yml)
* ShouldBeLessThan - [exemple](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldBeLessThan.yml)
* ShouldBeLessThanOrEqualTo - [exemple](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldBeLessThanOrEqualTo.yml)
* ShouldBeBetween - [exemple](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldBeBetween.yml)
* ShouldNotBeBetween - [exemple](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldNotBeBetween.yml)
* ShouldBeBetweenOrEqual - [exemple](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldBeBetweenOrEqual.yml)
* ShouldNotBeBetweenOrEqual - [exemple](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldNotBeBetweenOrEqual.yml)
* ShouldContain - [exemple](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldContain.yml)
* ShouldNotContain - [exemple](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldNotContain.yml)
* ShouldJSONContain - [exemple](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldJSONContain.yml)
* ShouldNotJSONContain - [exemple](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldNotJSONContain.yml)
* ShouldJSONContainWithKey - [exemple](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldJSONContainWithKey.yml)
* ShouldJSONContainAllWithKey - [exemple](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldJSONContainAllWithKey.yml)
* ShouldNotJSONContainWithKey - [exemple](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldNotJSONContainWithKey.yml)
* ShouldContainKey - [exemple](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldContainKey.yml)
* ShouldNotContainKey - [exemple](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldNotContainKey.yml)
* ShouldBeIn - [exemple](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldBeIn.yml)
* ShouldNotBeIn - [exemple](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldNotBeIn.yml)
* ShouldBeEmpty - [exemple](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldBeEmpty.yml)
* ShouldNotBeEmpty - [exemple](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldNotBeEmpty.yml)
* ShouldBeArray - [exemple](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldBeArray.yml)
* ShouldBeMap - [exemple](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldBeMap.yml)
* ShouldHaveLength - [exemple](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldHaveLength.yml)
* ShouldStartWith - [exemple](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldStartWith.yml)
* ShouldNotStartWith - [exemple](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldNotStartWith.yml)
* ShouldEndWith - [exemple](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldEndWith.yml)
* ShouldNotEndWith - [exemple](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldNotEndWith.yml)
* ShouldBeBlank - [exemple](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldBeBlank.yml)
* ShouldNotBeBlank - [exemple](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldNotBeBlank.yml)
* ShouldContainSubstring - [exemple](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldContainSubstring.yml)
* ShouldNotContainSubstring - [exemple](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldNotContainSubstring.yml)
* ShouldEqualTrimSpace - [exemple](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldEqualTrimSpace.yml)
* ShouldNotExist - [exemple](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldNotExist.yml)
* ShouldHappenBefore - [exemple](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldHappenBefore.yml)
* ShouldHappenOnOrBefore - [exemple](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldHappenOnOrBefore.yml)
* ShouldHappenAfter - [exemple](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldHappenAfter.yml)
* ShouldHappenOnOrAfter - [exemple](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldHappenOnOrAfter.yml)
* ShouldHappenBetween - [exemple](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldHappenBetween.yml)
* ShouldTimeEqual - [exemple](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldTimeEqual.yml)
* ShouldMatchRegex - [exemple](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldMatchRegex.yml)
* ShouldJSONEqual - [exemple](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldJSONEqual.yml)
* ShouldNotJSONEqual - [exemple](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldNotJSONEqual.yml)

#### Mots-clés `Must` {#must-keywords}

Tous les mots-clés d'assertion ci-dessus ont aussi un équivalent `Must`, qui permet de créer une assertion obligatoire et d'empêcher les cas de test (et les executors personnalisés) d'exécuter les étapes restantes.

Exemple :
```yml
- steps:
  - type: exec
    script: exit 1
    assertions:
      - result.code MustEqual 0
  # Remaining steps in this context will not be executed
```

#### Assertions utilisateur {#user-assertions}

Il est possible d'utiliser la [syntaxe des executors définis par l'utilisateur](#user-defined-executors) en préfixant le nom de votre executor par `Should` pour créer un nouveau mot-clé d'« assertion utilisateur ».

Exemple :
```yml
# lib/ShouldBeHttp2XX.yml
executor: ShouldBeHttp2XX
steps:
  - assertions:
    - a ShouldBeGreaterThanOrEqualTo 200
    - a ShouldBeLessThan 300
```

Vous pouvez aussi inclure des étapes supplémentaires, comme dans un executor défini par l'utilisateur ordinaire.

Les assertions utilisateur s'exécutent dans un contexte entièrement vierge, qui ne contient que les variables suivantes :
- `a` : l'opérande de gauche
- `b` : la (première) opérande de droite
- `argv` : les opérandes de droite

Si vous devez rester compatible avec la syntaxe `input` des executors définis par l'utilisateur, vous pouvez utiliser `argv` comme valeur par défaut et y accéder par la syntaxe habituelle `input.*`. 
```yaml
input:
  test: "{{.argv.argv1}}"
```

Pour appeler une assertion utilisateur enregistrée, utilisez simplement son nom enregistré dans le tableau `assertions`, comme n'importe quel mot-clé d'assertion intégré.

```yml
# test.yml
testcases:
- steps:
  - type: http
    url: https://example.com
    assertions:
    - result.statuscode ShouldBeHttp2XX
```

### Utiliser des opérateurs logiques {#using-logical-operators}

Alors que les assertions utilisent implicitement l'opérateur `and`, il est possible d'utiliser d'autres opérateurs logiques pour écrire des assertions complexes.

Les opérateurs pris en charge sont `and`, `or` et `xor`.

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

D'autres exemples sont disponibles dans [`tests/assertions_operators.yml`](/tests/assertions_operators.yml).

