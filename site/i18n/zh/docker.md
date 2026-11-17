---
translates: docker
title: Docker 镜像
description: 如何通过 Docker 镜像运行 venom。
fingerprint: ba838efdb064cdb08914baa943a79efe02f608659459b8827883285d36c7e84f
drafted_by: claude
drafted_on: 2026-11-16
status: reviewed
reviewed_by: Christian Stephan
reviewed_on: 2026-11-16
reviewed_fingerprint: ba838efdb064cdb08914baa943a79efe02f608659459b8827883285d36c7e84f
---
# Docker 镜像 {#docker-image}

您不必在本地安装（和更新）Venom，也可以用下面的命令将 Venom 作为 Docker 镜像启动。
此分支的镜像是 `ghcr.io/pluries/venom`。

假设您的测试套件（testsuite）位于当前目录下的 `./tests` 目录中，测试库位于 `./tests/lib` 下，那么结果将保存在 `results` 目录中。

```bash
$ mkdir -p results
$ docker run --mount type=bind,source=$(pwd)/tests,target=/workdir/tests --mount type=bind,source=$(pwd)/results,target=/workdir/results ghcr.io/pluries/venom:latest
```

每个发布版本还有自己的镜像标签，例如 `ghcr.io/pluries/venom:v1.5.0-pluries.1`。请在流水线中使用发布标签，这样镜像就不会在您不知情的情况下发生变化。
