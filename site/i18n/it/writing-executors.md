---
translates: writing-executors
title: Scrivere executor
description: Come scrivere un nuovo executor di venom.
fingerprint: 5657ec73882903ce4f26906bdc21e94ded3d4d9616874ddcc1f4496b8bc206c1
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
## Venom Executor {#venom-executor}

### Scrivi il tuo executor {#write-your-executor}

Un executor deve implementare questa interfaccia

```go

// Executor executes a testStep.
type Executor interface {
	// Run runs a Test Step
	Run(ctx context.Content, TestStep) (interface{}, error)
}
```

Esempio

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

Sentiti libero di aprire una Pull Request con i tuoi executor.

### TLS {#tls}

Sei executor costruiscono da soli la propria connessione TLS: http, grpc, kafka, imap, smtp e ovhapi. Leggono le loro
impostazioni TLS allo stesso modo. Ogni executor mantiene il proprio interruttore e il proprio valore predefinito.

| Executor | TLS è attivo quando | Saltare il controllo del server | CA radice | Certificato e chiave client | Certificato del server controllato per impostazione predefinita |
|---|---|---|---|---|---|
| http | `url` inizia con `https://` | `ignore_verify_ssl: true` | `tls_root_ca` | `tls_client_cert`, `tls_client_key` | sì |
| grpc | `tls_root_ca` è impostato | `ignore_verify_ssl: true` | `tls_root_ca` | `tls_client_cert`, `tls_client_key` | sì |
| kafka | `with_tls: true` (o `withTLS`) | `insecure_tls: true` (o `insecureTLS`) | `tls_root_ca` | `tls_client_cert`, `tls_client_key` | sì |
| imap | `auth.withtls: true` | lascia `auth.verifytls` non impostato o `false` | `auth.tls_root_ca` | `auth.tls_client_cert`, `auth.tls_client_key` | no: imposta `auth.verifytls: true` |
| smtp | `withtls: true` | lascia `verifytls` non impostato o `false` | `tls_root_ca` | `tls_client_cert`, `tls_client_key` | no: imposta `verifytls: true` |
| ovhapi | `endpoint` è `https://` (lo sono gli endpoint con nome, come `ovh-eu`) | nessuno | `tlsRootCA` o `tls_root_ca`, altrimenti la variabile `ovh.tlsRootCA` | nessuno: l'API di OVH non ne usa uno | sì |

Le regole che hanno in comune:

- Una CA radice, un certificato client o una chiave è il testo PEM stesso oppure il percorso di un file PEM. Un percorso relativo
  viene letto a partire dalla directory della test suite. Più CA si possono concatenare in un'unica CA radice.
- La CA radice è considerata attendibile in aggiunta alle radici di sistema, quindi un server firmato da una CA privata viene verificato
  senza saltare il controllo.
- Saltare il controllo del server rende irrilevante la CA radice, ma il certificato client viene comunque inviato.
- Un certificato client senza la sua chiave, o una chiave senza il suo certificato, fa fallire lo step con un errore che indica
  l'impostazione mancante. Vale anche quando TLS è disattivato.
- Su grpc, kafka, imap e smtp, una CA radice o un certificato client indicati mentre TLS è disattivato non vengono usati: la connessione
  resta in chiaro e venom scrive un avviso nel log. Su http e ovhapi decide l'URL: con un URL `http://` venom scrive un
  avviso anche in questo caso, che dice che un reindirizzamento a `https://` o un proxy `https://` (il `proxy` dello step, `ovh.proxy` o
  `HTTPS_PROXY`/`HTTP_PROXY`) li usa comunque (http segue i reindirizzamenti a meno che `no_follow_redirect` sia impostato). Un nome di endpoint ovhapi come `ovh-eu` è un URL `https://` e non genera mai avvisi.
- kafka legge entrambe le grafie dei suoi due interruttori: `with_tls` o `withTLS`, `insecure_tls` o `insecureTLS`. Una qualsiasi
  delle due attiva l'interruttore.
- ovhapi rifiuta uno step che imposta sia `tlsRootCA` sia `tls_root_ca`.

Fuori ambito: gli executor il cui TLS arriva da una stringa di connessione o da un driver (sql, mongo, redis, amqp, rabbitmq,
mqtt, couchbase) e l'executor web, che ignora sempre gli errori dei certificati HTTPS (vedi il suo README).

`executors/tls_conformance_test.go` esegue gli stessi casi su tutti e sei gli executor, contro un'unica CA usa e getta: una CA
attendibile, una CA che non ha firmato il certificato del server, un controllo saltato con un certificato client e un certificato
senza la sua chiave.
