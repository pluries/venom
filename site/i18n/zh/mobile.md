---
translates: mobile
title: Mobile
description: 如何在 venom 测试套件中使用 Mobile executor。
fingerprint: b5add67e3549852f87f31f8dbe9f13cb7b412228964257c765cad39d54a9af9c
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# Venom - Executor Mobile {#venom---executor-mobile}

用于从 venom 测试套件驱动原生或混合式 Android 或 iOS 应用的步骤：安装应用、查找元素、点按、输入、读取文本、滚动、截图、打开深度链接、切换到 web view、应答 iOS 系统警报，并对您看到的内容进行断言。它与一台 [Appium](https://appium.io) 3 服务器通信，该服务器运行在与 venom 相同的机器上，或者运行在 venom 通过 SSH 隧道连接的 Mac 上。Android 模拟器使用 UiAutomator2 驱动，iOS 模拟器使用 XCUITest 驱动。

状态：里程碑 M1（#2396）、M2（#2397）、M3（#2398）和 M4（#2399）。Android 在模拟器上、iOS 在模拟器上，均可在同一台机器上或通过 SSH 连接的 Mac 上运行；支持 web view、坐标、深度链接和警报。iOS 的调用针对脚本化服务器做过测试，并于 2026-10-28 和 2026-10-29 在模拟器上用内置的“设置”应用和 Appium 的 UIKitCatalog 运行过（`.zip` 安装、警报、`scroll` 加 `until`、一个 web view）。未在模拟器上运行过的有：`.app` 文件夹、坐标、`openURL`、`hideKeyboard` 以及警报的 `button` 标签（见「iOS」；详细的运行记录不公开）。

确认它可以工作：先运行封闭式测试（`go test ./executors/mobile/ -count=1`，不需要设备），再针对您自己的 Appium 和模拟器运行真实测试（见「运行测试」）。封闭式测试与一个脚本化的模拟服务器通信，因此无法证明 Appium 3 接受每一个请求：只有真实测试才能证明。

## 主机前提条件 {#host-prerequisites}

要为 Android 和 iOS 设置 Mac，请见下面的「设置 Mac」。

- 带有 UiAutomator2 驱动的 Appium 3（已用 Appium 3.9.0 和 uiautomator2 8.7.2 测试；Appium 2.x 未经验证），启动时要让它**只监听回环地址**（127.0.0.1）。
- 一个 Appium 能看到的 Android 模拟器（或设备）。
- 需要 Android SDK build-tools（`sdkmanager "build-tools;34.0.0"`），当 `mobile.app` 指向一个 `.apk` 时：UiAutomator2 需要 `aapt2` 来读取它。Appium 会缓存工具的查找结果，因此安装之后请重启 Appium。启动 Appium 的 shell 中必须设置 `ANDROID_HOME`。
- venom 与 Appium 在同一台机器上。M1 从 Appium 读取的本地文件路径安装应用，所以两者必须共用一台主机。

该 executor 不需要在 Appium 服务器上加 `--relaxed-security` 参数。

## 变量 {#variables}

在测试套件的 `vars: mobile:` 下设置它们（也可以在 `.venomrc` 中，或用 `--var`）。会话在每个测试用例的第一个 `mobile` 步骤之前打开一次，在它的最后一个步骤之后关闭，步骤失败时也一样。

```yaml
vars:
  mobile:
    url: http://127.0.0.1:4723
    platform: android
    device: emulator-5554
    app: ./build/app-debug.apk
```

| Variable | Required | Meaning |
|---|---|---|
| `mobile.url` | 是 | Appium 服务器的地址，`http` 或 `https`，其中不能带凭据。 |
| `mobile.platform` | 是 | `android` 或 `ios`。 |
| `mobile.device` | 是 | 设备或模拟器。在 Android 上，它会同时作为 `appium:deviceName` 和 `appium:udid` 发送给 Appium。在 iOS 上，它是模拟器名称（只作为 `appium:deviceName` 发送）或 UDID（同时也作为 `appium:udid` 发送）。executor 从不替您挑选设备。请使用 adb 序列号，例如 `emulator-5554`；像 `venom_test` 这样的 AVD 名称会被 Appium 拒绝（“not in the list of connected devices”）。 |
| `mobile.osVersion` | 否 | 作为 `appium:platformVersion` 发送。报告中显示服务器回答的版本，没有则显示此值。 |
| `mobile.app` | 否 | 会话打开时要安装的应用的路径：Android 上是 `.apk`，iOS 上是模拟器的 `.app` 文件夹或其 `.zip`。相对路径相对于测试套件所在的文件夹解析。 |
| `mobile.appSHA256` | 否 | `.apk` 或 `.zip` 的预期 SHA-256，十六进制（大写或小写均可）。需要 `mobile.app`。与 `.app` 文件夹一起使用会被拒绝。 |
| `mobile.appId` | 否 | 应用的包 id（Android）或 bundle id（iOS）。没有 `mobile.app` 时，会话会启动这个已安装的应用，此时在 iOS 上它是必填的。两者都有时，它也是 `launch` 和 `terminate` 的默认值。 |
| `mobile.ssh.host` | 否 | Mac 的主机（名称或地址）。设置后，venom 会通过 SSH 隧道连到该主机自身的回环地址（端口取自 `mobile.url`）来访问 Appium 服务器，并把应用复制到那里。不设置时，按原样使用 `mobile.url`。 |
| `mobile.ssh.user` | 有 `ssh.host` 时必填 | SSH 用户。 |
| `mobile.ssh.key` | 有 `ssh.host` 时必填 | 私钥的路径。只支持密钥认证，不支持密码。 |
| `mobile.ssh.passphrase` | 否 | 密钥的口令。请在 `secrets` 下声明它。 |
| `mobile.ssh.port` | 否 | SSH 端口，默认 22。 |
| `mobile.ssh.knownhosts` / `mobile.ssh.hostkey` | 否 | 主机密钥检查，与 ssh executor 中相同：一个 known_hosts 文件，或一个固定的密钥。二者不能同时设置。两者都没有时，venom 每次运行警告一次，并且不检查主机密钥。 |
| `mobile.capabilities` | 否 | 额外的 Appium capabilities，为一个映射。见下文。 |
| `mobile.syncTimeout` | 否 | executor 等待元素的秒数，默认 `10`。 |
| `mobile.sessionTimeout` | 否 | 打开会话允许的秒数，Android 上默认 `180`，iOS 上默认 `300`，因为 iOS 上的第一个会话要构建 WebDriverAgent。 |
| `mobile.idleTimeout` | 否 | 服务器在没有命令时保持会话打开的秒数，默认 `120`（作为 `appium:newCommandTimeout` 发送）。 |

三个超时中任何一个为负数都会被拒绝。错误的值会在发送任何请求之前被拒绝。

### 应用文件及其 SHA-256 {#the-app-file-and-its-sha-256}

设置了 `mobile.app` 时，venom 会读取该文件一次并计算它的 SHA-256。如果设置了 `mobile.appSHA256` 且两者不同，测试用例会**在打开任何会话之前**失败，消息中会给出两个哈希。计算出的哈希总是显示在报告中（`appsha256`），因此报告会说明运行的是哪个构建。设置了 `mobile.appSHA256` 却没有 `mobile.app` 会被拒绝。

没有 SSH 隧道时，哈希覆盖的是 venom 读取的字节，而 Appium 随后才从该路径安装：中间被替换的文件不会被发现。使用 `mobile.ssh.*` 时，文件会被复制到 Mac，哈希覆盖的是发送出去的字节（见「通过 SSH 连接 Mac」）。

### 额外的 capabilities {#extra-capabilities}

`mobile.capabilities` 向会话请求添加 capabilities。executor 自己设置的 capability 会在任何请求之前被拒绝，消息中会指出拥有它的变量。比较名称时不带 `appium:` 前缀，`appium:options` 内部也一样。

| Capability | Set by |
|---|---|
| `platformName` | `mobile.platform` |
| `deviceName`, `udid` | `mobile.device` |
| `platformVersion` | `mobile.osVersion` |
| `app` | `mobile.app` |
| `appPackage`, `bundleId` | `mobile.appId` |
| `automationName` | executor 本身（Android 上是 UiAutomator2，iOS 上是 XCUITest） |
| `newCommandTimeout` | `mobile.idleTimeout` |
| `autoAcceptAlerts`, `autoDismissAlerts` | alert 操作：由该步骤处理警报，所以会话不能处理 |

## 输入 {#input}

```yaml
  - action (mandatory): one of the 17 actions below
  - accessibilityId, text, id, xpath, uiautomator, css, name, predicate, classChain (one of them, see "Locators"): which element the step targets
  - choice (alert): accept (default) or dismiss, in any case
  - button (alert): the label of the button to press, instead of the default one
  - syncTimeout (optional): seconds to wait for the element in this step, overrides mobile.syncTimeout
  - value (fill): the text to type
  - clear (fill): true to clear the field first
  - attribute (read): read this attribute instead of the text
  - state (waitFor): present (default) or absent
  - direction (swipe, scroll): up, down, left or right
  - until (scroll): true to scroll until the element is visible
  - percent (swipe, scroll): the part of the area to cover, above 0 and at most 1, default 0.75
  - x, y (tap, swipe): the point to tap, or where the swipe starts; 0 is valid. Not with a locator
  - toX, toY (swipe): where a swipe from a point ends; give both, and no direction or percent
  - unit (tap, swipe): the unit of x, y, toX, toY: pixels (default, also px) or percent. Percent is of the window rect (0 to 100)
  - appId (launch, terminate): the package id, default mobile.appId
  - path (screenshot): the file to write, relative to the directory venom runs in
  - context (context): native, a web view name starting with WEBVIEW, or empty for the first web view
  - url (openURL): the deep link, scheme://..., at most 2048 bytes, no spaces or control characters
```

`text` 是一个**定位器**（它按元素的可见文本查找元素）。您要输入的文本是 `value`。

### 定位器 {#locators}

针对元素的步骤要给出**恰好一个**定位器。零个或两个都会在任何请求之前报错。

| Field | Finds the element by |
|---|---|
| `accessibilityId` | 它的无障碍 id（content description） |
| `text` | 它完整的可见文本（UiSelector 的 `text`）；其中的引号和反斜杠会被转义 |
| `id` | 它的资源 id |
| `xpath` | 针对视图层级的 XPath |
| `uiautomator` | UiAutomator 选择器，按原样发送 |
| `css` | CSS 选择器，仅限 web view 内部 |
| `name` | 仅限 iOS：元素名称 |
| `predicate` | 仅限 iOS：NSPredicate 字符串，按原样发送 |
| `classChain` | 仅限 iOS：iOS class chain，按原样发送 |

`id` 和 `uiautomator` 仅限 Android；`name`、`predicate` 和 `classChain` 仅限 iOS。另一个平台的类型会在任何请求之前被拒绝，消息中会说明该改用什么。iOS 的对应关系见「iOS」。

元素是会被等待的：executor 每 500 ms 询问一次，直到元素出现或等待结束（默认 10 s，或取步骤的 `syncTimeout`，或 `mobile.syncTimeout`）。如果元素始终没有出现，失败消息会指出定位器和等待时间。第一次就找到的元素不会增加任何延迟。

## 操作与结果 {#actions-and-results}

| Action | Needs | What it does | Result keys it fills |
|---|---|---|---|
| `tap` | 一个定位器，或 `x` 和 `y`（可选 `unit`） | 点击元素，或点按该坐标点 | `found`（使用定位器时） |
| `fill` | 一个定位器、`value`，可选 `clear` | 把 `value` 输入到元素中；当 `clear: true` 时先清空它。空的 `value` 什么也不输入 | `found` |
| `clear` | 一个定位器 | 清空元素 | `found` |
| `read` | 一个定位器，可选 `attribute` | 读取元素的文本，或其属性 | `found`、`text`（没有 attribute 时）或 `value`（有 attribute 时；缺失的属性读为空） |
| `count` | 一个定位器 | 立即统计匹配的数量，不等待。零是一个答案，不是失败 | `count`、`found`（大于零时为 true） |
| `waitFor` | 一个定位器，可选 `state` | 等待元素出现（默认）或消失 | `found` |
| `swipe` | `direction`，可选定位器和 `percent`；或 `x`、`y`、`toX`、`toY`（可选 `unit`），从一个点拖动到另一个点 | 在元素内部滑动；没有定位器时，在设备的可见区域内滑动，该区域是窗口每边向内缩进 10 percent 后的范围，因此永远不会碰到系统手势的边缘 | 给出定位器时为 `found` |
| `scroll` | `direction`，可选定位器和 `percent` | 在元素或可见区域内滚动（窗口每边向内缩进 10 percent，因此永远不会碰到系统手势的边缘）。使用 `until: true` 和定位器时，它会滚动可见区域，直到该元素出现在屏幕上，最多滚动 10 次 | 给出定位器时为 `found` |
| `back` | 无 | 按下“返回”键 | 无 |
| `hideKeyboard` | 无 | 隐藏软键盘。如果没有显示键盘，这不算失败 | 无 |
| `launch` | `appId` 或 `mobile.appId` | 把应用带到前台（`mobile: activateApp`） | 无 |
| `terminate` | `appId` 或 `mobile.appId` | 停止应用（`mobile: terminateApp`） | 无 |
| `screenshot` | 可选 `path` | 截取屏幕截图。有 `path` 时，把 PNG 写到那里（权限 0600） | `found`、`screenshot`（路径） |
| `source` | 无 | 以 XML 返回视图层级 | `found`、`source` |
| `context` | `context`：`native`、一个 web view 名称（`WEBVIEW_...`），或留空表示第一个 web view | 在原生应用和 web view 之间切换。切换到 web view 时，会等待它出现在列表中（使用步骤的等待时间）。`css` 定位器需要 web view | `context`（当前活动的）、`contexts`（服务器给出的列表；切换到 native 之后为空） |
| `openURL` | `url` | 打开深度链接：`url` 为 scheme://...，已知应用包时限定在该应用包内（`mobile: deepLink`） | 无 |
| `alert` | 可选 `choice` 和 `button` | 仅限 iOS：接受或关闭当前打开的系统警报（见「iOS」） | `text`（警报文本） |

未知的操作会被拒绝，并列出受支持的操作。某个操作在 iOS 上有差异时，见「与 Android 的差异」。

## 输出 {#output}

```yaml
  - result.text: the text read (read)
  - result.value: the attribute read (read with attribute)
  - result.count: the number of matches (count)
  - result.found: true when the step found its element, or the screen content
  - result.screenshot: the path written (screenshot with path)
  - result.source: the view hierarchy as XML (source)
  - result.timeseconds: duration of the step
  - result.err: the failure of the step, empty when it worked
  - result.platform, result.device, result.osversion: where the test ran
  - result.app, result.appsha256, result.appid: what ran
```

运行信息（`platform`、`device`、`osversion`、`app`、`appsha256`、`appid`）会在**每一个**结果中重复出现，因此报告中的每个测试用例都会显示哪个构建在哪台设备上运行。没有值的键不会出现在结果中。

默认断言：`result.err ShouldBeEmpty`。

### 错误 {#errors}

- **误用**（两个定位器、未知的操作、错误的 `direction`、缺少设置）是在发送任何请求之前产生的 venom 错误。步骤会失败，并给出指明原因的消息。
- **服务器或设备上的失败**（找不到元素、命令被拒绝、服务器不可达、调用挂起）会出现在 `result.err` 中，形式为 `action <name>: <message>`。步骤在默认断言上失败，运行继续。Appium 的堆栈跟踪绝不会放进结果。

## Web view 与混合式应用 {#web-views-and-hybrid-apps}

混合式应用会在原生界面中显示网页。`context` 操作用于在两者之间切换。

- `context: native` 回到原生应用。
- `context: WEBVIEW_<package>` 切换到该 web view。
- 空的 `context` 切换到服务器列出的第一个 web view。

对于 web view，executor 最多等待步骤的等待时间（`syncTimeout`、`mobile.syncTimeout`）让它出现在列表中，因此加载较晚的页面也能找到。如果在时限内没有出现，失败消息会列出服务器实际报告的 context。结果中包含 `context`（当前活动的）和 `contexts`（服务器给出的列表）。

在 web view 内部，搜索由 chromedriver 处理，因此应使用 `css` 和 `xpath` 定位器。`text`、`accessibilityId` 和 `uiautomator` 是原生策略，在 web view 中会被服务器拒绝（已在设备上检查，2026-10-28：`accessibilityId` 返回了错误）；`id` 在那里的含义（HTML 的 id）未经验证。执行 `context: native` 之后，原生定位器照常工作。在 native context 中使用 `css` 会在任何搜索之前被拒绝并给出消息。`source` 在 web view 中返回页面的 HTML，在 native context 中返回 XML 层级。

```yaml
      - type: mobile
        action: context
        context: WEBVIEW_com.example.app
      - type: mobile
        action: fill
        css: "#email"
        value: demo-user
      - type: mobile
        action: tap
        css: "button[type=submit]"
      - type: mobile
        action: context
        context: native
      - type: mobile
        action: waitFor
        accessibilityId: welcome_banner
```

### Web view 需要什么 {#what-the-web-view-needs}

- **必须开启调试。** 应用必须启用 WebView 调试。否则服务器只会列出 `NATIVE_APP`，切换到 web view 会失败，并给出该列表。
- **匹配的 chromedriver。** UiAutomator2 通过一个版本与设备上的 WebView 相匹配的 chromedriver 来驱动 web view。版本不匹配会使服务器上的切换失败。请设置 `appium:chromedriverExecutable`（Appium 主机上的路径），方式是通过 `mobile.capabilities`，或在服务器自己的配置中：

  ```yaml
  vars:
    mobile:
      capabilities:
        appium:chromedriverExecutable: /opt/chromedrivers/chromedriver-114
  ```
- **是否自动下载由运维人员决定。** Appium 服务器可以自己获取 chromedriver（`chromedriver_autodownload`）。这需要运维人员在 Appium 服务器上设置一个服务器选项。venom 从不发送它，也从不需要 `--relaxed-security` 或 `--allow-insecure`。

## 没有标签的应用与坐标 {#unlabelled-apps-and-coordinates}

不公开任何无障碍标签的应用（某些 Flutter release 构建、游戏、自绘视图）只能通过坐标来操作。如果应用有任何标签、id 或文本，请优先使用定位器：布局、屏幕尺寸或字体一变，坐标就会失效。

`tap` 和 `swipe` 用一个坐标点代替定位器：

| Field | Meaning |
|---|---|
| `x`, `y` | 要点按的点，或滑动的起点。0 是有效值 |
| `toX`, `toY` | 滑动的终点。两个都要给出，并且不能有 `direction` 或 `percent` |
| `unit` | `pixels`（默认，也可写 `px`）或 `percent` |

使用 `unit: percent` 时，该点是窗口矩形的一个比例，取值 0 到 100：`x: 50` 和 `y: 50` 就是中心。在 1080x2400 的窗口上，它发送的点是 540, 1200。像素值按给定值发送，由服务器限定范围。带定位器的坐标点、`x` 而没有 `y`、负值、NaN 或无穷大、超过 10 百万的像素值，或超过 100 的百分比，都会在任何请求之前被拒绝。

百分比坐标，以及不带定位器的 `swipe` 或 `scroll`，会从服务器读取窗口尺寸。在 web view 内部，该尺寸可能是浏览器的单位，而不是设备的像素：请先切换回 native（`context: native`），再使用百分比坐标或不带定位器的 `swipe` 和 `scroll`。未在设备上验证：手工运行只在 native context 中使用过像素和百分比坐标。

```yaml
      - type: mobile
        action: tap
        x: 50
        y: 50
        unit: percent
      - type: mobile
        action: swipe
        x: 540
        y: 1800
        toX: 540
        toY: 600
```

## 深度链接 {#deep-links}

`openURL` 通过 Appium 的 `mobile: deepLink` 在应用中打开一个链接：

```yaml
      - type: mobile
        action: openURL
        url: myapp://orders/42
```

- `url` 必须形如 `scheme://...`，不含空格或控制字符，最长 2048 字节。其他任何内容都会在任何请求之前被拒绝，并且消息不会重复该链接（它可能带有令牌）。
- 已知应用包时（步骤的 `appId`，否则是 `mobile.appId`），链接会被限定在该应用包内。
- 自定义 scheme 是被接受的：这正是深度链接的意义所在。链接会传给 Appium 的 `mobile: deepLink`，由它通过 `am start` 运行；venom 不构建任何 shell 命令。在测试过的环境中（Appium 3.9.0、uiautomator2 8.7.2、Android 14 模拟器），五个带有 shell 元字符的链接到达了服务器，因为没有任何程序处理它们而失败，并且没有在设备上创建任何文件。较旧的 Appium 或驱动可能不会为它们加引号：请不要把来自不受信任输入的深度链接放进测试套件。

## 限制 {#limits}

| Limit | Value |
|---|---|
| 对服务器的一次调用 | 60 s |
| 打开会话 | 180 s（`mobile.sessionTimeout`） |
| 服务器上的空闲会话 | 120 s（`mobile.idleTimeout`） |
| 两次尝试查找元素之间的停顿 | 500 ms |
| 等待元素的默认时间 | 10 s（`mobile.syncTimeout`、`syncTimeout`） |
| `scroll` 加 `until` 时的滚动次数 | 10 |
| 解码后的一张截图 | 12 MiB |
| 关闭会话 | 15 s |
| 两个诊断请求中的每一个 | 15 s |

服务器返回的重定向不会被跟随。步骤自己的 `timeout:` 仍然叠加在这些限制之上生效。

## 失败步骤的诊断信息 {#diagnostics-on-a-failing-step}

当一个步骤在服务器或设备上失败时，executor 会在 venom 运行所在的目录中保存两个文件：

- `screen.png`：设备的截图；
- `screen.dump.xml`：视图层级。

**警告：这两个文件都会显示屏幕上的内容，包括您输入的或应用显示的机密。** 它们不在 venom 报告的机密隐藏范围之内。它们以权限 0600 写入工作文件夹。请不要把它们作为构建产物发布。下一个失败的步骤会替换它们；通过的步骤不会写入也不会删除任何东西。获取其中一个文件失败只会在日志中产生一条警告，不会掩盖步骤自身的失败。误用错误，或服务器已不存在的会话，不会写入任何东西。

在**同一个目录**中启动的两次 venom 运行会互相覆盖对方的文件。请从不同的目录运行它们。

## 通过 SSH 连接 Mac {#reaching-the-mac-through-ssh}

让 Appium 保持监听 Mac 的回环地址（`127.0.0.1:4723`）；它自己没有密码。设置 `mobile.ssh.host`、`mobile.ssh.user` 和 `mobile.ssh.key`，并让 `mobile.url` 保持为 Mac 所见的地址（`http://127.0.0.1:4723`）：venom 在测试用例开始时打开隧道，在结束时关闭它。隧道只监听 `127.0.0.1`，并且总是连到 Mac 的回环地址。

- **主机密钥：** 设置 `mobile.ssh.knownhosts` 或 `mobile.ssh.hostkey`。不匹配的密钥会在任何请求之前拒绝连接。
- **应用文件：** 使用隧道时，`mobile.app` 会被复制到 Mac 上的 `/tmp/venom-mobile-<random>/app.apk`（iOS 上是 `app.zip`），从那里安装，并在会话结束时删除。在 Mac 上运行的命令中不会出现任何测试套件的值。`mobile.appSHA256` 是针对发送出去的字节检查的；不匹配会在任何会话之前失败，并删除该副本。被强行终止的运行可能会把该文件夹留下：请查找 `/tmp/venom-mobile-*`。
- **限制：** 通过隧道的 `https://` URL 是针对 `127.0.0.1` 验证的，因此在隧道内请使用 `http://`。连接和 SSH 握手受 `mobile.sessionTimeout`（默认 180 s）限制，所以不应答的主机会让 Setup 失败，而不是让它挂起。
- **没有隧道的明文 http：** 指向另一台机器的 `http://` URL 每次运行会打印一次警告；回环 URL 则不打印。

## 安全说明 {#security-notes}

- **Appium 必须只监听回环地址。** 网络上的 Appium 服务器会让任何能连上它的人控制模拟器和这台机器。在 M1 中，venom 连接同一台主机上的 Appium。
- 应用是从**本地文件**安装的，所以在 M1 中 Appium 和 venom 必须共用一台主机。
- **截图、转储文件和 `result.source` 会显示屏幕上的内容，包括您输入的机密。** 它们不在 venom 报告的机密隐藏范围之内，这与 web executor 的 trace 的限制相同。请不要让 `screen.png` 和 `screen.dump.xml` 出现在已发布的产物和共享的构建文件夹中。
- `fill` 从不在结果或 `-v` 调用日志中回显所输入的文本。使用 `-v` 时，对服务器的每次调用都会被记录（方法、路径、耗时；从不记录请求体）。
- 只使用 Appium 3 的端点，并且 executor 不需要 `--relaxed-security`。
- **web view 的页面内容和输入其中的文本可能出现在截图和 `result.source` 中**，与任何原生界面一样，并且机密隐藏不覆盖它们。用 `read` 读取的页面文本和任何其他结果值一样。
- **深度链接 URL 会传给 Appium 的 `mobile: deepLink`，由它通过 `am start` 运行。** venom 自己从不构建 shell 命令；链接会检查形状和长度，并在已知应用包时限定在该应用包内。在测试过的环境中，Appium 会为 shell 元字符加引号（Appium 3.9.0、uiautomator2 8.7.2、Android 14 arm64 模拟器，2026-10-28 测得）。较旧的 Appium 或驱动可能不会，所以请不要把来自不受信任输入的深度链接放进测试套件。
- **chromedriver 运行在 Appium 主机上**，使用哪个二进制文件由运维人员决定。venom 不发送任何会开启 `--relaxed-security` 或 `--allow-insecure` 的选项，服务器也都不需要。
- **iOS 警报的文本** 只会进入 `result.text`，不会去别处。它不会写进日志行或错误中，但它和任何其他结果值一样，所以其中的机密只有在是已声明的机密时才会被隐藏。
- **`.app` 文件夹会以 `app.zip` 的形式复制到 Mac**，放在一个由 venom 命名的文件夹中（`/tmp/venom-mobile-<random>`，权限 0700），在那里运行的任何命令中都没有测试套件的值。venom 会拒绝含有符号链接的 `.app`，并且不为文件夹计算哈希：它的完整性不会被检查。需要检查时，请提供 `.zip` 和 `mobile.appSHA256`。
- **XCUITest 驱动和 WebDriverAgent 运行在 Mac 上**，由运维人员安装。venom 不获取也不构建它们；第一个会话会在那里构建 WebDriverAgent。

## iOS {#ios}

使用 `mobile.platform: ios` 时，同一个测试套件通过 XCUITest 驱动来驱动 iOS **模拟器**上的应用。真实的 iPhone 不在覆盖范围内。iOS 的调用是根据 XCUITest 驱动文档所述的行为实现的，并针对脚本化服务器做过测试。它们在模拟器上运行过（iPhone 17、iOS 26.5、Appium 3.9.0、XCUITest 驱动 12.22.3），共两次：

- 2026-10-28，通过 bundle id 启动内置的“设置”应用，测试套件 `examples/handrun-settings.yml`，3 个测试用例全部通过（3 of 3）：打开会话、`waitFor`、`tap`、按方向滑动、`terminate`、`launch`、定位器 `text`、`name`、`predicate` 和 `classChain`、`count`，以及没有打开警报时的 `alert`。
- 2026-10-29，以 `UIKitCatalog.zip` 提供并带 `mobile.appSHA256` 的 Appium UIKitCatalog v4.0.5，通过 SSH 隧道，测试套件 `examples/handrun-uikitcatalog.yml`，最终状态 PASS：`.zip` 安装、`alert` 接受（警报文本在 `result.text` 中返回）和关闭、`scroll` 加 `until`，以及一个 web view（`context` 报告了一个 `WEBVIEW` context，`css: body` 统计出大于 0，`context: native` 切换回去了）。iOS 的 web view 不需要 chromedriver。

未在模拟器上运行过：`.app` 文件夹、坐标、`openURL`、`hideKeyboard`、警报的 `button` 标签、百分比坐标。详细的运行记录不公开。
类似“设置”的列表中的行在 iOS 26 上不是 `XCUIElementTypeCell`：请把 class chain 写成 `**/*[...]`，不要使用 cell 类型。
第一个会话花了几分钟（构建 WebDriverAgent），这就是默认会话超时为 300 s 的原因。

### Mac 上的前提条件 {#prerequisites-on-the-mac}

- Xcode 及其命令行工具，以及至少一个模拟器。
- 带有 XCUITest 驱动的 Appium 3：`appium driver install xcuitest`。与 Android 一样，让 Appium 保持在回环地址上。
- 第一个会话会构建 WebDriverAgent，需要几分钟。这就是 `mobile.sessionTimeout` 在 iOS 上默认为 300 s 的原因。
- 模拟器不需要代码签名。
- 在 Apple Silicon Mac 上，应用必须有 arm64 切片，并且那里的 Android 模拟器镜像也是 arm64。只有 i386 或 x86_64 切片的 iOS `.app` 会被 Appium 拒绝（“does not support the arm64 Simulator architecture”）；只有 `armeabi-v7a` 和 `x86` 库的 APK 会以 `INSTALL_FAILED_NO_MATCHING_ABIS` 失败（它需要 `arm64-v8a`，或者没有原生库）。Cloud Grey 的 TheApp v1.10.0（2019）在维护者的 Mac 上两种情况都失败了。
- 一个可直接用来试用 iOS 的示例：Appium 的 UIKitCatalog（`UIKitCatalog.zip`，带有 arm64 和 x86_64 切片的模拟器构建，bundle id 为 `com.example.apple-samplecode.UICatalog`），可从 <https://github.com/appium/ios-uicatalog> 的发布页面获取。测试套件 `examples/handrun-uikitcatalog.yml` 就在它上面运行。

### 在 iOS 上有变化的设置 {#settings-that-change-on-ios}

| Setting | On iOS |
|---|---|
| `mobile.device` | 模拟器名称（`iPhone 16`），或它的 UDID。UDID 也会作为 `appium:udid` 发送；名称只作为 `appium:deviceName` 发送。 |
| `mobile.osVersion` | 与 Android 上一样，作为 `appium:platformVersion` 发送。 |
| `mobile.app` | 模拟器的 `.app` 文件夹，或它的 `.zip`。**`.ipa` 是设备构建，无法安装到模拟器上**：它会在任何请求之前被拒绝。请改为构建模拟器的 `.app`。 |
| `mobile.appSHA256` | 仅用于 `.zip`。`.app` 文件夹没有可以计算哈希的单个文件，所以与它一起设置会被拒绝。 |
| `mobile.appId` | bundle id。没有 `mobile.app` 时，会按 bundle id 启动已安装的应用，此时 `mobile.appId` 是必填的。 |
| `mobile.sessionTimeout` | 默认 300 s。 |

通过 SSH 时，`.app` 文件夹会在运行 venom 的机器上被打成 zip，并以 `app.zip` 的形式复制到 Mac；`.zip` 则原样复制。见「通过 SSH 连接 Mac」。

### iOS 上的定位器 {#locators-on-ios}

| Field | Finds the element by |
|---|---|
| `accessibilityId` | 它的无障碍 id |
| `text` | 它的标签：作为谓词 `label == "<text>"` 发送，文本中的引号、反斜杠和换行会被转义 |
| `name` | 它的名称 |
| `predicate` | NSPredicate 字符串，按原样发送 |
| `classChain` | iOS class chain，按原样发送 |
| `xpath` | 针对视图层级的 XPath |
| `css` | CSS 选择器，仅限 web view 内部 |

`id` 和 `uiautomator` 在 iOS 上会被拒绝；`name`、`predicate` 和 `classChain` 在 Android 上会被拒绝。与 Android 一样，一个步骤只给出一个定位器。

### 与 Android 的差异 {#differences-from-android}

| Action | On iOS |
|---|---|
| `tap`, `fill`, `clear`, `read`, `count`, `waitFor`, `screenshot`, `source`, `context` | 与 Android 上相同的命令。 |
| `tap` 和 `swipe`，按 `x`、`y` | 坐标点以 point 为单位，而不是像素。`tap` 使用 `mobile: tap`，`swipe` 使用 `mobile: dragFromToForDuration`，时长半秒。`unit: percent` 是相对于窗口矩形的，iOS 以 point 报告它。 |
| `swipe`、`scroll`，按 `direction` | 使用 `mobile: swipe` 和 `mobile: scroll`，作用于元素或整个应用。`percent` 会被拒绝：XCUITest 没有这样的参数。 |
| `scroll` 加 `until` | 重复 `mobile: scroll` 最多 10 次。服务器不会告知列表何时结束，所以循环在找到元素或达到上限时停止。 |
| `back` | 被拒绝：iOS 没有返回键。请点按应用自己的返回控件。 |
| `hideKeyboard` | `mobile: hideKeyboard`。没有显示的键盘不算失败。 |
| `launch`, `terminate` | `mobile: activateApp` 和 `mobile: terminateApp`，通过 `bundleId`（`appId` 或 `mobile.appId`）指定应用。 |
| `openURL` | `mobile: deepLink`，带 `url`，已知时还带 `bundleId`。 |
| `alert` | 仅限 iOS。在 Android 上会被拒绝。 |

### 警报 {#alerts}

iOS 应用可能会弹出系统警报，例如权限请求。`alert` 操作先读取它的文本，然后接受或关闭它：

```yaml
      - type: mobile
        action: tap
        accessibilityId: enable_location
      - type: mobile
        action: alert
        choice: accept
        assertions:
          - result.err ShouldBeEmpty
          - result.text ShouldContainSubstring Location
      - type: mobile
        action: alert
        choice: dismiss
        button: Don't Allow
```

- `choice` 是 `accept`（默认）或 `dismiss`。其他任何值都会在任何请求之前被拒绝。
- 没有 `button` 时，步骤用普通的接受或关闭命令来应答。有 `button` 时，它会按下带有该标签的按钮（`mobile: alert`）。
- `result.text` 保存警报文本。**警报可能显示机密**（验证码、名字）：该文本绝不会写进日志行或错误消息，并且 venom 报告的机密隐藏只覆盖已声明的机密。
- 没有打开的警报时，步骤会失败并给出 `no alert open`，它位于 `result.err` 中。
- 会话不能自己应答警报：`autoAcceptAlerts` 和 `autoDismissAlerts`（在 `mobile.capabilities` 中）会被拒绝，因为由 `alert` 步骤来处理它们。
- 模拟器上的深度链接可能会弹出系统确认警报（“Open in ...?”）。如果您的应用是以这种方式打开的，请在 `openURL` 之后跟一个 `alert` 步骤，否则下一个步骤会发现警报挡在路上。

## 尚未实现 {#not-yet}

计划中的、被拒绝的或目前缺失的：

- 真实的 iOS 设备：只有模拟器在范围内。
- iOS 模拟器上的 Flutter 只有 debug 构建：Flutter 不会为模拟器构建 release 应用。
- 在 Apple Silicon Mac 上，没有 arm64 切片的应用。
- 通过 SSH 的密码登录：只支持密钥认证。
- 替您下载 chromedriver：对于 Android 的 web view，由运维人员提供匹配的版本。
- 尚未在模拟器上运行过（见「iOS」；详细的运行记录不公开）：`.app` 文件夹的安装（只运行过 `.zip`）、`x`/`y` 和百分比坐标、`openURL`、`hideKeyboard`，以及警报的 `button` 标签。测试套件 `examples/handrun-ios.yml` 就是为 `.app` 文件夹的安装而写的。

## 示例 {#example}

下面的每个步骤都是该 executor 的测试所运行的形态。

```yaml
name: Mobile smoke
vars:
  mobile:
    url: http://127.0.0.1:4723
    platform: android
    device: emulator-5554
    osVersion: "14"
    app: ./build/app-debug.apk
    appSHA256: <sha-256 of the build, in hex>
    appId: com.example.app
    syncTimeout: 5

testcases:
  - name: the home screen is usable
    steps:
      - type: mobile
        action: waitFor
        accessibilityId: login_button
        syncTimeout: 20
      - type: mobile
        action: count
        id: com.example.app:id/item
        assertions:
          - result.count ShouldBeGreaterThan 0
      - type: mobile
        action: fill
        id: com.example.app:id/username
        value: demo-user
        clear: true
      - type: mobile
        action: hideKeyboard
      - type: mobile
        action: tap
        accessibilityId: login_button
      - type: mobile
        action: read
        id: com.example.app:id/title
        assertions:
          - result.err ShouldBeEmpty
          - result.text ShouldEqual Welcome
      - type: mobile
        action: scroll
        direction: down
        until: true
        text: Settings
      - type: mobile
        action: screenshot
        path: home.png
      - type: mobile
        action: back
      - type: mobile
        action: terminate
      - type: mobile
        action: launch
```

## 设置 Mac {#set-up-the-mac}

当 venom 运行在另一台机器（Windows、Linux）上，而模拟器运行在 Mac 上时，请使用本节。下面的一切都安装在 Mac 上。venom 不安装其中任何东西。

1. 装有两个驱动的 **Appium 3**：`appium driver install uiautomator2` 和 `appium driver install xcuitest`。启动它时让它只监听回环地址：`appium --address 127.0.0.1 --port 4723`。在启动它的 shell 中设置 `ANDROID_HOME`。
2. **Android：** Android SDK（platform-tools、`build-tools;34.0.0`），以及在 Apple Silicon Mac 上的 **arm64** 模拟器镜像。请在运行之前启动模拟器。
3. **iOS：** Xcode、它的命令行工具以及至少一个模拟器。启动它，或者让 Appium 来启动。第一个会话会构建 WebDriverAgent，需要几分钟。
4. **Android 的 web view：** 一个与模拟器上的 WebView 相匹配的 chromedriver，并把它的路径放在 `appium:chromedriverExecutable` 中（见「Web view 需要什么」）。iOS 的 web view 不需要它。
5. **SSH：** 一个 Mac 接受的、对应您在 `mobile.ssh.user` 中给出的用户的密钥。不支持密码登录。

在真实 Mac 上遇到的坑：

- **工具的第一次运行可能会在 SSH 上卡住。** macOS Gatekeeper 可能会请求一个在 SSH 会话上没人看得到的确认，于是二进制文件就在等待。请先在 Mac 本机的“终端”中运行一次 `flutter --version`（或相关的工具），并回答提示。用 Homebrew cask 安装的工具也可能带有隔离标记。是否用 `xattr -dr com.apple.quarantine <folder>` 去掉它由您决定：只对您信任的文件夹这样做。
- **应用需要 arm64 切片**（在 Apple Silicon Mac 上）（见「Mac 上的前提条件」）。
- **Flutter 只为 iOS 模拟器构建 debug 应用。** Flutter 的 release 构建只可能用于 Android。
- **CocoaPods 需要 UTF-8 区域设置**（`export LANG=en_US.UTF-8 LC_ALL=en_US.UTF-8`），例如当它在通过 SSH 启动的脚本中运行时。

## 示例应用 {#sample-apps}

维护者的运行所使用的公开应用。每一个都已支持 arm64。

| App | Platform | Where from |
|---|---|---|
| ApiDemos | Android | Appium 的 `ApiDemos-debug.apk`，来自 <https://github.com/appium/android-apidemos> 的发布版本 |
| UIKitCatalog v4.0.5 | iOS 模拟器 | `UIKitCatalog.zip`，来自 <https://github.com/appium/ios-uicatalog> 的发布版本 |
| Flutter 计数器应用 | 两者 | `flutter create`，由 `examples/build-sample-apps.sh` 构建（Android release APK、iOS 模拟器 debug 应用） |
| React Native 应用 | 两者 | `react-native init`，由同一个脚本构建（Android release APK、iOS Release 模拟器应用） |

该脚本在 Mac 上运行。如果缺少 Flutter SDK 和 CocoaPods，它会安装它们，用各框架自己的模板构建这四个应用，并在最后列出它们。请先手动运行一次 `flutter --version`（见「设置 Mac」）。

测试套件在 `examples/` 中：`examples/flutter.yml` 和 `examples/rn.yml` 不加修改就能在两个平台上运行，使用各平台的变量文件（`examples/android.vars.example.yml`、`examples/ios.vars.example.yml`：请填写 `<...>` 的值）；`examples/failing.yml` 故意失败，用来展示诊断文件。详细的运行记录不公开。

## 运行测试 {#run-the-tests}

封闭式测试（没有设备、没有网络；属于 `go test ./...` 的一部分）：

```bash
go test ./executors/mobile/ -count=1
```

真实测试，位于构建标签 `mobile` 之后。它需要在回环地址上带有 UiAutomator2 的 Appium 3 和一个模拟器，并从环境变量读取设置。有任何一个未设置时，它会带消息跳过。

```bash
export VENOM_MOBILE_URL=http://127.0.0.1:4723
export VENOM_MOBILE_DEVICE=emulator-5554
export VENOM_MOBILE_APP=/path/to/sample.apk        # or VENOM_MOBILE_APPID=<package id of an installed app>
export VENOM_MOBILE_LOCATOR_KIND=accessibilityId   # accessibilityId, text, id, xpath or uiautomator
export VENOM_MOBILE_LOCATOR_VALUE=<an element on the first screen>
export VENOM_MOBILE_EXPECT_TEXT=<its text>         # optional
export VENOM_MOBILE_OSVERSION=14                   # optional
go test -tags mobile ./executors/mobile/ -run Real -v -count=1
```

它会打开一个会话，查找并读取该元素，检查通过的步骤不会留下诊断文件，故意让一个步骤失败并检查 `screen.png` 和 `screen.dump.xml` 出现，然后关闭会话。

另外两个真实测试共用上面的会话变量（`VENOM_MOBILE_URL`、`_DEVICE`、`_APP` 或 `_APPID`）：

| Test | Extra variables | What it checks |
|---|---|---|
| `TestReal_CoordinatesAndDeepLink` | 无 | 百分比和像素的滑动，以及在 50 % / 50 % 处的点按都成功；`openURL venomtest://nothing/handles/this` 返回一个非空的 `result.err` |
| `TestReal_IOSSession` | `VENOM_MOBILE_PLATFORM=ios`，`VENOM_MOBILE_DEVICE` 为模拟器名称，`VENOM_MOBILE_APPID` 为 bundle id（或 `VENOM_MOBILE_APP`），`VENOM_MOBILE_IOS_LOCATOR`（第一个屏幕上的无障碍 id，“设置”应用默认为 `General`） | `waitFor`、向上滑动、`terminate`、`launch`，以及没有打开警报时 `alert` 回答 `no alert open`；除非 `VENOM_MOBILE_PLATFORM` 为 `ios`，否则跳过 |
| `TestReal_WebViewContext` | `VENOM_MOBILE_CHROMEDRIVER`（Appium 主机上匹配的 chromedriver 的路径）和 `VENOM_MOBILE_WEBVIEW_LOCATOR`（打开 web view 界面的无障碍 id） | `context` 到达一个 `WEBVIEW...`，`css: body` 统计出大于 0，`context: native` 返回 `NATIVE_APP`；除非两者都设置了，否则跳过 |
