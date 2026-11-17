---
translates: mobile
title: Mobile
description: Cómo usar el executor Mobile en una suite de pruebas de venom.
fingerprint: b5add67e3549852f87f31f8dbe9f13cb7b412228964257c765cad39d54a9af9c
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# Venom - Executor Mobile {#venom---executor-mobile}

Paso para controlar desde una suite de venom una aplicación nativa o híbrida de Android o iOS: instalar una aplicación, encontrar elementos, tocar, escribir, leer texto, desplazarse, hacer una captura de pantalla, abrir un enlace profundo, cambiar a una vista web, responder a una alerta del sistema de iOS y comprobar con aserciones lo que se ve. Se comunica con un servidor [Appium](https://appium.io) 3 que se ejecuta en la misma
máquina que venom, o en un Mac al que venom llega mediante un túnel SSH. Usa el controlador UiAutomator2 para un
emulador de Android y el controlador XCUITest para un simulador de iOS.

Estado: hitos M1 (#2396), M2 (#2397), M3 (#2398) y M4 (#2399). Android en un emulador e iOS en un simulador, en la misma máquina o en un Mac al que se llega por SSH; vistas web, coordenadas, enlaces profundos y alertas. Las llamadas de iOS se probaron contra un servidor con guion y se ejecutaron en un simulador el 2026-10-28 y el 2026-10-29 con la aplicación Ajustes integrada y con UIKitCatalog de Appium (una instalación `.zip`, alertas, `scroll` con `until`, una vista web). No se ejecutaron en un simulador: una carpeta `.app`, las coordenadas, `openURL`, `hideKeyboard` y la etiqueta `button` de la alerta (véase "iOS"; las notas detalladas de las ejecuciones no se publican).

Para comprobar que funciona: ejecute las pruebas herméticas (`go test ./executors/mobile/ -count=1`, sin dispositivo), y después la
prueba real con su propio Appium y emulador (véase "Ejecutar las pruebas"). Las pruebas herméticas se comunican con un servidor falso con guion, por lo que
no pueden demostrar que Appium 3 acepta todas las peticiones: solo la prueba real lo demuestra.

## Requisitos del host {#host-prerequisites}

Para preparar un Mac para Android e iOS, véase "Preparar el Mac" más abajo.

- Appium 3 con el controlador UiAutomator2 (probado con Appium 3.9.0 y uiautomator2 8.7.2; Appium 2.x no está verificado), iniciado de modo que **escuche solo en la dirección de loopback** (127.0.0.1).
- Un emulador de Android (o un dispositivo) visible para Appium.
- Las build-tools del SDK de Android (`sdkmanager "build-tools;34.0.0"`) cuando `mobile.app` nombra un `.apk`: UiAutomator2 necesita `aapt2` para leerlo. Appium guarda en caché la búsqueda de sus herramientas, así que reinícielo tras instalarlas. `ANDROID_HOME` debe estar definida en el shell que inicia Appium.
- venom en la misma máquina. M1 instala la aplicación desde una ruta de archivo local que Appium lee, de modo que ambos deben compartir host.

El executor no necesita la opción `--relaxed-security` en el servidor Appium.

## Variables {#variables}

Defínalas en `vars: mobile:` de la suite (o en un `.venomrc`, o con `--var`). La sesión se abre una vez por caso de prueba,
antes de su primer paso `mobile`, y se cierra tras su último paso, también cuando un paso ha fallado.

```yaml
vars:
  mobile:
    url: http://127.0.0.1:4723
    platform: android
    device: emulator-5554
    app: ./build/app-debug.apk
```

| Variable | Obligatoria | Significado |
|---|---|---|
| `mobile.url` | sí | Dirección del servidor Appium, `http` o `https`, sin credenciales. |
| `mobile.platform` | sí | `android` o `ios`. |
| `mobile.device` | sí | El dispositivo o emulador. En Android se envía a Appium como `appium:deviceName` y como `appium:udid`. En iOS es el nombre de un simulador (se envía solo como `appium:deviceName`) o un UDID (se envía también como `appium:udid`). El executor nunca elige un dispositivo por usted. Use el número de serie de adb, como `emulator-5554`; Appium rechaza el nombre de un AVD como `venom_test` ("not in the list of connected devices"). |
| `mobile.osVersion` | no | Se envía como `appium:platformVersion`. El informe muestra la versión con la que responde el servidor, o si no este valor. |
| `mobile.app` | no | Ruta de la aplicación que se instala al abrir la sesión: un `.apk` en Android, una carpeta `.app` de simulador o su `.zip` en iOS. Una ruta relativa se resuelve respecto a la carpeta de la suite. |
| `mobile.appSHA256` | no | SHA-256 esperado del `.apk` o del `.zip`, en hexadecimal (mayúsculas o minúsculas). Requiere `mobile.app`. Se rechaza con una carpeta `.app`. |
| `mobile.appId` | no | Identificador de paquete (Android) o de bundle (iOS) de la aplicación. Sin `mobile.app`, la sesión inicia esta aplicación ya instalada, y en iOS entonces es obligatorio. Con ambos, es también el valor predeterminado de `launch` y `terminate`. |
| `mobile.ssh.host` | no | Host (nombre o dirección) del Mac. Si se define, venom llega al servidor Appium mediante un túnel SSH hacia el loopback del propio host, en el puerto de `mobile.url`, y copia allí la aplicación. Sin él se usa `mobile.url` tal cual. |
| `mobile.ssh.user` | con `ssh.host` | Usuario SSH. |
| `mobile.ssh.key` | con `ssh.host` | Ruta de la clave privada. Solo autenticación por clave, sin contraseña. |
| `mobile.ssh.passphrase` | no | Frase de contraseña de la clave. Declárela en `secrets`. |
| `mobile.ssh.port` | no | Puerto SSH, 22 por defecto. |
| `mobile.ssh.knownhosts` / `mobile.ssh.hostkey` | no | Comprobación de la clave del host, como en el executor ssh: un archivo known_hosts, o una clave fija. No ambas. Con ninguna, venom avisa una vez por ejecución y no comprueba la clave del host. |
| `mobile.capabilities` | no | Capacidades de Appium adicionales, como un mapa. Véase más abajo. |
| `mobile.syncTimeout` | no | Segundos que el executor espera a un elemento, `10` por defecto. |
| `mobile.sessionTimeout` | no | Segundos permitidos para abrir la sesión, `180` por defecto en Android y `300` en iOS, donde la primera sesión compila WebDriverAgent. |
| `mobile.idleTimeout` | no | Segundos que el servidor mantiene abierta la sesión sin recibir un comando, `120` por defecto (se envía como `appium:newCommandTimeout`). |

Un número negativo en cualquiera de los tres tiempos de espera se rechaza. Un valor incorrecto se rechaza antes de enviar cualquier petición.

### El archivo de la aplicación y su SHA-256 {#the-app-file-and-its-sha-256}

Con `mobile.app`, venom lee el archivo una vez y calcula su SHA-256. Si `mobile.appSHA256` está definido y difiere, el
caso de prueba falla **antes de abrir ninguna sesión**, y el mensaje da ambos hashes. El hash calculado se muestra siempre en
el informe (`appsha256`), de modo que un informe indica qué compilación se ejecutó. `mobile.appSHA256` sin `mobile.app` se rechaza.

Sin un túnel SSH, el hash cubre los bytes que venom leyó, y Appium instala desde la ruta después: un archivo reemplazado
entre ambos momentos no se detectaría. Con `mobile.ssh.*` el archivo se copia al Mac y el hash cubre los bytes enviados
(véase "Llegar al Mac mediante SSH").

### Capacidades adicionales {#extra-capabilities}

`mobile.capabilities` añade capacidades a la petición de sesión. Una capacidad que el executor define por sí mismo se rechaza
antes de cualquier petición, con un mensaje que nombra la variable que la controla. Los nombres se comparan sin el prefijo
`appium:`, y también dentro de `appium:options`.

| Capacidad | Definida por |
|---|---|
| `platformName` | `mobile.platform` |
| `deviceName`, `udid` | `mobile.device` |
| `platformVersion` | `mobile.osVersion` |
| `app` | `mobile.app` |
| `appPackage`, `bundleId` | `mobile.appId` |
| `automationName` | el executor (UiAutomator2 en Android, XCUITest en iOS) |
| `newCommandTimeout` | `mobile.idleTimeout` |
| `autoAcceptAlerts`, `autoDismissAlerts` | la acción alert: el paso gestiona las alertas, por lo que la sesión no debe hacerlo |

## Entrada {#input}

```yaml
  - action (mandatory): one of the 17 actions below
  - accessibilityId, text, id, xpath, uiautomator, css, name, predicate, classChain (one of them, see "Locators"): which element the step targets
  - choice (alert): accept (default) or dismiss, in any case
  - button (alert): the label of the button to press, instead of the default one
  - syncTimeout (optional): seconds to wait for the element in this step, overrides mobile.syncTimeout
  - value (fill): the text to type
  - clear (fill): true to clear the field first
  - attribute (read): read this attribute instead of the text
  - state (waitFor): present (default) or absent
  - direction (swipe, scroll): up, down, left or right
  - until (scroll): true to scroll until the element is visible
  - percent (swipe, scroll): the part of the area to cover, above 0 and at most 1, default 0.75
  - x, y (tap, swipe): the point to tap, or where the swipe starts; 0 is valid. Not with a locator
  - toX, toY (swipe): where a swipe from a point ends; give both, and no direction or percent
  - unit (tap, swipe): the unit of x, y, toX, toY: pixels (default, also px) or percent. Percent is of the window rect (0 to 100)
  - appId (launch, terminate): the package id, default mobile.appId
  - path (screenshot): the file to write, relative to the directory venom runs in
  - context (context): native, a web view name starting with WEBVIEW, or empty for the first web view
  - url (openURL): the deep link, scheme://..., at most 2048 bytes, no spaces or control characters
```

`text` es un **localizador** (encuentra un elemento por su texto visible). El texto que usted escribe es `value`.

### Localizadores {#locators}

Un paso que apunta a un elemento da **exactamente un** localizador. Cero o dos es un error antes de cualquier petición.

| Campo | Encuentra el elemento por |
|---|---|
| `accessibilityId` | su identificador de accesibilidad (descripción de contenido) |
| `text` | su texto visible exacto (un `text` de UiSelector); las comillas y las barras invertidas que contenga se escapan |
| `id` | su identificador de recurso |
| `xpath` | un XPath sobre la jerarquía de vistas |
| `uiautomator` | un selector de UiAutomator, enviado tal como está escrito |
| `css` | un selector CSS, solo dentro de una vista web |
| `name` | solo iOS: el nombre del elemento |
| `predicate` | solo iOS: una cadena NSPredicate, enviada tal como está escrita |
| `classChain` | solo iOS: una cadena de clases de iOS, enviada tal como está escrita |

`id` y `uiautomator` son solo de Android; `name`, `predicate` y `classChain` son solo de iOS. Un tipo de la otra plataforma
se rechaza antes de cualquier petición, con un mensaje que indica qué usar en su lugar. La correspondencia de iOS está en "iOS".

Se espera a un elemento: el executor vuelve a preguntar cada 500 ms hasta que aparece o se agota la espera (10 s por defecto,
o el `syncTimeout` del paso, o `mobile.syncTimeout`). Si el elemento nunca aparece, el fallo nombra el localizador y
la espera. Un elemento encontrado al primer intento no añade ningún retraso.

## Acciones y resultados {#actions-and-results}

| Acción | Necesita | Qué hace | Claves de resultado que rellena |
|---|---|---|---|
| `tap` | un localizador, o `x` e `y` (con `unit` opcional) | hace clic en el elemento, o toca el punto | `found` (con un localizador) |
| `fill` | un localizador, `value`, `clear` opcional | escribe `value` en el elemento, tras borrarlo cuando `clear: true`. Un `value` vacío no escribe nada | `found` |
| `clear` | un localizador | borra el elemento | `found` |
| `read` | un localizador, `attribute` opcional | lee el texto del elemento, o el atributo | `found`, `text` (sin atributo) o `value` (atributo; un atributo ausente se lee como vacío) |
| `count` | un localizador | cuenta las coincidencias en ese momento, sin esperar. Cero es una respuesta, no un fallo | `count`, `found` (verdadero cuando es mayor que cero) |
| `waitFor` | un localizador, `state` opcional | espera a que el elemento esté presente (por defecto) o ausente | `found` |
| `swipe` | `direction`, localizador y `percent` opcionales; o `x`, `y`, `toX`, `toY` (con `unit` opcional) para arrastrar de un punto a otro | desliza dentro del elemento, o dentro del área visible del dispositivo cuando no hay localizador; esa área es la ventana recortada un 10 por ciento en cada lado, para no tocar nunca los bordes de los gestos del sistema | `found` cuando se dio un localizador |
| `scroll` | `direction`, localizador y `percent` opcionales | se desplaza dentro del elemento o del área visible (la ventana recortada un 10 por ciento en cada lado, para no tocar nunca los bordes de los gestos del sistema). Con `until: true` y un localizador, se desplaza por el área visible hasta que ese elemento esté en pantalla, como máximo 10 desplazamientos | `found` cuando se dio un localizador |
| `back` | nada | pulsa el botón Atrás | ninguna |
| `hideKeyboard` | nada | oculta el teclado en pantalla. Si no se muestra ninguno, no es un fallo | ninguna |
| `launch` | `appId` o `mobile.appId` | trae la aplicación al primer plano (`mobile: activateApp`) | ninguna |
| `terminate` | `appId` o `mobile.appId` | detiene la aplicación (`mobile: terminateApp`) | ninguna |
| `screenshot` | `path` opcional | hace una captura de pantalla. Con `path`, escribe allí el PNG (modo 0600) | `found`, `screenshot` (la ruta) |
| `source` | nada | devuelve la jerarquía de vistas como XML | `found`, `source` |
| `context` | `context`: `native`, el nombre de una vista web (`WEBVIEW_...`), o vacío para la primera vista web | cambia entre la aplicación nativa y una vista web. Con una vista web, espera (la espera del paso) a que aparezca en la lista. Un localizador `css` necesita una vista web | `context` (el activo), `contexts` (la lista que dio el servidor; vacía tras un cambio a nativo) |
| `openURL` | `url` | abre un enlace profundo: `url` scheme://..., restringido al paquete de la aplicación cuando se conoce (`mobile: deepLink`) | ninguna |
| `alert` | `choice` y `button` opcionales | solo iOS: acepta o descarta la alerta del sistema abierta (véase "iOS") | `text` (el texto de la alerta) |

Una acción desconocida se rechaza con la lista de las admitidas. Donde una acción difiere en iOS, véase "Diferencias con Android".

## Salida {#output}

```yaml
  - result.text: the text read (read)
  - result.value: the attribute read (read with attribute)
  - result.count: the number of matches (count)
  - result.found: true when the step found its element, or the screen content
  - result.screenshot: the path written (screenshot with path)
  - result.source: the view hierarchy as XML (source)
  - result.timeseconds: duration of the step
  - result.err: the failure of the step, empty when it worked
  - result.platform, result.device, result.osversion: where the test ran
  - result.app, result.appsha256, result.appid: what ran
```

Los datos de la ejecución (`platform`, `device`, `osversion`, `app`, `appsha256`, `appid`) se repiten en **todos** los resultados, de modo que cada
caso de prueba de un informe muestra qué compilación se ejecutó en qué dispositivo. Una clave sin valor está ausente del resultado.

Aserción predeterminada: `result.err ShouldBeEmpty`.

### Errores {#errors}

- Un **uso incorrecto** (dos localizadores, una acción desconocida, un `direction` no válido, un ajuste que falta) es un error de venom antes de enviar
  ninguna petición. El paso falla con un mensaje que nombra la causa.
- Un **fallo en el servidor o en el dispositivo** (elemento no encontrado, un comando rechazado, un servidor inalcanzable, una llamada que
  se cuelga) aparece en `result.err` como `action <name>: <message>`. El paso falla en la aserción predeterminada y la ejecución continúa.
  La traza de pila de Appium nunca se incluye en el resultado.

## Vistas web y aplicaciones híbridas {#web-views-and-hybrid-apps}

Una aplicación híbrida muestra páginas web dentro de una pantalla nativa. La acción `context` cambia entre ambas.

- `context: native` vuelve a la aplicación nativa.
- `context: WEBVIEW_<package>` cambia a esa vista web.
- Un `context` vacío cambia a la primera vista web que enumera el servidor.

Para una vista web, el executor espera hasta la espera del paso (`syncTimeout`, `mobile.syncTimeout`) a que aparezca en la lista, de modo que
se encuentra una página que carga tarde. Si ninguna aparece a tiempo, el fallo enumera los contextos que el servidor sí informó. El
resultado contiene `context` (el activo) y `contexts` (la lista que dio el servidor).

Dentro de una vista web la búsqueda la gestiona chromedriver, así que `css` y `xpath` son los localizadores que se deben usar. `text`,
`accessibilityId` y `uiautomator` son estrategias nativas y el servidor las rechaza en una vista web
(comprobado en un dispositivo, 2026-10-28: `accessibilityId` devolvió un error); lo que significa `id` allí (el id HTML) no está verificado. Tras
`context: native`, un localizador nativo funciona como antes. `css` en el contexto nativo se rechaza con un mensaje antes de cualquier búsqueda. `source` devuelve el HTML de la página en una vista web y la jerarquía
XML en el contexto nativo.

```yaml
      - type: mobile
        action: context
        context: WEBVIEW_com.example.app
      - type: mobile
        action: fill
        css: "#email"
        value: demo-user
      - type: mobile
        action: tap
        css: "button[type=submit]"
      - type: mobile
        action: context
        context: native
      - type: mobile
        action: waitFor
        accessibilityId: welcome_banner
```

### Lo que necesita la vista web {#what-the-web-view-needs}

- **La depuración debe estar activada.** La aplicación tiene que habilitar la depuración de WebView. Sin ella el servidor solo enumera `NATIVE_APP`, y
  un cambio a una vista web falla con esa lista.
- **Un chromedriver compatible.** UiAutomator2 controla una vista web mediante un chromedriver cuya versión coincide con la de WebView en
  el dispositivo. Una discrepancia hace que el cambio falle en el servidor. Defina `appium:chromedriverExecutable` (una ruta en el host
  de Appium) mediante `mobile.capabilities`, o en la propia configuración del servidor:

  ```yaml
  vars:
    mobile:
      capabilities:
        appium:chromedriverExecutable: /opt/chromedrivers/chromedriver-114
  ```
- **La descarga automática es decisión del operador.** El servidor Appium puede obtener un chromedriver por sí mismo
  (`chromedriver_autodownload`). Eso requiere una opción del servidor que define el operador en el servidor Appium. venom nunca
  la envía y nunca necesita `--relaxed-security` ni `--allow-insecure`.

## Aplicaciones sin etiquetas y coordenadas {#unlabelled-apps-and-coordinates}

Una aplicación que no publica etiquetas de accesibilidad (algunas compilaciones de lanzamiento de Flutter, juegos, vistas dibujadas a medida) solo se puede
manejar por coordenadas. Si la aplicación tiene alguna etiqueta, identificador o texto, prefiera un localizador: una coordenada se rompe cuando cambian el diseño,
el tamaño de pantalla o la fuente.

`tap` y `swipe` aceptan un punto en lugar de un localizador:

| Campo | Significado |
|---|---|
| `x`, `y` | el punto que se toca, o donde empieza el deslizamiento. 0 es un valor válido |
| `toX`, `toY` | donde termina el deslizamiento. Dé ambos, y ningún `direction` ni `percent` |
| `unit` | `pixels` (por defecto, también `px`) o `percent` |

Con `unit: percent` el punto es una proporción, de 0 a 100, del rectángulo de la ventana: `x: 50` e `y: 50` es el centro. En una
ventana de 1080x2400 eso envía el punto 540, 1200. Los píxeles se envían tal como se dan y el servidor los acota. Un punto con un
localizador, `x` sin `y`, un valor negativo, NaN o un infinito, un valor en píxeles superior a 10 millones, o un porcentaje superior a 100
se rechaza antes de cualquier petición.

Las coordenadas en porcentaje y `swipe` o `scroll` sin localizador leen el tamaño de la ventana del servidor. Dentro de una vista web ese
tamaño puede estar en unidades del navegador, no en píxeles del dispositivo: vuelva a nativo (`context: native`) antes de usar
coordenadas en porcentaje o un `swipe` y un `scroll` sin localizador. No verificado en un dispositivo: la ejecución manual usó coordenadas en píxeles y en porcentaje
solo en el contexto nativo.

```yaml
      - type: mobile
        action: tap
        x: 50
        y: 50
        unit: percent
      - type: mobile
        action: swipe
        x: 540
        y: 1800
        toX: 540
        toY: 600
```

## Enlaces profundos {#deep-links}

`openURL` abre un enlace en la aplicación, mediante `mobile: deepLink` de Appium:

```yaml
      - type: mobile
        action: openURL
        url: myapp://orders/42
```

- `url` debe tener la forma `scheme://...`, sin espacios ni caracteres de control, de 2048 bytes como máximo. Cualquier otra cosa se
  rechaza antes de cualquier petición, y el mensaje no repite el enlace (puede contener un token).
- El enlace se restringe al paquete de la aplicación (el `appId` del paso, o si no `mobile.appId`) cuando se conoce uno.
- Se aceptan esquemas personalizados: son el objetivo de un enlace profundo. El enlace se pasa a `mobile: deepLink` de Appium, que
  lo ejecuta mediante `am start`; venom no construye ningún comando de shell. En la pila probada (Appium 3.9.0, uiautomator2 8.7.2,
  emulador de Android 14) cinco enlaces con metacaracteres de shell llegaron al servidor, fallaron porque nada los gestionaba,
  y no crearon ningún archivo en el dispositivo. Un Appium o un controlador más antiguos pueden no escaparlos: mantenga fuera de las suites los enlaces profundos
  que provengan de una entrada no confiable.

## Límites {#limits}

| Límite | Valor |
|---|---|
| Una llamada al servidor | 60 s |
| Abrir la sesión | 180 s (`mobile.sessionTimeout`) |
| Sesión inactiva en el servidor | 120 s (`mobile.idleTimeout`) |
| Pausa entre dos intentos de encontrar un elemento | 500 ms |
| Espera predeterminada de un elemento | 10 s (`mobile.syncTimeout`, `syncTimeout`) |
| Desplazamientos de `scroll` con `until` | 10 |
| Una captura de pantalla decodificada | 12 MiB |
| Cerrar la sesión | 15 s |
| Cada una de las dos peticiones de diagnóstico | 15 s |

Una redirección del servidor no se sigue. El `timeout:` propio del paso sigue aplicándose además de estos límites.

## Diagnósticos en un paso que falla {#diagnostics-on-a-failing-step}

Cuando un paso falla en el servidor o en el dispositivo, el executor guarda dos archivos en el directorio donde se ejecuta venom:

- `screen.png`: una captura de pantalla del dispositivo;
- `screen.dump.xml`: la jerarquía de vistas.

**Advertencia: ambos archivos muestran lo que hay en la pantalla, incluido un secreto que usted haya escrito o que la aplicación muestre.** No están
cubiertos por la ocultación de secretos de los informes de venom. Se escriben con modo 0600 en la carpeta de trabajo. No los publique
como artefactos de compilación. El siguiente paso que falle los reemplaza; un paso que pasa no escribe ni borra nada. Un
fallo al obtener un archivo es una advertencia en el registro y no oculta el fallo propio del paso. Un error de uso incorrecto, o una sesión
que el servidor ya no tiene, no escribe nada.

Dos ejecuciones de venom iniciadas en el **mismo directorio** se sobrescriben los archivos. Ejecútelas desde directorios distintos.

## Llegar al Mac mediante SSH {#reaching-the-mac-through-ssh}

Mantenga Appium escuchando en el loopback del Mac (`127.0.0.1:4723`); no tiene contraseña propia. Defina `mobile.ssh.host`, `mobile.ssh.user` y `mobile.ssh.key` y mantenga `mobile.url` como la dirección que ve el Mac (`http://127.0.0.1:4723`): venom abre el túnel cuando empieza el caso de prueba y lo cierra cuando termina. El túnel escucha solo en `127.0.0.1` y siempre se conecta al loopback del Mac.

- **Clave del host:** defina `mobile.ssh.knownhosts` o `mobile.ssh.hostkey`. Una clave que no coincide rechaza la conexión antes de cualquier petición.
- **El archivo de la aplicación:** con un túnel, `mobile.app` se copia a `/tmp/venom-mobile-<random>/app.apk` (`app.zip` en iOS) en el Mac, se instala desde allí y se elimina cuando termina la sesión. Ningún valor de la suite aparece en un comando que se ejecute en el Mac. `mobile.appSHA256` se comprueba sobre los bytes enviados; una discrepancia falla antes de cualquier sesión y elimina la copia. Una ejecución interrumpida a la fuerza puede dejar la carpeta: busque `/tmp/venom-mobile-*`.
- **Límites:** una URL `https://` a través del túnel se verifica contra `127.0.0.1`, así que use `http://` dentro del túnel. La conexión y el handshake SSH están acotados por `mobile.sessionTimeout` (180 s por defecto), de modo que un host que no responde hace fallar el Setup en lugar de bloquearlo.
- **http sin cifrar y sin túnel:** una URL `http://` hacia otra máquina imprime un aviso por ejecución; una URL de loopback no imprime ninguno.

## Notas de seguridad {#security-notes}

- **Appium debe escuchar solo en loopback.** Un servidor Appium expuesto en la red permite a cualquiera que llegue a él controlar el
  emulador y la máquina. En M1, venom se conecta a Appium en el mismo host.
- La aplicación se instala desde un **archivo local**, así que en M1 Appium y venom deben compartir host.
- **Las capturas de pantalla, el volcado y `result.source` muestran lo que hay en la pantalla, incluido un secreto que usted haya escrito.** NO están
  cubiertos por la ocultación de secretos de los informes de venom, el mismo límite que tiene la traza del executor web. Mantenga `screen.png` y
  `screen.dump.xml` fuera de los artefactos publicados y de las carpetas de compilación compartidas.
- `fill` nunca repite el texto escrito en un resultado ni en el registro de llamadas de `-v`. Con `-v`, se registra cada llamada al servidor (método, ruta, duración; nunca un cuerpo).
- Solo se usan endpoints de Appium 3, y el executor no necesita `--relaxed-security`.
- **El contenido de la página de una vista web y el texto escrito en ella pueden aparecer en las capturas de pantalla y en `result.source`**, como cualquier
  pantalla nativa, y la ocultación de secretos no los cubre. El texto de página leído con `read` es un valor de resultado como cualquier otro.
- **La URL de un enlace profundo se pasa a `mobile: deepLink` de Appium, que la ejecuta mediante `am start`.** venom nunca
  construye por sí mismo un comando de shell; se comprueban la forma y la longitud del enlace, y se restringe al paquete de la aplicación cuando se conoce uno.
  Appium escapó los metacaracteres de shell en la pila probada (Appium 3.9.0, uiautomator2 8.7.2, emulador arm64 de Android 14,
  medido el 2026-10-28). Un Appium o un controlador más antiguos pueden no hacerlo, así que mantenga fuera de las suites los enlaces profundos que provengan de una entrada no confiable.
- **El chromedriver se ejecuta en el host de Appium**, y qué binario sea sigue siendo decisión del operador. venom no envía ninguna
  opción que active `--relaxed-security` o `--allow-insecure`, y el servidor no necesita ninguna de las dos.
- **El texto de una alerta de iOS** va a `result.text` y a ningún otro sitio. No se escribe en una línea de registro ni en un error, pero
  es un valor de resultado como cualquier otro, así que un secreto que contenga solo se oculta si es un secreto declarado.
- **Una carpeta `.app` se copia al Mac como `app.zip`**, en una carpeta con nombre de venom (`/tmp/venom-mobile-<random>`, modo 0700),
  sin ningún valor de la suite en ningún comando que se ejecute allí. venom rechaza un `.app` que contenga un enlace simbólico, y no calcula ningún hash de
  una carpeta: su integridad no se comprueba. Dé un `.zip` y `mobile.appSHA256` cuando se necesite.
- **El controlador XCUITest y WebDriverAgent se ejecutan en el Mac** y los instala el operador. venom no los descarga ni los compila;
  la primera sesión compila allí WebDriverAgent.

## iOS {#ios}

Con `mobile.platform: ios` la misma suite controla una aplicación en un **simulador** de iOS mediante el controlador XCUITest. Un
iPhone real no está cubierto. Las llamadas de iOS se implementan a partir del comportamiento documentado del controlador XCUITest y se prueban contra un
servidor con guion. Se ejecutaron en un simulador (iPhone 17, iOS 26.5, Appium 3.9.0, controlador XCUITest 12.22.3), en dos ejecuciones:

- 2026-10-28, la aplicación Ajustes integrada iniciada por su identificador de bundle, suite `examples/handrun-settings.yml`, 3 de 3 casos de
  prueba pasaron: apertura de sesión, `waitFor`, `tap`, deslizamiento por dirección, `terminate`, `launch`, los localizadores `text`, `name`,
  `predicate` y `classChain`, `count`, y `alert` sin ninguna alerta abierta.
- 2026-10-29, UIKitCatalog v4.0.5 de Appium dado como `UIKitCatalog.zip` con `mobile.appSHA256`, a través del túnel SSH,
  suite `examples/handrun-uikitcatalog.yml`, estado final PASS: la instalación del `.zip`, `alert` aceptar (el texto de la alerta
  volvió en `result.text`) y descartar, `scroll` con `until`, y una vista web (`context` informó de un contexto
  `WEBVIEW`, `css: body` contó más de 0, `context: native` volvió). La vista web de iOS no necesitó chromedriver.

No se ejecutaron en un simulador: una carpeta `.app`, las coordenadas, `openURL`, `hideKeyboard`, la etiqueta `button` de la alerta, las coordenadas
en porcentaje. Las notas detalladas de las ejecuciones no se publican.
Las filas de una lista como la de Ajustes no son `XCUIElementTypeCell` en iOS 26: escriba una cadena de clases como `**/*[...]`, no con un tipo de celda.
La primera sesión tardó minutos (compilación de WebDriverAgent), por eso el tiempo de espera predeterminado de la sesión es 300 s.

### Requisitos en el Mac {#prerequisites-on-the-mac}

- Xcode y sus herramientas de línea de comandos, y al menos un simulador.
- Appium 3 con el controlador XCUITest: `appium driver install xcuitest`. Mantenga Appium en la dirección de loopback, como en Android.
- La primera sesión compila WebDriverAgent y tarda minutos. Por eso `mobile.sessionTimeout` es 300 s por defecto en iOS.
- Un simulador no necesita firma de código.
- En un Mac con Apple Silicon la aplicación debe tener una porción arm64, y las imágenes del emulador de Android también son arm64 allí. Appium rechaza una
  `.app` de iOS con solo porciones i386 o x86_64 ("does not support the arm64 Simulator architecture"); un
  APK con solo bibliotecas `armeabi-v7a` y `x86` falla con `INSTALL_FAILED_NO_MATCHING_ABIS` (necesita `arm64-v8a`, o
  ninguna biblioteca nativa). TheApp v1.10.0 (2019) de Cloud Grey falló de ambas maneras en el Mac del responsable del proyecto.
- Una muestra lista para probar iOS: UIKitCatalog de Appium (`UIKitCatalog.zip`, una compilación para simulador con porciones arm64 y x86_64,
  identificador de bundle `com.example.apple-samplecode.UICatalog`), desde la página de versiones de
  <https://github.com/appium/ios-uicatalog>. La suite `examples/handrun-uikitcatalog.yml` se ejecuta sobre ella.

### Ajustes que cambian en iOS {#settings-that-change-on-ios}

| Ajuste | En iOS |
|---|---|
| `mobile.device` | El nombre de un simulador (`iPhone 16`), o su UDID. Un UDID se envía también como `appium:udid`; un nombre solo como `appium:deviceName`. |
| `mobile.osVersion` | Se envía como `appium:platformVersion`, como en Android. |
| `mobile.app` | Una carpeta `.app` de simulador, o su `.zip`. Un **`.ipa` es una compilación para dispositivo y no se instala en un simulador**: se rechaza antes de cualquier petición. Compile en su lugar una `.app` de simulador. |
| `mobile.appSHA256` | Solo para un `.zip`. Una carpeta `.app` no tiene un único archivo que calcular, así que el ajuste se rechaza con una. |
| `mobile.appId` | El identificador de bundle. Sin `mobile.app`, la aplicación instalada se inicia por su identificador de bundle y entonces `mobile.appId` es obligatorio. |
| `mobile.sessionTimeout` | 300 s por defecto. |

Mediante SSH, una carpeta `.app` se comprime en un zip en la máquina que ejecuta venom y se copia al Mac como `app.zip`; un `.zip` se copia
tal cual. Véase "Llegar al Mac mediante SSH".

### Localizadores en iOS {#locators-on-ios}

| Campo | Encuentra el elemento por |
|---|---|
| `accessibilityId` | su identificador de accesibilidad |
| `text` | su etiqueta: se envía como el predicado `label == "<text>"`, con las comillas, las barras invertidas y los saltos de línea del texto escapados |
| `name` | su nombre |
| `predicate` | una cadena NSPredicate, enviada tal como está escrita |
| `classChain` | una cadena de clases de iOS, enviada tal como está escrita |
| `xpath` | un XPath sobre la jerarquía de vistas |
| `css` | un selector CSS, solo dentro de una vista web |

`id` y `uiautomator` se rechazan en iOS; `name`, `predicate` y `classChain` se rechazan en Android. Como en Android, un
paso da exactamente un localizador.

### Diferencias con Android {#differences-from-android}

| Acción | En iOS |
|---|---|
| `tap`, `fill`, `clear`, `read`, `count`, `waitFor`, `screenshot`, `source`, `context` | Los mismos comandos que en Android. |
| `tap` y `swipe` por `x`, `y` | El punto está en puntos, no en píxeles. `tap` usa `mobile: tap`, `swipe` usa `mobile: dragFromToForDuration` durante medio segundo. `unit: percent` es del rectángulo de la ventana, que iOS informa en puntos. |
| `swipe`, `scroll` por `direction` | `mobile: swipe` y `mobile: scroll`, sobre el elemento o sobre toda la aplicación. `percent` se rechaza: XCUITest no tiene ese argumento. |
| `scroll` con `until` | Repite `mobile: scroll` hasta 10 veces. El servidor no indica cuándo termina la lista, así que el bucle se detiene al encontrar el elemento o al llegar al límite. |
| `back` | Se rechaza: iOS no tiene tecla Atrás. Toque el propio control de retroceso de la aplicación. |
| `hideKeyboard` | `mobile: hideKeyboard`. Un teclado que no se muestra no es un fallo. |
| `launch`, `terminate` | `mobile: activateApp` y `mobile: terminateApp`, que nombran la aplicación por `bundleId` (`appId` o `mobile.appId`). |
| `openURL` | `mobile: deepLink` con la `url`, y el `bundleId` cuando se conoce. |
| `alert` | Solo iOS. En Android se rechaza. |

### Alertas {#alerts}

Una aplicación de iOS puede lanzar una alerta del sistema, por ejemplo una solicitud de permiso. La acción `alert` lee su texto y después la acepta
o la descarta:

```yaml
      - type: mobile
        action: tap
        accessibilityId: enable_location
      - type: mobile
        action: alert
        choice: accept
        assertions:
          - result.err ShouldBeEmpty
          - result.text ShouldContainSubstring Location
      - type: mobile
        action: alert
        choice: dismiss
        button: Don't Allow
```

- `choice` es `accept` (por defecto) o `dismiss`. Cualquier otro valor se rechaza antes de cualquier petición.
- Sin `button`, el paso responde con el comando simple de aceptar o descartar. Con `button`, pulsa el botón con
  esa etiqueta (`mobile: alert`).
- `result.text` contiene el texto de la alerta. **Una alerta puede mostrar un secreto** (un código, un nombre): el texto nunca se escribe en una línea
  de registro ni en un mensaje de error, y la ocultación de secretos de los informes de venom solo cubre los secretos declarados.
- Sin ninguna alerta abierta, el paso falla con `no alert open` en `result.err`.
- La sesión no debe responder a las alertas por sí misma: `autoAcceptAlerts` y `autoDismissAlerts` en `mobile.capabilities` se
  rechazan, porque el paso `alert` las gestiona.
- Un enlace profundo en un simulador puede lanzar una alerta de confirmación del sistema ("Open in ...?"). Haga que a un `openURL` le siga un paso `alert`
  cuando su aplicación se abra de ese modo, o el paso siguiente encontrará la alerta en su camino.

## Pendiente {#not-yet}

Previsto, rechazado o ausente hoy:

- Un dispositivo iOS real: solo se contemplan simuladores.
- Flutter en el simulador de iOS es solo una compilación de depuración: Flutter no genera ninguna aplicación de lanzamiento para el simulador.
- Aplicaciones sin una porción arm64, en un Mac con Apple Silicon.
- Inicio de sesión con contraseña mediante SSH: solo autenticación por clave.
- Un chromedriver descargado por usted: para una vista web de Android el operador proporciona uno compatible.
- Aún no ejecutado en un simulador (véase "iOS"; las notas detalladas de las ejecuciones no se publican): la instalación de una carpeta `.app` (solo se
  ejecutó un `.zip`), las coordenadas `x`/`y` y en porcentaje, `openURL`, `hideKeyboard` y la etiqueta `button` de la alerta. La suite
  `examples/handrun-ios.yml` está escrita para la instalación de la carpeta `.app`.

## Ejemplo {#example}

Cada paso siguiente es una forma que ejecutan las pruebas de este executor.

```yaml
name: Mobile smoke
vars:
  mobile:
    url: http://127.0.0.1:4723
    platform: android
    device: emulator-5554
    osVersion: "14"
    app: ./build/app-debug.apk
    appSHA256: <sha-256 of the build, in hex>
    appId: com.example.app
    syncTimeout: 5

testcases:
  - name: the home screen is usable
    steps:
      - type: mobile
        action: waitFor
        accessibilityId: login_button
        syncTimeout: 20
      - type: mobile
        action: count
        id: com.example.app:id/item
        assertions:
          - result.count ShouldBeGreaterThan 0
      - type: mobile
        action: fill
        id: com.example.app:id/username
        value: demo-user
        clear: true
      - type: mobile
        action: hideKeyboard
      - type: mobile
        action: tap
        accessibilityId: login_button
      - type: mobile
        action: read
        id: com.example.app:id/title
        assertions:
          - result.err ShouldBeEmpty
          - result.text ShouldEqual Welcome
      - type: mobile
        action: scroll
        direction: down
        until: true
        text: Settings
      - type: mobile
        action: screenshot
        path: home.png
      - type: mobile
        action: back
      - type: mobile
        action: terminate
      - type: mobile
        action: launch
```

## Preparar el Mac {#set-up-the-mac}

Úselo cuando venom se ejecute en otra máquina (Windows, Linux) y el emulador y el simulador se ejecuten en un Mac. Todo lo
siguiente se instala en el Mac. venom no instala nada de ello.

1. **Appium 3** con ambos controladores: `appium driver install uiautomator2` y `appium driver install xcuitest`. Inícielo de modo
   que escuche solo en la dirección de loopback: `appium --address 127.0.0.1 --port 4723`. Defina `ANDROID_HOME` en el
   shell que lo inicia.
2. **Android:** el SDK de Android (platform-tools, `build-tools;34.0.0`) y una imagen de emulador **arm64** en un Mac con Apple
   Silicon. Inicie el emulador antes de la ejecución.
3. **iOS:** Xcode, sus herramientas de línea de comandos y al menos un simulador. Arránquelo, o deje que Appium lo arranque. La primera sesión
   compila WebDriverAgent y tarda minutos.
4. **Vistas web de Android:** un chromedriver que coincida con la WebView del emulador, y su ruta en
   `appium:chromedriverExecutable` (véase "Lo que necesita la vista web"). Las vistas web de iOS no necesitan ninguno.
5. **SSH:** una clave que el Mac acepte para el usuario que indique en `mobile.ssh.user`. El inicio de sesión con contraseña no se admite.

Problemas vistos en un Mac real:

- **La primera ejecución de una herramienta puede quedarse bloqueada por SSH.** Gatekeeper de macOS puede pedir una confirmación que nadie ve en una sesión
  SSH, y el binario se queda esperando. Ejecute una vez `flutter --version` (o la herramienta en cuestión) en un Terminal en el propio Mac
  y responda al aviso. Las herramientas instaladas con un cask de Homebrew también pueden llevar la marca de cuarentena. Quitarla con
  `xattr -dr com.apple.quarantine <folder>` es decisión suya: hágalo solo en una carpeta de confianza.
- **Las aplicaciones necesitan una porción arm64** en un Mac con Apple Silicon (véase "Requisitos en el Mac").
- **Flutter solo compila aplicaciones de depuración para el simulador de iOS.** Una compilación de lanzamiento de Flutter solo es posible en Android.
- **CocoaPods necesita una configuración regional UTF-8** (`export LANG=en_US.UTF-8 LC_ALL=en_US.UTF-8`), por ejemplo cuando se ejecuta desde un
  script iniciado por SSH.

## Aplicaciones de ejemplo {#sample-apps}

Aplicaciones públicas que usaron las ejecuciones del responsable del proyecto. Todas están listas para arm64.

| Aplicación | Plataforma | De dónde |
|---|---|---|
| ApiDemos | Android | `ApiDemos-debug.apk` de Appium, de las versiones de <https://github.com/appium/android-apidemos> |
| UIKitCatalog v4.0.5 | Simulador de iOS | `UIKitCatalog.zip`, de las versiones de <https://github.com/appium/ios-uicatalog> |
| Aplicación contador de Flutter | ambas | `flutter create`, compilada por `examples/build-sample-apps.sh` (APK de lanzamiento de Android, aplicación de depuración para el simulador de iOS) |
| Aplicación React Native | ambas | `react-native init`, compilada por el mismo script (APK de lanzamiento de Android, aplicación Release para el simulador de iOS) |

El script se ejecuta en el Mac. Instala el SDK de Flutter y CocoaPods si faltan, compila las cuatro aplicaciones a partir de las plantillas propias de los
frameworks y las enumera al final. Ejecute antes `flutter --version` una vez a mano (véase "Preparar el Mac").

Las suites están en `examples/`: `examples/flutter.yml` y `examples/rn.yml` se ejecutan sin cambios en ambas plataformas, con el archivo de variables de
cada plataforma (`examples/android.vars.example.yml`, `examples/ios.vars.example.yml`: rellene los valores `<...>`); `examples/failing.yml` falla
a propósito para mostrar los archivos de diagnóstico. Las notas detalladas de las ejecuciones no se publican.

## Ejecutar las pruebas {#run-the-tests}

Pruebas herméticas (sin dispositivo, sin red; forman parte de `go test ./...`):

```bash
go test ./executors/mobile/ -count=1
```

Prueba real, tras la etiqueta de compilación `mobile`. Necesita Appium 3 con UiAutomator2 en loopback y un emulador, y lee
sus ajustes del entorno. Se omite con un mensaje cuando alguno no está definido.

```bash
export VENOM_MOBILE_URL=http://127.0.0.1:4723
export VENOM_MOBILE_DEVICE=emulator-5554
export VENOM_MOBILE_APP=/path/to/sample.apk        # or VENOM_MOBILE_APPID=<package id of an installed app>
export VENOM_MOBILE_LOCATOR_KIND=accessibilityId   # accessibilityId, text, id, xpath or uiautomator
export VENOM_MOBILE_LOCATOR_VALUE=<an element on the first screen>
export VENOM_MOBILE_EXPECT_TEXT=<its text>         # optional
export VENOM_MOBILE_OSVERSION=14                   # optional
go test -tags mobile ./executors/mobile/ -run Real -v -count=1
```

Abre una sesión, encuentra y lee el elemento, comprueba que un paso que pasa no deja diagnósticos, hace fallar un paso a
propósito y comprueba que aparecen `screen.png` y `screen.dump.xml`, y después cierra la sesión.

Dos pruebas reales más comparten las variables de sesión anteriores (`VENOM_MOBILE_URL`, `_DEVICE`, `_APP` o `_APPID`):

| Prueba | Variables adicionales | Qué comprueba |
|---|---|---|
| `TestReal_CoordinatesAndDeepLink` | ninguna | los deslizamientos en porcentaje y en píxeles y un toque al 50 % / 50 % tienen éxito; `openURL venomtest://nothing/handles/this` devuelve un `result.err` no vacío |
| `TestReal_IOSSession` | `VENOM_MOBILE_PLATFORM=ios`, `VENOM_MOBILE_DEVICE` el nombre de un simulador, `VENOM_MOBILE_APPID` un identificador de bundle (o `VENOM_MOBILE_APP`), `VENOM_MOBILE_IOS_LOCATOR` (identificador de accesibilidad en la primera pantalla, `General` por defecto para la aplicación Ajustes) | `waitFor`, deslizar hacia arriba, `terminate`, `launch`, y `alert` sin ninguna abierta responde `no alert open`; se omite salvo que `VENOM_MOBILE_PLATFORM` sea `ios` |
| `TestReal_WebViewContext` | `VENOM_MOBILE_CHROMEDRIVER` (ruta de un chromedriver compatible en el host de Appium) y `VENOM_MOBILE_WEBVIEW_LOCATOR` (identificador de accesibilidad que abre una pantalla con vista web) | `context` llega a un `WEBVIEW...`, `css: body` cuenta más de 0, `context: native` devuelve `NATIVE_APP`; se omite salvo que ambas estén definidas |
