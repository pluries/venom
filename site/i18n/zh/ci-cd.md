---
translates: ci-cd
title: 在 CI/CD 流水线中使用 venom
description: 如何在流水线中运行 venom。
fingerprint: bc188979d8a80ca6b0e6bdf359e7d064b61169dbbd9b40a16c5bd7703adba768
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# 在 CI/CD 流水线中使用 venom {#use-venom-in-cicd-pipelines}

Venom 既可以用于开发环境，也可以用于您的 CI 服务器。
为了正确显示 venom 的输出，您可能需要在运行 venom 之前导出环境变量 `IS_TTY=true`。
