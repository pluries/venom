---
translates: testsuites
title: 测试套件
description: 如何编写测试套件及其测试用例。
fingerprint: 692000e4f0fd4c5e632c7755229552c8524f88d83321a5f7cb777257e8c3018a
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# 测试套件 {#testsuites}

测试套件是一组测试用例的集合，用来测试某个软件程序，以证明它具备一组指定的行为。
测试用例是对输入、执行条件、测试流程和预期结果的规格说明，它定义了一个为达成特定软件测试目标而执行的单项测试，例如覆盖某条特定的程序路径，或验证是否符合某项具体需求。

在 `venom` 中，同一个测试套件内的测试用例按顺序执行。每个测试用例是一组有序的步骤。每个步骤都基于一个 `executor`，由它提供某种特定的行为。

在 `venom` 中，一个测试套件写在一个 `YAML` 文件中，并遵循如下结构：

```yaml

name: Title of TestSuite
description: A detailed description of the TestSuite, in markdown.
testcases:
- name: TestCase with default value, exec cmd. Check if exit code != 1
  steps:
  - script: echo 'foo'
    type: exec

- name: Title of First TestCase
  steps:
  - script: echo 'foo'
    assertions:
    - result.code ShouldEqual 0
  - script: echo 'bar'
    assertions:
    - result.systemout ShouldNotContainSubstring foo
    - result.timeseconds ShouldBeLessThan 1

- name: GET http testcase, with 5 seconds timeout
  steps:
  - type: http
    method: GET
    url: https://eu.api.ovh.com/1.0/
    timeout: 5
    assertions:
    - result.body ShouldContainSubstring /dedicated/server
    - result.body ShouldContainSubstring /ipLoadbalancing
    - result.statuscode ShouldEqual 200
    - result.timeseconds ShouldBeLessThan 1

- name: Test with retries and delay in seconds between each try
  steps:
  - type: http
    method: GET
    url: https://eu.api.ovh.com/1.0/
    retry: 3
    retry_if: # (optional, lets you early break unrecoverable errors)
    - result.statuscode ShouldNotEqual 403
    delay: 2
    assertions:
    - result.statuscode ShouldEqual 200

```

### HTTP multipart form {#http-multipart-form}

`multipart_form` 会把请求体作为 `multipart/form-data` 发送。它不能与 `body` 或 `bodyFile` 同时使用。完整的字段列表请参阅 [HTTP executor](https://github.com/ovh/venom/tree/master/executors/http)。

每个键是一个表单字段。值可以是单个字符串，也可以是字符串列表，列表中的每一项都会以同一个字段名各发送一次（例如多文件上传时重复出现的 `files` 部分）。

以 `@` 开头的值是要上传的文件路径，相对于当前工作目录解析。在其后追加 `;type=<content-type>` 即可设置该部分的 Content-Type，默认值为 `application/octet-stream`。这与 curl 的 `-F field=@path;type=...` 一致。

```yaml
- name: upload files
  steps:
  - type: http
    method: POST
    url: https://httpbin.org/post
    multipart_form:
      description: "a plain text field"
      tags:
        - "first"
        - "second"
      files:
        - "@./testdata/report.pdf;type=application/pdf"
        - "@./testdata/notes.txt;type=text/plain"
    assertions:
    - result.statuscode ShouldEqual 200
```
