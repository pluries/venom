---
translates: amqp
title: AMQP
description: Come usare l'executor AMQP in una test suite di venom.
fingerprint: e8acaa4b982c026fe5078d6f3f02f3913d9bba4cdd4e284f094732b9d4f17b78
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# Venom - Executor AMQP {#venom---executor-amqp}

Step per pubblicare / sottoscrivere messaggi su un broker compatibile con AMQP 1.0 (implementato, tra gli altri, da QPID e ActiveMQ)

## Input {#input}

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

## Output {#output}

*Popolato quando ClientType è consumer*

```yaml
- result.messages     (array of strings, each containing the body of a response message)
- result.messagesJSON (if response is JSON, corresponding index will be populated with the navigable body of a response)
```
## Esempi {#examples}

### Publisher {#publisher}
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

### Consumer {#consumer}
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
