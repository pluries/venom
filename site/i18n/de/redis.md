---
translates: redis
title: Redis
description: Den Redis-executor in einer venom-Test-Suite verwenden.
fingerprint: 3a8eed14b8743bc49ac23050af8244dc6ba4e18034d1258c07373943dcc6a7ba
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# Venom - Executor Redis {#venom---executor-redis}

Schritt zum Ausführen von Befehlen auf Redis

Anwendungsfall: Ihre Software muss Aufrufe an Redis senden.

## Eingabe {#input}

Die folgenden Eingaben stehen zur Verfügung:
- `commands`: ein Array von Redis-Befehlen
- `path`: eine Datei mit einer Reihe von Redis-Befehlen. Ist die Eigenschaft path gesetzt, wird die Eigenschaft commands ignoriert.
- `dialURL`: URL des Redis-Servers

Die URL sollte dem Entwurf der IANA-Spezifikation für das Schema folgen (https://www.iana.org/assignments/uri-schemes/prov/redis).
Wenn mehrere Testfälle oder Schritte dieselbe Redis-URL verwenden, können Sie die Einstellung `dialURL` einmal als Variable der Test-Suite definieren.

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

## Ausgabe {#output}

Der executor gibt ein Ergebnisobjekt zurück, das den ausgeführten Redis-Befehl enthält.

- result.commands enthält die Liste der ausgeführten Redis-Befehle
- result.commands.commandI.response stellt die Antwort des Redis-Befehls dar. Sie kann je nach Redis-Befehl ein Array oder ein String sein

## Beispiele {#examples}

Weitere Beispiele finden Sie im Ordner `tests` dieses Repositorys.
