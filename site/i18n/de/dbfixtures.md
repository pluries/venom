---
translates: dbfixtures
title: Database Fixtures
description: So verwenden Sie den Database-Fixtures-executor in einer venom-Test-Suite.
fingerprint: cd0aced2e2be5fb203dfe5bd8cbc08bc489220a9db58ed1b6a9a15db99717762
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# Venom - Executor Database Fixtures {#venom---executor-database-fixtures}

Schritt zum Laden von Fixtures in **MySQL**- und **PostgreSQL**-Datenbanken.

Er nutzt intern das Paket `testfixtures/v3`: https://github.com/go-testfixtures/testfixtures
Lesen Sie dessen Dokumentation für weitere Details zu den Parametern dieses executors, vor allem zu `folder` und `files`, und dazu, wie Sie die Fixtures schreiben sollten.

## Input {#input}

In Ihrer YAML-Datei deklarieren Sie den Schritt so

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

- `schemas` ist eine Liste von Pfaden zu mehreren `.sql`-Dateien, die die Schemas der Tabellen Ihrer Datenbank enthalten. Wenn angegeben, wird der Inhalt jeder Datei vor dem Laden der Fixtures ausgeführt.
- Der Parameter `files` wird nur als Ersatz verwendet, wenn `folder` nicht verwendet wird.
- `migrations` ist ein Ordnerpfad mit SQL-Migrationsdateien, mit denen sich die Datenbank statt über eine Liste von Schemas initialisieren lässt. **Beachten Sie: Wenn `schemas` nicht leer ist, hat es Vorrang, und die Migrationsdateien werden ignoriert.**
- `migrationsTable` ist die Tabelle, in der die Migrationsversion gespeichert wird.
- `skipResetSequences` wird nur für PostgreSQL-Datenbanken verwendet und steuert, ob der Index zurückgesetzt wird (Standard) oder nicht

Beispiel (_mysql_):
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

*Hinweis: Im obigen Beispiel ist der Query-Parameter `multiStatements=true` zwingend erforderlich, wenn das Schema geladen werden soll.*

## SQL-Treiber {#sql-drivers}

Dieser executor verwendet die folgenden SQL-Treiber:

- _MySQL_: https://github.com/go-sql-driver/mysql
- _PostgreSQL_: https://github.com/lib/pq
