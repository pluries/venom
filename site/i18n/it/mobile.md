---
translates: mobile
title: Mobile
description: Come usare l'executor Mobile in una test suite di venom.
fingerprint: b5add67e3549852f87f31f8dbe9f13cb7b412228964257c765cad39d54a9af9c
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# Venom - Executor Mobile {#venom---executor-mobile}

Step per pilotare da una suite di venom un'app Android o iOS nativa o ibrida: installare un'app, trovare elementi, toccare, scrivere, leggere il testo, scorrere, fare uno screenshot, aprire un deep link, passare a una web view, rispondere a un alert di sistema iOS e verificare con le assertion ciò che vedi. Parla con un server [Appium](https://appium.io) 3 che gira sulla stessa
macchina di venom, oppure su un Mac che venom raggiunge tramite un tunnel SSH. Usa il driver UiAutomator2 per un emulatore
Android e il driver XCUITest per un simulatore iOS.

Stato: milestone M1 (#2396), M2 (#2397), M3 (#2398) e M4 (#2399). Android su un emulatore e iOS su un simulatore, sulla stessa macchina o su un Mac raggiunto tramite SSH; web view, coordinate, deep link e alert. Le chiamate iOS sono testate contro un server con risposte predefinite e sono state eseguite su un simulatore il 2026-10-28 e il 2026-10-29 con l'app Impostazioni integrata e con UIKitCatalog di Appium (un'installazione da `.zip`, gli alert, `scroll` con `until`, una web view). Non eseguiti su un simulatore: una cartella `.app`, le coordinate, `openURL`, `hideKeyboard` e l'etichetta `button` dell'alert (vedi "iOS"; le note dettagliate delle esecuzioni non sono pubblicate).

Per verificare che funzioni: esegui i test ermetici (`go test ./executors/mobile/ -count=1`, non serve nessun dispositivo), poi il
test reale contro il tuo Appium e il tuo emulatore (vedi "Esegui i test"). I test ermetici parlano con un finto server con risposte predefinite, quindi
non possono dimostrare che Appium 3 accetti ogni richiesta: solo il test reale lo dimostra.

## Prerequisiti dell'host {#host-prerequisites}

Per preparare un Mac per Android e iOS, vedi "Configura il Mac" più sotto.

- Appium 3 con il driver UiAutomator2 (testato con Appium 3.9.0 e uiautomator2 8.7.2; Appium 2.x non è verificato), avviato in modo che **ascolti solo sull'indirizzo di loopback** (127.0.0.1).
- Un emulatore Android (o un dispositivo) visibile ad Appium.
- Le build-tools dell'SDK Android (`sdkmanager "build-tools;34.0.0"`) quando `mobile.app` indica un `.apk`: UiAutomator2 ha bisogno di `aapt2` per leggerlo. Appium mette in cache la ricerca degli strumenti, quindi riavvia Appium dopo averli installati. `ANDROID_HOME` deve essere impostata nella shell che avvia Appium.
- venom sulla stessa macchina. M1 installa l'app da un percorso di file locale che Appium legge, quindi entrambi devono stare sullo stesso host.

L'executor non richiede nessun flag `--relaxed-security` sul server Appium.

## Variabili {#variables}

Impostale sotto `vars: mobile:` nella suite (oppure in un `.venomrc`, o con `--var`). La sessione si apre una volta per test case,
prima del primo step `mobile`, e si chiude dopo l'ultimo step, anche quando uno step è fallito.

```yaml
vars:
  mobile:
    url: http://127.0.0.1:4723
    platform: android
    device: emulator-5554
    app: ./build/app-debug.apk
```

| Variabile | Obbligatoria | Significato |
|---|---|---|
| `mobile.url` | sì | Indirizzo del server Appium, `http` o `https`, senza credenziali. |
| `mobile.platform` | sì | `android` o `ios`. |
| `mobile.device` | sì | Il dispositivo o l'emulatore. Su Android viene inviato ad Appium sia come `appium:deviceName` sia come `appium:udid`. Su iOS è il nome di un simulatore (inviato solo come `appium:deviceName`) oppure un UDID (inviato anche come `appium:udid`). L'executor non sceglie mai un dispositivo al posto tuo. Usa il seriale adb, come `emulator-5554`; un nome di AVD come `venom_test` viene rifiutato da Appium ("not in the list of connected devices"). |
| `mobile.osVersion` | no | Inviata come `appium:platformVersion`. Il report mostra la versione con cui risponde il server, altrimenti questo valore. |
| `mobile.app` | no | Percorso dell'app da installare quando la sessione si apre: un `.apk` su Android, una cartella `.app` del simulatore o il suo `.zip` su iOS. Un percorso relativo viene risolto rispetto alla cartella della suite. |
| `mobile.appSHA256` | no | SHA-256 atteso dell'`.apk` o dello `.zip`, in esadecimale (maiuscolo o minuscolo). Richiede `mobile.app`. Rifiutato con una cartella `.app`. |
| `mobile.appId` | no | Id del pacchetto (Android) o bundle id (iOS) dell'app. Senza `mobile.app`, la sessione avvia questa app già installata, e su iOS è allora obbligatorio. Con entrambi, è anche il valore predefinito per `launch` e `terminate`. |
| `mobile.ssh.host` | no | Host (nome o indirizzo) del Mac. Quando è impostato, venom raggiunge il server Appium tramite un tunnel SSH verso il loopback dell'host stesso, sulla porta di `mobile.url`, e vi copia l'app. Senza, `mobile.url` viene usato così com'è. |
| `mobile.ssh.user` | con `ssh.host` | Utente SSH. |
| `mobile.ssh.key` | con `ssh.host` | Percorso della chiave privata. Solo autenticazione a chiave, nessuna password. |
| `mobile.ssh.passphrase` | no | Passphrase della chiave. Dichiarala sotto `secrets`. |
| `mobile.ssh.port` | no | Porta SSH, per impostazione predefinita 22. |
| `mobile.ssh.knownhosts` / `mobile.ssh.hostkey` | no | Controllo della chiave dell'host, come nell'executor ssh: un file known_hosts, oppure una sola chiave fissa. Non entrambi. Con nessuno dei due, venom avverte una sola volta per esecuzione e non controlla la chiave dell'host. |
| `mobile.capabilities` | no | Capability Appium aggiuntive, come mappa. Vedi sotto. |
| `mobile.syncTimeout` | no | Secondi di attesa dell'executor per un elemento, per impostazione predefinita `10`. |
| `mobile.sessionTimeout` | no | Secondi concessi per aprire la sessione, per impostazione predefinita `180` su Android e `300` su iOS, dove la prima sessione compila WebDriverAgent. |
| `mobile.idleTimeout` | no | Secondi per cui il server tiene aperta la sessione senza comandi, per impostazione predefinita `120` (inviato come `appium:newCommandTimeout`). |

Un numero negativo in uno qualsiasi dei tre timeout viene rifiutato. Un valore errato viene rifiutato prima dell'invio di qualsiasi richiesta.

### Il file dell'app e il suo SHA-256 {#the-app-file-and-its-sha-256}

Con `mobile.app`, venom legge il file una volta e ne calcola lo SHA-256. Se `mobile.appSHA256` è impostato e differisce, il
test case fallisce **prima che venga aperta qualsiasi sessione**, e il messaggio riporta entrambi gli hash. L'hash calcolato è sempre mostrato nel
report (`appsha256`), così un report dice quale build è stata eseguita. `mobile.appSHA256` senza `mobile.app` viene rifiutato.

Senza un tunnel SSH l'hash copre i byte letti da venom, e Appium installa dal percorso subito dopo: un file sostituito
nel frattempo non verrebbe rilevato. Con `mobile.ssh.*` il file viene copiato sul Mac e l'hash copre i byte inviati
(vedi "Raggiungere il Mac tramite SSH").

### Capability aggiuntive {#extra-capabilities}

`mobile.capabilities` aggiunge capability alla richiesta di sessione. Una capability che l'executor imposta da sé viene rifiutata
prima di qualsiasi richiesta, con un messaggio che indica la variabile a cui appartiene. I nomi vengono confrontati senza il prefisso
`appium:`, anche dentro `appium:options`.

| Capability | Impostata da |
|---|---|
| `platformName` | `mobile.platform` |
| `deviceName`, `udid` | `mobile.device` |
| `platformVersion` | `mobile.osVersion` |
| `app` | `mobile.app` |
| `appPackage`, `bundleId` | `mobile.appId` |
| `automationName` | l'executor (UiAutomator2 su Android, XCUITest su iOS) |
| `newCommandTimeout` | `mobile.idleTimeout` |
| `autoAcceptAlerts`, `autoDismissAlerts` | l'azione alert: è lo step a gestire gli alert, quindi non deve farlo la sessione |

## Input {#input}

```yaml
  - action (mandatory): one of the 17 actions below
  - accessibilityId, text, id, xpath, uiautomator, css, name, predicate, classChain (one of them, see "Locators"): which element the step targets
  - choice (alert): accept (default) or dismiss, in any case
  - button (alert): the label of the button to press, instead of the default one
  - syncTimeout (optional): seconds to wait for the element in this step, overrides mobile.syncTimeout
  - value (fill): the text to type
  - clear (fill): true to clear the field first
  - attribute (read): read this attribute instead of the text
  - state (waitFor): present (default) or absent
  - direction (swipe, scroll): up, down, left or right
  - until (scroll): true to scroll until the element is visible
  - percent (swipe, scroll): the part of the area to cover, above 0 and at most 1, default 0.75
  - x, y (tap, swipe): the point to tap, or where the swipe starts; 0 is valid. Not with a locator
  - toX, toY (swipe): where a swipe from a point ends; give both, and no direction or percent
  - unit (tap, swipe): the unit of x, y, toX, toY: pixels (default, also px) or percent. Percent is of the window rect (0 to 100)
  - appId (launch, terminate): the package id, default mobile.appId
  - path (screenshot): the file to write, relative to the directory venom runs in
  - context (context): native, a web view name starting with WEBVIEW, or empty for the first web view
  - url (openURL): the deep link, scheme://..., at most 2048 bytes, no spaces or control characters
```

`text` è un **locator** (trova un elemento dal suo testo visibile). Il testo che scrivi è `value`.

### Locator {#locators}

Uno step che punta a un elemento indica **esattamente un** locator. Zero o due è un errore prima di qualsiasi richiesta.

| Campo | Trova l'elemento tramite |
|---|---|
| `accessibilityId` | il suo accessibility id (content description) |
| `text` | il suo testo visibile esatto (un `text` di UiSelector); virgolette e backslash al suo interno vengono sottoposti a escape |
| `id` | il suo resource id |
| `xpath` | un XPath sulla gerarchia delle view |
| `uiautomator` | un selettore UiAutomator, inviato così com'è |
| `css` | un selettore CSS, solo dentro una web view |
| `name` | solo iOS: il nome dell'elemento |
| `predicate` | solo iOS: una stringa NSPredicate, inviata così com'è |
| `classChain` | solo iOS: una class chain iOS, inviata così com'è |

`id` e `uiautomator` valgono solo per Android; `name`, `predicate` e `classChain` solo per iOS. Un tipo dell'altra piattaforma
viene rifiutato prima di qualsiasi richiesta, con un messaggio che dice che cosa usare al suo posto. La corrispondenza per iOS è in "iOS".

Un elemento viene atteso: l'executor chiede di nuovo ogni 500 ms finché l'elemento compare o l'attesa è finita (10 s per
impostazione predefinita, oppure il `syncTimeout` dello step, oppure `mobile.syncTimeout`). Se l'elemento non compare mai, l'errore indica il locator e
l'attesa. Un elemento trovato al primo tentativo non aggiunge nessun ritardo.

## Azioni e risultati {#actions-and-results}

| Azione | Richiede | Che cosa fa | Chiavi del risultato che compila |
|---|---|---|---|
| `tap` | un locator, oppure `x` e `y` (con `unit` opzionale) | fa clic sull'elemento, o tocca il punto | `found` (con un locator) |
| `fill` | un locator, `value`, `clear` opzionale | scrive `value` nell'elemento, dopo averlo svuotato quando `clear: true`. Un `value` vuoto non scrive nulla | `found` |
| `clear` | un locator | svuota l'elemento | `found` |
| `read` | un locator, `attribute` opzionale | legge il testo dell'elemento, oppure l'attributo | `found`, `text` (senza attributo) oppure `value` (attributo; un attributo mancante si legge come vuoto) |
| `count` | un locator | conta le corrispondenze in questo momento, senza attendere. Zero è una risposta, non un errore | `count`, `found` (true quando è maggiore di zero) |
| `waitFor` | un locator, `state` opzionale | attende che l'elemento sia presente (predefinito) o assente | `found` |
| `swipe` | `direction`, locator e `percent` opzionali; oppure `x`, `y`, `toX`, `toY` (con `unit` opzionale) per trascinare da un punto a un altro | fa scorrere il dito dentro l'elemento, oppure dentro l'area visibile del dispositivo quando non c'è un locator; quell'area è la finestra ridotta del 10 percento su ogni lato, in modo da non toccare mai i bordi dei gesti di sistema | `found` quando è stato dato un locator |
| `scroll` | `direction`, locator e `percent` opzionali | scorre dentro l'elemento o l'area visibile (la finestra ridotta del 10 percento su ogni lato, in modo da non toccare mai i bordi dei gesti di sistema). Con `until: true` e un locator, scorre l'area visibile finché quell'elemento è sullo schermo, al massimo 10 scorrimenti | `found` quando è stato dato un locator |
| `back` | niente | preme il pulsante Indietro | nessuna |
| `hideKeyboard` | niente | nasconde la tastiera virtuale. Se non ne è mostrata nessuna, non è un errore | nessuna |
| `launch` | `appId` o `mobile.appId` | porta l'app in primo piano (`mobile: activateApp`) | nessuna |
| `terminate` | `appId` o `mobile.appId` | arresta l'app (`mobile: terminateApp`) | nessuna |
| `screenshot` | `path` opzionale | fa uno screenshot. Con `path`, scrive lì il PNG (modo 0600) | `found`, `screenshot` (il percorso) |
| `source` | niente | restituisce la gerarchia delle view come XML | `found`, `source` |
| `context` | `context`: `native`, il nome di una web view (`WEBVIEW_...`), oppure vuoto per la prima web view | passa dall'app nativa a una web view e viceversa. Con una web view, attende (l'attesa dello step) che sia elencata. Un locator `css` richiede una web view | `context` (quello attivo), `contexts` (l'elenco dato dal server; vuoto dopo il passaggio al nativo) |
| `openURL` | `url` | apre un deep link: `url` scheme://..., limitato al pacchetto dell'app quando è noto (`mobile: deepLink`) | nessuna |
| `alert` | `choice` e `button` opzionali | solo iOS: accetta o ignora l'alert di sistema aperto (vedi "iOS") | `text` (il testo dell'alert) |

Un'azione sconosciuta viene rifiutata con l'elenco di quelle supportate. Dove un'azione differisce su iOS, vedi "Differenze rispetto ad Android".

## Output {#output}

```yaml
  - result.text: the text read (read)
  - result.value: the attribute read (read with attribute)
  - result.count: the number of matches (count)
  - result.found: true when the step found its element, or the screen content
  - result.screenshot: the path written (screenshot with path)
  - result.source: the view hierarchy as XML (source)
  - result.timeseconds: duration of the step
  - result.err: the failure of the step, empty when it worked
  - result.platform, result.device, result.osversion: where the test ran
  - result.app, result.appsha256, result.appid: what ran
```

I dati dell'esecuzione (`platform`, `device`, `osversion`, `app`, `appsha256`, `appid`) sono ripetuti in **ogni** risultato, così ogni
test case di un report mostra quale build è stata eseguita su quale dispositivo. Una chiave senza valore è assente dal risultato.

Assertion predefinita: `result.err ShouldBeEmpty`.

### Errori {#errors}

- Un **uso errato** (due locator, un'azione sconosciuta, una `direction` non valida, un'impostazione mancante) è un errore di venom prima dell'invio di qualsiasi
  richiesta. Lo step fallisce con un messaggio che ne indica la causa.
- Un **errore sul server o sul dispositivo** (elemento non trovato, un comando rifiutato, un server irraggiungibile, una chiamata che
  si blocca) finisce in `result.err` come `action <name>: <message>`. Lo step fallisce sull'assertion predefinita e l'esecuzione prosegue.
  Lo stack trace di Appium non viene mai inserito nel risultato.

## Web view e app ibride {#web-views-and-hybrid-apps}

Un'app ibrida mostra pagine web dentro una schermata nativa. L'azione `context` passa dall'una all'altra.

- `context: native` torna all'app nativa.
- `context: WEBVIEW_<package>` passa a quella web view.
- Un `context` vuoto passa alla prima web view elencata dal server.

Per una web view, l'executor attende fino all'attesa dello step (`syncTimeout`, `mobile.syncTimeout`) che sia elencata, così una
pagina che si carica in ritardo viene trovata. Se nessuna compare in tempo, l'errore elenca i context che il server ha effettivamente riportato. Il
risultato contiene `context` (quello attivo) e `contexts` (l'elenco dato dal server).

Dentro una web view la ricerca è gestita da chromedriver, quindi `css` e `xpath` sono i locator da usare. `text`,
`accessibilityId` e `uiautomator` sono strategie native e vengono rifiutate dal server in una web view
(verificato su un dispositivo, 2026-10-28: `accessibilityId` è tornato come errore); che cosa significhi `id` lì (l'id HTML) non è verificato. Dopo
`context: native`, un locator nativo funziona come prima. `css` nel context nativo viene rifiutato con un messaggio prima di qualsiasi ricerca. `source` restituisce l'HTML della pagina in una web view e la gerarchia
XML nel context nativo.

```yaml
      - type: mobile
        action: context
        context: WEBVIEW_com.example.app
      - type: mobile
        action: fill
        css: "#email"
        value: demo-user
      - type: mobile
        action: tap
        css: "button[type=submit]"
      - type: mobile
        action: context
        context: native
      - type: mobile
        action: waitFor
        accessibilityId: welcome_banner
```

### Che cosa serve alla web view {#what-the-web-view-needs}

- **Il debug deve essere attivo.** L'app deve abilitare il debug delle WebView. Senza, il server elenca solo `NATIVE_APP`, e
  il passaggio a una web view fallisce con quell'elenco.
- **Un chromedriver corrispondente.** UiAutomator2 pilota una web view tramite un chromedriver la cui versione corrisponde alla WebView sul
  dispositivo. Una discordanza fa fallire il passaggio sul server. Imposta `appium:chromedriverExecutable` (un percorso sull'host
  di Appium) tramite `mobile.capabilities`, oppure nella configurazione del server stesso:

  ```yaml
  vars:
    mobile:
      capabilities:
        appium:chromedriverExecutable: /opt/chromedrivers/chromedriver-114
  ```
- **Il download automatico è una scelta di chi gestisce il server.** Il server Appium può scaricare da solo un chromedriver
  (`chromedriver_autodownload`). Serve però un'opzione che chi gestisce il server imposta sul server Appium. venom
  non la invia mai e non ha mai bisogno di `--relaxed-security` né di `--allow-insecure`.

## App senza etichette e coordinate {#unlabelled-apps-and-coordinates}

Un'app che non pubblica etichette di accessibilità (alcune build release di Flutter, giochi, view disegnate a mano) si può raggiungere
solo con le coordinate. Se l'app ha qualche etichetta, id o testo, preferisci un locator: una coordinata si rompe quando cambiano il layout,
la dimensione dello schermo o il font.

`tap` e `swipe` accettano un punto al posto di un locator:

| Campo | Significato |
|---|---|
| `x`, `y` | il punto da toccare, o da dove parte lo swipe. 0 è un valore valido |
| `toX`, `toY` | dove finisce lo swipe. Indicali entrambi, e niente `direction` né `percent` |
| `unit` | `pixels` (predefinito, anche `px`) oppure `percent` |

Con `unit: percent` il punto è una quota, da 0 a 100, del rettangolo della finestra: `x: 50` e `y: 50` è il centro. Su una
finestra di 1080x2400 questo invia il punto 540, 1200. I pixel vengono inviati così come sono e il server li limita. Un punto con un
locator, `x` senza `y`, un valore negativo, NaN o un infinito, un valore in pixel sopra 10 milioni, o una percentuale sopra 100
viene rifiutato prima di qualsiasi richiesta.

Le coordinate in percentuale e `swipe` o `scroll` senza un locator leggono la dimensione della finestra dal server. Dentro una web view quella
dimensione può essere nelle unità del browser, non nei pixel del dispositivo: torna al nativo (`context: native`) prima di usare
coordinate in percentuale o uno `swipe` e `scroll` senza locator. Non verificato su un dispositivo: la prova manuale ha usato coordinate in pixel e in percentuale
solo nel context nativo.

```yaml
      - type: mobile
        action: tap
        x: 50
        y: 50
        unit: percent
      - type: mobile
        action: swipe
        x: 540
        y: 1800
        toX: 540
        toY: 600
```

## Deep link {#deep-links}

`openURL` apre un link nell'app, tramite `mobile: deepLink` di Appium:

```yaml
      - type: mobile
        action: openURL
        url: myapp://orders/42
```

- `url` deve avere la forma `scheme://...`, senza spazi né caratteri di controllo, al massimo 2048 byte. Qualsiasi altra cosa viene
  rifiutata prima di qualsiasi richiesta, e il messaggio non ripete il link (può contenere un token).
- Il link è limitato al pacchetto dell'app (l'`appId` dello step, altrimenti `mobile.appId`) quando ne è noto uno.
- Gli schemi personalizzati sono accettati: sono lo scopo di un deep link. Il link viene passato a `mobile: deepLink` di Appium, che
  lo esegue tramite `am start`; venom non costruisce nessun comando di shell. Nello stack testato (Appium 3.9.0, uiautomator2 8.7.2,
  emulatore Android 14) cinque link con metacaratteri di shell sono arrivati al server, sono falliti perché nulla li gestiva,
  e non hanno creato nessun file sul dispositivo. Un Appium o un driver più vecchio potrebbe non fare l'escape: tieni i deep link provenienti da input non attendibile
  fuori dalle suite.

## Limiti {#limits}

| Limite | Valore |
|---|---|
| Una chiamata al server | 60 s |
| Apertura della sessione | 180 s (`mobile.sessionTimeout`) |
| Sessione inattiva sul server | 120 s (`mobile.idleTimeout`) |
| Pausa tra due tentativi di trovare un elemento | 500 ms |
| Attesa predefinita per un elemento | 10 s (`mobile.syncTimeout`, `syncTimeout`) |
| Scorrimenti di `scroll` con `until` | 10 |
| Uno screenshot decodificato | 12 MiB |
| Chiusura della sessione | 15 s |
| Ciascuna delle due richieste di diagnostica | 15 s |

Un reindirizzamento dal server non viene seguito. Il `timeout:` proprio dello step vale comunque in aggiunta a questi limiti.

## Diagnostica su uno step che fallisce {#diagnostics-on-a-failing-step}

Quando uno step fallisce sul server o sul dispositivo, l'executor salva due file nella directory in cui gira venom:

- `screen.png`: uno screenshot del dispositivo;
- `screen.dump.xml`: la gerarchia delle view.

**Attenzione: entrambi i file mostrano ciò che c'è sullo schermo, compreso un secret che hai scritto o che l'app visualizza.** Non sono
coperti dall'oscuramento dei secret dei report di venom. Vengono scritti con modo 0600 nella cartella di lavoro. Non pubblicarli
come artefatti di build. Il successivo step che fallisce li sostituisce; uno step che passa non scrive e non cancella nulla. Il
mancato recupero di un file è un avviso nel log e non nasconde l'errore dello step stesso. Un errore di uso errato, o una sessione
che il server non ha più, non scrive nulla.

Due esecuzioni di venom avviate nella **stessa directory** si sovrascrivono a vicenda i file. Eseguile da directory diverse.

## Raggiungere il Mac tramite SSH {#reaching-the-mac-through-ssh}

Tieni Appium in ascolto sul loopback del Mac (`127.0.0.1:4723`); non ha una password propria. Imposta `mobile.ssh.host`, `mobile.ssh.user` e `mobile.ssh.key` e lascia `mobile.url` come indirizzo visto dal Mac (`http://127.0.0.1:4723`): venom apre il tunnel all'inizio del test case e lo chiude alla fine. Il tunnel ascolta solo su `127.0.0.1` e si collega sempre al loopback del Mac.

- **Chiave dell'host:** imposta `mobile.ssh.knownhosts` oppure `mobile.ssh.hostkey`. Una chiave che non corrisponde rifiuta la connessione prima di qualsiasi richiesta.
- **Il file dell'app:** con un tunnel, `mobile.app` viene copiato in `/tmp/venom-mobile-<random>/app.apk` (`app.zip` su iOS) sul Mac, installato da lì e rimosso quando la sessione termina. Nessun valore della suite compare in un comando eseguito sul Mac. `mobile.appSHA256` viene verificato sui byte inviati; una discordanza fa fallire prima di qualsiasi sessione e rimuove la copia. Un'esecuzione interrotta con kill può lasciare la cartella: cerca `/tmp/venom-mobile-*`.
- **Limiti:** un URL `https://` attraverso il tunnel viene verificato rispetto a `127.0.0.1`, quindi usa `http://` dentro il tunnel. La connessione e l'handshake SSH sono limitati da `mobile.sessionTimeout` (180 s per impostazione predefinita), quindi un host che non risponde fa fallire il Setup invece di bloccarlo.
- **http semplice senza tunnel:** un URL `http://` verso un'altra macchina stampa un avviso per esecuzione; un URL di loopback non ne stampa nessuno.

## Note di sicurezza {#security-notes}

- **Appium deve ascoltare solo sul loopback.** Un server Appium esposto in rete permette a chiunque lo raggiunga di controllare
  l'emulatore e la macchina. In M1, venom si connette ad Appium sullo stesso host.
- L'app viene installata da un **file locale**, quindi in M1 Appium e venom devono stare sullo stesso host.
- **Gli screenshot, il dump e `result.source` mostrano ciò che c'è sullo schermo, compreso un secret che hai scritto.** NON sono
  coperti dall'oscuramento dei secret dei report di venom, lo stesso limite della trace dell'executor web. Tieni `screen.png` e
  `screen.dump.xml` fuori dagli artefatti pubblicati e dalle cartelle di build condivise.
- `fill` non ripete mai il testo scritto in un risultato né nel log delle chiamate con `-v`. Con `-v`, ogni chiamata al server viene registrata (metodo, percorso, durata; mai un body).
- Vengono usati solo gli endpoint di Appium 3 e l'executor non richiede `--relaxed-security`.
- **Il contenuto di pagina di una web view e il testo scritto al suo interno possono comparire negli screenshot e in `result.source`**, come in qualsiasi
  schermata nativa, e l'oscuramento dei secret non li copre. Il testo di pagina letto con `read` è un valore del risultato come gli altri.
- **L'URL di un deep link viene passato a `mobile: deepLink` di Appium, che lo esegue tramite `am start`.** venom stesso non
  costruisce mai un comando di shell; il link viene controllato nella forma e nella lunghezza, e limitato al pacchetto dell'app quando ne è noto uno.
  Appium ha fatto l'escape dei metacaratteri di shell nello stack testato (Appium 3.9.0, uiautomator2 8.7.2, emulatore Android 14 arm64,
  misurato il 2026-10-28). Un Appium o un driver più vecchio potrebbe non farlo, quindi tieni i deep link provenienti da input non attendibile fuori dalle suite.
- **Il chromedriver gira sull'host di Appium**, e quale binario sia resta una scelta di chi gestisce il server. venom non invia nessuna
  opzione che attivi `--relaxed-security` o `--allow-insecure`, e il server non ha bisogno né dell'una né dell'altra.
- **Il testo di un alert iOS** va in `result.text` e da nessun'altra parte. Non viene scritto in una riga di log né in un errore, ma
  è un valore del risultato come gli altri, quindi un secret al suo interno viene oscurato solo se è un secret dichiarato.
- **Una cartella `.app` viene copiata sul Mac come `app.zip`**, in una cartella con nome venom (`/tmp/venom-mobile-<random>`, modo 0700),
  senza nessun valore della suite in alcun comando eseguito lì. venom rifiuta una `.app` che contiene un collegamento simbolico, e non calcola nessun hash di
  una cartella: la sua integrità non viene verificata. Indica uno `.zip` e `mobile.appSHA256` quando serve.
- **Il driver XCUITest e WebDriverAgent girano sul Mac** e sono un'installazione di chi gestisce il server. venom non li scarica né li compila;
  la prima sessione compila lì WebDriverAgent.

## iOS {#ios}

Con `mobile.platform: ios` la stessa suite pilota un'app su un **simulatore** iOS tramite il driver XCUITest. Un iPhone
reale non è coperto. Le chiamate iOS sono implementate a partire dal comportamento documentato del driver XCUITest e testate contro un
server con risposte predefinite. Sono state eseguite su un simulatore (iPhone 17, iOS 26.5, Appium 3.9.0, driver XCUITest 12.22.3), in due esecuzioni:

- 2026-10-28, l'app Impostazioni integrata avviata tramite bundle id, suite `examples/handrun-settings.yml`, 3 test
  case su 3 superati: sessione aperta, `waitFor`, `tap`, swipe per direzione, `terminate`, `launch`, i locator `text`, `name`,
  `predicate` e `classChain`, `count`, e `alert` senza nessun alert aperto.
- 2026-10-29, UIKitCatalog v4.0.5 di Appium fornito come `UIKitCatalog.zip` con `mobile.appSHA256`, tramite il tunnel SSH,
  suite `examples/handrun-uikitcatalog.yml`, stato finale PASS: l'installazione da `.zip`, `alert` accept (il testo dell'alert
  è tornato in `result.text`) e dismiss, `scroll` con `until`, e una web view (`context` ha riportato un context `WEBVIEW`,
  `css: body` ha contato più di 0, `context: native` è tornato indietro). La web view iOS non ha richiesto nessun chromedriver.

Non eseguiti su un simulatore: una cartella `.app`, le coordinate, `openURL`, `hideKeyboard`, l'etichetta `button` dell'alert, le coordinate
in percentuale. Le note dettagliate delle esecuzioni non sono pubblicate.
Le righe di una lista simile a Impostazioni non sono `XCUIElementTypeCell` su iOS 26: scrivi una class chain come `**/*[...]`, non con un tipo cell.
La prima sessione ha richiesto minuti (compilazione di WebDriverAgent), ed è per questo che il timeout di sessione predefinito è 300 s.

### Prerequisiti sul Mac {#prerequisites-on-the-mac}

- Xcode e i suoi strumenti da riga di comando, e almeno un simulatore.
- Appium 3 con il driver XCUITest: `appium driver install xcuitest`. Tieni Appium sull'indirizzo di loopback, come per Android.
- La prima sessione compila WebDriverAgent e richiede minuti. Per questo `mobile.sessionTimeout` vale 300 s per impostazione predefinita su iOS.
- Un simulatore non richiede la firma del codice.
- Su un Mac Apple Silicon l'app deve avere uno slice arm64, e anche le immagini dell'emulatore Android lì sono arm64. Una `.app` iOS
  con solo slice i386 o x86_64 viene rifiutata da Appium ("does not support the arm64 Simulator architecture"); un
  APK con solo librerie `armeabi-v7a` e `x86` fallisce con `INSTALL_FAILED_NO_MATCHING_ABIS` (serve `arm64-v8a`, oppure
  nessuna libreria nativa). TheApp v1.10.0 (2019) di Cloud Grey è fallita in entrambi i modi sul Mac del maker.
- Un esempio pronto per provare iOS: UIKitCatalog di Appium (`UIKitCatalog.zip`, una build per simulatore con slice arm64 e x86_64,
  bundle id `com.example.apple-samplecode.UICatalog`), dalla pagina delle release di
  <https://github.com/appium/ios-uicatalog>. La suite `examples/handrun-uikitcatalog.yml` gira su di essa.

### Impostazioni che cambiano su iOS {#settings-that-change-on-ios}

| Impostazione | Su iOS |
|---|---|
| `mobile.device` | Il nome di un simulatore (`iPhone 16`), oppure il suo UDID. Un UDID viene inviato anche come `appium:udid`; un nome solo come `appium:deviceName`. |
| `mobile.osVersion` | Inviata come `appium:platformVersion`, come su Android. |
| `mobile.app` | Una cartella `.app` del simulatore, oppure il suo `.zip`. Un **`.ipa` è una build per dispositivo e non si installa su un simulatore**: viene rifiutato prima di qualsiasi richiesta. Compila invece una `.app` per simulatore. |
| `mobile.appSHA256` | Solo per uno `.zip`. Una cartella `.app` non ha un singolo file da calcolare, quindi l'impostazione viene rifiutata con una cartella. |
| `mobile.appId` | Il bundle id. Senza `mobile.app`, l'app installata viene avviata dal suo bundle id e `mobile.appId` è allora obbligatorio. |
| `mobile.sessionTimeout` | 300 s per impostazione predefinita. |

Tramite SSH, una cartella `.app` viene compressa in zip sulla macchina che esegue venom e copiata sul Mac come `app.zip`; uno `.zip` viene copiato
così com'è. Vedi "Raggiungere il Mac tramite SSH".

### Locator su iOS {#locators-on-ios}

| Campo | Trova l'elemento tramite |
|---|---|
| `accessibilityId` | il suo accessibility id |
| `text` | la sua etichetta: inviata come predicato `label == "<text>"`, con virgolette, backslash e a capo nel testo sottoposti a escape |
| `name` | il suo nome |
| `predicate` | una stringa NSPredicate, inviata così com'è |
| `classChain` | una class chain iOS, inviata così com'è |
| `xpath` | un XPath sulla gerarchia delle view |
| `css` | un selettore CSS, solo dentro una web view |

`id` e `uiautomator` vengono rifiutati su iOS; `name`, `predicate` e `classChain` vengono rifiutati su Android. Come su Android, uno
step indica esattamente un locator.

### Differenze rispetto ad Android {#differences-from-android}

| Azione | Su iOS |
|---|---|
| `tap`, `fill`, `clear`, `read`, `count`, `waitFor`, `screenshot`, `source`, `context` | Gli stessi comandi che su Android. |
| `tap` e `swipe` con `x`, `y` | Il punto è in punti, non in pixel. `tap` usa `mobile: tap`, `swipe` usa `mobile: dragFromToForDuration` per mezzo secondo. `unit: percent` è relativo al rettangolo della finestra, che iOS riporta in punti. |
| `swipe`, `scroll` con `direction` | `mobile: swipe` e `mobile: scroll`, sull'elemento o su tutta l'app. `percent` viene rifiutato: XCUITest non ha un argomento simile. |
| `scroll` con `until` | Ripete `mobile: scroll` fino a 10 volte. Il server non dice quando la lista finisce, quindi il ciclo si ferma sull'elemento o sul limite. |
| `back` | Rifiutato: iOS non ha un tasto Indietro. Tocca il controllo di ritorno dell'app stessa. |
| `hideKeyboard` | `mobile: hideKeyboard`. Una tastiera non mostrata non è un errore. |
| `launch`, `terminate` | `mobile: activateApp` e `mobile: terminateApp`, indicando l'app con `bundleId` (`appId` o `mobile.appId`). |
| `openURL` | `mobile: deepLink` con `url`, e il `bundleId` quando è noto. |
| `alert` | Solo iOS. Su Android viene rifiutata. |

### Alert {#alerts}

Un'app iOS può far comparire un alert di sistema, per esempio una richiesta di permesso. L'azione `alert` ne legge il testo e poi lo accetta
o lo ignora:

```yaml
      - type: mobile
        action: tap
        accessibilityId: enable_location
      - type: mobile
        action: alert
        choice: accept
        assertions:
          - result.err ShouldBeEmpty
          - result.text ShouldContainSubstring Location
      - type: mobile
        action: alert
        choice: dismiss
        button: Don't Allow
```

- `choice` è `accept` (predefinito) oppure `dismiss`. Qualsiasi altro valore viene rifiutato prima di qualsiasi richiesta.
- Senza `button`, lo step risponde con il semplice comando di accettazione o di rifiuto. Con `button`, preme il pulsante con
  quell'etichetta (`mobile: alert`).
- `result.text` contiene il testo dell'alert. **Un alert può mostrare un secret** (un codice, un nome): il testo non viene mai scritto in una riga
  di log né in un messaggio di errore, e l'oscuramento dei secret dei report di venom copre solo i secret dichiarati.
- Se non c'è nessun alert aperto, lo step fallisce con `no alert open` in `result.err`.
- La sessione non deve rispondere da sola agli alert: `autoAcceptAlerts` e `autoDismissAlerts` in `mobile.capabilities` vengono
  rifiutati, perché è lo step `alert` a gestirli.
- Un deep link su un simulatore può far comparire un alert di conferma di sistema ("Open in ...?"). Fai seguire a un `openURL` uno step `alert`
  quando la tua app si apre in quel modo, altrimenti lo step successivo troverà l'alert di mezzo.

## Non ancora {#not-yet}

Previsto, rifiutato o assente oggi:

- Un vero dispositivo iOS: sono nell'ambito solo i simulatori.
- Flutter sul simulatore iOS è solo una build di debug: Flutter non produce nessuna app release per il simulatore.
- App senza uno slice arm64, su un Mac Apple Silicon.
- Accesso con password tramite SSH: solo autenticazione a chiave.
- Un chromedriver scaricato al posto tuo: per una web view Android è chi gestisce il server a fornirne uno corrispondente.
- Non ancora eseguiti su un simulatore (vedi "iOS"; le note dettagliate delle esecuzioni non sono pubblicate): l'installazione di una cartella `.app` (è girato solo
  uno `.zip`), le coordinate `x`/`y` e in percentuale, `openURL`, `hideKeyboard`, e l'etichetta `button` dell'alert. La suite
  `examples/handrun-ios.yml` è scritta per l'installazione da cartella `.app`.

## Esempio {#example}

Ogni step qui sotto è una forma che i test di questo executor eseguono.

```yaml
name: Mobile smoke
vars:
  mobile:
    url: http://127.0.0.1:4723
    platform: android
    device: emulator-5554
    osVersion: "14"
    app: ./build/app-debug.apk
    appSHA256: <sha-256 of the build, in hex>
    appId: com.example.app
    syncTimeout: 5

testcases:
  - name: the home screen is usable
    steps:
      - type: mobile
        action: waitFor
        accessibilityId: login_button
        syncTimeout: 20
      - type: mobile
        action: count
        id: com.example.app:id/item
        assertions:
          - result.count ShouldBeGreaterThan 0
      - type: mobile
        action: fill
        id: com.example.app:id/username
        value: demo-user
        clear: true
      - type: mobile
        action: hideKeyboard
      - type: mobile
        action: tap
        accessibilityId: login_button
      - type: mobile
        action: read
        id: com.example.app:id/title
        assertions:
          - result.err ShouldBeEmpty
          - result.text ShouldEqual Welcome
      - type: mobile
        action: scroll
        direction: down
        until: true
        text: Settings
      - type: mobile
        action: screenshot
        path: home.png
      - type: mobile
        action: back
      - type: mobile
        action: terminate
      - type: mobile
        action: launch
```

## Configura il Mac {#set-up-the-mac}

Usalo quando venom gira su un'altra macchina (Windows, Linux) e l'emulatore e il simulatore girano su un Mac. Tutto ciò che segue
si installa sul Mac. venom non ne installa nulla.

1. **Appium 3** con entrambi i driver: `appium driver install uiautomator2` e `appium driver install xcuitest`. Avvialo in modo
   che ascolti solo sull'indirizzo di loopback: `appium --address 127.0.0.1 --port 4723`. Imposta `ANDROID_HOME` nella
   shell che lo avvia.
2. **Android:** l'SDK Android (platform-tools, `build-tools;34.0.0`) e un'immagine di emulatore **arm64** su un Mac Apple
   Silicon. Avvia l'emulatore prima dell'esecuzione.
3. **iOS:** Xcode, i suoi strumenti da riga di comando e almeno un simulatore. Avvialo, oppure lascia che lo avvii Appium. La prima sessione
   compila WebDriverAgent e richiede minuti.
4. **Web view Android:** un chromedriver che corrisponda alla WebView sull'emulatore, e il suo percorso in
   `appium:chromedriverExecutable` (vedi "Che cosa serve alla web view"). Le web view iOS non ne richiedono nessuno.
5. **SSH:** una chiave che il Mac accetta per l'utente che indichi in `mobile.ssh.user`. L'accesso con password non è supportato.

Insidie incontrate su un Mac reale:

- **La prima esecuzione di uno strumento può bloccarsi tramite SSH.** Gatekeeper di macOS può chiedere una conferma che nessuno vede in una
  sessione SSH, e il binario resta in attesa. Esegui `flutter --version` (o lo strumento in questione) una volta in un Terminale sul Mac stesso
  e rispondi alla richiesta. Anche gli strumenti installati con un cask di Homebrew possono portare il flag di quarantena. Rimuoverlo con
  `xattr -dr com.apple.quarantine <folder>` è una tua decisione: fallo solo per una cartella di cui ti fidi.
- **Le app richiedono uno slice arm64** su un Mac Apple Silicon (vedi "Prerequisiti sul Mac").
- **Flutter compila solo app di debug per il simulatore iOS.** Una build release di Flutter è possibile solo su Android.
- **CocoaPods richiede un locale UTF-8** (`export LANG=en_US.UTF-8 LC_ALL=en_US.UTF-8`), per esempio quando gira da uno
  script avviato tramite SSH.

## App di esempio {#sample-apps}

App pubbliche usate nelle esecuzioni del maker. Ciascuna è pronta per arm64.

| App | Piattaforma | Dove trovarla |
|---|---|---|
| ApiDemos | Android | `ApiDemos-debug.apk` di Appium, dalle release di <https://github.com/appium/android-apidemos> |
| UIKitCatalog v4.0.5 | simulatore iOS | `UIKitCatalog.zip`, dalle release di <https://github.com/appium/ios-uicatalog> |
| App contatore Flutter | entrambe | `flutter create`, compilata da `examples/build-sample-apps.sh` (APK release Android, app di debug per simulatore iOS) |
| App React Native | entrambe | `react-native init`, compilata dallo stesso script (APK release Android, app Release per simulatore iOS) |

Lo script gira sul Mac. Installa l'SDK Flutter e CocoaPods se mancano, compila le quattro app dai template
propri dei framework e le elenca alla fine. Esegui prima a mano una volta `flutter --version` (vedi "Configura il Mac").

Le suite sono in `examples/`: `examples/flutter.yml` e `examples/rn.yml` girano senza modifiche su entrambe le piattaforme, con il file di variabili di
ciascuna piattaforma (`examples/android.vars.example.yml`, `examples/ios.vars.example.yml`: compila i valori `<...>`); `examples/failing.yml` fallisce
di proposito per mostrare i file di diagnostica. Le note dettagliate delle esecuzioni non sono pubblicate.

## Esegui i test {#run-the-tests}

Test ermetici (nessun dispositivo, nessuna rete; fanno parte di `go test ./...`):

```bash
go test ./executors/mobile/ -count=1
```

Test reale, dietro il build tag `mobile`. Richiede Appium 3 con UiAutomator2 sul loopback e un emulatore, e legge
le sue impostazioni dall'ambiente. Viene saltato con un messaggio quando una è mancante.

```bash
export VENOM_MOBILE_URL=http://127.0.0.1:4723
export VENOM_MOBILE_DEVICE=emulator-5554
export VENOM_MOBILE_APP=/path/to/sample.apk        # or VENOM_MOBILE_APPID=<package id of an installed app>
export VENOM_MOBILE_LOCATOR_KIND=accessibilityId   # accessibilityId, text, id, xpath or uiautomator
export VENOM_MOBILE_LOCATOR_VALUE=<an element on the first screen>
export VENOM_MOBILE_EXPECT_TEXT=<its text>         # optional
export VENOM_MOBILE_OSVERSION=14                   # optional
go test -tags mobile ./executors/mobile/ -run Real -v -count=1
```

Apre una sessione, trova e legge l'elemento, verifica che uno step che passa non lasci diagnostica, fa fallire uno step di
proposito e verifica che compaiano `screen.png` e `screen.dump.xml`, poi chiude la sessione.

Altri due test reali condividono le variabili di sessione qui sopra (`VENOM_MOBILE_URL`, `_DEVICE`, `_APP` oppure `_APPID`):

| Test | Variabili aggiuntive | Che cosa verifica |
|---|---|---|
| `TestReal_CoordinatesAndDeepLink` | nessuna | swipe in percentuale e in pixel e un tap al 50 % / 50 % riescono; `openURL venomtest://nothing/handles/this` torna come `result.err` non vuoto |
| `TestReal_IOSSession` | `VENOM_MOBILE_PLATFORM=ios`, `VENOM_MOBILE_DEVICE` il nome di un simulatore, `VENOM_MOBILE_APPID` un bundle id (oppure `VENOM_MOBILE_APP`), `VENOM_MOBILE_IOS_LOCATOR` (accessibility id nella prima schermata, `General` per impostazione predefinita per l'app Impostazioni) | `waitFor`, swipe verso l'alto, `terminate`, `launch`, e `alert` senza nessun alert aperto risponde `no alert open`; saltato a meno che `VENOM_MOBILE_PLATFORM` sia `ios` |
| `TestReal_WebViewContext` | `VENOM_MOBILE_CHROMEDRIVER` (percorso di un chromedriver corrispondente sull'host di Appium) e `VENOM_MOBILE_WEBVIEW_LOCATOR` (accessibility id che apre una schermata con web view) | `context` raggiunge un `WEBVIEW...`, `css: body` conta più di 0, `context: native` restituisce `NATIVE_APP`; saltato a meno che entrambe siano impostate |
