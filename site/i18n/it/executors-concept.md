---
translates: executors-concept
title: Executor
description: Cos'è un executor e quali sono inclusi in venom.
fingerprint: a4853e15b94adabbd472d94df4fa4fe4dded85edb7b601378c3188c9b5214206
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# Executor {#executors}

* **amqp**: https://github.com/ovh/venom/tree/master/executors/amqp
* **couchbase**: https://github.com/ovh/venom/tree/master/executors/couchbase
* **dbfixtures**: https://github.com/ovh/venom/tree/master/executors/dbfixtures
* **exec**: https://github.com/ovh/venom/tree/master/executors/exec `exec` è il tipo predefinito di uno step
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

`odbc` e i tuoi executor `lib/*.so` sono plugin Go: si caricano solo nella versione esatta di venom e di Go con cui sono stati
compilati, quindi ricompilali dopo ogni aggiornamento (vedi [executors/plugins](executors/plugins/README.md)).

### Executor definiti dall'utente {#user-defined-executors}

Puoi definire un executor con un singolo file YAML. È un buon modo per astrarre comportamenti tecnici o funzionali e riutilizzarli in testsuite complesse.

Esempio:

file `lib/customA.yml`:

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

file `testsuite.yml`:

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

Nota la variabile `alljson`. Tutte le variabili dichiarate in output vengono convertite automaticamente in formato JSON con il suffisso `json`. Nell'esempio qui sopra sono disponibili due variabili implicite: `displayjson.hello` e `alljson`.

Venom carica gli executor definiti dall'utente dalla directory `lib/` relativa al percorso della testsuite. Puoi aggiungere percorsi di origine degli executor con il flag `--lib-dir`. 
Nota che tutte le cartelle elencate con `--lib-dir` vengono scansionate ricorsivamente per trovare i file `.yml` da usare come executor utente.

Un retry non recupera lo step di un executor utente dal fallimento di uno dei suoi step interni. Con `retry: 2` su uno step che usa un executor utente, uno step interno che fallisce al primo tentativo mantiene lo step in errore, anche se tutti i tentativi successivi passano (con o senza `timeout`). Un executor integrato si comporta in modo diverso: decide solo l'ultimo tentativo. Per ritentare qualcosa di instabile, metti `retry` sullo step interno dentro l'executor utente, oppure usa per quel caso un executor integrato.

Gli executor definiti dall'utente funzionano con il templating, e puoi controllare il risultato del templating in `venom.log`. In quel file, se vedi un errore come `error converting YAML to JSON: yaml: line 14: found unexpected end of stream`, probabilmente devi correggere l'indentazione con la funzione di templating `indent`. 

Esempio:

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

che usa questo executor:

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
