---
translates: mongo
title: Mongo
description: Comment utiliser l'executor Mongo dans une suite de tests venom.
fingerprint: 06f23fc462b5feaf748ec60eb8983d082305404d10385ef51907b2975b7b62fa
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# Venom - Executor Mongo {#venom---executor-mongo}

Étape pour exécuter des actions sur une base MongoDB.

**Version du serveur :** MongoDB 4.4 ou ultérieure. L'executor utilise le driver Go officiel v2, qui refuse les serveurs plus anciens (le driver v1 acceptait la 3.6 et les versions ultérieures). Les propres suites de venom s'exécutent sur MongoDB 8.2.

## Input {#input}

Voir aussi [tests/mongo.yml](../../tests/mongo.yml) pour des exemples exécutables.

Remarque : la plupart des champs acceptent [MongoDB Extended JSON v2](https://www.mongodb.com/docs/manual/reference/mongodb-extended-json/).
Certaines valeurs spéciales (ObjectIds, ISODates, etc.) peuvent donc être représentées dans le format relaxed.

Exemple :

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

Cette action **supprime d'abord toutes les collections de la base**, puis charge plusieurs collections à la fois depuis un dossier.
Le dossier de fixtures doit contenir un fichier par collection, nommé d'après la collection. Par exemple, `cards.yml` crée une collection `cards`.
Les éléments des collections sont déclarés sous forme de tableau YAML. Par exemple :

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

`fixtures/my-collection.jsonlist` :

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

Voir : [MongoDB Extended JSON - Type Representations](https://www.mongodb.com/docs/manual/reference/mongodb-extended-json/#type-representations)

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
