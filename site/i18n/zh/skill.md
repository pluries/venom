---
translates: skill
title: 在 Claude Code 中使用
description: 安装 venom-pluries 技能，让 Claude Code 在您的项目中配置 Venom Pluries。
fingerprint: ee9619c51479fe441be081d380614b2cd7bc62c1c2f86e5b8c338448a471a5ba
drafted_by: claude
drafted_on: 2026-11-27
status: machine-drafted
---
`venom-pluries` 技能让 Claude Code 在您的项目中配置 Venom Pluries。它会安装经过校验的版本，编写第一个测试套件，把这一步加入您的构建，并检查网页和移动页面的 recipe。它要运行的每条命令都由您批准。

## 安装 {#install-it}

请指定一个发布标签。不要从会不断变化的默认分支安装。

```bash
$ npx skills add pluries/venom@skill-v1.0.0 --skill venom-pluries
```

该工具会询问为哪个代理安装：请选择 Claude Code。它把技能放进项目的 `venom-pluries`（`skills` 之下，`.claude` 之内） 文件夹，并在旁边写入 `skills-lock.json`。除非您事先设置 `DISABLE_TELEMETRY=1`（或 `DO_NOT_TRACK=1`），该工具会发送匿名使用数据。

技能以您的代理的权限运行。使用前请阅读已安装文件夹中的 `SKILL.md`。技能会显示每条命令并等待您的批准，也从不要求您关闭批准。

想看清复制了什么？请克隆该标签，自己复制文件夹：

```bash
$ git clone --depth 1 --branch skill-v1.0.0 https://github.com/pluries/venom venom-skill
```

把该克隆中的 `skills/venom-pluries/` 文件夹复制到项目的 `venom-pluries`（`skills` 之下，`.claude` 之内） 文件夹，然后删除克隆。

## 确认它已就位 {#check-that-it-is-there}

在项目中启动 Claude Code，问它有哪些技能：`venom-pluries` 会出现在列表中。`Skill version` 一行位于其 `SKILL.md` 的顶部，写明版本，应与您选择的标签一致。

## 使用 {#use-it}

用您自己的话提出请求，例如“在这个项目中配置 venom 并写第一个测试”“在 GitHub Actions 中运行它”或“检查我的 recipe”。技能会安装它所针对的 venom 版本（写在它的 `SKILL.md` 和您的 `.venom-version` 中），在运行任何东西之前检查校验和、签名和签名者，并告诉您它做了什么、没做什么。

## 维护者没有运行过的部分 {#what-the-maintainers-did-not-run}

GitLab CI 文件从未在真实的 GitLab 服务器上运行过。技能会如实说明，第一次运行由您来做。对于移动 recipe，技能不会安装或启动设备主机：您需要提供 Appium 服务器以及模拟器或设备。

## 更新 {#update-it}

以同样的方式安装新标签。如果 `SIGNER_SHA256`（在 `release.env` 中）与您项目所用的不同，说明签名证书已更新。此时技能会告诉您复制新的脚本，并显示差异。
