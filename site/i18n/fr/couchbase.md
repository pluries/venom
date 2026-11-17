---
translates: couchbase
title: couchbase
description: Comment utiliser l'executor couchbase dans une suite de tests venom.
fingerprint: e731f1a0163fb3e5b6411c0cd94c7b4bfebdc321ff26413506ce63f0145d372f
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# Venom - Executor couchbase {#venom---executor-couchbase}

Étape pour exécuter des actions sur une base couchbase avec le SDK [github.com/couchbase/gocb/v2](https://pkg.go.dev/github.com/couchbase/gocb/v2).

Vous trouverez plus d'informations [ici](https://docs.couchbase.com/go-sdk/current/hello-world/start-using-sdk.html)

## Connection {#connection}

Pour utiliser l'executor couchbase, vous devez indiquer au moins le bucket couchbase (en l'absence d'authentification et en local).

Une configuration couchbase minimale peut être la suivante :

```yaml
- type: couchbase
  dsn:  "couchbase://localhost"         # connection string to connect to couchbase.
  username: "{{ .couchbase_username }}" # can be omitted
  password: "{{ .couchbase_password }}" # can be omitted
  bucket: "{{ .couchbase_bucket }}"     # mandatory
```

Une configuration couchbase complète peut être la suivante

```yaml
- type: couchbase
  dsn:  "couchbase://localhost"         # will use couchbase://localhost if omitted
  username: "{{ .couchbase_username }}" # can be omitted
  password: "{{ .couchbase_password }}" # can be omitted
  bucket: "{{ .couchbase_bucket }}"     # mandatory
  scope:  "_default"                    # can be omitted, default scope will be used
  collection: "_default"                # can be omitted, default collection will be used
  transcoder: "json"                    # can be omitted, "legacy" will be used
  expiry: 600                           # in seconds, can be omitted (no expiration by default)
  wait_until_ready_timeout: 5           # in seconds, can be 0 (means no wait until bucket is ready)
  profile_wan_development: true         # if true, will use high timeouts to avoid latency issues. default false.
```

Les champs `bucket`, `scope` et `collection` servent de valeurs par défaut. Chaque action peut définir localement un autre bucket, scope ou collection.

Pour le transcoder, les options sont les suivantes :

- json : utilise [gocb.JSONTranscoder](https://pkg.go.dev/github.com/couchbase/gocb/v2#JSONTranscoder) (c'est le transcoder par défaut de gocb)
- raw : utilise [gocb.RawBinaryTranscoder](https://pkg.go.dev/github.com/couchbase/gocb/v2#RawBinaryTranscoder)
- rawjson : utilise [gocb.RawJSONTranscoder](https://pkg.go.dev/github.com/couchbase/gocb/v2#RawJSONTranscoder)
- rawstring : utilise [gocb.RawStringTranscoder](https://pkg.go.dev/github.com/couchbase/gocb/v2#RawStringTranscoder)
- legacy : utilise [gocb.LegacyTranscoder](https://pkg.go.dev/github.com/couchbase/gocb/v2#LegacyTranscoder) (utilisé par défaut dans l'executor couchbase de venom)

## Input {#input}

Voir aussi [tests/couchbase.yml](../../tests/couchbase.yml) pour des exemples exécutables.

Couchbase peut stocker des données structurées en JSON, mais aussi des types binaires et des chaînes.

Exemple :

```yaml
airline_01: # entries are specified as map id => content
  "id": 1
  "type": "airline"
  "name": "first airline"
  "country": "Latveria"
```

### Load fixtures {#load-fixtures}

Pas encore implémenté.

### Retrieve documents {#retrieve-documents}

Get récupère des documents par id. Si l'id est introuvable, le drapeau `found` vaut false.

Avec le drapeau `with_expiry`, l'expiration de l'entrée demandée est renvoyée (ou zéro s'il n'y a pas d'expiration).

Avec le drapeau `expiry`, l'opération get devient un `get and touch` et l'option `with_expiry` est ignorée.

```yaml
- type: couchbase
  dsn:  "{{ .couchbase_dsn }}"
  username: "{{ .couchbase_username }}"
  password: "{{ .couchbase_password }}"
  bucket: "travel-sample"
  actions:
    - type: get
      with_expiry: true
      ids: ["airline_10","airline_01"]
    assertions:
      - result.actions.actions0.airline_10.expiry ShouldBeZeroValue
      - result.actions.actions0.airline_10.data ShouldJSONEqual {"id":10,"type":"airline","name":"40-Mile Air","iata":"Q5","icao":"MLA","callsign":"MILE-AIR","country":"United States"}
      - result.actions.actions0.airline_01 ShouldNotBeNil
      - result.actions.actions0.airline_01.found ShouldBeFalse
```

### Insert documents {#insert-documents}

Insert crée des documents par id. Si l'id existe déjà, l'opération n'est pas exécutée et le drapeau `inserted` vaut false.

Une expiration, en secondes, peut être définie avec le champ `expiry` pour remplacer l'`expiry` _par défaut_ (s'il existe).

```yaml
- type: couchbase
  dsn:  "{{ .couchbase_dsn }}"
  username: "{{ .couchbase_username }}"
  password: "{{ .couchbase_password }}"
  bucket: "travel-sample"
  actions:
    - type: insert
      entries:
        airline_01:
          "id": 1
          "type": "airline"
          "name": "first airline"
          "country": "Latveria"
  assertions:
    - result.actions.actions0.airline_01.inserted ShouldBeTrue
```

### Replace documents {#replace-documents}

Replace met à jour des documents par id. Si l'id n'existe pas, l'opération n'est pas exécutée et le drapeau `replaced` vaut false.

Il est possible de conserver l'expiration d'origine avec le champ booléen `preserve_expiry`.
Une expiration, en secondes, peut être définie avec le champ `expiry` pour remplacer l'`expiry` _par défaut_ (s'il existe).

```yaml
- type: couchbase
  dsn:  "{{ .couchbase_dsn }}"
  username: "{{ .couchbase_username }}"
  password: "{{ .couchbase_password }}"
  bucket: "travel-sample"
  actions:
    - type: replace
      entries:
        airline_01:
          "id": 1
          "type": "airline"
          "name": "first airline"
          "country": "Latveria"
  assertions:
    - result.actions.actions0.airline_01.replaced ShouldBeTrue
```

### Upsert documents {#upsert-documents}

Upsert insère ou met à jour des documents par id. Il met le drapeau `upserted` à true.

Il est possible de conserver l'expiration d'origine (s'il y en a une) avec le champ booléen `preserve_expiry`.

```yaml
- type: couchbase
  dsn:  "{{ .couchbase_dsn }}"
  username: "{{ .couchbase_username }}"
  password: "{{ .couchbase_password }}"
  bucket: "travel-sample"
  actions:
    - type: upsert
      entries:
        airline_01:
          "id": 1
          "type": "airline"
          "name": "first airline"
          "country": "Latveria"
  assertions:
    - result.actions.actions0.airline_01.upserted ShouldBeTrue
```

### Remove documents {#remove-documents}

Delete supprime par id. Il met le drapeau `deleted` à true si l'id a été trouvé.

```yaml
- type: couchbase
  dsn:  "{{ .couchbase_dsn }}"
  username: "{{ .couchbase_username }}"
  password: "{{ .couchbase_password }}"
  bucket: "travel-sample"
  actions:
    - type: delete
      ids: ["airline_01"]
  assertions:
    - result.actions.actions0.airline_01.deleted ShouldBeTrue
```

### Touch documents {#touch-documents}

Touch met à jour l'expiration de l'objet pour un id donné. Il met le drapeau `touch` à true si l'id a été trouvé.

Vous pouvez indiquer un `expiry` en secondes. Sinon, c'est l'`expiry` global qui est utilisé.

```yaml
- type: couchbase
  dsn:  "{{ .couchbase_dsn }}"
  username: "{{ .couchbase_username }}"
  password: "{{ .couchbase_password }}"
  bucket: "travel-sample"
  actions:
    - type: touch
      expiry: 3600    # 1 hour
      ids: ["airline_01"]
  assertions:
    - result.actions.actions0.airline_01.touched ShouldBeTrue
```

### Check if document exists {#check-if-document-exists}

La commande `exists` vérifie si l'id du document existe ou non. Elle renseigne le drapeau booléen `found` avec ce statut.

```yaml
- type: couchbase
  dsn:  "{{ .couchbase_dsn }}"
  username: "{{ .couchbase_username }}"
  password: "{{ .couchbase_password }}"
  bucket: "travel-sample"
  actions:
    - type: exists
      ids: ["airline_10","airline_01"]
  assertions:
    - result.actions.actions0.airline_10.found ShouldBeTrue
    - result.actions.actions0.airline_01.found ShouldBeFalse
```
