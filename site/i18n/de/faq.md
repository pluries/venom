---
translates: faq
title: FAQ
description: Antworten auf häufige Fragen.
fingerprint: dd2ea1a96488bc21ef4960ea695d9dad539eeceac367b25b58543ff4d256d0f4
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# FAQ {#faq}

## Häufige Fehler mit Anführungszeichen {#common-errors-with-quotes}

Wenn Sie einen Fehler dieser Art erhalten:

```
err:unable to parse file "foo.yaml": error converting YAML to JSON: yaml: line 8: did not find expected key
```

liegt das wahrscheinlich daran, dass Sie einen JSON-Wert anstelle einer Zeichenkette verwenden. Weitere Details finden Sie in der Datei `venom.log`.

Falsch:

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

Richtig:


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

Beachten Sie das einfache Anführungszeichen beim Wert von `body`.
