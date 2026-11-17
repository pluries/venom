---
translates: executors-concept
title: Executor
description: executor 是什么，以及 venom 自带哪些 executor。
fingerprint: a4853e15b94adabbd472d94df4fa4fe4dded85edb7b601378c3188c9b5214206
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# Executor {#executors}

* **amqp**: https://github.com/ovh/venom/tree/master/executors/amqp
* **couchbase**: https://github.com/ovh/venom/tree/master/executors/couchbase
* **dbfixtures**: https://github.com/ovh/venom/tree/master/executors/dbfixtures
* **exec**: https://github.com/ovh/venom/tree/master/executors/exec `exec` 是步骤的默认类型
* **grpc**: https://github.com/ovh/venom/tree/master/executors/grpc
* **http**: https://github.com/ovh/venom/tree/master/executors/http
* **imap**: https://github.com/ovh/venom/tree/master/executors/imap
* **kafka** https://github.com/ovh/venom/tree/master/executors/kafka
* **mcp**: https://github.com/ovh/venom/tree/master/executors/mcp
* **mobile**: https://github.com/ovh/venom/tree/master/executors/mobile
* **mongo**: https://github.com/ovh/venom/tree/master/executors/mongo
* **mqtt** https://github.com/ovh/venom/tree/master/executors/mqtt
* **odbc**: https://github.com/ovh/venom/tree/master/executors/plugins/odbc
* **ovhapi**: https://github.com/ovh/venom/tree/master/executors/ovhapi
* **rabbitmq**: https://github.com/ovh/venom/tree/master/executors/rabbitmq
* **readfile**: https://github.com/ovh/venom/tree/master/executors/readfile
* **redis**: https://github.com/ovh/venom/tree/master/executors/redis
* **smtp**: https://github.com/ovh/venom/tree/master/executors/smtp
* **sql**: https://github.com/ovh/venom/tree/master/executors/sql
* **ssh**: https://github.com/ovh/venom/tree/master/executors/ssh
* **web**: https://github.com/ovh/venom/tree/master/executors/web

`odbc` 和您自己的 `lib/*.so` executor 都是 Go 插件：它们只能加载到与其构建时完全相同的 venom 和 Go 版本中，因此每次升级后都要重新构建（参见 [executors/plugins](executors/plugins/README.md)）。

### 用户自定义 executor {#user-defined-executors}

您可以用一个 YAML 文件定义一个 executor。这是抽象技术行为或业务行为的好办法，便于在复杂的测试套件中复用。

示例：

文件 `lib/customA.yml`：

```yml
executor: hello
input:
  myarg: {}
steps:
- script: echo "{\"hello\":\"{{.input.myarg}}\"}"
  assertions:
  - result.code ShouldEqual 0
  vars:
    hello:
      from: result.systemoutjson.hello
    all:
      from: result.systemoutjson
output:
  display:
    hello: "{{.hello}}"
  all: "{{.all}}"
```

文件 `testsuite.yml`：

```yml
name: testsuite with a user executor
testcases:
- name: testA
  steps:
  - type: hello
    myarg: World
    assertions:
    - result.display.hello ShouldContainSubstring World
    - result.alljson.hello ShouldContainSubstring World
```

请注意变量 `alljson`。在 output 中声明的所有变量都会自动转换为 JSON 格式，并带上后缀 `json`。在上面的示例中，有两个隐式变量可用：`displayjson.hello` 和 `alljson`。

Venom 会从相对于测试套件路径的 `lib/` 目录加载用户自定义 executor。您可以用 `--lib-dir` 标志添加 executor 的源码路径。
请注意，`--lib-dir` 列出的所有文件夹都会被递归扫描，以找到作为用户 executor 的 `.yml` 文件。

重试无法使用户 executor 的步骤从其内部步骤的失败中恢复。对使用用户 executor 的步骤设置 `retry: 2` 时，如果某个内部步骤在第一次尝试中失败，该步骤就一直保持失败，即使之后的每次尝试都通过（无论有没有 `timeout`）。内置 executor 的行为则不同：只有最后一次尝试起决定作用。要重试不稳定的操作，请把 `retry` 放在用户 executor 内部的那个步骤上，或者改用内置 executor。

用户自定义 executor 支持模板，您可以在 `venom.log` 中查看模板的渲染结果。如果在该文件中看到类似 `error converting YAML to JSON: yaml: line 14: found unexpected end of stream` 的错误，您很可能需要用模板函数 `indent` 调整缩进。

示例：

```yml
name: testsuite with a user executor multilines
testcases:
- name: test
  steps:
  - type: multilines
    script: |
      # test multilines
      echo "5"
    assertions:
    - result.alljson ShouldEqual 5
```

使用的 executor 如下：

```yml
executor: multilines
input:
  script: "echo 'foo'"
steps:
- type: exec
  script: {{ .input.script | nindent 4 }}
  assertions:
  - result.code ShouldEqual 0
  vars:
    all:
      from: result.systemoutjson
output:
  all: '{{.all}}'
```


```bash
# lib/*.yml files will be loaded as executors.
$ venom run testsuite.yml 

# executors will be loaded from /etc/venom/lib, $HOME/venom.d/lib and lib/ directory relative to testsuite.yml file.
$ venom run --lib-dir=/etc/venom/lib:$HOME/venom.d/lib testsuite.yml 
```
