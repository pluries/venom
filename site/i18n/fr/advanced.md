---
translates: advanced
title: Utilisation avancée
description: Options pour les suites plus volumineuses.
fingerprint: a7992da938eb488d1fc5fa9a37e1159081f4a0bf684768a8c2d34b4bb13924ff
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# Utilisation avancée {#advanced-usage}

## Déboguer vos suites de tests {#debug-your-testsuites}

Un fichier *venom.log* est généré à chaque commande `venom run`.

Il y a deux façons de déboguer une suite de tests :
 - utiliser l'option `-v` du binaire venom.
   - `$ venom run -v test.yml` affichera le détail de chaque étape
   - `$ venom run -vv test.yml` générera des fichiers *dump.json* pour chaque étape de test.
   - `$ venom run -vvv test.yml` générera des fichiers *pprof*, un profil CPU et un profil mémoire par
     fichier de test, nommés `pprof_cpu_profile_<file>.prof` et `pprof_mem_profile_<file>.prof` dans le
     répertoire de sortie. Ouvrez-les avec `go tool pprof`. Le profilage se fait par fichier pour que vous voyiez quelle
     suite est lente, ce qui explique que `-vvv` ne puisse pas être combiné avec `--parallel 2` ou plus : le profileur
     CPU de Go porte sur tout le processus.
 - utiliser le mot-clé `info` dans votre étape de test :
fichier `test.yml` :
```yml
name: Exec testsuite
testcases:
- name: testA
  steps:
  - type: exec
    script: echo 'foo with a bar here'
    info:
      - this a first info
      - and a second...
- name: cat json
  steps:
  - script: cat exec/testa.json
    info: "the value of result.systemoutjson is {{.result.systemoutjson}}"
    assertions:
    - result.systemoutjson.foo ShouldContainSubstring bar
```

```bash
$ venom run test.yml

# output:

 • Exec testsuite (exec.yml)
  • testA SUCCESS
    [info] this a first info (exec.yml:8)
    [info] and a second... (exec.yml:9)
  • testB SUCCESS
  • sleep 1 SUCCESS
  • cat json SUCCESS
    [info] the value of result.systemoutjson is map[foo:bar] (exec.yml:34)
```

## Ignorer des cas de test et des étapes {#skip-testcase-and-teststeps}

Il est possible d'ignorer un `testcase` selon certaines `assertions`. Par exemple, l'exemple suivant ignorera le dernier cas de test.

```yaml
name: "Skip testsuite"
vars:
  foo: bar

testcases:
- name: init
  steps:
  - type: exec
    script: echo {{.foo}}
    assertions:
    - result.code ShouldEqual 0
    - result.systemout ShouldContainSubstring bar

- name: do-not-skip-this
  skip: 
  - foo ShouldNotBeEmpty
  steps:
  - type: exec
    script: exit 0

- name: skip-this
  skip: 
    - foo ShouldBeEmpty
  steps:
  - type: exec
    script: command_not_found
    assertions:
    - result.code ShouldEqual 0

```

Une instruction `skip` peut aussi être placée au niveau des étapes pour n'exécuter qu'une partie d'un cas de test. Si une condition du bloc skip n'est pas vraie, l'étape est ignorée.

Si toutes les étapes d'un cas de test sont ignorées, le cas de test lui-même est aussi traité comme « ignoré » plutôt que comme « réussi » ou « en échec ».

```yaml
name: "Skip testsuite"
vars:
  foo: bar

testcases:
- name: skip-one-of-these
  steps:
  - name: do-not-skip-this
    type: exec
    script: exit 0
    assertions:
    - result.code ShouldEqual 0
    skip:
    - foo ShouldNotBeEmpty
  - name: skip-this
    type: exec
    script: exit 1
    assertions:
    - result.code ShouldEqual 0
    skip:
    - foo ShouldBeEmpty

```

## Itérer sur des données {#iterating-over-data}

Il est possible d'itérer sur des données avec l'attribut `range`.

Les types de données suivants sont pris en charge, chacun exposant les variables contextuelles `.index`, `.key` et `.value` :

- Un tableau dont chaque valeur est parcourue (`[]interface{}`)
  - `.index`/`.key` : indice de l'itération courante
  - `.value` : valeur de l'élément de l'itération courante
- Une map dont chaque clé est parcourue (`map[string]interface{}`)
  - `.index` : indice de l'itération courante
  - `.key` : clé de l'élément de l'itération courante
  - `.value` : valeur de l'élément de l'itération courante
- Un entier pour exécuter l'étape ciblée `n` fois (`int`)
  - `.index`/`.key`/`.value` : indice de l'itération courante
- Une chaîne templatisée dont le résultat est de l'un des types ci-dessus (`string`)
  - Elle peut être héritée d'un fichier de variables, ou interpolée à partir du résultat d'une étape précédente

Par exemple, l'exemple suivant itère sur un tableau de deux éléments contenant des maps :
```yaml
- name: range with hardcoded array
  steps:
  - type: exec
    range:
      - actual: hello
        expected: hello
      - actual: world
        expected: world
    script: echo "{{.value.actual}}"
    assertions:
    - result.code ShouldEqual 0
    - result.systemout ShouldEqual "{{.value.expected}}"
```

D'autres exemples sont disponibles dans [`tests/ranged.yml`](/tests/ranged.yml).

