---
translates: smtp
title: SMTP
description: 如何在 venom 测试套件中使用 SMTP executor。
fingerprint: abcd87d7bf80d280db5eadb6c6eadc1b8cee31c1beb4c1e0abe1ccc6e0654d52
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# Venom - Executor SMTP {#venom---executor-smtp}

用于发送 SMTP 邮件的步骤

使用场景：您的软件需要检查邮件，并对邮件做进一步处理？
Venom 可以发送邮件，然后对您的软件执行一些测试。

## 输入 {#input}

```yaml
name: TestSuite with SMTP Steps
testcases:
- name: TestCase SMTP
  steps:
  - type: smtp
    withtls: false
    verifytls: false # Optional, used with withtls: true (see TLS below)
    host: localhost
    port: 25 # 465 if using TLS
    user: yourSMTPUsername # Optional, only works with TLS
    password: yourSMTPPassword # Optional, only works with TLS
    from: venom@smtp.net
    to: destinationa@yourdomain.com,destinationb@yourdomain.com
    subject: Title of mail
    body: Body of mail
    tls_root_ca: ca.pem # Optional, used with withtls: true and verifytls: true (see TLS below)
    tls_client_cert: client.pem # Optional, used with withtls: true, together with tls_client_key
    tls_client_key: client.key # Optional, used with withtls: true, together with tls_client_cert
    assertions:
    - result.err ShouldBeEmpty
```

### TLS {#tls}

| 参数 | 说明 | 默认值 |
|---|---|---|
| withtls | 可选。从第一个字节起就通过 TLS 连接（通常是 465 端口） | false |
| verifytls | 可选，与 `withtls: true` 配合使用。`false`：不检查服务器证书，因此自签名的测试服务器也会被接受。`true`：证书必须链接到系统根证书或 `tls_root_ca`，并且对 `host` 有效，否则该步骤失败 | false |
| tls_root_ca | 可选，与 `withtls: true` 配合使用。在系统根证书之外额外信任的 CA 证书，使 `verifytls: true` 能够接受由私有 CA 签发的服务器。可以拼接多个 CA。仅当 `verifytls: true` 时才用于验证 | |
| tls_client_cert | 可选，与 `withtls: true` 配合使用。客户端证书（双向 TLS），先放叶子证书，后接中间证书。即使 `verifytls` 为 `false` 也会发送。它需要 `tls_client_key`：只有证书而没有密钥，该步骤会失败 | |
| tls_client_key | 可选，与 `withtls: true` 配合使用。`tls_client_cert` 的私钥。它需要 `tls_client_cert`：只有密钥而没有证书，该步骤会失败 | |

`tls_root_ca`、`tls_client_cert` 和 `tls_client_key` 的值可以是 PEM 内容本身，也可以是 PEM 文件的路径，相对于
测试套件所在的目录。没有设置 `withtls: true` 时它们不会被使用：连接保持明文，并记录一条警告
（只有证书没有密钥，或者反过来，仍然会使该步骤失败）。

环境变量 `SSL_CERT_FILE` 或 `SSL_CERT_DIR` 可以让 venom 使用另一组系统根证书。

## 输出 {#output}

没有输出，只有在出错时才有 result.err。

## 默认断言 {#default-assertion}

```yaml
result.err ShouldBeEmpty
```
