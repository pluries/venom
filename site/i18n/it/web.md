---
translates: web
title: Web
description: Come usare l'executor Web in una test suite di venom.
fingerprint: d700bc6151e18429c9339fb06ffe6bc6b56b451abd92c6c1328ee3c6cbec08f8
drafted_by: claude
drafted_on: 2026-11-19
status: machine-drafted
---
# Venom - Executor Web {#venom---executor-web}

Step per navigare in un'applicazione web

Caso d'uso: hai un'applicazione web e vuoi verificarne alcuni comportamenti.

L'executor web ti permette di navigare nella tua applicazione web ed eseguire azioni.

## Certificati HTTPS {#https-certificates}

Per impostazione predefinita l'executor web ignora gli errori dei certificati HTTPS (il contesto del browser viene creato con `IgnoreHttpsErrors`).
Uno step web va a buon fine anche su un sito con un certificato scaduto, autofirmato o intestato a un altro host, quindi non
fare affidamento su di esso per controllare i certificati. Per far rifiutare al browser un certificato non valido, imposta la
variabile `verifyTls` a `true`: lo step fallisce. L'executor web non può considerare attendibile una CA privata: Playwright non offre
questa opzione per un contesto del browser.

## Browser {#browsers}

L'executor web pilota un browser tramite [Playwright](https://playwright.dev) (binding Go
`github.com/mxschmitt/playwright-go`). Playwright gestisce le proprie build dei browser: non c'è nessun `chromedriver` né `geckodriver`
da installare e nessuna versione da abbinare al browser.

Installa una sola volta per macchina il driver di Playwright e i browser usati dalle tue suite, insieme ai relativi pacchetti
del sistema operativo (`--with-deps` richiede `sudo` su Linux):

```bash
go run github.com/mxschmitt/playwright-go/cmd/playwright@v0.6201.1 install --with-deps chromium   # or firefox, webkit
```

venom non scarica mai un browser da solo: una suite che ne richiede uno non installato fallisce in fase di setup, e
il messaggio di errore riporta questo comando.

### Argomenti {#arguments}
* Chromium: [elenco degli argomenti](https://peter.sh/experiments/chromium-command-line-switches/). Un argomento senza
  trattino iniziale riceve `--`, come faceva chromedriver.
* Firefox: [elenco degli argomenti](https://wiki.mozilla.org/Firefox/CommandLineOptions),
  [elenco delle preferenze](https://searchfox.org/mozilla-central/source/modules/libpref/init/all.js)

## Variabili {#variables}

Puoi definire parametri per configurare il browser usato durante la test suite. Tutti i parametri sono opzionali:
* driver (default: `chrome`): browser da usare:
  * `chrome`: Chromium
  * `gecko`: Firefox (la build di Playwright)
  * `webkit`: WebKit, il motore alla base di Safari
* width, height: dimensioni della pagina. Senza di essi la pagina mantiene le dimensioni della finestra del browser
* args: argomenti del browser
* prefs: preferenze di Firefox (solo `gecko`)
* timeout (default: `180`): tempo massimo, in secondi, per il caricamento di una pagina (`navigate`, `historyAction`, la risposta a un popup),
  e perché finisca una navigazione avviata da uno step
* proxy: proxy da usare per interrogare il sito web (formato: `[server]:[port]`)
* verifyTls (default: `false`): fa rifiutare al browser le pagine con un certificato HTTPS non valido, così lo step fallisce. Senza questa variabile
  gli errori dei certificati vengono ignorati, come è sempre stato
* headless (default: `false`): esegue il browser senza finestra. Utile in una pipeline CI/CD
* binaryPath: eseguibile del browser da usare al posto di quello di Playwright (solo `chrome`)
* trace (default: `false`): salva una trace di Playwright di ogni step fallito in `page.trace.zip`. Vedi
  [Diagnostica degli errori](#failure-diagnostics)
* testIdAttribute (default: `data-testid`): l'attributo che legge un locator `TESTID` (vedi [Locator](#locators)). Impostalo
  una sola volta per la suite, per una pagina che marca i suoi elementi con `data-test` o con un altro nome. Vedi [Insidie](#traps). Il nome deve corrispondere a `^[A-Za-z_][A-Za-z0-9_:.-]*$`: qualsiasi altro valore fa fallire il Setup, prima dell'avvio del browser

Chromium viene eseguito con la sandbox attiva, come sotto chromedriver. Una suite che ha bisogno di disattivarla (in un container eseguito come
root, per esempio) passa `--no-sandbox` in `args`. Un host che limita i user namespace non privilegiati, come fa Ubuntu
23.10 e successive in versione nativa, non riesce ad avviare la sandbox del Chromium di Playwright: il setup fallisce e indica le soluzioni:
`--no-sandbox` in `args`, `binaryPath` che punta a un Chrome di sistema, oppure consentire i namespace
(`sysctl kernel.apparmor_restrict_unprivileged_userns=0`).

Gli errori dei certificati HTTPS vengono ignorati per impostazione predefinita, come avveniva con la sessione WebDriver usata in precedenza da questo executor
(`acceptInsecureCerts`). Imposta `verifyTls` per controllarli (vedi Certificati HTTPS più sopra).

Ancora accettati, ma **ignorati con un avviso**, perché non hanno un equivalente in Playwright:
* driverPath, driverPort: non esiste più alcun processo driver
* detach: Playwright non può lasciare il browser in esecuzione dopo la suite
* debug, logLevel: impostavano il livello di log del vecchio client WebDriver; usa invece l'opzione `-v` di venom
* prefs con `chrome`, binaryPath con `gecko`: Playwright pilota la propria build di Firefox

## Azioni {#action}
Le azioni ti permettono di manipolare il browser

Un'azione su un elemento (`click`, `fill`, `select`, `uploadFile`, `selectFrame`, `hover`, `dragAndDrop` e `press` con `find`) attende che l'elemento sia pronto,
fino al suo `syncTimeout` o a 30 secondi. `waitFor` attende che l'elemento raggiunga il suo `state`, entro il suo `syncTimeout`. `find` non attende mai: conta gli elementi presenti in quel momento, quindi
`result.find ShouldEqual 0` resta veloce. `find`, `waitFor`, `hover`, `dragAndDrop` e `press`, come le azioni sopra,
attendono una navigazione già in corso prima di eseguirsi. Dopo un'azione che può caricare una pagina (`click`, `fill`, `select`,
`uploadFile`, `execute`, `waitFor`, `hover`, `dragAndDrop`, `press`, la risposta a un popup), lo step attende che la pagina finisca di caricarsi prima di leggere
`result.title` e `result.url`. Questo vale anche per una navigazione che la pagina avvia da sola, da uno script
(`location.href = …`) o da un timer senza ritardo. Una navigazione che non finisce entro `timeout` fa fallire lo step,
e anche quelli successivi, finché un `navigate` non la sostituisce. Nessun `page.dump.html` viene scritto mentre la pagina sta navigando.

Una chiamata alla pagina che non ritorna mai fa fallire il suo step: dopo 60 secondi per una lettura (il titolo, il sorgente, un conteggio, un
testo; più il `syncTimeout` dello step per un elemento che attende), dopo 30 secondi per `execute`.
La chiamata al browser continua comunque a girare, quindi lo step successivo la attende prima di inviare la propria. Una chiamata con un limite
proprio (le letture e `execute`) attende al massimo quel limite e poi fallisce allo stesso modo, mentre l'attesa prosegue
per lo step successivo. Un'azione senza un limite proprio (un click, una select, uno screenshot) attende al massimo 60 secondi, registra un
avviso e prosegue. `navigate`, `historyAction` e uno svuotamento del timer che ha concluso una navigazione non attendono mai, e un dialog
aperto nel frattempo viene chiuso prima, come di consueto. Uno step fallito non scrive `page.dump.html` solo finché quella chiamata
è ancora in esecuzione: registra invece un avviso.

Elenco dei valori di output disponibili:
* result.url: URL della pagina corrente
* result.timeseconds: durata dell'esecuzione dell'azione
* result.title: titolo della pagina corrente

### Navigate {#navigate}
Naviga verso un URL specifico

#### Input {#input}
* URL: URL verso cui navigare
* Reset: reimposta lo stato del browser

#### Esempio {#example}
```yaml
  - type: web
    action:
      navigate:
        url: https://www.google.fr
```

### Find {#find}
Cerca un elemento a partire da un selettore

[Link al tutorial sui selettori CSS](https://developer.mozilla.org/fr/docs/Web/CSS/CSS_Selectors)
[Link al tutorial sui selettori XPATH](https://developer.mozilla.org/fr/docs/Web/XPath)

#### Input {#input-1}
* Selector: espressione da usare per identificare l'elemento web
* Locator: locator da usare per cercare l'elemento: `CSS` o `XPATH`, oppure uno dei tipi descritti in [Locator](#locators)

#### Output {#output}
* result.find: restituisce il numero di oggetti identificati

#### Esempio {#example-1}
```yaml
  - type: web
    action:
      find: 
        selector: .gsfi
        locator: CSS
    assertions:
    - result.find ShouldEqual 2

  - type: web
    action:
      find:
        selector: //div[@class='FPdoLc lJ9FBc']/center/input[@value='Recherche Google']
        locator: XPATH
    assertions:
    - result.find ShouldEqual 1
    - result.value ShouldEqual Recherche Google
```

### Locator {#locators}
Un elemento `find` indica il proprio tipo in `locator`. Oltre a `CSS` e `XPATH`, cinque tipi corrispondono a ciò che vede un utente
o a ciò che la pagina marca per i test. Preferiscili a un percorso CSS: sopravvivono a un restyling della pagina.

#### Input {#input-2}
* `locator: ROLE`: `selector` è un ruolo ARIA (`button`, `link`, `textbox`, `checkbox`, `heading`, `listitem`, ...).
  `name` è facoltativo e filtra per il nome accessibile dell'elemento; un `name` vuoto significa nessun filtro sul nome
* `locator: TEXT`: `selector` è il testo dell'elemento
* `locator: LABEL`: `selector` è il testo dell'etichetta di un campo di un modulo
* `locator: PLACEHOLDER`: `selector` è il segnaposto (placeholder) di un campo
* `locator: TESTID`: `selector` è l'id di test. Legge l'attributo `data-testid`, oppure l'attributo impostato dalla variabile
  `testIdAttribute`. Corrisponde all'intero valore e non accetta `exact`
* `exact` (`true` o `false`, default `false`): per `ROLE` si applica a `name`, e per `TEXT`, `LABEL` e `PLACEHOLDER`
  a `selector`

Per impostazione predefinita `TEXT`, `LABEL` e il nome di un `ROLE` corrispondono senza distinguere maiuscole e minuscole, come
sottostringa, e ignorano gli spazi in più: il testo `add` corrisponde al pulsante `Add item`. Con `exact: true` la corrispondenza
riguarda l'intero testo, con le maiuscole come scritte. `PLACEHOLDER` corrisponde all'attributo così come è scritto.

Un valore è sempre letto come testo semplice. Non è mai un selettore CSS, un prefisso di motore o un'espressione regolare. Uno step
che trova più elementi usa il primo.

#### Esempio {#example-2}
```yaml
  - type: web
    action:
      click:
        find:
          locator: ROLE
          selector: button
          name: Add item
          exact: true

  - type: web
    action:
      fill:
      - find:
          locator: LABEL
          selector: Email
        text: user@example.com

  - type: web
    action:
      find:
        locator: TESTID
        selector: cart-count
    assertions:
    - result.find ShouldEqual 1
```

#### Errori {#errors}
Uno step con un `find` errato fallisce prima di agire, e il messaggio nomina la chiave:
* un `locator` che non è uno dei sette tipi: `invalid find element locator 'X' (must be CSS, XPATH, ROLE, TEXT, LABEL, PLACEHOLDER or TESTID)`
* un selettore di `ROLE` che non è un ruolo ARIA: il messaggio elenca i ruoli
* un `selector` vuoto per `TEXT`, `LABEL`, `PLACEHOLDER` o `TESTID`
* un selettore `TESTID` che contiene un carattere NUL
* `name` su un tipo diverso da `ROLE`, oppure `exact` su `TESTID`: il messaggio dice quali tipi lo accettano
* `name` che non è una stringa, oppure `exact` che non è `true` né `false`
* qualsiasi altra chiave: `invalid find element key 'X' for a ROLE locator (must be locator, selector, name, exact)`

Questi controlli valgono per i cinque tipi di questa sezione. Le chiavi in più di un elemento `CSS` o `XPATH` vengono ignorate, come lo sono sempre state.

### Click {#click}
Fa clic su un elemento a partire da un selettore

#### Input {#input-3}
* Find: elemento da cercare (maggiori informazioni nella sezione find)
* Wait: tempo di attesa dopo il clic (in secondi)
* SyncTimeout: tempo massimo, in secondi, di attesa che l'elemento sia pronto (presente, visibile, abilitato, fermo, non coperto). Default: `30`

#### Esempio {#example-3}
```yaml
  - type: web
    action:
      click:
        find:
          selector: button
          locator: CSS
```

### Fill {#fill}
Compila un elemento (input o textarea) con un testo. Il testo viene digitato dopo il contenuto attuale del campo, come faceva WebDriver;
non lo sostituisce

#### Input {#input-4}
Array di input strutturati:
* Find: elemento da cercare (maggiori informazioni nella sezione find)
* Text: aggiorna il valore dell'elemento con un testo
* Key: aggiorna il valore dell'elemento con un tasto
* SyncTimeout: tempo massimo, in secondi, di attesa che l'elemento sia pronto (presente, visibile, abilitato, fermo, non coperto). Default: `30`

#### Esempio {#example-4}
```yaml
  - type: web
    action:
      fill:
      - find:
          selector: input
          locator: CSS
        text: userName
```

### Select {#select}
Seleziona un'opzione di un elemento web

#### Input {#input-5}
* Find: elemento da cercare (maggiori informazioni nella sezione find)
* Text: opzione da selezionare
* Wait: tempo di attesa dopo il clic (in secondi)
* SyncTimeout: tempo massimo, in secondi, di attesa che l'elemento sia pronto (presente, visibile, abilitato, fermo, non coperto). Default: `30`

#### Esempio {#example-5}
```yaml
  - type: web
    action:
      select:
        find:
          selector: select
          locator: CSS
        text: Option 1
```

### Upload file {#upload-file}
Carica un file su un elemento web

#### Input {#input-6}
* Find: elemento da cercare (maggiori informazioni nella sezione find)
* Files: array di stringhe che definisce i file da caricare
* Wait: tempo di attesa dopo il clic (in secondi)
* SyncTimeout: tempo massimo, in secondi, di attesa che l'elemento sia pronto (presente, visibile, abilitato, fermo, non coperto). Default: `30`

#### Esempio {#example-6}
```yaml
  - type: web
    action:
      uploadFile:
        find:
          selector: input
          locator: CSS
        files: 
        - toUpload.csv
```

### Wait {#wait}
Attende un certo tempo
Un valore intero che definisce il numero di secondi di attesa

#### Esempio {#example-7}
```yaml
  - type: web
    action:
      wait: 1
```

### Wait for {#wait-for}
Attende finché un elemento è in un certo stato. Usalo quando la pagina carica dati o apre un pannello dopo il primo rendering.

#### Input {#input-7}
* Find: elemento da attendere (maggiori informazioni in find e [Locator](#locators))
* State: `visible` (default, anche quando `state` è vuoto), `hidden`, `attached` o `detached`
  * `visible`: l'elemento esiste ed è mostrato
  * `hidden`: l'elemento manca o non è mostrato
  * `attached`: l'elemento esiste nella pagina, mostrato o no
  * `detached`: l'elemento non è nella pagina
* SyncTimeout: tempo massimo, in secondi, di attesa dello stato. Default: `30`. Un valore pari o inferiore a `0` equivale a `30`

Per `visible`, lo step attende sulla prima corrispondenza.

Ogni controllo guarda il primo elemento che corrisponde in quel momento. Se quell'elemento viene rimosso e un altro corrisponde ancora, l'attesa prosegue con il successivo. `detached` attende finché nessun elemento corrisponde più: quando il primo elemento viene rimosso, il successivo diventa il primo, quindi con più corrispondenze l'attesa dura finché non ne resta nessuna. `hidden` segue il primo elemento che corrisponde, che resta lo stesso elemento finché esiste (un elemento nascosto con CSS corrisponde ancora), quindi con più corrispondenze `hidden` può terminare mentre gli altri sono ancora mostrati. Per attendere che ognuno sia nascosto, attendi `detached`, oppure dai a ogni elemento il suo locator.

#### Esempio {#example-8}
```yaml
  - type: web
    action:
      waitFor:
        find:
          locator: TEXT
          selector: Order saved
        state: visible
        syncTimeout: 10

  - type: web
    action:
      waitFor:
        find:
          locator: TESTID
          selector: spinner
        state: hidden
```

#### Errori {#errors-1}
* `waitFor: the field 'find' is required (allowed: find, state, syncTimeout)`
* `waitFor: invalid 'state' X (allowed: visible, hidden, attached, detached)`: lo stato distingue maiuscole e minuscole
* un campo diverso da `find`, `state` e `syncTimeout`: `waitFor: unknown field 'X' (allowed: find, state, syncTimeout)`
* lo stato non viene raggiunto in tempo: `waitFor: the element ROLE "button" name "Add" did not become visible within 30s`, seguito dal
  motivo del browser

### Hover {#hover}
Sposta il puntatore su un elemento, per aprire un menu o un tooltip che compare al passaggio del mouse. Lo step attende che l'elemento sia
pronto, come fa un clic.

#### Input {#input-8}
* Find: elemento su cui passare (maggiori informazioni in find e [Locator](#locators)). Si usa la prima corrispondenza
* SyncTimeout: tempo massimo, in secondi, di attesa che l'elemento sia pronto. Default: `30`

#### Esempio {#example-9}
```yaml
  - type: web
    action:
      hover:
        find:
          locator: ROLE
          selector: link
          name: Products

  - type: web
    action:
      waitFor:
        find:
          locator: ROLE
          selector: menuitem
          name: Shoes
```

#### Errori {#errors-2}
* `hover: the field 'find' is required (allowed: find, syncTimeout)`
* un campo diverso da `find` e `syncTimeout`: `hover: unknown field 'X' (allowed: find, syncTimeout)`
* l'elemento non è pronto in tempo: `hover: ROLE "link" name "Products"`, seguito dal motivo del browser

### Trascinamento {#drag-and-drop}
Trascina un elemento e lo rilascia su un altro. Lo step attende che entrambi siano pronti.

#### Input {#input-9}
* From: elemento da trascinare (maggiori informazioni in find e [Locator](#locators)). Si usa la prima corrispondenza
* To: elemento su cui rilasciare. Si usa la prima corrispondenza
* SyncTimeout: tempo massimo, in secondi, di attesa del completamento del trascinamento. Default: `30`

#### Esempio {#example-10}
```yaml
  - type: web
    action:
      dragAndDrop:
        from:
          locator: TEXT
          selector: Task A
          exact: true
        to:
          locator: TESTID
          selector: column-done
```

#### Errori {#errors-3}
* `dragAndDrop: the field 'from' is required (allowed: from, to, syncTimeout)`, e lo stesso per `to`
* un campo diverso da `from`, `to` e `syncTimeout`: `dragAndDrop: unknown field 'X' (allowed: from, to, syncTimeout)`
* il rilascio fallisce o scade: `dragAndDrop: TEXT "Task A" exact to TESTID "column-done"`, seguito dal motivo del browser

### Press {#press}
Preme un tasto o una combinazione di tasti, come `Enter`, `Escape`, `Tab` o `Control+A`. Con `find`, il tasto va a quell'elemento.
Senza, il tasto va all'elemento che ha il focus. `press` accetta un tasto o una combinazione, non un testo: per digitare un
testo, usa `fill`.

#### Input {#input-10}
* Key: il tasto o la combinazione (obbligatorio). Distingue maiuscole e minuscole. Usa un nome di tasto di Playwright (`Enter`,
  `ArrowDown`, `F5`, `Backspace`) oppure un tasto di un solo carattere (`a`, `7`). Una combinazione unisce i tasti con `+`:
  `Control+A`, `Shift+Tab`. Funzionano anche i nomi WebDriver che `fill` accetta per `key` (`ENTER`, `ESCAPE`, `ARROW_DOWN`),
  come tasto singolo. Una combinazione accetta solo nomi di Playwright. `press` accetta i nomi di tasto di un elenco fisso; un tasto Playwright raro che non è nell'elenco
  (`ShiftLeft`, `ControlLeft`, `CapsLock`, `ContextMenu`, `NumLock`, `ScrollLock`, `PrintScreen`, `AltGraph`, `F13` e oltre) viene rifiutato con un messaggio che nomina il tasto, e un tasto sbagliato non viene mai inviato. Un tasto di un solo carattere è sempre accettato
* Find: elemento a cui inviare il tasto (maggiori informazioni in find e [Locator](#locators)). Facoltativo. Si usa la prima corrispondenza
* SyncTimeout: tempo massimo, in secondi, di attesa che l'elemento sia pronto. Default: `30`. Senza `find` non interviene alcun elemento, quindi non è un'attesa di elemento: estende soltanto il limite della chiamata del tasto stessa (60 secondi, più `syncTimeout`)

#### Esempio {#example-11}
```yaml
  - type: web
    action:
      press:
        key: Enter
        find:
          locator: PLACEHOLDER
          selector: Search

  - type: web
    action:
      press:
        key: Control+A
```

#### Errori {#errors-4}
* `press: the field 'key' is required (allowed: key, find, syncTimeout): press takes one key or one chord`
* un campo diverso da `key`, `find` e `syncTimeout`: `press: unknown field 'X' (allowed: key, find, syncTimeout)`
* una `key` che non è un testo, un numero YAML come `key: 5`: il messaggio dice di metterla tra virgolette (`key: "5"`)
* un tasto sconosciuto, oppure un testo: `press: invalid 'key' "X": not a key name; press takes one key or one chord`. In una
  combinazione, il messaggio nomina la parte che è fallita
* un tasto WebDriver per cui Playwright non ha un tasto (`NULL`, `CANCEL`, `HELP`): il messaggio dice che non ha un
  equivalente in Playwright

### Select frame {#select-frame}
Seleziona un frame presente nella pagina corrente

#### Input {#input-11}
* Find: frame da cercare (maggiori informazioni nella sezione find)
* SyncTimeout: tempo massimo, in secondi, di attesa che l'elemento sia pronto (presente, visibile, abilitato, fermo, non coperto). Default: `30`

#### Esempio {#example-12}
```yaml
  - type: web
    action:
      selectFrame:
        find:
          selector: "#iframeResult"
          locator: CSS
```

### Select root frame {#select-root-frame}
Seleziona il frame radice presente nella pagina corrente
Un valore booleano da impostare a true per selezionare il frame radice

#### Esempio {#example-13}
```yaml
  - type: web
    action:
      selectRootFrame: true
```

### Next window {#next-window}
Seleziona la finestra (scheda) successiva del browser, tornando alla prima dopo l'ultima; con una sola finestra lo step
resta su di essa
Un valore booleano da impostare a true per selezionare la finestra successiva

#### Esempio {#example-14}
```yaml
  - type: web
    action:
      nextWindow: true
```

### Back {#back}
Torna alla pagina precedente
Un valore booleano da impostare a true per tornare alla pagina precedente

#### Esempio {#example-15}
```yaml
  - type: web
    action:
      historyAction: back
    assertions:
    - result.title ShouldStartWith GitHub
    - result.url ShouldEqual https://github.com/
```

### Forward {#forward}
Avanza alla pagina successiva
Un valore booleano da impostare a true per avanzare alla pagina successiva

#### Esempio {#example-16}
```yaml
  - type: web
    action:
      historyAction: forward
    assertions:
    - result.title ShouldStartWith GitHub
    - result.url ShouldEqual https://github.com/team
```

### Refresh {#refresh}
Aggiorna la pagina
Un valore booleano da impostare a true per aggiornare la pagina

#### Esempio {#example-17}
```yaml
  - type: web
    action:
      historyAction: refresh
    assertions:
    - result.title ShouldStartWith GitHub
    - result.url ShouldEqual https://github.com/team
```

### Confirm popup {#confirm-popup}
Conferma il dialog popup (finestra di conferma e finestra di avviso)

Un dialog aperto da uno step resta aperto in attesa che lo step successivo risponda con `confirmPopup` o `cancelPopup`; finché è
aperto, `result.title` e `result.url` sono vuoti. Qualsiasi altra azione eseguita mentre un dialog è aperto lo chiude e fallisce
con `unexpected alert open`, come faceva WebDriver.
Un valore booleano da impostare a true per confermare il dialog popup

#### Esempio {#example-18}
```yaml
  - type: web
    action:
      confirmPopup: true
```

### Cancel popup {#cancel-popup}
Annulla il dialog popup
Un valore booleano da impostare a true per annullare il dialog popup

#### Esempio {#example-19}
```yaml
  - type: web
    action:
      cancelPopup: true
```

### Execute javascript {#execute-javascript}
Esegue codice javascript. Il comando è il corpo di una funzione: `return` e `arguments[0]`, `arguments[1]`… funzionano
come con WebDriver

#### Input {#input-12}
* Command: codice javascript da eseguire
* Args: array di stringhe che definisce gli argomenti javascript

#### Output {#output-1}
Ciò che lo script restituisce, se restituisce qualcosa. Quando restituisce una promise, lo step attende il suo valore.
* result.value: il valore come testo. Una stringa resta com'è. Qualsiasi altro valore viene scritto come JSON: `42`, `true`,
  `[1,"a"]`, `{"user":{"name":"Ada"}}`, con le chiavi di un oggetto ordinate
* result.valuejson: lo stesso valore, tipizzato, come `result.bodyjson` per l'executor http. Usalo per confrontare numeri o
  per raggiungere l'interno di un oggetto o di una lista: `result.valuejson.user.name`

Il risultato di ogni step web ha una variabile `result.valuejson`. È vuota, a meno che lo step esegua uno script che restituisce
un valore.

Uno script che non restituisce nulla, `undefined` o `null` lascia vuoti entrambi i campi. Alcuni valori non hanno una forma JSON:
* `NaN`, `Infinity` e `-Infinity` vengono letti come queste parole
* una `Date` viene letta come il suo orario UTC in RFC 3339 (`1970-01-01T00:00:00Z`)
* un BigInt, una URL e una RegExp vengono letti come il loro testo
* un elemento DOM, `document` o `window` viene letto come `ref: <Node>`, `ref: <Document>` o `ref: <Window>`

Un valore che fa riferimento a sé stesso, come un oggetto con una proprietà che punta all'oggetto stesso, lascia vuoti entrambi i campi.
Lo stesso accade con una URL che il browser accetta e venom non riesce a leggere, come una con un `%` non seguito da due cifre esadecimali
(`https://a.test/x#50%off`). venom registra un avviso e lo step va a buon fine.

#### Esempio {#example-20}
```yaml
  - type: web
    action:
      execute:
        command: "window.editor.setValue(\"<!DOCTYPE html> <html></html>\"); window.editor.save();"

  - type: web
    action:
      execute:
        command: "return {count: document.querySelectorAll('li').length, first: document.querySelector('li').innerText};"
    assertions:
    - result.valuejson.count ShouldEqual 3
    - result.valuejson.first ShouldEqual Apples
```


Maggiori informazioni sulle azioni (https://github.com/ovh/venom/tree/master/executors/web/types.go)
Per un'azione, puoi fare uno screenshot del browser con il seguente comando: `screenshot: [fileName].png`

## Mock delle chiamate di rete {#mock-network-calls}
Uno step può rispondere da sé alle richieste di rete della pagina, così una suite può testare un front end senza il suo backend. Aggiungi una
lista `mock:` allo step, accanto a `action:`. Ogni voce risponde a tutte le richieste il cui URL corrisponde al suo `url`, e la
richiesta non raggiunge mai la rete. L'host non deve esistere: un mock può servire la pagina stessa, e poi le
chiamate della pagina stessa.

#### Input {#input-13}
Array di input strutturati:
* Url: pattern degli URL a cui rispondere (obbligatorio). `*` corrisponde a qualsiasi carattere tranne `/`, `**` corrisponde a qualsiasi carattere,
  `{a,b}` corrisponde ad `a` o a `b`. Il pattern deve corrispondere all'intero URL: `**/api/users` risponde a
  `https://app.test/api/users`, non a `https://app.test/api/users?page=2`; `**/api/users*` risponde a entrambi. Ogni `{` richiede
  la sua `}`, altrimenti lo step fallisce: scrivi `\{` per una graffa letterale (`\\{` tra doppi apici)
* Status: status HTTP della risposta. Default: `200`
* Headers: header della risposta. Scrivi ogni valore come stringa: `X-Total: "3"`, non `X-Total: 3`
* ContentType: il `Content-Type` della risposta. Default: nessuno
* Body: il corpo della risposta, come testo

Un mock inizia a rispondere prima che venga eseguita l'azione dello step e continua a rispondere negli step successivi dello stesso test
case, in ogni scheda. Quando più mock corrispondono a un URL, risponde il più recente. Il test case successivo parte senza
alcun mock. Uno step può contenere `mock:` senza alcuna azione.

Impostare `mock:` mentre un popup è aperto lo chiude e fa fallire lo step, come qualsiasi altra azione. Imposta il mock in
uno step precedente a quello che apre il popup.

Una richiesta a cui risponde da sé il service worker della pagina non raggiunge mai il mock.

#### Esempio {#example-21}
```yaml
  - type: web
    mock:
    - url: "**/app"
      contentType: text/html
      body: "<title>Users</title><h1>Users</h1>"
    - url: "**/api/users"
      status: 200
      contentType: application/json
      headers:
        X-Total: "1"
      body: '[{"name": "Ada"}]'
    action:
      navigate:
        url: http://app.venom.invalid/app
    assertions:
    - result.title ShouldEqual Users

  - type: web
    action:
      execute:
        command: "return fetch('/api/users').then(r => r.json()).then(users => users[0].name);"
    assertions:
    - result.value ShouldEqual Ada
```

## Diagnostica degli errori {#failure-diagnostics}
Quando uno step fallisce, l'executor web scrive `page.dump.html` nella directory di lavoro: il sorgente del frame corrente.
Salta il dump mentre la pagina sta ancora caricando un nuovo documento. Uno step con `screenshot:` scrive il suo screenshot,
e il dump, una volta che la sua azione è riuscita.

Con la variabile `trace: true`, uno step fallito scrive anche `page.trace.zip` accanto a `page.dump.html`: una trace di Playwright
di quello step, con ogni azione, uno snapshot della pagina prima e dopo di essa, gli screenshot e le chiamate di rete.
Aprila con il trace viewer di Playwright:

```bash
go run github.com/mxschmitt/playwright-go/cmd/playwright@v0.6201.1 show-trace page.trace.zip
```

Il viewer richiede il driver di Playwright; il comando lo scarica se manca, come fa il comando di installazione.
Il successivo step fallito sostituisce entrambi i file. Uno step che va a buon fine non scrive alcuna trace.

La trace contiene tutto ciò che lo step ha visto e digitato, **password incluse**, oltre agli header e ai corpi delle sue chiamate
di rete. Il mascheramento dei secret di venom non vi si applica. venom la scrive leggibile solo dal suo proprietario, come
`page.dump.html`. Non pubblicarla come artefatto di build e non farne il commit. Il tracing
rallenta inoltre ogni step, di circa 20 ms su una pagina semplice: attivalo mentre indaghi su un errore.

## Insidie {#traps}
* Un selettore scritto male supera subito `waitFor` con lo stato `hidden` o `detached`: un elemento che non esiste è
  nascosto e staccato. Controlla prima il selettore con un `find` o con un'attesa `visible`.
* `TESTID` legge `data-testid` per impostazione predefinita. Una pagina che marca i suoi elementi con `data-test` (o con un altro nome) non viene
  trovata finché la suite non imposta `testIdAttribute`. Senza, lo step non trova l'elemento.
* Il dump della pagina (`page.dump.html`) e la trace contengono ciò che `fill` e `press` hanno digitato. Il mascheramento dei segreti di venom
  non li raggiunge. Non digitare un segreto con `press`, un tasto alla volta. Vedi [Diagnostica degli errori](#failure-diagnostics).
* `press` non è digitare. Invia tasti, non un testo: `press: Hello` fallisce. Usa `fill` per inserire un testo.

## Esempio {#example-22}
Un esempio globale

```yaml
name: TestSuite Web
vars:
  web:
    driver: chrome
    width: 1920
    height: 1080
    args:
    - 'browser-test'
    prefs:
      profile.default_content_settings.popups: 0
      profile.default_content_setting_values.notifications: 1
    timeout: 60
    debug: true
testcases:
- name: TestCase Google search
  steps:
  - type: web
    action:
      navigate:
        url: https://www.google.fr
    assertions:
    - result.title ShouldEqual Google
    - result.url ShouldEqual https://www.google.fr
  - type: web
    action:
      find: input[name="q"]
    assertions:
     - result.find ShouldEqual 1
  - type: web
    action:
      fill:
      - find: input[name="q"]
        text: "venom ovh"
  - type: web
    action:
      click:
        find: input[value="Recherche Google"]
        wait: 1
    screenshot: googlesearch.png

```
