---
translates: couchbase
title: couchbase
description: Come usare l'executor couchbase in una test suite di venom.
fingerprint: e731f1a0163fb3e5b6411c0cd94c7b4bfebdc321ff26413506ce63f0145d372f
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# Venom - Executor couchbase {#venom---executor-couchbase}

Step per eseguire azioni su un database couchbase tramite l'sdk [github.com/couchbase/gocb/v2](https://pkg.go.dev/github.com/couchbase/gocb/v2).

Trovi maggiori informazioni [qui](https://docs.couchbase.com/go-sdk/current/hello-world/start-using-sdk.html)

## Connessione {#connection}

Per usare l'executor couchbase devi indicare almeno il bucket couchbase (nel caso di nessuna autenticazione e di esecuzione in locale).

Una configurazione couchbase minima può essere la seguente:

```yaml
- type: couchbase
  dsn:  "couchbase://localhost"         # connection string to connect to couchbase.
  username: "{{ .couchbase_username }}" # can be omitted
  password: "{{ .couchbase_password }}" # can be omitted
  bucket: "{{ .couchbase_bucket }}"     # mandatory
```

Una configurazione couchbase completa può essere la seguente

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

I campi `bucket`, `scope` e `collection` sono usati come valori predefiniti. Ogni azione può definire un bucket, uno scope o una collection diversi da usare localmente.

Per il transcoder hai le seguenti opzioni:

- json: usa [gocb.JSONTranscoder](https://pkg.go.dev/github.com/couchbase/gocb/v2#JSONTranscoder) (è il transcoder predefinito di gocb)
- raw: usa [gocb.RawBinaryTranscoder](https://pkg.go.dev/github.com/couchbase/gocb/v2#RawBinaryTranscoder)
- rawjson: usa [gocb.RawJSONTranscoder](https://pkg.go.dev/github.com/couchbase/gocb/v2#RawJSONTranscoder)
- rawstring: usa [gocb.RawStringTranscoder](https://pkg.go.dev/github.com/couchbase/gocb/v2#RawStringTranscoder)
- legacy: usa [gocb.LegacyTranscoder](https://pkg.go.dev/github.com/couchbase/gocb/v2#LegacyTranscoder) (usato per impostazione predefinita dall'executor couchbase di venom)

## Input {#input}

Vedi anche [tests/couchbase.yml](../../tests/couchbase.yml) per esempi eseguibili.

Couchbase può essere usato per memorizzare dati strutturati JSON, ma anche tipi binari e stringhe.

Esempio:

```yaml
airline_01: # entries are specified as map id => content
  "id": 1
  "type": "airline"
  "name": "first airline"
  "country": "Latveria"
```

### Carica le fixture {#load-fixtures}

Non ancora implementato.

### Recupera documenti {#retrieve-documents}

Get recupera i documenti per id. Se l'id non viene trovato, il flag `found` sarà false.

Con il flag `with_expiry` viene restituita la scadenza della voce indicata (oppure zero se non ha scadenza).

Con il flag `expiry` l'operazione get diventa `get and touch` e l'opzione `with_expiry` viene ignorata.

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

### Inserisci documenti {#insert-documents}

Insert crea i documenti per id. Se l'id esiste già, l'operazione non viene eseguita e il flag `inserted` sarà false.

Si può definire una scadenza, in secondi, con il campo `expiry` per sovrascrivere l'`expiry` _predefinito_ (se presente)

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

### Sostituisci documenti {#replace-documents}

Replace aggiorna i documenti per id. Se l'id non esiste, l'operazione non viene eseguita e il flag `replaced` sarà false.

È possibile mantenere la scadenza originale con il campo booleano `preserve_expiry`.
Si può definire una scadenza, in secondi, con il campo `expiry` per sovrascrivere l'`expiry` _predefinito_ (se presente).

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

### Upsert di documenti {#upsert-documents}

Upsert inserisce o aggiorna i documenti per id. Imposta il flag `upserted` a true.

È possibile mantenere la scadenza originale (se presente) con il campo booleano `preserve_expiry`.

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

### Rimuovi documenti {#remove-documents}

Delete rimuove per id. Imposta il flag `deleted` a true nel caso in cui l'id sia stato trovato.

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

### Aggiorna la scadenza dei documenti {#touch-documents}

Touch aggiorna la scadenza dell'oggetto per un dato id. Imposta il flag `touch` a true nel caso in cui l'id sia stato trovato.

Si può indicare un `expiry` in secondi da usare. Altrimenti viene usato l'`expiry` globale.

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

### Verifica se un documento esiste {#check-if-document-exists}

Il comando `exists` verifica se l'id del documento esiste o no. Imposta un flag booleano `found` con l'esito.

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
