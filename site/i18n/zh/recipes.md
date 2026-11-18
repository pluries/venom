---
translates: recipes
title: 配方
description: 把浏览器和移动端测试写成配方，并用检查和反向对照来证明它们。
fingerprint: 3dd31333d7ad4d4487d1c297fdc7e02dc69595aee1619af22617d3bfb867d5d5
drafted_by: claude
drafted_on: 2026-11-18
status: machine-drafted
---
配方是一个驱动网页或移动应用的 venom 测试套件。本页说明如何组织配方、如何命名、如何证明它们确实在测试某些东西，以及如何不让密钥和结果出现在不该出现的地方。您需要 venom；如果是网页配方，还需要一个由 Playwright 安装的浏览器。[web executor](web.html) 和 [mobile executor](mobile.html) 两个页面列出了所有操作。

配方就是普通的测试套件。它使用 `web` 步骤或 `mobile` 步骤，没有任何新东西。下面是一个小型网页配方。页面地址是变量 `appUrl`，因此同一个文件可以针对该页面的任何副本运行：

{{include site/examples/recipes/web-page.yml yaml}}

用您页面的地址来运行它：

```
venom run --var appUrl=http://localhost:8000 web-page.yml
```

## 文件夹布局 {#folder-layout}

把同一个功能的配方放在 `tests/` 下的同一个文件夹里，与其他 venom 测试套件放在一起。例如：

```
tests/
  site/
    site-languages.yml
  recipes/
    reference/
      mobile/
        counter.yml
```

一个功能一个文件夹，里面有一个或多个 `.yml` 文件。对于带有子文件夹的文件夹，通过指定各个子文件夹来检查。包含凭据的变量文件要放在该文件夹之外，或者让 git 忽略它（见“凭据”一节）。

## 测试用例名称与规格编号 {#testcase-names-and-the-spec-id}

每个测试用例证明某份规格的一个验收场景。测试用例名称以场景编号开头，后面跟一段简短描述：`US1-AS3 an item typed in the input is added to the list`。编号的形式是 `USn-ASm`：用户故事 `n`，验收场景 `m`。

在套件的一个变量中写明一次规格：

```
vars:
  spec: 014-web-mobile-recipes
```

该值是规格所在文件夹的名称。配方检查会读取该文件夹中的 `spec.md`，并在其中查找编号。选项 `--specs-dir` 告诉检查规格文件夹在哪里。同一个文件夹中的两个文件不得声明同一个场景。

要查看某个配方证明了哪些场景，请加上 `--coverage` 并给出规格编号：`venom recipe check --specs-dir <specs folder> --coverage 014-web-mobile-recipes tests/recipes`。在列出发现之后，它会列出规格中的每个场景，格式为 `US1-AS3 proven: <file> / <testcase>` 或 `US1-AS4 not proven`。只有当测试用例没有被跳过，并且包含读取应用的断言时，它才算证明了一个场景。该选项不会改变退出状态。开发集的覆盖率脚本不读取 `.yml` 文件，因此统计配方时请使用此选项，而不是该脚本。

## 选择器 {#selectors}

`find`、`click` 或 `fill` 步骤需要一个选择器。使用 CSS 或 XPath。优先使用稳定的测试属性，例如 `data-test`：

```
find: '[data-test="add-button"]'
```

当有人重新设计页面样式时，id 和类名会改变。`data-test` 属性的存在只是为了让测试能找到该元素。如果应用没有这类属性，请在编写配方之前请开发人员添加。

`find` 不会等待。它统计的是此刻已经存在的元素数量。如果页面在首次绘制之后才加载数据，请先用 `wait` 等待，或者使用 `click` 或 `fill`，它们会等待各自的元素。在移动端步骤中，只设置一个定位器，例如 `accessibilityId`。

## 证明步骤 {#proof-steps}

一个通过的配方，在您亲眼看到它失败之前，什么也证明不了。三个步骤给出证明：

1. 重放。针对应用用 `venom run` 运行配方。每个测试用例都必须通过。多重放几次：一次通过一次失败的测试用例不可用。
2. 反向对照。对每个测试用例，复制一份配方，修改第一个来自应用的期望值，然后运行这份副本。运行必须以状态 2 退出，并指出该测试用例。原始文件保持不动。
3. 配方检查。运行检查，它不需要浏览器或设备，只读取文件：

```
venom recipe check --specs-dir <folder of the specs> tests/site
```

检查在没有发现任何问题时以 0 退出，报告发现项时以 1 退出，无法读取输入时以 2 退出。每个发现项都会指出文件、行号、规则和测试用例。它会报告以下情况：没有场景编号的测试用例、重复的编号、规格中不存在的编号、没有对从应用读取的值做断言的测试用例、不可能失败的断言、类似凭据的文本，以及写死的地址。

该检查是静态的。它无法读取 `execute` 脚本的内部，也无法知道某个期望值是否正确。反向对照涵盖这两点。

## executor 无法做到的事 {#what-the-executors-cannot-do}

web executor 没有悬停、拖动或键盘快捷键的步骤，而且 `find` 无法读取滚动宽度之类的数字。`execute` 操作会在页面中运行脚本并返回一个值，您可以对该值做断言。下面这个例子读取页面可以横向滚动多远，以及它列出了多少个条目：

{{include site/examples/recipes/web-execute.yml yaml}}

如果 `execute` 仍然不够，就把这个缺口记录下来，不要为该场景编写配方。不要把一个测试用例的所有断言都放在同一个脚本结果上而不做反向对照：检查读不了脚本。

移动端配方需要一个 Appium 服务器以及一台设备或模拟器。没有它们就无法运行。下面这个配方的第一行带有标记 `# needs: device`，因此在没有设备的地方，站点的测试会跳过它：

{{include site/examples/recipes/mobile-counter.yml yaml}}

## 凭据 {#credentials}

配方中不包含密码、令牌或密钥。登录信息来自变量。可以在命令行中传入，或者放在 git 忽略的变量文件中：

```
venom run --var-from-file login.vars.yml tests/site/login.yml
```

在配方中写变量：`text: "{{.password}}"`。检查会报告任何步骤或默认值中类似凭据的文本。

配方针对测试环境运行，绝不针对生产环境。地址是由流水线设置的变量，而 `venom run` 不会阻止您把它指向真实系统。请在您的流水线中维护允许访问的主机列表。

## 结果与产物 {#results-and-artifacts}

失败的步骤可以写出页面转储、截图或跟踪文件。这些文件可能包含来自应用的数据。请把它们写入 git 忽略的结果文件夹，不要提交。在 CI 中，只保留较短的时间，例如几天，并且只向需要的人开放访问：存储空间是受限的。不要把它们复制到长期存储中。

## 未定义的变量 {#undefined-variables}

默认情况下，忘记传入的变量不会使运行停止。Venom 会把 `{{.password}}` 原样保留为文本 `{{.password}}`，并且步骤会输入这段文本。运行不会因此失败。

传入 `--strict-variables` 可以改变这一点。此时 venom 会以退出状态 2 停止，并给出消息 `unknown variable(s) "password"`。请在每个运行配方的流水线中使用该选项：

```
venom run --strict-variables --var-from-file login.vars.yml tests/site/login.yml
```
