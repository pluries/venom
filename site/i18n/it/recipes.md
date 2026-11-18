---
translates: recipes
title: Ricette
description: Scrivi test per browser e app mobile come ricette che un controllo e un controllo negativo possono dimostrare.
fingerprint: 3dd31333d7ad4d4487d1c297fdc7e02dc69595aee1619af22617d3bfb867d5d5
drafted_by: claude
drafted_on: 2026-11-18
status: machine-drafted
---
Una ricetta è una test suite di venom che pilota una pagina web o un'app mobile. Questa pagina mostra come organizzare le ricette, come darle un nome, come dimostrare che testano qualcosa e come tenere segreti e risultati fuori dai posti sbagliati. Ti servono venom e, per le ricette web, un browser installato da Playwright. Le pagine dell'[executor web](web.html) e dell'[executor mobile](mobile.html) elencano ogni azione.

Una ricetta è una suite normale. Usa step `web` o step `mobile` e nient'altro di nuovo. Ecco una piccola ricetta web. L'indirizzo della pagina è la variabile `appUrl`, così lo stesso file funziona su qualsiasi copia della pagina:

{{include site/examples/recipes/web-page.yml yaml}}

Eseguila con l'indirizzo della tua pagina:

```
venom run --var appUrl=http://localhost:8000 web-page.yml
```

## Struttura delle cartelle {#folder-layout}

Tieni le ricette di una funzionalità in una sola cartella sotto `tests/`, accanto alle altre suite di venom. Per esempio:

```
tests/
  site/
    site-languages.yml
  recipes/
    reference/
      mobile/
        counter.yml
```

Una cartella per funzionalità. Uno o più file `.yml` al suo interno. Una cartella con sottocartelle si controlla indicando le sottocartelle. Tieni i file di variabili con le credenziali fuori dalla cartella, oppure ignorati da git (vedi Credenziali).

## Nomi dei testcase e id della specifica {#testcase-names-and-the-spec-id}

Ogni testcase dimostra uno scenario di accettazione di una specifica. Fai iniziare il nome del testcase con l'id dello scenario, poi una breve descrizione: `US1-AS3 an item typed in the input is added to the list`. L'id ha la forma `USn-ASm`: user story `n`, scenario di accettazione `m`.

Indica la specifica una sola volta, in una variabile della suite:

```
vars:
  spec: 014-web-mobile-recipes
```

Il valore è il nome della cartella della specifica. Il controllo delle ricette legge `spec.md` in quella cartella e cerca gli id. L'opzione `--specs-dir` gli dice dove si trovano le cartelle delle specifiche. Due file della stessa cartella non devono rivendicare lo stesso scenario.

Per vedere quali scenari dimostra una ricetta, aggiungi `--coverage` con l'id della specifica: `venom recipe check --specs-dir <specs folder> --coverage 014-web-mobile-recipes tests/recipes`. Dopo i rilievi elenca ogni scenario della specifica, `US1-AS3 proven: <file> / <testcase>` oppure `US1-AS4 not proven`. Un testcase dimostra uno scenario solo se non è saltato e contiene un'asserzione che legge l'app. L'opzione non cambia lo stato di uscita. Lo script di copertura del dev set non legge i file `.yml`: per contare le ricette usa quindi questa opzione, non quello script.

## Selettori {#selectors}

Uno step `find`, `click` o `fill` richiede un selettore. Usa CSS o XPath. Preferisci un attributo di test stabile, come `data-test`:

```
find: '[data-test="add-button"]'
```

Gli id e i nomi di classe cambiano quando qualcuno ridisegna la pagina. Un attributo `data-test` esiste solo perché i test possano trovare l'elemento. Se l'app non ne ha, chiedi agli sviluppatori di aggiungerli prima di scrivere la ricetta.

`find` non aspetta. Conta gli elementi che esistono in quel momento. Quando la pagina carica dati dopo il primo rendering, aspetta prima con `wait`, oppure usa `click` o `fill`, che aspettano il proprio elemento. In uno step mobile, imposta esattamente un localizzatore, come `accessibilityId`.

## Passi di dimostrazione {#proof-steps}

Una ricetta che passa non dimostra nulla finché non l'hai vista fallire. Tre passi forniscono la dimostrazione:

1. Replay. Esegui la ricetta con `venom run` sull'app. Ogni testcase deve passare. Ripetila alcune volte: un testcase che passa una volta e fallisce un'altra non è utilizzabile.
2. Controllo negativo. Per ogni testcase, copia la ricetta, cambia il primo valore atteso che proviene dall'app ed esegui la copia. L'esecuzione deve terminare con stato 2 e nominare quel testcase. Lascia intatto il file originale.
3. Controllo della ricetta. Esegui il controllo, che legge i file senza un browser né un dispositivo:

```
venom recipe check --specs-dir <folder of the specs> tests/site
```

Il controllo termina con 0 quando non trova nulla, con 1 quando segnala un problema e con 2 quando non riesce a leggere il proprio input. Ogni problema indica il file, la riga, la regola e il testcase. Segnala un testcase senza id di scenario, un id ripetuto, un id che la specifica non ha, un testcase senza alcuna assertion su un valore letto dall'app, un'assertion che non può fallire, un testo simile a una credenziale e un indirizzo scritto in modo letterale.

Il controllo è statico. Non può leggere dentro uno script `execute` e non può sapere se un valore atteso è giusto. Il controllo negativo copre entrambi i casi.

## Cosa gli executor non sanno fare {#what-the-executors-cannot-do}

L'executor web non ha uno step per hover, trascinamento o scorciatoie da tastiera, e `find` non può leggere un numero come la larghezza di scorrimento. L'azione `execute` esegue uno script nella pagina e restituisce un valore, su cui puoi fare un'assertion. Questo esempio legge di quanto scorre la pagina in orizzontale e quanti elementi elenca:

{{include site/examples/recipes/web-execute.yml yaml}}

Se `execute` non basta, annota la lacuna e non scrivere una ricetta per quello scenario. Non mettere tutte le assertion di un testcase su un solo risultato di script senza un controllo negativo: il controllo non può leggere lo script.

Una ricetta mobile richiede un server Appium e un dispositivo o un emulatore. Senza di essi non funziona. Questa porta l'etichetta `# needs: device` alla prima riga, così i test del sito la saltano dove non esiste alcun dispositivo:

{{include site/examples/recipes/mobile-counter.yml yaml}}

## Credenziali {#credentials}

Una ricetta non contiene password, token né chiavi. Un login proviene da variabili. Passale sulla riga di comando o in un file di variabili che git ignora:

```
venom run --var-from-file login.vars.yml tests/site/login.yml
```

Nella ricetta scrivi la variabile: `text: "{{.password}}"`. Il controllo segnala un testo simile a una credenziale in qualsiasi step o valore predefinito.

Le ricette girano su un ambiente di test, mai sulla produzione. L'indirizzo è una variabile che imposta la pipeline, e `venom run` non ti impedisce di puntarlo a un sistema reale. Tieni l'elenco degli host consentiti nella tua pipeline.

## Risultati e artefatti {#results-and-artifacts}

Uno step fallito può scrivere un dump della pagina, uno screenshot o una traccia. Questi file possono contenere dati dell'app. Scrivili in una cartella dei risultati che git ignora e non farne il commit. In CI, conservali per poco tempo, per esempio alcuni giorni, e dai accesso solo alle persone che ne hanno bisogno: lo spazio di archiviazione è riservato. Non copiarli in archivi di lunga durata.

## Variabili non definite {#undefined-variables}

Una variabile che dimentichi di passare, per impostazione predefinita, non ferma l'esecuzione. Venom lascia `{{.password}}` come testo letterale `{{.password}}` e lo step digita quel testo. L'esecuzione non fallisce per questo motivo.

Passa `--strict-variables` per cambiare questo comportamento. Venom si ferma allora con stato di uscita 2 e il messaggio `unknown variable(s) "password"`. Usa l'opzione in ogni pipeline che esegue ricette:

```
venom run --strict-variables --var-from-file login.vars.yml tests/site/login.yml
```
