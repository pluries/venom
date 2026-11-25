---
translates: installing
title: 安装
description: 如何安装 venom。
fingerprint: 6b24ed9fedcdd72c1c379a6a9fa397e4c41b8332cfcf676a4cae3eca8d6e997e
drafted_by: claude
drafted_on: 2026-11-24
status: reviewed
reviewed_by: Christian Stephan
reviewed_on: 2026-11-25
reviewed_fingerprint: 6b24ed9fedcdd72c1c379a6a9fa397e4c41b8332cfcf676a4cae3eca8d6e997e
---
# 安装 {#installing}

## 从二进制文件安装 {#install-from-binaries}

此分支的每个发布版本都发布在 https://github.com/pluries/venom/releases/latest 。每个平台对应一个可执行文件：
`venom.linux-amd64`、`venom.linux-arm64`、`venom.linux-arm`、`venom.linux-386`、`venom.darwin-amd64`、`venom.darwin-arm64`、
`venom.linux-ppc64le`、`venom.windows-amd64.exe`、`venom.freebsd-amd64` 和 `venom.openbsd-amd64`，此外还有校验和清单 `SHA256SUMS`、其签名 `SHA256SUMS.p7s` 以及该签名的时间戳令牌 `SHA256SUMS.p7s.tsr`。

Linux 示例：

```bash
$ curl -L -o venom https://github.com/pluries/venom/releases/latest/download/venom.linux-amd64
$ chmod +x venom && sudo mv venom /usr/local/bin/venom
$ venom -h
```

### 验证下载内容 {#verify-the-download}

校验和清单使用 Certum 代码签名证书签名，Windows 可执行文件还带有 Authenticode 签名。
您只需要 `sha256sum` 和 `openssl`，不需要其他工具：

```bash
$ curl -LO https://github.com/pluries/venom/releases/latest/download/SHA256SUMS
$ curl -LO https://github.com/pluries/venom/releases/latest/download/SHA256SUMS.p7s
$ sha256sum -c SHA256SUMS --ignore-missing
$ openssl cms -verify -inform DER -in SHA256SUMS.p7s -content SHA256SUMS -binary -purpose any
```

第一项检查将您的文件与清单进行比对，第二项检查证明该清单由维护者的证书签名。
