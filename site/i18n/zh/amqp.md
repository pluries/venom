---
translates: amqp
title: AMQP
description: 如何在 venom 测试套件中使用 AMQP executor。
fingerprint: e8acaa4b982c026fe5078d6f3f02f3913d9bba4cdd4e284f094732b9d4f17b78
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# Venom - AMQP Executor {#venom---executor-amqp}

用于向兼容 AMQP 1.0 的消息代理发布或订阅消息的步骤（目前 QPID 和 ActiveMQ 等均已实现该协议）。

## 输入 {#input}

```yaml
- addr         (address of the amqp broker to connect to, in the format "amqp://<host>:<port>")
- clientType   (consumer or producer)

# Consumer Parameters
- sourceAddr   (source topic/queue to consume messages from)
- messageLimit (number of messages to read from the broker before returning result)

# Producer Parameters
- targetAddr   (topic/queue to which messages should be published)
- messages     (array of message bodies to send to the broker)
```

## 输出 {#output}

*当 ClientType 为 consumer 时填充*

```yaml
- result.messages     (array of strings, each containing the body of a response message)
- result.messagesJSON (if response is JSON, corresponding index will be populated with the navigable body of a response)
```
## 示例 {#examples}

### 发布者 {#publisher}
```yaml
name: AMQP
testcases:
  - name: Producer
    steps:
      - type: amqp
        addr: amqp://localhost:5673
        clientType: producer
        targetAddr: amqp-test
        messages:
          - '{"key1":"value1","key2":"value2"}'
          - '{"key3":"value3","key4":"value4"}'
          - 'not json'
          - '["value5","value6"]'
```

### 消费者 {#consumer}
```yaml
name: AMQP
testcases:
  - name: Consumer
    steps:
     - type: amqp
        addr: amqp://localhost:5673
        clientType: consumer
        sourceAddr: amqp-test
        messageLimit: 4
        assertions:
          - result.messages.__Len__ ShouldEqual 4
          - result.messages.messages0 ShouldEqual '{"key1":"value1","key2":"value2"}'
          - result.messages.messages1 ShouldEqual '{"key3":"value3","key4":"value4"}'
          - result.messages.messages2 ShouldEqual 'not json'
          - result.messages.messages3 ShouldEqual '["value5","value6"]'
          - result.messagesjson.__Len__ ShouldEqual 4
          - result.messagesjson.messagesjson0.key1 ShouldEqual value1
          - result.messagesjson.messagesjson0.key2 ShouldEqual value2
          - result.messagesjson.messagesjson1.key3 ShouldEqual value3
          - result.messagesjson.messagesjson1.key4 ShouldEqual value4
          - result.messagesjson.messagesjson3.messagesjson30 ShouldEqual value5
          - result.messagesjson.messagesjson3.messagesjson31 ShouldEqual value6
```
