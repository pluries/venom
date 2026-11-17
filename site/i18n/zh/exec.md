---
translates: exec
title: Exec
description: 如何在 venom 测试套件中使用 Exec executor。
fingerprint: fc1f5ab5e0397848ca237c08b3e7fd811ce670d0c6a309363d41c0481b77f07e
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# Venom - Exec Executor {#venom---executor-exec}

用于执行脚本的步骤。

脚本以运行 venom 的用户的权限执行。请勿把步骤的结果或任何不受信任的值不加引号地放进脚本中：参见[根目录 README](../../README.md#trust-model) 中的信任模型。


## 输入 {#input}

示例

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

多行脚本：

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

输入：

```yaml
name: Title of TestSuite
testcases:
- name: with stdin
  steps:
  - type: exec
    stdin: Foo
    script: cat
```

显式命令（不经过 shell）：

```yaml
name: Title of TestSuite
testcases:
- name: explicit command
  steps:
  - type: exec
    stdin: "{\"foo\":\"bar\"}"
    command: ["jq", ".foo"]
```

## 输出 {#output}

```yaml
systemout
systemoutjson
systemerr
systemerrjson
err
code
timeseconds
```

- result.timeseconds：执行耗时
- result.err：如果存在，该字段包含错误详情
- result.systemout：所执行脚本的标准输出
- result.systemoutjson：所执行脚本的标准输出，解析为 JSON 对象
- result.systemerr：所执行脚本的错误输出
- result.systemerrjson：所执行脚本的错误输出，解析为 JSON 对象
- result.code：退出码

## 默认断言 {#default-assertion}

```yaml
result.code ShouldEqual 0
```
