---
translates: readfile
title: 读取文件
description: 如何在 venom 测试套件中使用 Read file executor。
fingerprint: 8370675caf1915a0b8257d34f10772113674ba996cf21e009ccff6d505434aaa
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# Venom - Executor Read file {#venom---executor-read-file}

用于读取文件的步骤

使用场景：您的应用程序会在磁盘上写入一个文件。Venom 可以检查该文件是否已生成、读取它，
并返回其内容。该内容可以被测试套件的其他步骤使用。

路径可以是完整路径、通配符或目录：

```
- path: /a/full/path/file.txt
- path: afile.txt
- path: *.yml
- path: a_directory/
- path: ./foo/b*/**/z*.txt
```

## 输入 {#input}

```yaml
name: TestSuite Read File
testcases:
- name: TestCase Read File
  steps:
  - type: readfile
    path: yourfile.txt
    assertions:
    - result.err ShouldBeEmpty
```
```yaml
name: TestSuite Read Yaml File
testcases:
  - name: Read File
    steps:
      - type: readfile
        path: yourfile.yml
        assertions:
          - result.err ShouldBeEmpty
```

## 输出 {#output}

```yaml
  result.timeseconds
  result.content
  result.contentjson
  result.size.filename
  result.md5sum.filename
  result.modtime.filename
  result.mod.filename
```

- result.timeseconds：执行耗时
- result.err：如果文件不存在，该字段包含一条错误
- result.content：所读取文件的内容
- result.contentjson：所读取文件的内容（如果它是 JSON 文件）。例如，您可以用 result.contentjson.yourkey 访问 JSON 数据
- result.size.filename：文件 'filename' 的大小
- result.md5sum.filename：文件 'filename' 的 md5
- result.modtime.filename：文件 'filename' 的修改时间，例如：1487698253
- result.mod.filename：文件 'filename' 的权限，例如：-rw-r--r--

注意：'filename' 的值等于路径，其中所有字母均为小写，所有 '/' 都已替换为 '_'。

## 默认断言 {#default-assertion}

```yaml
result.err ShouldBeEmpty
```

## 示例 {#example}

文件 testa.txt：

```
simple content
multilines
```

文件 testa.json：

```json
{
  "foo": "bar"
}
```

文件 testb.json：

```json
[
  {
    "foo": "bar",
    "foo2": "bar2"
  }
]

```

venom 测试文件：

```yaml
name: TestSuite Read File
testcases:
- name: TestCase Read File
  steps:
  - type: readfile
    path: testa.json
    assertions:
      - result.contentjson.foo ShouldEqual bar

  - type: readfile
    path: testb.json
    assertions:
      - result.contentjson.contentjson0.foo2 ShouldEqual bar2

  - type: readfile
    path: testa.txt
    assertions:
      - result.content ShouldContainSubstring multilines
```
