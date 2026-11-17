---
translates: mcp
title: MCP
description: Comment utiliser l'executor MCP dans une suite de tests venom.
fingerprint: 8d470c5d14201b424e787037fbd21d3ce531fda24704df29703bb787038699e0
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# Venom - Executor MCP {#venom---executor-mcp}

Étape qui appelle un serveur MCP (Model Context Protocol) : lister et appeler ses outils, lister et lire ses ressources, lister et obtenir ses prompts, puis poser des assertions sur la réponse.
Utilisez-la pour tester un serveur MCP depuis une suite venom, en stdio (venom démarre le serveur) ou en HTTP streamable.

Depuis : v1.1.0 (prévue). L'executor est nouveau dans la version qui suit la v1.0.0.

Pour vérifier qu'il fonctionne : `tests/mcp.yml` l'exécute de bout en bout contre un serveur de référence (voir « Lancer la suite »), et
`tests/realserver/filesystem.yml` l'exécute contre un vrai serveur tiers (voir « L'essayer contre un vrai serveur »).

## Entrée {#input}

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

`tools/call` envoie les arguments en JSON. `prompts/get` les envoie sous forme de chaînes, comme le prévoit le protocole : les chaînes sont
conservées, les nombres et les booléens sont écrits en toutes lettres (`3` devient `"3"`, `true` devient `"true"`), et une valeur imbriquée (map ou
liste) est une erreur d'usage avant toute connexion.

Un argument de prompt `null` (`who:` sans valeur) est refusé comme une erreur d'usage, comme une valeur imbriquée.

Assertion par défaut : `result.err ShouldBeEmpty`.

## Sortie {#output}

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

### Les clés remplies par chaque méthode {#which-keys-each-method-fills}

`serverinfo`, `capabilities` et `protocolversion` sont remplies par toutes les méthodes une fois la connexion ouverte
(elles viennent de la réponse à initialize). `iserror`, `err` et `timeseconds` existent toujours. Une clé sans valeur est
absente du résultat (un texte vide, une liste manquante). Le tableau montre les clés propres à chaque méthode.

| Méthode | Clés qu'elle remplit (en plus des clés communes ci-dessus) |
|---|---|
| `initialize` | aucune : les clés communes forment toute la réponse |
| `ping` | aucune |
| `tools/list` | `tools`, `toolsjson` |
| `tools/call` | `content`, `text`, `structured`, `structuredjson` (quand l'outil envoie du contenu structuré), `iserror` |
| `resources/list` | `resources` |
| `resources/read` | `resources`, `content`, `text` |
| `prompts/list` | `prompts` |
| `prompts/get` | `prompts`, `text`, `description` |

Si l'échange échoue après l'ouverture de la connexion, `err` est renseignée et les clés ci-dessus peuvent manquer ; les clés
communes restent présentes. Si la connexion elle-même échoue, seules `err` et `timeseconds` sont renseignées.

Les listes reviennent dans l'ordre où le serveur les a envoyées (le SDK Go trie par URI ou par nom). Posez vos assertions sur une entrée avec
`result.resources.resources0.uri` ou `result.prompts.prompts1.name`.

Un blob est renvoyé sous forme de chaîne base64 dans `blob`, avec `text` vide. Il n'y a pas de limite de taille.

Les contenus et les messages sont renvoyés tels que le serveur les a envoyés. Déclarez dans `secrets:` ce qui doit rester hors des rapports :
l'executor n'exécute ni ne transmet jamais ces données, et le masquage du moteur cache les secrets déclarés dans les rapports.

## Limites de la version 1 {#limits-of-version-1}

- Une connexion par étape : aucune session n'est partagée entre les étapes.
- Les requêtes de sampling et d'elicitation venant du serveur sont refusées.
- Les notifications du serveur sont ignorées.
- Les modèles de ressources (`resources/templates/list`) ne sont pas pris en charge ; ils pourraient faire l'objet d'une suite.
- Pas de limite de taille pour les blobs : un gros blob est conservé en mémoire et placé dans le résultat.

## Erreurs {#errors}

- Une étape incorrecte (transport inconnu, `command`, `url`, `tool`, `uri` ou `prompt` manquant, méthode inconnue, argument de prompt imbriqué) est une
  erreur Go : l'étape échoue avant toute connexion.
- Tout ce qui se passe mal pendant l'échange (connexion impossible, 401, délai dépassé, erreur JSON-RPC) est placé dans
  `result.err` ; l'assertion par défaut fait alors échouer l'étape avec ce texte, et vous pouvez poser une assertion dessus à la place.
- Un outil qui signale une erreur (`isError`) met `result.iserror` à true et `result.err` à
  `tool "<name>" returned an error: <text of the server>`. Avec l'assertion par défaut, l'étape échoue ; pour attendre
  l'erreur, utilisez l'assertion `result.iserror ShouldBeTrue`.

## Connexion, délai, environnement {#connection-timeout-environment}

- Une connexion par étape : l'étape se connecte, exécute sa méthode et se ferme. Un serveur stdio est démarré pour l'étape
  et arrêté à sa fin.
- Délai : le `timeout` de l'étape (moteur) s'applique ; sans lui, l'executor utilise 30 secondes.
- Le processus enfant stdio hérite de tout l'environnement de venom.
- Protocole : `initialize` est envoyé avec la version 2025-11-25 sauf si `protocol_version` indique autre chose, de sorte qu'une suite
  ne change pas quand le SDK change sa valeur par défaut.

## Note de sécurité pour stdio {#security-note-for-stdio}

`command` s'exécute avec les droits de la suite et hérite de tout l'environnement de venom, jetons compris. Pour un serveur
tiers, démarrez-le dans un environnement propre : `env -i PATH=... HOME=<tempdir> <server>`. La suite
`tests/realserver/filesystem.yml` le fait :

```yaml
    command: [env, -i, "PATH={{.prepare.nodebin}}:/usr/bin:/bin", "HOME={{.prepare.tmpdir}}", npx, -y, "@modelcontextprotocol/server-filesystem@2026.1.14", "{{.prepare.tmpdir}}"]
```

## Secrets {#secrets}

Déclarez un jeton dans les `secrets:` de la suite et utilisez-le dans `headers` ; venom le masque dans les journaux et les rapports. L'executor
ne renvoie jamais les en-têtes dans son résultat.

## Exemples {#examples}

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

### Conserver une valeur avec `vars:` {#keep-a-value-with-vars}

Une valeur du résultat peut être conservée pour les cas de test suivants. Cette étape, tirée de `tests/realserver/filesystem.yml`, conserve le
nom du serveur ; les cas de test suivants le lisent sous la forme `{{.initialize.servername}}` :

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

## Lancer la suite {#run-the-suite}

`tests/mcp.yml` construit le serveur de référence (`executors/mcp/internal/refserver`) avec `go build` dans un répertoire
`mktemp -d` neuf (binaire, fichier d'adresse et fichier de pid) : il faut donc la chaîne d'outils Go, et deux exécutions simultanées ne
se gênent pas. Le dernier cas de test supprime le répertoire. Lancez-la seule, depuis le répertoire `tests/` :
`venom run mcp.yml`. Pas de docker, pas de réseau.
Les fixtures manuelles `tests/failing/mcp-iserror.yml` (une erreur d'outil) et `tests/failing/mcp-resource-unknown.yml`
(un URI de ressource inconnu) montrent les messages d'échec. Chacune construit son propre serveur dans un répertoire temporaire, le supprime
ensuite, et doit échouer : `cd tests && venom run failing/mcp-iserror.yml`.

## L'essayer contre un vrai serveur {#try-it-against-a-real-server}

`tests/realserver/filesystem.yml` pilote un vrai serveur tiers, `@modelcontextprotocol/server-filesystem`, figé
à la version 2026.1.14, en stdio. Elle ne fait pas partie de `make run-test` : elle a besoin du réseau et exécute du code tiers.
Lancez-la à la main :

```bash
cd tests && venom run realserver/filesystem.yml
```

Il faut `node`, `npx` et le réseau npm (npx télécharge le paquet à l'exécution). La suite travaille dans un répertoire
temporaire avec un fichier synthétique et le supprime à la fin.

L'exécution enregistrée a réussi (5 cas de test, 8 étapes). Le serveur s'est annoncé comme `secure-filesystem-server` 0.2.0,
le protocole était 2025-11-25, `tools/list` a renvoyé 14 outils, et la suite a utilisé `list_directory` et
`read_text_file`.
