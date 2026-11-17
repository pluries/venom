---
translates: web
title: Web
description: 如何在 venom 测试套件中使用 Web executor。
fingerprint: 841e8ee00f470e50ad60416a4c8b072458dd7b33e432bb145851d952a4649966
drafted_by: claude
drafted_on: 2026-11-16
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

对元素执行的动作（`click`、`fill`、`select`、`uploadFile`、`selectFrame`）会等待元素就绪，
最长等待其 `syncTimeout` 或 30 秒。`find` 从不等待：它统计此刻存在的元素个数，因此
`result.find ShouldEqual 0` 依然很快。在可能加载页面的动作（`click`、`fill`、`select`、
`uploadFile`、`execute`、弹窗应答）之后，步骤会等页面加载完成，再读取
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
* Locator：查找元素所用的定位方式（CSS 或 XPATH）

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

### 点击 {#click}
通过选择器点击元素

#### 输入 {#input-2}
* Find：要查找的元素（更多信息见“查找”一节）
* Wait：点击之后的等待时间（单位为秒）
* SyncTimeout：等待元素就绪（存在、可见、可用、不在移动、未被遮挡）的最长时间，单位为秒。默认值：`30`

#### 示例 {#example-2}
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

#### 输入 {#input-3}
结构化输入的数组：
* Find：要查找的元素（更多信息见“查找”一节）
* Text：用文本更新元素的值
* Key：用按键更新元素的值
* SyncTimeout：等待元素就绪（存在、可见、可用、不在移动、未被遮挡）的最长时间，单位为秒。默认值：`30`

#### 示例 {#example-3}
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

#### 输入 {#input-4}
* Find：要查找的元素（更多信息见“查找”一节）
* Text：要选择的选项
* Wait：点击之后的等待时间（单位为秒）
* SyncTimeout：等待元素就绪（存在、可见、可用、不在移动、未被遮挡）的最长时间，单位为秒。默认值：`30`

#### 示例 {#example-4}
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

#### 输入 {#input-5}
* Find：要查找的元素（更多信息见“查找”一节）
* Files：字符串数组，用来定义要上传的文件
* Wait：点击之后的等待时间（单位为秒）
* SyncTimeout：等待元素就绪（存在、可见、可用、不在移动、未被遮挡）的最长时间，单位为秒。默认值：`30`

#### 示例 {#example-5}
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

#### 示例 {#example-6}
```yaml
  - type: web
    action:
      wait: 1
```

### 选择框架 {#select-frame}
选择当前页面中存在的某个框架

#### 输入 {#input-6}
* Find：要查找的框架（更多信息见“查找”一节）
* SyncTimeout：等待元素就绪（存在、可见、可用、不在移动、未被遮挡）的最长时间，单位为秒。默认值：`30`

#### 示例 {#example-7}
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

#### 示例 {#example-8}
```yaml
  - type: web
    action:
      selectRootFrame: true
```

### 下一个窗口 {#next-window}
选择浏览器的下一个窗口（标签页），从最后一个绕回第一个；只有一个窗口时，步骤
停留在该窗口上
设为 true 的布尔值表示选择下一个窗口

#### 示例 {#example-9}
```yaml
  - type: web
    action:
      nextWindow: true
```

### 后退 {#back}
返回上一页
设为 true 的布尔值表示返回上一页

#### 示例 {#example-10}
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

#### 示例 {#example-11}
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

#### 示例 {#example-12}
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

#### 示例 {#example-13}
```yaml
  - type: web
    action:
      confirmPopup: true
```

### 取消弹窗 {#cancel-popup}
取消弹出的对话框
设为 true 的布尔值表示取消弹出的对话框

#### 示例 {#example-14}
```yaml
  - type: web
    action:
      cancelPopup: true
```

### 执行 JavaScript {#execute-javascript}
执行 JavaScript 代码。命令是一个函数的函数体：`return` 以及 `arguments[0]`、`arguments[1]`…… 的用法
与 WebDriver 一致

#### 输入 {#input-7}
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

#### 示例 {#example-15}
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

#### 输入 {#input-8}
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

#### 示例 {#example-16}
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

## 示例 {#example-17}
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
