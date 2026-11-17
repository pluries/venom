---
translates: quickstart
title: Quickstart
description: Instale venom, escriba una suite y ejecútela.
fingerprint: 86cf4090a52b3ea8d391e1a37a1038fbbf6b86519dd9a23afd4e7391ceb9b402
drafted_by: claude
drafted_on: 2026-11-16
status: reviewed
reviewed_by: Christian Stephan
reviewed_on: 2026-11-16
reviewed_fingerprint: 86cf4090a52b3ea8d391e1a37a1038fbbf6b86519dd9a23afd4e7391ceb9b402
---
Esta página le lleva de cero a una suite de pruebas que pasa. Necesita Go y un clon del repositorio de venom.

## Instalación {#install}

Compile la herramienta de línea de comandos desde un clon del repositorio:

```
go build -o venom ./cmd/venom
```

El comando escribe un binario `venom` en la carpeta actual. Cuando exista una versión publicada, puede descargar su binario en lugar de compilarlo.

## Escribir una suite {#write-a-suite}

Una suite es un archivo YAML. Esta tiene un caso de prueba con un paso. El paso ejecuta un comando de shell y dos aserciones comprueban su resultado:

````yaml
name: Quickstart suite
testcases:
  - name: say hello
    steps:
      - type: exec
        script: echo "hello venom"
        assertions:
          - result.systemout ShouldContainSubstring hello
          - result.code ShouldEqual 0
````

- `type: exec` ejecuta el `script` en un shell.
- `result.systemout ShouldContainSubstring hello` comprueba que la salida contiene la palabra `hello`.
- `result.code ShouldEqual 0` comprueba que el comando terminó sin error.

## Ejecutarla {#run-it}

Guarde el archivo como `quickstart.yml` y ejecútelo:

```
./venom run quickstart.yml
```

## Leer el resultado {#read-the-result}

Venom imprime cada caso de prueba y su estado. Cuando todas las aserciones se cumplen, la última línea es:

```
final status: PASS
```

Una aserción fallida se imprime con el paso, la aserción y el valor que venom encontró. El código de salida es distinto de cero cuando una suite falla.

Venom es un fork independiente de [ovh/venom](https://github.com/ovh/venom). A continuación, lea la [visión general](overview.html) o la página sobre las [suites de pruebas](testsuites.html).
