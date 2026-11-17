---
translates: dbfixtures
title: Database Fixtures
description: Cómo usar el executor Database Fixtures en una suite de pruebas de venom.
fingerprint: cd0aced2e2be5fb203dfe5bd8cbc08bc489220a9db58ed1b6a9a15db99717762
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# Venom - Executor Database Fixtures {#venom---executor-database-fixtures}

Paso para cargar fixtures en bases de datos **MySQL** y **PostgreSQL**.

Utiliza internamente el paquete `testfixtures/v3`: https://github.com/go-testfixtures/testfixtures
Consulte su documentación para obtener más detalles sobre los parámetros de este executor, especialmente `folder` y `files`, y sobre cómo debe escribir las fixtures.

## Input {#input}

En su archivo YAML, declare el paso de esta forma

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

- `schemas` es una lista de rutas a varios archivos `.sql` que contienen los esquemas de las tablas de su base de datos. Si se indica, el contenido de cada archivo se ejecutará antes de cargar las fixtures.
- El parámetro `files` solo se usa como alternativa si no se usa `folder`.
- `migrations` es la ruta de una carpeta que contiene archivos de migración SQL que pueden usarse para inicializar la base de datos, en lugar de una lista de esquemas. **Tenga en cuenta que si `schemas` no está vacío, tendrá prioridad y los archivos de migración se ignorarán.**
- `migrationsTable` es la tabla que se usa para almacenar la versión de la migración.
- `skipResetSequences` solo se usa con bases de datos PostgreSQL; controla si el índice debe reiniciarse (valor por defecto) o no

Ejemplo de uso (_mysql_):
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

*nota: en el ejemplo anterior, el parámetro de consulta `multiStatements=true` es obligatorio si queremos poder cargar el esquema.*

## SQL drivers {#sql-drivers}

Este executor utiliza los siguientes drivers SQL:

- _MySQL_: https://github.com/go-sql-driver/mysql
- _PostgreSQL_: https://github.com/lib/pq
