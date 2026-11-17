---
translates: faq
title: 常见问题
description: 常见问题解答。
fingerprint: dd2ea1a96488bc21ef4960ea695d9dad539eeceac367b25b58543ff4d256d0f4
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# 常见问题 {#faq}

## 引号导致的常见错误 {#common-errors-with-quotes}

如果您遇到这样的错误：

```
err:unable to parse file "foo.yaml": error converting YAML to JSON: yaml: line 8: did not find expected key
```

这很可能是因为您试图使用 json 值而不是字符串。您应该能在 `venom.log` 文件中看到更多细节。

错误写法：

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

正确写法：


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

请注意 `body` 的值使用了单引号。
