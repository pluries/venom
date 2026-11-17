---
translates: rabbitmq
title: RabbitMQ
description: Cómo usar el executor RabbitMQ en una suite de pruebas de venom.
fingerprint: 599ec2b85981637b67f72cdf17968f82fae0e2529d451982f8d88bad6bdf0118
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# Venom - Executor RabbitMQ {#venom---executor-rabbitmq}

Se admiten tres tipos de ejecución:
- **publisher**: publica un mensaje en una cola o en un exchange.
- **subscriber**: se enlaza a una cola o a un exchange (mediante la routing key) y espera a que se consuman uno o varios mensajes.
- **client**: publica un mensaje en una cola o en un exchange y espera la respuesta en la cola [reply-to](https://www.rabbitmq.com/docs/direct-reply-to).

Pasos para publicar y suscribirse en RabbitMQ:

## Entrada {#input}
En su archivo YAML puede usar:

```yaml

  # RabbitMQ connection
  - addrs optional        (default amqp:/localhost:5672)
  - user optional         (default guest)
  - password optional     (default guest)

  - clientType mandatory (publisher, subscriber or client)

  # RabbitMQ Q configuration
  - qName mandatory

  # Exchange configuration
  - routingKey optional   (default qName)
  - exchangeType optional  (default "fanout")
  - exchange optional     (default "")

  # For subscriber and client only
  - messageLimit optional (default 1)

  # For publisher and client only
  - messages
    - durable optional      (true or false) (default false)
    - contentType optional  
    - contentEncoding optional
    - persistent optional (default true)
    - headers optional
      - name: value

```

## Salida {#output}

El resultado de un paso subscriber incluye `messages`, `bodyjson` y `headers` (vea los ejemplos). Cada mensaje de
`result.messages.messagesN` incluye además `Acknowledger`, que es la propia estructura de canal de la biblioteca AMQP
(`github.com/rabbitmq/amqp091-go`). Considere `result.messages.messagesN.Acknowledger.*` como interno e inestable: sus claves
y el número de campos que informa cambian con la biblioteca (cambiaron cuando venom pasó de `streadway/amqp` a
`amqp091-go`, por ejemplo `__Len__` pasó de 21 a 28). No escriba aserciones sobre él.

## Ejemplos: {#examples}

### Publisher (workQ) {#publisher-workq}
```yaml
name: TestSuite RabbitMQ
vars:
  addrs: 'amqp://localhost:5672'
  user: 
  password: 
testcases:
  - name: RabbitMQ publish (work Q)
    steps:
      - type: rabbitmq
        addrs: "{{.addrs}}"
        user: "{{.user}}"
        password: "{{.password}}"
        clientType: publisher
        qName: TEST
        messages: 
          - value: '{"a": "b"}'
            contentType: application/json
            contentEncoding: utf8
            persistent: false
            headers: 
              myCustomHeader: value
              myCustomHeader2: value2
```

### Subscriber (workQ) {#subscriber-workq}
```yaml
name: TestSuite RabbitMQ
vars:
  addrs: 'amqp://localhost:5672'
  user: 
  password: 
  - name: RabbitMQ subscribe testcase
    steps:
      - type: rabbitmq
        addrs: "{{.addrs}}"
        user: "{{.user}}"
        password: "{{.password}}"
        clientType: subscriber
        qName: "{{.qName}}"
        durable: true
        messageLimit: 1
        assertions: 
          - result.bodyjson.bodyjson0.a ShouldEqual b   
          - result.headers.headers0.mycustomheader ShouldEqual value
          - result.headers.headers0.mycustomheader2 ShouldEqual value2
          - result.messages.messages0.contentencoding ShouldEqual utf8
          - result.messages.messages0.contenttype ShouldEqual application/json
```

### Publisher (pubsub) {#publisher-pubsub}
```yaml
name: TestSuite RabbitMQ
vars:
  addrs: 'amqp://localhost:5672'
  user: 
  password: 
testcases:
  - name: RabbitMQ publish (work Q)
    steps:
      - type: rabbitmq
        addrs: "{{.addrs}}"
        user: "{{.user}}"
        password: "{{.password}}"
        clientType: publisher
        exchange: exchange_test
        routingKey: pubsub_test
        messages: 
          - value: '{"a": "b"}'
            contentType: application/json
            contentEncoding: utf8
            persistent: false
            headers: 
              myCustomHeader: value
              myCustomHeader2: value2
```

### Subscriber (pubsub) {#subscriber-pubsub}
```yaml
name: TestSuite RabbitMQ
vars:
  addrs: 'amqp://localhost:5672'
  user: 
  password: 
  - name: RabbitMQ subscribe testcase
    steps:
      - type: rabbitmq
        addrs: "{{.addrs}}"
        user: "{{.user}}"
        password: "{{.password}}"
        clientType: subscriber
        exchange: exchange_test
        routingKey: pubsub_test
        messageLimit: 1
        assertions: 
          - result.bodyjson.bodyjson0.a ShouldEqual b   
          - result.headers.headers0.mycustomheader ShouldEqual value
          - result.headers.headers0.mycustomheader2 ShouldEqual value2
          - result.messages.messages0.contentencoding ShouldEqual utf8
          - result.messages.messages0.contenttype ShouldEqual application/json
```

### Client (pubsub RPC) {#client-pubsub-rpc}
```yaml
name: TestSuite RabbitMQ
vars:
  addrs: 'amqp://localhost:5672'
  user: 
  password: 
testcases:
  - name: RabbitMQ request/reply
    steps:
      - type: rabbitmq
        addrs: "{{.addrs}}"
        user: "{{.user}}"
        password: "{{.password}}"
        clientType: client
        exchange: exchange_test
        routingKey: pubsub_test
        messages: 
          - value: '{"a": "b"}'
            contentType: application/json
            contentEncoding: utf8
            persistent: false
            headers: 
              myCustomHeader: value
              myCustomHeader2: value2
        messageLimit: 1
        assertions: 
          - result.bodyjson.bodyjson0 ShouldContainKey Status
          - result.bodyjson.bodyjson0.Status ShouldEqual Succeeded
```
