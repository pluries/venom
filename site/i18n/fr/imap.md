---
translates: imap
title: IMAP
description: Comment utiliser l'executor IMAP dans une suite de tests venom.
fingerprint: c71cd52180b824cdd8978dd27d199675ccde776d54bf93ad3fe5e475d251783f
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# Venom - Executor IMAP {#venom---executor-imap}

Cas d'usage : vous avez un serveur de messagerie et vous voulez tester des commandes IMAP ?

L'executor IMAP de venom implémente quelques commandes IMAP, comme FETCH, APPEND, MOVE, STORE...

## Comment tester {#how-to-test}
### Authentification {#authentication}

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

| Paramètre | Description | Valeur par défaut |
|---|---|---|
| auth.withtls | Facultatif. Se connecte en TLS dès le premier octet (en général le port 993) | false |
| auth.verifytls | Facultatif, utilisé avec `withtls: true`. `false` : le certificat du serveur n'est pas vérifié, donc un serveur de test auto-signé est accepté. `true` : le certificat doit se rattacher à une racine du système ou à `tls_root_ca` et être valide pour `host`, sinon l'étape échoue | false |
| auth.tls_root_ca | Facultatif, utilisé avec `withtls: true`. Certificats d'autorité de certification (CA) approuvés en plus des racines du système, afin que `verifytls: true` accepte un serveur signé par une CA privée. Plusieurs CA peuvent être concaténées. Il ne sert à vérifier que si `verifytls: true` | |
| auth.tls_client_cert | Facultatif, utilisé avec `withtls: true`. Le certificat client (TLS mutuel), le certificat feuille en premier, suivi des certificats intermédiaires. Il est envoyé même quand `verifytls` vaut `false`. Il exige `tls_client_key` : un certificat sans sa clé fait échouer l'étape | |
| auth.tls_client_key | Facultatif, utilisé avec `withtls: true`. La clé privée de `tls_client_cert`. Elle exige `tls_client_cert` : une clé sans son certificat fait échouer l'étape | |

`tls_root_ca`, `tls_client_cert` et `tls_client_key` prennent le contenu PEM lui-même ou le chemin d'un fichier PEM, relatif
au répertoire de la suite de tests. Sans `withtls: true`, ils ne sont pas utilisés : la connexion reste en clair et un avertissement
est journalisé (un certificat sans sa clé, ou l'inverse, fait quand même échouer l'étape).

La variable d'environnement `SSL_CERT_FILE` ou `SSL_CERT_DIR` permet d'indiquer à venom un autre jeu de racines du système.

### Commandes {#commands}

À ce jour, les commandes disponibles sont les suivantes : 
- **append** : crée un nouveau message à la fin d'une boîte aux lettres
- **create** : crée une nouvelle boîte aux lettres
- **clear** : supprime du serveur de messagerie tous les messages d'une boîte aux lettres
- **delete** : supprime un message
- **fetch** : récupère un message
- **flag** : ajoute, retire ou définit les indicateurs (flags) d'un message
- **move** : déplace un message d'une boîte aux lettres vers une autre

Plusieurs commandes peuvent être indiquées à la fois : elles sont exécutées dans l'ordre où elles figurent sous le champ `commands`.

Chaque commande comporte un champ `name` qui précise la commande à exécuter.
Elle est ensuite composée du champ `search` et/ou du champ `args`.

Le champ `search` a toujours la même syntaxe. Il récupère et renvoie le premier message trouvé selon les critères de recherche :
```yaml
search:
  mailbox: testBox # The mailbox in which to search the mail
  uid: 1 # The UID to search for
  from: .*@your-domain.localhost # The "From" header field to search for
  to: you@company.tld # The "To" header field to search for
  subject: Title of mail with * # The "Subject" header field to search for
  body: .*a body content.* # The "Body" field to search for
```
Il permet de préciser sur quel message la commande agit.
> ⚠️ Attention, les critères de recherche sont des expressions régulières. Si vous cherchez des caractères spéciaux, n'oubliez pas de les échapper : \
> ❌ `subject: [IMPORTANT] READ THIS` \
> ✅ `subject: \[IMPORTANT\] READ THIS`

Souvent combiné au champ search, le champ `args` permet de préciser les arguments de la commande.
Sa syntaxe dépend de la commande.

Dans cette section, nous décrivons la syntaxe de chaque commande.

#### Commande append {#append-command}

La commande append crée un nouveau message à la fin d'une boîte aux lettres.

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

#### Commande create {#create-command}

La commande create crée une nouvelle boîte aux lettres.

```yaml
commands:
  - name: create
    args:
      mailbox: mailboxName # MANDATORY: The name of the new mailbox to create
```

#### Commande clear {#clear-command}

La commande clear supprime tous les messages d'une boîte aux lettres. Elle peut être appelée de deux façons : 
- D'abord, en listant chaque boîte aux lettres à vider :
```yaml
commands:
  - name: clear
    args:
      mailboxes:
        - Mailbox1
        - Mailbox2
        ...
```
- Ensuite, en utilisant l'argument joker pour vider toutes les boîtes aux lettres existantes :
```yaml
commands:
  - name: clear
    args:
      mailboxes:
        - "*"
```

#### Commande delete {#delete-command}

La commande delete supprime définitivement un message récupéré par le champ search.

```yaml
commands:
  - name: delete
    search:
      [...]
```

#### Commande fetch {#fetch-command}

La commande fetch récupère un message par le champ search.

```yaml
commands:
  - name: fetch
    search:
      [...]
```

#### Commande flag {#flag-command}

La commande flag modifie les indicateurs (flags) d'un message. Elle se compose des champs `search` et `args`.
Le champ `args` peut contenir jusqu'à 3 champs différents :
- **add** : les indicateurs à ajouter au message
- **remove** : les indicateurs à retirer du message
- **set** : les indicateurs à définir sur le message (remplace tout indicateur existant)

Tous ces champs peuvent être utilisés en même temps mais, s'il est indiqué, seul `set` est pris en compte.
Ils ont tous la même syntaxe.
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

#### Commande move {#move-command}

La commande move déplace un message d'une boîte aux lettres vers une autre. Elle se compose des champs `search` et `args`.


```yaml
commands:
  - name: move
    search:
      [...]
    args:
      mailbox: mailboxname # The mailbox to move the mail to
```

### Assertions {#assertions}

Chaque exécution d'une commande produit un `result`. Son contenu dépend de la commande et de son exécution.\
Comme plusieurs commandes peuvent être exécutées dans un même cas de test, l'objet `result` suit cette syntaxe :
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
Chaque commande a ses propres résultats, sur lesquels on peut poser des assertions.

À titre d'exemple, voici une liste d'assertions possibles après l'exécution de deux commandes dans le même cas de test :
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

Les commandes qui ne modifient pas le contenu d'un message (en-têtes et corps) ne renseignent pas le champ `mail`.\
À titre d'exemple, voici le résultat typique d'une commande `fetch`, `delete` ou `move` :
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

Autre exemple, voici le résultat typique d'une commande `create` ou `clear` :
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

## Exemple de testsuite {#testsuite-example}

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

Pour d'autres exemples, consultez le [fichier de test de l'executor IMAP](../../tests/imap.yml).
