---
translates: mongo
title: Mongo
description: Cómo usar el executor Mongo en una suite de pruebas de venom.
fingerprint: 06f23fc462b5feaf748ec60eb8983d082305404d10385ef51907b2975b7b62fa
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# Venom - Executor Mongo {#venom---executor-mongo}

Paso para ejecutar acciones en una base de datos MongoDB.

**Versión del servidor:** MongoDB 4.4 o posterior. El executor usa el driver oficial de Go v2, que rechaza servidores más antiguos (el driver v1 aceptaba 3.6 y posteriores). Las suites propias de venom se ejecutan contra MongoDB 8.2.

## Input {#input}

Consulte también [tests/mongo.yml](../../tests/mongo.yml) para ver ejemplos ejecutables.

Nota: la mayoría de los campos admiten [MongoDB Extended JSON v2](https://www.mongodb.com/docs/manual/reference/mongodb-extended-json/).
Esto significa que algunos valores especiales (ObjectIds, ISODates, etc.) pueden representarse con el formato relajado.

Ejemplo:

```json
{
  "_id": {
    "$oid": "5d505646cf6d4fe581014ab2"
  }
}
```

### Load fixtures {#load-fixtures}

```yaml
- type: mongo
  uri: mongodb://localhost:27017
  database: my-database
  actions:
    - type: loadFixtures
      folder: fixtures/
```

Esta acción primero **elimina todas las colecciones de la base de datos** y después carga varias colecciones a la vez desde una carpeta.
La carpeta de fixtures debe contener un archivo por colección, con el nombre de la colección. Por ejemplo, `cards.yml` creará una colección `cards`.
Los elementos de las colecciones se declaran como un array YAML. Por ejemplo:

```yaml
# fixtures/cards.yml
- suit: clubs
  value: jack

- suit: clubs
  value: queen

- suit: clubs
  value: king
```

### Insert documents {#insert-documents}

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

### Insert documents from a file {#insert-documents-from-a-file}

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

### Find documents {#find-documents}

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

### Find documents by ObjectID {#find-documents-by-objectid}

Consulte: [MongoDB Extended JSON - Type Representations](https://www.mongodb.com/docs/manual/reference/mongodb-extended-json/#type-representations)

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

### Count documents {#count-documents}

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

### Update documents {#update-documents}

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

### Delete documents {#delete-documents}

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

### Aggregate documents {#aggregate-documents}

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

### Create a collection {#create-a-collection}

```yaml
- type: mongo
  uri: mongodb://localhost:27017
  database: my-database
  collection: my-collection
  actions:
    - type: createCollection
```

### Drop a collection {#drop-a-collection}

```yaml
- type: mongo
  uri: mongodb://localhost:27017
  database: my-database
  collection: my-collection
  actions:
    - type: dropCollection
```
