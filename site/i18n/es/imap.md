---
translates: imap
title: IMAP
description: Cómo usar el executor IMAP en una suite de pruebas de venom.
fingerprint: c71cd52180b824cdd8978dd27d199675ccde776d54bf93ad3fe5e475d251783f
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# Venom - Executor IMAP {#venom---executor-imap}

Caso de uso: ¿tiene un servidor de correo y quiere probar comandos IMAP?

El executor IMAP de venom implementa algunos comandos IMAP como FETCH, APPEND, MOVE, STORE...

## Cómo probar {#how-to-test}
### Autenticación {#authentication}

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

| Parámetro | Descripción | Valor por defecto |
|---|---|---|
| auth.withtls | Opcional. Conecta mediante TLS desde el primer byte (normalmente el puerto 993) | false |
| auth.verifytls | Opcional, se usa con `withtls: true`. `false`: no se comprueba el certificado del servidor, de modo que se acepta un servidor de pruebas con certificado autofirmado. `true`: el certificado debe encadenarse a una raíz del sistema o a `tls_root_ca` y ser válido para `host`; de lo contrario, el paso falla | false |
| auth.tls_root_ca | Opcional, se usa con `withtls: true`. Certificados de CA en los que se confía además de las raíces del sistema, para que `verifytls: true` acepte un servidor firmado por una CA privada. Se pueden concatenar varias CA. Solo se usa para verificar cuando `verifytls: true` | |
| auth.tls_client_cert | Opcional, se usa con `withtls: true`. El certificado de cliente (TLS mutuo), primero el certificado hoja, seguido de los intermedios. Se envía incluso cuando `verifytls` es `false`. Requiere `tls_client_key`: un certificado sin su clave hace fallar el paso | |
| auth.tls_client_key | Opcional, se usa con `withtls: true`. La clave privada de `tls_client_cert`. Requiere `tls_client_cert`: una clave sin su certificado hace fallar el paso | |

`tls_root_ca`, `tls_client_cert` y `tls_client_key` admiten el contenido PEM en sí o la ruta a un archivo PEM, relativa
al directorio de la suite de pruebas. Sin `withtls: true` no se usan: la conexión sigue en texto plano y se registra una advertencia
(un certificado sin su clave, o a la inversa, sigue haciendo fallar el paso).

La variable de entorno `SSL_CERT_FILE` o `SSL_CERT_DIR` permite indicar a venom otro conjunto de raíces del sistema.

### Comandos {#commands}

Hoy, estos son los comandos disponibles: 
- **append**: crea un correo nuevo al final de un buzón
- **create**: crea un buzón nuevo
- **clear**: elimina del servidor de correo todos los mensajes de un buzón
- **delete**: elimina un mensaje
- **fetch**: recupera un correo
- **flag**: agrega, quita o establece las marcas de un correo
- **move**: mueve un correo de un buzón a otro

Se pueden indicar varios comandos a la vez: se ejecutan en el orden en que aparecen en el campo `commands`.

Cada comando consta de un campo `name` que permite especificar el comando que se ejecutará.
Después se compone del campo `search` y/o del campo `args`.

El campo `search` siempre tiene la misma sintaxis. Recupera y devuelve el primer correo encontrado según los criterios de búsqueda:
```yaml
search:
  mailbox: testBox # The mailbox in which to search the mail
  uid: 1 # The UID to search for
  from: .*@your-domain.localhost # The "From" header field to search for
  to: you@company.tld # The "To" header field to search for
  subject: Title of mail with * # The "Subject" header field to search for
  body: .*a body content.* # The "Body" field to search for
```
Permite especificar sobre qué correo actuará el comando.
> ⚠️ Tenga cuidado: los criterios de búsqueda son expresiones regulares. Si desea buscar caracteres especiales, no olvide escaparlos: \
> ❌ `subject: [IMPORTANT] READ THIS` \
> ✅ `subject: \[IMPORTANT\] READ THIS`

A menudo combinado con el campo search, el campo `args` permite especificar los argumentos del comando.
Su sintaxis depende del comando.

En esta sección se describe la sintaxis de cada comando.

#### Comando append {#append-command}

El comando append crea un correo nuevo al final de un buzón.

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

El comando create crea un buzón nuevo.

```yaml
commands:
  - name: create
    args:
      mailbox: mailboxName # MANDATORY: The name of the new mailbox to create
```

#### Comando clear {#clear-command}

El comando clear elimina todos los mensajes de un buzón. Se puede llamar de dos maneras: 
- La primera, enumerando cada buzón que se vaciará:
```yaml
commands:
  - name: clear
    args:
      mailboxes:
        - Mailbox1
        - Mailbox2
        ...
```
- La segunda, usando el argumento comodín para vaciar todos los buzones existentes:
```yaml
commands:
  - name: clear
    args:
      mailboxes:
        - "*"
```

#### Comando delete {#delete-command}

El comando delete elimina de forma permanente un mensaje recuperado mediante el campo search.

```yaml
commands:
  - name: delete
    search:
      [...]
```

#### Comando fetch {#fetch-command}

El comando fetch recupera un mensaje mediante el campo search.

```yaml
commands:
  - name: fetch
    search:
      [...]
```

#### Comando flag {#flag-command}

El comando flag modifica las marcas de un mensaje. Se compone de los campos `search` y `args`.
El campo `args` puede constar de hasta 3 campos distintos:
- **add**: marcas que se agregan al correo
- **remove**: marcas que se quitan del correo
- **set**: las marcas del correo que se establecen (sustituyen cualquier marca existente)

Todos estos campos se pueden usar a la vez pero, si se especifica, solo se tiene en cuenta `set`.
Todos tienen la misma sintaxis.
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

El comando move mueve un correo de un buzón a otro. Se compone de los campos `search` y `args`.


```yaml
commands:
  - name: move
    search:
      [...]
    args:
      mailbox: mailboxname # The mailbox to move the mail to
```

### Aserciones {#assertions}

Cada ejecución de un comando produce un `result`. Su contenido depende del comando y de su ejecución.\
Como se pueden ejecutar varios comandos en un mismo caso de prueba, el objeto `result` sigue esta sintaxis:
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
Cada comando tiene sus propios resultados, sobre los que se puede hacer una aserción.

Como ejemplo, esta es una lista de posibles aserciones tras ejecutar dos comandos en el mismo caso de prueba:
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

Los comandos que no modifican el contenido de un correo (encabezados y cuerpo) no tendrán relleno el campo `mail`.\
Como ejemplo, este es el resultado típico de un comando `fetch`, `delete` o `move`:
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

Como otro ejemplo, este es el resultado típico de un comando `create` o `clear`:
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

## Ejemplo de suite de pruebas {#testsuite-example}

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

Para ver más ejemplos, consulte el [archivo de pruebas del executor IMAP](../../tests/imap.yml).

