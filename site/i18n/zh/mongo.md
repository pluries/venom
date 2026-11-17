---
translates: mongo
title: Mongo
description: 如何在 venom 测试套件中使用 Mongo executor。
fingerprint: 06f23fc462b5feaf748ec60eb8983d082305404d10385ef51907b2975b7b62fa
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# Venom - Mongo Executor {#venom---executor-mongo}

用于在 MongoDB 数据库上执行操作的步骤。

**服务器版本：** MongoDB 4.4 或更高版本。该 executor 使用官方 Go 驱动 v2，它会拒绝更旧的服务器（v1 驱动接受 3.6 及更高版本）。Venom 自己的测试套件针对 MongoDB 8.2 运行。

## 输入 {#input}

可执行的示例另见 [tests/mongo.yml](../../tests/mongo.yml)。

注意：大多数字段支持 [MongoDB Extended JSON v2](https://www.mongodb.com/docs/manual/reference/mongodb-extended-json/)。
这意味着某些特殊值（ObjectId、ISODate 等）可以使用宽松格式来表示。

示例：

```json
{
  "_id": {
    "$oid": "5d505646cf6d4fe581014ab2"
  }
}
```

### 加载 fixture {#load-fixtures}

```yaml
- type: mongo
  uri: mongodb://localhost:27017
  database: my-database
  actions:
    - type: loadFixtures
      folder: fixtures/
```

该操作会先**删除数据库中的所有集合**，然后从一个文件夹中一次加载多个集合。
fixture 文件夹中每个集合必须对应一个文件，并以该集合命名。例如，`cards.yml` 会创建一个 `cards` 集合。
集合中的条目以 YAML 数组的形式声明。例如：

```yaml
# fixtures/cards.yml
- suit: clubs
  value: jack

- suit: clubs
  value: queen

- suit: clubs
  value: king
```

### 插入文档 {#insert-documents}

```yaml
- type: mongo
  uri: mongodb://localhost:27017
  database: my-database
  collection: my-collection
  actions:
    - type: insert
      documents:
        - '{"suit": "hearts", "value": "queen"}'
        - '{"suit": "diamonds", "value": "three"}'
```

### 从文件插入文档 {#insert-documents-from-a-file}

```yaml
- type: mongo
  uri: mongodb://localhost:27017
  database: my-database
  collection: my-collection
  actions:
    - type: insert
      file: fixtures/my-collection.jsonlist
```

`fixtures/my-collection.jsonlist`:

```json
{
  "suit": "hearts",
  "value": "queen"
}
{
  "suit": "diamonds",
  "value": "three"
}
```

### 查找文档 {#find-documents}

```yaml
- type: mongo
  uri: mongodb://localhost:27017
  database: my-database
  collection: my-collection
  actions:
    - type: find
      filter: |
        {
          "suit": "clubs"
        }
      options: # optional
        limit: 3
        skip: 1
        sort: '{"_id": -1}'
        projection: '{"value": 1}'
```

### 按 ObjectID 查找文档 {#find-documents-by-objectid}

参见：[MongoDB Extended JSON - Type Representations](https://www.mongodb.com/docs/manual/reference/mongodb-extended-json/#type-representations)

```yaml
- type: mongo
  uri: mongodb://localhost:27017
  database: my-database
  collection: my-collection
  actions:
    - type: find
      filter: |
        {
          "_id": {
            "$oid": "5d505646cf6d4fe581014ab2"
          }
        }
      options: # optional
        limit: 3
        skip: 1
        sort: '{"_id": -1}'
        projection: '{"value": 1}'
```

### 统计文档数量 {#count-documents}

```yaml
- type: mongo
  uri: mongodb://localhost:27017
  database: my-database
  collection: my-collection
  actions:
    - type: count
      filter: |
        {
          "suit": "clubs"
        }
```

### 更新文档 {#update-documents}

```yaml
- type: mongo
  uri: mongodb://localhost:27017
  database: my-database
  collection: my-collection
  actions:
    - type: update
      filter: |
        {
          "suit": {
            "$in": ["clubs", "spades"]
          }
        }
      update: |
        {
          "$set": {
            "color": "black"
          }
        }
```

### 删除文档 {#delete-documents}

```yaml
- type: mongo
  uri: mongodb://localhost:27017
  database: my-database
  collection: my-collection
  actions:
    - type: delete
      filter: |
        {
          "suit": "circles"
        }
```

### 聚合文档 {#aggregate-documents}

```yaml
- type: mongo
  uri: mongodb://localhost:27017
  database: my-database
  collection: my-collection
  actions:
    - type: aggregate
      pipeline:
        - |
          { "$match": {
            "color": "black"
          }}
        - |
          { "$group": {
            "_id": "$value",
            "count": {"$sum": 1}
          }}
```

### 创建集合 {#create-a-collection}

```yaml
- type: mongo
  uri: mongodb://localhost:27017
  database: my-database
  collection: my-collection
  actions:
    - type: createCollection
```

### 删除集合 {#drop-a-collection}

```yaml
- type: mongo
  uri: mongodb://localhost:27017
  database: my-database
  collection: my-collection
  actions:
    - type: dropCollection
```
