---
translates: cli-usage
title: CLI 用法
description: venom 的命令及其选项。
fingerprint: 02ee0f4bbd022eb88342dc24560b249d66c272730ab0c8285af92241332fa21a
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# CLI 用法 {#cli-usage}

`venom` CLI 由多个命令组成：

```bash
$ venom -h
Venom - RUN Integration Tests

Usage:
  venom [command]

Available Commands:
  help        Help about any command
  run         Run Tests
  version     Display Version of venom: venom version

Flags:
  -h, --help   help for venom

Use "venom [command] --help" for more information about a command.
```

您可以用 `venom [command] -h` 查看某个命令的帮助：

```bash
$ venom run -h

run integration tests

Usage:
  venom run [flags]

Examples:
  Run all testsuites containing in files ending with *.yml or *.yaml: venom run
  Run a single testsuite: venom run mytestfile.yml
  Run a single testsuite and export the result in JSON format in test/ folder: venom run mytestfile.yml --format=json --output-dir=test
  Run a single testsuite and export the result in XML and HTML formats in test/ folder: venom run mytestfile.yml --format=xml --output-dir=test --html-report
  Run a single testsuite and specify a variable: venom run mytestfile.yml --var="foo=bar"
  Run a single testsuite and load all variables from a file: venom run mytestfile.yml --var-from-file variables.yaml
  Run all testsuites containing in files ending with *.yml or *.yaml with verbosity: VENOM_VERBOSE=2 venom run

  Notice that variables initialized with -var-from-file argument can be overrided with -var argument

  More info: https://github.com/ovh/venom

Flags:
      --format string           --format:json, tap, xml, yaml (default "xml")
  -h, --help                    help for run
      --html-report             Generate HTML Report
      --lib-dir string          Lib Directory: can contain user executors. example:/etc/venom/lib:$HOME/venom.d/lib
      --output-dir string       Output Directory: create tests results file inside this directory
      --parallel int            Number of test files to run at the same time. 1 (default) runs them one after another (default 1)
      --stop-on-failure         Stop running Test Suite on first Test Case failure
      --strict-variables        Fail a step that names an unknown variable
      --var stringArray         --var cds='cds -f config.json' --var cds2='cds -f config.json'
      --var-from-file strings   --var-from-file filename.yaml --var-from-file filename2.yaml: yaml, must contains a dictionary
  -v, --verbose count           verbose. -v (INFO level in venom.log file), -vv to very verbose (DEBUG level) and -vvv to very verbose with CPU Profiling
```

## 按指定顺序运行测试套件 {#run-test-suites-in-a-specific-order}

- `venom run 01_foo.yml 02_foo.yml` 会先运行 01，再运行 02。
- `venom run 02_foo.yml 01_foo.yml` 会先运行 02，再运行 01。

如果要对多个测试套件文件排序，可以使用标准命令，例如：

```bash
venom run `find . -type f -name "*.yml"|sort`
```

## 同时运行多个测试文件 {#run-several-test-files-at-the-same-time}

默认情况下，`venom` 一次只运行一个测试文件。`--parallel N` 最多同时运行 `N` 个：

```bash
$ venom run --parallel 4
```

集成测试的大部分时间都花在等待其他系统上，而这个选项分摊的正是这部分等待。在八个都需要等待的文件上实测：**`--parallel 4` 时快 3.4 倍，`--parallel 8` 时快 5.6 倍**。

并行的单位是**文件**：文件内的测试用例，以及测试用例内的测试步骤，仍然依次运行，因为一个测试步骤会把它的值传给下一个步骤。因此，对单个文件使用 `--parallel 4` 不会有任何变化。

**能获得多少收益，取决于您的文件是否真的在等待。** 每次运行大约有 0.35 秒是固定开销（启动、解析、写入报告），无论使用多少个 worker 都无法消除。在 venom 自己的 `tests/` 测试套件上，这些套件很短，大多不需要等待，`--parallel 4` 只快了约 1.2 倍。当您的文件需要等待某个服务时，可以期待明显的收益；如果不需要等待，收益就很小。

有两点需要了解：

- **控制台的每一行都带有它所属文件的标记**，这样同时运行的多个文件的输出仍然可以分辨：

  ```
  [exec.yml]   • exec testsuite (tests/exec.yml)
  [vars.yml]   • vars testsuite (tests/vars.yml)
  [exec.yml]   	• stdin PASS
  [vars.yml]   	• set PASS
  ```

  来自不同文件的行确实会交错出现，它们不会被分组，但每一行都会标明是谁写的，而且一行绝不会被拆分到两个文件中。您可以用 `grep` 跟踪单个文件：`venom run --parallel 4 | grep '^\[exec.yml'`。使用 `--parallel 1` 时完全没有标记，输出与以往完全一致。无论哪种情况，日志文件都不受影响：其中的每一行本来就已经带有它所属测试文件的标记。
- **`--stop-on-failure` 会停止整个运行。** 一个测试文件失败后，不会再启动新的文件；已经在运行的文件会继续运行至结束，因此每个已启动的文件仍然会报告结果。从未启动的文件会被报告为已跳过。在早期版本中，这个选项只会跳过失败文件中剩余的测试用例，运行会继续处理下一个文件。

`-vvv` 不能与 `--parallel 2` 或更大的值同时使用：CPU 性能分析作用于整个进程，因此无法为同时运行的每个文件各生成一份分析结果。运行会在启动时停止并说明原因。

## 支持 Globstar {#globstar-support}

`venom` CLI 支持 globstar：

```
$ venom run ./foo/b*/**/z*.yml
```

## 变量 {#variables}

要在命令行上指定单个变量，请使用 `--var` 选项，并在运行 `venom run` 命令时带上它：

```bash
$ venom run --var="foo=bar"
$ venom run --var='foo_list=["biz","buz"]'
$ venom run --var='foo={"biz":"bar","biz":"barr"}'
```

`--var` 选项可以在一条命令中多次使用。

### 变量定义文件 {#variable-definitions-files}

如果要设置大量变量，更方便的做法是把它们的值写在变量定义文件中。该文件是一个 YAML 字典。您需要在命令行上用 `--var-from-file` 指定该文件：

```bash
venom run --var-from-file variables.yaml
```

### 环境变量 {#environment-variables}

作为其他定义变量方式的后备，`venom` 工具会在自身进程的环境中查找名为 `VENOM_VAR_` 后接已声明变量名的环境变量。

```bash
$ export VENOM_VAR_foo=bar
$ venom run *.yml
```

您也可以在一行中定义环境变量并运行测试套件：

```bash
$ VENOM_VAR_foo=bar venom run *.yml
```

## 参数 {#arguments}

您可以在命令行上使用选项名称来定义参数。

这些选项列在 help 命令的结果中。

`venom run` 命令的可用选项列表：

```
Flags:
      --format string           --format:json, tap, xml, yaml (default "xml")
  -h, --help                    help for run
      --html-report             Generate HTML Report
      --lib-dir string          Lib Directory: can contain user executors. example:/etc/venom/lib:$HOME/venom.d/lib
      --output-dir string       Output Directory: create tests results file inside this directory
      --parallel int            Number of test files to run at the same time. 1 (default) runs them one after another (default 1)
      --stop-on-failure         Stop running Test Suite on first Test Case failure
      --strict-variables        Fail a step that names an unknown variable
      --var stringArray         --var cds='cds -f config.json' --var cds2='cds -f config.json'
      --var-from-file strings   --var-from-file filename.yaml --var-from-file filename2.yaml: yaml, must contains a dictionary
  -v, --verbose count           verbose. -vv to very verbose and -vvv to very verbose with CPU Profiling
```

### 用环境变量定义参数 {#define-arguments-with-environment-variables}

您也可以用环境变量来定义参数：

```bash
# is the same as
VENOM_FORMAT=json venom run my-test-suite.yml

# is equivalent to
venom run my-test-suite.yml --format=json
```

选项与其对应的环境变量用法：

- `--format="json"` 选项等价于 `VENOM_FORMAT="json"` 环境变量
- `--lib-dir="/etc/venom/lib:$HOME/venom.d/lib"` 选项等价于 `VENOM_LIB_DIR="/etc/venom/lib"` 环境变量
- `--output-dir="test-results"` 选项等价于 `VENOM_OUTPUT_DIR="test-results"` 环境变量
- `--stop-on-failure` 选项等价于 `VENOM_STOP_ON_FAILURE=true` 环境变量
- `--strict-variables` 选项等价于 `VENOM_STRICT_VARIABLES=true` 环境变量
- `--var foo=bar` 选项等价于 `VENOM_VAR_foo='bar'` 环境变量
- `--var-from-file fileA.yml fileB.yml` 选项等价于 `VENOM_VAR_FROM_FILE="fileA.yml fileB.yml"` 环境变量
- `--parallel 4` 选项等价于 `VENOM_PARALLEL=4` 环境变量
- `-v` 选项等价于 `VENOM_VERBOSE=1` 环境变量
- `-vv` 选项等价于 `VENOM_VERBOSE=2` 环境变量

可以设置 `NO_COLOR=1` 环境变量来关闭输出中的颜色。

## 使用配置文件 {#use-a-configuration-file}

您可以用配置文件 `.venomrc` 来定义 Venom 的设置。该配置文件应放在当前目录或 `home` 目录中。

```yml
variables: 
  - foo=bar
variables_files:
  - my_var_file.yaml
stop_on_failure: true
strict_variables: true
format: xml
output_dir: output
lib_dir: lib
verbosity: 3
parallel: 4
```

请注意，命令行选项会覆盖配置文件，而配置文件会覆盖环境变量。

## 信任模型 {#trust-model}

Venom 没有沙箱。测试套件就是代码，它以运行 venom 的用户的权限运行。

- 测试套件、`lib/` 中的用户自定义 executor 以及 Go 插件（`lib/*.so`）都以您的权限运行。只运行那些您愿意当作脚本来运行的测试套件。
- 切勿在不加引号的情况下，把测试步骤的结果或任何您无法控制的值放进 `script:` 或 SQL 文本中。测试步骤的结果可能包含任何内容，而 venom 不会为 shell 或 SQL 对其进行转义。
- 在 CI 中，不要运行来自未经审查的 fork 或拉取请求的测试套件，也不要让这类运行获得密钥。
- 在 `secrets` 或 `.venomrc` 中声明的密钥，会在报告、日志和控制台中被隐藏。只有已声明的值才会被隐藏。
- `ssh`、`imap` 和 `smtp` 这几个 executor 默认不检查服务器的身份。出现这种情况时，venom 每次运行会记录一次警告。要启用检查，请设置 `knownhosts` 或 `hostkey`（用于 `ssh` 步骤），以及 `verifytls: true`（位于 `auth:` 之下，对 `imap` 而言）和 `withtls: true`（二者用于 `imap` 或 `smtp` 步骤）。
- `web` executor 默认忽略 HTTPS 证书错误。把 `verifyTls` 变量设置为 `true`，可以让浏览器拒绝无效的证书。参见 [executors/web](executors/web/README.md)。
