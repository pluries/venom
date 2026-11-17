---
translates: couchbase
title: couchbase
description: 如何在 venom 测试套件中使用 couchbase executor。
fingerprint: e731f1a0163fb3e5b6411c0cd94c7b4bfebdc321ff26413506ce63f0145d372f
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# Venom - couchbase Executor {#venom---executor-couchbase}

用于通过 [github.com/couchbase/gocb/v2](https://pkg.go.dev/github.com/couchbase/gocb/v2) SDK 在 couchbase 数据库上执行操作的步骤。

更多信息请参阅[此处](https://docs.couchbase.com/go-sdk/current/hello-world/start-using-sdk.html)

## 连接 {#connection}

要使用 couchbase executor，您至少需要指定 couchbase 的 bucket（在没有认证且本地运行的情况下）。

最简的 couchbase 配置可以是下面这样：

```yaml
- type: couchbase
  dsn:  "couchbase://localhost"         # connection string to connect to couchbase.
  username: "{{ .couchbase_username }}" # can be omitted
  password: "{{ .couchbase_password }}" # can be omitted
  bucket: "{{ .couchbase_bucket }}"     # mandatory
```

完整的 couchbase 配置可以是下面这样

```yaml
- type: couchbase
  dsn:  "couchbase://localhost"         # will use couchbase://localhost if omitted
  username: "{{ .couchbase_username }}" # can be omitted
  password: "{{ .couchbase_password }}" # can be omitted
  bucket: "{{ .couchbase_bucket }}"     # mandatory
  scope:  "_default"                    # can be omitted, default scope will be used
  collection: "_default"                # can be omitted, default collection will be used
  transcoder: "json"                    # can be omitted, "legacy" will be used
  expiry: 600                           # in seconds, can be omitted (no expiration by default)
  wait_until_ready_timeout: 5           # in seconds, can be 0 (means no wait until bucket is ready)
  profile_wan_development: true         # if true, will use high timeouts to avoid latency issues. default false.
```

字段 `bucket`、`scope` 和 `collection` 用作默认值。每个操作都可以在局部定义要使用的不同 bucket、scope 或 collection。

对于 transcoder，您有以下选项：

- json：使用 [gocb.JSONTranscoder](https://pkg.go.dev/github.com/couchbase/gocb/v2#JSONTranscoder) （这是 gocb 的默认 transcoder）
- raw：使用 [gocb.RawBinaryTranscoder](https://pkg.go.dev/github.com/couchbase/gocb/v2#RawBinaryTranscoder)
- rawjson：使用 [gocb.RawJSONTranscoder](https://pkg.go.dev/github.com/couchbase/gocb/v2#RawJSONTranscoder)
- rawstring：使用 [gocb.RawStringTranscoder](https://pkg.go.dev/github.com/couchbase/gocb/v2#RawStringTranscoder)
- legacy：使用 [gocb.LegacyTranscoder](https://pkg.go.dev/github.com/couchbase/gocb/v2#LegacyTranscoder) （venom 的 couchbase executor 默认使用）

## 输入 {#input}

可执行的示例另见 [tests/couchbase.yml](../../tests/couchbase.yml)。

Couchbase 可用于存储 JSON 结构化数据，也可以存储二进制和字符串类型的数据。

示例：

```yaml
airline_01: # entries are specified as map id => content
  "id": 1
  "type": "airline"
  "name": "first airline"
  "country": "Latveria"
```

### 加载 fixture {#load-fixtures}

尚未实现。

### 获取文档 {#retrieve-documents}

Get 按 id 获取文档。如果找不到该 id，标志 `found` 将为 false。

使用标志 `with_expiry` 时，会返回指定条目的过期时间（如果没有过期时间则为零）。

使用标志 `expiry` 时，get 操作会变为 `get and touch`，选项 `with_expiry` 将被忽略。

```yaml
- type: couchbase
  dsn:  "{{ .couchbase_dsn }}"
  username: "{{ .couchbase_username }}"
  password: "{{ .couchbase_password }}"
  bucket: "travel-sample"
  actions:
    - type: get
      with_expiry: true
      ids: ["airline_10","airline_01"]
    assertions:
      - result.actions.actions0.airline_10.expiry ShouldBeZeroValue
      - result.actions.actions0.airline_10.data ShouldJSONEqual {"id":10,"type":"airline","name":"40-Mile Air","iata":"Q5","icao":"MLA","callsign":"MILE-AIR","country":"United States"}
      - result.actions.actions0.airline_01 ShouldNotBeNil
      - result.actions.actions0.airline_01.found ShouldBeFalse
```

### 插入文档 {#insert-documents}

Insert 按 id 创建文档。如果该 id 已存在，则不执行该操作，标志 `inserted` 将为 false。

可以通过字段 `expiry` 定义以秒为单位的过期时间，以覆盖_默认_的 `expiry`（如果有）

```yaml
- type: couchbase
  dsn:  "{{ .couchbase_dsn }}"
  username: "{{ .couchbase_username }}"
  password: "{{ .couchbase_password }}"
  bucket: "travel-sample"
  actions:
    - type: insert
      entries:
        airline_01:
          "id": 1
          "type": "airline"
          "name": "first airline"
          "country": "Latveria"
  assertions:
    - result.actions.actions0.airline_01.inserted ShouldBeTrue
```

### 替换文档 {#replace-documents}

Replace 按 id 更新文档。如果该 id 不存在，则不执行该操作，标志 `replaced` 将为 false。

可以通过布尔字段 `preserve_expiry` 保留原有的过期时间。
可以通过字段 `expiry` 定义以秒为单位的过期时间，以覆盖_默认_的 `expiry`（如果有）。

```yaml
- type: couchbase
  dsn:  "{{ .couchbase_dsn }}"
  username: "{{ .couchbase_username }}"
  password: "{{ .couchbase_password }}"
  bucket: "travel-sample"
  actions:
    - type: replace
      entries:
        airline_01:
          "id": 1
          "type": "airline"
          "name": "first airline"
          "country": "Latveria"
  assertions:
    - result.actions.actions0.airline_01.replaced ShouldBeTrue
```

### Upsert 文档 {#upsert-documents}

Upsert 按 id 插入或更新文档。它会把标志 `upserted` 设为 true。

可以通过布尔字段 `preserve_expiry` 保留原有的过期时间（如果有）。

```yaml
- type: couchbase
  dsn:  "{{ .couchbase_dsn }}"
  username: "{{ .couchbase_username }}"
  password: "{{ .couchbase_password }}"
  bucket: "travel-sample"
  actions:
    - type: upsert
      entries:
        airline_01:
          "id": 1
          "type": "airline"
          "name": "first airline"
          "country": "Latveria"
  assertions:
    - result.actions.actions0.airline_01.upserted ShouldBeTrue
```

### 删除文档 {#remove-documents}

Delete 按 id 删除。如果找到了该 id，它会把标志 `deleted` 设为 true。

```yaml
- type: couchbase
  dsn:  "{{ .couchbase_dsn }}"
  username: "{{ .couchbase_username }}"
  password: "{{ .couchbase_password }}"
  bucket: "travel-sample"
  actions:
    - type: delete
      ids: ["airline_01"]
  assertions:
    - result.actions.actions0.airline_01.deleted ShouldBeTrue
```

### Touch 文档 {#touch-documents}

Touch 更新指定 id 对象的过期时间。如果找到了该 id，它会把标志 `touch` 设为 true。

可以指定以秒为单位的 `expiry` 来使用。否则将使用全局的 `expiry`。

```yaml
- type: couchbase
  dsn:  "{{ .couchbase_dsn }}"
  username: "{{ .couchbase_username }}"
  password: "{{ .couchbase_password }}"
  bucket: "travel-sample"
  actions:
    - type: touch
      expiry: 3600    # 1 hour
      ids: ["airline_01"]
  assertions:
    - result.actions.actions0.airline_01.touched ShouldBeTrue
```

### 检查文档是否存在 {#check-if-document-exists}

命令 `exists` 检查文档 id 是否存在。它会用检查结果设置布尔标志 `found`。

```yaml
- type: couchbase
  dsn:  "{{ .couchbase_dsn }}"
  username: "{{ .couchbase_username }}"
  password: "{{ .couchbase_password }}"
  bucket: "travel-sample"
  actions:
    - type: exists
      ids: ["airline_10","airline_01"]
  assertions:
    - result.actions.actions0.airline_10.found ShouldBeTrue
    - result.actions.actions0.airline_01.found ShouldBeFalse
```
