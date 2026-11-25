---
translates: quickstart
title: Quickstart
description: Instale venom, escriba una suite y ejecútela.
fingerprint: ff09c7857b2e31620d2ae8d74003a0e186283bb52cf0d3334055d5d020980dc4
drafted_by: claude
drafted_on: 2026-11-24
status: reviewed
reviewed_by: Christian Stephan
reviewed_on: 2026-11-25
reviewed_fingerprint: ff09c7857b2e31620d2ae8d74003a0e186283bb52cf0d3334055d5d020980dc4
---
Esta página le lleva de cero a una suite de pruebas que pasa. Necesita un terminal en Linux, macOS o Windows.

## Instalación {#install}

Descargue el ejecutable de la última versión para su plataforma. Este es el ejemplo para Linux. La página [instalación](installing.html) lista todas las plataformas, y la página [verificar una versión](verify-release.html) muestra cómo comprobar la firma:

```
curl -L -o venom https://github.com/pluries/venom/releases/latest/download/venom.linux-amd64
chmod +x venom
```

Los comandos escriben un ejecutable `venom` en la carpeta actual.

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
