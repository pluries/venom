---
translates: sql
title: SQL
description: Den SQL-executor in einer venom-Test-Suite verwenden.
fingerprint: 3f894c972db9710614e251686e0fcdd4e79bbb0320842bd24c31aaeebbfd4b9d
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# Venom - Executor SQL {#venom---executor-sql}

Schritt zum Ausführen von SQL-Abfragen in Datenbanken:
* **MySQL**
* **PostgreSQL**
* **Oracle**
* **Sqlite**

Im Hintergrund wird das Paket `sqlx` verwendet: https://github.com/jmoiron/sqlx, um Zeilen als Liste von map[string]interface{} abzurufen

## Eingabe {#input}

In Ihrer YAML-Datei deklarieren Sie Ihren Schritt so

```yaml
  - driver mandatory [mysql/postgres/oracle/sqlite]
  - dsn mandatory
  - commands optional
  - file optional
 ```

- `commands` ist eine Liste von SQL-Abfragen.
- Der Parameter `file` wird nur als Ersatz verwendet, wenn `commands` nicht verwendet wird.

Anwendungsbeispiel (_mysql_, _oracle_, _SQLServer_):

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

Beispiel mit einer Abfragedatei:

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

*Hinweis: Im obigen Beispiel werden die Ergebnisse jedes Befehls im Ergebnis-Array gespeichert*

## SQL-Treiber {#sql-drivers}

Dieser executor verwendet die folgenden SQL-Treiber:

- _MySQL_: https://github.com/go-sql-driver/mysql
- _PostgreSQL_: https://github.com/lib/pq
- _Oracle_: https://github.com/sijms/go-ora
- _Sqlite_: https://pkg.go.dev/modernc.org/sqlite
