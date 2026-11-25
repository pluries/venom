---
translates: quickstart
title: 快速入门
description: 安装 venom，编写测试套件并运行它。
fingerprint: ff09c7857b2e31620d2ae8d74003a0e186283bb52cf0d3334055d5d020980dc4
drafted_by: claude
drafted_on: 2026-11-24
status: reviewed
reviewed_by: Christian Stephan
reviewed_on: 2026-11-25
reviewed_fingerprint: ff09c7857b2e31620d2ae8d74003a0e186283bb52cf0d3334055d5d020980dc4
---
本页带您从零开始，直到一个测试套件成功通过。您需要 Linux、macOS 或 Windows 上的一个终端。

## 安装 {#install}

下载适用于您平台的最新发布版本的可执行文件。这是 Linux 的示例。[安装](installing.html)页面列出了所有平台，[验证发布版本](verify-release.html)页面说明如何检查签名：

```
curl -L -o venom https://github.com/pluries/venom/releases/latest/download/venom.linux-amd64
chmod +x venom
```

这些命令会在当前文件夹中生成一个 `venom` 可执行文件。

## 编写测试套件 {#write-a-suite}

测试套件是一个 YAML 文件。下面这个套件只有一个测试用例，该用例只有一个步骤。该步骤运行一条 shell 命令，并由两条断言检查其结果：

````yaml
name: Quickstart suite
testcases:
  - name: say hello
    steps:
      - type: exec
        script: echo "hello venom"
        assertions:
          - result.systemout ShouldContainSubstring hello
          - result.code ShouldEqual 0
````

- `type: exec` 会在 shell 中运行 `script`。
- `result.systemout ShouldContainSubstring hello` 检查输出中是否包含单词 `hello`。
- `result.code ShouldEqual 0` 检查命令是否无错误地结束。

## 运行 {#run-it}

将文件保存为 `quickstart.yml`，然后运行：

```
./venom run quickstart.yml
```

## 查看结果 {#read-the-result}

Venom 会打印每个测试用例及其状态。当所有断言都成立时，最后一行是：

```
final status: PASS
```

断言失败时，会打印出对应的步骤、该断言，以及 venom 实际得到的值。测试套件失败时，退出码不为零。

Venom 是 [ovh/venom](https://github.com/ovh/venom) 的一个独立分支。接下来，请阅读[概览](overview.html)，或查看关于[测试套件](testsuites.html)的页面。
