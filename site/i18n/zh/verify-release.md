---
translates: verify-release
title: 验证发布版本
description: 检查某个发布版本是否由维护者的证书签名。
fingerprint: 11141e8943d763bea06a3462db682b51480988aa7bd53a6b320de816db989244
drafted_by: claude
drafted_on: 2026-11-24
status: machine-drafted
---
这个分支的每个发布版本都经过签名，您无需信任下载页面就能验证签名。本页说明如何证明某个发布版本确实来自维护者：校验和列表已签名，签名者是维护者的证书，并且您的文件在该列表中。您需要 `openssl` 3 或更高版本、`sha256sum` 和 `curl`。macOS 自带的 LibreSSL 无法验证签名的时间戳，请先在该系统上安装 OpenSSL 3。

下载校验和列表、它的签名以及您需要的可执行文件：

```bash
$ curl -LO https://github.com/pluries/venom/releases/latest/download/SHA256SUMS
$ curl -LO https://github.com/pluries/venom/releases/latest/download/SHA256SUMS.p7s
$ curl -LO https://github.com/pluries/venom/releases/latest/download/venom.linux-amd64
```

## 检查签名者 {#check-the-signer}

提取为列表签名的证书，并输出其 SHA-256 指纹：

```bash
$ openssl cms -verify -inform DER -in SHA256SUMS.p7s -content SHA256SUMS -binary -noverify -signer signer.pem -out /dev/null
$ openssl x509 -in signer.pem -outform DER | sha256sum
```

从 `v1.5.0-pluries.2` 起，每个发布版本的签名者指纹为：

```
42632819c286ec58a829fea06323c5e53772982d7eaff3d2756bd997527a54f2
```

如果您的结果不同，请停止：该列表不是由维护者的证书签名的。

## 检查列表和文件 {#check-the-list-and-the-file}

```bash
$ openssl cms -verify -inform DER -in SHA256SUMS.p7s -content SHA256SUMS -binary -purpose any
$ sha256sum -c SHA256SUMS --ignore-missing
```

第一条命令证明列表已签名且未被更改。第二条命令证明您的可执行文件就是列表中的那一个。

## 让 venom-pluries skill 来做 {#let-the-venom-pluries-skill-do-it}

适用于 Claude Code 的 `venom-pluries` skill 带有一个脚本，会执行以上所有检查，把签名者与上面的指纹比较，任何检查失败时都拒绝安装。向项目添加 venom 时请使用它。
