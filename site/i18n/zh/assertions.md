---
translates: assertions
title: 断言
description: 可用于检查测试步骤结果的断言。
fingerprint: e848fd058b87a06a7e564aacd53bf6df2867aceff122501a2ebea797b0529545
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# 断言 {#assertions}

### 关键字 {#keywords}

* ShouldEqual - [示例](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldEqual.yml)
* ShouldNotEqual - [示例](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldNotEqual.yml)
* ShouldAlmostEqual - [示例](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldAlmostEqual.yml)
* ShouldNotAlmostEqual - [示例](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldNotAlmostEqual.yml)
* ShouldBeNil - [示例](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldBeNil.yml)
* ShouldNotBeNil - [示例](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldNotBeNil.yml)
* ShouldBeTrue - [示例](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldBeTrue.yml)
* ShouldBeFalse - [示例](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldBeFalse.yml)
* ShouldBeZeroValue - [示例](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldBeZeroValue.yml)
* ShouldBeGreaterThan - [示例](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldBeGreaterThan.yml)
* ShouldBeGreaterThanOrEqualTo - [示例](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldBeGreaterThanOrEqualTo.yml)
* ShouldBeLessThan - [示例](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldBeLessThan.yml)
* ShouldBeLessThanOrEqualTo - [示例](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldBeLessThanOrEqualTo.yml)
* ShouldBeBetween - [示例](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldBeBetween.yml)
* ShouldNotBeBetween - [示例](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldNotBeBetween.yml)
* ShouldBeBetweenOrEqual - [示例](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldBeBetweenOrEqual.yml)
* ShouldNotBeBetweenOrEqual - [示例](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldNotBeBetweenOrEqual.yml)
* ShouldContain - [示例](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldContain.yml)
* ShouldNotContain - [示例](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldNotContain.yml)
* ShouldJSONContain - [示例](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldJSONContain.yml)
* ShouldNotJSONContain - [示例](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldNotJSONContain.yml)
* ShouldJSONContainWithKey - [示例](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldJSONContainWithKey.yml)
* ShouldJSONContainAllWithKey - [示例](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldJSONContainAllWithKey.yml)
* ShouldNotJSONContainWithKey - [示例](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldNotJSONContainWithKey.yml)
* ShouldContainKey - [示例](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldContainKey.yml)
* ShouldNotContainKey - [示例](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldNotContainKey.yml)
* ShouldBeIn - [示例](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldBeIn.yml)
* ShouldNotBeIn - [示例](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldNotBeIn.yml)
* ShouldBeEmpty - [示例](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldBeEmpty.yml)
* ShouldNotBeEmpty - [示例](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldNotBeEmpty.yml)
* ShouldBeArray - [示例](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldBeArray.yml)
* ShouldBeMap - [示例](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldBeMap.yml)
* ShouldHaveLength - [示例](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldHaveLength.yml)
* ShouldStartWith - [示例](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldStartWith.yml)
* ShouldNotStartWith - [示例](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldNotStartWith.yml)
* ShouldEndWith - [示例](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldEndWith.yml)
* ShouldNotEndWith - [示例](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldNotEndWith.yml)
* ShouldBeBlank - [示例](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldBeBlank.yml)
* ShouldNotBeBlank - [示例](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldNotBeBlank.yml)
* ShouldContainSubstring - [示例](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldContainSubstring.yml)
* ShouldNotContainSubstring - [示例](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldNotContainSubstring.yml)
* ShouldEqualTrimSpace - [示例](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldEqualTrimSpace.yml)
* ShouldNotExist - [示例](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldNotExist.yml)
* ShouldHappenBefore - [示例](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldHappenBefore.yml)
* ShouldHappenOnOrBefore - [示例](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldHappenOnOrBefore.yml)
* ShouldHappenAfter - [示例](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldHappenAfter.yml)
* ShouldHappenOnOrAfter - [示例](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldHappenOnOrAfter.yml)
* ShouldHappenBetween - [示例](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldHappenBetween.yml)
* ShouldTimeEqual - [示例](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldTimeEqual.yml)
* ShouldMatchRegex - [示例](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldMatchRegex.yml)
* ShouldJSONEqual - [示例](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldJSONEqual.yml)
* ShouldNotJSONEqual - [示例](https://github.com/ovh/venom/tree/master/tests/assertions/ShouldNotJSONEqual.yml)

#### `Must` 关键字 {#must-keywords}

上述所有断言关键字都有对应的 `Must` 版本，可用来创建必须通过的断言，并阻止测试用例（以及自定义 executor）运行其余的步骤。

示例：
```yml
- steps:
  - type: exec
    script: exit 1
    assertions:
      - result.code MustEqual 0
  # Remaining steps in this context will not be executed
```

#### 用户断言 {#user-assertions}

可以使用[用户自定义 executor 的语法](#user-defined-executors)，并在您的 executor 名称前加上 `Should` 前缀，来创建一个新的“用户断言”关键字。

示例：
```yml
# lib/ShouldBeHttp2XX.yml
executor: ShouldBeHttp2XX
steps:
  - assertions:
    - a ShouldBeGreaterThanOrEqualTo 200
    - a ShouldBeLessThan 300
```

您也可以像普通的用户自定义 executor 一样包含额外的步骤。

用户断言在一个完全干净的上下文中执行，其中只包含以下变量：
- `a`：左操作数
- `b`：（第一个）右操作数
- `argv`：右操作数

如果需要与用户自定义 executor 的 `input` 语法兼容，可以把 `argv` 用作默认值，并通过常规的 `input.*` 语法访问它们。
```yaml
input:
  test: "{{.argv.argv1}}"
```

要调用已注册的用户断言，只需在 `assertions` 数组中使用它的注册名称，就像使用任何内置断言关键字一样。

```yml
# test.yml
testcases:
- steps:
  - type: http
    url: https://example.com
    assertions:
    - result.statuscode ShouldBeHttp2XX
```

### 使用逻辑运算符 {#using-logical-operators}

断言之间默认隐式使用 `and` 运算符，但也可以使用其他逻辑运算符来编写复杂的断言。

支持的运算符有 `and`、`or` 和 `xor`。

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

更多示例见 [`tests/assertions_operators.yml`](/tests/assertions_operators.yml)。

