---
translates: writing-executors
title: Écrire des executors
description: Comment écrire un nouvel executor venom.
fingerprint: 5657ec73882903ce4f26906bdc21e94ded3d4d9616874ddcc1f4496b8bc206c1
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
## Executor venom {#venom-executor}

### Écrire votre executor {#write-your-executor}

Un executor doit implémenter cette interface

```go

// Executor executes a testStep.
type Executor interface {
	// Run runs a Test Step
	Run(ctx context.Content, TestStep) (interface{}, error)
}
```

Exemple

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

N'hésitez pas à ouvrir une Pull Request avec vos executors.

### TLS {#tls}

Six executors construisent leur propre connexion TLS : http, grpc, kafka, imap, smtp et ovhapi. Ils lisent leurs réglages
TLS de la même manière. Chaque executor garde son propre interrupteur et sa propre valeur par défaut.

| Executor | TLS est activé quand | Ignorer la vérification du serveur | CA racine | Certificat et clé client | Certificat du serveur vérifié par défaut |
|---|---|---|---|---|---|
| http | `url` commence par `https://` | `ignore_verify_ssl: true` | `tls_root_ca` | `tls_client_cert`, `tls_client_key` | oui |
| grpc | `tls_root_ca` est renseigné | `ignore_verify_ssl: true` | `tls_root_ca` | `tls_client_cert`, `tls_client_key` | oui |
| kafka | `with_tls: true` (ou `withTLS`) | `insecure_tls: true` (ou `insecureTLS`) | `tls_root_ca` | `tls_client_cert`, `tls_client_key` | oui |
| imap | `auth.withtls: true` | laissez `auth.verifytls` non renseigné ou à `false` | `auth.tls_root_ca` | `auth.tls_client_cert`, `auth.tls_client_key` | non : renseignez `auth.verifytls: true` |
| smtp | `withtls: true` | laissez `verifytls` non renseigné ou à `false` | `tls_root_ca` | `tls_client_cert`, `tls_client_key` | non : renseignez `verifytls: true` |
| ovhapi | `endpoint` est en `https://` (les endpoints nommés, comme `ovh-eu`, le sont) | aucun | `tlsRootCA` ou `tls_root_ca`, sinon la variable `ovh.tlsRootCA` | aucun : l'API OVH n'en utilise pas | oui |

Les règles qu'ils partagent :

- Une CA racine, un certificat client ou une clé est soit le texte PEM lui-même, soit le chemin d'un fichier PEM. Un chemin relatif
  est lu depuis le répertoire de la testsuite. Plusieurs CA peuvent être concaténées dans une seule CA racine.
- La CA racine est reconnue en plus des racines du système, donc un serveur signé par une CA privée est vérifié sans qu'il faille
  ignorer la vérification.
- Ignorer la vérification du serveur rend la CA racine sans objet, mais le certificat client est quand même envoyé.
- Un certificat client sans sa clé, ou une clé sans son certificat, fait échouer l'étape avec une erreur qui nomme le
  réglage manquant. C'est vrai même quand TLS est désactivé.
- Sur grpc, kafka, imap et smtp, une CA racine ou un certificat client fourni alors que TLS est désactivé n'est pas utilisé : la connexion
  reste en clair et venom journalise un avertissement. Sur http et ovhapi, c'est l'URL qui décide : avec une URL `http://`, venom journalise aussi un
  avertissement, qui précise qu'une redirection vers `https://` ou un proxy `https://` (le `proxy` de l'étape, `ovh.proxy` ou
  `HTTPS_PROXY`/`HTTP_PROXY`) les utilise quand même (http suit les redirections sauf si `no_follow_redirect` est renseigné). Un nom d'endpoint ovhapi tel que `ovh-eu` est une URL `https://` et n'avertit jamais.
- kafka lit les deux orthographes de ses deux interrupteurs : `with_tls` ou `withTLS`, `insecure_tls` ou `insecureTLS`. L'une ou
  l'autre active l'interrupteur.
- ovhapi refuse une étape qui renseigne à la fois `tlsRootCA` et `tls_root_ca`.

Hors périmètre : les executors dont le TLS vient d'une chaîne de connexion ou d'un pilote (sql, mongo, redis, amqp, rabbitmq,
mqtt, couchbase) et l'executor web, qui ignore toujours les erreurs de certificat HTTPS (voir son README).

`executors/tls_conformance_test.go` exécute les mêmes cas sur les six executors avec une seule CA jetable : une CA
reconnue, une CA qui n'a pas signé le certificat du serveur, une vérification ignorée avec un certificat client, et un certificat
sans sa clé.
