---
translates: rabbitmq
title: RabbitMQ
description: Den RabbitMQ-executor in einer venom-Test-Suite verwenden.
fingerprint: 599ec2b85981637b67f72cdf17968f82fae0e2529d451982f8d88bad6bdf0118
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# Venom - Executor RabbitMQ {#venom---executor-rabbitmq}

Drei Ausführungsarten werden unterstützt:
- **publisher**: eine Nachricht an eine Queue oder einen Exchange senden.
- **subscriber**: an eine Queue oder einen Exchange binden (über den Routing-Key) und darauf warten, dass Nachrichten konsumiert werden.
- **client**: eine Nachricht an eine Queue oder einen Exchange senden und auf die Antwortnachricht in der [reply-to](https://www.rabbitmq.com/docs/direct-reply-to)-Queue warten.

Schritte, um Publish / Subscribe auf einem RabbitMQ zu verwenden:

## Eingabe {#input}
In Ihrer YAML-Datei können Sie Folgendes verwenden:

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

## Ausgabe {#output}

Das Ergebnis eines subscriber-Schritts enthält `messages`, `bodyjson` und `headers` (siehe die Beispiele). Jede Nachricht in
`result.messages.messagesN` trägt außerdem `Acknowledger`, die eigene Channel-Struktur der AMQP-Bibliothek
(`github.com/rabbitmq/amqp091-go`). Behandeln Sie `result.messages.messagesN.Acknowledger.*` als intern und instabil: Seine Schlüssel
und die Anzahl der gemeldeten Felder ändern sich mit der Bibliothek (sie änderten sich, als venom von `streadway/amqp` auf
`amqp091-go` wechselte, zum Beispiel stieg `__Len__` von 21 auf 28). Schreiben Sie keine Assertions darauf.

## Beispiele: {#examples}

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
