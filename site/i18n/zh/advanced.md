---
translates: advanced
title: 高级用法
description: 面向大型测试套件的选项。
fingerprint: a7992da938eb488d1fc5fa9a37e1159081f4a0bf684768a8c2d34b4bb13924ff
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# 高级用法 {#advanced-usage}

## 调试您的测试套件 {#debug-your-testsuites}

每次执行 `venom run` 命令都会生成一个 *venom.log* 文件。

调试测试套件有两种方式：
 - 在 venom 二进制文件上使用 `-v` 选项。
   - `$ venom run -v test.yml` 会输出每个测试步骤的详细信息
   - `$ venom run -vv test.yml` 会为每个测试步骤生成 *dump.json* 文件。
   - `$ venom run -vvv test.yml` 会生成 *pprof* 文件，每个测试文件一份 CPU 分析文件和一份内存分析文件，在输出目录中分别命名为 `pprof_cpu_profile_<file>.prof` 和 `pprof_mem_profile_<file>.prof`。用 `go tool pprof` 打开它们。分析是按文件进行的，这样您就能看出哪个测试套件慢，这也是 `-vvv` 不能与 `--parallel 2` 或更大的值同时使用的原因：Go 的 CPU 性能分析器作用于整个进程。
 - 在测试步骤中使用 `info` 关键字：
`test.yml` 文件：
```yml
name: Exec testsuite
testcases:
- name: testA
  steps:
  - type: exec
    script: echo 'foo with a bar here'
    info:
      - this a first info
      - and a second...
- name: cat json
  steps:
  - script: cat exec/testa.json
    info: "the value of result.systemoutjson is {{.result.systemoutjson}}"
    assertions:
    - result.systemoutjson.foo ShouldContainSubstring bar
```

```bash
$ venom run test.yml

# output:

 • Exec testsuite (exec.yml)
  • testA SUCCESS
    [info] this a first info (exec.yml:8)
    [info] and a second... (exec.yml:9)
  • testB SUCCESS
  • sleep 1 SUCCESS
  • cat json SUCCESS
    [info] the value of result.systemoutjson is map[foo:bar] (exec.yml:34)
```

## 跳过测试用例和测试步骤 {#skip-testcase-and-teststeps}

可以跳过 `testcase`，依据是某些 `assertions`。例如，下面的示例会跳过最后一个测试用例。

```yaml
name: "Skip testsuite"
vars:
  foo: bar

testcases:
- name: init
  steps:
  - type: exec
    script: echo {{.foo}}
    assertions:
    - result.code ShouldEqual 0
    - result.systemout ShouldContainSubstring bar

- name: do-not-skip-this
  skip: 
  - foo ShouldNotBeEmpty
  steps:
  - type: exec
    script: exit 0

- name: skip-this
  skip: 
    - foo ShouldBeEmpty
  steps:
  - type: exec
    script: command_not_found
    assertions:
    - result.code ShouldEqual 0

```

`skip` 语句也可以放在测试步骤级别，以便只执行测试用例的一部分。只要 skip 块中有一个条件不成立，该步骤就会被跳过。

如果一个测试用例的所有步骤都被跳过，该测试用例本身也会被视为“已跳过”，而不是“通过”或“失败”。

```yaml
name: "Skip testsuite"
vars:
  foo: bar

testcases:
- name: skip-one-of-these
  steps:
  - name: do-not-skip-this
    type: exec
    script: exit 0
    assertions:
    - result.code ShouldEqual 0
    skip:
    - foo ShouldNotBeEmpty
  - name: skip-this
    type: exec
    script: exit 1
    assertions:
    - result.code ShouldEqual 0
    skip:
    - foo ShouldBeEmpty

```

## 遍历数据 {#iterating-over-data}

可以用 `range` 属性遍历数据。

支持以下数据类型，每种类型都会提供上下文变量 `.index`、`.key` 和 `.value`：

- 数组，其中每个值都会被遍历 (`[]interface{}`)
  - `.index`/`.key`：当前迭代的索引
  - `.value`：当前迭代项的值
- 映射，其中每个键都会被遍历 (`map[string]interface{}`)
  - `.index`：当前迭代的索引
  - `.key`：当前迭代项的键
  - `.value`：当前迭代项的值
- 整数，使目标步骤执行 `n` 次 (`int`)
  - `.index`/`.key`/`.value`：当前迭代的索引
- 模板字符串，其结果是上述类型之一 (`string`)
  - 它可以继承自变量文件，也可以由前一个步骤的结果插值得到

例如，下面的示例会遍历一个包含两个映射项的数组：
```yaml
- name: range with hardcoded array
  steps:
  - type: exec
    range:
      - actual: hello
        expected: hello
      - actual: world
        expected: world
    script: echo "{{.value.actual}}"
    assertions:
    - result.code ShouldEqual 0
    - result.systemout ShouldEqual "{{.value.expected}}"
```

更多示例见 [`tests/ranged.yml`](/tests/ranged.yml)。
