---
translates: recipes
title: Rezepte
description: Rezepte für Web und Mobile aufbauen, benennen, prüfen und absichern.
fingerprint: 3dd31333d7ad4d4487d1c297fdc7e02dc69595aee1619af22617d3bfb867d5d5
drafted_by: claude
drafted_on: 2026-11-18
status: machine-drafted
---
Ein Rezept ist eine venom-Test-Suite, die eine Webseite oder eine mobile App steuert. Diese Seite zeigt, wie Sie Rezepte anlegen, benennen, beweisen, dass sie etwas testen, und wie Sie Geheimnisse und Ergebnisse von den falschen Orten fernhalten. Sie brauchen venom und, für Web-Rezepte, einen Browser, den Playwright installiert hat. Die Seiten zum [Web-Executor](web.html) und zum [Mobile-Executor](mobile.html) listen jede Aktion auf.

Ein Rezept ist eine gewöhnliche Suite. Es verwendet `web`-Schritte oder `mobile`-Schritte und nichts Neues. Hier ist ein kleines Web-Rezept. Die Adresse der Seite ist die Variable `appUrl`, sodass dieselbe Datei gegen jede Kopie der Seite läuft:

{{include site/examples/recipes/web-page.yml yaml}}

Führen Sie es mit der Adresse Ihrer Seite aus:

```
venom run --var appUrl=http://localhost:8000 web-page.yml
```

## Ordneraufbau {#folder-layout}

Legen Sie die Rezepte einer Funktion in einen Ordner unter `tests/`, neben die anderen venom-Suites. Zum Beispiel:

```
tests/
  site/
    site-languages.yml
  recipes/
    reference/
      mobile/
        counter.yml
```

Ein Ordner pro Funktion. Darin eine oder mehrere `.yml`-Dateien. Ein Ordner mit Unterordnern wird geprüft, indem Sie die Unterordner nennen. Halten Sie Variablendateien mit Zugangsdaten außerhalb des Ordners oder von git ignoriert (siehe Zugangsdaten).

## Testfall-Namen und die Spec-ID {#testcase-names-and-the-spec-id}

Jeder Testfall belegt ein Akzeptanzszenario einer Spec. Beginnen Sie den Namen des Testfalls mit der Szenario-ID, danach eine kurze Beschreibung: `US1-AS3 an item typed in the input is added to the list`. Die ID hat die Form `USn-ASm`: User Story `n`, Akzeptanzszenario `m`.

Nennen Sie die Spec einmal, in einer Variable der Suite:

```
vars:
  spec: 014-web-mobile-recipes
```

Der Wert ist der Ordnername der Spec. Die Rezeptprüfung liest `spec.md` in diesem Ordner und schlägt die IDs nach. Die Option `--specs-dir` sagt ihr, wo die Spec-Ordner liegen. Zwei Dateien eines Ordners dürfen nicht dasselbe Szenario beanspruchen.

Um zu sehen, welche Szenarien ein Rezept beweist, fügen Sie `--coverage` mit der Spec-ID hinzu: `venom recipe check --specs-dir <specs folder> --coverage 014-web-mobile-recipes tests/recipes`. Nach den Befunden listet die Prüfung jedes Szenario der Spec auf, `US1-AS3 proven: <file> / <testcase>` oder `US1-AS4 not proven`. Ein Testfall beweist ein Szenario nur, wenn er nicht übersprungen wird und eine Assertion enthält, die die App liest. Die Option ändert den Exit-Status nicht. Das Coverage-Skript des Dev-Sets liest keine `.yml`-Dateien; verwenden Sie daher diese Option und nicht jenes Skript, um Rezepte zu zählen.

## Selektoren {#selectors}

Ein Schritt `find`, `click` oder `fill` nimmt einen Selektor. Verwenden Sie CSS oder XPath. Bevorzugen Sie ein stabiles Testattribut, etwa `data-test`:

```
find: '[data-test="add-button"]'
```

IDs und Klassennamen ändern sich, wenn jemand die Seite neu gestaltet. Ein Attribut `data-test` existiert nur, damit Tests das Element finden. Hat die App keines, bitten Sie die Entwickler, es hinzuzufügen, bevor Sie das Rezept schreiben.

`find` wartet nicht. Es zählt die Elemente, die jetzt existieren. Lädt die Seite Daten nach dem ersten Zeichnen, warten Sie zuerst mit `wait` oder verwenden Sie `click` oder `fill`, die auf ihr Element warten. Setzen Sie in einem mobilen Schritt genau einen Locator, etwa `accessibilityId`.

## Beweisschritte {#proof-steps}

Ein Rezept, das besteht, beweist nichts, bis Sie es haben scheitern sehen. Drei Schritte liefern den Beweis:

1. Wiederholung. Führen Sie das Rezept mit `venom run` gegen die App aus. Jeder Testfall muss bestehen. Wiederholen Sie es einige Male: Ein Testfall, der einmal besteht und einmal scheitert, ist nicht brauchbar.
2. Negativkontrolle. Kopieren Sie das Rezept für jeden Testfall, ändern Sie den ersten erwarteten Wert, der aus der App stammt, und führen Sie die Kopie aus. Der Lauf muss mit Status 2 enden und diesen Testfall nennen. Lassen Sie die Originaldatei unberührt.
3. Rezeptprüfung. Führen Sie die Prüfung aus, die die Dateien ohne Browser oder Gerät liest:

```
venom recipe check --specs-dir <folder of the specs> tests/site
```

Die Prüfung endet mit 0, wenn sie nichts findet, mit 1, wenn sie einen Befund meldet, und mit 2, wenn sie ihre Eingabe nicht lesen kann. Jeder Befund nennt die Datei, die Zeile, die Regel und den Testfall. Sie meldet einen Testfall ohne Szenario-ID, eine wiederholte ID, eine ID, die die Spec nicht hat, einen Testfall ohne Assertion auf einen aus der App gelesenen Wert, eine Assertion, die nicht scheitern kann, einen zugangsdatenähnlichen Text und eine feste Adresse.

Die Prüfung ist statisch. Sie kann nicht in ein `execute`-Skript hineinlesen und weiß nicht, ob ein erwarteter Wert richtig ist. Die Negativkontrolle deckt beides ab.

## Was die Executors nicht können {#what-the-executors-cannot-do}

Der Web-Executor hat keinen Schritt für Hover, Ziehen oder Tastenkürzel, und `find` kann keine Zahl wie eine Scrollbreite lesen. Die Aktion `execute` führt ein Skript in der Seite aus und gibt einen Wert zurück, auf den Sie prüfen können. Dieses Beispiel liest, wie weit die Seite seitlich scrollt und wie viele Einträge sie auflistet:

{{include site/examples/recipes/web-execute.yml yaml}}

Reicht `execute` nicht aus, halten Sie die Lücke schriftlich fest und schreiben Sie für dieses Szenario kein Rezept. Legen Sie nicht alle Assertions eines Testfalls ohne Negativkontrolle auf ein Skriptergebnis: Die Prüfung kann das Skript nicht lesen.

Ein mobiles Rezept braucht einen Appium-Server und ein Gerät oder einen Emulator. Ohne sie läuft es nicht. Dieses hier trägt in seiner ersten Zeile die Markierung `# needs: device`, damit die Tests der Seite es dort überspringen, wo kein Gerät existiert:

{{include site/examples/recipes/mobile-counter.yml yaml}}

## Zugangsdaten {#credentials}

Ein Rezept enthält kein Passwort, kein Token und keinen Schlüssel. Eine Anmeldung kommt aus Variablen. Übergeben Sie sie auf der Kommandozeile oder in einer Variablendatei, die git ignoriert:

```
venom run --var-from-file login.vars.yml tests/site/login.yml
```

Schreiben Sie im Rezept die Variable: `text: "{{.password}}"`. Die Prüfung meldet einen zugangsdatenähnlichen Text in jedem Schritt oder Standardwert.

Rezepte laufen gegen eine Testumgebung, nie gegen die Produktion. Die Adresse ist eine Variable, die die Pipeline setzt, und `venom run` hindert Sie nicht daran, sie auf ein echtes System zu richten. Führen Sie die Liste der erlaubten Hosts in Ihrer Pipeline.

## Ergebnisse und Artefakte {#results-and-artifacts}

Ein fehlgeschlagener Schritt kann einen Seitenabzug, einen Screenshot oder einen Trace schreiben. Diese Dateien können Daten aus der App enthalten. Schreiben Sie sie in einen Ergebnisordner, den git ignoriert, und committen Sie sie nicht. Bewahren Sie sie in der CI nur kurz auf, etwa einige Tage, und geben Sie nur den Personen Zugriff, die ihn brauchen: Der Speicher ist beschränkt. Kopieren Sie sie nicht in langlebigen Speicher.

## Nicht definierte Variablen {#undefined-variables}

Eine Variable, die Sie nicht übergeben, stoppt den Lauf standardmäßig nicht. Venom lässt `{{.password}}` als den wörtlichen Text `{{.password}}` stehen, und der Schritt tippt diesen Text. Der Lauf scheitert deswegen nicht.

Übergeben Sie `--strict-variables`, um das zu ändern. Venom stoppt dann mit Exit-Status 2 und der Meldung `unknown variable(s) "password"`. Verwenden Sie die Option in jeder Pipeline, die Rezepte ausführt:

```
venom run --strict-variables --var-from-file login.vars.yml tests/site/login.yml
```
