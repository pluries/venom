---
translates: kafka
title: Kafka
description: Come usare l'executor Kafka in una test suite di venom.
fingerprint: 203adcc0e76d3ba3ad8df723c65a5da8e70724008296460332cc56291fc47cd2
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# Venom - Executor Kafka {#venom---executor-kafka}

Step per leggere da un topic Kafka / scrivervi. È anche possibile usare uno schema Avro per codificare i messaggi di un topic Kafka.

## Input {#input}

Nel tuo file YAML puoi usare:

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

`with_tls` e `withTLS` sono due grafie dello stesso interruttore, e lo stesso vale per `insecure_tls` e `insecureTLS`: con
l'una o l'altra grafia l'interruttore si attiva. Le versioni precedenti leggevano solo `withTLS` e `insecureTLS`: una suite che scriveva `with_tls`
o `insecure_tls` otteneva in silenzio nessun TLS, oppure la verifica completa. Ora una suite del genere ottiene ciò che chiede, quindi
`insecure_tls: true` smette di verificare il certificato del broker.

`tls_root_ca`, `tls_client_cert` e `tls_client_key` accettano ciascuno il contenuto PEM stesso oppure il percorso di un file
PEM, relativo al file della suite. Si applicano solo quando TLS è attivo:

- `tls_root_ca` è considerata attendibile in aggiunta alle CA di sistema, quindi un broker firmato da una CA privata viene verificato senza
  `insecure_tls`.
- `tls_client_cert` e `tls_client_key` vanno insieme: uno senza l'altro fa fallire lo step, indicando l'impostazione
  mancante, anche con TLS disattivato.
- Con `insecure_tls` il certificato del broker non viene controllato, ma il certificato client viene comunque inviato.
- Se indicate mentre TLS è disattivato, queste impostazioni non vengono usate: la connessione resta in chiaro e venom registra un avviso.

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

Esempio senza Avro:

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

Esempio con Avro:

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
