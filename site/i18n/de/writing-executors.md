---
translates: writing-executors
title: Executors schreiben
description: Einen neuen venom-executor schreiben.
fingerprint: 5657ec73882903ce4f26906bdc21e94ded3d4d9616874ddcc1f4496b8bc206c1
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
## Venom-Executor {#venom-executor}

### Ihren executor schreiben {#write-your-executor}

Ein executor muss diese Schnittstelle implementieren

```go

// Executor executes a testStep.
type Executor interface {
	// Run runs a Test Step
	Run(ctx context.Content, TestStep) (interface{}, error)
}
```

Beispiel

```go


// Name of executor
const Name = "myexecutor"

// New returns a new Executor
func New() venom.Executor {
	return &Executor{}
}

// Executor struct
type Executor struct {
	Command string `json:"command,omitempty" yaml:"command,omitempty"`
}

// Result represents a step result
type Result struct {
	Code        int    `json:"code,omitempty" yaml:"code,omitempty"`
	Command     string `json:"command,omitempty" yaml:"command,omitempty"`
	Systemout   string   `json:"systemout,omitempty" yaml:"systemout,omitempty"` // put in testcase.Systemout by venom if present
	Systemerr   string   `json:"systemerr,omitempty" yaml:"systemerr,omitempty"` // put in testcase.Systemerr by venom if present
}

// GetDefaultAssertions returns the default assertions for this executor
// Optional
func (Executor) GetDefaultAssertions() *venom.StepAssertions {
	return &venom.StepAssertions{Assertions: []venom.Assertion{"result.code ShouldEqual 0"}}
}

// Run executes TestStep
func (Executor)	Run(ctx context.Context, step venom.TestStep) (interface{}, error) {
	// transform step to Executor Instance
	var e Executor
	if err := mapstructure.Decode(step, &e); err != nil {
		return nil, err
	}

	// to something with e.Command here...
	//...

	systemout := "foo"
	outputCode := 0

	// prepare result
	r := Result{
		Code:    outputCode, // return Output Code
		Command: e.Command, // return Command executed
		Systemout: systemout, // return Output string
	}

	return r, nil
}

```

Eröffnen Sie gern einen Pull Request mit Ihren executors.

### TLS {#tls}

Sechs executors bauen ihre eigene TLS-Verbindung auf: http, grpc, kafka, imap, smtp und ovhapi. Sie lesen ihre TLS-Einstellungen
auf dieselbe Weise. Jeder executor behält seinen eigenen Schalter und seinen eigenen Standardwert.

| Executor | TLS ist aktiv, wenn | Serverprüfung überspringen | Root-CA | Client-Zertifikat und -Schlüssel | Serverzertifikat standardmäßig geprüft |
|---|---|---|---|---|---|
| http | `url` beginnt mit `https://` | `ignore_verify_ssl: true` | `tls_root_ca` | `tls_client_cert`, `tls_client_key` | ja |
| grpc | `tls_root_ca` ist gesetzt | `ignore_verify_ssl: true` | `tls_root_ca` | `tls_client_cert`, `tls_client_key` | ja |
| kafka | `with_tls: true` (oder `withTLS`) | `insecure_tls: true` (oder `insecureTLS`) | `tls_root_ca` | `tls_client_cert`, `tls_client_key` | ja |
| imap | `auth.withtls: true` | `auth.verifytls` nicht setzen oder `false` | `auth.tls_root_ca` | `auth.tls_client_cert`, `auth.tls_client_key` | nein: `auth.verifytls: true` setzen |
| smtp | `withtls: true` | `verifytls` nicht setzen oder `false` | `tls_root_ca` | `tls_client_cert`, `tls_client_key` | nein: `verifytls: true` setzen |
| ovhapi | `endpoint` ist `https://` (die benannten Endpoints, etwa `ovh-eu`, sind es) | keine | `tlsRootCA` oder `tls_root_ca`, sonst die Variable `ovh.tlsRootCA` | keine: die OVH-API verwendet keines | ja |

Die Regeln, die sie gemeinsam haben:

- Eine Root-CA, ein Client-Zertifikat oder ein Schlüssel ist entweder der PEM-Text selbst oder der Pfad einer PEM-Datei. Ein relativer Pfad
  wird ab dem Verzeichnis der Test-Suite gelesen. Mehrere CAs können in einer Root-CA aneinandergehängt werden.
- Der Root-CA wird zusätzlich zu den System-Roots vertraut, sodass ein von einer privaten CA signierter Server geprüft wird, ohne
  die Prüfung zu überspringen.
- Wird die Serverprüfung übersprungen, spielt die Root-CA keine Rolle mehr, das Client-Zertifikat wird aber weiterhin gesendet.
- Ein Client-Zertifikat ohne seinen Schlüssel oder ein Schlüssel ohne sein Zertifikat lässt den Schritt mit einem Fehler fehlschlagen, der die
  fehlende Einstellung nennt. Das gilt auch, wenn TLS aus ist.
- Bei grpc, kafka, imap und smtp wird eine Root-CA oder ein Client-Zertifikat, die bei ausgeschaltetem TLS angegeben sind, nicht verwendet: Die Verbindung
  bleibt unverschlüsselt, und venom protokolliert eine Warnung. Bei http und ovhapi entscheidet die URL: Bei einer `http://`-URL protokolliert venom ebenfalls eine
  Warnung, die besagt, dass eine Weiterleitung auf `https://` oder ein `https://`-Proxy (`proxy` des Schritts, `ovh.proxy` oder
  `HTTPS_PROXY`/`HTTP_PROXY`) sie dennoch verwendet (http folgt Weiterleitungen, solange `no_follow_redirect` nicht gesetzt ist). Ein ovhapi-Endpoint-Name wie `ovh-eu` ist eine `https://`-URL und warnt nie.
- kafka liest beide Schreibweisen seiner zwei Schalter: `with_tls` oder `withTLS`, `insecure_tls` oder `insecureTLS`. Jede
  von beiden schaltet den jeweiligen Schalter ein.
- ovhapi weist einen Schritt zurück, der `tlsRootCA` und `tls_root_ca` zugleich setzt.

Nicht im Umfang: die executors, deren TLS aus einer Verbindungszeichenfolge oder einem Treiber stammt (sql, mongo, redis, amqp, rabbitmq,
mqtt, couchbase), und der web-executor, der HTTPS-Zertifikatsfehler immer ignoriert (siehe dessen README).

`executors/tls_conformance_test.go` führt dieselben Fälle durch alle sechs executors gegen eine Wegwerf-CA aus: eine vertrauenswürdige
CA, eine CA, die das Serverzertifikat nicht signiert hat, eine übersprungene Prüfung mit Client-Zertifikat und ein Zertifikat
ohne seinen Schlüssel.
