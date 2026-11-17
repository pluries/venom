---
translates: couchbase
title: couchbase
description: So verwenden Sie den couchbase-executor in einer venom-Test-Suite.
fingerprint: e731f1a0163fb3e5b6411c0cd94c7b4bfebdc321ff26413506ce63f0145d372f
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# Venom - Executor couchbase {#venom---executor-couchbase}

Schritt zum Ausführen von Aktionen auf einer couchbase-Datenbank mit dem SDK [github.com/couchbase/gocb/v2](https://pkg.go.dev/github.com/couchbase/gocb/v2).

Weitere Informationen finden Sie [hier](https://docs.couchbase.com/go-sdk/current/hello-world/start-using-sdk.html)

## Verbindung {#connection}

Um den couchbase-executor zu verwenden, müssen Sie mindestens den couchbase-Bucket angeben (bei lokalem Betrieb ohne Authentifizierung).

Eine minimale couchbase-Konfiguration kann so aussehen:

```yaml
- type: couchbase
  dsn:  "couchbase://localhost"         # connection string to connect to couchbase.
  username: "{{ .couchbase_username }}" # can be omitted
  password: "{{ .couchbase_password }}" # can be omitted
  bucket: "{{ .couchbase_bucket }}"     # mandatory
```

Eine vollständige couchbase-Konfiguration kann so aussehen

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

Die Felder `bucket`, `scope` und `collection` dienen als Standardwerte. Jede Aktion kann lokal einen anderen Bucket, Scope oder eine andere Collection festlegen.

Für den Transcoder stehen die folgenden Optionen zur Verfügung:

- json: verwendet [gocb.JSONTranscoder](https://pkg.go.dev/github.com/couchbase/gocb/v2#JSONTranscoder) (der Standard-Transcoder von gocb)
- raw: verwendet [gocb.RawBinaryTranscoder](https://pkg.go.dev/github.com/couchbase/gocb/v2#RawBinaryTranscoder)
- rawjson: verwendet [gocb.RawJSONTranscoder](https://pkg.go.dev/github.com/couchbase/gocb/v2#RawJSONTranscoder)
- rawstring: verwendet [gocb.RawStringTranscoder](https://pkg.go.dev/github.com/couchbase/gocb/v2#RawStringTranscoder)
- legacy: verwendet [gocb.LegacyTranscoder](https://pkg.go.dev/github.com/couchbase/gocb/v2#LegacyTranscoder) (im venom-couchbase-executor standardmäßig verwendet)

## Input {#input}

Ausführbare Beispiele finden Sie auch in [tests/couchbase.yml](../../tests/couchbase.yml).

Couchbase kann JSON-strukturierte Daten speichern, aber auch binäre Daten und Strings.

Beispiel:

```yaml
airline_01: # entries are specified as map id => content
  "id": 1
  "type": "airline"
  "name": "first airline"
  "country": "Latveria"
```

### Fixtures laden {#load-fixtures}

Noch nicht implementiert.

### Dokumente abrufen {#retrieve-documents}

Get ruft Dokumente anhand ihrer ID ab. Wird die ID nicht gefunden, ist das Flag `found` false.

Mit dem Flag `with_expiry` wird der Ablauf des jeweiligen Eintrags zurückgegeben (oder null, wenn er nicht abläuft).

Mit dem Flag `expiry` wird die Get-Operation zu `get and touch`, und die Option `with_expiry` wird ignoriert.

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

### Dokumente einfügen {#insert-documents}

Insert legt Dokumente anhand ihrer ID an. Existiert die ID bereits, wird die Operation nicht ausgeführt, und das Flag `inserted` ist false.

Über das Feld `expiry` lässt sich ein Ablauf in Sekunden festlegen, der den _Standardwert_ von `expiry` (falls vorhanden) überschreibt

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

### Dokumente ersetzen {#replace-documents}

Replace aktualisiert Dokumente anhand ihrer ID. Existiert die ID nicht, wird die Operation nicht ausgeführt, und das Flag `replaced` ist false.

Über das boolesche Feld `preserve_expiry` lässt sich der ursprüngliche Ablauf beibehalten.
Über das Feld `expiry` lässt sich ein Ablauf in Sekunden festlegen, der den _Standardwert_ von `expiry` (falls vorhanden) überschreibt.

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

### Dokumente per Upsert schreiben {#upsert-documents}

Upsert fügt Dokumente anhand ihrer ID ein oder aktualisiert sie. Es setzt das Flag `upserted` auf true.

Über das boolesche Feld `preserve_expiry` lässt sich der ursprüngliche Ablauf (falls vorhanden) beibehalten.

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

### Dokumente entfernen {#remove-documents}

Delete entfernt anhand der ID. Es setzt das Flag `deleted` auf true, falls die ID gefunden wurde.

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

### Dokumente per Touch aktualisieren {#touch-documents}

Touch aktualisiert den Ablauf des Objekts für eine ID. Es setzt das Flag `touch` auf true, falls die ID gefunden wurde.

Es lässt sich ein `expiry` in Sekunden angeben. Andernfalls wird das globale `expiry` verwendet.

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

### Prüfen, ob ein Dokument existiert {#check-if-document-exists}

Der Befehl `exists` prüft, ob die Dokument-ID existiert oder nicht. Er setzt ein boolesches Flag `found` mit dem Ergebnis.

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
