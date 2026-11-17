---
translates: rabbitmq
title: RabbitMQ
description: 如何在 venom 测试套件中使用 RabbitMQ executor。
fingerprint: 599ec2b85981637b67f72cdf17968f82fae0e2529d451982f8d88bad6bdf0118
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# Venom - Executor RabbitMQ {#venom---executor-rabbitmq}

支持三种执行类型：
- **publisher**：向队列或交换机发布消息。
- **subscriber**：绑定到队列或交换机（使用 routing key），并等待消息被消费。
- **client**：向队列或交换机发布消息，并等待在 [reply-to](https://www.rabbitmq.com/docs/direct-reply-to) 队列上收到响应消息。

在 RabbitMQ 上使用发布 / 订阅的步骤：

## 输入 {#input}
在您的 yaml 文件中，可以使用：

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

## 输出 {#output}

subscriber 步骤的结果包含 `messages`、`bodyjson` 和 `headers`（见示例）。`result.messages.messagesN` 中的每条消息还带有 `Acknowledger`，它是 AMQP 库自己的 channel 结构体
（`github.com/rabbitmq/amqp091-go`）。请把 `result.messages.messagesN.Acknowledger.*` 视为内部且不稳定的内容：它的键
以及所报告的字段数量会随库而变化（例如 venom 从 `streadway/amqp` 迁移到
`amqp091-go` 时它们就变了，`__Len__` 从 21 变成了 28）。请不要对它编写断言。

## 示例： {#examples}

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
