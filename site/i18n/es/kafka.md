---
translates: kafka
title: Kafka
description: Cómo usar el executor Kafka en una suite de pruebas de venom.
fingerprint: 203adcc0e76d3ba3ad8df723c65a5da8e70724008296460332cc56291fc47cd2
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# Venom - Executor Kafka {#venom---executor-kafka}

Paso para leer de un topic de Kafka o escribir en él. También es posible usar un esquema Avro para codificar los mensajes de un topic de Kafka.

## Input {#input}

En su archivo YAML, puede usar:

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

`with_tls` y `withTLS` son dos grafías del mismo interruptor, y lo mismo ocurre con `insecure_tls` e `insecureTLS`: cualquiera
de las dos grafías activa su interruptor. Las versiones anteriores leían solo `withTLS` e `insecureTLS`: una suite que escribía `with_tls`
o `insecure_tls` se quedaba sin TLS o con verificación completa, sin ningún aviso. Ahora una suite así obtiene lo que pide, de modo que
`insecure_tls: true` deja de verificar el certificado del broker.

`tls_root_ca`, `tls_client_cert` y `tls_client_key` aceptan cada uno el contenido PEM en sí o la ruta de un archivo
PEM, relativa al archivo de la suite. Solo se aplican cuando TLS está activado:

- `tls_root_ca` se considera de confianza además de las CA del sistema, de modo que un broker firmado por una CA privada se verifica sin
  `insecure_tls`.
- `tls_client_cert` y `tls_client_key` van juntos: uno sin el otro hace fallar el paso, indicando el ajuste que
  falta, incluso con TLS desactivado.
- Con `insecure_tls`, el certificado del broker no se comprueba, pero el certificado de cliente se envía igualmente.
- Si se indican con TLS desactivado, estos ajustes no se usan: la conexión sigue en texto claro y venom registra una advertencia.

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

Ejemplo sin Avro:

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

Ejemplo con Avro:

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
