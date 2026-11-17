---
translates: executors-concept
title: Executors
description: Ce qu'est un executor et lesquels sont fournis avec venom.
fingerprint: a4853e15b94adabbd472d94df4fa4fe4dded85edb7b601378c3188c9b5214206
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# Executors {#executors}

* **amqp**: https://github.com/ovh/venom/tree/master/executors/amqp
* **couchbase**: https://github.com/ovh/venom/tree/master/executors/couchbase
* **dbfixtures**: https://github.com/ovh/venom/tree/master/executors/dbfixtures
* **exec**: https://github.com/ovh/venom/tree/master/executors/exec `exec` est le type par défaut d'une étape
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

`odbc` et vos propres executors `lib/*.so` sont des plugins Go : ils ne se chargent que dans la version exacte de venom et de Go avec laquelle ils ont été compilés, donc recompilez-les après chaque mise à jour (voir [executors/plugins](executors/plugins/README.md)).

### Executors définis par l'utilisateur {#user-defined-executors}

Vous pouvez définir un executor dans un simple fichier YAML. C'est une bonne façon d'abstraire des comportements techniques ou fonctionnels et de les réutiliser dans des testsuites complexes.

Exemple :

fichier `lib/customA.yml` :

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

fichier `testsuite.yml` :

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

Remarquez la variable `alljson`. Toutes les variables déclarées dans output sont automatiquement converties au format JSON avec le suffixe `json`. Dans l'exemple ci-dessus, deux variables implicites sont disponibles : `displayjson.hello` et `alljson`.

Venom charge les executors définis par l'utilisateur depuis le répertoire `lib/`, relatif au chemin de la testsuite. Vous pouvez ajouter des chemins de sources d'executors avec l'option `--lib-dir`. 
Notez que tous les dossiers listés avec `--lib-dir` sont parcourus récursivement pour trouver les fichiers `.yml` qui servent d'executors utilisateur.

Un retry ne permet pas à une étape d'un executor utilisateur de se rétablir après l'échec d'une de ses étapes internes. Avec `retry: 2` sur une étape qui utilise un executor utilisateur, une étape interne qui échoue à la première tentative laisse l'étape en échec, même si toutes les tentatives suivantes réussissent (avec ou sans `timeout`). Un executor intégré se comporte autrement : seule la dernière tentative décide. Pour relancer quelque chose d'instable, placez `retry` sur l'étape interne, à l'intérieur de l'executor utilisateur, ou utilisez pour cela un executor intégré.

Les executors définis par l'utilisateur fonctionnent avec les templates : vous pouvez consulter le résultat du templating dans `venom.log`. Dans ce fichier, si vous voyez une erreur comme `error converting YAML to JSON: yaml: line 14: found unexpected end of stream`, il faut probablement ajuster l'indentation avec la fonction de template `indent`. 

Exemple :

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

avec cet executor :

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
