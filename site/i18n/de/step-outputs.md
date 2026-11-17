---
translates: step-outputs
title: Die Ausgabe eines Testschritts als Eingabe eines anderen verwenden
description: Wie ein Schritt sein Ergebnis an den nächsten Schritt weitergibt.
fingerprint: 0b48c4fa6f51fc685b64e68b3c67da6261ca7863c29554e824ea525eac5d0679
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# Die Ausgabe eines Testschritts als Eingabe eines anderen Testschritts verwenden {#use-outputs-from-a-test-step-as-input-of-another-test-step}

Um eine Eigenschaft eines Testschritts in einem folgenden Testcase oder Schritt wiederzuverwenden, müssen Sie die Variable extrahieren, wie im folgenden Beispiel. 

Nach der Ausführung des ersten Schritts extrahiert `venom` mit dem regulären Ausdruck `foo with a ([a-z]+) here` einen Wert aus dem Inhalt der Eigenschaft `result.systemout`, die der `executor` zurückgibt.
Diese Variable kann dann in einem anderen Test unter dem Namen `testA.myvariable` wiederverwendet werden, wobei `testA` dem Namen des Testcases entspricht. Zusätzlich kann ein Standardwert angegeben werden, falls die Variable nicht aus der Ausgabe extrahiert werden kann, was beim Auswerten von JSON-Ausgaben häufig vorkommt.

```yaml
name: MyTestSuite
testcases:
- name: testA
  steps:
  - type: exec
    script: echo 'foo with a bar here'
    vars:
      myvariable:
        from: result.systemout
        regex: foo with a ([a-z]+) here
        default: "somevalue"

- name: testB
  steps:
  - type: exec
    script: echo {{.testA.myvariable}}
    assertions:
    - result.code ShouldEqual 0
    - result.systemout ShouldContainSubstring bar
```
