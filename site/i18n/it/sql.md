---
translates: sql
title: SQL
description: Come usare l'executor SQL in una test suite di venom.
fingerprint: 3f894c972db9710614e251686e0fcdd4e79bbb0320842bd24c31aaeebbfd4b9d
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# Venom - Executor SQL {#venom---executor-sql}

Step per eseguire query SQL su database:
* **MySQL**
* **PostgreSQL**
* **Oracle**
* **Sqlite**

Usa internamente il package `sqlx`: https://github.com/jmoiron/sqlx per recuperare le righe come lista di map[string]interface{}

## Input {#input}

Nel tuo file YAML dichiari lo step così

```yaml
  - driver mandatory [mysql/postgres/oracle/sqlite]
  - dsn mandatory
  - commands optional
  - file optional
 ```

- `commands` è una lista di query SQL.
- Il parametro `file` viene usato solo come ripiego se `commands` non è usato.

Esempio di utilizzo (_mysql_, _oracle_, _SQLServer_):

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

Esempio con un file di query:

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

*nota: nell'esempio qui sopra, i risultati di ogni comando sono memorizzati nell'array dei risultati*

## Driver SQL {#sql-drivers}

Questo executor usa i seguenti driver SQL:

- _MySQL_: https://github.com/go-sql-driver/mysql
- _PostgreSQL_: https://github.com/lib/pq
- _Oracle_: https://github.com/sijms/go-ora
- _Sqlite_: https://pkg.go.dev/modernc.org/sqlite

