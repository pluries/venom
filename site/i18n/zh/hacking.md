---
translates: hacking
title: 参与开发
description: 如何参与 venom 本身的开发。
fingerprint: 14fa6d0c590f57d86e6112e8fe3972d3ef1a258de0438f7d48aaaa38eca8425f
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# 参与开发 {#hacking}

[如何编写您自己的 executor？](https://github.com/ovh/venom/tree/master/executors#venom-executor)

## 如何编译？ {#how-to-compile}
```bash
$ make build
```

## 如何测试？ {#how-to-test}

### 单元测试： {#unit-tests}

```bash
make test
```

### 集成测试： {#integration-tests}

准备测试环境：

```bash
make build OS=linux ARCH=amd64
cp dist/venom.linux-amd64 tests/venom
cd tests
make start-test-stack  # (wait a few seconds)
make build-test-binary-docker
```

运行集成测试：

```bash
make run-test
```

运行 `tests/` 中的测试套件：这些测试套件通过 `{{.stackHost}}` 访问测试环境，其默认值（`localhost`）位于 `tests/.venomrc` 中，而 venom 会从当前目录读取该文件。如果要针对另一台机器上的测试环境运行，请传入它的地址：`make start-test-stack STACK_HOST=<address>` 和 `make run-test STACK_HOST=<address>`。

清理：

```bash
make clean
make stop-test-stack
```
