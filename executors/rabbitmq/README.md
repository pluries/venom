# Venom - Executor RabbitMQ

Three types of execution are supported:
- **publisher**: publish a message to a queue or to an exchange.
- **subscriber**: bind to a queue or an exchange (using routing key) and wait for message(s) to be consumed.
- **client**: publish a message to a queue or to an exchange and wait for the response message to be received on the [reply-to](https://www.rabbitmq.com/docs/direct-reply-to) queue.

Steps to use publish / subscribe on a RabbitMQ:

## Input
In your yaml file, you can use:

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

## Output

The result of a subscriber step has `messages`, `bodyjson` and `headers` (see the examples). Each message in
`result.messages.messagesN` also carries `Acknowledger`, which is the AMQP library's own channel struct
(`github.com/rabbitmq/amqp091-go`). Treat `result.messages.messagesN.Acknowledger.*` as internal and unstable: its keys
and the number of fields it reports change with the library (they changed when venom moved from `streadway/amqp` to
`amqp091-go`, for example `__Len__` went from 21 to 28). Do not write assertions on it.

## Examples:

### Publisher (workQ)
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

### Subscriber (workQ)
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

### Publisher (pubsub)
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

### Subscriber (pubsub)
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

### Client (pubsub RPC)
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
