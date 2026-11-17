---
translates: mcp
title: MCP
description: Cómo usar el executor MCP en una suite de pruebas de venom.
fingerprint: 8d470c5d14201b424e787037fbd21d3ce531fda24704df29703bb787038699e0
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# Venom - Executor MCP {#venom---executor-mcp}

Paso para llamar a un servidor MCP (Model Context Protocol): enumerar y llamar a sus herramientas, enumerar y leer sus recursos, enumerar y obtener sus prompts, y comprobar la respuesta mediante aserciones.
Úselo para probar un servidor MCP desde una suite de venom, por stdio (venom inicia el servidor) o por HTTP streamable.

Desde: v1.1.0 (prevista). El executor es nuevo en la versión posterior a v1.0.0.

Para comprobar que funciona: `tests/mcp.yml` lo ejecuta de extremo a extremo contra un servidor de referencia (véase «Ejecutar la suite»), y
`tests/realserver/filesystem.yml` lo ejecuta contra un servidor real de terceros (véase «Probarlo contra un servidor real»).

## Entrada {#input}

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

`tools/call` envía los argumentos como JSON. `prompts/get` los envía como cadenas, tal como indica el protocolo: las cadenas se
conservan, los números y los booleanos se escriben como texto (`3` pasa a ser `"3"`, `true` pasa a ser `"true"`), y un valor anidado (mapa o
lista) es un error de uso antes de cualquier conexión.

Un argumento de prompt `null` (`who:` sin valor) se rechaza como error de uso, igual que un valor anidado.

Aserción por defecto: `result.err ShouldBeEmpty`.

## Salida {#output}

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

### Qué claves rellena cada método {#which-keys-each-method-fills}

`serverinfo`, `capabilities` y `protocolversion` los rellena todos los métodos una vez abierta la conexión
(proceden de la respuesta de initialize). `iserror`, `err` y `timeseconds` siempre existen. Una clave sin valor
no aparece en el resultado (un texto vacío, una lista ausente). La tabla muestra las claves propias de cada método.

| Método | Claves que rellena (además de las comunes indicadas arriba) |
|---|---|
| `initialize` | ninguna: las claves comunes son toda la respuesta |
| `ping` | ninguna |
| `tools/list` | `tools`, `toolsjson` |
| `tools/call` | `content`, `text`, `structured`, `structuredjson` (cuando la herramienta envía contenido estructurado), `iserror` |
| `resources/list` | `resources` |
| `resources/read` | `resources`, `content`, `text` |
| `prompts/list` | `prompts` |
| `prompts/get` | `prompts`, `text`, `description` |

Si el intercambio falla después de abrirse la conexión, se rellena `err` y puede que falten las claves anteriores; las claves
comunes siguen presentes. Si falla la propia conexión, solo se rellenan `err` y `timeseconds`.

Las listas se devuelven en el orden en que las envió el servidor (el SDK de Go ordena por URI o por nombre). Para comprobar una entrada, use
`result.resources.resources0.uri` o `result.prompts.prompts1.name`.

Un blob se devuelve como una cadena en base64 en `blob`, con `text` vacío. No hay límite de tamaño.

Los contenidos y los mensajes se devuelven tal como los envió el servidor. Declare en `secrets:` lo que no debe aparecer en los informes:
el executor nunca ejecuta ni reenvía esos datos, y la redacción del motor oculta en los informes los secretos declarados.

## Límites de la versión 1 {#limits-of-version-1}

- Una conexión por paso: no se comparte ninguna sesión entre pasos.
- Las solicitudes de muestreo (sampling) y de elicitación del servidor se rechazan.
- Las notificaciones del servidor se ignoran.
- Las plantillas de recursos (`resources/templates/list`) no se admiten; son una posible ampliación futura.
- No hay límite de tamaño para los blobs: un blob grande se mantiene en memoria y se incluye en el resultado.

## Errores {#errors}

- Un paso incorrecto (transport desconocido, falta `command`, `url`, `tool`, `uri` o `prompt`, método desconocido, argumento de prompt anidado) es un error
  de Go: el paso falla antes de cualquier conexión.
- Todo lo que falla en el intercambio (no se puede conectar, 401, tiempo de espera agotado, error JSON-RPC) se coloca en
  `result.err`; entonces la aserción por defecto hace fallar el paso con ese texto, y usted puede hacer una aserción sobre él en su lugar.
- Una herramienta que notifica un error (`isError`) establece `result.iserror` en true y `result.err` en
  `tool "<name>" returned an error: <text of the server>`. Con la aserción por defecto el paso falla; para esperar
  el error, use `result.iserror ShouldBeTrue`.

## Conexión, tiempo de espera, entorno {#connection-timeout-environment}

- Una conexión por paso: el paso se conecta, ejecuta su método y cierra. Un servidor stdio se inicia para el paso
  y se detiene al terminar este.
- Tiempo de espera: se aplica el `timeout` del paso (del motor); si no hay ninguno, el executor usa 30 segundos.
- El proceso hijo stdio hereda todo el entorno de venom.
- Protocolo: se envía `initialize` con la versión 2025-11-25 salvo que `protocol_version` indique otra, de modo que una suite
  no cambia cuando el SDK cambia su valor por defecto.

## Nota de seguridad para stdio {#security-note-for-stdio}

`command` se ejecuta con los derechos de la suite y hereda todo el entorno de venom, incluidos los tokens. Para un
servidor de terceros, inícielo con un entorno limpio: `env -i PATH=... HOME=<tempdir> <server>`. La suite
`tests/realserver/filesystem.yml` lo hace así:

```yaml
    command: [env, -i, "PATH={{.prepare.nodebin}}:/usr/bin:/bin", "HOME={{.prepare.tmpdir}}", npx, -y, "@modelcontextprotocol/server-filesystem@2026.1.14", "{{.prepare.tmpdir}}"]
```

## Secretos {#secrets}

Declare un token en los `secrets:` de la suite y úselo en `headers`; venom lo oculta en los registros y los informes. El executor
nunca devuelve los encabezados en su resultado.

## Ejemplos {#examples}

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

### Conservar un valor con `vars:` {#keep-a-value-with-vars}

Un valor del resultado se puede conservar para los casos de prueba siguientes. Este paso, tomado de `tests/realserver/filesystem.yml`, conserva el
nombre del servidor; los casos de prueba posteriores lo leen como `{{.initialize.servername}}`:

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

## Ejecutar la suite {#run-the-suite}

`tests/mcp.yml` compila el servidor de referencia (`executors/mcp/internal/refserver`) con `go build` en un directorio
`mktemp -d` nuevo (binario, archivo de dirección y archivo de pid), por lo que necesita la cadena de herramientas de Go, y dos ejecuciones simultáneas no
colisionan. El último caso de prueba elimina el directorio. Ejecútela por separado, desde el directorio `tests/`:
`venom run mcp.yml`. Sin docker, sin red.
Los archivos de prueba manuales `tests/failing/mcp-iserror.yml` (un error de herramienta) y `tests/failing/mcp-resource-unknown.yml`
(un URI de recurso desconocido) muestran los mensajes de fallo. Cada uno compila su propio servidor en un directorio temporal, lo elimina
y se espera que falle: `cd tests && venom run failing/mcp-iserror.yml`.

## Probarlo contra un servidor real {#try-it-against-a-real-server}

`tests/realserver/filesystem.yml` maneja un servidor real de terceros, `@modelcontextprotocol/server-filesystem`, fijado
en la versión 2026.1.14, por stdio. Queda fuera de `make run-test`: necesita red y ejecuta código de terceros.
Ejecútelo a mano:

```bash
cd tests && venom run realserver/filesystem.yml
```

Necesita `node`, `npx` y la red de npm (npx descarga el paquete en tiempo de ejecución). La suite trabaja en un directorio
temporal con un archivo sintético y lo elimina al final.

La ejecución registrada fue correcta (5 casos de prueba, 8 pasos). El servidor se anunció como `secure-filesystem-server` 0.2.0,
el protocolo era 2025-11-25, `tools/list` devolvió 14 herramientas, y la suite usó `list_directory` y
`read_text_file`.

