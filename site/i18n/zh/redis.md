---
translates: redis
title: Redis
description: 如何在 venom 测试套件中使用 Redis executor。
fingerprint: 3a8eed14b8743bc49ac23050af8244dc6ba4e18034d1258c07373943dcc6a7ba
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# Venom - Executor Redis {#venom---executor-redis}

用于在 Redis 上执行命令的步骤

使用场景：您的软件需要调用 Redis。

## 输入 {#input}

可用的输入如下：
- `commands`：Redis 命令数组
- `path`：包含一系列 Redis 命令的文件。如果填写了 path 属性，则会忽略 commands 属性。
- `dialURL`：Redis 服务器 URL

URL 应遵循该 scheme 的 IANA 规范草案 (https://www.iana.org/assignments/uri-schemes/prov/redis)。
如果有多个测试用例或步骤使用同一个 Redis URL，您可以只把 `dialURL` 设置定义一次，作为测试套件变量。

```
Commands file is read line by line and each command is split by [strings.Fields](https://golang.org/pkg/strings/#Fields) method

```text
SET Foo Bar
SET bar beez
SET Bar {"foo" : "bar", "poo" : ["lol", "lil", "greez"]}
Keys *
```

```yaml
name: Redis testsuite
vars: 
  redis.dialURL: "redis://localhost:6379/0"
  
testcases:
- name: test-commands
  steps:
  - type: redis
    commands:
        - FLUSHALL
  - type: redis
    commands:
        - SET foo bar
        - GET foo
        - KEYS *
    assertions:
        - result.commands.commands0.response ShouldEqual OK
        - result.commands.commands1.response ShouldEqual bar
        - result.commands.commands2.response.response0 ShouldEqual foo
  - type: redis
    commands:
        - KEYS *
    assertions:
        - result.commands.commands0.response.response0 ShouldEqual foo

- name: test-commands-from-file
  steps:
  - type: redis
    path: testredis/commands.txt
    dialURL: "redis://localhost:6379/0" # The global dialURL is overridden by this setting
    assertions:
        - result.commands.commands0.response ShouldEqual OK
        - result.commands.commands1.response ShouldEqual bar
        - result.commands.commands2.response.response0 ShouldEqual foo
```

## 输出 {#output}

该 executor 返回一个结果对象，其中包含已执行的 Redis 命令。

- result.commands 包含已执行的 Redis 命令列表
- result.commands.commandI.response 表示该 Redis 命令的响应。根据 Redis 命令的不同，它可以是数组，也可以是字符串

## 更多示例 {#examples}

更多示例可以在本仓库的 `tests` 文件夹中找到。
