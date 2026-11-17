---
translates: updating
title: 更新
description: 如何更新 venom。
fingerprint: a0c3a4ff4fa192302942a56576ce1f0f240ffb1450dbbc780aeb963709181873
drafted_by: claude
drafted_on: 2026-11-16
status: reviewed
reviewed_by: Christian Stephan
reviewed_on: 2026-11-16
reviewed_fingerprint: a0c3a4ff4fa192302942a56576ce1f0f240ffb1450dbbc780aeb963709181873
---
# 更新 {#updating}

此分支没有 `venom update` 命令。上游的该命令会下载上游项目的最新发布版本，
却不检查校验和或签名（#2296）。要更新，请从
https://github.com/pluries/venom/releases/latest 下载新的可执行文件并[验证它](#verify-the-download)，或者自行构建对应的标签（`go build ./cmd/venom`）。
然后检查版本：

```bash
$ venom version
```
