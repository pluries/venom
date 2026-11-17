---
translates: contributing
title: 参与贡献
description: 如何向 venom 提交修改。
fingerprint: 0697dcef089c507469ff2eec4d800cba3c5835f38a44669c87c3dc8b868edb8c
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# 参与 Venom 的贡献 {#contributing-to-venom}

本项目接受贡献。为了做出贡献，您需要注意几件事：

1. 您的代码必须遵循编码风格规则
2. 您的代码必须有完整的文档
3. 您的工作必须经过签署
4. 使用 GitHub 拉取请求 (Pull Request)

## 编码与文档风格： {#coding-and-documentation-style}

- 代码必须用 `go fmt` 格式化
- 代码必须通过 `go vet`
- 代码必须通过 `golint`

## 提交修改： {#submitting-modifications}

贡献应以 GitHub 拉取请求的形式提交。其准则与 Linux 内核的补丁提交准则相同，唯一的例外是下文定义的 DCO。这些准则定义在 'submitting-patches' 文件中，该文件位于 Linux 内核源码树的 'Documentation/process' 目录路径下。

也可以在线访问：

https://www.kernel.org/doc/Documentation/process/submitting-patches.rst

您可以通过 GitHub 提交您的补丁

## 新文件的许可： {#licensing-for-new-files}

Venom 采用（修改版）BSD 许可证。任何贡献给 Venom 的内容都必须以该许可证发布。

向项目中引入新文件时，请确保它带有版权头，清楚说明它以哪种许可证发布。

## 开发者原创声明 (Developer Certificate of Origin)： {#developer-certificate-of-origin}

```
To improve tracking of contributions to this project we will use a
process modeled on the modified DCO 1.1 and use a "sign-off" procedure
on patches that are being contributed.

The sign-off is a simple line at the end of the explanation for the
patch, which certifies that you wrote it or otherwise have the right
to pass it on as an open-source patch. The rules are pretty simple:
if you can certify the below:

By making a contribution to this project, I certify that:

(a) The contribution was created in whole or in part by me and I have
    the right to submit it under the open source license indicated in
    the file; or

(b) The contribution is based upon previous work that, to the best of
    my knowledge, is covered under an appropriate open source License
    and I have the right under that license to submit that work with
    modifications, whether created in whole or in part by me, under
    the same open source license (unless I am permitted to submit
    under a different license), as indicated in the file; or

(c) The contribution was provided directly to me by some other person
    who certified (a), (b) or (c) and I have not modified it.

(d) The contribution is made free of any other party''s intellectual
    property claims or rights.

(e) I understand and agree that this project and the contribution are
    public and that a record of the contribution (including all
    personal information I submit with it, including my sign-off) is
    maintained indefinitely and may be redistributed consistent with
    this project or the open source license(s) involved.

```
