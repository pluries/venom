---
translates: readfile
title: Read file
description: Cómo usar el executor Read file en una suite de pruebas de venom.
fingerprint: 8370675caf1915a0b8257d34f10772113674ba996cf21e009ccff6d505434aaa
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# Venom - Executor Read file {#venom---executor-read-file}

Paso para leer un archivo

Caso de uso: su aplicación escribe un archivo en disco. Venom puede comprobar que ese archivo se ha generado, leerlo
y devolver su contenido. Otros pasos de la suite de pruebas pueden usar ese contenido.

La ruta puede ser una ruta completa, un comodín o un directorio:

```
- path: /a/full/path/file.txt
- path: afile.txt
- path: *.yml
- path: a_directory/
- path: ./foo/b*/**/z*.txt
```

## Entrada {#input}

```yaml
name: TestSuite Read File
testcases:
- name: TestCase Read File
  steps:
  - type: readfile
    path: yourfile.txt
    assertions:
    - result.err ShouldBeEmpty
```
```yaml
name: TestSuite Read Yaml File
testcases:
  - name: Read File
    steps:
      - type: readfile
        path: yourfile.yml
        assertions:
          - result.err ShouldBeEmpty
```

## Salida {#output}

```yaml
  result.timeseconds
  result.content
  result.contentjson
  result.size.filename
  result.md5sum.filename
  result.modtime.filename
  result.mod.filename
```

- result.timeseconds: duración de la ejecución
- result.err: si el archivo no existe, este campo contiene un error
- result.content: contenido del archivo leído
- result.contentjson: contenido del archivo leído si es un archivo JSON. Puede acceder a los datos JSON como result.contentjson.yourkey, por ejemplo
- result.size.filename: tamaño del archivo 'filename'
- result.md5sum.filename: md5 del archivo 'filename'
- result.modtime.filename: fecha de modificación del archivo 'filename', por ejemplo: 1487698253
- result.mod.filename: permisos del archivo 'filename', por ejemplo: -rw-r--r--

Nota: el valor de 'filename' es igual a la ruta, con todas las letras en minúsculas y con todos los '/' sustituidos por '_'.

## Aserción por defecto {#default-assertion}

```yaml
result.err ShouldBeEmpty
```

## Ejemplo {#example}

Archivo testa.txt:

```
simple content
multilines
```

Archivo testa.json:

```json
{
  "foo": "bar"
}
```

Archivo testb.json:

```json
[
  {
    "foo": "bar",
    "foo2": "bar2"
  }
]

```

Archivo de pruebas de venom:

```yaml
name: TestSuite Read File
testcases:
- name: TestCase Read File
  steps:
  - type: readfile
    path: testa.json
    assertions:
      - result.contentjson.foo ShouldEqual bar

  - type: readfile
    path: testb.json
    assertions:
      - result.contentjson.contentjson0.foo2 ShouldEqual bar2

  - type: readfile
    path: testa.txt
    assertions:
      - result.content ShouldContainSubstring multilines
```
