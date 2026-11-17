---
translates: kafka
title: Kafka
description: 如何在 venom 测试套件中使用 Kafka executor。
fingerprint: 203adcc0e76d3ba3ad8df723c65a5da8e70724008296460332cc56291fc47cd2
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# Venom - Kafka Executor {#venom---executor-kafka}

用于从 Kafka 主题读取消息或向其写入消息的步骤。也可以使用 Avro 模式对 Kafka 主题中的消息进行编码。

## 输入 {#input}

在您的 yaml 文件中，可以使用：

```yaml
  - addrs mandatory
  - with_tls optional (also spelled withTLS) - connect to the brokers over TLS
  - with_sasl optional
  - with_sasl_handshaked optional
  - with_avro optional - describes if this test should expect Avro schema to be used. NOTE: if you use it for the consumer, you will have to use it for the producer too.
  - user optional
  - password optional
  - kafka_version optional, default is 0.10.2.0
  - insecure_tls optional (also spelled insecureTLS) - with TLS on, do not verify the broker certificate. Only for test brokers you control
  - tls_root_ca optional - CA certificate trusted on top of the system ones, to verify the broker certificate
  - tls_client_cert optional - client certificate, for brokers that require one (mutual TLS)
  - tls_client_key optional - private key of tls_client_cert

  - client_type mandatory: producer or consumer

  # for consumer client type:
  - group_id mandatory
  - topics mandatory
  - timeout optional
  - message_limit optional
  - initial_offset optional - Sarama default is newest
  - mark_offset optional
  - wait_for optional - Wait X seconds before returning the consumed
  messages from the topic.
  - key_filter optional - perform filtering per key

  # for producer client type:
  - messages
  - messages.topic - Topic where to post message
  - messages.headers - Headers for message (optional)
  - messages.value - Value for message
  - messages.valueFile - Take value for message from file provided here
  - messages.avroSchemaFile - Specify an Avro schema file. messages.valueFile or messages.value should have a value that can be encoded with that schema. If not provided, then it will retrieve the latest available version from schema registry using the Topic Name strategy, that is, ${topicName}-value as subject.
```

### TLS {#tls}

`with_tls` 和 `withTLS` 是同一个开关的两种写法，`insecure_tls` 和 `insecureTLS` 也是如此：使用任何一种写法都会打开对应的开关。
早期版本只读取 `withTLS` 和 `insecureTLS`：写成 `with_tls` 或 `insecure_tls` 的测试套件会悄无声息地得不到 TLS，或得到完整验证。
现在这样的测试套件会得到它所要求的行为，因此 `insecure_tls: true` 会停止验证代理证书。

`tls_root_ca`、`tls_client_cert` 和 `tls_client_key` 各自既可以填写 PEM 内容本身，也可以填写 PEM 文件的路径（相对于测试套件文件）。
它们仅在启用 TLS 时生效：

- `tls_root_ca` 会在系统 CA 之外额外受到信任，因此由私有 CA 签发证书的代理无需 `insecure_tls` 也能通过验证。
- `tls_client_cert` 和 `tls_client_key` 必须成对出现：只给其中一个会使步骤失败，并指出缺少的设置，即使 TLS 处于关闭状态也是如此。
- 使用 `insecure_tls` 时，不检查代理证书，但仍会发送客户端证书。
- 在 TLS 关闭时给出这些设置，它们不会被使用：连接仍为明文，并且 venom 会记录一条警告。

```yaml
  - type: kafka
    clientType: producer
    with_tls: true
    tls_root_ca: certs/ca.pem
    tls_client_cert: certs/client.pem
    tls_client_key: certs/client-key.pem
    addrs:
      - "{{.kafkaHost}}:{{.kafkaPort}}"
    messages:
    - topic: test-topic
      value: '{"hello":"bar"}'
```

不使用 Avro 的示例：

```yaml
name: My Kafka testsuite
version: "2"
testcases:
- name: Kafka test
  description: Test Kafka
  steps:
  - type: kafka
    clientType: producer
    withSASL: true
    withTLS: true
    user: "{{.kafkaUser}}"
    password: "{{.kafkaPwd}}"
    addrs:
      - "{{.kafkaHost}}:{{.kafkaPort}}"
    messages:
    - topic: test-topic
      value: '{"hello":"bar"}'
  - type: kafka
    clientType: consumer
    withTLS: true
    withSASL: true
    user: "{{.kafkaUser}}"
    password: "{{.kafkaPwd}}"
    markOffset: true
    initialOffset: oldest
    messageLimit: 1
    groupID: venom
    addrs:
      - "{{.kafkaHost}}:{{.kafkaPort}}"
    topics:
      - test-topic
    assertions:
    - result.messagesjson.messagesjson0.value.hello ShouldEqual bar
    - result.messages.__Len__ ShouldEqual 1

```

使用 Avro 的示例：

```yaml
name: My Kafka testsuite
version: "2"
testcases:
- name: Kafka test
  description: Test Kafka
  steps:
  - type: kafka
    clientType: producer
    withSASL: true
    withTLS: true
    user: "{{.kafkaUser}}"
    password: "{{.kafkaPwd}}"
    addrs:
      - "{{.kafkaHost}}:{{.kafkaPort}}"
    messages:
    - topic: test-topic
      valueFile: "kafka/values/message2.json"
      avroSchemaFile: "kafka/schemas/message.avsc"
    - topic: test-topic
      valueFile: "kafka/values/message3.json"
  - type: kafka
    clientType: consumer
    withTLS: true
    withSASL: true
    user: "{{.kafkaUser}}"
    password: "{{.kafkaPwd}}"
    markOffset: true
    initialOffset: oldest
    messageLimit: 2
    groupID: venom
    addrs:
      - "{{.kafkaHost}}:{{.kafkaPort}}"
    topics:
      - test-topic
    assertions:
    - result.messagesjson.messagesjson0.value.id ShouldEqual 1
    - result.messagesjson.messagesjson0.value.message ShouldEqual "Some test"
    - result.messagesjson.messagesjson1.value.id ShouldEqual 2
    - result.messages.__Len__ ShouldEqual 2
```
