---
translates: dbfixtures
title: Database Fixtures
description: 如何在 venom 测试套件中使用 Database Fixtures executor。
fingerprint: cd0aced2e2be5fb203dfe5bd8cbc08bc489220a9db58ed1b6a9a15db99717762
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# Venom - Database Fixtures Executor {#venom---executor-database-fixtures}

用于向 **MySQL** 和 **PostgreSQL** 数据库加载 fixture 的步骤。

它在底层使用 `testfixtures/v3` 包：https://github.com/go-testfixtures/testfixtures
关于该 executor 各参数的更多细节，尤其是 `folder` 和 `files`，以及 fixture 应如何编写，请阅读它的文档。

## 输入 {#input}

在您的 yaml 文件中，像这样声明步骤

```yaml
  - database mandatory [mysql/postgres/sqlite3 (or sqlite); SQLite is the pure Go driver, no cgo]
  - dsn mandatory
  - schemas optional
  - migrations optional
  - migrationsTable optional
  - skipResetSequences optional
  - files optional
  - folder optional
 ```

- `schemas` 是若干 `.sql` 文件的路径列表，这些文件包含数据库中各表的结构定义。如果指定，会在加载 fixture 之前执行每个文件的内容。
- `files` 参数仅在未使用 `folder` 时作为后备使用。
- `migrations` 是一个文件夹路径，其中的 SQL 迁移文件可用来初始化数据库，用来代替结构定义列表。**请注意，如果 `schemas` 不为空，它具有优先权，迁移文件将被忽略。**
- `migrationsTable` 是用于存放迁移版本的表。
- `skipResetSequences` 仅用于 PostgreSQL 数据库，它控制是否重置索引（默认重置）

用法示例（_mysql_）：
```yaml

name: Title of TestSuite
testcases:

  - name: Load database fixtures
    steps:
      - type: dbfixtures
        database: mysql
        dsn: user:password@(localhost:3306)/venom?multiStatements=true
        schemas:
          - schemas/mysql.sql
        folder: fixtures
        files:
          - fixtures/table.yml

```

*注意：在上面的示例中，如果要能够加载结构定义，查询参数 `multiStatements=true` 是必需的。*

## SQL 驱动 {#sql-drivers}

该 executor 使用以下 SQL 驱动：

- _MySQL_：https://github.com/go-sql-driver/mysql
- _PostgreSQL_：https://github.com/lib/pq
