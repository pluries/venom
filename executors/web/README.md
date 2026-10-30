# Venom - Executor Web

Navigate within a web application

Use case: You have a web application and you want to check some behaviors.

The web executor allows you to navigate into your web application and execute actions.

## HTTPS certificates

By default the web executor ignores HTTPS certificate errors (the browser context is created with `IgnoreHttpsErrors`).
A web step passes against a site with an expired, self-signed or wrong-host certificate, so do not rely on it to check
certificates. To make the browser reject an invalid certificate, set the `verifyTls` variable to `true`; the step then
fails. The web executor cannot trust a private CA: Playwright has no such option for a browser context.

## Browsers

The web executor drives a browser through [Playwright](https://playwright.dev) (Go binding
`github.com/mxschmitt/playwright-go`). Playwright manages its own browser builds: no `chromedriver` or `geckodriver`
to install, and no version to pair with the browser.

Install the Playwright driver and the browser(s) your suites use once per machine, with their operating-system
packages (`--with-deps` needs `sudo` on Linux):

```bash
go run github.com/mxschmitt/playwright-go/cmd/playwright@v0.6201.1 install --with-deps chromium   # or firefox, webkit
```

venom never downloads a browser on its own: a suite that needs one that is not installed fails at setup, and the
error message gives this command.

### Arguments
* Chromium: [list of arguments](https://peter.sh/experiments/chromium-command-line-switches/). An argument without
  a leading dash gets `--`, as chromedriver did.
* Firefox: [list of arguments](https://wiki.mozilla.org/Firefox/CommandLineOptions),
  [list of preferences](https://searchfox.org/mozilla-central/source/modules/libpref/init/all.js)

## Variables

You can define parameters to configure the browser used during the test suite. All parameters are optional:
* driver (default: `chrome`): browser to use:
  * `chrome`: Chromium
  * `gecko`: Firefox (Playwright's own build)
  * `webkit`: WebKit, the engine behind Safari
* width, height: size of the page. Without them the page keeps the browser window's own size
* args: browser arguments
* prefs: Firefox preferences (`gecko` only)
* timeout (default: `180`): maximum time, in seconds, for a page to load (`navigate`, `historyAction`, a popup answer),
  and for a navigation a step starts to finish
* proxy: proxy to use to query web site (format: `[server]:[port]`)
* verifyTls (default: `false`): make the browser reject pages with an invalid HTTPS certificate, so the step fails. Without it
  certificate errors are ignored, as they always were
* headless (default: `false`): run the browser without a window. Useful in a CI/CD pipeline
* binaryPath: browser executable to use instead of Playwright's (`chrome` only)
* trace (default: `false`): save a Playwright trace of each failing step in `page.trace.zip`. See
  [Failure diagnostics](#failure-diagnostics)

Chromium runs with its sandbox on, as it did under chromedriver. A suite that needs it off (in a container running as
root, for example) passes `--no-sandbox` in `args`. A host that restricts unprivileged user namespaces, as native
Ubuntu 23.10 and later do, cannot start the sandbox of Playwright's own Chromium: Setup fails and names the fixes —
`--no-sandbox` in `args`, `binaryPath` pointing at a system Chrome, or allowing the namespaces
(`sysctl kernel.apparmor_restrict_unprivileged_userns=0`).

HTTPS certificate errors are ignored by default, as they were with the WebDriver session this executor used before
(`acceptInsecureCerts`). Set `verifyTls` to check them (see HTTPS certificates above).

Still accepted, **ignored with a warning** — they have no Playwright equivalent:
* driverPath, driverPort: there is no driver process any more
* detach: Playwright cannot leave the browser running after the suite
* debug, logLevel: they set the old WebDriver client's log level; use venom's own `-v` instead
* prefs with `chrome`, binaryPath with `gecko`: Playwright drives its own Firefox build

## Action
Actions allow you to manipulate the browser

An action on an element (`click`, `fill`, `select`, `uploadFile`, `selectFrame`) waits for the element to be ready,
up to its `syncTimeout` or 30 seconds. `find` never waits: it counts the elements present now, so
`result.find ShouldEqual 0` stays fast. After an action that can load a page (`click`, `fill`, `select`,
`uploadFile`, `execute`, a popup answer), the step waits for the page to finish loading before it reads
`result.title` and `result.url`. That includes a navigation the page starts on its own, from a script
(`location.href = …`) or from a timer with no delay. A navigation that does not finish within `timeout` fails the step,
and the following steps too, until a `navigate` replaces it. No `page.dump.html` is written while the page navigates.

A call to the page that never returns fails its step: after 60 seconds for a read (the title, the source, a count, a
text; plus the step's `syncTimeout` on an element it waits for), after 30 seconds for `execute`.
The browser call itself keeps running, so the next step waits for it before it sends its own call. A call with a limit
of its own (the reads and `execute`) waits at most that limit and then fails the same way, the wait going on for the
step after it. An action with no limit of its own (a click, a select, a screenshot) waits at most 60 seconds, logs a
warning and goes on. `navigate`, `historyAction` and a timer flush that a navigation ended never wait, and a dialog that
opened meanwhile is dismissed first, as usual. A failing step writes no `page.dump.html` only while that call still
runs: it logs a warning instead.

List of output values available:
* result.url: URL of the current page
* result.timeseconds: duration of the action execution
* result.title: title of the current page

### Navigate
Navigate to a specific URL

#### Input
* URL: Url to navigate
* Reset: Reset browser state

#### Example
```yaml
  - type: web
    action:
      navigate:
        url: https://www.google.fr
```

### Find
Search an element from a selector

[Link for CSS selector tutorial](https://developer.mozilla.org/fr/docs/Web/CSS/CSS_Selectors)
[Link for XPATH selector tutorial](https://developer.mozilla.org/fr/docs/Web/XPath)

#### Input
* Selector: Expression to use to identify web element
* Locator: Locator to use to search element (CSS or XPATH)

#### Output
* result.find: returns the number of objects identified

#### Example
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

### Click
Click on an element from a selector

#### Input
* Find: Element to find (More information in find section)
* Wait: Time to wait after click (in seconds)
* SyncTimeout: maximum time, in seconds, to wait for the element to be ready (present, visible, enabled, not moving, not covered). Default: `30`

#### Example
```yaml
  - type: web
    action:
      click:
        find:
          selector: button
          locator: CSS
```

### Fill
Fill an element (input or textarea) with a text. The text is typed after the field's current content, as WebDriver
did; it does not replace it

#### Input
Array of structured input:
* Find: Element to find (More information in find section)
* Text: Update element value with text
* Key: Update element value with key
* SyncTimeout: maximum time, in seconds, to wait for the element to be ready (present, visible, enabled, not moving, not covered). Default: `30`

#### Example
```yaml
  - type: web
    action:
      fill:
      - find:
          selector: input
          locator: CSS
        text: userName
```

### Select
Select an option of a web element

#### Input
* Find: Element to find (More information in find section)
* Text: Option to select
* Wait: Time to wait after click (in seconds)
* SyncTimeout: maximum time, in seconds, to wait for the element to be ready (present, visible, enabled, not moving, not covered). Default: `30`

#### Example
```yaml
  - type: web
    action:
      select:
        find:
          selector: select
          locator: CSS
        text: Option 1
```

### Upload file
Upload file on a web element

#### Input
* Find: Element to find (More information in find section)
* Files: String array to define files to upload
* Wait: Time to wait after click (in seconds)
* SyncTimeout: maximum time, in seconds, to wait for the element to be ready (present, visible, enabled, not moving, not covered). Default: `30`

#### Example
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

### Wait
Wait time
An integer value to define number of seconds to wait

#### Example
```yaml
  - type: web
    action:
      wait: 1
```

### Select frame
Select a frame presents in the current page

#### Input
* Find: Frame to find (More information in find section)
* SyncTimeout: maximum time, in seconds, to wait for the element to be ready (present, visible, enabled, not moving, not covered). Default: `30`

#### Example
```yaml
  - type: web
    action:
      selectFrame:
        find:
          selector: "#iframeResult"
          locator: CSS
```

### Select root frame
Select root frame presents in the current page
A boolean value to set to true to select root frame

#### Example
```yaml
  - type: web
    action:
      selectRootFrame: true
```

### Next window
Select the next window (tab) of the browser, wrapping from the last to the first; with a single window the step
stays on it
A boolean value to set to true to select next window

#### Example
```yaml
  - type: web
    action:
      nextWindow: true
```

### Back
Back to previous page
A boolean value to set to true to back to previous page

#### Example
```yaml
  - type: web
    action:
      historyAction: back
    assertions:
    - result.title ShouldStartWith GitHub
    - result.url ShouldEqual https://github.com/
```

### Forward
Forward to next page
A boolean value to set to true to forward page

#### Example
```yaml
  - type: web
    action:
      historyAction: forward
    assertions:
    - result.title ShouldStartWith GitHub
    - result.url ShouldEqual https://github.com/team
```

### Refresh
Refresh page
A boolean value to set to true to refresh page

#### Example
```yaml
  - type: web
    action:
      historyAction: refresh
    assertions:
    - result.title ShouldStartWith GitHub
    - result.url ShouldEqual https://github.com/team
```

### Confirm popup
Confirm popup dialog (confirm dialog box and alert dialog box)

A dialog opened by a step stays open for the next step to answer with `confirmPopup` or `cancelPopup`; while it is
open, `result.title` and `result.url` are empty. Any other action while a dialog is open dismisses it and fails
with `unexpected alert open`, as WebDriver did.
A boolean value to set to true to confirm popup dialog

#### Example
```yaml
  - type: web
    action:
      confirmPopup: true
```

### Cancel popup
Cancel popup dialog
A boolean value to set to true to cancel popup dialog

#### Example
```yaml
  - type: web
    action:
      cancelPopup: true
```

### Execute javascript
Execute javascript code. The command is the body of a function: `return` and `arguments[0]`, `arguments[1]`… work
as they did with WebDriver

#### Input
* Command: Javascript code to execute
* Args: String array to define javascript arguments

#### Output
What the script returns, if anything. When it returns a promise, the step waits for its value.
* result.value: the value as text. A string stays as it is. Any other value is written as JSON: `42`, `true`,
  `[1,"a"]`, `{"user":{"name":"Ada"}}`, with the keys of an object sorted
* result.valuejson: the same value, typed, as `result.bodyjson` is for the http executor. Use it to compare numbers or
  to reach inside an object or a list: `result.valuejson.user.name`

Every web step's result has a `result.valuejson` variable. It is empty unless the step executes a script that returns
a value.

A script that returns nothing, `undefined` or `null` leaves both empty. A few values have no JSON form:
* `NaN`, `Infinity` and `-Infinity` read as those words
* a `Date` reads as its UTC time in RFC 3339 (`1970-01-01T00:00:00Z`)
* a BigInt, a URL and a RegExp read as their text
* a DOM element, `document` or `window` reads as `ref: <Node>`, `ref: <Document>` or `ref: <Window>`

A value that refers to itself, such as an object with a property pointing back at the object, leaves both fields empty.
So does a URL that the browser accepts and venom cannot read, such as one with a `%` not followed by two hex digits
(`https://a.test/x#50%off`). venom logs a warning and the step passes.

#### Example
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


More information about actions (https://github.com/ovh/venom/tree/master/executors/web/types.go)
For an action, you can take screenshot of browser with following command: `screenshot: [fileName].png`

## Mock network calls
A step can answer the page's network requests itself, so a suite can test a front end without its backend. Add a
`mock:` list to the step, next to `action:`. Each entry answers every request whose URL matches its `url`, and the
request never reaches the network. The host does not need to exist: a mock can serve the page itself, and then the
page's own calls.

#### Input
Array of structured input:
* Url: pattern of the URLs to answer (required). `*` matches any characters except `/`, `**` matches any characters,
  `{a,b}` matches `a` or `b`. The pattern matches the whole URL: `**/api/users` answers
  `https://app.test/api/users`, not `https://app.test/api/users?page=2`; `**/api/users*` answers both. Every `{` needs
  its `}`, or the step fails: write `\{` for a literal brace (`\\{` inside double quotes)
* Status: HTTP status of the response. Default: `200`
* Headers: response headers. Write each value as a string: `X-Total: "3"`, not `X-Total: 3`
* ContentType: the `Content-Type` of the response. Default: none
* Body: the response body, as text

A mock starts answering before the step's action runs, and keeps answering in the following steps of the same test
case, in every tab. When several mocks match a URL, the most recent one answers. The next test case starts with no
mock. A step can hold `mock:` and no action.

Setting `mock:` while a popup is open dismisses it and fails the step, as any other action does. Set the mock on a
step before the one that opens the popup.

A request that the page's service worker answers itself never reaches the mock.

#### Example
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

## Failure diagnostics
When a step fails, the web executor writes `page.dump.html` in the working directory: the source of the current frame.
It skips the dump while the page is still loading a new document. A step with `screenshot:` writes its screenshot,
and the dump, once its action has succeeded.

With the variable `trace: true`, a failing step also writes `page.trace.zip` next to `page.dump.html`: a Playwright
trace of that step, with each action, a snapshot of the page before and after it, screenshots, and the network calls.
Open it with Playwright's trace viewer:

```bash
go run github.com/mxschmitt/playwright-go/cmd/playwright@v0.6201.1 show-trace page.trace.zip
```

The viewer needs the Playwright driver; the command downloads it if it is missing, as the install command does.
The next failing step replaces both files. A passing step writes no trace.

The trace holds everything the step saw and typed, **passwords included**, and the headers and bodies of its network
calls. venom's masking of secrets does not apply to it. venom writes it readable by its owner only, as
`page.dump.html`. Do not publish it as a build artefact or commit it. Tracing
also slows each step down, by about 20 ms on a simple page: turn it on while you investigate a failure.

## Example
A global example

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
