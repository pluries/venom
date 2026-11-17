---
translates: mobile
title: Mobile
description: Den Mobile-executor in einer venom-Test-Suite verwenden.
fingerprint: b5add67e3549852f87f31f8dbe9f13cb7b412228964257c765cad39d54a9af9c
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# Venom - Executor Mobile {#venom---executor-mobile}

Schritt, um eine native oder hybride Android- oder iOS-App aus einer venom-Suite zu steuern: eine App installieren, Elemente finden, tippen, Text eingeben, Text lesen, scrollen,
einen Screenshot aufnehmen, einen Deep-Link öffnen, in eine Web-View wechseln, einen iOS-Systemdialog beantworten und prüfen, was zu sehen ist. Der Schritt spricht mit einem [Appium](https://appium.io)-3-Server, der auf demselben
Rechner wie venom läuft oder auf einem Mac, den venom über einen SSH-Tunnel erreicht. Er verwendet den UiAutomator2-Treiber für einen Android-
Emulator und den XCUITest-Treiber für einen iOS-Simulator.

Stand: die Meilensteine M1 (#2396), M2 (#2397), M3 (#2398) und M4 (#2399). Android auf einem Emulator und iOS auf einem Simulator, auf demselben Rechner oder auf einem Mac, der über SSH erreicht wird; Web-Views, Koordinaten, Deep-Links und Dialoge. Die iOS-Aufrufe sind gegen einen geskripteten Server getestet und wurden am 2026-10-28 und am 2026-10-29 auf einem Simulator ausgeführt, mit der integrierten App Settings und mit Appiums UIKitCatalog (eine `.zip`-Installation, Dialoge, `scroll` mit `until`, eine Web-View). Nicht auf einem Simulator ausgeführt: ein `.app`-Ordner, Koordinaten, `openURL`, `hideKeyboard` und die Beschriftung `button` des Dialogs (siehe „iOS"; die ausführlichen Lauf-Notizen werden nicht veröffentlicht).

Prüfen Sie, ob es funktioniert: Führen Sie die hermetischen Tests aus (`go test ./executors/mobile/ -count=1`, kein Gerät nötig), danach den echten
Test gegen Ihr eigenes Appium und Ihren Emulator (siehe „Die Tests ausführen"). Die hermetischen Tests sprechen mit einem geskripteten Fake-Server,
sie können also nicht beweisen, dass Appium 3 jede Anfrage akzeptiert: Das kann nur der echte Test.

## Voraussetzungen für den Host {#host-prerequisites}

Wie Sie einen Mac für Android und iOS einrichten, steht weiter unten unter „Den Mac einrichten".

- Appium 3 mit dem UiAutomator2-Treiber (getestet mit Appium 3.9.0 und uiautomator2 8.7.2; Appium 2.x ist nicht geprüft), so gestartet, dass es **nur auf der Loopback-Adresse lauscht** (127.0.0.1).
- Ein Android-Emulator (oder ein Gerät), den Appium sieht.
- Die Android-SDK-Build-Tools (`sdkmanager "build-tools;34.0.0"`), wenn `mobile.app` eine `.apk` nennt: UiAutomator2 braucht `aapt2`, um sie zu lesen. Appium legt die Suche nach seinen Werkzeugen im Cache ab, starten Sie Appium also nach der Installation neu. `ANDROID_HOME` muss in der Shell gesetzt sein, die Appium startet.
- venom auf demselben Rechner. M1 installiert die App aus einem lokalen Dateipfad, den Appium liest, daher müssen beide denselben Host teilen.

Der executor braucht auf dem Appium-Server kein `--relaxed-security`-Flag.

## Variablen {#variables}

Setzen Sie sie unter `vars: mobile:` in der Suite (oder in einer `.venomrc` oder mit `--var`). Die Sitzung wird einmal pro Testfall geöffnet,
vor dem ersten `mobile`-Schritt, und nach dem letzten Schritt geschlossen, auch wenn ein Schritt fehlgeschlagen ist.

```yaml
vars:
  mobile:
    url: http://127.0.0.1:4723
    platform: android
    device: emulator-5554
    app: ./build/app-debug.apk
```

| Variable | Erforderlich | Bedeutung |
|---|---|---|
| `mobile.url` | ja | Adresse des Appium-Servers, `http` oder `https`, ohne Zugangsdaten darin. |
| `mobile.platform` | ja | `android` oder `ios`. |
| `mobile.device` | ja | Das Gerät oder der Emulator. Unter Android wird er an Appium sowohl als `appium:deviceName` als auch als `appium:udid` gesendet. Unter iOS ist es ein Simulatorname (nur als `appium:deviceName` gesendet) oder eine UDID (zusätzlich als `appium:udid` gesendet). Der executor wählt nie von sich aus ein Gerät. Verwenden Sie die adb-Seriennummer, etwa `emulator-5554`; ein AVD-Name wie `venom_test` wird von Appium abgelehnt („not in the list of connected devices"). |
| `mobile.osVersion` | nein | Wird als `appium:platformVersion` gesendet. Der Bericht zeigt die Version, mit der der Server antwortet, sonst diesen Wert. |
| `mobile.app` | nein | Pfad der App, die beim Öffnen der Sitzung installiert wird: eine `.apk` unter Android, ein Simulator-`.app`-Ordner oder dessen `.zip` unter iOS. Ein relativer Pfad wird gegen den Ordner der Suite aufgelöst. |
| `mobile.appSHA256` | nein | Erwartete SHA-256 der `.apk` oder der `.zip`, hexadezimal (Groß- oder Kleinbuchstaben). Braucht `mobile.app`. Wird bei einem `.app`-Ordner abgelehnt. |
| `mobile.appId` | nein | Paket-ID (Android) oder Bundle-ID (iOS) der App. Ohne `mobile.app` startet die Sitzung diese installierte App, und unter iOS ist sie dann erforderlich. Sind beide gesetzt, ist sie auch der Standardwert für `launch` und `terminate`. |
| `mobile.ssh.host` | nein | Host (Name oder Adresse) des Mac. Ist er gesetzt, erreicht venom den Appium-Server über einen SSH-Tunnel zum eigenen Loopback des Hosts, auf dem Port von `mobile.url`, und kopiert die App dorthin. Ohne ihn wird `mobile.url` unverändert verwendet. |
| `mobile.ssh.user` | mit `ssh.host` | SSH-Benutzer. |
| `mobile.ssh.key` | mit `ssh.host` | Pfad des privaten Schlüssels. Nur Schlüssel-Authentifizierung, kein Passwort. |
| `mobile.ssh.passphrase` | nein | Passphrase des Schlüssels. Deklarieren Sie sie unter `secrets`. |
| `mobile.ssh.port` | nein | SSH-Port, Standard 22. |
| `mobile.ssh.knownhosts` / `mobile.ssh.hostkey` | nein | Prüfung des Host-Keys, wie im ssh-executor: eine known_hosts-Datei oder ein fester Schlüssel. Nicht beides. Ist keines gesetzt, warnt venom einmal pro Lauf und prüft den Host-Key nicht. |
| `mobile.capabilities` | nein | Zusätzliche Appium-Capabilities als Map. Siehe unten. |
| `mobile.syncTimeout` | nein | Sekunden, die der executor auf ein Element wartet, Standard `10`. |
| `mobile.sessionTimeout` | nein | Sekunden, die zum Öffnen der Sitzung erlaubt sind, Standard `180` unter Android und `300` unter iOS, wo die erste Sitzung WebDriverAgent baut. |
| `mobile.idleTimeout` | nein | Sekunden, die der Server die Sitzung ohne Befehl offen hält, Standard `120` (gesendet als `appium:newCommandTimeout`). |

Eine negative Zahl für eines der drei Timeouts wird abgelehnt. Ein falscher Wert wird abgelehnt, bevor eine Anfrage gesendet wird.

### Die App-Datei und ihre SHA-256 {#the-app-file-and-its-sha-256}

Mit `mobile.app` liest venom die Datei einmal und berechnet ihre SHA-256. Ist `mobile.appSHA256` gesetzt und weicht ab, schlägt der
Testfall fehl, **bevor eine Sitzung geöffnet wird**, und die Meldung nennt beide Hashes. Der berechnete Hash steht immer im
Bericht (`appsha256`), sodass ein Bericht sagt, welcher Build gelaufen ist. `mobile.appSHA256` ohne `mobile.app` wird abgelehnt.

Ohne SSH-Tunnel deckt der Hash die Bytes ab, die venom gelesen hat, und Appium installiert danach aus dem Pfad: Eine
dazwischen ausgetauschte Datei würde nicht erkannt. Mit `mobile.ssh.*` wird die Datei auf den Mac kopiert, und der Hash deckt die gesendeten Bytes ab
(siehe „Den Mac über SSH erreichen").

### Zusätzliche Capabilities {#extra-capabilities}

`mobile.capabilities` fügt der Sitzungsanfrage Capabilities hinzu. Eine Capability, die der executor selbst setzt, wird
vor jeder Anfrage abgelehnt, mit einer Meldung, die die Variable nennt, der sie gehört. Die Namen werden ohne das Präfix
`appium:` verglichen, auch innerhalb von `appium:options`.

| Capability | Gesetzt durch |
|---|---|
| `platformName` | `mobile.platform` |
| `deviceName`, `udid` | `mobile.device` |
| `platformVersion` | `mobile.osVersion` |
| `app` | `mobile.app` |
| `appPackage`, `bundleId` | `mobile.appId` |
| `automationName` | der executor (UiAutomator2 unter Android, XCUITest unter iOS) |
| `newCommandTimeout` | `mobile.idleTimeout` |
| `autoAcceptAlerts`, `autoDismissAlerts` | die Aktion alert: Der Schritt behandelt Dialoge, also darf es die Sitzung nicht tun |

## Eingabe {#input}

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

`text` ist ein **Locator** (er findet ein Element über seinen sichtbaren Text). Der Text, den Sie eintippen, ist `value`.

### Locators {#locators}

Ein Schritt, der ein Element anspricht, gibt **genau einen** Locator an. Null oder zwei sind ein Fehler vor jeder Anfrage.

| Feld | Findet das Element über |
|---|---|
| `accessibilityId` | seine Accessibility-ID (Content-Description) |
| `text` | seinen exakten sichtbaren Text (ein UiSelector-`text`); Anführungszeichen und Backslashes darin werden maskiert |
| `id` | seine Resource-ID |
| `xpath` | einen XPath über die View-Hierarchie |
| `uiautomator` | einen UiAutomator-Selektor, unverändert gesendet |
| `css` | einen CSS-Selektor, nur innerhalb einer Web-View |
| `name` | nur iOS: den Namen des Elements |
| `predicate` | nur iOS: einen NSPredicate-String, unverändert gesendet |
| `classChain` | nur iOS: eine iOS-Class-Chain, unverändert gesendet |

`id` und `uiautomator` gibt es nur unter Android; `name`, `predicate` und `classChain` nur unter iOS. Eine Art der jeweils anderen Plattform
wird vor jeder Anfrage abgelehnt, mit einer Meldung, was stattdessen zu verwenden ist. Die Zuordnung für iOS steht unter „iOS".

Auf ein Element wird gewartet: Der executor fragt alle 500 ms erneut, bis es erscheint oder die Wartezeit abgelaufen ist (standardmäßig 10 s,
oder das `syncTimeout` des Schritts, oder `mobile.syncTimeout`). Erscheint das Element nie, nennt der Fehler den Locator und
die Wartezeit. Ein Element, das beim ersten Versuch gefunden wird, verursacht keine Verzögerung.

## Aktionen und Ergebnisse {#actions-and-results}

| Aktion | Braucht | Was sie tut | Ergebnisschlüssel, die sie füllt |
|---|---|---|---|
| `tap` | einen Locator oder `x` und `y` (mit optionalem `unit`) | klickt das Element oder tippt den Punkt an | `found` (mit einem Locator) |
| `fill` | einen Locator, `value`, optional `clear` | tippt `value` in das Element, nachdem es geleert wurde, wenn `clear: true` gilt. Ein leeres `value` tippt nichts | `found` |
| `clear` | einen Locator | leert das Element | `found` |
| `read` | einen Locator, optional `attribute` | liest den Text des Elements oder das Attribut | `found`, `text` (ohne Attribut) oder `value` (Attribut; ein fehlendes Attribut wird als leer gelesen) |
| `count` | einen Locator | zählt die Treffer jetzt, ohne zu warten. Null ist eine Antwort, kein Fehler | `count`, `found` (wahr, wenn größer als null) |
| `waitFor` | einen Locator, optional `state` | wartet, bis das Element vorhanden (Standard) oder nicht mehr vorhanden ist | `found` |
| `swipe` | `direction`, optional Locator und `percent`; oder `x`, `y`, `toX`, `toY` (mit optionalem `unit`), um von einem Punkt zu einem anderen zu ziehen | wischt innerhalb des Elements oder, ohne Locator, innerhalb des sichtbaren Bereichs des Geräts; dieser Bereich ist das Fenster, an jeder Seite um 10 Prozent eingerückt, damit die Randbereiche der Systemgesten nie berührt werden | `found`, wenn ein Locator angegeben wurde |
| `scroll` | `direction`, optional Locator und `percent` | scrollt innerhalb des Elements oder des sichtbaren Bereichs (das Fenster, an jeder Seite um 10 Prozent eingerückt, damit die Randbereiche der Systemgesten nie berührt werden). Mit `until: true` und einem Locator scrollt es den sichtbaren Bereich, bis das Element auf dem Bildschirm ist, höchstens 10 Mal | `found`, wenn ein Locator angegeben wurde |
| `back` | nichts | drückt die Zurück-Taste | keine |
| `hideKeyboard` | nichts | blendet die Bildschirmtastatur aus. Ist keine zu sehen, ist das kein Fehler | keine |
| `launch` | `appId` oder `mobile.appId` | bringt die App in den Vordergrund (`mobile: activateApp`) | keine |
| `terminate` | `appId` oder `mobile.appId` | beendet die App (`mobile: terminateApp`) | keine |
| `screenshot` | optional `path` | nimmt einen Screenshot auf. Mit `path` wird das PNG dorthin geschrieben (Modus 0600) | `found`, `screenshot` (der Pfad) |
| `source` | nichts | liefert die View-Hierarchie als XML | `found`, `source` |
| `context` | `context`: `native`, ein Web-View-Name (`WEBVIEW_...`) oder leer für die erste Web-View | wechselt zwischen der nativen App und einer Web-View. Bei einer Web-View wartet es (die Wartezeit des Schritts), bis sie aufgelistet ist. Ein `css`-Locator braucht eine Web-View | `context` (die aktive), `contexts` (die Liste, die der Server geliefert hat; nach einem Wechsel zu native leer) |
| `openURL` | `url` | öffnet einen Deep-Link: `url` scheme://..., auf das App-Paket beschränkt, wenn es bekannt ist (`mobile: deepLink`) | keine |
| `alert` | optional `choice` und `button` | nur iOS: akzeptiert oder verwirft den offenen Systemdialog (siehe „iOS") | `text` (der Text des Dialogs) |

Eine unbekannte Aktion wird mit der Liste der unterstützten abgelehnt. Wo sich eine Aktion unter iOS unterscheidet, siehe „Unterschiede zu Android".

## Ausgabe {#output}

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

Die Fakten zum Lauf (`platform`, `device`, `osversion`, `app`, `appsha256`, `appid`) stehen in **jedem** Ergebnis, sodass jeder
Testfall eines Berichts zeigt, welcher Build auf welchem Gerät gelaufen ist. Ein Schlüssel ohne Wert fehlt im Ergebnis.

Standard-Assertion: `result.err ShouldBeEmpty`.

### Fehler {#errors}

- Eine **Fehlbedienung** (zwei Locators, eine unbekannte Aktion, eine falsche `direction`, eine fehlende Einstellung) ist ein venom-Fehler, bevor
  eine Anfrage gesendet wird. Der Schritt schlägt mit einer Meldung fehl, die die Ursache nennt.
- Ein **Fehler auf dem Server oder dem Gerät** (Element nicht gefunden, ein abgelehnter Befehl, ein nicht erreichbarer Server, ein Aufruf,
  der hängt) landet in `result.err` als `action <name>: <message>`. Der Schritt schlägt an der Standard-Assertion fehl, und der Lauf geht weiter.
  Der Appium-Stacktrace kommt nie ins Ergebnis.

## Web-Views und hybride Apps {#web-views-and-hybrid-apps}

Eine hybride App zeigt Webseiten innerhalb eines nativen Bildschirms. Die Aktion `context` wechselt zwischen beiden.

- `context: native` geht zurück zur nativen App.
- `context: WEBVIEW_<package>` wechselt zu dieser Web-View.
- Ein leeres `context` wechselt zur ersten Web-View, die der Server auflistet.

Bei einer Web-View wartet der executor bis zur Wartezeit des Schritts (`syncTimeout`, `mobile.syncTimeout`) darauf, dass sie aufgelistet wird, sodass eine
spät ladende Seite gefunden wird. Erscheint rechtzeitig keine, listet der Fehler die Contexts auf, die der Server gemeldet hat. Das
Ergebnis enthält `context` (die aktive) und `contexts` (die Liste, die der Server geliefert hat).

Innerhalb einer Web-View übernimmt chromedriver die Suche, daher sind `css` und `xpath` die Locators, die Sie verwenden sollten. `text`,
`accessibilityId` und `uiautomator` sind native Strategien und werden vom Server in einer Web-View abgelehnt
(auf einem Gerät geprüft, 2026-10-28: `accessibilityId` kam als Fehler zurück); was `id` dort bedeutet (die HTML-ID), ist nicht geprüft. Nach
`context: native` funktioniert ein nativer Locator wie zuvor. `css` im nativen Context wird mit einer Meldung vor jeder Suche abgelehnt. `source` liefert in einer Web-View das HTML der Seite und im nativen Context die XML-
Hierarchie.

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

### Was die Web-View braucht {#what-the-web-view-needs}

- **Debugging muss aktiviert sein.** Die App muss das WebView-Debugging aktivieren. Ohne es listet der Server nur `NATIVE_APP`, und
  ein Wechsel zu einer Web-View schlägt mit dieser Liste fehl.
- **Ein passender chromedriver.** UiAutomator2 steuert eine Web-View über einen chromedriver, dessen Version zur WebView auf
  dem Gerät passt. Passen die Versionen nicht, schlägt der Wechsel auf dem Server fehl. Setzen Sie `appium:chromedriverExecutable` (ein Pfad auf dem Appium-
  Host) über `mobile.capabilities` oder in der eigenen Konfiguration des Servers:

  ```yaml
  vars:
    mobile:
      capabilities:
        appium:chromedriverExecutable: /opt/chromedrivers/chromedriver-114
  ```
- **Der automatische Download ist Sache des Betreibers.** Der Appium-Server kann selbst einen chromedriver holen
  (`chromedriver_autodownload`). Dafür ist eine Server-Option nötig, die der Betreiber auf dem Appium-Server setzt. venom
  sendet sie nie und braucht weder `--relaxed-security` noch `--allow-insecure`.

## Apps ohne Labels und Koordinaten {#unlabelled-apps-and-coordinates}

Eine App, die keine Accessibility-Labels veröffentlicht (manche Flutter-Release-Builds, Spiele, selbst gezeichnete Views), ist nur
über Koordinaten erreichbar. Hat die App irgendwelche Labels, IDs oder Texte, bevorzugen Sie einen Locator: Eine Koordinate bricht, wenn sich das Layout,
die Bildschirmgröße oder die Schrift ändert.

`tap` und `swipe` nehmen statt eines Locators einen Punkt:

| Feld | Bedeutung |
|---|---|
| `x`, `y` | der Punkt, der angetippt wird, oder wo das Wischen beginnt. 0 ist ein gültiger Wert |
| `toX`, `toY` | wo das Wischen endet. Geben Sie beide an, und weder `direction` noch `percent` |
| `unit` | `pixels` (Standard, auch `px`) oder `percent` |

Mit `unit: percent` ist der Punkt ein Anteil, 0 bis 100, des Fensterrechtecks: `x: 50` und `y: 50` ist die Mitte. Bei einem
Fenster von 1080x2400 sendet das den Punkt 540, 1200. Pixel werden wie angegeben gesendet, und der Server begrenzt sie. Ein Punkt mit einem
Locator, `x` ohne `y`, ein negativer Wert, NaN oder unendlich, ein Pixelwert über 10 Millionen oder ein Prozentwert über 100
wird vor jeder Anfrage abgelehnt.

Prozentuale Koordinaten sowie `swipe` oder `scroll` ohne Locator lesen die Fenstergröße vom Server. In einer Web-View kann diese
Größe in den Einheiten des Browsers angegeben sein, nicht in den Pixeln des Geräts: Wechseln Sie zurück zu native (`context: native`), bevor Sie
prozentuale Koordinaten oder ein `swipe` und `scroll` ohne Locator verwenden. Nicht auf einem Gerät geprüft: Der Handlauf verwendete Pixel- und Prozent-
Koordinaten nur im nativen Context.

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

## Deep-Links {#deep-links}

`openURL` öffnet einen Link in der App, über Appiums `mobile: deepLink`:

```yaml
      - type: mobile
        action: openURL
        url: myapp://orders/42
```

- `url` muss wie `scheme://...` aussehen, ohne Leerzeichen oder Steuerzeichen, höchstens 2048 Bytes. Alles andere wird
  vor jeder Anfrage abgelehnt, und die Meldung wiederholt den Link nicht (er kann ein Token enthalten).
- Der Link ist auf das App-Paket beschränkt (das `appId` des Schritts, sonst `mobile.appId`), wenn eines bekannt ist.
- Eigene Schemata werden akzeptiert: Sie sind der Sinn eines Deep-Links. Der Link wird an Appiums `mobile: deepLink` übergeben, das
  ihn über `am start` ausführt; venom baut keinen Shell-Befehl. Im getesteten Stack (Appium 3.9.0, uiautomator2 8.7.2,
  Android-14-Emulator) erreichten fünf Links mit Shell-Metazeichen den Server, schlugen fehl, weil sie niemand behandelte,
  und legten auf dem Gerät keine Datei an. Ein älteres Appium oder ein älterer Treiber maskiert sie womöglich nicht: Halten Sie Deep-Links aus nicht vertrauenswürdiger Eingabe
  aus Suites heraus.

## Grenzen {#limits}

| Grenze | Wert |
|---|---|
| Ein Aufruf an den Server | 60 s |
| Die Sitzung öffnen | 180 s (`mobile.sessionTimeout`) |
| Untätige Sitzung auf dem Server | 120 s (`mobile.idleTimeout`) |
| Pause zwischen zwei Versuchen, ein Element zu finden | 500 ms |
| Standardwartezeit für ein Element | 10 s (`mobile.syncTimeout`, `syncTimeout`) |
| Scroll-Vorgänge von `scroll` mit `until` | 10 |
| Ein dekodierter Screenshot | 12 MiB |
| Die Sitzung schließen | 15 s |
| Jede der beiden Diagnose-Anfragen | 15 s |

Eine Weiterleitung des Servers wird nicht verfolgt. Das eigene `timeout:` des Schritts gilt zusätzlich zu diesen Grenzen weiter.

## Diagnose bei einem fehlschlagenden Schritt {#diagnostics-on-a-failing-step}

Schlägt ein Schritt auf dem Server oder dem Gerät fehl, speichert der executor zwei Dateien in dem Verzeichnis, in dem venom läuft:

- `screen.png`: ein Screenshot des Geräts;
- `screen.dump.xml`: die View-Hierarchie.

**Warnung: Beide Dateien zeigen, was auf dem Bildschirm steht, auch ein Secret, das Sie getippt haben oder das die App anzeigt.** Sie sind nicht
von der Maskierung von Secrets in den Berichten von venom abgedeckt. Sie werden mit Modus 0600 im Arbeitsordner geschrieben. Veröffentlichen Sie
sie nicht als Build-Artefakte. Der nächste fehlschlagende Schritt ersetzt sie; ein erfolgreicher Schritt schreibt und löscht nichts. Dass sich eine
Datei nicht abrufen lässt, ist eine Warnung im Log und verdeckt nicht den eigenen Fehler des Schritts. Ein Fehler wegen Fehlbedienung oder eine Sitzung,
die der Server nicht mehr hat, schreibt nichts.

Zwei venom-Läufe, die im **selben Verzeichnis** gestartet werden, überschreiben gegenseitig ihre Dateien. Starten Sie sie aus verschiedenen Verzeichnissen.

## Den Mac über SSH erreichen {#reaching-the-mac-through-ssh}

Lassen Sie Appium auf dem Loopback des Mac lauschen (`127.0.0.1:4723`); es hat kein eigenes Passwort. Setzen Sie `mobile.ssh.host`, `mobile.ssh.user` und `mobile.ssh.key` und lassen Sie `mobile.url` als die Adresse, die der Mac sieht (`http://127.0.0.1:4723`): venom öffnet den Tunnel, wenn der Testfall beginnt, und schließt ihn, wenn er endet. Der Tunnel lauscht nur auf `127.0.0.1` und wählt immer den Loopback des Mac an.

- **Host-Key:** Setzen Sie `mobile.ssh.knownhosts` oder `mobile.ssh.hostkey`. Ein Schlüssel, der nicht passt, lehnt die Verbindung vor jeder Anfrage ab.
- **Die App-Datei:** Mit einem Tunnel wird `mobile.app` auf den Mac nach `/tmp/venom-mobile-<random>/app.apk` (`app.zip` unter iOS) kopiert, von dort installiert und am Ende der Sitzung entfernt. Kein Wert der Suite erscheint in einem Befehl, der auf dem Mac ausgeführt wird. `mobile.appSHA256` wird über die gesendeten Bytes geprüft; eine Abweichung lässt den Lauf vor jeder Sitzung fehlschlagen und entfernt die Kopie. Ein abgebrochener Lauf kann den Ordner zurücklassen: Suchen Sie nach `/tmp/venom-mobile-*`.
- **Grenzen:** Eine `https://`-URL durch den Tunnel wird gegen `127.0.0.1` geprüft, verwenden Sie daher im Tunnel `http://`. Der Verbindungsaufbau und der SSH-Handshake sind durch `mobile.sessionTimeout` begrenzt (standardmäßig 180 s), sodass ein Host, der nicht antwortet, das Setup fehlschlagen lässt, statt es zu blockieren.
- **Einfaches http ohne Tunnel:** Eine `http://`-URL zu einem anderen Rechner gibt eine Warnung pro Lauf aus; eine Loopback-URL gibt keine aus.

## Sicherheitshinweise {#security-notes}

- **Appium darf nur auf dem Loopback lauschen.** Ein Appium-Server im Netzwerk lässt jeden, der ihn erreicht, den
  Emulator und den Rechner steuern. In M1 verbindet sich venom mit Appium auf demselben Host.
- Die App wird aus einer **lokalen Datei** installiert, daher müssen sich Appium und venom in M1 einen Host teilen.
- **Screenshots, der Dump und `result.source` zeigen, was auf dem Bildschirm steht, auch ein Secret, das Sie getippt haben.** Sie sind NICHT
  von der Maskierung von Secrets in den Berichten von venom abgedeckt, dieselbe Grenze wie beim Trace des web-executors. Halten Sie `screen.png` und
  `screen.dump.xml` aus veröffentlichten Artefakten und gemeinsamen Build-Ordnern heraus.
- `fill` gibt den getippten Text nie in einem Ergebnis oder im Aufruf-Log von `-v` wieder. Mit `-v` wird jeder Aufruf an den Server protokolliert (Methode, Pfad, Dauer; nie ein Body).
- Es werden nur Endpunkte von Appium 3 verwendet, und der executor braucht kein `--relaxed-security`.
- **Der Seiteninhalt einer Web-View und der darin getippte Text können in Screenshots und in `result.source` erscheinen**, wie bei jedem
  nativen Bildschirm, und die Maskierung von Secrets deckt sie nicht ab. Mit `read` gelesener Seitentext ist ein Ergebniswert wie jeder andere.
- **Die URL eines Deep-Links wird an Appiums `mobile: deepLink` übergeben, das sie über `am start` ausführt.** venom selbst
  baut nie einen Shell-Befehl; der Link wird auf Form und Länge geprüft und auf das App-Paket beschränkt, wenn eines bekannt ist.
  Appium hat Shell-Metazeichen im getesteten Stack maskiert (Appium 3.9.0, uiautomator2 8.7.2, Android-14-arm64-Emulator,
  gemessen am 2026-10-28). Ein älteres Appium oder ein älterer Treiber tut das womöglich nicht, halten Sie daher Deep-Links aus nicht vertrauenswürdiger Eingabe aus Suites heraus.
- **Der chromedriver läuft auf dem Appium-Host**, und welche Binärdatei es ist, bleibt Sache des Betreibers. venom sendet keine
  Option, die `--relaxed-security` oder `--allow-insecure` einschaltet, und der Server braucht beides nicht.
- **Der Text eines iOS-Dialogs** geht nach `result.text` und sonst nirgendwohin. Er wird weder in eine Log-Zeile noch in einen Fehler geschrieben, ist aber
  ein Ergebniswert wie jeder andere, ein Secret darin wird also nur ausgeblendet, wenn es ein deklariertes Secret ist.
- **Ein `.app`-Ordner wird als `app.zip` auf den Mac kopiert**, in einen von venom benannten Ordner (`/tmp/venom-mobile-<random>`, Modus 0700),
  ohne dass ein Wert der Suite in einem dort ausgeführten Befehl vorkommt. venom lehnt ein `.app` ab, das einen symbolischen Link enthält, und berechnet keinen Hash für
  einen Ordner: Seine Integrität wird nicht geprüft. Geben Sie eine `.zip` und `mobile.appSHA256` an, wenn sie geprüft werden soll.
- **Der XCUITest-Treiber und WebDriverAgent laufen auf dem Mac** und werden vom Betreiber installiert. venom lädt oder baut
  sie nicht; die erste Sitzung baut dort WebDriverAgent.

## iOS {#ios}

Mit `mobile.platform: ios` steuert dieselbe Suite eine App auf einem iOS-**Simulator** über den XCUITest-Treiber. Ein echtes
iPhone ist nicht abgedeckt. Die iOS-Aufrufe sind nach dem dokumentierten Verhalten des XCUITest-Treibers implementiert und gegen einen
geskripteten Server getestet. Sie wurden auf einem Simulator ausgeführt (iPhone 17, iOS 26.5, Appium 3.9.0, XCUITest-Treiber 12.22.3), in zwei Läufen:

- 2026-10-28, die integrierte App Settings, gestartet über die Bundle-ID, Suite `examples/handrun-settings.yml`, 3 von 3 Testfällen
  bestanden: Sitzung öffnen, `waitFor`, `tap`, Wischen nach Richtung, `terminate`, `launch`, die Locators `text`, `name`,
  `predicate` und `classChain`, `count` und `alert` ohne offenen Dialog.
- 2026-10-29, Appiums UIKitCatalog v4.0.5 als `UIKitCatalog.zip` mit `mobile.appSHA256`, durch den SSH-Tunnel,
  Suite `examples/handrun-uikitcatalog.yml`, Endstatus PASS: die `.zip`-Installation, `alert` akzeptieren (der Text des Dialogs
  kam in `result.text` zurück) und verwerfen, `scroll` mit `until` und eine Web-View (`context` meldete einen `WEBVIEW`-
  Context, `css: body` zählte mehr als 0, `context: native` ging zurück). Die iOS-Web-View brauchte keinen chromedriver.

Nicht auf einem Simulator ausgeführt: ein `.app`-Ordner, Koordinaten, `openURL`, `hideKeyboard`, die Beschriftung `button` des Dialogs, prozentuale
Koordinaten. Die ausführlichen Lauf-Notizen werden nicht veröffentlicht.
Zeilen einer Liste wie in Settings sind unter iOS 26 kein `XCUIElementTypeCell`: Schreiben Sie eine Class-Chain als `**/*[...]`, nicht mit einem Cell-Typ.
Die erste Sitzung dauerte Minuten (Build von WebDriverAgent), deshalb beträgt das Standard-Timeout der Sitzung 300 s.

### Voraussetzungen auf dem Mac {#prerequisites-on-the-mac}

- Xcode und seine Kommandozeilenwerkzeuge sowie mindestens ein Simulator.
- Appium 3 mit dem XCUITest-Treiber: `appium driver install xcuitest`. Lassen Sie Appium auf der Loopback-Adresse lauschen, wie unter Android.
- Die erste Sitzung baut WebDriverAgent und dauert Minuten. Deshalb ist `mobile.sessionTimeout` unter iOS standardmäßig 300 s.
- Ein Simulator braucht keine Code-Signierung.
- Auf einem Apple-Silicon-Mac muss die App einen arm64-Slice haben, und die Android-Emulator-Images sind dort ebenfalls arm64. Eine iOS-
  `.app` mit nur i386- oder x86_64-Slices wird von Appium abgelehnt („does not support the arm64 Simulator architecture"); eine
  APK mit nur `armeabi-v7a`- und `x86`-Bibliotheken schlägt mit `INSTALL_FAILED_NO_MATCHING_ABIS` fehl (sie braucht `arm64-v8a` oder
  keine nativen Bibliotheken). Cloud Greys TheApp v1.10.0 (2019) scheiterte auf dem Mac des Maintainers auf beide Arten.
- Ein fertiges Beispiel, um iOS auszuprobieren: Appiums UIKitCatalog (`UIKitCatalog.zip`, ein Simulator-Build mit arm64- und x86_64-
  Slices, Bundle-ID `com.example.apple-samplecode.UICatalog`), von der Release-Seite von
  <https://github.com/appium/ios-uicatalog>. Die Suite `examples/handrun-uikitcatalog.yml` läuft darauf.

### Einstellungen, die sich unter iOS ändern {#settings-that-change-on-ios}

| Einstellung | Unter iOS |
|---|---|
| `mobile.device` | Ein Simulatorname (`iPhone 16`) oder seine UDID. Eine UDID wird auch als `appium:udid` gesendet; ein Name nur als `appium:deviceName`. |
| `mobile.osVersion` | Wird als `appium:platformVersion` gesendet, wie unter Android. |
| `mobile.app` | Ein Simulator-`.app`-Ordner oder dessen `.zip`. Eine **`.ipa` ist ein Geräte-Build und lässt sich nicht auf einem Simulator installieren**: Sie wird vor jeder Anfrage abgelehnt. Bauen Sie stattdessen ein Simulator-`.app`. |
| `mobile.appSHA256` | Nur für eine `.zip`. Ein `.app`-Ordner hat keine einzelne Datei, die sich hashen ließe, daher wird die Einstellung mit einem solchen abgelehnt. |
| `mobile.appId` | Die Bundle-ID. Ohne `mobile.app` wird die installierte App über ihre Bundle-ID gestartet, und `mobile.appId` ist dann erforderlich. |
| `mobile.sessionTimeout` | Standardmäßig 300 s. |

Über SSH wird ein `.app`-Ordner auf dem Rechner gezippt, auf dem venom läuft, und als `app.zip` auf den Mac kopiert; eine `.zip` wird
unverändert kopiert. Siehe „Den Mac über SSH erreichen".

### Locators unter iOS {#locators-on-ios}

| Feld | Findet das Element über |
|---|---|
| `accessibilityId` | seine Accessibility-ID |
| `text` | sein Label: gesendet als Prädikat `label == "<text>"`, wobei Anführungszeichen, Backslashes und Zeilenumbrüche im Text maskiert werden |
| `name` | seinen Namen |
| `predicate` | einen NSPredicate-String, unverändert gesendet |
| `classChain` | eine iOS-Class-Chain, unverändert gesendet |
| `xpath` | einen XPath über die View-Hierarchie |
| `css` | einen CSS-Selektor, nur innerhalb einer Web-View |

`id` und `uiautomator` werden unter iOS abgelehnt; `name`, `predicate` und `classChain` werden unter Android abgelehnt. Wie unter Android gibt ein
Schritt genau einen Locator an.

### Unterschiede zu Android {#differences-from-android}

| Aktion | Unter iOS |
|---|---|
| `tap`, `fill`, `clear`, `read`, `count`, `waitFor`, `screenshot`, `source`, `context` | Dieselben Befehle wie unter Android. |
| `tap` und `swipe` über `x`, `y` | Der Punkt ist in Points angegeben, nicht in Pixeln. `tap` verwendet `mobile: tap`, `swipe` verwendet `mobile: dragFromToForDuration` über eine halbe Sekunde. `unit: percent` bezieht sich auf das Fensterrechteck, das iOS in Points meldet. |
| `swipe`, `scroll` über `direction` | `mobile: swipe` und `mobile: scroll`, auf dem Element oder der ganzen App. `percent` wird abgelehnt: XCUITest hat kein solches Argument. |
| `scroll` mit `until` | Wiederholt `mobile: scroll` bis zu 10 Mal. Der Server sagt nicht, wann die Liste endet, daher stoppt die Schleife am Element oder an der Grenze. |
| `back` | Wird abgelehnt: iOS hat keine Zurück-Taste. Tippen Sie das eigene Zurück-Bedienelement der App an. |
| `hideKeyboard` | `mobile: hideKeyboard`. Eine Tastatur, die nicht zu sehen ist, ist kein Fehler. |
| `launch`, `terminate` | `mobile: activateApp` und `mobile: terminateApp`, die App wird über `bundleId` benannt (`appId` oder `mobile.appId`). |
| `openURL` | `mobile: deepLink` mit der `url` und der `bundleId`, wenn sie bekannt ist. |
| `alert` | Nur iOS. Unter Android wird sie abgelehnt. |

### Dialoge {#alerts}

Eine iOS-App kann einen Systemdialog auslösen, zum Beispiel eine Berechtigungsanfrage. Die Aktion `alert` liest seinen Text und akzeptiert
oder verwirft ihn dann:

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

- `choice` ist `accept` (Standard) oder `dismiss`. Jeder andere Wert wird vor jeder Anfrage abgelehnt.
- Ohne `button` antwortet der Schritt mit dem einfachen Befehl zum Akzeptieren oder Verwerfen. Mit `button` drückt er die Schaltfläche mit
  dieser Beschriftung (`mobile: alert`).
- `result.text` enthält den Text des Dialogs. **Ein Dialog kann ein Secret zeigen** (einen Code, einen Namen): Der Text wird nie in eine Log-
  Zeile oder eine Fehlermeldung geschrieben, und die Maskierung von Secrets in den Berichten von venom deckt nur deklarierte Secrets ab.
- Ist kein Dialog offen, schlägt der Schritt mit `no alert open` in `result.err` fehl.
- Die Sitzung darf Dialoge nicht von selbst beantworten: `autoAcceptAlerts` und `autoDismissAlerts` in `mobile.capabilities` werden
  abgelehnt, weil der `alert`-Schritt sie behandelt.
- Ein Deep-Link auf einem Simulator kann einen System-Bestätigungsdialog auslösen („Open in ...?"). Lassen Sie auf ein `openURL` einen `alert`-
  Schritt folgen, wenn Ihre App auf diese Weise geöffnet wird, sonst findet der nächste Schritt den Dialog in seinem Weg.

## Noch nicht {#not-yet}

Geplant, abgelehnt oder heute nicht vorhanden:

- Ein echtes iOS-Gerät: Nur Simulatoren sind im Umfang.
- Flutter auf dem iOS-Simulator gibt es nur als Debug-Build: Flutter baut keine Release-App für den Simulator.
- Apps ohne arm64-Slice auf einem Apple-Silicon-Mac.
- Passwort-Anmeldung über SSH: nur Schlüssel-Authentifizierung.
- Ein für Sie heruntergeladener chromedriver: Für eine Android-Web-View stellt der Betreiber einen passenden bereit.
- Noch nicht auf einem Simulator ausgeführt (siehe „iOS"; die ausführlichen Lauf-Notizen werden nicht veröffentlicht): die Installation eines `.app`-Ordners (nur eine
  `.zip` lief), `x`/`y` und prozentuale Koordinaten, `openURL`, `hideKeyboard` und die Beschriftung `button` des Dialogs. Die Suite
  `examples/handrun-ios.yml` ist für die Installation des `.app`-Ordners geschrieben.

## Beispiel {#example}

Jeder Schritt unten hat eine Form, die die Tests dieses executors ausführen.

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

## Den Mac einrichten {#set-up-the-mac}

Verwenden Sie das, wenn venom auf einem anderen Rechner läuft (Windows, Linux) und der Emulator und der Simulator auf einem Mac laufen. Alles
Folgende wird auf dem Mac installiert. venom installiert nichts davon.

1. **Appium 3** mit beiden Treibern: `appium driver install uiautomator2` und `appium driver install xcuitest`. Starten Sie es so,
   dass es nur auf der Loopback-Adresse lauscht: `appium --address 127.0.0.1 --port 4723`. Setzen Sie `ANDROID_HOME` in der
   Shell, die es startet.
2. **Android:** das Android-SDK (platform-tools, `build-tools;34.0.0`) und auf einem Apple-Silicon-
   Mac ein **arm64**-Emulator-Image. Starten Sie den Emulator vor dem Lauf.
3. **iOS:** Xcode, seine Kommandozeilenwerkzeuge und mindestens ein Simulator. Booten Sie ihn, oder lassen Sie Appium ihn booten. Die erste Sitzung
   baut WebDriverAgent und dauert Minuten.
4. **Android-Web-Views:** ein chromedriver, der zur WebView auf dem Emulator passt, und sein Pfad in
   `appium:chromedriverExecutable` (siehe „Was die Web-View braucht"). iOS-Web-Views brauchen keinen.
5. **SSH:** ein Schlüssel, den der Mac für den in `mobile.ssh.user` angegebenen Benutzer akzeptiert. Passwort-Anmeldung wird nicht unterstützt.

Stolperfallen auf einem echten Mac:

- **Der erste Lauf eines Werkzeugs kann über SSH hängen bleiben.** macOS Gatekeeper verlangt womöglich eine Bestätigung, die in einer SSH-Sitzung
  niemand sieht, und die Binärdatei wartet. Führen Sie `flutter --version` (oder das betreffende Werkzeug) einmal in einem Terminal auf dem Mac selbst aus
  und beantworten Sie die Abfrage. Mit einem Homebrew-Cask installierte Werkzeuge können außerdem das Quarantäne-Flag tragen. Es mit
  `xattr -dr com.apple.quarantine <folder>` zu entfernen, ist Ihre Entscheidung: Tun Sie es nur für einen Ordner, dem Sie vertrauen.
- **Apps brauchen einen arm64-Slice** auf einem Apple-Silicon-Mac (siehe „Voraussetzungen auf dem Mac").
- **Flutter baut für den iOS-Simulator nur Debug-Apps.** Ein Flutter-Release-Build ist nur für Android möglich.
- **CocoaPods braucht ein UTF-8-Locale** (`export LANG=en_US.UTF-8 LC_ALL=en_US.UTF-8`), zum Beispiel wenn es aus einem
  über SSH gestarteten Skript läuft.

## Beispiel-Apps {#sample-apps}

Öffentliche Apps, die die Läufe des Maintainers verwendet haben. Jede ist arm64-tauglich.

| App | Plattform | Bezugsquelle |
|---|---|---|
| ApiDemos | Android | Appiums `ApiDemos-debug.apk`, aus den Releases von <https://github.com/appium/android-apidemos> |
| UIKitCatalog v4.0.5 | iOS-Simulator | `UIKitCatalog.zip`, aus den Releases von <https://github.com/appium/ios-uicatalog> |
| Flutter-Zähler-App | beide | `flutter create`, gebaut von `examples/build-sample-apps.sh` (Android-Release-APK, iOS-Simulator-Debug-App) |
| React-Native-App | beide | `react-native init`, gebaut vom selben Skript (Android-Release-APK, iOS-Release-Simulator-App) |

Das Skript läuft auf dem Mac. Es installiert das Flutter-SDK und CocoaPods, falls sie fehlen, baut die vier Apps aus den eigenen Vorlagen der
Frameworks und listet sie am Ende auf. Führen Sie zuerst einmal von Hand `flutter --version` aus (siehe „Den Mac einrichten").

Die Suites liegen in `examples/`: `examples/flutter.yml` und `examples/rn.yml` laufen unverändert auf beiden Plattformen, mit der Variablendatei
der jeweiligen Plattform (`examples/android.vars.example.yml`, `examples/ios.vars.example.yml`: Füllen Sie die `<...>`-Werte aus); `examples/failing.yml` schlägt
absichtlich fehl, um die Diagnosedateien zu zeigen. Die ausführlichen Lauf-Notizen werden nicht veröffentlicht.

## Die Tests ausführen {#run-the-tests}

Hermetische Tests (kein Gerät, kein Netzwerk; Teil von `go test ./...`):

```bash
go test ./executors/mobile/ -count=1
```

Echter Test, hinter dem Build-Tag `mobile`. Er braucht Appium 3 mit UiAutomator2 auf dem Loopback und einen Emulator, und er liest
seine Einstellungen aus der Umgebung. Er überspringt sich mit einer Meldung, wenn eine nicht gesetzt ist.

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

Er öffnet eine Sitzung, findet und liest das Element, prüft, dass ein erfolgreicher Schritt keine Diagnose hinterlässt, lässt absichtlich einen Schritt
fehlschlagen und prüft, dass `screen.png` und `screen.dump.xml` erscheinen, und schließt dann die Sitzung.

Zwei weitere echte Tests teilen sich die obigen Sitzungsvariablen (`VENOM_MOBILE_URL`, `_DEVICE`, `_APP` oder `_APPID`):

| Test | Zusätzliche Variablen | Was er prüft |
|---|---|---|
| `TestReal_CoordinatesAndDeepLink` | keine | prozentuale und pixelbasierte Wischgesten und ein Tippen bei 50 % / 50 % gelingen; `openURL venomtest://nothing/handles/this` kommt als nicht leeres `result.err` zurück |
| `TestReal_IOSSession` | `VENOM_MOBILE_PLATFORM=ios`, `VENOM_MOBILE_DEVICE` ein Simulatorname, `VENOM_MOBILE_APPID` eine Bundle-ID (oder `VENOM_MOBILE_APP`), `VENOM_MOBILE_IOS_LOCATOR` (Accessibility-ID auf dem ersten Bildschirm, Standard `General` für die App Settings) | `waitFor`, Wischen nach oben, `terminate`, `launch`, und `alert` ohne offenen Dialog antwortet `no alert open`; übersprungen, außer `VENOM_MOBILE_PLATFORM` ist `ios` |
| `TestReal_WebViewContext` | `VENOM_MOBILE_CHROMEDRIVER` (Pfad eines passenden chromedrivers auf dem Appium-Host) und `VENOM_MOBILE_WEBVIEW_LOCATOR` (Accessibility-ID, die einen Web-View-Bildschirm öffnet) | `context` erreicht eine `WEBVIEW...`, `css: body` zählt mehr als 0, `context: native` liefert `NATIVE_APP`; übersprungen, außer beide sind gesetzt |
