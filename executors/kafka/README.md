# Venom - Executor Kafka

Step to read from / write to a Kafka topic. It is also possible to use an Avro schema to encode messages in a Kafka topic.

## Input

In your yaml file, you can use:

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

### TLS

`with_tls` and `withTLS` are two spellings of the same switch, and so are `insecure_tls` and `insecureTLS`: either
spelling turns its switch on. Earlier versions read only `withTLS` and `insecureTLS`: a suite that wrote `with_tls`
or `insecure_tls` silently got no TLS, or full verification. Such a suite now gets what it asks for, so
`insecure_tls: true` stops verifying the broker certificate.

`tls_root_ca`, `tls_client_cert` and `tls_client_key` each take either the PEM content itself or the path of a PEM
file, relative to the suite file. They apply only when TLS is on:

- `tls_root_ca` is trusted on top of the system CAs, so a broker signed by a private CA is verified without
  `insecure_tls`.
- `tls_client_cert` and `tls_client_key` go together: one without the other fails the step, naming the missing
  setting, even with TLS off.
- With `insecure_tls`, the broker certificate is not checked, but the client certificate is still sent.
- Given while TLS is off, these settings are not used: the connection stays plaintext and venom logs a warning.

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

Example without Avro:

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

Example with Avro:

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
