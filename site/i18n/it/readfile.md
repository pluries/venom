---
translates: readfile
title: Read file
description: Come usare l'executor Read file in una test suite di venom.
fingerprint: 8370675caf1915a0b8257d34f10772113674ba996cf21e009ccff6d505434aaa
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# Venom - Executor Read file {#venom---executor-read-file}

Step usato per leggere un file

Caso d'uso: la tua applicazione scrive un file su disco. Venom può verificare che il file venga prodotto, leggerlo
e restituirne il contenuto. Il contenuto può essere usato dagli altri step della test suite.

Il path può contenere un percorso completo, un carattere jolly o una directory:

```
- path: /a/full/path/file.txt
- path: afile.txt
- path: *.yml
- path: a_directory/
- path: ./foo/b*/**/z*.txt
```

## Input {#input}

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

## Output {#output}

```yaml
  result.timeseconds
  result.content
  result.contentjson
  result.size.filename
  result.md5sum.filename
  result.modtime.filename
  result.mod.filename
```

- result.timeseconds: durata dell'esecuzione
- result.err: se il file non esiste, questo campo contiene un errore
- result.content: contenuto del file letto
- result.contentjson: contenuto del file letto, se è un file JSON. Puoi accedere ai dati JSON come result.contentjson.yourkey, per esempio
- result.size.filename: dimensione del file 'filename'
- result.md5sum.filename: md5 del file 'filename'
- result.modtime.filename: data di modifica del file 'filename', esempio: 1487698253
- result.mod.filename: permessi sul file 'filename', esempio: -rw-r--r--

Nota: il valore di 'filename' è uguale al path, con tutte le lettere in minuscolo e con tutti i '/' sostituiti da '_'.

## Assertion predefinita {#default-assertion}

```yaml
result.err ShouldBeEmpty
```

## Esempio {#example}

File testa.txt:

```
simple content
multilines
```

File testa.json:

```json
{
  "foo": "bar"
}
```

File testb.json:

```json
[
  {
    "foo": "bar",
    "foo2": "bar2"
  }
]

```

File di test di venom:

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
