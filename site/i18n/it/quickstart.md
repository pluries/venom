---
translates: quickstart
title: Guida rapida
description: Installa venom, scrivi una suite ed eseguila.
fingerprint: 86cf4090a52b3ea8d391e1a37a1038fbbf6b86519dd9a23afd4e7391ceb9b402
drafted_by: claude
drafted_on: 2026-11-16
status: reviewed
reviewed_by: Christian Stephan
reviewed_on: 2026-11-16
reviewed_fingerprint: 86cf4090a52b3ea8d391e1a37a1038fbbf6b86519dd9a23afd4e7391ceb9b402
---
Questa pagina ti porta da zero a una test suite che passa. Ti servono Go e un clone del repository di venom.

## Installa {#install}

Compila lo strumento da riga di comando da un clone del repository:

```
go build -o venom ./cmd/venom
```

Il comando scrive un binario `venom` nella cartella corrente. Quando esiste una release, puoi scaricarne il binario invece di compilarlo.

## Scrivi una suite {#write-a-suite}

Una suite è un file YAML. Questa ha un test case con uno step. Lo step esegue un comando di shell e due assertion ne verificano il risultato:

````yaml
name: Quickstart suite
testcases:
  - name: say hello
    steps:
      - type: exec
        script: echo "hello venom"
        assertions:
          - result.systemout ShouldContainSubstring hello
          - result.code ShouldEqual 0
````

- `type: exec` esegue lo `script` in una shell.
- `result.systemout ShouldContainSubstring hello` verifica che l'output contenga la parola `hello`.
- `result.code ShouldEqual 0` verifica che il comando sia terminato senza errori.

## Eseguila {#run-it}

Salva il file come `quickstart.yml` ed eseguilo:

```
./venom run quickstart.yml
```

## Leggi il risultato {#read-the-result}

Venom stampa ogni test case e il suo stato. Quando tutte le assertion sono soddisfatte, l'ultima riga è:

```
final status: PASS
```

Un'assertion fallita viene stampata con lo step, l'assertion e il valore trovato da venom. Il codice di uscita è diverso da zero quando una suite fallisce.

Venom è un fork indipendente di [ovh/venom](https://github.com/ovh/venom). Poi leggi la [panoramica](overview.html) o la pagina sulle [test suite](testsuites.html).
