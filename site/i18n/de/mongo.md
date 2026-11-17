---
translates: mongo
title: Mongo
description: So verwenden Sie den Mongo-executor in einer venom-Test-Suite.
fingerprint: 06f23fc462b5feaf748ec60eb8983d082305404d10385ef51907b2975b7b62fa
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# Venom - Executor Mongo {#venom---executor-mongo}

Schritt zum Ausführen von Aktionen auf einer MongoDB-Datenbank.

**Server-Version:** MongoDB 4.4 oder neuer. Der executor nutzt den offiziellen Go-Treiber v2, der ältere Server ablehnt (der Treiber v1 akzeptierte 3.6 und neuer). Die eigenen Suites von Venom laufen gegen MongoDB 8.2.

## Input {#input}

Ausführbare Beispiele finden Sie auch in [tests/mongo.yml](../../tests/mongo.yml).

Hinweis: Die meisten Felder unterstützen [MongoDB Extended JSON v2](https://www.mongodb.com/docs/manual/reference/mongodb-extended-json/).
Das heißt, einige Sonderwerte (ObjectIds, ISODates usw.) lassen sich im Relaxed-Format darstellen.

Beispiel:

```json
{
  "_id": {
    "$oid": "5d505646cf6d4fe581014ab2"
  }
}
```

### Fixtures laden {#load-fixtures}

```yaml
- type: mongo
  uri: mongodb://localhost:27017
  database: my-database
  actions:
    - type: loadFixtures
      folder: fixtures/
```

Diese Aktion **löscht zuerst alle Collections der Datenbank** und lädt dann mehrere Collections auf einmal aus einem Ordner.
Der Fixtures-Ordner muss pro Collection eine Datei enthalten, die nach der Collection benannt ist. So legt `cards.yml` eine Collection `cards` an.
Die Einträge der Collections werden als YAML-Array deklariert. Zum Beispiel:

```yaml
# fixtures/cards.yml
- suit: clubs
  value: jack

- suit: clubs
  value: queen

- suit: clubs
  value: king
```

### Dokumente einfügen {#insert-documents}

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

### Dokumente aus einer Datei einfügen {#insert-documents-from-a-file}

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

### Dokumente suchen {#find-documents}

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

### Dokumente nach ObjectID suchen {#find-documents-by-objectid}

Siehe: [MongoDB Extended JSON - Type Representations](https://www.mongodb.com/docs/manual/reference/mongodb-extended-json/#type-representations)

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

### Dokumente zählen {#count-documents}

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

### Dokumente aktualisieren {#update-documents}

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

### Dokumente löschen {#delete-documents}

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

### Dokumente aggregieren {#aggregate-documents}

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

### Eine Collection anlegen {#create-a-collection}

```yaml
- type: mongo
  uri: mongodb://localhost:27017
  database: my-database
  collection: my-collection
  actions:
    - type: createCollection
```

### Eine Collection löschen {#drop-a-collection}

```yaml
- type: mongo
  uri: mongodb://localhost:27017
  database: my-database
  collection: my-collection
  actions:
    - type: dropCollection
```
