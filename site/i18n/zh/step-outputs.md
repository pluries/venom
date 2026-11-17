---
translates: step-outputs
title: 把一个测试步骤的输出用作另一个测试步骤的输入
description: 步骤如何把它的结果传给下一个步骤。
fingerprint: 0b48c4fa6f51fc685b64e68b3c67da6261ca7863c29554e824ea525eac5d0679
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# 把一个测试步骤的输出用作另一个测试步骤的输入 {#use-outputs-from-a-test-step-as-input-of-another-test-step}

要在后续的测试用例或步骤中复用某个测试步骤的属性，必须先提取该变量，如下例所示。

第一个步骤执行之后，`venom` 使用正则表达式 `foo with a ([a-z]+) here`，从 `result.systemout` 属性的内容中提取一个值，该属性由 `executor` 返回。
随后，这个变量可在其他测试中以 `testA.myvariable` 的名称复用，其中 `testA` 对应测试用例的名称。如果无法从输出中提取该变量（解析 JSON 输出时常会出现这种情况），还可以提供一个默认值。

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
