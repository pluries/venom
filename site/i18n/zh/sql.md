---
translates: sql
title: SQL
description: 如何在 venom 测试套件中使用 SQL executor。
fingerprint: 3f894c972db9710614e251686e0fcdd4e79bbb0320842bd24c31aaeebbfd4b9d
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# Venom - Executor SQL {#venom---executor-sql}

在数据库中执行 SQL 查询的步骤：
* **MySQL**
* **PostgreSQL**
* **Oracle**
* **Sqlite**

它在底层使用 `sqlx` 包：https://github.com/jmoiron/sqlx ，将行检索为 map[string]interface{} 的列表

## 输入 {#input}

在您的 yaml 文件中，这样声明步骤

```yaml
  - driver mandatory [mysql/postgres/oracle/sqlite]
  - dsn mandatory
  - commands optional
  - file optional
 ```

- `commands` 是 SQL 查询的列表。
- `file` 参数仅在未使用 `commands` 时作为后备使用。

用法示例（_mysql_、_oracle_、_SQLServer_）：

```yaml
name: Title of TestSuite
testcases:

  - name: Query database
    steps:
      - type: sql
        driver: mysql
        dsn: user:password@(localhost:3306)/venom
        commands:
          - "SELECT * FROM employee;"
          - "SELECT * FROM person;"
        assertions:
          - result.queries.__Len__ ShouldEqual 2
          - result.queries.queries0.rows.rows0.name ShouldEqual Jack
          - result.queries.queries1.rows.rows0.age ShouldEqual 21

  - name: Oracle
    steps:
      - type: sql
        driver: oracle
        dsn: "oracle://system:oracle@localhost:49161/XE"
        commands:
          - select * from v$version
```

使用查询文件的示例：

```yaml
name: Title of TestSuite
testcases:

  - name: Query database
    steps:
      - type: sql
        database: mysql
        dsn: user:password@(localhost:3306)/venom
        file: ./test.sql
        assertions:
          - result.queries ShouldHaveLength 1
```

*注意：在上面的示例中，每条命令的结果都存储在 results 数组中*

## SQL 驱动 {#sql-drivers}

该 executor 使用以下 SQL 驱动：

- _MySQL_：https://github.com/go-sql-driver/mysql
- _PostgreSQL_：https://github.com/lib/pq
- _Oracle_：https://github.com/sijms/go-ora
- _Sqlite_：https://pkg.go.dev/modernc.org/sqlite
