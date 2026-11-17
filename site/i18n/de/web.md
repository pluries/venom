---
translates: web
title: Web
description: Den Web-executor in einer venom-Test-Suite verwenden.
fingerprint: 841e8ee00f470e50ad60416a4c8b072458dd7b33e432bb145851d952a4649966
drafted_by: claude
drafted_on: 2026-11-16
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

Eine Aktion auf einem Element (`click`, `fill`, `select`, `uploadFile`, `selectFrame`) wartet, bis das Element bereit ist,
höchstens bis zu seinem `syncTimeout` oder 30 Sekunden. `find` wartet nie: Es zählt die Elemente, die jetzt vorhanden sind, sodass
`result.find ShouldEqual 0` schnell bleibt. Nach einer Aktion, die eine Seite laden kann (`click`, `fill`, `select`,
`uploadFile`, `execute`, eine Antwort auf ein Popup), wartet der Schritt, bis die Seite fertig geladen ist, bevor er
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
* Locator: Locator, mit dem das Element gesucht wird (CSS oder XPATH)

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

### Click {#click}
Auf ein Element über einen Selektor klicken

#### Eingabe {#input-2}
* Find: Element, das gesucht wird (weitere Informationen im Abschnitt Find)
* Wait: Wartezeit nach dem Klick (in Sekunden)
* SyncTimeout: maximale Zeit in Sekunden, die auf die Bereitschaft des Elements gewartet wird (vorhanden, sichtbar, aktiviert, unbewegt, nicht verdeckt). Standard: `30`

#### Beispiel {#example-2}
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

#### Eingabe {#input-3}
Array strukturierter Eingaben:
* Find: Element, das gesucht wird (weitere Informationen im Abschnitt Find)
* Text: den Wert des Elements mit Text aktualisieren
* Key: den Wert des Elements mit einer Taste aktualisieren
* SyncTimeout: maximale Zeit in Sekunden, die auf die Bereitschaft des Elements gewartet wird (vorhanden, sichtbar, aktiviert, unbewegt, nicht verdeckt). Standard: `30`

#### Beispiel {#example-3}
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

#### Eingabe {#input-4}
* Find: Element, das gesucht wird (weitere Informationen im Abschnitt Find)
* Text: auszuwählende Option
* Wait: Wartezeit nach dem Klick (in Sekunden)
* SyncTimeout: maximale Zeit in Sekunden, die auf die Bereitschaft des Elements gewartet wird (vorhanden, sichtbar, aktiviert, unbewegt, nicht verdeckt). Standard: `30`

#### Beispiel {#example-4}
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

#### Eingabe {#input-5}
* Find: Element, das gesucht wird (weitere Informationen im Abschnitt Find)
* Files: String-Array, das die hochzuladenden Dateien festlegt
* Wait: Wartezeit nach dem Klick (in Sekunden)
* SyncTimeout: maximale Zeit in Sekunden, die auf die Bereitschaft des Elements gewartet wird (vorhanden, sichtbar, aktiviert, unbewegt, nicht verdeckt). Standard: `30`

#### Beispiel {#example-5}
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

#### Beispiel {#example-6}
```yaml
  - type: web
    action:
      wait: 1
```

### Select frame {#select-frame}
Ein Frame auswählen, das in der aktuellen Seite vorhanden ist

#### Eingabe {#input-6}
* Find: Frame, das gesucht wird (weitere Informationen im Abschnitt Find)
* SyncTimeout: maximale Zeit in Sekunden, die auf die Bereitschaft des Elements gewartet wird (vorhanden, sichtbar, aktiviert, unbewegt, nicht verdeckt). Standard: `30`

#### Beispiel {#example-7}
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

#### Beispiel {#example-8}
```yaml
  - type: web
    action:
      selectRootFrame: true
```

### Next window {#next-window}
Das nächste Fenster (den nächsten Tab) des Browsers auswählen, vom letzten zum ersten umlaufend; bei nur einem Fenster
bleibt der Schritt darauf
Ein boolescher Wert, der auf true gesetzt wird, um das nächste Fenster auszuwählen

#### Beispiel {#example-9}
```yaml
  - type: web
    action:
      nextWindow: true
```

### Back {#back}
Zur vorherigen Seite zurückgehen
Ein boolescher Wert, der auf true gesetzt wird, um zur vorherigen Seite zurückzugehen

#### Beispiel {#example-10}
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

#### Beispiel {#example-11}
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

#### Beispiel {#example-12}
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

#### Beispiel {#example-13}
```yaml
  - type: web
    action:
      confirmPopup: true
```

### Cancel popup {#cancel-popup}
Den Popup-Dialog abbrechen
Ein boolescher Wert, der auf true gesetzt wird, um den Popup-Dialog abzubrechen

#### Beispiel {#example-14}
```yaml
  - type: web
    action:
      cancelPopup: true
```

### Execute javascript {#execute-javascript}
JavaScript-Code ausführen. Der Befehl ist der Rumpf einer Funktion: `return` und `arguments[0]`, `arguments[1]`… funktionieren
wie mit WebDriver

#### Eingabe {#input-7}
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

#### Beispiel {#example-15}
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

#### Eingabe {#input-8}
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

#### Beispiel {#example-16}
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

## Beispiel {#example-17}
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
