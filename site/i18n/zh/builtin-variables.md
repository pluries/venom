---
translates: builtin-variables
title: venom 内置变量
description: venom 为您设置的变量。
fingerprint: 57868302669e41f5bae786f964d1e4ad3735ca9dcc92970c85f06a769329fbae
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# venom 内置变量 {#builtin-venom-variables}

```yaml
name: MyTestSuite
testcases:
- name: testA
  steps:
  - type: exec
    script: echo '{{.venom.testsuite}} {{.venom.testsuite.filename}} {{.venom.testcase}} {{.venom.teststep.number}} {{.venom.datetime}} {{.venom.timestamp}} {{.venom.testcase.totalSteps}} {{.venom.testsuite.totalSteps}}'
    # will display something as: MyTestSuite MyTestSuiteWithVenomBuiltinVar.yml testA 0 2018-08-05T21:38:24+02:00 1533497904 3 5

```

内置变量：

* {{.venom.datetime}}
* {{.venom.executable}}
* {{.venom.libdir}}
* {{.venom.outputdir}}
* {{.venom.testcase}}
* {{.venom.testcase.totalSteps}}
* {{.venom.teststep.number}}
* {{.venom.testsuite.name}}
* {{.venom.testsuite.filename}}
* {{.venom.testsuite.filepath}}
* {{.venom.testsuite.shortName}}
* {{.venom.testsuite.workdir}}
* {{.venom.testsuite}}
* {{.venom.testsuite.totalSteps}}
* {{.venom.timestamp}}


### 机密变量 {#secrets-variables}

示例：

```yml
name: Your Testsuite
vars:
  foo : this-value-is-secret
secrets:
  - foo
testcases:
- name: myvar_first
  steps:
  - type: exec
    script: "echo myvar {{.foo}}"
```

值 `this-value-is-secret` 不会打印在您的控制台、`venom.log` 和 `...dump.json` 文件中。

#### 哪些会被隐藏，哪些不会 {#what-is-hidden-and-what-is-not}

venom 按值来隐藏机密，凡是运行过程会打印或写入该值的地方都在其列：控制台、`venom.log`、各类报告和 `...dump.json` 文件，包括步骤输出、名称、assertion 文本和错误消息。

会被隐藏的：

- 在 `secrets:`（套件中）或 `secrets`（`.venomrc` 中）里声明的每一个值；
- 该值的 base64 形式，对于 `basic_auth_password`，还包括 `user:password` 的 base64；
- venom 在测试用例名称中给出的 slug 形式（仅当其长度至少为 4 个字符时）；
- URL 编码形式：查询串（`a+b%21`）和路径（`a%20b%21`），前提是它们与原值不同且长度至少为 4 个字符。

不会被隐藏的：

- 您没有声明的值，即使它来自机密来源；
- 任何其他变换：十六进制、HTML 实体、哈希、签名、被切成若干片段或与其他文本拼接的值，以及 JSON 转义后的形式。

当某个步骤可能以另一种形式打印机密时，请避免打印它，或者把变换后的值也声明到 `secrets:` 中。
