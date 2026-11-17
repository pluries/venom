---
translates: dbfixtures
title: Database Fixtures
description: Come usare l'executor Database Fixtures in una test suite di venom.
fingerprint: cd0aced2e2be5fb203dfe5bd8cbc08bc489220a9db58ed1b6a9a15db99717762
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# Venom - Executor Database Fixtures {#venom---executor-database-fixtures}

Step per caricare le fixture nei database **MySQL** e **PostgreSQL**.

Usa internamente il pacchetto `testfixtures/v3`: https://github.com/go-testfixtures/testfixtures
Leggi la sua documentazione per i dettagli sui parametri di questo executor, in particolare `folder` e `files`, e su come scrivere le fixture.

## Input {#input}

Nel tuo file YAML dichiari lo step così

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

- `schemas` è un elenco di percorsi di più file `.sql` che contengono gli schemi delle tabelle del tuo database. Se è indicato, il contenuto di ogni file viene eseguito prima di caricare le fixture.
- Il parametro `files` è usato solo come ripiego se `folder` non è usato.
- `migrations` è il percorso di una cartella che contiene file di migrazione SQL, utilizzabili per inizializzare il database al posto di un elenco di schemi. **Nota: se `schemas` non è vuoto, ha la precedenza e i file di migrazione vengono ignorati.**
- `migrationsTable` è la tabella usata per memorizzare la versione della migrazione.
- `skipResetSequences` è usato solo con i database PostgreSQL e controlla se l'indice va reimpostato (predefinito) o no

Esempio d'uso (_mysql_):
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

*nota: nell'esempio sopra, il parametro di query `multiStatements=true` è obbligatorio se vuoi poter caricare lo schema.*

## Driver SQL {#sql-drivers}

Questo executor usa i seguenti driver SQL:

- _MySQL_: https://github.com/go-sql-driver/mysql
- _PostgreSQL_: https://github.com/lib/pq
