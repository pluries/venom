---
translates: mcp
title: MCP
description: So verwenden Sie den MCP-executor in einer venom-Test-Suite.
fingerprint: 8d470c5d14201b424e787037fbd21d3ce531fda24704df29703bb787038699e0
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# Venom - Executor MCP {#venom---executor-mcp}

Schritt zum Aufrufen eines MCP-Servers (Model Context Protocol): seine Tools auflisten und aufrufen, seine Ressourcen auflisten und lesen, seine Prompts auflisten und abrufen und die Antwort mit Assertions prüfen.
Verwenden Sie ihn, um einen MCP-Server aus einer venom-Suite zu testen, über stdio (venom startet den Server) oder über Streamable HTTP.

Seit: v1.1.0 (geplant). Der executor ist neu in der Version nach v1.0.0.

Prüfen, ob er funktioniert: `tests/mcp.yml` führt ihn von Anfang bis Ende gegen einen Referenzserver aus (siehe "Run the suite"), und
`tests/realserver/filesystem.yml` führt ihn gegen einen echten Server eines Drittanbieters aus (siehe "Try it against a real server").

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

`tools/call` sendet die Argumente als JSON. `prompts/get` sendet sie als Strings, wie es das Protokoll vorschreibt: Strings
bleiben unverändert, Zahlen und Booleans werden ausgeschrieben (`3` wird zu `"3"`, `true` wird zu `"true"`), und ein
verschachtelter Wert (Map oder Liste) ist ein Anwendungsfehler, noch bevor eine Verbindung besteht.

Ein `null`-Prompt-Argument (`who:` ohne Wert) wird wie ein verschachtelter Wert als Anwendungsfehler abgelehnt.

Standard-Assertion: `result.err ShouldBeEmpty`.

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

### Welche Schlüssel jede Methode füllt {#which-keys-each-method-fills}

`serverinfo`, `capabilities` und `protocolversion` werden von jeder Methode gefüllt, sobald die Verbindung offen ist
(sie stammen aus der Antwort auf initialize). `iserror`, `err` und `timeseconds` sind immer vorhanden. Ein Schlüssel ohne Wert fehlt im
Ergebnis (ein leerer Text, eine fehlende Liste). Die Tabelle zeigt die Schlüssel, die für die jeweilige Methode spezifisch sind.

| Methode | Schlüssel, die sie füllt (außer den oben genannten gemeinsamen) |
|---|---|
| `initialize` | keine: Die gemeinsamen Schlüssel sind die ganze Antwort |
| `ping` | keine |
| `tools/list` | `tools`, `toolsjson` |
| `tools/call` | `content`, `text`, `structured`, `structuredjson` (wenn das Tool strukturierten Inhalt sendet), `iserror` |
| `resources/list` | `resources` |
| `resources/read` | `resources`, `content`, `text` |
| `prompts/list` | `prompts` |
| `prompts/get` | `prompts`, `text`, `description` |

Schlägt der Austausch nach dem Öffnen der Verbindung fehl, wird `err` gesetzt, und die obigen Schlüssel können fehlen; die gemeinsamen
Schlüssel sind weiterhin vorhanden. Schlägt die Verbindung selbst fehl, werden nur `err` und `timeseconds` gesetzt.

Listen kommen in der Reihenfolge zurück, in der der Server sie gesendet hat (das Go-SDK sortiert nach URI oder Name). Prüfen Sie einen Eintrag mit
`result.resources.resources0.uri` oder `result.prompts.prompts1.name`.

Ein Blob wird als Base64-String in `blob` zurückgegeben, wobei `text` leer bleibt. Es gibt keine Größenbegrenzung.

Inhalte und Nachrichten werden so zurückgegeben, wie der Server sie gesendet hat. Deklarieren Sie in `secrets:`, was nicht in Berichte gelangen darf:
Der executor führt diese Daten nie aus und leitet sie nie weiter, und die Schwärzung der Engine verbirgt deklarierte Secrets in Berichten.

## Grenzen der Version 1 {#limits-of-version-1}

- Eine Verbindung pro Schritt: Zwischen Schritten wird keine Sitzung geteilt.
- Sampling- und Elicitation-Anfragen des Servers werden abgelehnt.
- Benachrichtigungen des Servers werden ignoriert.
- Ressourcen-Templates (`resources/templates/list`) werden nicht unterstützt; sie sind ein möglicher Folgeschritt.
- Keine Größenbegrenzung für Blobs: Ein großer Blob wird im Speicher gehalten und in das Ergebnis übernommen.

## Fehler {#errors}

- Ein falscher Schritt (unbekannter Transport, fehlendes `command`, `url`, `tool`, `uri` oder `prompt`, unbekannte Methode, verschachteltes Prompt-Argument) ist ein Go-
  Fehler: Der Schritt schlägt fehl, noch bevor eine Verbindung besteht.
- Alles, was beim Austausch schiefgeht (keine Verbindung, 401, Timeout, JSON-RPC-Fehler), wird in
  `result.err` abgelegt; die Standard-Assertion lässt den Schritt dann mit diesem Text fehlschlagen, und Sie können stattdessen darauf prüfen.
- Ein Tool, das einen Fehler meldet (`isError`), setzt `result.iserror` auf true und `result.err` auf
  `tool "<name>" returned an error: <text of the server>`. Mit der Standard-Assertion schlägt der Schritt fehl; um den
  Fehler zu erwarten, prüfen Sie `result.iserror ShouldBeTrue`.

## Verbindung, Timeout, Umgebung {#connection-timeout-environment}

- Eine Verbindung pro Schritt: Der Schritt verbindet sich, führt seine Methode aus und schließt die Verbindung. Ein stdio-Server wird für den Schritt gestartet
  und an dessen Ende beendet.
- Timeout: Das `timeout` des Schritts (Engine) gilt; ohne eines verwendet der executor 30 Sekunden.
- Der stdio-Kindprozess erbt die gesamte Umgebung von venom.
- Protokoll: `initialize` wird mit der Version 2025-11-25 gesendet, sofern `protocol_version` nichts anderes festlegt, sodass sich eine Suite
  nicht ändert, wenn das SDK seinen Standard verschiebt.

## Sicherheitshinweis für stdio {#security-note-for-stdio}

`command` läuft mit den Rechten der Suite und erbt die gesamte Umgebung von venom, einschließlich der Tokens. Starten Sie einen
Server eines Drittanbieters über eine saubere Umgebung: `env -i PATH=... HOME=<tempdir> <server>`. Die Suite
`tests/realserver/filesystem.yml` tut dies:

```yaml
    command: [env, -i, "PATH={{.prepare.nodebin}}:/usr/bin:/bin", "HOME={{.prepare.tmpdir}}", npx, -y, "@modelcontextprotocol/server-filesystem@2026.1.14", "{{.prepare.tmpdir}}"]
```

## Secrets {#secrets}

Deklarieren Sie ein Token in den `secrets:` der Suite und verwenden Sie es in `headers`; venom verbirgt es in Logs und Berichten. Der executor
gibt die Header nie in seinem Ergebnis aus.

## Beispiele {#examples}

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

### Einen Wert mit `vars:` behalten {#keep-a-value-with-vars}

Ein Ergebniswert kann für die folgenden Testfälle behalten werden. Dieser Schritt aus `tests/realserver/filesystem.yml` behält den
Servernamen; spätere Testfälle lesen ihn als `{{.initialize.servername}}`:

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

## Die Suite ausführen {#run-the-suite}

`tests/mcp.yml` baut den Referenzserver (`executors/mcp/internal/refserver`) mit `go build` in ein frisches
`mktemp -d`-Verzeichnis (Binärdatei, Adressdatei und PID-Datei), benötigt also die Go-Toolchain, und zwei gleichzeitige Läufe
kollidieren nicht. Der letzte Testfall entfernt das Verzeichnis. Führen Sie sie allein aus dem Verzeichnis `tests/` aus:
`venom run mcp.yml`. Kein Docker, kein Netzwerk.
Die manuellen Fixtures `tests/failing/mcp-iserror.yml` (ein Tool-Fehler) und `tests/failing/mcp-resource-unknown.yml`
(eine unbekannte Ressourcen-URI) zeigen die Fehlermeldungen. Jede baut ihren eigenen Server in einem temporären Verzeichnis, entfernt
ihn wieder und soll fehlschlagen: `cd tests && venom run failing/mcp-iserror.yml`.

## Gegen einen echten Server ausprobieren {#try-it-against-a-real-server}

`tests/realserver/filesystem.yml` steuert über stdio einen echten Server eines Drittanbieters, `@modelcontextprotocol/server-filesystem`, festgelegt
auf Version 2026.1.14. Sie liegt außerhalb von `make run-test`: Sie benötigt das Netzwerk und führt Code eines Drittanbieters aus.
Führen Sie sie von Hand aus:

```bash
cd tests && venom run realserver/filesystem.yml
```

Sie benötigt `node`, `npx` und das npm-Netzwerk (npx lädt das Paket zur Laufzeit herunter). Die Suite arbeitet in einem temporären
Verzeichnis mit einer synthetischen Datei und entfernt es am Ende.

Der aufgezeichnete Lauf war erfolgreich (5 Testfälle, 8 Schritte). Der Server meldete sich als `secure-filesystem-server` 0.2.0,
das Protokoll war 2025-11-25, `tools/list` lieferte 14 Tools, und die Suite verwendete `list_directory` und
`read_text_file`.
