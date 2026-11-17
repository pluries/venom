---
translates: cli-usage
title: Uso de la CLI
description: Los comandos de venom y sus opciones.
fingerprint: 02ee0f4bbd022eb88342dc24560b249d66c272730ab0c8285af92241332fa21a
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# Uso de la CLI {#cli-usage}

La CLI `venom` se compone de varios comandos:

```bash
$ venom -h
Venom - RUN Integration Tests

Usage:
  venom [command]

Available Commands:
  help        Help about any command
  run         Run Tests
  version     Display Version of venom: venom version

Flags:
  -h, --help   help for venom

Use "venom [command] --help" for more information about a command.
```

Puede consultar la ayuda de un comando con `venom [command] -h`:

```bash
$ venom run -h

run integration tests

Usage:
  venom run [flags]

Examples:
  Run all testsuites containing in files ending with *.yml or *.yaml: venom run
  Run a single testsuite: venom run mytestfile.yml
  Run a single testsuite and export the result in JSON format in test/ folder: venom run mytestfile.yml --format=json --output-dir=test
  Run a single testsuite and export the result in XML and HTML formats in test/ folder: venom run mytestfile.yml --format=xml --output-dir=test --html-report
  Run a single testsuite and specify a variable: venom run mytestfile.yml --var="foo=bar"
  Run a single testsuite and load all variables from a file: venom run mytestfile.yml --var-from-file variables.yaml
  Run all testsuites containing in files ending with *.yml or *.yaml with verbosity: VENOM_VERBOSE=2 venom run

  Notice that variables initialized with -var-from-file argument can be overrided with -var argument

  More info: https://github.com/ovh/venom

Flags:
      --format string           --format:json, tap, xml, yaml (default "xml")
  -h, --help                    help for run
      --html-report             Generate HTML Report
      --lib-dir string          Lib Directory: can contain user executors. example:/etc/venom/lib:$HOME/venom.d/lib
      --output-dir string       Output Directory: create tests results file inside this directory
      --parallel int            Number of test files to run at the same time. 1 (default) runs them one after another (default 1)
      --stop-on-failure         Stop running Test Suite on first Test Case failure
      --strict-variables        Fail a step that names an unknown variable
      --var stringArray         --var cds='cds -f config.json' --var cds2='cds -f config.json'
      --var-from-file strings   --var-from-file filename.yaml --var-from-file filename2.yaml: yaml, must contains a dictionary
  -v, --verbose count           verbose. -v (INFO level in venom.log file), -vv to very verbose (DEBUG level) and -vvv to very verbose with CPU Profiling
```

## Ejecutar suites de pruebas en un orden determinado {#run-test-suites-in-a-specific-order}

- `venom run 01_foo.yml 02_foo.yml` ejecutará 01 antes que 02. 
- `venom run 02_foo.yml 01_foo.yml` ejecutará 02 antes que 01.

Si desea ordenar muchos archivos de suites de pruebas, puede usar comandos estándar, por ejemplo:

```bash
venom run `find . -type f -name "*.yml"|sort`
```

## Ejecutar varios archivos de prueba a la vez {#run-several-test-files-at-the-same-time}

De forma predeterminada, `venom` ejecuta un archivo de prueba cada vez. `--parallel N` ejecuta hasta `N` de ellos a la vez:

```bash
$ venom run --parallel 4
```

Las pruebas de integración pasan la mayor parte del tiempo esperando a otros sistemas, y esa espera es
lo que se reparte. Con ocho archivos que esperan cada uno, medido: **3.4× más rápido con `--parallel 4`, 5.6× con
`--parallel 8`**.

La unidad es el **archivo**: los casos de prueba dentro de un archivo, y los pasos dentro de un caso, se siguen
ejecutando uno tras otro, porque un paso transmite sus valores al siguiente. Por tanto, ejecutar un único archivo con
`--parallel 4` no cambia nada.

**La ganancia depende de si sus archivos realmente esperan.** Unos 0.35 s de cada ejecución son
fijos (arranque, análisis, escritura de los informes) y ningún número de workers los elimina. En las suites
de `tests/` del propio venom, que son cortas y casi no esperan, `--parallel 4` solo es aproximadamente 1.2× más rápido.
Espere una ganancia real cuando sus archivos pasen tiempo esperando a un servicio, y poca cuando no lo hagan.

Hay dos cosas que conviene saber:

- **Cada línea de la consola lleva la etiqueta del archivo del que procede**, de modo que la salida de archivos que se
  ejecutan a la vez sigue siendo legible:

  ```
  [exec.yml]   • exec testsuite (tests/exec.yml)
  [vars.yml]   • vars testsuite (tests/vars.yml)
  [exec.yml]   	• stdin PASS
  [vars.yml]   	• set PASS
  ```

  Las líneas de distintos archivos sí se entremezclan (no se agrupan), pero cada una indica quién la escribió,
  y una línea nunca se divide entre dos archivos. Para seguir un solo archivo use `grep`:
  `venom run --parallel 4 | grep '^\[exec.yml'`. Con `--parallel 1` no hay ninguna etiqueta y la
  salida es exactamente la de siempre. El archivo de registro no se ve afectado en ningún caso: cada línea
  ya lleva la etiqueta de su archivo de prueba.
- **`--stop-on-failure` detiene toda la ejecución.** Cuando falla un archivo de prueba, no se inicia ningún otro archivo;
  los archivos que ya se están ejecutando terminan, por lo que todo archivo iniciado informa de un resultado.
  Los archivos que nunca se iniciaron se notifican como omitidos. En versiones anteriores, este indicador solo omitía los
  casos de prueba restantes del archivo que había fallado, y la ejecución continuaba con el siguiente archivo.

`-vvv` no puede combinarse con `--parallel 2` o más: el perfilado de CPU abarca todo el proceso, por lo que no puede
producir un perfil por cada archivo que se ejecuta a la vez. La ejecución se detiene al arrancar e informa de ello.

## Compatibilidad con globstar {#globstar-support}

La CLI `venom` admite globstar:

```
$ venom run ./foo/b*/**/z*.yml
```

## Variables {#variables}

Para especificar variables individuales en la línea de comandos, use la opción `--var` al ejecutar los comandos `venom run`:

```bash
$ venom run --var="foo=bar"
$ venom run --var='foo_list=["biz","buz"]'
$ venom run --var='foo={"biz":"bar","biz":"barr"}'
```

La opción `--var` puede usarse varias veces en un mismo comando.

### Archivos de definición de variables {#variable-definitions-files}

Para definir muchas variables, resulta más cómodo indicar sus valores en un archivo de definición de variables. Este archivo es un diccionario YAML. Debe indicar ese archivo en la línea de comandos con `--var-from-file`:

```bash
venom run --var-from-file variables.yaml
```

### Variables de entorno {#environment-variables}

Como alternativa a las demás formas de definir variables, la herramienta `venom` busca en el entorno de su propio proceso variables de entorno llamadas `VENOM_VAR_` seguidas del nombre de una variable declarada.

```bash
$ export VENOM_VAR_foo=bar
$ venom run *.yml
```

También puede definir la variable de entorno y ejecutar su suite de pruebas en una sola línea:

```bash
$ VENOM_VAR_foo=bar venom run *.yml
```

## Argumentos {#arguments}

Puede definir argumentos en la línea de comandos usando el nombre del indicador.

Los indicadores figuran en el resultado del comando de ayuda.

Lista de indicadores disponibles para el comando `venom run`:

```
Flags:
      --format string           --format:json, tap, xml, yaml (default "xml")
  -h, --help                    help for run
      --html-report             Generate HTML Report
      --lib-dir string          Lib Directory: can contain user executors. example:/etc/venom/lib:$HOME/venom.d/lib
      --output-dir string       Output Directory: create tests results file inside this directory
      --parallel int            Number of test files to run at the same time. 1 (default) runs them one after another (default 1)
      --stop-on-failure         Stop running Test Suite on first Test Case failure
      --strict-variables        Fail a step that names an unknown variable
      --var stringArray         --var cds='cds -f config.json' --var cds2='cds -f config.json'
      --var-from-file strings   --var-from-file filename.yaml --var-from-file filename2.yaml: yaml, must contains a dictionary
  -v, --verbose count           verbose. -vv to very verbose and -vvv to very verbose with CPU Profiling
```

### Definir argumentos con variables de entorno {#define-arguments-with-environment-variables}

También puede definir los argumentos con variables de entorno:

```bash
# is the same as
VENOM_FORMAT=json venom run my-test-suite.yml

# is equivalent to
venom run my-test-suite.yml --format=json
```

Indicadores y su equivalente mediante variables de entorno:

- El indicador `--format="json"` equivale a la variable de entorno `VENOM_FORMAT="json"`
- El indicador `--lib-dir="/etc/venom/lib:$HOME/venom.d/lib"` equivale a la variable de entorno `VENOM_LIB_DIR="/etc/venom/lib"`
- El indicador `--output-dir="test-results"` equivale a la variable de entorno `VENOM_OUTPUT_DIR="test-results"`
- El indicador `--stop-on-failure` equivale a la variable de entorno `VENOM_STOP_ON_FAILURE=true`
- El indicador `--strict-variables` equivale a la variable de entorno `VENOM_STRICT_VARIABLES=true`
- El indicador `--var foo=bar` equivale a la variable de entorno `VENOM_VAR_foo='bar'`
- El indicador `--var-from-file fileA.yml fileB.yml` equivale a la variable de entorno `VENOM_VAR_FROM_FILE="fileA.yml fileB.yml"`
- El indicador `--parallel 4` equivale a la variable de entorno `VENOM_PARALLEL=4`
- El indicador `-v` equivale a la variable de entorno `VENOM_VERBOSE=1`
- El indicador `-vv` equivale a la variable de entorno `VENOM_VERBOSE=2`

Se puede definir la variable de entorno `NO_COLOR=1` para desactivar los colores de la salida.

## Usar un archivo de configuración {#use-a-configuration-file}

Puede definir la configuración de Venom mediante un archivo de configuración `.venomrc`. Este archivo debe colocarse en el directorio actual o en el directorio `home`.

```yml
variables: 
  - foo=bar
variables_files:
  - my_var_file.yaml
stop_on_failure: true
strict_variables: true
format: xml
output_dir: output
lib_dir: lib
verbosity: 3
parallel: 4
```

Tenga en cuenta que los indicadores de la línea de comandos prevalecen sobre el archivo de configuración. El archivo de configuración prevalece sobre las variables de entorno.

## Modelo de confianza {#trust-model}

Venom no tiene entorno aislado (sandbox). Una suite de pruebas es código y se ejecuta con los permisos del usuario que ejecuta venom.

- Las suites de pruebas, los executors de usuario en `lib/` y los plugins de Go (`lib/*.so`) se ejecutan todos con sus permisos. Ejecute solo una suite que ejecutaría como un script.
- No incluya nunca el resultado de un paso, ni ningún valor que no controle, en un `script:` ni en un texto SQL sin entrecomillarlo. El resultado de un paso puede contener cualquier cosa, y venom no lo escapa para un shell ni para SQL.
- En CI, no ejecute suites procedentes de forks o pull requests sin revisar, y no ponga secretos a disposición de esas ejecuciones.
- Un secreto declarado en `secrets` o en `.venomrc` se oculta en los informes, los registros y la consola. Solo se ocultan los valores declarados.
- Los executors `ssh`, `imap` y `smtp` no comprueban la identidad del servidor de forma predeterminada. Venom registra una advertencia una vez por ejecución cuando es así. Para activar la comprobación, defina `knownhosts` o `hostkey` en un paso `ssh`, y `verifytls: true` (bajo `auth:` para `imap`) junto con `withtls: true` en un paso `imap` o `smtp`.
- El executor `web` ignora de forma predeterminada los errores de certificado HTTPS. Defina la variable `verifyTls` como `true` para que el navegador rechace un certificado no válido. Consulte [executors/web](executors/web/README.md).

