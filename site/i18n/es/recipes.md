---
translates: recipes
title: Recetas
description: Escriba pruebas de navegador y de móvil como recetas que una comprobación y un control negativo pueden demostrar.
fingerprint: 3dd31333d7ad4d4487d1c297fdc7e02dc69595aee1619af22617d3bfb867d5d5
drafted_by: claude
drafted_on: 2026-11-18
status: machine-drafted
---
Una receta es una suite de venom que maneja una página web o una app móvil. Esta página muestra cómo organizar las recetas, nombrarlas, demostrar que prueban algo y mantener los secretos y los resultados fuera de los sitios equivocados. Necesita venom y, para las recetas web, un navegador que Playwright haya instalado. Las páginas del [executor web](web.html) y del [executor mobile](mobile.html) listan todas las acciones.

Una receta es una suite normal. Usa pasos `web` o pasos `mobile` y nada nuevo. Esta es una receta web pequeña. La dirección de la página es la variable `appUrl`, de modo que el mismo archivo se ejecuta contra cualquier copia de la página:

{{include site/examples/recipes/web-page.yml yaml}}

Ejecútela con la dirección de su página:

```
venom run --var appUrl=http://localhost:8000 web-page.yml
```

## Estructura de carpetas {#folder-layout}

Guarde las recetas de una funcionalidad en una carpeta bajo `tests/`, junto a las otras suites de venom. Por ejemplo:

```
tests/
  site/
    site-languages.yml
  recipes/
    reference/
      mobile/
        counter.yml
```

Una carpeta por funcionalidad. Uno o más archivos `.yml` dentro. Una carpeta con subcarpetas se comprueba nombrando las subcarpetas. Mantenga los archivos de variables con credenciales fuera de la carpeta, o ignorados por git (vea Credenciales).

## Nombres de los casos de prueba y el id de la spec {#testcase-names-and-the-spec-id}

Cada caso de prueba demuestra un escenario de aceptación de una spec. Empiece el nombre del caso de prueba con el id del escenario y después una descripción breve: `US1-AS3 an item typed in the input is added to the list`. El id tiene la forma `USn-ASm`: historia de usuario `n`, escenario de aceptación `m`.

Nombre la spec una sola vez, en una variable de la suite:

```
vars:
  spec: 014-web-mobile-recipes
```

El valor es el nombre de la carpeta de la spec. La comprobación de recetas lee `spec.md` en esa carpeta y busca los ids. La opción `--specs-dir` le indica dónde están las carpetas de las specs. Dos archivos de una misma carpeta no deben reclamar el mismo escenario.

Para ver qué escenarios demuestra una receta, añada `--coverage` con el id de la spec: `venom recipe check --specs-dir <specs folder> --coverage 014-web-mobile-recipes tests/recipes`. Después de los hallazgos, lista cada escenario de la spec, `US1-AS3 proven: <file> / <testcase>` o `US1-AS4 not proven`. Un testcase solo demuestra un escenario si no se omite y contiene una aserción que lee la app. La opción no cambia el estado de salida. El script de cobertura del dev set no lee archivos `.yml`; por eso, para contar las recetas use esta opción y no ese script.

## Selectores {#selectors}

Un paso `find`, `click` o `fill` recibe un selector. Use CSS o XPath. Prefiera un atributo de prueba estable, como `data-test`:

```
find: '[data-test="add-button"]'
```

Los ids y los nombres de clase cambian cuando alguien rediseña la página. Un atributo `data-test` existe solo para que las pruebas encuentren el elemento. Si la app no tiene ninguno, pida a los desarrolladores que los añadan antes de escribir la receta.

`find` no espera. Cuenta los elementos que existen en ese momento. Cuando la página carga datos después del primer pintado, espere antes con `wait`, o use un `click` o un `fill`, que esperan a su elemento. En un paso mobile, defina exactamente un localizador, como `accessibilityId`.

## Pasos de demostración {#proof-steps}

Una receta que pasa no demuestra nada hasta que la ha visto fallar. Tres pasos dan la demostración:

1. Repetición. Ejecute la receta con `venom run` contra la app. Todos los casos de prueba deben pasar. Repítala varias veces: un caso de prueba que pasa una vez y falla otra no es utilizable.
2. Control negativo. Para cada caso de prueba, copie la receta, cambie el primer valor esperado que viene de la app y ejecute la copia. La ejecución debe terminar con el estado 2 y nombrar ese caso de prueba. Deje intacto el archivo original.
3. Comprobación de la receta. Ejecute la comprobación, que lee los archivos sin navegador ni dispositivo:

```
venom recipe check --specs-dir <folder of the specs> tests/site
```

La comprobación termina con 0 cuando no encuentra nada, con 1 cuando informa de un hallazgo y con 2 cuando no puede leer su entrada. Cada hallazgo nombra el archivo, la línea, la regla y el caso de prueba. Informa de un caso de prueba sin id de escenario, de un id repetido, de un id que la spec no tiene, de un caso de prueba sin ninguna assertion sobre un valor leído de la app, de una assertion que no puede fallar, de un texto con aspecto de credencial y de una dirección escrita como literal.

La comprobación es estática. No puede leer dentro de un script `execute` y no puede saber si un valor esperado es correcto. El control negativo cubre ambas cosas.

## Lo que los executors no pueden hacer {#what-the-executors-cannot-do}

El executor web no tiene paso para hover, arrastrar ni un atajo de teclado, y `find` no puede leer un número como un ancho de desplazamiento. La acción `execute` ejecuta un script en la página y devuelve un valor sobre el que puede hacer assertions. Este ejemplo lee cuánto se desplaza la página hacia un lado y cuántos elementos lista:

{{include site/examples/recipes/web-execute.yml yaml}}

Si `execute` no basta, anote la carencia y no escriba una receta para ese escenario. No ponga todas las assertions de un caso de prueba sobre el resultado de un script sin un control negativo: la comprobación no puede leer el script.

Una receta mobile necesita un servidor Appium y un dispositivo o un emulador. No se ejecuta sin ellos. Esta lleva la etiqueta `# needs: device` en su primera línea, de modo que las pruebas del sitio la omiten donde no hay dispositivo:

{{include site/examples/recipes/mobile-counter.yml yaml}}

## Credenciales {#credentials}

Una receta no contiene contraseñas, tokens ni claves. Un inicio de sesión viene de variables. Páselas en la línea de comandos o en un archivo de variables que git ignore:

```
venom run --var-from-file login.vars.yml tests/site/login.yml
```

En la receta, escriba la variable: `text: "{{.password}}"`. La comprobación informa de un texto con aspecto de credencial en cualquier paso o valor por defecto.

Las recetas se ejecutan contra un entorno de pruebas, nunca contra producción. La dirección es una variable que define la pipeline, y `venom run` no le impide apuntarla a un sistema real. Mantenga la lista de hosts permitidos en su pipeline.

## Resultados y artefactos {#results-and-artifacts}

Un paso que falla puede escribir un volcado de la página, una captura de pantalla o una traza. Estos archivos pueden contener datos de la app. Escríbalos en una carpeta de resultados que git ignore y no los suba al repositorio. En CI, consérvelos poco tiempo, como unos pocos días, y dé acceso solo a las personas que lo necesitan: el almacenamiento es restringido. No los copie a un almacenamiento de larga duración.

## Variables no definidas {#undefined-variables}

Una variable que olvida pasar no detiene la ejecución por defecto. Venom deja `{{.password}}` como el texto literal `{{.password}}` y el paso escribe ese texto. La ejecución no falla por ello.

Pase `--strict-variables` para cambiarlo. Entonces venom se detiene con el estado de salida 2 y el mensaje `unknown variable(s) "password"`. Use la opción en toda pipeline que ejecute recetas:

```
venom run --strict-variables --var-from-file login.vars.yml tests/site/login.yml
```
