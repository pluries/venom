---
translates: redis
title: Redis
description: Comment utiliser l'executor Redis dans une testsuite venom.
fingerprint: 3a8eed14b8743bc49ac23050af8244dc6ba4e18034d1258c07373943dcc6a7ba
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# Venom - Executor Redis {#venom---executor-redis}

Étape qui exécute des commandes sur Redis

Cas d'usage : votre logiciel doit faire des appels à Redis.

## Entrée {#input}

Les entrées suivantes sont disponibles :
- `commands` : un tableau de commandes Redis
- `path` : un fichier qui contient une série de commandes Redis. Si la propriété path est renseignée, la propriété commands est ignorée.
- `dialURL` : l'URL du serveur Redis

L'URL doit suivre la spécification provisoire de l'IANA pour ce schéma (https://www.iana.org/assignments/uri-schemes/prov/redis).
Si plusieurs testcases ou étapes utilisent la même URL Redis, vous pouvez définir le réglage `dialURL` une seule fois comme variable de testsuite.

```
Commands file is read line by line and each command is split by [strings.Fields](https://golang.org/pkg/strings/#Fields) method

```text
SET Foo Bar
SET bar beez
SET Bar {"foo" : "bar", "poo" : ["lol", "lil", "greez"]}
Keys *
```

```yaml
name: Redis testsuite
vars: 
  redis.dialURL: "redis://localhost:6379/0"
  
testcases:
- name: test-commands
  steps:
  - type: redis
    commands:
        - FLUSHALL
  - type: redis
    commands:
        - SET foo bar
        - GET foo
        - KEYS *
    assertions:
        - result.commands.commands0.response ShouldEqual OK
        - result.commands.commands1.response ShouldEqual bar
        - result.commands.commands2.response.response0 ShouldEqual foo
  - type: redis
    commands:
        - KEYS *
    assertions:
        - result.commands.commands0.response.response0 ShouldEqual foo

- name: test-commands-from-file
  steps:
  - type: redis
    path: testredis/commands.txt
    dialURL: "redis://localhost:6379/0" # The global dialURL is overridden by this setting
    assertions:
        - result.commands.commands0.response ShouldEqual OK
        - result.commands.commands1.response ShouldEqual bar
        - result.commands.commands2.response.response0 ShouldEqual foo
```

## Sortie {#output}

L'executor renvoie un objet résultat qui contient la commande Redis exécutée.

- result.commands contient la liste des commandes Redis exécutées
- result.commands.commandI.response représente la réponse de la commande Redis. Ce peut être un tableau ou une chaîne, selon la commande Redis

## Exemples {#examples}

Vous trouverez d'autres exemples dans le dossier `tests` de ce dépôt.
