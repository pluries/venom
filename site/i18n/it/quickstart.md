---
translates: quickstart
title: Guida rapida
description: Installa venom, scrivi una suite ed eseguila.
fingerprint: ff09c7857b2e31620d2ae8d74003a0e186283bb52cf0d3334055d5d020980dc4
drafted_by: claude
drafted_on: 2026-11-24
status: reviewed
reviewed_by: Christian Stephan
reviewed_on: 2026-11-25
reviewed_fingerprint: ff09c7857b2e31620d2ae8d74003a0e186283bb52cf0d3334055d5d020980dc4
---
Questa pagina ti porta da zero a una test suite che passa. Ti serve un terminale su Linux, macOS o Windows.

## Installa {#install}

Scarica l'eseguibile dell'ultima release per la tua piattaforma. Questo è l'esempio per Linux. La pagina [installazione](installing.html) elenca tutte le piattaforme e la pagina [verificare una release](verify-release.html) mostra come controllare la firma:

```
curl -L -o venom https://github.com/pluries/venom/releases/latest/download/venom.linux-amd64
chmod +x venom
```

I comandi scrivono un eseguibile `venom` nella cartella corrente.

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
