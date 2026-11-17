---
translates: imap
title: IMAP
description: 如何在 venom 测试套件中使用 IMAP executor。
fingerprint: c71cd52180b824cdd8978dd27d199675ccde776d54bf93ad3fe5e475d251783f
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# Venom - Executor IMAP {#venom---executor-imap}

使用场景：您有一台邮件服务器，想测试 IMAP 命令？

Venom 的 IMAP executor 实现了一部分 IMAP 命令，例如 FETCH、APPEND、MOVE、STORE 等。

## 如何测试 {#how-to-test}
### 身份验证 {#authentication}

```yaml
auth:
  withtls: false
  verifytls: false # Optional, used with withtls: true (see below)
  host: yourimaphost
  port: 143 # Most probably 993 if using TLS
  user: imap@venom.com
  password: imapvenompassword
  tls_root_ca: ca.pem # Optional, used with withtls: true and verifytls: true
  tls_client_cert: client.pem # Optional, used with withtls: true, together with tls_client_key
  tls_client_key: client.key # Optional, used with withtls: true, together with tls_client_cert
```

| 参数 | 说明 | 默认值 |
|---|---|---|
| auth.withtls | 可选。从第一个字节起就通过 TLS 连接（通常是 993 端口） | false |
| auth.verifytls | 可选，与 `withtls: true` 搭配使用。`false`：不检查服务器证书，因此自签名的测试服务器也会被接受。`true`：证书必须能链接到某个系统根证书或 `tls_root_ca`，并且对 `host` 有效，否则该步骤失败 | false |
| auth.tls_root_ca | 可选，与 `withtls: true` 搭配使用。在系统根证书之外额外信任的 CA 证书，这样 `verifytls: true` 就能接受由私有 CA 签发的服务器。可以把多个 CA 拼接在一起。只有在 `verifytls: true` 时才用于验证 | |
| auth.tls_client_cert | 可选，与 `withtls: true` 搭配使用。客户端证书（双向 TLS），叶子证书在前，后面是中间证书。即使 `verifytls` 为 `false` 也会发送。它需要 `tls_client_key`：只有证书而没有密钥，该步骤会失败 | |
| auth.tls_client_key | 可选，与 `withtls: true` 搭配使用。`tls_client_cert` 的私钥。它需要 `tls_client_cert`：只有密钥而没有证书，该步骤会失败 | |

`tls_root_ca`、`tls_client_cert` 和 `tls_client_key` 可以填写 PEM 内容本身，也可以填写 PEM 文件的路径，相对路径从测试套件所在的目录算起。
如果没有 `withtls: true`，它们不会被使用：连接仍是明文，并会记录一条警告（只有证书而没有密钥，或者反过来，仍会使该步骤失败）。

可以用环境变量 `SSL_CERT_FILE` 或 `SSL_CERT_DIR` 让 venom 使用另一组系统根证书。

### 命令 {#commands}

目前可用的命令如下：
- **append**：在邮箱末尾创建一封新邮件
- **create**：创建一个新邮箱
- **clear**：从邮件服务器上删除邮箱中的所有邮件
- **delete**：删除一封邮件
- **fetch**：获取一封邮件
- **flag**：添加、移除或设置邮件的标志
- **move**：把邮件从一个邮箱移动到另一个邮箱

可以一次写出多个命令：它们会按照在 `commands` 字段下出现的顺序依次执行。

每个命令都有一个 `name` 字段，用来指定要执行的命令。
此外，命令由 `search` 字段和/或 `args` 字段组成。

`search` 字段的语法始终相同。它根据搜索条件获取并返回找到的第一封邮件：
```yaml
search:
  mailbox: testBox # The mailbox in which to search the mail
  uid: 1 # The UID to search for
  from: .*@your-domain.localhost # The "From" header field to search for
  to: you@company.tld # The "To" header field to search for
  subject: Title of mail with * # The "Subject" header field to search for
  body: .*a body content.* # The "Body" field to search for
```
它用来指定命令要作用于哪一封邮件。
> ⚠️ 请注意，搜索条件是正则表达式。如果要搜索特殊字符，别忘了对它们转义： \
> ❌ `subject: [IMPORTANT] READ THIS` \
> ✅ `subject: \[IMPORTANT\] READ THIS`

`args` 字段常与 search 字段一起使用，用来指定命令的参数。
它的语法取决于具体的命令。

在本节中，我们会逐一说明每个命令的语法。

#### Append 命令 {#append-command}

append 命令在邮箱末尾创建一封新邮件。

```yaml
commands:
  - name: append
    args:
      mailbox: mailboxName # MANDATORY: The mailbox in which to create the mail
      from: origin-of@-my-new-mail.com # MANDATORY: the "From" header field of the new mail
      to: recipient-of@my-new-mail.com # OPTIONAL: the "To" header field of the new mail
      subject: Subject of my new mail # OPTIONAL: the "Subject" header field of the new mail
      body: Body of my new mail # OPTIONAL: the body of the new mail
      flags: # OPTIONAL: the flags of the new mail (note that the "\Recent" flag will most probably be added as well)
        - Flag1
        - Flag2
        ...
```

#### Create 命令 {#create-command}

create 命令创建一个新邮箱。

```yaml
commands:
  - name: create
    args:
      mailbox: mailboxName # MANDATORY: The name of the new mailbox to create
```

#### Clear 命令 {#clear-command}

clear 命令删除邮箱中的所有邮件。它有两种调用方式：
- 第一种，列出要清空的每个邮箱：
```yaml
commands:
  - name: clear
    args:
      mailboxes:
        - Mailbox1
        - Mailbox2
        ...
```
- 第二种，使用通配符参数来清空所有现有邮箱：
```yaml
commands:
  - name: clear
    args:
      mailboxes:
        - "*"
```

#### Delete 命令 {#delete-command}

delete 命令永久删除通过 search 字段找到的一封邮件。

```yaml
commands:
  - name: delete
    search:
      [...]
```

#### Fetch 命令 {#fetch-command}

fetch 命令通过 search 字段获取一封邮件。

```yaml
commands:
  - name: fetch
    search:
      [...]
```

#### Flag 命令 {#flag-command}

flag 命令修改邮件的标志。它同时包含 `search` 和 `args` 字段。
`args` 字段最多可以包含 3 个不同的字段：
- **add**：要添加到邮件上的标志
- **remove**：要从邮件上移除的标志
- **set**：要设置的邮件标志（覆盖所有已有标志）

这些字段可以同时出现，但如果指定了 `set`，则只有它会生效。
它们的语法都相同。
```yaml
commands:
  - name: flag
    args:
      add:
        - Flag1
        - Flag2
        ...
      remove:
        - Flag1
        - Flag2
        ...
      set:
        - Flag1
        - Flag2
        ...
```

#### Move 命令 {#move-command}

move 命令把邮件从一个邮箱移动到另一个邮箱。它同时包含 `search` 和 `args` 字段。


```yaml
commands:
  - name: move
    search:
      [...]
    args:
      mailbox: mailboxname # The mailbox to move the mail to
```

### 断言 {#assertions}

每次执行命令都会产生一个 `result`。其内容取决于命令及其执行情况。\
由于一个测试用例中可以执行多个命令，`result` 对象遵循如下语法：
```json
{
  "result": {
    "commands": [
      {
        // Search represents the result of the command's search field
        "search": {
          "mailbox": "INBOX",
          "from": "from@mail-before-command-execution.com",
          "to": "to@mail-before-command-execution.com",
          "subject": "Title of mail BEFORE command execution",
          "body": "Body content of mail BEFORE command execution",
          "flags": [
            "Flag1",
            "Flag2"
          ]
        },
        // Mail represents the state of the searched mail after the command was executed
        "mail": { 
          "from": "from@mail-after-command-execution.com",
          "to": "to@mail-after-command-execution.com",
          "subject": "Title of mail AFTER command execution",
          "body": "Body content of mail AFTER command execution",
          "flags": [
            "Flag1",
            "Flag2",
            "Flag3"
          ]
        },
        "err": "Error of the command", // Err represents the command error message, supposing the command failed
        "timeseconds": 1.5 // TimeSeconds represents the duration of the command execution
      }
    ]
  }
}
```
每个命令都有各自的结果，可以对其进行断言。

例如，下面是在同一个测试用例中执行两个命令之后，可能用到的一组断言：
```yaml
assertions:
  # First command
  - result.commands.commands0.err ShouldBeEmpty
  # State of the mail before command execution (search)
  - result.commands.commands0.search.from ShouldEqual "from@mail-before-command-execution.com"
  # Mail as a result of command execution
  - result.commands.commands0.mail.from ShouldEqual "to@mail-after-command-execution.com"
  # Second command
  - result.commands.commands1.err ShouldBeEmpty
  - result.commands.commands1.search.from ShouldEqual ...
  - result.commands.commands1.mail.from ShouldEqual ...
  - result.commands.commands1.mail.flags.flags0 ShouldEqual ...
  - result.commands.commands1.mail.flags.flags1 ShouldEqual ...
```

不修改邮件内容（头部和正文）的命令，不会填充 `mail` 字段。\
例如，下面是 `fetch`、`delete` 或 `move` 命令的典型结果：
```json
{
  "result": {
    "commands": [
      {
        "search": {
          "mailbox": "INBOX",
          "from": "from@mail-before-command-execution.com",
          "to": "to@mail.com",
          "subject": "Title of retrieved mail",
          "body": "Body content of retrieved mail",
          "flags": [
            "Flag1",
            "Flag2"
          ]
        },
        "err": "",
        "timeseconds": 1.5
      }
    ]
  }
}
```

再举一个例子，下面是 `create` 或 `clear` 命令的典型结果：
```json
{
  "result": {
    "commands": [
      {
        "search": {},
        "err": "",
        "timeseconds": 1.5
      }
    ]
  }
}
```

## 测试套件示例 {#testsuite-example}

```yml
name: IMAP testsuite
vars:
  withTLS: false
  host: localhost
  port: 1143
  user: address@example.org
  password: pass
testcases:
  - name: Clear a mailbox
    steps:
      - type: imap
        auth:
          host: "{{.host}}"
          port: "{{.port}}"
          user: "{{.user}}"
          password: "{{.password}}"
        commands:
          - name: clear
            args:
              mailboxes:
                - INBOX
        assertions:
          # As multiple commands can be executed in a single testcase, we need to specify which command result we want to assert
          - result.commands.commands0.err ShouldBeEmpty
```

要查看更多示例，请参阅 [IMAP executor 测试文件](../../tests/imap.yml)。
