---
translates: quickstart
title: 快速入门
description: 安装 venom，编写测试套件并运行它。
fingerprint: 86cf4090a52b3ea8d391e1a37a1038fbbf6b86519dd9a23afd4e7391ceb9b402
drafted_by: claude
drafted_on: 2026-11-16
status: reviewed
reviewed_by: Christian Stephan
reviewed_on: 2026-11-16
reviewed_fingerprint: 86cf4090a52b3ea8d391e1a37a1038fbbf6b86519dd9a23afd4e7391ceb9b402
---
本页带您从零开始，直到一个测试套件成功通过。您需要 Go，以及 venom 仓库的一份克隆。

## 安装 {#install}

在仓库的克隆目录中构建命令行工具：

```
go build -o venom ./cmd/venom
```

该命令会在当前文件夹中生成一个 `venom` 可执行文件。如果已有发布版本，您也可以直接下载其可执行文件，而不必自行构建。

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
