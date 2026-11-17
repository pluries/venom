---
translates: variables
title: 变量
description: 如何在套件中定义和使用变量。
fingerprint: 571c5e16517c3ed7ec39bc40546cba3ab0da6e9c0ebd3a0e1e59d6875364590b
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# 变量 {#variables}

### 测试套件变量 {#testsuite-variables}

您可以在 `testsuite` 级别定义变量。

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

测试套件中用到的每个用户变量都必须在这一部分声明。您可以通过多种方式在运行时覆盖它的值：
- 单独使用命令行选项 `--var`。
- 通过变量定义文件，在命令行上用 `--var-from-file` 指定。
- 通过环境变量。



### 遇到未知变量时让步骤失败 {#fail-a-step-on-an-unknown-variable}

默认情况下，venom 会把它不认识的 `{{.name}}` 原样保留在步骤中，因此拼错的变量（例如 `{{.nmae}}`）会作为文本发送出去，测试仍可能通过。使用 `--strict-variables` 后，只要某个步骤引用了未知变量，它就会在运行之前失败，并且消息中会指明该步骤和该变量：

```
step #2: unknown variable(s) "nmae"
```

该设置默认关闭，并作用于整个运行。可以通过 `--strict-variables`、`VENOM_STRICT_VARIABLES=true`，或 `strict_variables: true`（写在 `.venomrc` 中）来设置。

下列情况永远不会被报告：由前面的步骤或 `vars` 设置的变量、range 变量（`key`、`value`、`index`）、`venom.*` 变量、使用 `default` 或 `ternary` 的表达式，以及 `{{.result...}}`（venom 在步骤运行之后，才在 `info`、断言和提取的变量中解析它们）。未知的辅助函数名称不会被检查。

有一个限制：venom 会先读取整个文件，并在步骤级别对每个仅由字母、数字、`.`、`-`、`_`、`"`、`|` 和空格组成的表达式求值。含有任何其他字符的表达式，例如 `{{ printf "%s" .a }}` 或 `{{.url | trimSuffix "/"}}`，会在读取文件时、步骤运行之前求值，因此严格模式看不到它，其中拼错的变量仍会得到空值或错误的值，而不会报错。

如果某个步骤需要让文本 `{{.something}}` 原封不动地传给另一个工具，例如 `docker ... --format '{{.ID}}'`，它在严格模式下会失败。venom 没有针对字面双花括号的转义方式，因此对这样的套件请保持该设置关闭。

### 变量辅助函数 {#variable-helpers}

可用的辅助函数及部分示例：

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
- `escape`: 把 ‘_‘、‘/’、‘.’ 替换为 ‘-’

更多示例见[此处](https://github.com/ovh/venom/tree/master/variable_helpers.md)
