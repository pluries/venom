---
translates: first-test
title: 编写并运行您的第一个测试套件
description: 一个较长的第一个测试套件的演练。
fingerprint: fda7e71e5e326bc03d72f8c9ef6f8b95feabec80763f205ae7362a10a3a571d1
drafted_by: claude
drafted_on: 2026-11-16
status: reviewed
reviewed_by: Christian Stephan
reviewed_on: 2026-11-16
reviewed_fingerprint: fda7e71e5e326bc03d72f8c9ef6f8b95feabec80763f205ae7362a10a3a571d1
---
# 编写并运行您的第一个测试套件 {#write-and-run-your-first-test-suite}

为了理解 Venom 是如何工作的，让我们一起创建并运行第一个测试套件。

在这个测试套件中，我们要做的第一批断言，是检查我们想要测试的站点（例如一个公开的 REST API，如 OVHcloud API）是否：
- 可以访问（以 200 状态码响应）
- 在 5 秒内响应
- 返回有效的响应（JSON 格式）

首先，创建您的测试套件，文件名例如 `testsuite.yml`。
用您喜欢的编辑器或 IDE 打开它，并填入以下内容：

```yaml
name: APIIntegrationTest

vars:
  url: https://eu.api.ovh.com

testcases:
- name: GET http testcase, with 5 seconds timeout
  steps:
  - type: http
    method: GET
    url: {{.url}}/1.0/
    timeout: 5
    assertions:
    - result.statuscode ShouldEqual 200
    - result.timeseconds ShouldBeLessThan 1
    - result.bodyjson ShouldContainKey apis
    - result.body ShouldContainSubstring /dedicated/server
    - result.body ShouldContainSubstring /ipLoadbalancing
```

然后，用以下命令运行您的测试套件：

```bash
$ venom run 

 • APIIntegrationTest (testsuite.yml)
 	• GET-http-testcase-with-5-seconds-timeout SUCCESS
```

您已经用 HTTP executor 编写并执行了您的第一个测试套件！:)
