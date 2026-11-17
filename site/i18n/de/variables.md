---
translates: variables
title: Variablen
description: Wie Variablen in einer Suite definiert und verwendet werden.
fingerprint: 571c5e16517c3ed7ec39bc40546cba3ab0da6e9c0ebd3a0e1e59d6875364590b
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# Variablen {#variables}

### Variablen der Testsuite {#testsuite-variables}

Sie können Variablen auf der Ebene der `testsuite` definieren.

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

Jede in der Testsuite verwendete Benutzervariable muss in diesem Abschnitt deklariert werden. Ihren Wert können Sie zur Laufzeit auf mehrere Arten überschreiben:
- einzeln, mit der Kommandozeilenoption `--var`;
- in Dateien mit Variablendefinitionen, die auf der Kommandozeile mit `--var-from-file` angegeben werden;
- als Umgebungsvariablen.



### Einen Schritt bei unbekannter Variable fehlschlagen lassen {#fail-a-step-on-an-unknown-variable}

Standardmäßig lässt venom ein `{{.name}}`, das es nicht kennt, so im Schritt stehen, wie es geschrieben wurde. Eine falsch geschriebene Variable wie
`{{.nmae}}` wird deshalb als Text gesendet, und der Test kann trotzdem bestehen. Mit `--strict-variables` schlägt ein Schritt, der eine unbekannte Variable nennt,
fehl, bevor er läuft, und die Meldung nennt den Schritt und die Variable:

```
step #2: unknown variable(s) "nmae"
```

Die Einstellung ist standardmäßig aus und gilt für den gesamten Lauf. Sie setzen sie mit `--strict-variables`,
`VENOM_STRICT_VARIABLES=true` oder `strict_variables: true` in `.venomrc`.

Nie gemeldet werden: Variablen, die ein früherer Schritt oder `vars` setzt, Range-Variablen (`key`, `value`, `index`),
`venom.*`-Variablen, Ausdrücke mit `default` oder `ternary` sowie `{{.result...}}` (venom löst diese erst nach
dem Schritt auf, in `info`, in Assertions und in extrahierten Variablen). Unbekannte Helper-Namen werden nicht geprüft.

Eine Einschränkung: venom liest zuerst die gesamte Datei und wertet auf der Ebene der Schritte jeden Ausdruck aus, der nur aus Buchstaben, Ziffern, `.`, `-`, `_`,
`"`, `|` und Leerzeichen besteht. Ein Ausdruck mit einem anderen Zeichen, etwa `{{ printf "%s" .a }}` oder
`{{.url | trimSuffix "/"}}`, wird schon beim Lesen der Datei ausgewertet, bevor die Schritte laufen. Der strikte Modus sieht ihn daher nicht,
und eine falsch geschriebene Variable darin liefert weiterhin ohne Fehlermeldung einen leeren oder falschen Wert.

Ein Schritt, bei dem der Text `{{.something}}` unverändert ein anderes Werkzeug erreichen muss, etwa `docker ... --format '{{.ID}}'`,
schlägt im strikten Modus fehl. Venom kennt kein Escaping für literale doppelte geschweifte Klammern; lassen Sie die Einstellung für eine solche Suite daher aus.

### Variablen-Helper {#variable-helpers}

Verfügbare Helper und einige Beispiele:

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
- `escape`: ersetzt ‘_‘, ‘/’, ‘.’ durch ‘-’

Weitere Beispiele finden Sie [hier](https://github.com/ovh/venom/tree/master/variable_helpers.md)
