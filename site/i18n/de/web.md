---
translates: web
title: Web
description: Den Web-executor in einer venom-Test-Suite verwenden.
fingerprint: d700bc6151e18429c9339fb06ffe6bc6b56b451abd92c6c1328ee3c6cbec08f8
drafted_by: claude
drafted_on: 2026-11-19
status: machine-drafted
---
# Venom - Executor Web {#venom---executor-web}

In einer Webanwendung navigieren

Anwendungsfall: Sie haben eine Webanwendung und möchten bestimmte Verhaltensweisen prüfen.

Der Web-executor erlaubt es Ihnen, in Ihrer Webanwendung zu navigieren und Aktionen auszuführen.

## HTTPS-Zertifikate {#https-certificates}

Standardmäßig ignoriert der Web-executor Fehler bei HTTPS-Zertifikaten (der Browser-Kontext wird mit `IgnoreHttpsErrors` erzeugt).
Ein Web-Schritt gelingt auch gegen eine Site mit abgelaufenem oder selbstsigniertem Zertifikat oder mit falschem Host. Verlassen Sie sich daher nicht darauf, um
Zertifikate zu prüfen. Damit der Browser ein ungültiges Zertifikat ablehnt, setzen Sie die Variable `verifyTls` auf `true`; der Schritt
schlägt dann fehl. Der Web-executor kann einer privaten Zertifizierungsstelle nicht vertrauen: Playwright bietet dafür keine Option für einen Browser-Kontext.

## Browser {#browsers}

Der Web-executor steuert einen Browser über [Playwright](https://playwright.dev) (Go-Binding
`github.com/mxschmitt/playwright-go`). Playwright verwaltet seine eigenen Browser-Builds: Es muss weder `chromedriver` noch `geckodriver`
installiert werden, und es gibt keine Version, die zum Browser passen müsste.

Installieren Sie den Playwright-Treiber und die Browser, die Ihre Suites verwenden, einmal pro Rechner, zusammen mit ihren Betriebssystempaketen
(`--with-deps` braucht unter Linux `sudo`):

```bash
go run github.com/mxschmitt/playwright-go/cmd/playwright@v0.6201.1 install --with-deps chromium   # or firefox, webkit
```

venom lädt von sich aus nie einen Browser herunter: Eine Suite, die einen nicht installierten Browser braucht, schlägt beim Setup fehl, und
die Fehlermeldung nennt diesen Befehl.

### Argumente {#arguments}
* Chromium: [Liste der Argumente](https://peter.sh/experiments/chromium-command-line-switches/). Ein Argument ohne
  führenden Bindestrich erhält `--`, wie es chromedriver tat.
* Firefox: [Liste der Argumente](https://wiki.mozilla.org/Firefox/CommandLineOptions),
  [Liste der Einstellungen](https://searchfox.org/mozilla-central/source/modules/libpref/init/all.js)

## Variablen {#variables}

Sie können Parameter definieren, um den Browser zu konfigurieren, der während der Test-Suite verwendet wird. Alle Parameter sind optional:
* driver (Standard: `chrome`): zu verwendender Browser:
  * `chrome`: Chromium
  * `gecko`: Firefox (der eigene Build von Playwright)
  * `webkit`: WebKit, die Engine hinter Safari
* width, height: Größe der Seite. Ohne sie behält die Seite die eigene Größe des Browserfensters
* args: Browser-Argumente
* prefs: Firefox-Einstellungen (nur `gecko`)
* timeout (Standard: `180`): maximale Zeit in Sekunden, bis eine Seite geladen ist (`navigate`, `historyAction`, eine Antwort auf ein Popup),
  und bis eine Navigation, die ein Schritt startet, abgeschlossen ist
* proxy: Proxy für die Abfragen der Website (Format: `[server]:[port]`)
* verifyTls (Standard: `false`): der Browser lehnt Seiten mit ungültigem HTTPS-Zertifikat ab, sodass der Schritt fehlschlägt. Ohne diese Variable
  werden Zertifikatsfehler ignoriert, wie es immer war
* headless (Standard: `false`): den Browser ohne Fenster ausführen. Nützlich in einer CI/CD-Pipeline
* binaryPath: Browser-Programm, das anstelle des Browsers von Playwright verwendet wird (nur `chrome`)
* trace (Standard: `false`): einen Playwright-Trace jedes fehlgeschlagenen Schritts in `page.trace.zip` speichern. Siehe
  [Fehlerdiagnose](#failure-diagnostics)
* testIdAttribute (Standard: `data-testid`): das Attribut, das ein Locator `TESTID` liest (siehe [Locators](#locators)). Setzen Sie es
  einmal für die Suite, für eine Seite, die ihre Elemente mit `data-test` oder einem anderen Namen markiert. Siehe [Fallstricke](#traps). Der Name muss zu `^[A-Za-z_][A-Za-z0-9_:.-]*$` passen: Jeder andere Wert lässt das Setup scheitern, bevor der Browser startet

Chromium läuft mit eingeschalteter Sandbox, wie unter chromedriver. Braucht eine Suite sie ausgeschaltet (etwa in einem Container, der als
root läuft), übergibt sie `--no-sandbox` in `args`. Ein Host, der unprivilegierte User-Namespaces einschränkt, wie es natives
Ubuntu 23.10 und neuer tut, kann die Sandbox des Chromium von Playwright nicht starten: Das Setup schlägt fehl und nennt die Abhilfen —
`--no-sandbox` in `args`, `binaryPath` mit Verweis auf ein System-Chrome oder die Freigabe der Namespaces
(`sysctl kernel.apparmor_restrict_unprivileged_userns=0`).

HTTPS-Zertifikatsfehler werden standardmäßig ignoriert, wie in der WebDriver-Sitzung, die dieser executor zuvor verwendete
(`acceptInsecureCerts`). Setzen Sie `verifyTls`, um sie zu prüfen (siehe HTTPS-Zertifikate oben).

Weiterhin akzeptiert, aber **mit einer Warnung ignoriert** — sie haben in Playwright keine Entsprechung:
* driverPath, driverPort: Es gibt keinen Treiberprozess mehr
* detach: Playwright kann den Browser nach der Suite nicht weiterlaufen lassen
* debug, logLevel: Sie setzten die Protokollstufe des alten WebDriver-Clients; verwenden Sie stattdessen das eigene `-v` von venom
* prefs mit `chrome`, binaryPath mit `gecko`: Playwright steuert seinen eigenen Firefox-Build

## Aktion {#action}
Aktionen erlauben es Ihnen, den Browser zu bedienen

Eine Aktion auf einem Element (`click`, `fill`, `select`, `uploadFile`, `selectFrame`, `hover`, `dragAndDrop` und `press` mit `find`) wartet, bis das Element bereit ist,
höchstens bis zu seinem `syncTimeout` oder 30 Sekunden. `waitFor` wartet, bis das Element seinen `state` erreicht, innerhalb seines `syncTimeout`. `find` wartet nie: Es zählt die Elemente, die jetzt vorhanden sind, sodass
`result.find ShouldEqual 0` schnell bleibt. `find`, `waitFor`, `hover`, `dragAndDrop` und `press` warten, wie die Aktionen oben,
auf eine bereits laufende Navigation, bevor sie ausgeführt werden. Nach einer Aktion, die eine Seite laden kann (`click`, `fill`, `select`,
`uploadFile`, `execute`, `waitFor`, `hover`, `dragAndDrop`, `press`, eine Antwort auf ein Popup), wartet der Schritt, bis die Seite fertig geladen ist, bevor er
`result.title` und `result.url` liest. Dazu gehört eine Navigation, die die Seite von selbst startet, aus einem Skript
(`location.href = …`) oder von einem Timer ohne Verzögerung. Eine Navigation, die nicht innerhalb von `timeout` abgeschlossen wird, lässt den Schritt fehlschlagen,
und die folgenden Schritte ebenfalls, bis ein `navigate` sie ersetzt. Solange die Seite navigiert, wird kein `page.dump.html` geschrieben.

Ein Aufruf an die Seite, der nie zurückkehrt, lässt seinen Schritt fehlschlagen: nach 60 Sekunden bei einem Lesezugriff (der Titel, der Quelltext, eine Anzahl, ein
Text; zuzüglich des `syncTimeout` des Schritts bei einem Element, auf das er wartet), nach 30 Sekunden bei `execute`.
Der Browser-Aufruf selbst läuft weiter, daher wartet der nächste Schritt auf ihn, bevor er seinen eigenen Aufruf sendet. Ein Aufruf mit eigener Grenze
(die Lesezugriffe und `execute`) wartet höchstens bis zu dieser Grenze und schlägt dann genauso fehl, wobei das Warten
für den folgenden Schritt weitergeht. Eine Aktion ohne eigene Grenze (ein Klick, eine Auswahl, ein Screenshot) wartet höchstens 60 Sekunden, protokolliert eine
Warnung und macht weiter. `navigate`, `historyAction` und ein Timer-Flush, den eine Navigation beendet hat, warten nie, und ein Dialog, der
inzwischen geöffnet wurde, wird zuerst wie üblich geschlossen. Ein fehlschlagender Schritt schreibt nur dann kein `page.dump.html`, solange dieser Aufruf noch
läuft: Stattdessen protokolliert er eine Warnung.

Liste der verfügbaren Ausgabewerte:
* result.url: URL der aktuellen Seite
* result.timeseconds: Dauer der Ausführung der Aktion
* result.title: Titel der aktuellen Seite

### Navigate {#navigate}
Zu einer bestimmten URL navigieren

#### Eingabe {#input}
* URL: URL, zu der navigiert wird
* Reset: Browser-Zustand zurücksetzen

#### Beispiel {#example}
```yaml
  - type: web
    action:
      navigate:
        url: https://www.google.fr
```

### Find {#find}
Ein Element über einen Selektor suchen

[Link zum Tutorial für CSS-Selektoren](https://developer.mozilla.org/fr/docs/Web/CSS/CSS_Selectors)
[Link zum Tutorial für XPATH-Selektoren](https://developer.mozilla.org/fr/docs/Web/XPath)

#### Eingabe {#input-1}
* Selector: Ausdruck, mit dem das Web-Element identifiziert wird
* Locator: Locator, mit dem das Element gesucht wird: `CSS` oder `XPATH`, oder eine der Arten unter [Locators](#locators)

#### Ausgabe {#output}
* result.find: liefert die Anzahl der identifizierten Objekte

#### Beispiel {#example-1}
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

### Locators {#locators}
Ein `find`-Element nennt seine Art in `locator`. Neben `CSS` und `XPATH` entsprechen fünf Arten dem, was ein Benutzer sieht, oder dem,
was die Seite für Tests markiert. Bevorzugen Sie sie gegenüber einem CSS-Pfad: Sie überstehen eine Neugestaltung der Seite.

#### Eingabe {#input-2}
* `locator: ROLE`: `selector` ist eine ARIA-Rolle (`button`, `link`, `textbox`, `checkbox`, `heading`, `listitem`, ...).
  `name` ist optional und filtert nach dem zugänglichen Namen des Elements; ein leerer `name` bedeutet keinen Namensfilter
* `locator: TEXT`: `selector` ist der Text des Elements
* `locator: LABEL`: `selector` ist der Text der Beschriftung eines Formularfelds
* `locator: PLACEHOLDER`: `selector` ist der Platzhaltertext (Placeholder) eines Felds
* `locator: TESTID`: `selector` ist die Test-ID. Gelesen wird das Attribut `data-testid` oder das Attribut, das die Variable
  `testIdAttribute` festlegt. Es muss den ganzen Wert treffen und nimmt kein `exact`
* `exact` (`true` oder `false`, Standard `false`): bei `ROLE` gilt es für `name`, bei `TEXT`, `LABEL` und `PLACEHOLDER`
  für `selector`

Standardmäßig treffen `TEXT`, `LABEL` und ein `ROLE`-Name ohne Beachtung der Groß- und Kleinschreibung, als Teilzeichenkette, und
ignorieren überzählige Leerzeichen: Der Text `add` trifft die Schaltfläche `Add item`. Mit `exact: true` muss der ganze Text
übereinstimmen, mit der Schreibweise wie angegeben. `PLACEHOLDER` trifft das Attribut so, wie es geschrieben ist.

Ein Wert wird immer als reiner Text gelesen. Er ist nie ein CSS-Selektor, ein Engine-Präfix oder ein regulärer Ausdruck. Ein Schritt,
der mehrere Elemente findet, verwendet das erste.

#### Beispiel {#example-2}
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

#### Fehler {#errors}
Ein Schritt mit einem falschen `find` schlägt fehl, bevor er handelt, und die Meldung nennt den Schlüssel:
* ein `locator`, der keine der sieben Arten ist: `invalid find element locator 'X' (must be CSS, XPATH, ROLE, TEXT, LABEL, PLACEHOLDER or TESTID)`
* ein Selektor für `ROLE`, der keine ARIA-Rolle ist: Die Meldung listet die Rollen auf
* ein leerer `selector` bei `TEXT`, `LABEL`, `PLACEHOLDER` oder `TESTID`
* ein `TESTID`-Selektor, der ein NUL-Zeichen enthält
* `name` bei einer anderen Art als `ROLE` oder `exact` bei `TESTID`: Die Meldung sagt, welche Arten es annehmen
* `name`, das keine Zeichenkette ist, oder `exact`, das weder `true` noch `false` ist
* jeder andere Schlüssel: `invalid find element key 'X' for a ROLE locator (must be locator, selector, name, exact)`

Diese Prüfungen gelten für die fünf Arten dieses Abschnitts. Die zusätzlichen Schlüssel eines `CSS`- oder `XPATH`-Elements werden ignoriert, wie sie es immer wurden.

### Click {#click}
Auf ein Element über einen Selektor klicken

#### Eingabe {#input-3}
* Find: Element, das gesucht wird (weitere Informationen im Abschnitt Find)
* Wait: Wartezeit nach dem Klick (in Sekunden)
* SyncTimeout: maximale Zeit in Sekunden, die auf die Bereitschaft des Elements gewartet wird (vorhanden, sichtbar, aktiviert, unbewegt, nicht verdeckt). Standard: `30`

#### Beispiel {#example-3}
```yaml
  - type: web
    action:
      click:
        find:
          selector: button
          locator: CSS
```

### Fill {#fill}
Ein Element (input oder textarea) mit einem Text füllen. Der Text wird hinter den aktuellen Inhalt des Felds getippt, wie es WebDriver
tat; er ersetzt ihn nicht

#### Eingabe {#input-4}
Array strukturierter Eingaben:
* Find: Element, das gesucht wird (weitere Informationen im Abschnitt Find)
* Text: den Wert des Elements mit Text aktualisieren
* Key: den Wert des Elements mit einer Taste aktualisieren
* SyncTimeout: maximale Zeit in Sekunden, die auf die Bereitschaft des Elements gewartet wird (vorhanden, sichtbar, aktiviert, unbewegt, nicht verdeckt). Standard: `30`

#### Beispiel {#example-4}
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
Eine Option eines Web-Elements auswählen

#### Eingabe {#input-5}
* Find: Element, das gesucht wird (weitere Informationen im Abschnitt Find)
* Text: auszuwählende Option
* Wait: Wartezeit nach dem Klick (in Sekunden)
* SyncTimeout: maximale Zeit in Sekunden, die auf die Bereitschaft des Elements gewartet wird (vorhanden, sichtbar, aktiviert, unbewegt, nicht verdeckt). Standard: `30`

#### Beispiel {#example-5}
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
Eine Datei auf ein Web-Element hochladen

#### Eingabe {#input-6}
* Find: Element, das gesucht wird (weitere Informationen im Abschnitt Find)
* Files: String-Array, das die hochzuladenden Dateien festlegt
* Wait: Wartezeit nach dem Klick (in Sekunden)
* SyncTimeout: maximale Zeit in Sekunden, die auf die Bereitschaft des Elements gewartet wird (vorhanden, sichtbar, aktiviert, unbewegt, nicht verdeckt). Standard: `30`

#### Beispiel {#example-6}
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
Zeit abwarten
Ein ganzzahliger Wert legt die Anzahl der Sekunden fest, die gewartet wird

#### Beispiel {#example-7}
```yaml
  - type: web
    action:
      wait: 1
```

### Wait for {#wait-for}
Warten, bis ein Element einen Zustand erreicht. Verwenden Sie es, wenn die Seite nach dem ersten Zeichnen Daten lädt oder ein Panel öffnet.

#### Eingabe {#input-7}
* Find: Element, auf das gewartet wird (mehr dazu unter find und [Locators](#locators))
* State: `visible` (Standard, auch wenn `state` leer ist), `hidden`, `attached` oder `detached`
  * `visible`: das Element existiert und wird angezeigt
  * `hidden`: das Element fehlt oder wird nicht angezeigt
  * `attached`: das Element existiert in der Seite, angezeigt oder nicht
  * `detached`: das Element ist nicht in der Seite
* SyncTimeout: maximale Zeit in Sekunden, die auf den Zustand gewartet wird. Standard: `30`. Ein Wert von `0` oder weniger bedeutet `30`

Bei `visible` wartet der Schritt auf den ersten Treffer.

Jede Prüfung betrachtet das erste Element, das in diesem Moment trifft. Wird dieses Element entfernt und trifft noch ein anderes, wartet der Schritt mit dem nächsten weiter. `detached` wartet, bis kein Element mehr trifft: Wird das erste Element entfernt, wird das nächste zum ersten, daher wartet der Schritt bei mehreren Treffern, bis keiner mehr übrig ist. `hidden` folgt dem ersten Element, das trifft, und das bleibt dasselbe Element, solange es existiert (ein per CSS ausgeblendetes Element trifft weiterhin), daher kann `hidden` bei mehreren Treffern enden, während die anderen noch angezeigt werden. Um zu warten, bis jedes davon ausgeblendet ist, warten Sie auf `detached` oder geben Sie jedem Element seinen eigenen Locator.

#### Beispiel {#example-8}
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

#### Fehler {#errors-1}
* `waitFor: the field 'find' is required (allowed: find, state, syncTimeout)`
* `waitFor: invalid 'state' X (allowed: visible, hidden, attached, detached)`: Der Zustand unterscheidet Groß- und Kleinschreibung
* ein anderes Feld als `find`, `state` und `syncTimeout`: `waitFor: unknown field 'X' (allowed: find, state, syncTimeout)`
* der Zustand wird nicht rechtzeitig erreicht: `waitFor: the element ROLE "button" name "Add" did not become visible within 30s`, gefolgt vom
  Grund des Browsers

### Hover {#hover}
Den Zeiger auf ein Element bewegen, um ein Menü oder einen Tooltip zu öffnen, der beim Darüberfahren erscheint. Der Schritt wartet, bis das Element
bereit ist, wie es ein Klick tut.

#### Eingabe {#input-8}
* Find: Element, über das gefahren wird (mehr dazu unter find und [Locators](#locators)). Der erste Treffer wird verwendet
* SyncTimeout: maximale Zeit in Sekunden, die gewartet wird, bis das Element bereit ist. Standard: `30`

#### Beispiel {#example-9}
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

#### Fehler {#errors-2}
* `hover: the field 'find' is required (allowed: find, syncTimeout)`
* ein anderes Feld als `find` und `syncTimeout`: `hover: unknown field 'X' (allowed: find, syncTimeout)`
* das Element ist nicht rechtzeitig bereit: `hover: ROLE "link" name "Products"`, gefolgt vom Grund des Browsers

### Ziehen und Ablegen {#drag-and-drop}
Ein Element ziehen und auf einem anderen ablegen. Der Schritt wartet, bis beide bereit sind.

#### Eingabe {#input-9}
* From: Element, das gezogen wird (mehr dazu unter find und [Locators](#locators)). Der erste Treffer wird verwendet
* To: Element, auf dem abgelegt wird. Der erste Treffer wird verwendet
* SyncTimeout: maximale Zeit in Sekunden, die gewartet wird, bis das Ziehen abgeschlossen ist. Standard: `30`

#### Beispiel {#example-10}
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

#### Fehler {#errors-3}
* `dragAndDrop: the field 'from' is required (allowed: from, to, syncTimeout)`, und ebenso für `to`
* ein anderes Feld als `from`, `to` und `syncTimeout`: `dragAndDrop: unknown field 'X' (allowed: from, to, syncTimeout)`
* das Ablegen schlägt fehl oder läuft in die Zeitüberschreitung: `dragAndDrop: TEXT "Task A" exact to TESTID "column-done"`, gefolgt vom Grund des Browsers

### Press {#press}
Eine Taste oder eine Tastenkombination drücken, etwa `Enter`, `Escape`, `Tab` oder `Control+A`. Mit `find` geht die Taste an dieses Element.
Ohne ihn geht sie an das Element, das den Fokus hat. `press` nimmt eine Taste oder eine Tastenkombination, keinen Text: Um einen
Text einzugeben, verwenden Sie `fill`.

#### Eingabe {#input-10}
* Key: die Taste oder die Tastenkombination (erforderlich). Groß- und Kleinschreibung zählt. Verwenden Sie einen Playwright-Tastennamen (`Enter`,
  `ArrowDown`, `F5`, `Backspace`) oder eine Taste aus einem Zeichen (`a`, `7`). Eine Kombination verbindet Tasten mit `+`:
  `Control+A`, `Shift+Tab`. Auch die WebDriver-Namen, die `fill` für `key` akzeptiert (`ENTER`, `ESCAPE`, `ARROW_DOWN`), funktionieren,
  als einzelne Taste. Eine Kombination nimmt nur Playwright-Namen. `press` akzeptiert die Tastennamen einer festen Liste; eine seltene Playwright-Taste, die nicht darin steht
  (`ShiftLeft`, `ControlLeft`, `CapsLock`, `ContextMenu`, `NumLock`, `ScrollLock`, `PrintScreen`, `AltGraph`, `F13` und höher), wird mit einer Meldung abgelehnt, die die Taste nennt, und es wird nie eine falsche Taste gesendet. Eine Taste aus einem einzelnen Zeichen wird immer akzeptiert
* Find: Element, an das die Taste geschickt wird (mehr dazu unter find und [Locators](#locators)). Optional. Der erste Treffer wird verwendet
* SyncTimeout: maximale Zeit in Sekunden, die gewartet wird, bis das Element bereit ist. Standard: `30`. Ohne `find` ist kein Element beteiligt, es ist also kein Warten auf ein Element: Es verlängert nur das Limit des Tastenaufrufs selbst (60 Sekunden plus `syncTimeout`)

#### Beispiel {#example-11}
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

#### Fehler {#errors-4}
* `press: the field 'key' is required (allowed: key, find, syncTimeout): press takes one key or one chord`
* ein anderes Feld als `key`, `find` und `syncTimeout`: `press: unknown field 'X' (allowed: key, find, syncTimeout)`
* ein `key`, der kein Text ist, eine YAML-Zahl wie `key: 5`: Die Meldung sagt, ihn in Anführungszeichen zu setzen (`key: "5"`)
* eine unbekannte Taste oder ein Text: `press: invalid 'key' "X": not a key name; press takes one key or one chord`. Bei einer
  Kombination nennt die Meldung den Teil, der fehlgeschlagen ist
* eine WebDriver-Taste, für die Playwright keine Taste hat (`NULL`, `CANCEL`, `HELP`): Die Meldung sagt, dass es dafür kein
  Playwright-Äquivalent gibt

### Select frame {#select-frame}
Ein Frame auswählen, das in der aktuellen Seite vorhanden ist

#### Eingabe {#input-11}
* Find: Frame, das gesucht wird (weitere Informationen im Abschnitt Find)
* SyncTimeout: maximale Zeit in Sekunden, die auf die Bereitschaft des Elements gewartet wird (vorhanden, sichtbar, aktiviert, unbewegt, nicht verdeckt). Standard: `30`

#### Beispiel {#example-12}
```yaml
  - type: web
    action:
      selectFrame:
        find:
          selector: "#iframeResult"
          locator: CSS
```

### Select root frame {#select-root-frame}
Das Root-Frame auswählen, das in der aktuellen Seite vorhanden ist
Ein boolescher Wert, der auf true gesetzt wird, um das Root-Frame auszuwählen

#### Beispiel {#example-13}
```yaml
  - type: web
    action:
      selectRootFrame: true
```

### Next window {#next-window}
Das nächste Fenster (den nächsten Tab) des Browsers auswählen, vom letzten zum ersten umlaufend; bei nur einem Fenster
bleibt der Schritt darauf
Ein boolescher Wert, der auf true gesetzt wird, um das nächste Fenster auszuwählen

#### Beispiel {#example-14}
```yaml
  - type: web
    action:
      nextWindow: true
```

### Back {#back}
Zur vorherigen Seite zurückgehen
Ein boolescher Wert, der auf true gesetzt wird, um zur vorherigen Seite zurückzugehen

#### Beispiel {#example-15}
```yaml
  - type: web
    action:
      historyAction: back
    assertions:
    - result.title ShouldStartWith GitHub
    - result.url ShouldEqual https://github.com/
```

### Forward {#forward}
Zur nächsten Seite vorgehen
Ein boolescher Wert, der auf true gesetzt wird, um zur nächsten Seite vorzugehen

#### Beispiel {#example-16}
```yaml
  - type: web
    action:
      historyAction: forward
    assertions:
    - result.title ShouldStartWith GitHub
    - result.url ShouldEqual https://github.com/team
```

### Refresh {#refresh}
Die Seite neu laden
Ein boolescher Wert, der auf true gesetzt wird, um die Seite neu zu laden

#### Beispiel {#example-17}
```yaml
  - type: web
    action:
      historyAction: refresh
    assertions:
    - result.title ShouldStartWith GitHub
    - result.url ShouldEqual https://github.com/team
```

### Confirm popup {#confirm-popup}
Den Popup-Dialog bestätigen (Bestätigungsdialog und Hinweisdialog)

Ein Dialog, den ein Schritt geöffnet hat, bleibt offen, damit der nächste Schritt ihn mit `confirmPopup` oder `cancelPopup` beantwortet; solange er
offen ist, sind `result.title` und `result.url` leer. Jede andere Aktion, während ein Dialog offen ist, schließt ihn und schlägt
mit `unexpected alert open` fehl, wie es WebDriver tat.
Ein boolescher Wert, der auf true gesetzt wird, um den Popup-Dialog zu bestätigen

#### Beispiel {#example-18}
```yaml
  - type: web
    action:
      confirmPopup: true
```

### Cancel popup {#cancel-popup}
Den Popup-Dialog abbrechen
Ein boolescher Wert, der auf true gesetzt wird, um den Popup-Dialog abzubrechen

#### Beispiel {#example-19}
```yaml
  - type: web
    action:
      cancelPopup: true
```

### Execute javascript {#execute-javascript}
JavaScript-Code ausführen. Der Befehl ist der Rumpf einer Funktion: `return` und `arguments[0]`, `arguments[1]`… funktionieren
wie mit WebDriver

#### Eingabe {#input-12}
* Command: auszuführender JavaScript-Code
* Args: String-Array, das die JavaScript-Argumente festlegt

#### Ausgabe {#output-1}
Was das Skript zurückgibt, falls überhaupt etwas. Gibt es ein Promise zurück, wartet der Schritt auf dessen Wert.
* result.value: der Wert als Text. Ein String bleibt, wie er ist. Jeder andere Wert wird als JSON geschrieben: `42`, `true`,
  `[1,"a"]`, `{"user":{"name":"Ada"}}`, wobei die Schlüssel eines Objekts sortiert sind
* result.valuejson: derselbe Wert, typisiert, wie `result.bodyjson` es für den http-executor ist. Verwenden Sie ihn, um Zahlen zu vergleichen oder
  in ein Objekt oder eine Liste hineinzugreifen: `result.valuejson.user.name`

Das Ergebnis jedes Web-Schritts hat eine Variable `result.valuejson`. Sie ist leer, es sei denn, der Schritt führt ein Skript aus, das
einen Wert zurückgibt.

Ein Skript, das nichts, `undefined` oder `null` zurückgibt, lässt beide leer. Einige Werte haben keine JSON-Form:
* `NaN`, `Infinity` und `-Infinity` werden als diese Wörter gelesen
* ein `Date` wird als seine UTC-Zeit in RFC 3339 gelesen (`1970-01-01T00:00:00Z`)
* ein BigInt, eine URL und ein RegExp werden als ihr Text gelesen
* ein DOM-Element, `document` oder `window` wird als `ref: <Node>`, `ref: <Document>` oder `ref: <Window>` gelesen

Ein Wert, der auf sich selbst verweist, etwa ein Objekt mit einer Eigenschaft, die auf das Objekt zurückzeigt, lässt beide Felder leer.
Das gilt auch für eine URL, die der Browser akzeptiert und venom nicht lesen kann, etwa eine mit einem `%`, auf das keine zwei Hexadezimalziffern folgen
(`https://a.test/x#50%off`). venom protokolliert eine Warnung, und der Schritt gelingt.

#### Beispiel {#example-20}
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


Weitere Informationen zu den Aktionen (https://github.com/ovh/venom/tree/master/executors/web/types.go)
Für eine Aktion können Sie mit dem folgenden Befehl einen Screenshot des Browsers erstellen: `screenshot: [fileName].png`

## Netzwerkaufrufe simulieren {#mock-network-calls}
Ein Schritt kann die Netzwerkanfragen der Seite selbst beantworten, sodass eine Suite ein Front-End ohne sein Backend testen kann. Fügen Sie dem Schritt eine
Liste `mock:` hinzu, neben `action:`. Jeder Eintrag beantwortet jede Anfrage, deren URL zu seinem `url` passt, und die
Anfrage erreicht nie das Netzwerk. Der Host muss nicht existieren: Ein Mock kann die Seite selbst ausliefern und danach
die eigenen Aufrufe der Seite.

#### Eingabe {#input-13}
Array strukturierter Eingaben:
* Url: Muster der URLs, die beantwortet werden (erforderlich). `*` passt auf beliebige Zeichen außer `/`, `**` passt auf beliebige Zeichen,
  `{a,b}` passt auf `a` oder `b`. Das Muster passt auf die gesamte URL: `**/api/users` beantwortet
  `https://app.test/api/users`, nicht `https://app.test/api/users?page=2`; `**/api/users*` beantwortet beide. Jedes `{` braucht
  sein `}`, sonst schlägt der Schritt fehl: Schreiben Sie `\{` für eine literale geschweifte Klammer (`\\{` innerhalb doppelter Anführungszeichen)
* Status: HTTP-Status der Antwort. Standard: `200`
* Headers: Antwort-Header. Schreiben Sie jeden Wert als String: `X-Total: "3"`, nicht `X-Total: 3`
* ContentType: der `Content-Type` der Antwort. Standard: keiner
* Body: der Antwort-Body als Text

Ein Mock beginnt zu antworten, bevor die Aktion des Schritts läuft, und antwortet weiter in den folgenden Schritten desselben Testfalls
in jedem Tab. Passen mehrere Mocks auf eine URL, antwortet der jüngste. Der nächste Testfall beginnt ohne
Mock. Ein Schritt kann `mock:` und keine Aktion enthalten.

Wird `mock:` gesetzt, während ein Popup offen ist, schließt das Popup und der Schritt schlägt fehl, wie bei jeder anderen Aktion. Setzen Sie den Mock
in einem Schritt vor dem Schritt, der das Popup öffnet.

Eine Anfrage, die der Service Worker der Seite selbst beantwortet, erreicht den Mock nie.

#### Beispiel {#example-21}
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

## Fehlerdiagnose {#failure-diagnostics}
Schlägt ein Schritt fehl, schreibt der Web-executor `page.dump.html` in das Arbeitsverzeichnis: den Quelltext des aktuellen Frames.
Er überspringt den Dump, solange die Seite noch ein neues Dokument lädt. Ein Schritt mit `screenshot:` schreibt seinen Screenshot
und den Dump, sobald seine Aktion erfolgreich war.

Mit der Variable `trace: true` schreibt ein fehlschlagender Schritt zusätzlich `page.trace.zip` neben `page.dump.html`: einen Playwright-Trace
dieses Schritts mit jeder Aktion, einem Schnappschuss der Seite davor und danach, Screenshots und den Netzwerkaufrufen.
Öffnen Sie ihn mit dem Trace-Viewer von Playwright:

```bash
go run github.com/mxschmitt/playwright-go/cmd/playwright@v0.6201.1 show-trace page.trace.zip
```

Der Viewer braucht den Playwright-Treiber; der Befehl lädt ihn herunter, falls er fehlt, wie es der Installationsbefehl tut.
Der nächste fehlschlagende Schritt ersetzt beide Dateien. Ein erfolgreicher Schritt schreibt keinen Trace.

Der Trace enthält alles, was der Schritt gesehen und getippt hat, **einschließlich Passwörtern**, sowie die Header und Bodys seiner Netzwerkaufrufe. Das Maskieren von Geheimnissen durch venom gilt für ihn nicht. venom schreibt ihn nur für seinen Eigentümer lesbar, wie
`page.dump.html`. Veröffentlichen Sie ihn nicht als Build-Artefakt und committen Sie ihn nicht. Tracing
verlangsamt außerdem jeden Schritt, bei einer einfachen Seite um etwa 20 ms: Schalten Sie es ein, solange Sie einen Fehler untersuchen.

## Fallstricke {#traps}
* Ein falsch geschriebener Selektor besteht `waitFor` mit dem Zustand `hidden` oder `detached` sofort: Ein Element, das nicht
  existiert, ist verborgen und abgelöst. Prüfen Sie den Selektor zuerst mit einem `find` oder einem Warten auf `visible`.
* `TESTID` liest standardmäßig `data-testid`. Eine Seite, die ihre Elemente mit `data-test` (oder einem anderen Namen) markiert, wird
  erst gefunden, wenn die Suite `testIdAttribute` setzt. Ohne es findet der Schritt das Element nicht.
* Der Seiten-Dump (`page.dump.html`) und der Trace enthalten, was `fill` und `press` eingegeben haben. Das Maskieren von Geheimnissen durch venom
  erreicht sie nicht. Geben Sie kein Geheimnis mit `press` ein, eine Taste nach der anderen. Siehe [Fehlerdiagnose](#failure-diagnostics).
* `press` ist kein Tippen. Es sendet Tasten, keinen Text: `press: Hello` schlägt fehl. Verwenden Sie `fill`, um einen Text einzugeben.

## Beispiel {#example-22}
Ein globales Beispiel

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
