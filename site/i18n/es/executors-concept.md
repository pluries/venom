---
translates: executors-concept
title: Executors
description: Qué es un executor y cuáles incluye venom.
fingerprint: a4853e15b94adabbd472d94df4fa4fe4dded85edb7b601378c3188c9b5214206
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# Executors {#executors}

* **amqp**: https://github.com/ovh/venom/tree/master/executors/amqp
* **couchbase**: https://github.com/ovh/venom/tree/master/executors/couchbase
* **dbfixtures**: https://github.com/ovh/venom/tree/master/executors/dbfixtures
* **exec**: https://github.com/ovh/venom/tree/master/executors/exec `exec` es el tipo por defecto de un paso
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

`odbc` y sus propios executors `lib/*.so` son plugins de Go: solo se cargan en la versión exacta de venom y de Go con la que se compilaron, así que recompílelos después de cada actualización (consulte [executors/plugins](executors/plugins/README.md)).

### Executors definidos por el usuario {#user-defined-executors}

Puede definir un executor con un único archivo YAML. Es una buena forma de abstraer comportamientos técnicos o funcionales y reutilizarlos en suites de pruebas complejas.

Ejemplo:

archivo `lib/customA.yml`:

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

archivo `testsuite.yml`:

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

Observe la variable `alljson`. Todas las variables declaradas en output se convierten automáticamente a formato JSON con el sufijo `json`. En el ejemplo anterior hay dos variables implícitas disponibles: `displayjson.hello` y `alljson`.

Venom cargará los executors definidos por el usuario desde el directorio `lib/` relativo a la ruta de la suite de pruebas. Puede añadir rutas de origen de executors con la opción `--lib-dir`. 
Tenga en cuenta que todas las carpetas indicadas con `--lib-dir` se analizarán de forma recursiva para encontrar archivos `.yml` como executors de usuario.

Un reintento no recupera un paso de executor de usuario tras el fallo de uno de sus pasos internos. Con `retry: 2` en un paso que usa un executor de usuario, un paso interno que falla en el primer intento mantiene el paso como fallido, aunque todos los intentos posteriores se superen (con o sin `timeout`). Un executor integrado se comporta de otra manera: solo decide el último intento. Para reintentar algo inestable, ponga `retry` en el paso interno dentro del executor de usuario, o use un executor integrado para ello.

Los executors definidos por el usuario funcionan con plantillas; puede comprobar el resultado de la plantilla en `venom.log`. En ese archivo, si ve un error como `error converting YAML to JSON: yaml: line 14: found unexpected end of stream`, probablemente deba ajustar la sangría con la función de plantilla `indent`. 

Ejemplo:

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

con este executor:

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
