---
translates: web
title: Web
description: Come usare l'executor Web in una test suite di venom.
fingerprint: 841e8ee00f470e50ad60416a4c8b072458dd7b33e432bb145851d952a4649966
drafted_by: claude
drafted_on: 2026-11-16
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

Un'azione su un elemento (`click`, `fill`, `select`, `uploadFile`, `selectFrame`) attende che l'elemento sia pronto,
fino al suo `syncTimeout` o a 30 secondi. `find` non attende mai: conta gli elementi presenti in quel momento, quindi
`result.find ShouldEqual 0` resta veloce. Dopo un'azione che può caricare una pagina (`click`, `fill`, `select`,
`uploadFile`, `execute`, la risposta a un popup), lo step attende che la pagina finisca di caricarsi prima di leggere
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
* Locator: locator da usare per cercare l'elemento (CSS o XPATH)

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

### Click {#click}
Fa clic su un elemento a partire da un selettore

#### Input {#input-2}
* Find: elemento da cercare (maggiori informazioni nella sezione find)
* Wait: tempo di attesa dopo il clic (in secondi)
* SyncTimeout: tempo massimo, in secondi, di attesa che l'elemento sia pronto (presente, visibile, abilitato, fermo, non coperto). Default: `30`

#### Esempio {#example-2}
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

#### Input {#input-3}
Array di input strutturati:
* Find: elemento da cercare (maggiori informazioni nella sezione find)
* Text: aggiorna il valore dell'elemento con un testo
* Key: aggiorna il valore dell'elemento con un tasto
* SyncTimeout: tempo massimo, in secondi, di attesa che l'elemento sia pronto (presente, visibile, abilitato, fermo, non coperto). Default: `30`

#### Esempio {#example-3}
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

#### Input {#input-4}
* Find: elemento da cercare (maggiori informazioni nella sezione find)
* Text: opzione da selezionare
* Wait: tempo di attesa dopo il clic (in secondi)
* SyncTimeout: tempo massimo, in secondi, di attesa che l'elemento sia pronto (presente, visibile, abilitato, fermo, non coperto). Default: `30`

#### Esempio {#example-4}
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

#### Input {#input-5}
* Find: elemento da cercare (maggiori informazioni nella sezione find)
* Files: array di stringhe che definisce i file da caricare
* Wait: tempo di attesa dopo il clic (in secondi)
* SyncTimeout: tempo massimo, in secondi, di attesa che l'elemento sia pronto (presente, visibile, abilitato, fermo, non coperto). Default: `30`

#### Esempio {#example-5}
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

#### Esempio {#example-6}
```yaml
  - type: web
    action:
      wait: 1
```

### Select frame {#select-frame}
Seleziona un frame presente nella pagina corrente

#### Input {#input-6}
* Find: frame da cercare (maggiori informazioni nella sezione find)
* SyncTimeout: tempo massimo, in secondi, di attesa che l'elemento sia pronto (presente, visibile, abilitato, fermo, non coperto). Default: `30`

#### Esempio {#example-7}
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

#### Esempio {#example-8}
```yaml
  - type: web
    action:
      selectRootFrame: true
```

### Next window {#next-window}
Seleziona la finestra (scheda) successiva del browser, tornando alla prima dopo l'ultima; con una sola finestra lo step
resta su di essa
Un valore booleano da impostare a true per selezionare la finestra successiva

#### Esempio {#example-9}
```yaml
  - type: web
    action:
      nextWindow: true
```

### Back {#back}
Torna alla pagina precedente
Un valore booleano da impostare a true per tornare alla pagina precedente

#### Esempio {#example-10}
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

#### Esempio {#example-11}
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

#### Esempio {#example-12}
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

#### Esempio {#example-13}
```yaml
  - type: web
    action:
      confirmPopup: true
```

### Cancel popup {#cancel-popup}
Annulla il dialog popup
Un valore booleano da impostare a true per annullare il dialog popup

#### Esempio {#example-14}
```yaml
  - type: web
    action:
      cancelPopup: true
```

### Execute javascript {#execute-javascript}
Esegue codice javascript. Il comando è il corpo di una funzione: `return` e `arguments[0]`, `arguments[1]`… funzionano
come con WebDriver

#### Input {#input-7}
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

#### Esempio {#example-15}
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

#### Input {#input-8}
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

#### Esempio {#example-16}
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

## Esempio {#example-17}
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
