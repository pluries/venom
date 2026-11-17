---
translates: overview
title: 概览
description: venom 能做什么，以及测试套件如何构成。
fingerprint: faf2593150cb4767a598a4183a06a215b68307ec7c26da0cc429744c250d4622
drafted_by: claude
drafted_on: 2026-11-16
status: reviewed
reviewed_by: Christian Stephan
reviewed_on: 2026-11-16
reviewed_fingerprint: faf2593150cb4767a598a4183a06a215b68307ec7c26da0cc429744c250d4622
---
# 概览 {#overview}

Venom 让您可以用编写应用程序代码的方式来处理集成测试。
借助 Venom，testcase 以代码的形式管理：测试具有良好的可读性，因此可以纳入代码评审。这样一来，对开发者和团队而言，编写和执行 testsuite（测试套件）都变得更加容易。

具体来说，您需要在一个 YAML 文件中编写测试套件。
Venom 会运行 executor（脚本、HTTP 请求、web、IMAP 等），并应用断言（assertion）。
它还可以生成 xUnit 结果文件。

<img src="./venom.gif" alt="Venom 演示">
