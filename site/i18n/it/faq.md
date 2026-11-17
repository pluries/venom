---
translates: faq
title: FAQ
description: Risposte alle domande più comuni.
fingerprint: dd2ea1a96488bc21ef4960ea695d9dad539eeceac367b25b58543ff4d256d0f4
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# FAQ {#faq}

## Errori comuni con le virgolette {#common-errors-with-quotes}

Se ottieni un errore di questo tipo:

```
err:unable to parse file "foo.yaml": error converting YAML to JSON: yaml: line 8: did not find expected key
```

probabilmente è perché stai usando un valore JSON al posto di una stringa. Nel file `venom.log` dovresti trovare maggiori dettagli.

Sbagliato:

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

Corretto:


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

Nota l'apice singolo attorno al valore di `body`.
