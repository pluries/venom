---
translates: web
title: Web
description: Cómo usar el executor Web en una suite de pruebas de venom.
fingerprint: d700bc6151e18429c9339fb06ffe6bc6b56b451abd92c6c1328ee3c6cbec08f8
drafted_by: claude
drafted_on: 2026-11-19
status: machine-drafted
---
# Venom - Executor Web {#venom---executor-web}

Navegar dentro de una aplicación web

Caso de uso: usted tiene una aplicación web y quiere comprobar algunos comportamientos.

El executor web permite navegar por su aplicación web y ejecutar acciones.

## Certificados HTTPS {#https-certificates}

Por defecto, el executor web ignora los errores de certificado HTTPS (el contexto del navegador se crea con `IgnoreHttpsErrors`).
Un paso web pasa contra un sitio con un certificado caducado, autofirmado o emitido para otro host, así que no debe usarse para comprobar
certificados. Para que el navegador rechace un certificado no válido, defina la variable `verifyTls` como `true`; el paso
falla entonces. El executor web no puede confiar en una autoridad certificadora privada: Playwright no ofrece esa opción para un contexto de navegador.

## Navegadores {#browsers}

El executor web maneja un navegador a través de [Playwright](https://playwright.dev) (binding de Go
`github.com/mxschmitt/playwright-go`). Playwright gestiona sus propias compilaciones de navegador: no hay que instalar `chromedriver` ni `geckodriver`,
ni emparejar ninguna versión con el navegador.

Instale una vez por máquina el driver de Playwright y los navegadores que usen sus suites, junto con sus paquetes
del sistema operativo (`--with-deps` necesita `sudo` en Linux):

```bash
go run github.com/mxschmitt/playwright-go/cmd/playwright@v0.6201.1 install --with-deps chromium   # or firefox, webkit
```

venom nunca descarga un navegador por su cuenta: una suite que necesite uno que no está instalado falla en la preparación, y el
mensaje de error indica este comando.

### Argumentos {#arguments}
* Chromium: [lista de argumentos](https://peter.sh/experiments/chromium-command-line-switches/). Un argumento sin
  guion inicial recibe `--`, como hacía chromedriver.
* Firefox: [lista de argumentos](https://wiki.mozilla.org/Firefox/CommandLineOptions),
  [lista de preferencias](https://searchfox.org/mozilla-central/source/modules/libpref/init/all.js)

## Variables {#variables}

Puede definir parámetros para configurar el navegador que se usa durante la suite de pruebas. Todos los parámetros son opcionales:
* driver (por defecto: `chrome`): navegador que se usa:
  * `chrome`: Chromium
  * `gecko`: Firefox (compilación propia de Playwright)
  * `webkit`: WebKit, el motor de Safari
* width, height: tamaño de la página. Sin ellos, la página conserva el tamaño propio de la ventana del navegador
* args: argumentos del navegador
* prefs: preferencias de Firefox (solo `gecko`)
* timeout (por defecto: `180`): tiempo máximo, en segundos, para que cargue una página (`navigate`, `historyAction`, la respuesta de una ventana emergente),
  y para que termine una navegación que inicia un paso
* proxy: proxy que se usa para consultar el sitio web (formato: `[server]:[port]`)
* verifyTls (por defecto: `false`): hace que el navegador rechace las páginas con un certificado HTTPS no válido, de modo que el paso falla. Sin ella,
  los errores de certificado se ignoran, como siempre
* headless (por defecto: `false`): ejecuta el navegador sin ventana. Útil en una pipeline de CI/CD
* binaryPath: ejecutable del navegador que se usa en lugar del de Playwright (solo `chrome`)
* trace (por defecto: `false`): guarda una traza de Playwright de cada paso fallido en `page.trace.zip`. Véase
  [Diagnóstico de fallos](#failure-diagnostics)
* testIdAttribute (por defecto: `data-testid`): el atributo que lee un localizador `TESTID` (véase [Localizadores](#locators)). Defínalo
  una vez para la suite, para una página que marca sus elementos con `data-test` u otro nombre. Véase [Trampas](#traps). El nombre debe cumplir `^[A-Za-z_][A-Za-z0-9_:.-]*$`: cualquier otro valor hace fallar el Setup, antes de que arranque el navegador

Chromium se ejecuta con su sandbox activado, como bajo chromedriver. Una suite que necesite desactivarlo (en un contenedor que se ejecuta como
root, por ejemplo) pasa `--no-sandbox` en `args`. Un host que restringe los espacios de nombres de usuario sin privilegios, como hacen
Ubuntu 23.10 y posteriores en instalación nativa, no puede iniciar el sandbox del Chromium propio de Playwright: la preparación falla y nombra las soluciones:
`--no-sandbox` en `args`, `binaryPath` apuntando a un Chrome del sistema, o permitir los espacios de nombres
(`sysctl kernel.apparmor_restrict_unprivileged_userns=0`).

Los errores de certificado HTTPS se ignoran por defecto, como ocurría con la sesión WebDriver que este executor usaba antes
(`acceptInsecureCerts`). Defina `verifyTls` para comprobarlos (véase Certificados HTTPS más arriba).

Se siguen aceptando, pero **se ignoran con una advertencia**, porque no tienen equivalente en Playwright:
* driverPath, driverPort: ya no hay un proceso de driver
* detach: Playwright no puede dejar el navegador en ejecución tras la suite
* debug, logLevel: fijan el nivel de registro del antiguo cliente WebDriver; use en su lugar el `-v` propio de venom
* prefs con `chrome`, binaryPath con `gecko`: Playwright maneja su propia compilación de Firefox

## Acción {#action}
Las acciones permiten manipular el navegador

Una acción sobre un elemento (`click`, `fill`, `select`, `uploadFile`, `selectFrame`, `hover`, `dragAndDrop` y `press` con `find`) espera a que el elemento esté listo,
hasta su `syncTimeout` o 30 segundos. `waitFor` espera a que el elemento alcance su `state`, dentro de su `syncTimeout`. `find` nunca espera: cuenta los elementos presentes en ese momento, de modo que
`result.find ShouldEqual 0` sigue siendo rápido. `find`, `waitFor`, `hover`, `dragAndDrop` y `press`, como las acciones anteriores,
esperan a una navegación ya en curso antes de ejecutarse. Tras una acción que puede cargar una página (`click`, `fill`, `select`,
`uploadFile`, `execute`, `waitFor`, `hover`, `dragAndDrop`, `press`, la respuesta a una ventana emergente), el paso espera a que la página termine de cargar antes de leer
`result.title` y `result.url`. Eso incluye una navegación que la página inicia por sí sola, desde un script
(`location.href = …`) o desde un temporizador sin retardo. Una navegación que no termina dentro de `timeout` hace fallar el paso,
y también los siguientes, hasta que un `navigate` la sustituya. No se escribe ningún `page.dump.html` mientras la página navega.

Una llamada a la página que nunca vuelve hace fallar su paso: tras 60 segundos para una lectura (el título, el código fuente, un recuento, un
texto; más el `syncTimeout` del paso sobre un elemento que espera), tras 30 segundos para `execute`.
La llamada al navegador sigue ejecutándose, así que el paso siguiente la espera antes de enviar la suya. Una llamada con un límite
propio (las lecturas y `execute`) espera como máximo ese límite y luego falla del mismo modo, y la espera continúa
en el paso siguiente. Una acción sin límite propio (un clic, una selección, una captura de pantalla) espera como máximo 60 segundos, registra una
advertencia y continúa. `navigate`, `historyAction` y un vaciado del temporizador que una navegación ha terminado nunca esperan, y un cuadro de diálogo que
se haya abierto mientras tanto se descarta primero, como de costumbre. Un paso fallido no escribe `page.dump.html` solo mientras esa llamada
sigue en ejecución: en su lugar registra una advertencia.

Lista de valores de salida disponibles:
* result.url: URL de la página actual
* result.timeseconds: duración de la ejecución de la acción
* result.title: título de la página actual

### Navigate {#navigate}
Navegar a una URL concreta

#### Entrada {#input}
* URL: URL a la que navegar
* Reset: restablecer el estado del navegador

#### Ejemplo {#example}
```yaml
  - type: web
    action:
      navigate:
        url: https://www.google.fr
```

### Find {#find}
Buscar un elemento a partir de un selector

[Enlace al tutorial de selectores CSS](https://developer.mozilla.org/fr/docs/Web/CSS/CSS_Selectors)
[Enlace al tutorial de selectores XPATH](https://developer.mozilla.org/fr/docs/Web/XPath)

#### Entrada {#input-1}
* Selector: expresión que se usa para identificar el elemento web
* Locator: localizador que se usa para buscar el elemento: `CSS` o `XPATH`, o uno de los tipos de [Localizadores](#locators)

#### Salida {#output}
* result.find: devuelve el número de objetos identificados

#### Ejemplo {#example-1}
```yaml
  - type: web
    action:
      find: 
        selector: .gsfi
        locator: CSS
    assertions:
    - result.find ShouldEqual 2

  - type: web
    action:
      find:
        selector: //div[@class='FPdoLc lJ9FBc']/center/input[@value='Recherche Google']
        locator: XPATH
    assertions:
    - result.find ShouldEqual 1
    - result.value ShouldEqual Recherche Google
```

### Localizadores {#locators}
Un elemento `find` indica su tipo en `locator`. Además de `CSS` y `XPATH`, cinco tipos corresponden a lo que ve un usuario
o a lo que la página marca para las pruebas. Prefiéralos a una ruta CSS: sobreviven a un rediseño de la página.

#### Entrada {#input-2}
* `locator: ROLE`: `selector` es un rol ARIA (`button`, `link`, `textbox`, `checkbox`, `heading`, `listitem`, ...).
  `name` es opcional y filtra por el nombre accesible del elemento; un `name` vacío significa que no hay filtro por nombre
* `locator: TEXT`: `selector` es el texto del elemento
* `locator: LABEL`: `selector` es el texto de la etiqueta de un campo de formulario
* `locator: PLACEHOLDER`: `selector` es el texto de ayuda (placeholder) de un campo
* `locator: TESTID`: `selector` es el id de prueba. Lee el atributo `data-testid`, o el atributo que fija la variable
  `testIdAttribute`. Coincide con el valor completo y no admite `exact`
* `exact` (`true` o `false`, por defecto `false`): para `ROLE` se aplica a `name`, y para `TEXT`, `LABEL` y `PLACEHOLDER`
  a `selector`

Por defecto, `TEXT`, `LABEL` y un nombre de `ROLE` coinciden sin distinguir mayúsculas de minúsculas, como subcadena, e ignoran los
espacios sobrantes: el texto `add` coincide con el botón `Add item`. Con `exact: true`, la coincidencia es con el texto completo,
con las mayúsculas tal como se escribieron. `PLACEHOLDER` coincide con el atributo tal como está escrito.

Un valor se lee siempre como texto plano. Nunca es un selector CSS, un prefijo de motor ni una expresión regular. Un paso que
encuentra varios elementos usa el primero.

#### Ejemplo {#example-2}
```yaml
  - type: web
    action:
      click:
        find:
          locator: ROLE
          selector: button
          name: Add item
          exact: true

  - type: web
    action:
      fill:
      - find:
          locator: LABEL
          selector: Email
        text: user@example.com

  - type: web
    action:
      find:
        locator: TESTID
        selector: cart-count
    assertions:
    - result.find ShouldEqual 1
```

#### Errores {#errors}
Un paso con un `find` incorrecto falla antes de actuar, y el mensaje nombra la clave:
* un `locator` que no es uno de los siete tipos: `invalid find element locator 'X' (must be CSS, XPATH, ROLE, TEXT, LABEL, PLACEHOLDER or TESTID)`
* un selector de `ROLE` que no es un rol ARIA: el mensaje lista los roles
* un `selector` vacío para `TEXT`, `LABEL`, `PLACEHOLDER` o `TESTID`
* un selector `TESTID` que contiene un carácter NUL
* `name` en un tipo distinto de `ROLE`, o `exact` en `TESTID`: el mensaje dice qué tipos lo admiten
* `name` que no es una cadena, o `exact` que no es `true` ni `false`
* cualquier otra clave: `invalid find element key 'X' for a ROLE locator (must be locator, selector, name, exact)`

Estas comprobaciones se aplican a los cinco tipos de esta sección. Las claves adicionales de un elemento `CSS` o `XPATH` se ignoran, como siempre.

### Click {#click}
Hacer clic en un elemento a partir de un selector

#### Entrada {#input-3}
* Find: elemento que se busca (más información en la sección find)
* Wait: tiempo de espera tras el clic (en segundos)
* SyncTimeout: tiempo máximo, en segundos, de espera a que el elemento esté listo (presente, visible, habilitado, quieto, no tapado). Por defecto: `30`

#### Ejemplo {#example-3}
```yaml
  - type: web
    action:
      click:
        find:
          selector: button
          locator: CSS
```

### Fill {#fill}
Rellenar un elemento (input o textarea) con un texto. El texto se escribe a continuación del contenido actual del campo, como hacía WebDriver;
no lo sustituye

#### Entrada {#input-4}
Lista de entradas estructuradas:
* Find: elemento que se busca (más información en la sección find)
* Text: actualiza el valor del elemento con un texto
* Key: actualiza el valor del elemento con una tecla
* SyncTimeout: tiempo máximo, en segundos, de espera a que el elemento esté listo (presente, visible, habilitado, quieto, no tapado). Por defecto: `30`

#### Ejemplo {#example-4}
```yaml
  - type: web
    action:
      fill:
      - find:
          selector: input
          locator: CSS
        text: userName
```

### Select {#select}
Seleccionar una opción de un elemento web

#### Entrada {#input-5}
* Find: elemento que se busca (más información en la sección find)
* Text: opción que se selecciona
* Wait: tiempo de espera tras el clic (en segundos)
* SyncTimeout: tiempo máximo, en segundos, de espera a que el elemento esté listo (presente, visible, habilitado, quieto, no tapado). Por defecto: `30`

#### Ejemplo {#example-5}
```yaml
  - type: web
    action:
      select:
        find:
          selector: select
          locator: CSS
        text: Option 1
```

### Upload file {#upload-file}
Subir un archivo a un elemento web

#### Entrada {#input-6}
* Find: elemento que se busca (más información en la sección find)
* Files: lista de cadenas que define los archivos que se suben
* Wait: tiempo de espera tras el clic (en segundos)
* SyncTimeout: tiempo máximo, en segundos, de espera a que el elemento esté listo (presente, visible, habilitado, quieto, no tapado). Por defecto: `30`

#### Ejemplo {#example-6}
```yaml
  - type: web
    action:
      uploadFile:
        find:
          selector: input
          locator: CSS
        files: 
        - toUpload.csv
```

### Wait {#wait}
Esperar un tiempo
Un valor entero que define el número de segundos de espera

#### Ejemplo {#example-7}
```yaml
  - type: web
    action:
      wait: 1
```

### Wait for {#wait-for}
Esperar hasta que un elemento esté en un estado. Úselo cuando la página carga datos o abre un panel después del primer pintado.

#### Entrada {#input-7}
* Find: elemento que se espera (más información en find y [Localizadores](#locators))
* State: `visible` (por defecto, también cuando `state` está vacío), `hidden`, `attached` o `detached`
  * `visible`: el elemento existe y se muestra
  * `hidden`: el elemento no existe o no se muestra
  * `attached`: el elemento existe en la página, se muestre o no
  * `detached`: el elemento no está en la página
* SyncTimeout: tiempo máximo, en segundos, de espera del estado. Por defecto: `30`. Un valor de `0` o menos equivale a `30`

Para `visible`, el paso espera sobre la primera coincidencia.

Cada comprobación mira el primer elemento que coincide en ese momento. Si ese elemento se elimina y otro sigue coincidiendo, la espera continúa con el siguiente. `detached` espera hasta que ningún elemento coincida ya: cuando se elimina el primer elemento, el siguiente pasa a ser el primero, así que con varias coincidencias la espera dura hasta que no queda ninguna. `hidden` sigue al primer elemento que coincide, que sigue siendo el mismo elemento mientras exista (un elemento oculto con CSS sigue coincidiendo), así que con varias coincidencias `hidden` puede terminar mientras los demás aún se muestran. Para esperar a que cada uno esté oculto, espera `detached`, o da a cada elemento su propio locator.

#### Ejemplo {#example-8}
```yaml
  - type: web
    action:
      waitFor:
        find:
          locator: TEXT
          selector: Order saved
        state: visible
        syncTimeout: 10

  - type: web
    action:
      waitFor:
        find:
          locator: TESTID
          selector: spinner
        state: hidden
```

#### Errores {#errors-1}
* `waitFor: the field 'find' is required (allowed: find, state, syncTimeout)`
* `waitFor: invalid 'state' X (allowed: visible, hidden, attached, detached)`: el estado distingue mayúsculas de minúsculas
* un campo distinto de `find`, `state` y `syncTimeout`: `waitFor: unknown field 'X' (allowed: find, state, syncTimeout)`
* el estado no se alcanza a tiempo: `waitFor: the element ROLE "button" name "Add" did not become visible within 30s`, seguido del
  motivo del navegador

### Hover {#hover}
Mover el puntero sobre un elemento, para abrir un menú o una información emergente que aparece al pasar el ratón. El paso espera a que el elemento esté
listo, como hace un clic.

#### Entrada {#input-8}
* Find: elemento sobre el que pasar el puntero (más información en find y [Localizadores](#locators)). Se usa la primera coincidencia
* SyncTimeout: tiempo máximo, en segundos, de espera a que el elemento esté listo. Por defecto: `30`

#### Ejemplo {#example-9}
```yaml
  - type: web
    action:
      hover:
        find:
          locator: ROLE
          selector: link
          name: Products

  - type: web
    action:
      waitFor:
        find:
          locator: ROLE
          selector: menuitem
          name: Shoes
```

#### Errores {#errors-2}
* `hover: the field 'find' is required (allowed: find, syncTimeout)`
* un campo distinto de `find` y `syncTimeout`: `hover: unknown field 'X' (allowed: find, syncTimeout)`
* el elemento no está listo a tiempo: `hover: ROLE "link" name "Products"`, seguido del motivo del navegador

### Arrastrar y soltar {#drag-and-drop}
Arrastrar un elemento y soltarlo sobre otro. El paso espera a que ambos estén listos.

#### Entrada {#input-9}
* From: elemento que se arrastra (más información en find y [Localizadores](#locators)). Se usa la primera coincidencia
* To: elemento sobre el que se suelta. Se usa la primera coincidencia
* SyncTimeout: tiempo máximo, en segundos, de espera a que termine el arrastre. Por defecto: `30`

#### Ejemplo {#example-10}
```yaml
  - type: web
    action:
      dragAndDrop:
        from:
          locator: TEXT
          selector: Task A
          exact: true
        to:
          locator: TESTID
          selector: column-done
```

#### Errores {#errors-3}
* `dragAndDrop: the field 'from' is required (allowed: from, to, syncTimeout)`, y lo mismo para `to`
* un campo distinto de `from`, `to` y `syncTimeout`: `dragAndDrop: unknown field 'X' (allowed: from, to, syncTimeout)`
* el soltado falla o agota el tiempo: `dragAndDrop: TEXT "Task A" exact to TESTID "column-done"`, seguido del motivo del navegador

### Press {#press}
Pulsar una tecla o una combinación de teclas, como `Enter`, `Escape`, `Tab` o `Control+A`. Con `find`, la tecla va a ese elemento.
Sin él, la tecla va al elemento que tiene el foco. `press` toma una tecla o una combinación, no un texto: para escribir un
texto, use `fill`.

#### Entrada {#input-10}
* Key: la tecla o la combinación (obligatorio). Distingue mayúsculas de minúsculas. Use un nombre de tecla de Playwright (`Enter`,
  `ArrowDown`, `F5`, `Backspace`) o una tecla de un solo carácter (`a`, `7`). Una combinación une las teclas con `+`:
  `Control+A`, `Shift+Tab`. Los nombres WebDriver que `fill` acepta para `key` (`ENTER`, `ESCAPE`, `ARROW_DOWN`) también funcionan,
  como tecla única. Una combinación solo admite nombres de Playwright. `press` acepta los nombres de tecla de una lista fija; una tecla rara de Playwright que no está en ella
  (`ShiftLeft`, `ControlLeft`, `CapsLock`, `ContextMenu`, `NumLock`, `ScrollLock`, `PrintScreen`, `AltGraph`, `F13` y superiores) se rechaza con un mensaje que nombra la tecla, y nunca se envía una tecla equivocada. Una tecla de un solo carácter se acepta siempre
* Find: elemento al que enviar la tecla (más información en find y [Localizadores](#locators)). Opcional. Se usa la primera coincidencia
* SyncTimeout: tiempo máximo, en segundos, de espera a que el elemento esté listo. Por defecto: `30`. Sin `find` no interviene ningún elemento, así que no es una espera de elemento: solo amplía el límite de la propia llamada de la tecla (60 segundos, más `syncTimeout`)

#### Ejemplo {#example-11}
```yaml
  - type: web
    action:
      press:
        key: Enter
        find:
          locator: PLACEHOLDER
          selector: Search

  - type: web
    action:
      press:
        key: Control+A
```

#### Errores {#errors-4}
* `press: the field 'key' is required (allowed: key, find, syncTimeout): press takes one key or one chord`
* un campo distinto de `key`, `find` y `syncTimeout`: `press: unknown field 'X' (allowed: key, find, syncTimeout)`
* una `key` que no es un texto, un número YAML como `key: 5`: el mensaje dice que se escriba entre comillas (`key: "5"`)
* una tecla desconocida, o un texto: `press: invalid 'key' "X": not a key name; press takes one key or one chord`. En una
  combinación, el mensaje nombra la parte que falló
* una tecla WebDriver para la que Playwright no tiene tecla (`NULL`, `CANCEL`, `HELP`): el mensaje dice que no tiene
  equivalente en Playwright

### Select frame {#select-frame}
Seleccionar un marco (frame) presente en la página actual

#### Entrada {#input-11}
* Find: marco que se busca (más información en la sección find)
* SyncTimeout: tiempo máximo, en segundos, de espera a que el elemento esté listo (presente, visible, habilitado, quieto, no tapado). Por defecto: `30`

#### Ejemplo {#example-12}
```yaml
  - type: web
    action:
      selectFrame:
        find:
          selector: "#iframeResult"
          locator: CSS
```

### Select root frame {#select-root-frame}
Seleccionar el marco raíz presente en la página actual
Un valor booleano que se define como true para seleccionar el marco raíz

#### Ejemplo {#example-13}
```yaml
  - type: web
    action:
      selectRootFrame: true
```

### Next window {#next-window}
Seleccionar la ventana siguiente (pestaña) del navegador, pasando de la última a la primera; con una sola ventana, el paso
se queda en ella
Un valor booleano que se define como true para seleccionar la ventana siguiente

#### Ejemplo {#example-14}
```yaml
  - type: web
    action:
      nextWindow: true
```

### Back {#back}
Volver a la página anterior
Un valor booleano que se define como true para volver a la página anterior

#### Ejemplo {#example-15}
```yaml
  - type: web
    action:
      historyAction: back
    assertions:
    - result.title ShouldStartWith GitHub
    - result.url ShouldEqual https://github.com/
```

### Forward {#forward}
Avanzar a la página siguiente
Un valor booleano que se define como true para avanzar de página

#### Ejemplo {#example-16}
```yaml
  - type: web
    action:
      historyAction: forward
    assertions:
    - result.title ShouldStartWith GitHub
    - result.url ShouldEqual https://github.com/team
```

### Refresh {#refresh}
Actualizar la página
Un valor booleano que se define como true para actualizar la página

#### Ejemplo {#example-17}
```yaml
  - type: web
    action:
      historyAction: refresh
    assertions:
    - result.title ShouldStartWith GitHub
    - result.url ShouldEqual https://github.com/team
```

### Confirm popup {#confirm-popup}
Confirmar un cuadro de diálogo emergente (cuadro de diálogo de confirmación y cuadro de diálogo de alerta)

Un cuadro de diálogo abierto por un paso permanece abierto para que el paso siguiente lo responda con `confirmPopup` o `cancelPopup`; mientras
está abierto, `result.title` y `result.url` están vacíos. Cualquier otra acción mientras hay un cuadro de diálogo abierto lo descarta y falla
con `unexpected alert open`, como hacía WebDriver.
Un valor booleano que se define como true para confirmar el cuadro de diálogo emergente

#### Ejemplo {#example-18}
```yaml
  - type: web
    action:
      confirmPopup: true
```

### Cancel popup {#cancel-popup}
Cancelar un cuadro de diálogo emergente
Un valor booleano que se define como true para cancelar el cuadro de diálogo emergente

#### Ejemplo {#example-19}
```yaml
  - type: web
    action:
      cancelPopup: true
```

### Execute javascript {#execute-javascript}
Ejecutar código javascript. El comando es el cuerpo de una función: `return` y `arguments[0]`, `arguments[1]`… funcionan
como lo hacían con WebDriver

#### Entrada {#input-12}
* Command: código javascript que se ejecuta
* Args: lista de cadenas que define los argumentos de javascript

#### Salida {#output-1}
Lo que devuelve el script, si devuelve algo. Cuando devuelve una promesa, el paso espera su valor.
* result.value: el valor como texto. Una cadena se mantiene tal cual. Cualquier otro valor se escribe como JSON: `42`, `true`,
  `[1,"a"]`, `{"user":{"name":"Ada"}}`, con las claves de un objeto ordenadas
* result.valuejson: el mismo valor, con tipo, como `result.bodyjson` en el executor http. Úselo para comparar números o
  para acceder al interior de un objeto o de una lista: `result.valuejson.user.name`

El resultado de todo paso web tiene una variable `result.valuejson`. Está vacía salvo que el paso ejecute un script que devuelva
un valor.

Un script que no devuelve nada, `undefined` o `null` deja ambos vacíos. Algunos valores no tienen forma JSON:
* `NaN`, `Infinity` y `-Infinity` se leen como esas palabras
* un `Date` se lee como su hora UTC en RFC 3339 (`1970-01-01T00:00:00Z`)
* un BigInt, una URL y una RegExp se leen como su texto
* un elemento DOM, `document` o `window` se lee como `ref: <Node>`, `ref: <Document>` o `ref: <Window>`

Un valor que se refiere a sí mismo, como un objeto con una propiedad que apunta de vuelta al objeto, deja ambos campos vacíos.
Lo mismo ocurre con una URL que el navegador acepta y venom no puede leer, como una con un `%` que no va seguido de dos dígitos hexadecimales
(`https://a.test/x#50%off`). venom registra una advertencia y el paso pasa.

#### Ejemplo {#example-20}
```yaml
  - type: web
    action:
      execute:
        command: "window.editor.setValue(\"<!DOCTYPE html> <html></html>\"); window.editor.save();"

  - type: web
    action:
      execute:
        command: "return {count: document.querySelectorAll('li').length, first: document.querySelector('li').innerText};"
    assertions:
    - result.valuejson.count ShouldEqual 3
    - result.valuejson.first ShouldEqual Apples
```


Más información sobre las acciones (https://github.com/ovh/venom/tree/master/executors/web/types.go)
Para una acción, puede hacer una captura de pantalla del navegador con el siguiente comando: `screenshot: [fileName].png`

## Simular llamadas de red {#mock-network-calls}
Un paso puede responder por sí mismo a las peticiones de red de la página, de modo que una suite pueda probar un front end sin su backend. Añada una
lista `mock:` al paso, junto a `action:`. Cada entrada responde a toda petición cuya URL coincide con su `url`, y la
petición nunca llega a la red. El host no necesita existir: un mock puede servir la propia página y, después, las
llamadas de la propia página.

#### Entrada {#input-13}
Lista de entradas estructuradas:
* Url: patrón de las URL a las que se responde (obligatorio). `*` coincide con cualquier carácter salvo `/`, `**` coincide con cualquier carácter,
  `{a,b}` coincide con `a` o con `b`. El patrón debe coincidir con la URL completa: `**/api/users` responde a
  `https://app.test/api/users`, no a `https://app.test/api/users?page=2`; `**/api/users*` responde a ambas. Cada `{` necesita
  su `}`, o el paso falla: escriba `\{` para una llave literal (`\\{` entre comillas dobles)
* Status: estado HTTP de la respuesta. Por defecto: `200`
* Headers: cabeceras de la respuesta. Escriba cada valor como una cadena: `X-Total: "3"`, no `X-Total: 3`
* ContentType: el `Content-Type` de la respuesta. Por defecto: ninguno
* Body: el cuerpo de la respuesta, como texto

Un mock empieza a responder antes de que se ejecute la acción del paso, y sigue respondiendo en los pasos siguientes del mismo caso
de prueba, en todas las pestañas. Cuando varios mocks coinciden con una URL, responde el más reciente. El caso de prueba siguiente empieza sin
ningún mock. Un paso puede contener `mock:` y ninguna acción.

Definir `mock:` mientras hay una ventana emergente abierta la descarta y hace fallar el paso, como cualquier otra acción. Defina el mock en
un paso anterior al que abre la ventana emergente.

Una petición que el service worker de la página responde por sí mismo nunca llega al mock.

#### Ejemplo {#example-21}
```yaml
  - type: web
    mock:
    - url: "**/app"
      contentType: text/html
      body: "<title>Users</title><h1>Users</h1>"
    - url: "**/api/users"
      status: 200
      contentType: application/json
      headers:
        X-Total: "1"
      body: '[{"name": "Ada"}]'
    action:
      navigate:
        url: http://app.venom.invalid/app
    assertions:
    - result.title ShouldEqual Users

  - type: web
    action:
      execute:
        command: "return fetch('/api/users').then(r => r.json()).then(users => users[0].name);"
    assertions:
    - result.value ShouldEqual Ada
```

## Diagnóstico de fallos {#failure-diagnostics}
Cuando un paso falla, el executor web escribe `page.dump.html` en el directorio de trabajo: el código fuente del marco actual.
Omite el volcado mientras la página sigue cargando un documento nuevo. Un paso con `screenshot:` escribe su captura de pantalla,
y el volcado, una vez que su acción ha tenido éxito.

Con la variable `trace: true`, un paso fallido escribe además `page.trace.zip` junto a `page.dump.html`: una traza de Playwright
de ese paso, con cada acción, una instantánea de la página antes y después de ella, capturas de pantalla y las llamadas de red.
Ábrala con el visor de trazas de Playwright:

```bash
go run github.com/mxschmitt/playwright-go/cmd/playwright@v0.6201.1 show-trace page.trace.zip
```

El visor necesita el driver de Playwright; el comando lo descarga si falta, como hace el comando de instalación.
El siguiente paso fallido sustituye ambos archivos. Un paso que pasa no escribe ninguna traza.

La traza contiene todo lo que el paso vio y escribió, **incluidas las contraseñas**, y las cabeceras y los cuerpos de sus llamadas
de red. El enmascaramiento de secretos de venom no se aplica a ella. venom la escribe legible solo por su propietario, igual que
`page.dump.html`. No la publique como artefacto de compilación ni la suba al repositorio. Además, el trazado
ralentiza cada paso, unos 20 ms en una página sencilla: actívelo mientras investiga un fallo.

## Trampas {#traps}
* Un selector mal escrito pasa de inmediato `waitFor` con el estado `hidden` o `detached`: un elemento que no existe está
  oculto y desacoplado. Compruebe antes el selector con un `find` o con una espera `visible`.
* `TESTID` lee `data-testid` por defecto. Una página que marca sus elementos con `data-test` (u otro nombre) no se
  encuentra hasta que la suite define `testIdAttribute`. Sin él, el paso no encuentra el elemento.
* El volcado de la página (`page.dump.html`) y la traza contienen lo que `fill` y `press` escribieron. El enmascaramiento de secretos de venom
  no los alcanza. No escriba un secreto con `press`, una tecla cada vez. Véase [Diagnóstico de fallos](#failure-diagnostics).
* `press` no es escribir. Envía teclas, no un texto: `press: Hello` falla. Use `fill` para introducir un texto.

## Ejemplo {#example-22}
Un ejemplo global

```yaml
name: TestSuite Web
vars:
  web:
    driver: chrome
    width: 1920
    height: 1080
    args:
    - 'browser-test'
    prefs:
      profile.default_content_settings.popups: 0
      profile.default_content_setting_values.notifications: 1
    timeout: 60
    debug: true
testcases:
- name: TestCase Google search
  steps:
  - type: web
    action:
      navigate:
        url: https://www.google.fr
    assertions:
    - result.title ShouldEqual Google
    - result.url ShouldEqual https://www.google.fr
  - type: web
    action:
      find: input[name="q"]
    assertions:
     - result.find ShouldEqual 1
  - type: web
    action:
      fill:
      - find: input[name="q"]
        text: "venom ovh"
  - type: web
    action:
      click:
        find: input[value="Recherche Google"]
        wait: 1
    screenshot: googlesearch.png

```
