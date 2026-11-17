---
translates: mongo
title: Mongo
description: Come usare l'executor Mongo in una test suite di venom.
fingerprint: 06f23fc462b5feaf748ec60eb8983d082305404d10385ef51907b2975b7b62fa
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# Venom - Executor Mongo {#venom---executor-mongo}

Step per eseguire azioni su un database MongoDB.

**Versione del server:** MongoDB 4.4 o successiva. L'executor usa il driver Go ufficiale v2, che rifiuta i server più vecchi (il driver v1 accettava la 3.6 e successive). Le suite di venom stesse girano su MongoDB 8.2.

## Input {#input}

Vedi anche [tests/mongo.yml](../../tests/mongo.yml) per esempi eseguibili.

Nota: la maggior parte dei campi supporta [MongoDB Extended JSON v2](https://www.mongodb.com/docs/manual/reference/mongodb-extended-json/).
Questo significa che alcuni valori speciali (ObjectId, ISODate, ecc.) possono essere rappresentati nel formato relaxed.

Esempio:

```json
{
  "_id": {
    "$oid": "5d505646cf6d4fe581014ab2"
  }
}
```

### Carica le fixture {#load-fixtures}

```yaml
- type: mongo
  uri: mongodb://localhost:27017
  database: my-database
  actions:
    - type: loadFixtures
      folder: fixtures/
```

Questa azione prima **elimina tutte le collection del database**, poi carica più collection in una volta da una cartella.
La cartella delle fixture deve contenere un file per collection, con il nome della collection. Ad esempio, `cards.yml` crea una collection `cards`.
Gli elementi delle collection sono dichiarati come array YAML. Ad esempio:

```yaml
# fixtures/cards.yml
- suit: clubs
  value: jack

- suit: clubs
  value: queen

- suit: clubs
  value: king
```

### Inserisci documenti {#insert-documents}

```yaml
- type: mongo
  uri: mongodb://localhost:27017
  database: my-database
  collection: my-collection
  actions:
    - type: insert
      documents:
        - '{"suit": "hearts", "value": "queen"}'
        - '{"suit": "diamonds", "value": "three"}'
```

### Inserisci documenti da un file {#insert-documents-from-a-file}

```yaml
- type: mongo
  uri: mongodb://localhost:27017
  database: my-database
  collection: my-collection
  actions:
    - type: insert
      file: fixtures/my-collection.jsonlist
```

`fixtures/my-collection.jsonlist`:

```json
{
  "suit": "hearts",
  "value": "queen"
}
{
  "suit": "diamonds",
  "value": "three"
}
```

### Trova documenti {#find-documents}

```yaml
- type: mongo
  uri: mongodb://localhost:27017
  database: my-database
  collection: my-collection
  actions:
    - type: find
      filter: |
        {
          "suit": "clubs"
        }
      options: # optional
        limit: 3
        skip: 1
        sort: '{"_id": -1}'
        projection: '{"value": 1}'
```

### Trova documenti per ObjectID {#find-documents-by-objectid}

Vedi: [MongoDB Extended JSON - Type Representations](https://www.mongodb.com/docs/manual/reference/mongodb-extended-json/#type-representations)

```yaml
- type: mongo
  uri: mongodb://localhost:27017
  database: my-database
  collection: my-collection
  actions:
    - type: find
      filter: |
        {
          "_id": {
            "$oid": "5d505646cf6d4fe581014ab2"
          }
        }
      options: # optional
        limit: 3
        skip: 1
        sort: '{"_id": -1}'
        projection: '{"value": 1}'
```

### Conta documenti {#count-documents}

```yaml
- type: mongo
  uri: mongodb://localhost:27017
  database: my-database
  collection: my-collection
  actions:
    - type: count
      filter: |
        {
          "suit": "clubs"
        }
```

### Aggiorna documenti {#update-documents}

```yaml
- type: mongo
  uri: mongodb://localhost:27017
  database: my-database
  collection: my-collection
  actions:
    - type: update
      filter: |
        {
          "suit": {
            "$in": ["clubs", "spades"]
          }
        }
      update: |
        {
          "$set": {
            "color": "black"
          }
        }
```

### Elimina documenti {#delete-documents}

```yaml
- type: mongo
  uri: mongodb://localhost:27017
  database: my-database
  collection: my-collection
  actions:
    - type: delete
      filter: |
        {
          "suit": "circles"
        }
```

### Aggrega documenti {#aggregate-documents}

```yaml
- type: mongo
  uri: mongodb://localhost:27017
  database: my-database
  collection: my-collection
  actions:
    - type: aggregate
      pipeline:
        - |
          { "$match": {
            "color": "black"
          }}
        - |
          { "$group": {
            "_id": "$value",
            "count": {"$sum": 1}
          }}
```

### Crea una collection {#create-a-collection}

```yaml
- type: mongo
  uri: mongodb://localhost:27017
  database: my-database
  collection: my-collection
  actions:
    - type: createCollection
```

### Elimina una collection {#drop-a-collection}

```yaml
- type: mongo
  uri: mongodb://localhost:27017
  database: my-database
  collection: my-collection
  actions:
    - type: dropCollection
```
