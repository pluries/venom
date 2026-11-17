---
translates: couchbase
title: couchbase
description: Cómo usar el executor couchbase en una suite de pruebas de venom.
fingerprint: e731f1a0163fb3e5b6411c0cd94c7b4bfebdc321ff26413506ce63f0145d372f
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# Venom - Executor couchbase {#venom---executor-couchbase}

Paso para ejecutar acciones en una base de datos couchbase mediante el SDK [github.com/couchbase/gocb/v2](https://pkg.go.dev/github.com/couchbase/gocb/v2).

Puede encontrar más información [aquí](https://docs.couchbase.com/go-sdk/current/hello-world/start-using-sdk.html)

## Connection {#connection}

Para usar el executor couchbase debe indicar al menos el bucket de couchbase (en caso de no haber autenticación y de ejecución local).

Una configuración mínima de couchbase puede ser la siguiente:

```yaml
- type: couchbase
  dsn:  "couchbase://localhost"         # connection string to connect to couchbase.
  username: "{{ .couchbase_username }}" # can be omitted
  password: "{{ .couchbase_password }}" # can be omitted
  bucket: "{{ .couchbase_bucket }}"     # mandatory
```

Una configuración completa de couchbase puede ser la siguiente

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

Los campos `bucket`, `scope` y `collection` se usan como valores por defecto. Cada acción puede definir un bucket, un scope o una collection distintos para usarlos localmente.

Para transcoder, dispone de las siguientes opciones:

- json: usa [gocb.JSONTranscoder](https://pkg.go.dev/github.com/couchbase/gocb/v2#JSONTranscoder) (es el transcoder por defecto de gocb)
- raw: usa [gocb.RawBinaryTranscoder](https://pkg.go.dev/github.com/couchbase/gocb/v2#RawBinaryTranscoder)
- rawjson: usa [gocb.RawJSONTranscoder](https://pkg.go.dev/github.com/couchbase/gocb/v2#RawJSONTranscoder)
- rawstring: usa [gocb.RawStringTranscoder](https://pkg.go.dev/github.com/couchbase/gocb/v2#RawStringTranscoder)
- legacy: usa [gocb.LegacyTranscoder](https://pkg.go.dev/github.com/couchbase/gocb/v2#LegacyTranscoder) (se usa por defecto en el executor couchbase de venom)

## Input {#input}

Consulte también [tests/couchbase.yml](../../tests/couchbase.yml) para ver ejemplos ejecutables.

Couchbase puede usarse para almacenar datos con estructura JSON, pero también puede almacenar tipos binarios y de cadena.

Ejemplo:

```yaml
airline_01: # entries are specified as map id => content
  "id": 1
  "type": "airline"
  "name": "first airline"
  "country": "Latveria"
```

### Load fixtures {#load-fixtures}

Aún no está implementado.

### Retrieve documents {#retrieve-documents}

Get recupera documentos por id. Si no se encuentra el id, el indicador `found` será false.

Con el indicador `with_expiry` se devuelve la expiración de la entrada indicada (o cero si no tiene expiración).

Con el indicador `expiry`, la operación get pasa a ser `get and touch` y la opción `with_expiry` se ignora.

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

Insert crea documentos por id. Si el id ya existe, la operación no se ejecuta y el indicador `inserted` será false.

Puede definirse una expiración, en segundos, mediante el campo `expiry` para sustituir el `expiry` _por defecto_ (si lo hay)

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

Replace actualiza documentos por id. Si el id no existe, la operación no se ejecuta y el indicador `replaced` será false.

Es posible conservar la expiración original mediante el campo booleano `preserve_expiry`.
Puede definirse una expiración, en segundos, mediante el campo `expiry` para sustituir el `expiry` _por defecto_ (si lo hay).

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

Upsert inserta o actualiza documentos por id. Establece el indicador `upserted` en true.

Es posible conservar la expiración original (si la hay) mediante el campo booleano `preserve_expiry`.

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

Delete elimina por id. Establece el indicador `deleted` en true en caso de que se haya encontrado el id.

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

Touch actualiza la expiración del objeto para un id determinado. Establece el indicador `touch` en true en caso de que se haya encontrado el id.

Puede especificarse un `expiry` en segundos para usarlo. En caso contrario se usará el `expiry` global.

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

El comando `exists` comprueba si el id del documento existe o no. Establece un indicador booleano `found` con el resultado.

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
