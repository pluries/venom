---
translates: redis
title: Redis
description: Come usare l'executor Redis in una test suite di venom.
fingerprint: 3a8eed14b8743bc49ac23050af8244dc6ba4e18034d1258c07373943dcc6a7ba
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# Venom - Executor Redis {#venom---executor-redis}

Step per eseguire comandi su Redis

Caso d'uso: il tuo software deve fare chiamate a Redis.

## Input {#input}

Sono disponibili gli input seguenti:
- `commands`: un array di comandi Redis
- `path`: un file che contiene una serie di comandi Redis. Se la proprietà path è valorizzata, la proprietà commands viene ignorata.
- `dialURL`: URL del server Redis

L'URL deve seguire la bozza di specifica IANA per lo schema (https://www.iana.org/assignments/uri-schemes/prov/redis).
Se hai più test case o step che usano lo stesso URL di Redis, puoi definire l'impostazione `dialURL` una volta sola come variabile della test suite.

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

## Output {#output}

L'executor restituisce un oggetto result che contiene il comando Redis eseguito.

- result.commands contiene la lista dei comandi Redis eseguiti
- result.commands.commandI.response rappresenta la risposta del comando Redis. Può essere un array o una stringa, a seconda del comando Redis

## Esempi {#examples}

Altri esempi si trovano nella cartella `tests` di questo repository.
