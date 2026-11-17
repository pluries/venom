---
translates: ssh
title: SSH
description: 如何在 venom 测试套件中使用 SSH executor。
fingerprint: 44c8a00c687f093c007b2632c39692ac92400c71c058513ec8972df353563c5d
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# Venom - Executor SSH {#venom---executor-ssh}

通过 SSH 在远程服务器上执行脚本的步骤


## 输入 {#input}

在您的 yaml 文件中，可以使用：

```yaml
  - host mandatory
  - command mandatory
  - user optional (default is OS username)
  - password optional (mandatory if no privatekey is found)
  - privatekey optional (default is $HOME/.ssh/id_rsa)
  - sudo optional
  - sudopassword optional (default to password)
  - knownhosts optional (path of an OpenSSH known_hosts file the host's key must be listed in; exclusive with hostkey)
  - hostkey optional (the one public key the host must present, in authorized_keys format; exclusive with knownhosts)
```

**主机密钥检查。** 默认情况下，`knownhosts` 和 `hostkey` 都没有设置，此时**不会检查**主机的密钥：
任何服务器都会被接受。这是这两个键出现之前该 executor 的行为。它适合密钥每次运行都会变化的临时测试
主机。设置这两个键中的一个，即可检查主机：

- `knownhosts`：一个采用 `~/.ssh/known_hosts` 格式的文件，例如 `ssh-keyscan -p 2222 192.0.2.10` 的输出。相对
  路径从测试套件所在的目录读取；绝对路径则原样使用。如果主机不在该文件中，或者出示了另一个密钥，该步骤会失败。
  该文件还可以用类似 `@cert-authority [192.0.2.10]:2222 ssh-ed25519 AAAAC3Nza…` 的一行来信任某个证书颁发机构：
  如果主机出示了由该机构签发的主机证书，就能通过检查，前提是证书的 principals（如果列出了的话）包含 `host` 中所写的主机
  （不含端口）。
- `hostkey`：形如 `ssh-ed25519 AAAAC3Nza… [comment]` 的一行，例如主机的
  `/etc/ssh/ssh_host_ed25519_key.pub` 的内容。如果主机出示了另一个密钥，该步骤会失败。

该 executor 会向主机请求您所列类型的密钥；当文件为该主机列出了证书颁发机构时，则请求主机证书。因此，
拥有多种密钥类型（RSA、ECDSA、Ed25519）的主机，只要文件或
`hostkey` 中包含其中任意一个密钥，就能通过检查。如果同时设置了两个键、文件无法读取，或者 `hostkey` 不是公钥，
该步骤会在连接之前失败。`knownhosts` 或 `hostkey` 被设为空值时也一样，例如某个展开后为空的变量：
空值是错误，与不设置该键不同，所以它无法关闭检查。与连接失败一样，原因会出现在 `result.err` 中。

示例

```yaml

name: Title of TestSuite
testcases:
- name: Check if exit code != 1 and echo command response in less than 1s
  steps:
  - type: ssh
    host: localhost:2222
    command: echo 'foo'
    assertions:
    - result.code ShouldEqual 0
    - result.timeseconds ShouldBeLessThan 1

- name: Use specific privatekey
  steps:
  - type: ssh
    host: 192.0.2.10:2222
    command: echo 'foo'
    user: bar
    privatekey: /home/foo/.ssh/id_rsa
    assertions:
    - result.code ShouldEqual 0

- name: Execute command as another user than bar
  steps:
  - type: ssh
    host: 192.0.2.10:2222
    command: echo 'foo'
    user: bar
    sudo: root
    sudopassword: '{{.mypassword}}'
    assertions:
    - result.code ShouldEqual 0

- name: Check the host's key against a known_hosts file next to the suite
  steps:
  - type: ssh
    host: 192.0.2.10:2222
    command: echo 'foo'
    user: bar
    knownhosts: ssh/known_hosts
    assertions:
    - result.code ShouldEqual 0

- name: Pin the host's key
  steps:
  - type: ssh
    host: 192.0.2.10:2222
    command: echo 'foo'
    user: bar
    hostkey: '{{.hostkey}}'
    assertions:
    - result.code ShouldEqual 0

```
*注意：sudo 选项使用伪终端*

## 输出 {#output}

```yaml
systemout
systemerr
err
code
timeseconds
```

- result.timeseconds：执行耗时
- result.err：如果存在，该字段包含错误
- result.systemout：所执行脚本的标准输出
- result.systemerr：所执行脚本的错误输出
- result.code：退出码

## 默认断言 {#default-assertion}

```yaml
result.code ShouldEqual 0
```
