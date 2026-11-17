---
translates: faq
title: Preguntas frecuentes
description: Respuestas a preguntas habituales.
fingerprint: dd2ea1a96488bc21ef4960ea695d9dad539eeceac367b25b58543ff4d256d0f4
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# Preguntas frecuentes {#faq}

## Errores habituales con las comillas {#common-errors-with-quotes}

Si obtiene un error de este tipo:

```
err:unable to parse file "foo.yaml": error converting YAML to JSON: yaml: line 8: did not find expected key
```

probablemente se debe a que intenta usar un valor JSON en lugar de una cadena. Debería encontrar más detalles en el archivo `venom.log`.

Incorrecto:

```yml
...
vars:
  body: >-
      {
        "the-attribute": "the-value"
      }
...
steps:
- type: http
  body: "{{.body}}"
...
```

Correcto:


```yml
...
vars:
  body: >-
      {
        "the-attribute": "the-value"
      }
...
steps:
- type: http
  body: '{{.body}}'
...
```

Fíjese en las comillas simples en el valor de `body`.
