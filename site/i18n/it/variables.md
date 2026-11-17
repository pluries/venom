---
translates: variables
title: Variabili
description: Come definire e usare le variabili in una suite.
fingerprint: 571c5e16517c3ed7ec39bc40546cba3ab0da6e9c0ebd3a0e1e59d6875364590b
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# Variabili {#variables}

### Variabili della testsuite {#testsuite-variables}

Puoi definire variabili a livello di `testsuite`.

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

Ogni variabile utente usata nella testsuite deve essere dichiarata in questa sezione. Puoi sovrascriverne il valore a runtime in diversi modi:
- singolarmente, con l'opzione da riga di comando `--var`;
- in file di definizione delle variabili, indicati sulla riga di comando con `--var-from-file`;
- come variabili d'ambiente.



### Far fallire uno step su una variabile sconosciuta {#fail-a-step-on-an-unknown-variable}

Per impostazione predefinita, venom lascia scritto nello step un `{{.name}}` che non conosce, quindi una variabile scritta male come
`{{.nmae}}` viene inviata come testo e il test può passare. Con `--strict-variables`, uno step che nomina una variabile sconosciuta
fallisce prima di essere eseguito, e il messaggio indica lo step e la variabile:

```
step #2: unknown variable(s) "nmae"
```

L'impostazione è disattivata per impostazione predefinita e vale per l'intera esecuzione. La si attiva con `--strict-variables`,
`VENOM_STRICT_VARIABLES=true` oppure `strict_variables: true` in `.venomrc`.

Queste non vengono mai segnalate: le variabili impostate da uno step precedente o da `vars`, le variabili di intervallo (`key`, `value`, `index`),
le variabili `venom.*`, le espressioni che usano `default` o `ternary` e `{{.result...}}` (venom le risolve dopo l'esecuzione
dello step, in `info`, nelle assertion e nelle variabili estratte). I nomi di helper sconosciuti non vengono controllati.

Un limite: venom legge prima l'intero file e valuta a livello di step ogni espressione composta solo da lettere, cifre, `.`, `-`, `_`,
`"`, `|` e spazi. Un'espressione con qualsiasi altro carattere, come `{{ printf "%s" .a }}` oppure
`{{.url | trimSuffix "/"}}`, viene valutata quando il file viene letto, prima dell'esecuzione degli step, quindi la modalità strict non la vede
e una variabile scritta male al suo interno produce comunque un valore vuoto o errato, senza alcun errore.

Uno step che ha bisogno che il testo `{{.something}}` arrivi invariato a un altro strumento, come `docker ... --format '{{.ID}}'`,
fallisce in modalità strict. Venom non ha un modo per fare l'escape delle doppie graffe letterali, quindi lascia l'impostazione disattivata per una suite di questo tipo.

### Helper per le variabili {#variable-helpers}

Helper disponibili e alcuni esempi:

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
- `escape`: sostituisce ‘_‘, ‘/’, ‘.’ con ‘-’

Altri esempi sono disponibili [qui](https://github.com/ovh/venom/tree/master/variable_helpers.md)
