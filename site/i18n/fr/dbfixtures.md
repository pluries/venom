---
translates: dbfixtures
title: Database Fixtures
description: Comment utiliser l'executor Database Fixtures dans une suite de tests venom.
fingerprint: cd0aced2e2be5fb203dfe5bd8cbc08bc489220a9db58ed1b6a9a15db99717762
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# Venom - Executor Database Fixtures {#venom---executor-database-fixtures}

Étape pour charger des fixtures dans des bases **MySQL** et **PostgreSQL**.

Elle s'appuie en interne sur le paquet `testfixtures/v3` : https://github.com/go-testfixtures/testfixtures
Lisez sa documentation pour les détails sur les paramètres de cet executor, en particulier `folder` et `files`, et sur la façon d'écrire les fixtures.

## Input {#input}

Dans votre fichier yaml, vous déclarez votre étape ainsi

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

- `schemas` est une liste de chemins vers plusieurs fichiers `.sql` qui contiennent les schémas des tables de votre base. S'il est renseigné, le contenu de chaque fichier est exécuté avant le chargement des fixtures.
- Le paramètre `files` ne sert que de solution de repli si `folder` n'est pas utilisé.
- `migrations` est le chemin d'un dossier qui contient des fichiers de migration SQL pouvant servir à initialiser la base, à la place d'une liste de schémas. **Notez que si `schemas` n'est pas vide, il est prioritaire et les fichiers de migration sont ignorés.**
- `migrationsTable` est la table utilisée pour stocker la version de migration.
- `skipResetSequences` ne sert que pour les bases PostgreSQL : il indique si l'index doit être réinitialisé (par défaut) ou non.

Exemple d'utilisation (_mysql_) :
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

*Remarque : dans l'exemple ci-dessus, le paramètre de requête `multiStatements=true` est obligatoire si vous voulez pouvoir charger le schéma.*

## SQL drivers {#sql-drivers}

Cet executor utilise les drivers SQL suivants :

- _MySQL_ : https://github.com/go-sql-driver/mysql
- _PostgreSQL_ : https://github.com/lib/pq
