---
translates: executors-concept
title: Executors
description: Was ein executor ist und welche mit venom ausgeliefert werden.
fingerprint: a4853e15b94adabbd472d94df4fa4fe4dded85edb7b601378c3188c9b5214206
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# Executors {#executors}

* **amqp**: https://github.com/ovh/venom/tree/master/executors/amqp
* **couchbase**: https://github.com/ovh/venom/tree/master/executors/couchbase
* **dbfixtures**: https://github.com/ovh/venom/tree/master/executors/dbfixtures
* **exec**: https://github.com/ovh/venom/tree/master/executors/exec `exec` ist der Standardtyp eines Schritts
* **grpc**: https://github.com/ovh/venom/tree/master/executors/grpc
* **http**: https://github.com/ovh/venom/tree/master/executors/http
* **imap**: https://github.com/ovh/venom/tree/master/executors/imap
* **kafka** https://github.com/ovh/venom/tree/master/executors/kafka
* **mcp**: https://github.com/ovh/venom/tree/master/executors/mcp
* **mobile**: https://github.com/ovh/venom/tree/master/executors/mobile
* **mongo**: https://github.com/ovh/venom/tree/master/executors/mongo
* **mqtt** https://github.com/ovh/venom/tree/master/executors/mqtt
* **odbc**: https://github.com/ovh/venom/tree/master/executors/plugins/odbc
* **ovhapi**: https://github.com/ovh/venom/tree/master/executors/ovhapi
* **rabbitmq**: https://github.com/ovh/venom/tree/master/executors/rabbitmq
* **readfile**: https://github.com/ovh/venom/tree/master/executors/readfile
* **redis**: https://github.com/ovh/venom/tree/master/executors/redis
* **smtp**: https://github.com/ovh/venom/tree/master/executors/smtp
* **sql**: https://github.com/ovh/venom/tree/master/executors/sql
* **ssh**: https://github.com/ovh/venom/tree/master/executors/ssh
* **web**: https://github.com/ovh/venom/tree/master/executors/web

`odbc` und Ihre eigenen `lib/*.so`-Executors sind Go-Plugins: Sie lassen sich nur in genau die venom- und Go-Version laden, mit der sie gebaut wurden. Bauen Sie sie deshalb nach jedem Upgrade neu (siehe [executors/plugins](executors/plugins/README.md)).

### Benutzerdefinierte Executors {#user-defined-executors}

Sie können einen executor mit einer einzigen YAML-Datei definieren. So lassen sich technisches oder fachliches Verhalten gut abstrahieren und in komplexen Testsuites wiederverwenden.

Beispiel:

Datei `lib/customA.yml`:

```yml
executor: hello
input:
  myarg: {}
steps:
- script: echo "{\"hello\":\"{{.input.myarg}}\"}"
  assertions:
  - result.code ShouldEqual 0
  vars:
    hello:
      from: result.systemoutjson.hello
    all:
      from: result.systemoutjson
output:
  display:
    hello: "{{.hello}}"
  all: "{{.all}}"
```

Datei `testsuite.yml`:

```yml
name: testsuite with a user executor
testcases:
- name: testA
  steps:
  - type: hello
    myarg: World
    assertions:
    - result.display.hello ShouldContainSubstring World
    - result.alljson.hello ShouldContainSubstring World
```

Beachten Sie die Variable `alljson`. Alle in output deklarierten Variablen werden automatisch mit dem Suffix `json` in das JSON-Format umgewandelt. Im obigen Beispiel stehen damit zwei implizite Variablen zur Verfügung: `displayjson.hello` und `alljson`.

Venom lädt benutzerdefinierte Executors aus dem Verzeichnis `lib/` relativ zum Pfad der Testsuite. Mit dem Flag `--lib-dir` können Sie weitere Quellpfade für Executors angeben.
Beachten Sie, dass alle mit `--lib-dir` aufgeführten Ordner rekursiv nach `.yml`-Dateien durchsucht werden, die als benutzerdefinierte Executors geladen werden.

Ein Retry stellt einen Schritt mit benutzerdefiniertem executor nicht wieder her, wenn einer seiner inneren Schritte fehlschlägt. Bei `retry: 2` an einem Schritt, der einen benutzerdefinierten executor verwendet, bleibt der Schritt fehlgeschlagen, wenn ein innerer Schritt beim ersten Versuch fehlschlägt, selbst wenn alle späteren Versuche bestehen (mit oder ohne `timeout`). Ein eingebauter executor verhält sich anders: Dort entscheidet nur der letzte Versuch. Um etwas Unzuverlässiges zu wiederholen, setzen Sie `retry` auf den inneren Schritt im benutzerdefinierten executor oder verwenden Sie dafür einen eingebauten executor.

Benutzerdefinierte Executors funktionieren mit Templating; das Ergebnis des Templatings können Sie in `venom.log` prüfen. Wenn Sie in dieser Datei einen Fehler wie `error converting YAML to JSON: yaml: line 14: found unexpected end of stream` sehen, müssen Sie wahrscheinlich die Einrückung mit der Templating-Funktion `indent` anpassen.

Beispiel:

```yml
name: testsuite with a user executor multilines
testcases:
- name: test
  steps:
  - type: multilines
    script: |
      # test multilines
      echo "5"
    assertions:
    - result.alljson ShouldEqual 5
```

mit diesem executor:

```yml
executor: multilines
input:
  script: "echo 'foo'"
steps:
- type: exec
  script: {{ .input.script | nindent 4 }}
  assertions:
  - result.code ShouldEqual 0
  vars:
    all:
      from: result.systemoutjson
output:
  all: '{{.all}}'
```


```bash
# lib/*.yml files will be loaded as executors.
$ venom run testsuite.yml 

# executors will be loaded from /etc/venom/lib, $HOME/venom.d/lib and lib/ directory relative to testsuite.yml file.
$ venom run --lib-dir=/etc/venom/lib:$HOME/venom.d/lib testsuite.yml 
```
