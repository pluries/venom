---
translates: readfile
title: Read file
description: Comment utiliser l'executor Read file dans une testsuite venom.
fingerprint: 8370675caf1915a0b8257d34f10772113674ba996cf21e009ccff6d505434aaa
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# Venom - Executor Read file {#venom---executor-read-file}

Étape qui sert à lire un fichier

Cas d'usage : votre application écrit un fichier sur disque. Venom peut vérifier que ce fichier est produit, le lire,
et renvoyer son contenu. Ce contenu peut être utilisé par d'autres étapes de la testsuite.

Le chemin peut être un chemin complet, un joker ou un répertoire :

```
- path: /a/full/path/file.txt
- path: afile.txt
- path: *.yml
- path: a_directory/
- path: ./foo/b*/**/z*.txt
```

## Entrée {#input}

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

## Sortie {#output}

```yaml
  result.timeseconds
  result.content
  result.contentjson
  result.size.filename
  result.md5sum.filename
  result.modtime.filename
  result.mod.filename
```

- result.timeseconds : durée d'exécution
- result.err : si le fichier n'existe pas, ce champ contient une erreur
- result.content : contenu du fichier lu
- result.contentjson : contenu du fichier lu s'il s'agit d'un fichier JSON. Vous pouvez accéder aux données JSON avec result.contentjson.yourkey, par exemple
- result.size.filename : taille du fichier 'filename'
- result.md5sum.filename : md5 du fichier 'filename'
- result.modtime.filename : date de modification du fichier 'filename', exemple : 1487698253
- result.mod.filename : droits sur le fichier 'filename', exemple : -rw-r--r--

Remarque : la valeur de 'filename' est égale au chemin, avec toutes les lettres en minuscules et tous les '/' remplacés par '_'.

## Assertion par défaut {#default-assertion}

```yaml
result.err ShouldBeEmpty
```

## Exemple {#example}

fichier testa.txt :

```
simple content
multilines
```

fichier testa.json :

```json
{
  "foo": "bar"
}
```

fichier testb.json :

```json
[
  {
    "foo": "bar",
    "foo2": "bar2"
  }
]

```

fichier de test venom :

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
