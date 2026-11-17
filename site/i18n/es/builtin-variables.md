---
translates: builtin-variables
title: Variables integradas de venom
description: Las variables que venom define por usted.
fingerprint: 57868302669e41f5bae786f964d1e4ad3735ca9dcc92970c85f06a769329fbae
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# Variables integradas de venom {#builtin-venom-variables}

```yaml
name: MyTestSuite
testcases:
- name: testA
  steps:
  - type: exec
    script: echo '{{.venom.testsuite}} {{.venom.testsuite.filename}} {{.venom.testcase}} {{.venom.teststep.number}} {{.venom.datetime}} {{.venom.timestamp}} {{.venom.testcase.totalSteps}} {{.venom.testsuite.totalSteps}}'
    # will display something as: MyTestSuite MyTestSuiteWithVenomBuiltinVar.yml testA 0 2018-08-05T21:38:24+02:00 1533497904 3 5

```

Variables integradas:

* {{.venom.datetime}}
* {{.venom.executable}}
* {{.venom.libdir}}
* {{.venom.outputdir}}
* {{.venom.testcase}}
* {{.venom.testcase.totalSteps}}
* {{.venom.teststep.number}}
* {{.venom.testsuite.name}}
* {{.venom.testsuite.filename}}
* {{.venom.testsuite.filepath}}
* {{.venom.testsuite.shortName}}
* {{.venom.testsuite.workdir}}
* {{.venom.testsuite}}
* {{.venom.testsuite.totalSteps}}
* {{.venom.timestamp}}


### Variables secretas {#secrets-variables}

Ejemplo:

```yml
name: Your Testsuite
vars:
  foo : this-value-is-secret
secrets:
  - foo
testcases:
- name: myvar_first
  steps:
  - type: exec
    script: "echo myvar {{.foo}}"
```

El valor `this-value-is-secret` no se imprimirá en su consola, en `venom.log` ni en los archivos `...dump.json`.

#### Qué se oculta y qué no {#what-is-hidden-and-what-is-not}

Venom oculta un secreto por su valor, dondequiera que la ejecución lo imprima o lo escriba: consola, `venom.log`, los informes y los archivos `...dump.json`, en la salida de los pasos, los nombres, los textos de las aserciones (assertions) y los mensajes de error.

Se oculta:

- todo valor declarado en `secrets:` (de la suite) o en `secrets` de `.venomrc`;
- la forma en base64 de ese valor y, para `basic_auth_password`, el base64 de `user:password`;
- la forma slug que venom le da dentro del nombre de un caso de prueba (solo cuando tiene al menos 4 caracteres);
- las formas codificadas como URL: de consulta (`a+b%21`) y de ruta (`a%20b%21`), cuando difieren del valor y tienen al menos 4 caracteres.

No se oculta:

- un valor que no declaró, aunque provenga de una fuente secreta;
- cualquier otra transformación: hexadecimal, entidades HTML, un hash, una firma, un valor cortado en partes o unido con otro texto, una forma con escapes JSON.

Cuando un paso pueda imprimir un secreto en otra forma, evite imprimirlo o declare también en `secrets:` el valor transformado.
