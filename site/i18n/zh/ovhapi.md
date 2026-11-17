---
translates: ovhapi
title: OVHAPI
description: 如何在 venom 测试套件中使用 OVHAPI executor。
fingerprint: 33b8b8e3e61e28e515ea5ca5ac5d85740f605d690b373504f4afcf4ea16ade0e
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# Venom - Executor OVHAPI {#venom---executor-ovhapi}

## 测试 OVH API 的步骤 {#step-to-test-ovh-api}

使用场景：您的软件需要调用 OVH API。<br>
调用 API 需要 OVH 凭据。您可以使用应用密钥（app keys）认证，也可以使用 OAuth2。

要使用应用密钥认证，请参考以下教程：<br>
EN: https://docs.ovh.com/gb/en/customer/first-steps-with-ovh-api/

要使用 OAuth2，请参考以下教程：<br>
EN: https://help.ovhcloud.com/csm/en-manage-service-account?id=kb_article_view&sysparm_article=KB0059343

## 输入 {#input}

可用的参数如下：

| 参数              | 说明                                                             | 默认值        |
|-------------------|------------------------------------------------------------------|---------------|
| endpoint          | 可选                                                             | ovh-eu        |
| applicationKey    | 设置 `noAuth` 时可选，使用应用密钥认证时必填                     |               |
| applicationSecret | 设置 `noAuth` 时可选，使用应用密钥认证时必填                     |               |
| consumerKey       | 设置 `noAuth` 时可选，使用应用密钥认证时必填                     |               |
| clientID          | 设置 `noAuth` 时可选，使用 OAuth2 时必填                         |               |
| clientSecret      | 设置 `noAuth` 时可选，使用 OAuth2 时必填                         |               |
| noAuth            | 可选                                                             |               |
| headers           | 可选                                                             |               |
| resolve           | 可选                                                             |               |
| proxy             | 可选                                                             |               |
| tlsRootCA         | 可选，参见[私有根 CA](#private-root-ca)                          |               |
| tls_root_ca       | 可选，`tlsRootCA` 的另一个名称                                   |               |

| 参数      | 说明 | 默认值 |
|-----------|------|--------|
| method    | 可选 | GET    |
| path      | 必填 |        |
| body      | 可选 |        |
| bodyFile  | 可选 |        |
 
第一组参数也可以像这样定义在 Venom 变量中：

```yaml
vars:
  ovh.endpoint: ovh-eu
  ovh.applicationKey: foo
  ovh.applicationSecret: foo
  ovh.clientID: foo
  ovh.clientSecret: foo
  ovh.consumerKey: foo
  ovh.noAuth: false
  ovh.headers:
    x-foo: foo
  ovh.resolve:
  - example.org:443:127.0.0.1
  ovh.proxy: localhost:8000
  ovh.tlsRootCA: |-
    -----BEGIN CERTIFICATE-----
    MIIF3jCCA8agAwIBAgIQAf1tMPyjylGoG7xkDjUDLTANBgkqhkiG9w0BAQwFADCB
    ...
    -----END CERTIFICATE-----
```

### 私有根 CA {#private-root-ca}

`tlsRootCA` 和 `tls_root_ca` 是同一个设置的两个名称：executor 在系统 CA 之外额外信任的一个 CA，
用于使用私有证书的 API。两者设置其一即可；如果步骤同时设置了两者，会在发起任何调用之前失败。
`ovh.tlsRootCA` 变量是这两个名称共用的变量，只有在步骤两者都未设置时才会使用。

该值是证书的 PEM 文本，或 PEM 文件的路径。相对路径从测试套件所在的
文件夹读取。`ovh.tlsRootCA` 变量同样可以使用路径。

根 CA 只用于 TLS 连接。如果 endpoint 是 `http://`，步骤会记录一条警告；重定向到
`https://` 或使用 `https://` 代理时仍会使用该 CA。像 `ovh-eu` 这样的 endpoint 名称就是一个 `https://` URL，不会产生警告。

```yaml
- type: ovhapi
  endpoint: 'https://api.example.internal/1.0'
  tls_root_ca: certs/internal-ca.pem
  noAuth: true
  path: /me
```

## 一个 __ovhapi__ 测试套件的示例 {#example-of-an-__ovhapi__-testsuite}

### 使用应用密钥认证 {#using-app-keys-authentication}

```yaml
name: Title of TestSuite
testcases:
- name: me
  steps:
  - type: ovhapi
    endpoint: 'ovh-eu'
    applicationKey: 'APPLICATION_KEY'
    applicationSecret: 'APPLICATION_SECRET'
    consumerKey: 'CONSUMER_KEY'
    method: GET
    path: /me
    retry: 3
    delay: 2
    assertions:
    - result.statuscode ShouldEqual 200
    - result.bodyjson.nichandle ShouldContainSubstring MY_NICHANDLE
```

### 使用 OAuth2 {#using-oauth2}

```yaml
name: Title of TestSuite
testcases:
- name: me
  steps:
  - type: ovhapi
    endpoint: 'ovh-eu'
    clientID: 'CLIENT_ID'
    clientSecret: 'CLIENT_SECRET'
    method: GET
    path: /me
    retry: 3
    delay: 2
    assertions:
    - result.statuscode ShouldEqual 200
    - result.bodyjson.nichandle ShouldContainSubstring MY_NICHANDLE
```

## 输出 {#output}

可用的输出字段如下：

| 字段               | 说明                                 |
|--------------------|--------------------------------------|
| result.executor    |                                      |
| result.timeseconds | 执行耗时                             |
| result.statuscode  | HTTP 响应的状态码                    |
| result.body        | HTTP 响应的正文                      |
| result.bodyjson    | HTTP 响应的正文（如果是 JSON）       |
| result.err         | 错误信息（如果存在）                 |

请注意，您可以通过 `result.bodyjson.yourkey` 这样的形式访问 JSON 数据。

## 默认断言 {#default-assertion}

```yaml
result.statuscode ShouldEqual 200
```
