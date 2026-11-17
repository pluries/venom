---
translates: imap
title: IMAP
description: So verwenden Sie den IMAP-executor in einer venom-Test-Suite.
fingerprint: c71cd52180b824cdd8978dd27d199675ccde776d54bf93ad3fe5e475d251783f
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# Venom - Executor IMAP {#venom---executor-imap}

Anwendungsfall: Sie haben einen Mailserver und möchten IMAP-Befehle testen?

Der venom-IMAP-executor implementiert einige IMAP-Befehle wie FETCH, APPEND, MOVE, STORE...

## Testen {#how-to-test}
### Authentifizierung {#authentication}

```yaml
auth:
  withtls: false
  verifytls: false # Optional, used with withtls: true (see below)
  host: yourimaphost
  port: 143 # Most probably 993 if using TLS
  user: imap@venom.com
  password: imapvenompassword
  tls_root_ca: ca.pem # Optional, used with withtls: true and verifytls: true
  tls_client_cert: client.pem # Optional, used with withtls: true, together with tls_client_key
  tls_client_key: client.key # Optional, used with withtls: true, together with tls_client_cert
```

| Parameter | Beschreibung | Standardwert |
|---|---|---|
| auth.withtls | Optional. Verbindet sich ab dem ersten Byte über TLS (üblicherweise Port 993) | false |
| auth.verifytls | Optional, wird mit `withtls: true` verwendet. `false`: Das Serverzertifikat wird nicht geprüft, sodass ein Testserver mit selbstsigniertem Zertifikat akzeptiert wird. `true`: Das Zertifikat muss zu einem System-Root-Zertifikat oder zu `tls_root_ca` führen und für `host` gültig sein, sonst schlägt der Schritt fehl | false |
| auth.tls_root_ca | Optional, wird mit `withtls: true` verwendet. CA-Zertifikate, denen zusätzlich zu den System-Root-Zertifikaten vertraut wird, damit `verifytls: true` einen von einer privaten CA signierten Server akzeptiert. Mehrere CAs können aneinandergehängt werden. Sie wird nur zur Prüfung verwendet, wenn `verifytls: true` gilt | |
| auth.tls_client_cert | Optional, wird mit `withtls: true` verwendet. Das Client-Zertifikat (gegenseitiges TLS), zuerst das Blattzertifikat, danach die Zwischenzertifikate. Es wird auch gesendet, wenn `verifytls` auf `false` steht. Es benötigt `tls_client_key`: Ein Zertifikat ohne seinen Schlüssel lässt den Schritt fehlschlagen | |
| auth.tls_client_key | Optional, wird mit `withtls: true` verwendet. Der private Schlüssel von `tls_client_cert`. Er benötigt `tls_client_cert`: Ein Schlüssel ohne sein Zertifikat lässt den Schritt fehlschlagen | |

`tls_root_ca`, `tls_client_cert` und `tls_client_key` nehmen den PEM-Inhalt selbst oder den Pfad zu einer PEM-Datei entgegen, relativ
zum Verzeichnis der Test-Suite. Ohne `withtls: true` werden sie nicht verwendet: Die Verbindung bleibt unverschlüsselt, und eine Warnung
wird protokolliert (ein Zertifikat ohne seinen Schlüssel oder umgekehrt lässt den Schritt dennoch fehlschlagen).

Die Umgebungsvariable `SSL_CERT_FILE` oder `SSL_CERT_DIR` kann venom auf einen anderen Satz von System-Root-Zertifikaten verweisen.

### Befehle {#commands}

Derzeit stehen diese Befehle zur Verfügung: 
- **append**: eine neue Mail am Ende eines Postfachs erstellen
- **create**: ein neues Postfach erstellen
- **clear**: alle Nachrichten eines Postfachs auf dem Mailserver löschen
- **delete**: eine Nachricht löschen
- **fetch**: eine Mail abrufen
- **flag**: die Flags einer Mail hinzufügen, entfernen oder setzen
- **move**: eine Mail von einem Postfach in ein anderes verschieben

Mehrere Befehle können gleichzeitig angegeben werden: Sie werden in der Reihenfolge ausgeführt, in der sie unter dem Feld `commands` stehen.

Jeder Befehl besteht aus einem Feld `name`, mit dem der auszuführende Befehl angegeben wird.
Danach setzt er sich aus dem Feld `search` und/oder dem Feld `args` zusammen.

Das Feld `search` hat immer dieselbe Syntax. Es ruft die erste Mail ab, die über die Suchkriterien gefunden wird, und gibt sie zurück:
```yaml
search:
  mailbox: testBox # The mailbox in which to search the mail
  uid: 1 # The UID to search for
  from: .*@your-domain.localhost # The "From" header field to search for
  to: you@company.tld # The "To" header field to search for
  subject: Title of mail with * # The "Subject" header field to search for
  body: .*a body content.* # The "Body" field to search for
```
Damit legen wir fest, auf welche Mail der Befehl wirkt.
> ⚠️ Beachten Sie, dass die Suchkriterien reguläre Ausdrücke sind. Wenn Sie nach Sonderzeichen suchen möchten, vergessen Sie nicht, sie zu maskieren: \
> ❌ `subject: [IMPORTANT] READ THIS` \
> ✅ `subject: \[IMPORTANT\] READ THIS`

Das Feld `args`, oft mit dem Feld search kombiniert, legt die Argumente des Befehls fest.
Seine Syntax hängt vom Befehl ab.

In diesem Abschnitt beschreiben wir die Syntax jedes Befehls.

#### Der Befehl append {#append-command}

Der Befehl append erstellt eine neue Mail am Ende eines Postfachs.

```yaml
commands:
  - name: append
    args:
      mailbox: mailboxName # MANDATORY: The mailbox in which to create the mail
      from: origin-of@-my-new-mail.com # MANDATORY: the "From" header field of the new mail
      to: recipient-of@my-new-mail.com # OPTIONAL: the "To" header field of the new mail
      subject: Subject of my new mail # OPTIONAL: the "Subject" header field of the new mail
      body: Body of my new mail # OPTIONAL: the body of the new mail
      flags: # OPTIONAL: the flags of the new mail (note that the "\Recent" flag will most probably be added as well)
        - Flag1
        - Flag2
        ...
```

#### Der Befehl create {#create-command}

Der Befehl create erstellt ein neues Postfach.

```yaml
commands:
  - name: create
    args:
      mailbox: mailboxName # MANDATORY: The name of the new mailbox to create
```

#### Der Befehl clear {#clear-command}

Der Befehl clear löscht alle Nachrichten eines Postfachs. Er kann auf zwei Arten aufgerufen werden: 
- Erstens, indem alle zu leerenden Postfächer aufgelistet werden:
```yaml
commands:
  - name: clear
    args:
      mailboxes:
        - Mailbox1
        - Mailbox2
        ...
```
- Zweitens, indem das Platzhalter-Argument verwendet wird, um alle vorhandenen Postfächer zu leeren:
```yaml
commands:
  - name: clear
    args:
      mailboxes:
        - "*"
```

#### Der Befehl delete {#delete-command}

Der Befehl delete löscht eine über das Feld search abgerufene Nachricht endgültig.

```yaml
commands:
  - name: delete
    search:
      [...]
```

#### Der Befehl fetch {#fetch-command}

Der Befehl fetch ruft eine Nachricht über das Feld search ab.

```yaml
commands:
  - name: fetch
    search:
      [...]
```

#### Der Befehl flag {#flag-command}

Der Befehl flag ändert die Flags einer Nachricht. Er besteht aus den Feldern `search` und `args`.
Das Feld `args` kann aus bis zu 3 verschiedenen Feldern bestehen:
- **add**: Flags, die der Mail hinzugefügt werden
- **remove**: Flags, die von der Mail entfernt werden
- **set**: die zu setzenden Flags der Mail (überschreibt alle vorhandenen Flags)

Alle diese Felder können gleichzeitig angegeben werden, aber wenn `set` angegeben ist, wird nur dieses berücksichtigt.
Sie haben alle dieselbe Syntax.
```yaml
commands:
  - name: flag
    args:
      add:
        - Flag1
        - Flag2
        ...
      remove:
        - Flag1
        - Flag2
        ...
      set:
        - Flag1
        - Flag2
        ...
```

#### Der Befehl move {#move-command}

Der Befehl move verschiebt eine Mail von einem Postfach in ein anderes. Er besteht aus den Feldern `search` und `args`.


```yaml
commands:
  - name: move
    search:
      [...]
    args:
      mailbox: mailboxname # The mailbox to move the mail to
```

### Assertions {#assertions}

Jede Ausführung eines Befehls erzeugt ein `result`. Sein Inhalt hängt vom Befehl und seiner Ausführung ab.\
Da in einem einzelnen Testfall mehrere Befehle ausgeführt werden können, folgt das Objekt `result` dieser Syntax:
```json
{
  "result": {
    "commands": [
      {
        // Search represents the result of the command's search field
        "search": {
          "mailbox": "INBOX",
          "from": "from@mail-before-command-execution.com",
          "to": "to@mail-before-command-execution.com",
          "subject": "Title of mail BEFORE command execution",
          "body": "Body content of mail BEFORE command execution",
          "flags": [
            "Flag1",
            "Flag2"
          ]
        },
        // Mail represents the state of the searched mail after the command was executed
        "mail": { 
          "from": "from@mail-after-command-execution.com",
          "to": "to@mail-after-command-execution.com",
          "subject": "Title of mail AFTER command execution",
          "body": "Body content of mail AFTER command execution",
          "flags": [
            "Flag1",
            "Flag2",
            "Flag3"
          ]
        },
        "err": "Error of the command", // Err represents the command error message, supposing the command failed
        "timeseconds": 1.5 // TimeSeconds represents the duration of the command execution
      }
    ]
  }
}
```
Dabei hat jeder Befehl eigene Ergebnisse, die geprüft werden können.

Als Beispiel hier eine Liste möglicher Assertions nach der Ausführung von zwei Befehlen im selben Testfall:
```yaml
assertions:
  # First command
  - result.commands.commands0.err ShouldBeEmpty
  # State of the mail before command execution (search)
  - result.commands.commands0.search.from ShouldEqual "from@mail-before-command-execution.com"
  # Mail as a result of command execution
  - result.commands.commands0.mail.from ShouldEqual "to@mail-after-command-execution.com"
  # Second command
  - result.commands.commands1.err ShouldBeEmpty
  - result.commands.commands1.search.from ShouldEqual ...
  - result.commands.commands1.mail.from ShouldEqual ...
  - result.commands.commands1.mail.flags.flags0 ShouldEqual ...
  - result.commands.commands1.mail.flags.flags1 ShouldEqual ...
```

Bei Befehlen, die den Inhalt einer Mail (Header und Body) nicht ändern, wird das Feld `mail` nicht gefüllt.\
Als Beispiel hier das typische Ergebnis eines Befehls `fetch`, `delete` oder `move`:
```json
{
  "result": {
    "commands": [
      {
        "search": {
          "mailbox": "INBOX",
          "from": "from@mail-before-command-execution.com",
          "to": "to@mail.com",
          "subject": "Title of retrieved mail",
          "body": "Body content of retrieved mail",
          "flags": [
            "Flag1",
            "Flag2"
          ]
        },
        "err": "",
        "timeseconds": 1.5
      }
    ]
  }
}
```

Als weiteres Beispiel hier das typische Ergebnis eines Befehls `create` oder `clear`:
```json
{
  "result": {
    "commands": [
      {
        "search": {},
        "err": "",
        "timeseconds": 1.5
      }
    ]
  }
}
```

## Beispiel einer Test-Suite {#testsuite-example}

```yml
name: IMAP testsuite
vars:
  withTLS: false
  host: localhost
  port: 1143
  user: address@example.org
  password: pass
testcases:
  - name: Clear a mailbox
    steps:
      - type: imap
        auth:
          host: "{{.host}}"
          port: "{{.port}}"
          user: "{{.user}}"
          password: "{{.password}}"
        commands:
          - name: clear
            args:
              mailboxes:
                - INBOX
        assertions:
          # As multiple commands can be executed in a single testcase, we need to specify which command result we want to assert
          - result.commands.commands0.err ShouldBeEmpty
```

Weitere Beispiele finden Sie in der [Testdatei des IMAP-executors](../../tests/imap.yml).
