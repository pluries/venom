# Web and mobile recipes

A recipe is an ordinary venom suite whose steps drive a web page (`type: web`) or a mobile app (`type: mobile`). Use this
part only when the project has a web or mobile feature to test. The Recipes page of the documentation, at
`https://venom.pluries.dev/recipes.html`, shows how to lay recipes out and name them; read it before you write one.

**Version.** Web recipes need the locators and actions of the web executor and the `venom recipe check` command. They exist
only in a venom release newer than the one that carried the first version of this skill. Before you write a recipe, run
`venom recipe check --help`. If the command does not exist in the pinned version, say which version has it, and do not write a
recipe that the pinned venom cannot run.

## Web

1. The web executor drives Playwright's browser and never downloads one. Check that a Go toolchain is present:

   ```bash
   go version
   ```

   - Present: show the developer this command and let Claude Code ask for approval. It installs the browser once per machine:

     ```bash
     go run github.com/mxschmitt/playwright-go/cmd/playwright@v0.6201.1 install --with-deps chromium
     ```

   - Not present: stop, and install nothing. Tell the developer what to install: Go, or a Chrome executable whose path is given in the
     variable `web.binaryPath` (it applies to `chrome` only). Do not install Go for them and do not go on.
2. Write the recipe against the running app. The address of the page is a variable, never a literal.
3. Locate elements by role, text, label, placeholder or test id. If the app has neither a stable test attribute nor an
   accessible name on an element, say which elements need one and stop for those; do not fall back to fragile selectors.
4. Check every recipe you write, with approval of the command:

   ```bash
   venom recipe check tests/recipes
   ```

   Add `--specs-dir <folder>` when the project keeps its specs in a folder. Report each finding. Exit code 0 means no finding,
   1 means findings, 2 means the input could not be used.
5. A recipe is not proven until the check is clean and its negative control fails as expected: change one expected value, show
   the recipe failing on it, restore it. Do not say a recipe is proven before that.

## Mobile

The mobile executor needs things the project provides; this skill never installs or starts them:

- an Appium 3 server with the UiAutomator2 (Android) or XCUITest (iOS) driver, listening on the loopback address of its host;
- an emulator, a simulator or a device that Appium can see;
- a host the developer names, reached over SSH when it is not the local machine.

1. Ask for the host. Check the connection with a command the developer approves, and report it as passing or failing, with what
   is missing (the host is unreachable, the Appium server does not answer, no device is visible).
2. Change nothing on that host. Do not install Appium, an emulator or a driver, and do not start one.
3. Write the recipe and check it as for web.
