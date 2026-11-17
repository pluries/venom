---
translates: mcp
title: MCP
description: Come usare l'executor MCP in una test suite di venom.
fingerprint: 8d470c5d14201b424e787037fbd21d3ce531fda24704df29703bb787038699e0
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# Venom - Executor MCP {#venom---executor-mcp}

Step per chiamare un server MCP (Model Context Protocol): elencare e chiamare i suoi tool, elencare e leggere le sue resource, elencare e ottenere i suoi prompt, e scrivere assertion sulla risposta.
Usalo per testare un server MCP da una suite venom, via stdio (venom avvia il server) o via HTTP streamable.

Dalla versione: v1.1.0 (prevista). L'executor è nuovo nella release successiva a v1.0.0.

Per verificare che funzioni: `tests/mcp.yml` lo esegue end to end su un server di riferimento (vedi «Esegui la suite»), e
`tests/realserver/filesystem.yml` lo esegue su un vero server di terze parti (vedi «Provalo su un server reale»).

## Input {#input}

```yaml
  - transport (mandatory): stdio or http
  - command (mandatory with stdio): the server command and its arguments, as a list
  - url (mandatory with http): the endpoint of the server
  - headers (optional, http): map of HTTP headers sent with every request, e.g. Authorization
  - method (mandatory): initialize, ping, tools/list, tools/call, resources/list, resources/read, prompts/list or prompts/get
  - tool (mandatory with tools/call): the name of the tool
  - uri (mandatory with resources/read): the absolute URI of the resource
  - prompt (mandatory with prompts/get): the name of the prompt
  - arguments (optional, tools/call and prompts/get): map of arguments
  - protocol_version (optional), default value: 2025-11-25
```

`tools/call` invia gli argomenti come JSON. `prompts/get` li invia come stringhe, come prevede il protocollo: le stringhe
restano tali, i numeri e i booleani vengono scritti per esteso (`3` diventa `"3"`, `true` diventa `"true"`), e un valore
annidato (mappa o lista) è un errore d'uso prima di qualsiasi connessione.

Un argomento di prompt `null` (`who:` senza valore) viene rifiutato come errore d'uso, come un valore annidato.

Assertion predefinita: `result.err ShouldBeEmpty`.

## Output {#output}

```yaml
  - result.content: the content blocks of a tool answer (tools/call); the text of each content item of a resource (resources/read), blobs left out
  - result.text: the text blocks of a tool answer, joined with a newline (tools/call); the texts of the resource items, joined (resources/read); the texts of the prompt messages, joined (prompts/get)
  - result.structured: the structured content of a tool answer, as JSON text (tools/call)
  - result.structuredjson: the same, parsed, so you can assert on result.structuredjson.<key>
  - result.tools: the tools, as JSON text (tools/list)
  - result.toolsjson: the same, parsed
  - result.resources: the resources, one entry each with uri, name, mimetype, description (resources/list); the content items, one entry each with uri, mimetype, text, blob (resources/read)
  - result.prompts: the prompts, one entry each with name, description, arguments (prompts/list); the messages, one entry each with role, text (prompts/get)
  - result.description: the description of the prompt (prompts/get)
  - result.serverinfo.name, result.serverinfo.version: what the server announced
  - result.capabilities: the capabilities the server announced
  - result.protocolversion: the protocol version the server answered with
  - result.iserror: true when a tool reported an error (tools/call)
  - result.err: the failure of the step, empty when it worked
  - result.timeseconds: duration of the step
```

### Quali chiavi riempie ogni metodo {#which-keys-each-method-fills}

`serverinfo`, `capabilities` e `protocolversion` vengono riempite da ogni metodo una volta aperta la connessione
(provengono dalla risposta di initialize). `iserror`, `err` e `timeseconds` esistono sempre. Una chiave senza valore è
assente dal risultato (un testo vuoto, una lista mancante). La tabella mostra le chiavi specifiche di ogni metodo.

| Metodo | Chiavi che riempie (oltre a quelle comuni sopra) |
|---|---|
| `initialize` | nessuna: le chiavi comuni sono l'intera risposta |
| `ping` | nessuna |
| `tools/list` | `tools`, `toolsjson` |
| `tools/call` | `content`, `text`, `structured`, `structuredjson` (quando il tool invia contenuto strutturato), `iserror` |
| `resources/list` | `resources` |
| `resources/read` | `resources`, `content`, `text` |
| `prompts/list` | `prompts` |
| `prompts/get` | `prompts`, `text`, `description` |

Se lo scambio fallisce dopo l'apertura della connessione, `err` viene valorizzata e le chiavi sopra possono mancare; le
chiavi comuni ci sono comunque. Se fallisce la connessione stessa, vengono valorizzate solo `err` e `timeseconds`.

Le liste tornano nell'ordine in cui le ha inviate il server (l'SDK Go ordina per URI o per nome). Scrivi un'assertion su una voce con
`result.resources.resources0.uri` o `result.prompts.prompts1.name`.

Un blob viene restituito come stringa base64 in `blob`, con `text` vuoto. Non c'è alcun limite di dimensione.

Contenuti e messaggi vengono restituiti così come li ha inviati il server. Dichiara in `secrets:` ciò che deve restare fuori dai report:
l'executor non esegue né inoltra mai quei dati, e la redazione del motore nasconde nei report i secret dichiarati.

## Limiti della versione 1 {#limits-of-version-1}

- Una connessione per step: nessuna sessione è condivisa tra gli step.
- Le richieste di sampling ed elicitation del server vengono rifiutate.
- Le notifiche del server vengono ignorate.
- I resource template (`resources/templates/list`) non sono supportati; sono un possibile sviluppo futuro.
- Nessun limite di dimensione sui blob: un blob grande viene tenuto in memoria e inserito nel risultato.

## Errori {#errors}

- Uno step sbagliato (transport sconosciuto, `command`, `url`, `tool`, `uri` o `prompt` mancante, metodo sconosciuto, argomento di prompt annidato) è un errore
  Go: lo step fallisce prima di qualsiasi connessione.
- Tutto ciò che va storto nello scambio (connessione impossibile, 401, timeout, errore JSON-RPC) viene messo in
  `result.err`; l'assertion predefinita fa allora fallire lo step con quel testo, e puoi scrivere un'assertion su di esso.
- Un tool che segnala un errore (`isError`) imposta `result.iserror` a true e `result.err` a
  `tool "<name>" returned an error: <text of the server>`. Con l'assertion predefinita lo step fallisce; per aspettarti
  l'errore, scrivi `result.iserror ShouldBeTrue`.

## Connessione, timeout, ambiente {#connection-timeout-environment}

- Una connessione per step: lo step si connette, esegue il suo metodo e chiude. Un server stdio viene avviato per lo step
  e fermato alla sua fine.
- Timeout: vale il `timeout` dello step (del motore); in sua assenza, l'executor usa 30 secondi.
- Il processo figlio stdio eredita l'intero ambiente di venom.
- Protocollo: `initialize` viene inviato con la versione 2025-11-25, salvo diversa indicazione di `protocol_version`, così una suite non
  cambia quando l'SDK cambia il proprio valore predefinito.

## Nota di sicurezza per stdio {#security-note-for-stdio}

`command` viene eseguito con i diritti della suite ed eredita l'intero ambiente di venom, token compresi. Per un server
di terze parti, avvialo con un ambiente pulito: `env -i PATH=... HOME=<tempdir> <server>`. La suite
`tests/realserver/filesystem.yml` fa così:

```yaml
    command: [env, -i, "PATH={{.prepare.nodebin}}:/usr/bin:/bin", "HOME={{.prepare.tmpdir}}", npx, -y, "@modelcontextprotocol/server-filesystem@2026.1.14", "{{.prepare.tmpdir}}"]
```

## Secret {#secrets}

Dichiara un token nei `secrets:` della suite e usalo in `headers`; venom lo nasconde nei log e nei report. L'executor
non riporta mai gli header nel suo risultato.

## Esempi {#examples}

```yaml
name: MCP testsuite
testcases:
- name: call a tool over stdio
  steps:
  - type: mcp
    transport: stdio
    command: ["/usr/local/bin/my-mcp-server", "--quiet"]
    method: tools/call
    tool: echo
    arguments:
      message: hello
    assertions:
    - result.err ShouldBeEmpty
    - result.text ShouldContainSubstring hello
    - result.structuredjson.message ShouldEqual hello
```

```yaml
name: MCP testsuite over HTTP
vars:
  token: changeme
secrets:
  - token
testcases:
- name: list the tools
  steps:
  - type: mcp
    transport: http
    url: http://127.0.0.1:8080/mcp
    headers:
      Authorization: Bearer {{.token}}
    method: tools/list
    assertions:
    - result.tools ShouldContainSubstring echo
```

```yaml
name: MCP resources and prompts
testcases:
- name: resources
  steps:
  - type: mcp
    transport: stdio
    command: ["/usr/local/bin/my-mcp-server"]
    method: resources/list
    assertions:
    - result.resources.resources0.uri ShouldEqual venom://text/hello
  - type: mcp
    transport: stdio
    command: ["/usr/local/bin/my-mcp-server"]
    method: resources/read
    uri: venom://text/hello
    assertions:
    - result.text ShouldEqual hello
    - result.content.content0 ShouldEqual hello
- name: prompts
  steps:
  - type: mcp
    transport: stdio
    command: ["/usr/local/bin/my-mcp-server"]
    method: prompts/list
    assertions:
    - result.prompts.prompts0.name ShouldEqual greet
  - type: mcp
    transport: stdio
    command: ["/usr/local/bin/my-mcp-server"]
    method: prompts/get
    prompt: greet
    arguments:
      who: venom
    assertions:
    - result.text ShouldEqual hello venom
    - result.description ShouldEqual A greeting
```

### Conservare un valore con `vars:` {#keep-a-value-with-vars}

Un valore del risultato può essere conservato per i test case successivi. Questo step, tratto da `tests/realserver/filesystem.yml`, conserva il
nome del server; i test case successivi lo leggono come `{{.initialize.servername}}`:

```yaml
- name: initialize
  steps:
  - type: mcp
    transport: stdio
    command: [env, -i, "PATH={{.prepare.nodebin}}:/usr/bin:/bin", "HOME={{.prepare.tmpdir}}", npx, -y, "@modelcontextprotocol/server-filesystem@2026.1.14", "{{.prepare.tmpdir}}"]
    method: initialize
    assertions:
    - result.err ShouldBeEmpty
    - result.serverinfo.name ShouldNotBeEmpty
    vars:
      servername:
        from: result.serverinfo.name
```

## Esegui la suite {#run-the-suite}

`tests/mcp.yml` compila il server di riferimento (`executors/mcp/internal/refserver`) con `go build` in una nuova
directory `mktemp -d` (binario, file dell'indirizzo e file del pid), quindi richiede la toolchain Go, e due esecuzioni contemporanee non
entrano in conflitto. L'ultimo test case rimuove la directory. Eseguila da sola, dalla directory `tests/`:
`venom run mcp.yml`. Niente docker, niente rete.
Le fixture manuali `tests/failing/mcp-iserror.yml` (un errore di tool) e `tests/failing/mcp-resource-unknown.yml`
(un URI di resource sconosciuto) mostrano i messaggi di errore. Ognuna compila il proprio server in una directory temporanea, la rimuove
ed è destinata a fallire: `cd tests && venom run failing/mcp-iserror.yml`.

## Provalo su un server reale {#try-it-against-a-real-server}

`tests/realserver/filesystem.yml` pilota un vero server di terze parti, `@modelcontextprotocol/server-filesystem`, fissato
alla versione 2026.1.14, via stdio. Sta fuori da `make run-test`: richiede la rete ed esegue codice di terze parti.
Eseguila a mano:

```bash
cd tests && venom run realserver/filesystem.yml
```

Servono `node`, `npx` e la rete npm (npx scarica il pacchetto al momento dell'esecuzione). La suite lavora in una directory
temporanea con un file sintetico e la rimuove alla fine.

L'esecuzione registrata è andata a buon fine (5 test case, 8 step). Il server si è presentato come `secure-filesystem-server` 0.2.0,
il protocollo era 2025-11-25, `tools/list` ha restituito 14 tool, e la suite ha usato `list_directory` e
`read_text_file`.
