---
translates: sql
title: SQL
description: Cómo usar el executor SQL en una suite de pruebas de venom.
fingerprint: 3f894c972db9710614e251686e0fcdd4e79bbb0320842bd24c31aaeebbfd4b9d
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# Venom - Executor SQL {#venom---executor-sql}

Paso para ejecutar consultas SQL en bases de datos:
* **MySQL**
* **PostgreSQL**
* **Oracle**
* **Sqlite**

Usa internamente el paquete `sqlx`: https://github.com/jmoiron/sqlx para obtener las filas como una lista de map[string]interface{}

## Entrada {#input}

En su archivo YAML, declara el paso así

```yaml
  - driver mandatory [mysql/postgres/oracle/sqlite]
  - dsn mandatory
  - commands optional
  - file optional
 ```

- `commands` es una lista de consultas SQL.
- El parámetro `file` solo se usa como alternativa si no se usa `commands`.

Ejemplo de uso (_mysql_, _oracle_, _SQLServer_):

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

Ejemplo con un archivo de consultas:

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

*nota: en el ejemplo anterior, los resultados de cada comando se guardan en el array de resultados*

## Drivers SQL {#sql-drivers}

Este executor usa los siguientes drivers SQL:

- _MySQL_: https://github.com/go-sql-driver/mysql
- _PostgreSQL_: https://github.com/lib/pq
- _Oracle_: https://github.com/sijms/go-ora
- _Sqlite_: https://pkg.go.dev/modernc.org/sqlite
