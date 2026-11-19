---
translates: web
title: Web
description: 如何在 venom 测试套件中使用 Web executor。
fingerprint: d700bc6151e18429c9339fb06ffe6bc6b56b451abd92c6c1328ee3c6cbec08f8
drafted_by: claude
drafted_on: 2026-11-19
status: machine-drafted
---
# Venom - Executor Web {#venom---executor-web}

在 Web 应用程序中进行导航

使用场景：您有一个 Web 应用程序，想要检查它的某些行为。

Web executor 让您可以在 Web 应用程序中导航并执行各种动作。

## HTTPS 证书 {#https-certificates}

默认情况下，Web executor 会忽略 HTTPS 证书错误（浏览器上下文是用 `IgnoreHttpsErrors` 创建的）。
面对证书已过期、自签名或主机名不符的站点，Web 步骤也会通过，因此不要依赖它来检查证书。要让浏览器拒绝无效证书，
请把变量 `verifyTls` 设为 `true`；此时步骤会失败。Web executor 无法信任私有 CA：Playwright 没有提供针对浏览器上下文的此类选项。

## 浏览器 {#browsers}

Web executor 通过 [Playwright](https://playwright.dev)（Go 绑定为
`github.com/mxschmitt/playwright-go`）驱动浏览器。Playwright 自行管理其浏览器版本：无需安装 `chromedriver` 或 `geckodriver`，
也无需让驱动版本与浏览器版本配套。

每台机器只需安装一次 Playwright 驱动，以及您的测试套件所用的浏览器及其操作系统软件包（在 Linux 上，`--with-deps` 需要 `sudo`）：

```bash
go run github.com/mxschmitt/playwright-go/cmd/playwright@v0.6201.1 install --with-deps chromium   # or firefox, webkit
```

venom 从不自行下载浏览器：如果测试套件需要的浏览器没有安装，它会在初始化阶段失败，
错误消息中会给出上面这条命令。

### 参数 {#arguments}
* Chromium：[参数列表](https://peter.sh/experiments/chromium-command-line-switches/)。不带前导短横线的参数会被加上 `--`，
  与 chromedriver 的做法一致。
* Firefox：[参数列表](https://wiki.mozilla.org/Firefox/CommandLineOptions)，
  [偏好设置列表](https://searchfox.org/mozilla-central/source/modules/libpref/init/all.js)

## 变量 {#variables}

您可以定义参数，用来配置测试套件期间所用的浏览器。所有参数都是可选的：
* driver（默认值：`chrome`）：要使用的浏览器：
  * `chrome`：Chromium
  * `gecko`：Firefox（Playwright 自带的构建）
  * `webkit`：WebKit，即 Safari 背后的引擎
* width、height：页面的尺寸。不设置时，页面保持浏览器窗口自身的大小
* args：浏览器参数
* prefs：Firefox 偏好设置（仅限 `gecko`）
* timeout（默认值：`180`）：页面加载（`navigate`、`historyAction`、弹窗应答）以及步骤所启动的导航完成的最长时间，单位为秒
* proxy：访问网站所用的代理（格式：`[server]:[port]`）
* verifyTls（默认值：`false`）：让浏览器拒绝 HTTPS 证书无效的页面，使步骤失败。不设置时，
  证书错误会被忽略，一如既往
* headless（默认值：`false`）：不显示窗口地运行浏览器。在 CI/CD 流水线中很有用
* binaryPath：要使用的浏览器可执行文件，用来取代 Playwright 自带的（仅限 `chrome`）
* trace（默认值：`false`）：把每个失败步骤的 Playwright 跟踪保存到 `page.trace.zip`。参见
  [失败诊断](#failure-diagnostics)
* testIdAttribute（默认值：`data-testid`）：`TESTID` 定位器读取的属性（参见[定位器](#locators)）。对于用 `data-test` 或其他名称标记元素的页面，
  请在测试套件中设置一次。参见[陷阱](#traps)。该名称必须匹配 `^[A-Za-z_][A-Za-z0-9_:.-]*$`：任何其他值都会使 Setup 在浏览器启动之前失败

Chromium 在开启沙箱的情况下运行，与在 chromedriver 下一样。需要关闭沙箱的测试套件（例如在以 root 身份运行的容器中）
请传入 `--no-sandbox`，放在 `args` 里。限制非特权用户命名空间的主机（例如原生 Ubuntu 23.10 及更高版本）
无法启动 Playwright 自带 Chromium 的沙箱：初始化会失败，并指出修复办法——
加入 `--no-sandbox`（放在 `args` 里），让 `binaryPath` 指向系统自带的 Chrome，或者允许这些命名空间
（`sysctl kernel.apparmor_restrict_unprivileged_userns=0`）。

HTTPS 证书错误默认会被忽略，与该 executor 之前使用的 WebDriver 会话一致
（`acceptInsecureCerts`）。设置 `verifyTls` 即可检查证书（参见上文的 HTTPS 证书）。

下列变量仍然被接受，但会**被忽略并给出警告**，因为它们在 Playwright 中没有对应项：
* driverPath、driverPort：已经不再有驱动进程
* detach：Playwright 无法在测试套件结束后让浏览器继续运行
* debug、logLevel：它们设置的是旧 WebDriver 客户端的日志级别；请改用 venom 自己的 `-v`
* 搭配 `chrome` 的 prefs，搭配 `gecko` 的 binaryPath：Playwright 驱动的是它自己的 Firefox 构建

## 动作 {#action}
动作让您可以操控浏览器

对元素执行的动作（`click`、`fill`、`select`、`uploadFile`、`selectFrame`、`hover`、`dragAndDrop`，以及 `press`（带 `find` 时））会等待元素就绪，
最长等待其 `syncTimeout` 或 30 秒。`waitFor` 会等待元素达到其 `state`，时限为其 `syncTimeout`。`find` 从不等待：它统计此刻存在的元素个数，因此
`result.find ShouldEqual 0` 依然很快。`find`、`waitFor`、`hover`、`dragAndDrop` 和 `press` 与上述动作一样，
会先等待已在进行的导航结束，然后才执行。在可能加载页面的动作（`click`、`fill`、`select`、
`uploadFile`、`execute`、`waitFor`、`hover`、`dragAndDrop`、`press`、弹窗应答）之后，步骤会等页面加载完成，再读取
`result.title` 和 `result.url`。这也包括页面自己发起的导航，无论来自脚本
（`location.href = …`）还是来自没有延迟的定时器。在 `timeout` 之内未完成的导航会使该步骤失败，
后续步骤也会失败，直到有一个 `navigate` 将其取代。页面处于导航过程中时，不会写出 `page.dump.html`。

对页面的调用如果始终不返回，其步骤会失败：读取操作（标题、源码、计数、文本；如果它在等待某个元素，
还要加上该步骤的 `syncTimeout`）在 60 秒后失败，`execute` 在 30 秒后失败。
浏览器侧的调用本身仍会继续运行，因此下一个步骤会先等它结束，再发出自己的调用。有自身时限的调用（读取操作和
`execute`）最多等待该时限，然后同样失败，而这一等待会延续到之后的步骤。没有自身时限的动作（点击、选择、截图）
最多等待 60 秒，记录一条警告后继续。`navigate`、`historyAction` 以及用于确认导航已结束的定时器刷新从不等待，
期间打开的对话框会先被关闭，一如往常。只有在该调用仍在运行时，失败的步骤才不会写出 `page.dump.html`：
它会改为记录一条警告。

可用的输出值列表：
* result.url：当前页面的 URL
* result.timeseconds：动作的执行耗时
* result.title：当前页面的标题

### 导航 {#navigate}
导航到指定的 URL

#### 输入 {#input}
* URL：要导航到的 URL
* Reset：重置浏览器状态

#### 示例 {#example}
```yaml
  - type: web
    action:
      navigate:
        url: https://www.google.fr
```

### 查找 {#find}
通过选择器查找元素

[CSS 选择器教程链接](https://developer.mozilla.org/fr/docs/Web/CSS/CSS_Selectors)
[XPATH 选择器教程链接](https://developer.mozilla.org/fr/docs/Web/XPath)

#### 输入 {#input-1}
* Selector：用来标识 Web 元素的表达式
* Locator：查找元素所用的定位方式：`CSS` 或 `XPATH`，或[定位器](#locators)中的某一种

#### 输出 {#output}
* result.find：返回所识别出的对象个数

#### 示例 {#example-1}
```yaml
  - type: web
    action:
      find: 
        selector: .gsfi
        locator: CSS
    assertions:
    - result.find ShouldEqual 2

  - type: web
    action:
      find:
        selector: //div[@class='FPdoLc lJ9FBc']/center/input[@value='Recherche Google']
        locator: XPATH
    assertions:
    - result.find ShouldEqual 1
    - result.value ShouldEqual Recherche Google
```

### 定位器 {#locators}
`find` 元素在 `locator` 中指明它的类型。除了 `CSS` 和 `XPATH`，还有五种类型，对应用户看到的内容，或页面为测试而标记的内容。
请优先使用它们，而不是 CSS 路径：页面改版后它们依然有效。

#### 输入 {#input-2}
* `locator: ROLE`：`selector` 是 ARIA 角色（`button`、`link`、`textbox`、`checkbox`、`heading`、`listitem`，...）。
  `name` 可选，按元素的可访问名称过滤；空的 `name` 表示不按名称过滤
* `locator: TEXT`：`selector` 是元素的文本
* `locator: LABEL`：`selector` 是表单字段标签的文本
* `locator: PLACEHOLDER`：`selector` 是字段的占位符（placeholder）
* `locator: TESTID`：`selector` 是测试 id。它读取属性 `data-testid`，或变量 `testIdAttribute` 所设置的属性。
  它匹配整个值，不接受 `exact`
* `exact`（`true` 或 `false`，默认 `false`）：对 `ROLE` 作用于 `name`，对 `TEXT`、`LABEL` 和 `PLACEHOLDER` 作用于 `selector`

默认情况下，`TEXT`、`LABEL` 和 `ROLE` 的 name 不区分大小写，按子串匹配，并忽略多余的空白：文本 `add` 可以匹配按钮 `Add item`。
使用 `exact: true` 时，匹配的是完整文本，并按所写的大小写。`PLACEHOLDER` 按属性原样匹配。

值始终按纯文本读取。它绝不是 CSS 选择器、引擎前缀或正则表达式。找到多个元素的步骤使用第一个。

#### 示例 {#example-2}
```yaml
  - type: web
    action:
      click:
        find:
          locator: ROLE
          selector: button
          name: Add item
          exact: true

  - type: web
    action:
      fill:
      - find:
          locator: LABEL
          selector: Email
        text: user@example.com

  - type: web
    action:
      find:
        locator: TESTID
        selector: cart-count
    assertions:
    - result.find ShouldEqual 1
```

#### 错误 {#errors}
`find` 有误的步骤会在执行动作之前失败，消息中会指出出错的键：
* `locator` 不是这七种类型之一：`invalid find element locator 'X' (must be CSS, XPATH, ROLE, TEXT, LABEL, PLACEHOLDER or TESTID)`
* `ROLE` 的选择器不是 ARIA 角色：消息会列出这些角色
* `selector` 为空，适用于 `TEXT`、`LABEL`、`PLACEHOLDER` 或 `TESTID`
* 包含 NUL 字符的 `TESTID` 选择器
* `name` 用于 `ROLE` 以外的类型，或 `exact` 用于 `TESTID`：消息会说明哪些类型接受它
* `name` 不是字符串，或 `exact` 既不是 `true` 也不是 `false`
* 任何其他键：`invalid find element key 'X' for a ROLE locator (must be locator, selector, name, exact)`

这些检查适用于本节的五种类型。`CSS` 或 `XPATH` 元素的多余键会被忽略，一如既往。

### 点击 {#click}
通过选择器点击元素

#### 输入 {#input-3}
* Find：要查找的元素（更多信息见“查找”一节）
* Wait：点击之后的等待时间（单位为秒）
* SyncTimeout：等待元素就绪（存在、可见、可用、不在移动、未被遮挡）的最长时间，单位为秒。默认值：`30`

#### 示例 {#example-3}
```yaml
  - type: web
    action:
      click:
        find:
          selector: button
          locator: CSS
```

### 填写 {#fill}
用文本填写元素（input 或 textarea）。文本会输入到字段现有内容之后，与 WebDriver 一致；
它不会替换原有内容

#### 输入 {#input-4}
结构化输入的数组：
* Find：要查找的元素（更多信息见“查找”一节）
* Text：用文本更新元素的值
* Key：用按键更新元素的值
* SyncTimeout：等待元素就绪（存在、可见、可用、不在移动、未被遮挡）的最长时间，单位为秒。默认值：`30`

#### 示例 {#example-4}
```yaml
  - type: web
    action:
      fill:
      - find:
          selector: input
          locator: CSS
        text: userName
```

### 选择 {#select}
选择 Web 元素的某个选项

#### 输入 {#input-5}
* Find：要查找的元素（更多信息见“查找”一节）
* Text：要选择的选项
* Wait：点击之后的等待时间（单位为秒）
* SyncTimeout：等待元素就绪（存在、可见、可用、不在移动、未被遮挡）的最长时间，单位为秒。默认值：`30`

#### 示例 {#example-5}
```yaml
  - type: web
    action:
      select:
        find:
          selector: select
          locator: CSS
        text: Option 1
```

### 上传文件 {#upload-file}
在 Web 元素上上传文件

#### 输入 {#input-6}
* Find：要查找的元素（更多信息见“查找”一节）
* Files：字符串数组，用来定义要上传的文件
* Wait：点击之后的等待时间（单位为秒）
* SyncTimeout：等待元素就绪（存在、可见、可用、不在移动、未被遮挡）的最长时间，单位为秒。默认值：`30`

#### 示例 {#example-6}
```yaml
  - type: web
    action:
      uploadFile:
        find:
          selector: input
          locator: CSS
        files: 
        - toUpload.csv
```

### 等待 {#wait}
等待一段时间
用一个整数来定义要等待的秒数

#### 示例 {#example-7}
```yaml
  - type: web
    action:
      wait: 1
```

### Wait for {#wait-for}
等待元素处于某种状态。当页面在首次绘制之后才加载数据或打开面板时使用。

#### 输入 {#input-7}
* Find：要等待的元素（更多信息见 find 和[定位器](#locators)）
* State：`visible`（默认；`state` 为空时也一样）、`hidden`、`attached` 或 `detached`
  * `visible`：元素存在并且已显示
  * `hidden`：元素不存在或未显示
  * `attached`：元素存在于页面中，无论是否显示
  * `detached`：元素不在页面中
* SyncTimeout：等待该状态的最长时间，单位为秒。默认值：`30`。取值为 `0` 或更小时按 `30` 处理

对于 `visible`，该步骤等待第一个匹配项。

每次检查都看此刻匹配的第一个元素。如果该元素被移除，而另一个元素仍然匹配，等待会继续，改看下一个元素。`detached` 会一直等到不再有元素匹配：第一个元素被移除后，下一个元素就成为第一个，所以有多个匹配项时，它会等到一个都不剩。`hidden` 跟随第一个匹配的元素，只要该元素还存在，它就始终是同一个元素（用 CSS 隐藏的元素仍然匹配），所以有多个匹配项时，`hidden` 可能在其他元素仍显示时就结束。要等到每一个都被隐藏，请等待 `detached`，或为每个元素各给一个 locator。

#### 示例 {#example-8}
```yaml
  - type: web
    action:
      waitFor:
        find:
          locator: TEXT
          selector: Order saved
        state: visible
        syncTimeout: 10

  - type: web
    action:
      waitFor:
        find:
          locator: TESTID
          selector: spinner
        state: hidden
```

#### 错误 {#errors-1}
* `waitFor: the field 'find' is required (allowed: find, state, syncTimeout)`
* `waitFor: invalid 'state' X (allowed: visible, hidden, attached, detached)`：状态区分大小写
* 除 `find`、`state` 和 `syncTimeout` 之外的字段：`waitFor: unknown field 'X' (allowed: find, state, syncTimeout)`
* 状态没有在限定时间内达到：`waitFor: the element ROLE "button" name "Add" did not become visible within 30s`，后面跟着浏览器给出的原因

### Hover {#hover}
把指针移到元素上，以打开悬停时才显示的菜单或提示。该步骤会像点击一样，等待元素就绪。

#### 输入 {#input-8}
* Find：要悬停的元素（更多信息见 find 和[定位器](#locators)）。使用第一个匹配项
* SyncTimeout：等待元素就绪的最长时间，单位为秒。默认值：`30`

#### 示例 {#example-9}
```yaml
  - type: web
    action:
      hover:
        find:
          locator: ROLE
          selector: link
          name: Products

  - type: web
    action:
      waitFor:
        find:
          locator: ROLE
          selector: menuitem
          name: Shoes
```

#### 错误 {#errors-2}
* `hover: the field 'find' is required (allowed: find, syncTimeout)`
* 除 `find` 和 `syncTimeout` 之外的字段：`hover: unknown field 'X' (allowed: find, syncTimeout)`
* 元素没有在限定时间内就绪：`hover: ROLE "link" name "Products"`，后面跟着浏览器给出的原因

### 拖放 {#drag-and-drop}
拖动一个元素并放到另一个元素上。该步骤会等待两者都就绪。

#### 输入 {#input-9}
* From：要拖动的元素（更多信息见 find 和[定位器](#locators)）。使用第一个匹配项
* To：要放置的目标元素。使用第一个匹配项
* SyncTimeout：等待拖动完成的最长时间，单位为秒。默认值：`30`

#### 示例 {#example-10}
```yaml
  - type: web
    action:
      dragAndDrop:
        from:
          locator: TEXT
          selector: Task A
          exact: true
        to:
          locator: TESTID
          selector: column-done
```

#### 错误 {#errors-3}
* `dragAndDrop: the field 'from' is required (allowed: from, to, syncTimeout)`，`to` 同理
* 除 `from`、`to` 和 `syncTimeout` 之外的字段：`dragAndDrop: unknown field 'X' (allowed: from, to, syncTimeout)`
* 放置失败或超时：`dragAndDrop: TEXT "Task A" exact to TESTID "column-done"`，后面跟着浏览器给出的原因

### Press {#press}
按下一个键或一个组合键，例如 `Enter`、`Escape`、`Tab` 或 `Control+A`。使用 `find` 时，按键发送给该元素。
不使用时，按键发送给拥有焦点的元素。`press` 接受一个键或一个组合键，而不是文本：要输入文本，请使用 `fill`。

#### 输入 {#input-10}
* Key：键或组合键（必填）。区分大小写。使用 Playwright 的键名（`Enter`、`ArrowDown`、`F5`、`Backspace`）
  或单字符键（`a`、`7`）。组合键用 `+` 连接各个键：`Control+A`、`Shift+Tab`。`fill` 的 `key` 接受的 WebDriver
  名称（`ENTER`、`ESCAPE`、`ARROW_DOWN`）也可以使用，但只能作为单个键。组合键只接受 Playwright 的名称。`press` 接受固定列表中的键名；不在列表中的少见 Playwright 键
  （`ShiftLeft`, `ControlLeft`, `CapsLock`, `ContextMenu`, `NumLock`, `ScrollLock`, `PrintScreen`, `AltGraph`，以及 `F13` 及以上）会被拒绝，错误信息会指出该键名，绝不会发送错误的键。单字符键始终被接受
* Find：接收按键的元素（更多信息见 find 和[定位器](#locators)）。可选。使用第一个匹配项
* SyncTimeout：等待元素就绪的最长时间，单位为秒。默认值：`30`。没有 `find` 时不涉及任何元素，因此它不是对元素的等待：它只延长按键调用本身的限制（60 秒，再加 `syncTimeout`）

#### 示例 {#example-11}
```yaml
  - type: web
    action:
      press:
        key: Enter
        find:
          locator: PLACEHOLDER
          selector: Search

  - type: web
    action:
      press:
        key: Control+A
```

#### 错误 {#errors-4}
* `press: the field 'key' is required (allowed: key, find, syncTimeout): press takes one key or one chord`
* 除 `key`、`find` 和 `syncTimeout` 之外的字段：`press: unknown field 'X' (allowed: key, find, syncTimeout)`
* 不是文本的 `key`，例如 YAML 数字 `key: 5`：消息会提示加上引号（`key: "5"`）
* 未知的键，或一段文本：`press: invalid 'key' "X": not a key name; press takes one key or one chord`。对于组合键，
  消息会指出出错的部分
* Playwright 没有对应键的 WebDriver 键（`NULL`、`CANCEL`、`HELP`）：消息会说明它没有 Playwright 对应项

### 选择框架 {#select-frame}
选择当前页面中存在的某个框架

#### 输入 {#input-11}
* Find：要查找的框架（更多信息见“查找”一节）
* SyncTimeout：等待元素就绪（存在、可见、可用、不在移动、未被遮挡）的最长时间，单位为秒。默认值：`30`

#### 示例 {#example-12}
```yaml
  - type: web
    action:
      selectFrame:
        find:
          selector: "#iframeResult"
          locator: CSS
```

### 选择根框架 {#select-root-frame}
选择当前页面中存在的根框架
设为 true 的布尔值表示选择根框架

#### 示例 {#example-13}
```yaml
  - type: web
    action:
      selectRootFrame: true
```

### 下一个窗口 {#next-window}
选择浏览器的下一个窗口（标签页），从最后一个绕回第一个；只有一个窗口时，步骤
停留在该窗口上
设为 true 的布尔值表示选择下一个窗口

#### 示例 {#example-14}
```yaml
  - type: web
    action:
      nextWindow: true
```

### 后退 {#back}
返回上一页
设为 true 的布尔值表示返回上一页

#### 示例 {#example-15}
```yaml
  - type: web
    action:
      historyAction: back
    assertions:
    - result.title ShouldStartWith GitHub
    - result.url ShouldEqual https://github.com/
```

### 前进 {#forward}
前进到下一页
设为 true 的布尔值表示前进到下一页

#### 示例 {#example-16}
```yaml
  - type: web
    action:
      historyAction: forward
    assertions:
    - result.title ShouldStartWith GitHub
    - result.url ShouldEqual https://github.com/team
```

### 刷新 {#refresh}
刷新页面
设为 true 的布尔值表示刷新页面

#### 示例 {#example-17}
```yaml
  - type: web
    action:
      historyAction: refresh
    assertions:
    - result.title ShouldStartWith GitHub
    - result.url ShouldEqual https://github.com/team
```

### 确认弹窗 {#confirm-popup}
确认弹出的对话框（confirm 对话框和 alert 对话框）

由某个步骤打开的对话框会保持打开，等待下一个步骤用 `confirmPopup` 或 `cancelPopup` 来应答；对话框打开期间，
`result.title` 和 `result.url` 为空。对话框打开时执行任何其他动作，都会将其关闭，并以 `unexpected alert open` 失败，
与 WebDriver 一致。
设为 true 的布尔值表示确认弹出的对话框

#### 示例 {#example-18}
```yaml
  - type: web
    action:
      confirmPopup: true
```

### 取消弹窗 {#cancel-popup}
取消弹出的对话框
设为 true 的布尔值表示取消弹出的对话框

#### 示例 {#example-19}
```yaml
  - type: web
    action:
      cancelPopup: true
```

### 执行 JavaScript {#execute-javascript}
执行 JavaScript 代码。命令是一个函数的函数体：`return` 以及 `arguments[0]`、`arguments[1]`…… 的用法
与 WebDriver 一致

#### 输入 {#input-12}
* Command：要执行的 JavaScript 代码
* Args：字符串数组，用来定义 JavaScript 参数

#### 输出 {#output-1}
脚本的返回值（如果有的话）。当脚本返回 promise 时，步骤会等待其值。
* result.value：以文本形式表示的值。字符串保持原样。其他任何值都写成 JSON：`42`、`true`、
  `[1,"a"]`、`{"user":{"name":"Ada"}}`，对象的键按顺序排序
* result.valuejson：同一个值，带类型，如同 http executor 的 `result.bodyjson`。可用它来比较数字，
  或访问对象或列表的内部：`result.valuejson.user.name`

每个 Web 步骤的结果都有一个 `result.valuejson` 变量。除非步骤执行了会返回值的脚本，否则它为空。

脚本不返回任何内容、返回 `undefined` 或 `null` 时，两者都为空。少数值没有 JSON 形式：
* `NaN`、`Infinity` 和 `-Infinity` 读作这几个单词
* `Date` 读作其 UTC 时间，采用 RFC 3339 格式（`1970-01-01T00:00:00Z`）
* BigInt、URL 和 RegExp 读作其文本
* DOM 元素、`document` 或 `window` 读作 `ref: <Node>`、`ref: <Document>` 或 `ref: <Window>`

引用自身的值（例如带有指回该对象本身的属性的对象），会使两个字段都为空。
浏览器接受而 venom 无法读取的 URL 也一样，例如带有后面没有跟两个十六进制数字的 `%` 的 URL
（`https://a.test/x#50%off`）。venom 会记录一条警告，步骤通过。

#### 示例 {#example-20}
```yaml
  - type: web
    action:
      execute:
        command: "window.editor.setValue(\"<!DOCTYPE html> <html></html>\"); window.editor.save();"

  - type: web
    action:
      execute:
        command: "return {count: document.querySelectorAll('li').length, first: document.querySelector('li').innerText};"
    assertions:
    - result.valuejson.count ShouldEqual 3
    - result.valuejson.first ShouldEqual Apples
```


有关动作的更多信息 (https://github.com/ovh/venom/tree/master/executors/web/types.go)
对于动作，您可以用以下命令对浏览器截图：`screenshot: [fileName].png`

## 模拟网络调用 {#mock-network-calls}
步骤可以自己应答页面的网络请求，因此测试套件无需后端就能测试前端。在步骤中添加一个 `mock:` 列表，
紧挨着 `action:`。每个条目会应答其 `url` 所匹配的所有请求，这些请求不会到达网络。主机无需真实存在：
模拟可以提供页面本身，进而提供页面自己发起的调用。

#### 输入 {#input-13}
结构化输入的数组：
* Url：要应答的 URL 的模式（必填）。`*` 匹配除 `/` 之外的任意字符，`**` 匹配任意字符，
  `{a,b}` 匹配 `a` 或 `b`。模式匹配的是整个 URL：`**/api/users` 会应答
  `https://app.test/api/users`，但不会应答 `https://app.test/api/users?page=2`；`**/api/users*` 两者都会应答。每个 `{` 都需要
  对应的 `}`，否则步骤会失败：要表示字面意义的花括号，请写 `\{`（在双引号内写 `\\{`）
* Status：响应的 HTTP 状态码。默认值：`200`
* Headers：响应头。请把每个值写成字符串：`X-Total: "3"`，而不是 `X-Total: 3`
* ContentType：响应的 `Content-Type`。默认值：无
* Body：响应体，文本形式

模拟在步骤的动作运行之前就开始应答，并在同一测试用例的后续步骤中、在所有标签页里继续应答。
当多个模拟匹配同一个 URL 时，由最近设置的那个应答。下一个测试用例开始时没有任何模拟。步骤可以只有 `mock:` 而没有动作。

在弹窗打开时设置 `mock:`，会将其关闭并使该步骤失败，与其他任何动作一样。请在打开弹窗的那个步骤之前的步骤中设置模拟。

由页面的 service worker 自行应答的请求，永远不会到达模拟。

#### 示例 {#example-21}
```yaml
  - type: web
    mock:
    - url: "**/app"
      contentType: text/html
      body: "<title>Users</title><h1>Users</h1>"
    - url: "**/api/users"
      status: 200
      contentType: application/json
      headers:
        X-Total: "1"
      body: '[{"name": "Ada"}]'
    action:
      navigate:
        url: http://app.venom.invalid/app
    assertions:
    - result.title ShouldEqual Users

  - type: web
    action:
      execute:
        command: "return fetch('/api/users').then(r => r.json()).then(users => users[0].name);"
    assertions:
    - result.value ShouldEqual Ada
```

## 失败诊断 {#failure-diagnostics}
步骤失败时，Web executor 会在工作目录中写出 `page.dump.html`：当前框架的源码。
当页面仍在加载新文档时，它会跳过这次转储。带有 `screenshot:` 的步骤，在其动作成功之后会写出截图
以及转储。

设置变量 `trace: true` 后，失败的步骤还会写出 `page.trace.zip`，与 `page.dump.html` 放在一起：该步骤的 Playwright
跟踪，包含每个动作、动作前后的页面快照、截图以及网络调用。
请用 Playwright 的跟踪查看器打开它：

```bash
go run github.com/mxschmitt/playwright-go/cmd/playwright@v0.6201.1 show-trace page.trace.zip
```

查看器需要 Playwright 驱动；如果缺少，该命令会下载它，与安装命令一样。
下一个失败的步骤会替换这两个文件。通过的步骤不会写出跟踪。

跟踪包含该步骤所看到和输入的一切，**包括密码**，以及其网络调用的请求头和响应体。venom 对机密信息的屏蔽
并不适用于它。venom 写出的跟踪只有其所有者可读，与 `page.dump.html` 一样。请不要把它作为构建产物发布，也不要提交它。跟踪
还会拖慢每个步骤，在简单页面上约慢 20 ms：请在调查失败时才开启它。

## 陷阱 {#traps}
* 拼错的选择器会让 `waitFor` 在状态为 `hidden` 或 `detached` 时立即通过：不存在的元素既是隐藏的，也是已分离的。
  请先用 `find` 或 `visible` 等待检查选择器。
* `TESTID` 默认读取 `data-testid`。用 `data-test`（或其他名称）标记元素的页面，在测试套件设置 `testIdAttribute` 之前找不到元素。
  没有它，该步骤找不到元素。
* 页面转储（`page.dump.html`）和跟踪包含 `fill` 和 `press` 输入的内容。venom 的密钥屏蔽不适用于它们。
  不要用 `press` 一个键一个键地输入密钥。参见[失败诊断](#failure-diagnostics)。
* `press` 不是输入。它发送的是按键，不是文本：`press: Hello` 会失败。要输入文本，请使用 `fill`。

## 示例 {#example-22}
一个完整的示例

```yaml
name: TestSuite Web
vars:
  web:
    driver: chrome
    width: 1920
    height: 1080
    args:
    - 'browser-test'
    prefs:
      profile.default_content_settings.popups: 0
      profile.default_content_setting_values.notifications: 1
    timeout: 60
    debug: true
testcases:
- name: TestCase Google search
  steps:
  - type: web
    action:
      navigate:
        url: https://www.google.fr
    assertions:
    - result.title ShouldEqual Google
    - result.url ShouldEqual https://www.google.fr
  - type: web
    action:
      find: input[name="q"]
    assertions:
     - result.find ShouldEqual 1
  - type: web
    action:
      fill:
      - find: input[name="q"]
        text: "venom ovh"
  - type: web
    action:
      click:
        find: input[value="Recherche Google"]
        wait: 1
    screenshot: googlesearch.png

```
