---
translates: grpc
title: Grpc
description: 如何在 venom 测试套件中使用 Grpc executor。
fingerprint: bc75454d8a0a9263d05f80a569e9e4373069e488aef05a31e37fabcb31c2c402
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# Venom - Executor Grpc {#venom---executor-grpc}

执行 gRPC 请求的步骤

基于 `grpcurl`，更多信息请参见 [grpcurl](https://github.com/fullstorydev/grpcurl)。
该 executor 依赖 gRPC 服务器反射（server reflection），服务器上需要按
[此处](https://github.com/grpc/grpc/blob/master/doc/server-reflection.md)的说明启用它。
gRPC 服务器反射默认不启用，也并非每个 gRPC 库都实现了它，
在使用该 executor 编写测试之前，请确认您选用的库支持反射。
gRPC 服务器反射在 `gogo/protobuf` 下也无法正常工作：grpc/grpc-go#1873

## 测试 {#tests}

测试结果会被解析为 JSON 并保存在 `systemoutjson` 中。状态码对应
gRPC 的官方状态码。
各个返回码的含义可以在[这里](https://github.com/grpc/grpc/blob/master/doc/statuscodes.md)查到。

## 输入 {#input}

在您的 yaml 文件中，可以使用：

```yaml
  - url mandatory
  - service mandatory: service to call
  - method mandatory: list, describe, or method of the endpoint
  - data optional: data to marshal to JSON and send as a request
  - headers optional: data to send as additional headers
  - connect_timeout optional: The maximum time, in seconds, to wait for connection to be established. Defaults to 10 seconds
  - default_fields optional: whether json formatter should emit default fields
  - include_text_separator optional: when protobuf string formatter is invoked to format multiple messages, all messages after the first one will be prefixed with character (0x1E)
  - tls_client_cert optional: a chain of certificates to identify the caller, first certificate in the chain is considered as the leaf, followed by intermediates. Setting it enables mutual TLS authentication, and the certificate is sent even when ignore_verify_ssl is true. Set the PEM content or the path to the PEM file. It needs tls_client_key: a certificate without its key fails the step. It is not used without tls_root_ca: the connection stays plaintext and a warning is logged.
  - tls_client_key optional: private key corresponding to the certificate. Set the PEM content or the path to the PEM file. It needs tls_client_cert: a key without its certificate fails the step. It is not used without tls_root_ca: the connection stays plaintext and a warning is logged.
  - tls_root_ca optional: root CAs trusted to verify the server certificate, on top of the system ones. Can contain multiple CAs concatenated together. Set the PEM content or the path to the PEM file. The step uses TLS only when this is set; without it the connection is plaintext.
  - ignore_verify_ssl optional: set to true to skip the server certificate check, for example a self-signed certificate that tls_root_ca does not cover
```

示例：

```yaml

name: Title of TestSuite
testcases:

- name: request GRPC
  steps:
  - type: grpc
    url: serverUrlWithoutHttp:8090
    data:
      foo: bar
    service: coolService.api
    method: GetAllFoos
    assertions:
    - result.code ShouldEqual 0
    - result.systemoutjson.foo ShouldEqual bar
```

TLS 示例：

```yaml

name: Title of TestSuite
testcases:

- name: request GRPC
  steps:
  - type: grpc
    url: serverUrlWithoutHttp:8090
    tls_root_ca: |-
      -----BEGIN CERTIFICATE-----
      ...
      -----END CERTIFICATE-----
    # ignore_verify_ssl: true # only when tls_root_ca and the system CAs do not cover the server certificate
    data:
      foo: bar
    service: coolService.api
    method: GetAllFoos
    assertions:
    - result.code ShouldEqual 0
    - result.systemoutjson.foo ShouldEqual bar
```

双向 TLS（mutual TLS）示例：

```yaml

name: Title of TestSuite
testcases:

- name: request GRPC
  steps:
  - type: grpc
    url: serverUrlWithoutHttp:8090
    tls_root_ca: |-
      -----BEGIN CERTIFICATE-----
      ...
      -----END CERTIFICATE-----
    tls_client_cert: |-
      -----BEGIN CERTIFICATE-----
      ...
      -----END CERTIFICATE-----
    tls_client_key: |-
      -----BEGIN PRIVATE KEY-----
      ...
      -----END PRIVATE KEY-----
    # ignore_verify_ssl: true # only when tls_root_ca and the system CAs do not cover the server certificate
    data:
      foo: bar
    service: coolService.api
    method: GetAllFoos
    assertions:
    - result.code ShouldEqual 0
    - result.systemoutjson.foo ShouldEqual bar
```

## 输出 {#output}

```yaml
executor
systemout
systemerr
err
code
timeseconds
```

- result.timeseconds：执行耗时
- result.executor.executor.script：所执行的脚本
- result.err：如果存在，该字段包含错误
- result.systemout：所执行脚本的标准输出
- result.systemerr：所执行脚本的错误输出
- result.code：退出码
