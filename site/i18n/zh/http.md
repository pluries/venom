---
translates: http
title: HTTP
description: 如何在 venom 测试套件中使用 HTTP executor。
fingerprint: 859e45938b22aae5cb32f1766961507301c17073180a5535014e3adc8f77cecb
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# Venom - HTTP Executor {#venom---executor-http}

用于执行 HTTP 请求的步骤

## 输入 {#input}
在您的 yaml 文件中，可以使用：

```yaml
  - method (optional), default value: GET
  - url mandatory
  - unix_sock (optional)
  - path (optional)
  - query_parameters (optional)
  - body (optional)
  - bodyFile (optional)
  - multipart_form (optional): cannot be combined with body or bodyFile
  - preserve_bodyfile (optional) skip file content interpolation
  - headers (optional)
  - proxy (optional): set to use a proxy server for connection to url
  - resolve (optional): array of custom resolver of host and port pair. example: foo.com:443:127.0.0.1
  - ignore_verify_ssl (optional): set to true if you use a self-signed SSL on remote for example
  - basic_auth_user (optional): username to use for HTTP basic authentication
  - basic_auth_password (optional): password to use for HTTP basic authentication
  - no_follow_redirect (optional): indicates that you don't want to follow Location if server returns a Redirect (301/302/...)
  - skip_body: skip the body and bodyjson result
  - skip_headers: skip the headers result
  - tls_client_cert (optional): a chain of certificates to identify the caller, first certificate in the chain is considered as the leaf, followed by intermediates. Setting it enables mutual TLS authentication, and the certificate is sent even when ignore_verify_ssl is true. Set the PEM content or the path to the PEM file. It needs tls_client_key: a certificate without its key fails the step.
  - tls_client_key (optional): private key corresponding to the certificate. Set the PEM content or the path to the PEM file. It needs tls_client_cert: a key without its certificate fails the step.
  - tls_root_ca (optional): defines additional root CAs to perform the call. Can contain multiple CAs concatenated together. Set the PEM content or the path to the PEM file.
  - tls_root_ca, tls_client_cert and tls_client_key only serve TLS connections. With an `http://` url the step logs a warning; a redirect to `https://` or an `https://` proxy still uses them.

```

```yaml

name: HTTP testsuite
testcases:
- name: get http testcase
  steps:
  - type: http
    method: GET
    url: https://eu.api.ovh.com/1.0/
    assertions:
    - result.body ShouldContainSubstring /dedicated/server
    - result.body ShouldContainSubstring /ipLoadbalancing
    - result.statuscode ShouldEqual 200
    - result.bodyjson.api ShouldBeNil
    - result.bodyjson.apis ShouldNotBeEmpty
    - result.bodyjson.apis.apis0 ShouldNotBeNil
    - result.bodyjson.apis.apis0.path ShouldEqual /allDom

- name: post http multipart
  steps:
  - type: http
    method: POST
    url: https://eu.api.ovh.com/1.0/auth/logout
    multipart_form:
      file: '@./venom.gif'
    assertions:
    - result.statuscode ShouldEqual 401
    vars:
      statuscode:
        from: result.statuscode

- name: post http with body
  steps:
    - type: http
      method: POST
      url: https://httpbin.org/post
      body: |
        {"key": "value"}
      headers:
        Content-Type: application/json
      assertions:
        - result.statuscode ShouldEqual 200
        - result.bodyjson.url ShouldEqual https://httpbin.org/post
      vars:
        statuscode:
          from: result.statuscode
          
- name: post http enhanced assertions
  steps:
  - type: http
    method: POST
    url: https://eu.api.ovh.com/1.0/newAccount/rules
    assertions:
      - result.statuscode ShouldEqual 200
      - result.bodyjson.__Type__ ShouldEqual Array
      # Ensure a minimum of fields are present.
      - result.bodyjson.__Len__ ShouldBeGreaterThanOrEqualTo 8
      # Ensure fields have the right keys.
      - result.bodyjson.bodyjson0 ShouldContainKey fieldName
      - result.bodyjson.bodyjson0 ShouldContainKey mandatory
      - result.bodyjson.bodyjson0 ShouldContainKey regularExpression
      - result.bodyjson.bodyjson0 ShouldContainKey prefix
      - result.bodyjson.bodyjson0 ShouldContainKey examples
      - result.bodyjson.bodyjson0 ShouldNotContainKey lol
      - result.statuscode ShouldNotEqual {{.post-http-multipart.statuscode}}

- name: get http (with options)
  steps:
  - type: http
    method: POST
    url: https://eu.api.ovh.com/1.0
    skip_body: true
    skip_headers: true
    info: request is {{.result.request.method}} {{.result.request.url}} {{.result.request.body}}
    assertions:
      - result.statuscode ShouldEqual 405
      - result.body ShouldBeEmpty
      - result.headers ShouldBeEmpty


```

## Multipart form {#multipart-form}

`multipart_form` 会把请求体作为 `multipart/form-data` 发送。它不能与 `body` 或 `bodyFile` 同时使用。

每个键是一个表单字段。值可以是单个字符串，也可以是字符串列表，
列表中的每一项都会以同一个字段名各发送一次（例如多文件上传时重复出现的 `files` 部分）。

以 `@` 开头的值是要上传的文件路径，相对于当前工作目录解析。
在其后追加 `;type=<content-type>` 即可设置该部分的 Content-Type，默认值为 `application/octet-stream`。
这与 curl 的 `-F field=@path;type=...` 一致。

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

## 输出 {#output}

```
result.request
result.timeseconds
result.statuscode
result.body
result.bodyjson
result.headers
result.err
```
- result.timeseconds：执行耗时
- result.request.method：请求的 HTTP 方法
- result.request.url：请求的 HTTP URL
- result.request.body：字符串形式的请求体内容
- result.request.form：HTTP 表单映射
- result.request.post_form：HTTP post 表单映射
- result.err：如果存在，该字段包含错误信息
- result.body：HTTP 响应的响应体
- result.bodyjson：如果 HTTP 响应体是 JSON，则为其解析结果。例如，您可以用 result.bodyjson.yourkey 访问其中的 json 数据。
- result.headers：HTTP 响应的响应头
- result.statuscode：HTTP 响应的状态码

### JSON 键 {#json-keys}

JSON 键会被自动转换为小写（例如，请使用 `result.bodyjson.yourkey`，而不是
`result.bodyjson.YourKey`）。

除此之外，如果 JSON 键包含特殊字符，这些字符会被转换为下划线。

### JSON 数组 {#json-arrays}

当 HTTP 响应包含 JSON 数组时，必须使用如下语法来访问数组中的特定键：
`result.bodyjson.array_name.array_name_index_in_array.key`

示例：如果要获取 `path` 键的值，且该键属于 `apis` 数组的*第二个*元素：`result.bodyjson.apis.apis1.path`


## 默认断言 {#default-assertion}

```yaml
result.statuscode ShouldEqual 200
```

## Cookie {#cookies}

在单个步骤内跟随重定向时，Cookie 会被自动处理。
在相邻的步骤之间不支持 Cookie。
