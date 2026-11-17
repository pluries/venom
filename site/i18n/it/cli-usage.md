---
translates: cli-usage
title: Uso della CLI
description: I comandi di venom e le loro opzioni.
fingerprint: 02ee0f4bbd022eb88342dc24560b249d66c272730ab0c8285af92241332fa21a
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# Uso della CLI {#cli-usage}

La CLI `venom` è composta da diversi comandi:

```bash
$ venom -h
Venom - RUN Integration Tests

Usage:
  venom [command]

Available Commands:
  help        Help about any command
  run         Run Tests
  version     Display Version of venom: venom version

Flags:
  -h, --help   help for venom

Use "venom [command] --help" for more information about a command.
```

Puoi vedere l'aiuto di un comando con `venom [command] -h`:

```bash
$ venom run -h

run integration tests

Usage:
  venom run [flags]

Examples:
  Run all testsuites containing in files ending with *.yml or *.yaml: venom run
  Run a single testsuite: venom run mytestfile.yml
  Run a single testsuite and export the result in JSON format in test/ folder: venom run mytestfile.yml --format=json --output-dir=test
  Run a single testsuite and export the result in XML and HTML formats in test/ folder: venom run mytestfile.yml --format=xml --output-dir=test --html-report
  Run a single testsuite and specify a variable: venom run mytestfile.yml --var="foo=bar"
  Run a single testsuite and load all variables from a file: venom run mytestfile.yml --var-from-file variables.yaml
  Run all testsuites containing in files ending with *.yml or *.yaml with verbosity: VENOM_VERBOSE=2 venom run

  Notice that variables initialized with -var-from-file argument can be overrided with -var argument

  More info: https://github.com/ovh/venom

Flags:
      --format string           --format:json, tap, xml, yaml (default "xml")
  -h, --help                    help for run
      --html-report             Generate HTML Report
      --lib-dir string          Lib Directory: can contain user executors. example:/etc/venom/lib:$HOME/venom.d/lib
      --output-dir string       Output Directory: create tests results file inside this directory
      --parallel int            Number of test files to run at the same time. 1 (default) runs them one after another (default 1)
      --stop-on-failure         Stop running Test Suite on first Test Case failure
      --strict-variables        Fail a step that names an unknown variable
      --var stringArray         --var cds='cds -f config.json' --var cds2='cds -f config.json'
      --var-from-file strings   --var-from-file filename.yaml --var-from-file filename2.yaml: yaml, must contains a dictionary
  -v, --verbose count           verbose. -v (INFO level in venom.log file), -vv to very verbose (DEBUG level) and -vvv to very verbose with CPU Profiling
```

## Eseguire le test suite in un ordine preciso {#run-test-suites-in-a-specific-order}

- `venom run 01_foo.yml 02_foo.yml` esegue 01 prima di 02. 
- `venom run 02_foo.yml 01_foo.yml` esegue 02 prima di 01.

Se vuoi ordinare molti file di testsuite, puoi usare i comandi standard, ad esempio:

```bash
venom run `find . -type f -name "*.yml"|sort`
```

## Eseguire più file di test contemporaneamente {#run-several-test-files-at-the-same-time}

Per impostazione predefinita `venom` esegue un file di test alla volta. `--parallel N` ne esegue fino a `N` insieme:

```bash
$ venom run --parallel 4
```

I test di integrazione passano la maggior parte del tempo ad aspettare altri sistemi, ed è proprio quell'attesa che
viene condivisa. Su otto file che aspettano tutti, misurato: **3.4× più veloce con `--parallel 4`, 5.6× con
`--parallel 8`**.

L'unità è il **file**: i test case dentro un file, e gli step dentro un test case, vengono ancora eseguiti uno
dopo l'altro, perché uno step passa i suoi valori a quello successivo. Eseguire un solo file con
`--parallel 4` quindi non cambia nulla.

**Quanto guadagni dipende dal fatto che i tuoi file aspettino davvero.** Circa 0.35 s di ogni esecuzione sono
fissi (avvio, parsing, scrittura dei report) e nessun numero di worker li elimina. Sulle suite `tests/` di venom,
che sono brevi e per lo più non aspettano, `--parallel 4` è solo circa 1.2× più veloce.
Aspettati un guadagno reale quando i tuoi file restano in attesa di un servizio, e poco quando non lo fanno.

Due cose da sapere:

- **Ogni riga della console è etichettata con il file da cui proviene**, così l'output dei file eseguiti
  contemporaneamente resta leggibile:

  ```
  [exec.yml]   • exec testsuite (tests/exec.yml)
  [vars.yml]   • vars testsuite (tests/vars.yml)
  [exec.yml]   	• stdin PASS
  [vars.yml]   	• set PASS
  ```

  Le righe di file diversi si alternano (non sono raggruppate), ma ognuna dice chi l'ha scritta,
  e una riga non viene mai spezzata tra due file. Segui un singolo file con `grep`:
  `venom run --parallel 4 | grep '^\[exec.yml'`. Con `--parallel 1` non c'è nessuna etichetta e
  l'output è esattamente quello di sempre. Il file di log non cambia in nessuno dei due casi: ogni riga
  è già etichettata con il suo file di test.
- **`--stop-on-failure` ferma l'intera esecuzione.** Quando un file di test fallisce, non ne viene avviato nessun altro;
  i file già in esecuzione vengono lasciati finire, così ogni file avviato riporta comunque un risultato.
  I file mai avviati sono riportati come saltati. Nelle versioni precedenti questo flag saltava soltanto i
  test case rimanenti del file fallito, e l'esecuzione proseguiva con il file successivo.

`-vvv` non può essere combinato con `--parallel 2` o più: il profiling della CPU è a livello di processo, quindi non può
produrre un profilo per ciascun file eseguito contemporaneamente. L'esecuzione si ferma all'avvio e lo dichiara.

## Supporto di globstar {#globstar-support}

La CLI `venom` supporta globstar:

```
$ venom run ./foo/b*/**/z*.yml
```

## Variabili {#variables}

Per specificare singole variabili da riga di comando, usa l'opzione `--var` quando esegui i comandi `venom run`:

```bash
$ venom run --var="foo=bar"
$ venom run --var='foo_list=["biz","buz"]'
$ venom run --var='foo={"biz":"bar","biz":"barr"}'
```

L'opzione `--var` può essere usata più volte in un singolo comando.

### File di definizione delle variabili {#variable-definitions-files}

Per impostare molte variabili, è più comodo specificarne i valori in un file di definizione delle variabili. Questo file è un dizionario YAML. Devi indicare quel file da riga di comando con `--var-from-file`:

```bash
venom run --var-from-file variables.yaml
```

### Variabili d'ambiente {#environment-variables}

Come ripiego rispetto agli altri modi di definire le variabili, lo strumento `venom` cerca nell'ambiente del proprio processo le variabili d'ambiente chiamate `VENOM_VAR_` seguite dal nome di una variabile dichiarata.

```bash
$ export VENOM_VAR_foo=bar
$ venom run *.yml
```

Puoi anche definire la variabile d'ambiente ed eseguire la tua testsuite in una sola riga:

```bash
$ VENOM_VAR_foo=bar venom run *.yml
```

## Argomenti {#arguments}

Puoi definire gli argomenti da riga di comando usando il nome del flag.

I flag sono elencati nel risultato del comando di aiuto.

Elenco dei flag disponibili per il comando `venom run`:

```
Flags:
      --format string           --format:json, tap, xml, yaml (default "xml")
  -h, --help                    help for run
      --html-report             Generate HTML Report
      --lib-dir string          Lib Directory: can contain user executors. example:/etc/venom/lib:$HOME/venom.d/lib
      --output-dir string       Output Directory: create tests results file inside this directory
      --parallel int            Number of test files to run at the same time. 1 (default) runs them one after another (default 1)
      --stop-on-failure         Stop running Test Suite on first Test Case failure
      --strict-variables        Fail a step that names an unknown variable
      --var stringArray         --var cds='cds -f config.json' --var cds2='cds -f config.json'
      --var-from-file strings   --var-from-file filename.yaml --var-from-file filename2.yaml: yaml, must contains a dictionary
  -v, --verbose count           verbose. -vv to very verbose and -vvv to very verbose with CPU Profiling
```

### Definire gli argomenti con le variabili d'ambiente {#define-arguments-with-environment-variables}

Puoi anche definire gli argomenti con le variabili d'ambiente:

```bash
# is the same as
VENOM_FORMAT=json venom run my-test-suite.yml

# is equivalent to
venom run my-test-suite.yml --format=json
```

I flag e il loro equivalente con le variabili d'ambiente:

- il flag `--format="json"` equivale alla variabile d'ambiente `VENOM_FORMAT="json"`
- il flag `--lib-dir="/etc/venom/lib:$HOME/venom.d/lib"` equivale alla variabile d'ambiente `VENOM_LIB_DIR="/etc/venom/lib"`
- il flag `--output-dir="test-results"` equivale alla variabile d'ambiente `VENOM_OUTPUT_DIR="test-results"`
- il flag `--stop-on-failure` equivale alla variabile d'ambiente `VENOM_STOP_ON_FAILURE=true`
- il flag `--strict-variables` equivale alla variabile d'ambiente `VENOM_STRICT_VARIABLES=true`
- il flag `--var foo=bar` equivale alla variabile d'ambiente `VENOM_VAR_foo='bar'`
- il flag `--var-from-file fileA.yml fileB.yml` equivale alla variabile d'ambiente `VENOM_VAR_FROM_FILE="fileA.yml fileB.yml"`
- il flag `--parallel 4` equivale alla variabile d'ambiente `VENOM_PARALLEL=4`
- il flag `-v` equivale alla variabile d'ambiente `VENOM_VERBOSE=1`
- il flag `-vv` equivale alla variabile d'ambiente `VENOM_VERBOSE=2`

È possibile impostare la variabile d'ambiente `NO_COLOR=1` per disattivare i colori nell'output.

## Usare un file di configurazione {#use-a-configuration-file}

Puoi definire le impostazioni di Venom con un file di configurazione `.venomrc`. Questo file di configurazione va posto nella directory corrente o nella directory `home`.

```yml
variables: 
  - foo=bar
variables_files:
  - my_var_file.yaml
stop_on_failure: true
strict_variables: true
format: xml
output_dir: output
lib_dir: lib
verbosity: 3
parallel: 4
```

Tieni presente che i flag da riga di comando hanno la precedenza sul file di configurazione. Il file di configurazione ha la precedenza sulle variabili d'ambiente.

## Modello di fiducia {#trust-model}

Venom non ha una sandbox. Una test suite è codice, e viene eseguita con i permessi dell'utente che esegue venom.

- Le test suite, gli executor utente in `lib/` e i plugin Go (`lib/*.so`) vengono tutti eseguiti con i tuoi permessi. Esegui solo una suite che eseguiresti come script.
- Non inserire mai il risultato di uno step, né alcun valore che non controlli, in uno `script:` o in un testo SQL senza applicare il quoting. Il risultato di uno step può contenere qualsiasi cosa, e venom non ne fa l'escape per una shell né per SQL.
- In CI, non eseguire suite provenienti da fork o pull request non riviste, e non rendere disponibili i secret a tali esecuzioni.
- Un secret dichiarato in `secrets` o in `.venomrc` viene nascosto nei report, nei log e nella console. Vengono nascosti solo i valori dichiarati.
- Gli executor `ssh`, `imap` e `smtp` non verificano l'identità del server per impostazione predefinita. Venom registra un avviso una volta per esecuzione quando è così. Per attivare la verifica, imposta `knownhosts` o `hostkey` su uno step `ssh`, e `verifytls: true` (sotto `auth:` per `imap`) insieme a `withtls: true` su uno step `imap` o `smtp`.
- L'executor `web` ignora per impostazione predefinita gli errori dei certificati HTTPS. Imposta la variabile `verifyTls` a `true` perché il browser rifiuti un certificato non valido. Vedi [executors/web](executors/web/README.md).
