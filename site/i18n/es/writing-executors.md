---
translates: writing-executors
title: Escribir executors
description: Cómo escribir un nuevo executor de venom.
fingerprint: 5657ec73882903ce4f26906bdc21e94ded3d4d9616874ddcc1f4496b8bc206c1
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
## Executor de Venom {#venom-executor}

### Escriba su executor {#write-your-executor}

Un executor debe implementar esta interfaz

```go

// Executor executes a testStep.
type Executor interface {
	// Run runs a Test Step
	Run(ctx context.Content, TestStep) (interface{}, error)
}
```

Ejemplo

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

Puede abrir una Pull Request con sus executors.

### TLS {#tls}

Seis executors crean su propia conexión TLS: http, grpc, kafka, imap, smtp y ovhapi. Todos leen sus ajustes de TLS
de la misma manera. Cada executor conserva su propio interruptor y su propio valor por defecto.

| Executor | TLS está activado cuando | Omitir la comprobación del servidor | CA raíz | Certificado y clave de cliente | Certificado del servidor comprobado por defecto |
|---|---|---|---|---|---|
| http | `url` empieza por `https://` | `ignore_verify_ssl: true` | `tls_root_ca` | `tls_client_cert`, `tls_client_key` | sí |
| grpc | `tls_root_ca` está definido | `ignore_verify_ssl: true` | `tls_root_ca` | `tls_client_cert`, `tls_client_key` | sí |
| kafka | `with_tls: true` (o `withTLS`) | `insecure_tls: true` (o `insecureTLS`) | `tls_root_ca` | `tls_client_cert`, `tls_client_key` | sí |
| imap | `auth.withtls: true` | deje `auth.verifytls` sin definir o en `false` | `auth.tls_root_ca` | `auth.tls_client_cert`, `auth.tls_client_key` | no: defina `auth.verifytls: true` |
| smtp | `withtls: true` | deje `verifytls` sin definir o en `false` | `tls_root_ca` | `tls_client_cert`, `tls_client_key` | no: defina `verifytls: true` |
| ovhapi | `endpoint` es `https://` (los endpoints con nombre, como `ovh-eu`, lo son) | ninguno | `tlsRootCA` o `tls_root_ca`, o bien la variable `ovh.tlsRootCA` | ninguno: la API de OVH no usa uno | sí |

Las reglas que comparten:

- Una CA raíz, un certificado de cliente o una clave es el propio texto PEM o la ruta de un archivo PEM. Una ruta relativa
  se lee desde el directorio de la suite de pruebas. Pueden concatenarse varias CA en una sola CA raíz.
- Se confía en la CA raíz además de en las raíces del sistema, de modo que un servidor firmado por una CA privada se verifica sin omitir
  la comprobación.
- Omitir la comprobación del servidor hace irrelevante la CA raíz, pero el certificado de cliente se sigue enviando.
- Un certificado de cliente sin su clave, o una clave sin su certificado, hace fallar el paso con un error que nombra el
  ajuste que falta. Esto se cumple incluso con TLS desactivado.
- En grpc, kafka, imap y smtp, una CA raíz o un certificado de cliente indicados con TLS desactivado no se usan: la conexión
  sigue en texto plano y venom registra una advertencia. En http y ovhapi decide la URL: con una URL `http://` venom registra también una
  advertencia, que indica que una redirección a `https://` o un proxy `https://` (el `proxy` del paso, `ovh.proxy` o
  `HTTPS_PROXY`/`HTTP_PROXY`) siguen usándolos (http sigue las redirecciones salvo que se defina `no_follow_redirect`). Un nombre de endpoint de ovhapi como `ovh-eu` es una URL `https://` y nunca genera advertencia.
- kafka lee las dos grafías de sus dos interruptores: `with_tls` o `withTLS`, `insecure_tls` o `insecureTLS`. Cualquiera
  de ellas activa su interruptor.
- ovhapi rechaza un paso que defina a la vez `tlsRootCA` y `tls_root_ca`.

Fuera de alcance: los executors cuyo TLS procede de una cadena de conexión o de un driver (sql, mongo, redis, amqp, rabbitmq,
mqtt, couchbase) y el executor web, que siempre ignora los errores de certificado HTTPS (consulte su README).

`executors/tls_conformance_test.go` ejecuta los mismos casos en los seis executors contra una única CA desechable: una CA
de confianza, una CA que no firmó el certificado del servidor, una comprobación omitida con un certificado de cliente y un certificado
sin su clave.
