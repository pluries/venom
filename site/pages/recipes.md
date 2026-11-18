A recipe is a venom test suite that drives a web page or a mobile app. This page shows how to lay recipes out, name them, prove they test something, and keep secrets and results out of the wrong places. You need venom and, for web recipes, a browser that Playwright installed. The [web executor](web.html) and the [mobile executor](mobile.html) pages list every action.

A recipe is an ordinary suite. It uses `web` steps or `mobile` steps and nothing new. Here is a small web recipe. The address of the page is the variable `appUrl`, so the same file runs against any copy of the page:

{{include site/examples/recipes/web-page.yml yaml}}

Run it with the address of your page:

```
venom run --var appUrl=http://localhost:8000 web-page.yml
```

## Folder layout

Keep the recipes of one feature in one folder under `tests/`, next to the other venom suites. For example:

```
tests/
  site/
    site-languages.yml
  recipes/
    reference/
      mobile/
        counter.yml
```

One folder per feature. One or more `.yml` files in it. A folder with subfolders is checked by naming the subfolders. Keep variable files with credentials outside the folder, or ignored by git (see Credentials).

## Testcase names and the spec id

Each testcase proves one acceptance scenario of a spec. Start the testcase name with the scenario id, then a short description: `US1-AS3 an item typed in the input is added to the list`. The id has the form `USn-ASm`: user story `n`, acceptance scenario `m`.

Name the spec once, in a variable of the suite:

```
vars:
  spec: 014-web-mobile-recipes
```

The value is the folder name of the spec. The recipe check reads `spec.md` in that folder and looks the ids up. The option `--specs-dir` tells it where the spec folders are. Two files of one folder must not claim the same scenario.

To see which scenarios a recipe proves, add `--coverage` with the spec id: `venom recipe check --specs-dir <specs folder> --coverage 014-web-mobile-recipes tests/recipes`. After the findings it lists every scenario of the spec, `US1-AS3 proven: <file> / <testcase>` or `US1-AS4 not proven`. A testcase proves a scenario only when it is not skipped and holds an assertion that reads the app. The option does not change the exit status. The coverage script of the dev set does not read `.yml` files, so use this option, not that script, to count recipes.

## Selectors

A `find`, `click` or `fill` step takes a selector. Use CSS or XPath. Prefer a stable test attribute, such as `data-test`:

```
find: '[data-test="add-button"]'
```

Ids and class names change when someone restyles the page. A `data-test` attribute exists only so that tests can find the element. If the app has none, ask the developers to add them before you write the recipe.

`find` does not wait. It counts the elements that exist now. When the page loads data after the first paint, wait first with `wait`, or use a `click` or `fill`, which wait for their element. In a mobile step, set exactly one locator, such as `accessibilityId`.

## Proof steps

A recipe that passes proves nothing until you have seen it fail. Three steps give the proof:

1. Replay. Run the recipe with `venom run` against the app. Every testcase must pass. Replay it a few times: a testcase that passes once and fails once is not usable.
2. Negative control. For each testcase, copy the recipe, change the first expected value that comes from the app, and run the copy. The run must exit with status 2 and name that testcase. Leave the original file untouched.
3. Recipe check. Run the check, which reads the files without a browser or a device:

```
venom recipe check --specs-dir <folder of the specs> tests/site
```

The check exits 0 when it finds nothing, 1 when it reports a finding and 2 when it cannot read its input. Each finding names the file, the line, the rule and the testcase. It reports a testcase without a scenario id, a repeated id, an id that the spec does not have, a testcase with no assertion on a value read from the app, an assertion that cannot fail, a credential-like text and a literal address.

The check is static. It cannot read inside an `execute` script and cannot know whether an expected value is right. The negative control covers both.

## What the executors cannot do

The web executor has no step for hover, drag or a keyboard shortcut, and `find` cannot read a number such as a scroll width. The `execute` action runs a script in the page and returns a value, which you can assert on. This example reads how far the page scrolls sideways and how many items it lists:

{{include site/examples/recipes/web-execute.yml yaml}}

If `execute` is not enough, write down the gap and do not write a recipe for that scenario. Do not put all assertions of a testcase on one script result without a negative control: the check cannot read the script.

A mobile recipe needs an Appium server and a device or an emulator. It does not run without them. This one is tagged `# needs: device` on its first line, so the tests of the site skip it where no device exists:

{{include site/examples/recipes/mobile-counter.yml yaml}}

## Credentials

A recipe holds no password, token or key. A login comes from variables. Pass them on the command line or in a variable file that git ignores:

```
venom run --var-from-file login.vars.yml tests/site/login.yml
```

In the recipe, write the variable: `text: "{{.password}}"`. The check reports a credential-like text in any step or default value.

Recipes run against a test environment, never against production. The address is a variable that the pipeline sets, and `venom run` does not stop you from pointing it at a real system. Keep the list of allowed hosts in your pipeline.

## Results and artifacts

A failing step can write a page dump, a screenshot or a trace. These files can hold data from the app. Write them to a results folder that git ignores, and do not commit them. In CI, keep them for a short time, such as a few days, and give access only to the people who need them: the storage is restricted. Do not copy them into long-lived storage.

## Undefined variables

A variable you forget to pass does not stop the run by default. Venom leaves `{{.password}}` as the literal text `{{.password}}` and the step types that text. The run does not fail because of it.

Pass `--strict-variables` to change that. Venom then stops with exit status 2 and the message `unknown variable(s) "password"`. Use the option in every pipeline that runs recipes:

```
venom run --strict-variables --var-from-file login.vars.yml tests/site/login.yml
```
