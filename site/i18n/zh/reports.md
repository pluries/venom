---
translates: reports
title: 导出测试报告
description: 如何把结果导出为 JSON、XML 或 HTML。
fingerprint: ddb2c03f51333e036451a070d587b91fd37fddc6fe592bb51e66a08e1e6e6454
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# 导出测试报告 {#export-tests-report}

您可以把测试套件的结果导出为报告，支持多种格式：xUnit (XML)、JSON、YAML、TAP。

您可以用 `--output-dir` 选项指定输出目录，用 `--format` 选项指定格式（默认为 XML）：

```bash
$ venom run --format=xml --output-dir="."

# html export
$ venom run --output-dir="." --html-report
```

导出为 XML 的报告，可以用 xUnit/jUnit 查看器直接在您常用的 CI/CD 工具链中查看，例如逐次运行地观察结果。
