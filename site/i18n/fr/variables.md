---
translates: variables
title: Variables
description: Comment définir et utiliser des variables dans une suite.
fingerprint: 571c5e16517c3ed7ec39bc40546cba3ab0da6e9c0ebd3a0e1e59d6875364590b
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# Variables {#variables}

### Variables de testsuite {#testsuite-variables}

Vous pouvez définir des variables au niveau de la `testsuite`.

```yaml
name: myTestSuite
vars:
  foo: foo
  biz:
    bar: bar
  aString: '{"foo": "bar"}'

testcases:
- name: first-test-case
  steps:
  - type: exec
    script: echo '{{.foo}} {{.biz.bar}}'
    assertions:
    - result.code ShouldEqual 0
    - result.systemout ShouldEqual "foo bar"

- name: foobar
  steps:
  - script: echo '{{.aString}}'
    info: value of aString is {{.aString}}
    assertions:
    - result.systemoutjson.foo ShouldEqual bar
...
```

Chaque variable utilisateur employée dans la testsuite doit être déclarée dans cette section. Vous pouvez surcharger sa valeur à l'exécution de plusieurs façons :
- Individuellement, avec l'option de ligne de commande `--var`.
- Dans des fichiers de définition de variables, indiqués en ligne de commande avec `--var-from-file`.
- Avec des variables d'environnement.



### Faire échouer une étape sur une variable inconnue {#fail-a-step-on-an-unknown-variable}

Par défaut, venom laisse tel quel dans l'étape un `{{.name}}` qu'il ne connaît pas : une variable mal orthographiée comme
`{{.nmae}}` est donc envoyée comme du texte, et le test peut passer. Avec `--strict-variables`, une étape qui nomme une variable inconnue
échoue avant de s'exécuter, et le message nomme l'étape et la variable :

```
step #2: unknown variable(s) "nmae"
```

Ce réglage est désactivé par défaut et s'applique à toute l'exécution. Activez-le avec `--strict-variables`,
`VENOM_STRICT_VARIABLES=true`, ou `strict_variables: true` dans `.venomrc`.

Ne sont jamais signalées : les variables définies par une étape précédente ou par `vars`, les variables de boucle (`key`, `value`, `index`),
les variables `venom.*`, les expressions qui utilisent `default` ou `ternary`, et `{{.result...}}` (venom les résout après l'exécution
de l'étape, dans `info`, dans les assertions et dans les variables extraites). Les noms d'helpers inconnus ne sont pas vérifiés.

Une limite : venom lit d'abord tout le fichier et évalue, au niveau de l'étape, chaque expression composée uniquement de lettres, de chiffres, de `.`, `-`, `_`,
`"`, `|` et d'espaces. Une expression qui contient un autre caractère, comme `{{ printf "%s" .a }}` ou
`{{.url | trimSuffix "/"}}`, est évaluée à la lecture du fichier, avant l'exécution des étapes ; le mode strict ne la voit donc pas,
et une variable mal orthographiée dans une telle expression donne toujours une valeur vide ou fausse, sans erreur.

Une étape qui doit transmettre le texte `{{.something}}` tel quel à un autre outil, comme `docker ... --format '{{.ID}}'`,
échoue en mode strict. Venom n'a aucun échappement pour les doubles accolades littérales : laissez donc ce réglage désactivé pour une telle suite.

### Helpers de variables {#variable-helpers}

Helpers disponibles et quelques exemples :

- `abbrev`
- `abbrevboth`
- `trunc`
- `trim`
- `upper`: {{.myvar | upper}}
- `lower`: {{.myvar | lower}}
- `title`
- `untitle`
- `substr`
- `repeat`
- `trimall`
- `trimAll`
- `trimSuffix`
- `trimPrefix`
- `nospace`
- `initials`
- `randAlphaNum`
- `randAlpha`
- `randASCII`
- `randNumeric`
- `swapcase`
- `shuffle`
- `snakecase`
- `camelcase`
- `quote`
- `squote`
- `indent`
- `nindent`
- `replace`: {{.myvar | replace "_" "."}}
- `plural`
- `default`: {{.myvar | default ""}}
- `empty`
- `coalesce`
- `toJSON`
- `toPrettyJSON`
- `b64enc`
- `b64dec` {{.result.bodyjson | b64enc}}
- `escape`: remplace ‘_‘, ‘/’, ‘.’ par ‘-’

D'autres exemples sont disponibles [ici](https://github.com/ovh/venom/tree/master/variable_helpers.md)
