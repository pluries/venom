---
translates: kafka
title: Kafka
description: Comment utiliser l'executor Kafka dans une suite de tests venom.
fingerprint: 203adcc0e76d3ba3ad8df723c65a5da8e70724008296460332cc56291fc47cd2
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# Venom - Executor Kafka {#venom---executor-kafka}

Étape pour lire depuis un topic Kafka ou y écrire. Il est aussi possible d'utiliser un schéma Avro pour encoder les messages d'un topic Kafka.

## Input {#input}

Dans votre fichier yaml, vous pouvez utiliser :

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

`with_tls` et `withTLS` sont deux écritures du même interrupteur, de même que `insecure_tls` et `insecureTLS` : l'une
ou l'autre active son interrupteur. Les versions précédentes ne lisaient que `withTLS` et `insecureTLS` : une suite qui écrivait `with_tls`
ou `insecure_tls` n'obtenait silencieusement aucun TLS, ou une vérification complète. Une telle suite obtient désormais ce qu'elle demande, donc
`insecure_tls: true` cesse de vérifier le certificat du broker.

`tls_root_ca`, `tls_client_cert` et `tls_client_key` acceptent chacun soit le contenu PEM lui-même, soit le chemin d'un fichier
PEM, relatif au fichier de la suite. Ils ne s'appliquent que lorsque TLS est activé :

- `tls_root_ca` est approuvé en plus des CA du système : un broker signé par une CA privée est donc vérifié sans
  `insecure_tls`.
- `tls_client_cert` et `tls_client_key` vont ensemble : l'un sans l'autre fait échouer l'étape en nommant le
  paramètre manquant, même si TLS est désactivé.
- Avec `insecure_tls`, le certificat du broker n'est pas vérifié, mais le certificat client est tout de même envoyé.
- Fournis alors que TLS est désactivé, ces paramètres ne sont pas utilisés : la connexion reste en clair et venom journalise un avertissement.

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

Exemple sans Avro :

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

Exemple avec Avro :

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
