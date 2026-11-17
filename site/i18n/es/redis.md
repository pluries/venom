---
translates: redis
title: Redis
description: Cómo usar el executor Redis en una suite de pruebas de venom.
fingerprint: 3a8eed14b8743bc49ac23050af8244dc6ba4e18034d1258c07373943dcc6a7ba
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# Venom - Executor Redis {#venom---executor-redis}

Paso para ejecutar comandos en Redis

Caso de uso: su software necesita hacer llamadas a Redis.

## Entrada {#input}

Están disponibles las siguientes entradas:
- `commands`: una lista de comandos de Redis
- `path`: un archivo que contiene una serie de comandos de Redis. Si se rellena la propiedad path, la propiedad commands se ignora.
- `dialURL`: URL del servidor Redis

La URL debe seguir el borrador de la especificación de IANA para el esquema (https://www.iana.org/assignments/uri-schemes/prov/redis).
Si varios casos de prueba o pasos usan la misma URL de Redis, puede definir el ajuste `dialURL` una sola vez como variable de la suite de pruebas.

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

## Salida {#output}

El executor devuelve un objeto de resultado que contiene el comando de Redis ejecutado.

- result.commands contiene la lista de comandos de Redis ejecutados
- result.commands.commandI.response representa la respuesta del comando de Redis. Puede ser una lista o una cadena, según el comando de Redis

## Ejemplos {#examples}

Encontrará más ejemplos en la carpeta `tests` de este repositorio.
