---
translates: imap
title: IMAP
description: Come usare l'executor IMAP in una test suite di venom.
fingerprint: c71cd52180b824cdd8978dd27d199675ccde776d54bf93ad3fe5e475d251783f
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# Venom - Executor IMAP {#venom---executor-imap}

Caso d'uso: hai un server di posta e vuoi testare i comandi IMAP?

L'executor IMAP di venom implementa alcuni comandi IMAP come FETCH, APPEND, MOVE, STORE...

## Come testare {#how-to-test}
### Autenticazione {#authentication}

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

| Parametro | Descrizione | Valore predefinito |
|---|---|---|
| auth.withtls | Facoltativo. Si connette via TLS fin dal primo byte (di solito porta 993) | false |
| auth.verifytls | Facoltativo, usato con `withtls: true`. `false`: il certificato del server non viene controllato, quindi un server di test con certificato autofirmato viene accettato. `true`: il certificato deve risalire a una root di sistema o a `tls_root_ca` ed essere valido per `host`, altrimenti lo step fallisce | false |
| auth.tls_root_ca | Facoltativo, usato con `withtls: true`. Certificati CA considerati attendibili oltre alle root di sistema, in modo che `verifytls: true` accetti un server firmato da una CA privata. Si possono concatenare più CA. Viene usato per la verifica solo quando `verifytls: true` | |
| auth.tls_client_cert | Facoltativo, usato con `withtls: true`. Il certificato client (TLS reciproco), prima il certificato foglia, poi gli intermedi. Viene inviato anche quando `verifytls` è `false`. Richiede `tls_client_key`: un certificato senza la sua chiave fa fallire lo step | |
| auth.tls_client_key | Facoltativo, usato con `withtls: true`. La chiave privata di `tls_client_cert`. Richiede `tls_client_cert`: una chiave senza il suo certificato fa fallire lo step | |

`tls_root_ca`, `tls_client_cert` e `tls_client_key` accettano il contenuto PEM stesso oppure il percorso di un file PEM, relativo
alla directory della test suite. Senza `withtls: true` non vengono usati: la connessione resta in chiaro e viene registrato
un avviso (un certificato senza la sua chiave, o viceversa, fa comunque fallire lo step).

La variabile d'ambiente `SSL_CERT_FILE` o `SSL_CERT_DIR` può indirizzare venom verso un altro insieme di root di sistema.

### Comandi {#commands}

Allo stato attuale, i comandi disponibili sono questi: 
- **append**: crea una nuova mail in fondo a una casella
- **create**: crea una nuova casella
- **clear**: elimina dal server di posta tutti i messaggi di una casella
- **delete**: elimina un messaggio
- **fetch**: recupera una mail
- **flag**: aggiunge, rimuove o imposta i flag di una mail
- **move**: sposta una mail da una casella a un'altra

Si possono indicare più comandi alla volta: vengono eseguiti nell'ordine in cui compaiono nel campo `commands`.

Ogni comando è formato da un campo `name` che permette di specificare il comando da eseguire.
Poi è composto dal campo `search` e/o dal campo `args`.

Il campo `search` ha sempre la stessa sintassi. Recupera e restituisce la prima mail trovata con i criteri di ricerca:
```yaml
search:
  mailbox: testBox # The mailbox in which to search the mail
  uid: 1 # The UID to search for
  from: .*@your-domain.localhost # The "From" header field to search for
  to: you@company.tld # The "To" header field to search for
  subject: Title of mail with * # The "Subject" header field to search for
  body: .*a body content.* # The "Body" field to search for
```
Permette di specificare su quale mail agirà il comando.
> ⚠️ Attenzione: i criteri di ricerca sono espressioni regolari. Se cerchi caratteri speciali, ricorda di fare l'escape: \
> ❌ `subject: [IMPORTANT] READ THIS` \
> ✅ `subject: \[IMPORTANT\] READ THIS`

Spesso combinato con il campo search, il campo `args` permette di specificare gli argomenti del comando.
La sua sintassi dipende dal comando.

In questa sezione descriveremo la sintassi di ogni comando.

#### Comando append {#append-command}

Il comando append crea una nuova mail in fondo a una casella.

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

#### Comando create {#create-command}

Il comando create crea una nuova casella.

```yaml
commands:
  - name: create
    args:
      mailbox: mailboxName # MANDATORY: The name of the new mailbox to create
```

#### Comando clear {#clear-command}

Il comando clear elimina tutti i messaggi di una casella. Si può richiamare in due modi diversi: 
- Il primo, elencando tutte le caselle da svuotare:
```yaml
commands:
  - name: clear
    args:
      mailboxes:
        - Mailbox1
        - Mailbox2
        ...
```
- Il secondo, usando l'argomento jolly per svuotare tutte le caselle esistenti:
```yaml
commands:
  - name: clear
    args:
      mailboxes:
        - "*"
```

#### Comando delete {#delete-command}

Il comando delete elimina definitivamente un messaggio recuperato tramite il campo search.

```yaml
commands:
  - name: delete
    search:
      [...]
```

#### Comando fetch {#fetch-command}

Il comando fetch recupera un messaggio tramite il campo search.

```yaml
commands:
  - name: fetch
    search:
      [...]
```

#### Comando flag {#flag-command}

Il comando flag modifica i flag di un messaggio. È composto sia dal campo `search` sia dal campo `args`.
Il campo `args` può contenere fino a 3 campi diversi:
- **add**: i flag da aggiungere alla mail
- **remove**: i flag da rimuovere dalla mail
- **set**: i flag da impostare sulla mail (sovrascrive qualsiasi flag esistente)

Si possono indicare tutti questi campi insieme ma, se è specificato, viene considerato solo `set`.
Hanno tutti la stessa sintassi.
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

#### Comando move {#move-command}

Il comando move sposta una mail da una casella a un'altra. È composto sia dal campo `search` sia dal campo `args`.


```yaml
commands:
  - name: move
    search:
      [...]
    args:
      mailbox: mailboxname # The mailbox to move the mail to
```

### Assertion {#assertions}

Ogni esecuzione di un comando produce un `result`. Il suo contenuto dipende dal comando e dalla sua esecuzione.\
Poiché in un singolo test case si possono eseguire più comandi, l'oggetto `result` segue questa sintassi:
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
Ogni comando ha i propri risultati, su cui si possono scrivere assertion.

Come esempio, ecco un elenco di possibili assertion dopo l'esecuzione di due comandi nello stesso test case:
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

I comandi che non modificano il contenuto di una mail (intestazioni e corpo) non avranno il campo `mail` valorizzato.\
Come esempio, ecco il risultato tipico di un comando `fetch`, `delete` o `move`:
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

Come altro esempio, ecco il risultato tipico di un comando `create` o `clear`:
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

## Esempio di test suite {#testsuite-example}

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

Per vedere altri esempi, consulta il [file di test dell'executor IMAP](../../tests/imap.yml).
