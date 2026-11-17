---
translates: faq
title: FAQ
description: Réponses aux questions courantes.
fingerprint: dd2ea1a96488bc21ef4960ea695d9dad539eeceac367b25b58543ff4d256d0f4
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# FAQ {#faq}

## Erreurs courantes liées aux guillemets {#common-errors-with-quotes}

Si vous avez ce genre d'erreur :

```
err:unable to parse file "foo.yaml": error converting YAML to JSON: yaml: line 8: did not find expected key
```

c'est probablement parce que vous essayez d'utiliser une valeur JSON à la place d'une chaîne de caractères. Vous trouverez plus de détails dans le fichier `venom.log`.

Incorrect :

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

Correct :


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

Notez l'apostrophe (guillemet simple) autour de la valeur de `body`.
