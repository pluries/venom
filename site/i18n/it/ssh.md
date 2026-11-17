---
translates: ssh
title: SSH
description: Come usare l'executor SSH in una test suite di venom.
fingerprint: 44c8a00c687f093c007b2632c39692ac92400c71c058513ec8972df353563c5d
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# Venom - Executor SSH {#venom---executor-ssh}

Step per eseguire uno script su un server remoto via SSH


## Input {#input}

Nel tuo file YAML puoi usare:

```yaml
  - host mandatory
  - command mandatory
  - user optional (default is OS username)
  - password optional (mandatory if no privatekey is found)
  - privatekey optional (default is $HOME/.ssh/id_rsa)
  - sudo optional
  - sudopassword optional (default to password)
  - knownhosts optional (path of an OpenSSH known_hosts file the host's key must be listed in; exclusive with hostkey)
  - hostkey optional (the one public key the host must present, in authorized_keys format; exclusive with knownhosts)
```

**Controllo della chiave dell'host.** Per impostazione predefinita non sono impostati né `knownhosts` né `hostkey`, e la chiave
dell'host **non viene controllata**: qualsiasi server viene accettato. È il comportamento dell'executor da prima che
esistessero le due chiavi. Va bene per host di test effimeri, la cui chiave cambia a ogni esecuzione. Imposta una delle due
chiavi per controllare l'host:

- `knownhosts`: un file nel formato di `~/.ssh/known_hosts`, per esempio l'output di `ssh-keyscan -p 2222 192.0.2.10`. Un
  percorso relativo viene letto a partire dalla directory della test suite; un percorso assoluto viene usato così com'è. Lo step
  fallisce se l'host non è nel file o presenta un'altra chiave. Il file può anche fidarsi di una certificate authority con una riga come
  `@cert-authority [192.0.2.10]:2222 ssh-ed25519 AAAAC3Nza…`: un host che presenta un certificato host firmato da quella
  authority supera il controllo, a condizione che i principal del certificato, quando ne elenca, includano l'host così come è scritto
  in `host` (senza la porta).
- `hostkey`: una riga come `ssh-ed25519 AAAAC3Nza… [comment]`, per esempio il contenuto del file
  `/etc/ssh/ssh_host_ed25519_key.pub` dell'host. Lo step fallisce se l'host presenta un'altra chiave.

L'executor chiede all'host una chiave del tipo che hai indicato, oppure un certificato host quando il file elenca una
authority per quell'host. Un host con più tipi di chiave (RSA, ECDSA, Ed25519) supera quindi il controllo se il file o
`hostkey` contiene una qualsiasi delle sue chiavi. Impostare entrambe le chiavi, un file che non si può leggere o un `hostkey` che non è una
chiave pubblica fa fallire lo step prima della connessione. Lo stesso vale per un `knownhosts` o un `hostkey` impostato a un valore vuoto, come una variabile
che si espande in niente: un valore vuoto è un errore, non equivale a omettere la chiave, quindi non può disattivare
il controllo. Come per un errore di connessione, il motivo si trova in `result.err`.

Esempio

```yaml

name: Title of TestSuite
testcases:
- name: Check if exit code != 1 and echo command response in less than 1s
  steps:
  - type: ssh
    host: localhost:2222
    command: echo 'foo'
    assertions:
    - result.code ShouldEqual 0
    - result.timeseconds ShouldBeLessThan 1

- name: Use specific privatekey
  steps:
  - type: ssh
    host: 192.0.2.10:2222
    command: echo 'foo'
    user: bar
    privatekey: /home/foo/.ssh/id_rsa
    assertions:
    - result.code ShouldEqual 0

- name: Execute command as another user than bar
  steps:
  - type: ssh
    host: 192.0.2.10:2222
    command: echo 'foo'
    user: bar
    sudo: root
    sudopassword: '{{.mypassword}}'
    assertions:
    - result.code ShouldEqual 0

- name: Check the host's key against a known_hosts file next to the suite
  steps:
  - type: ssh
    host: 192.0.2.10:2222
    command: echo 'foo'
    user: bar
    knownhosts: ssh/known_hosts
    assertions:
    - result.code ShouldEqual 0

- name: Pin the host's key
  steps:
  - type: ssh
    host: 192.0.2.10:2222
    command: echo 'foo'
    user: bar
    hostkey: '{{.hostkey}}'
    assertions:
    - result.code ShouldEqual 0

```
*NB: l'opzione sudo usa uno pseudotty*

## Output {#output}

```yaml
systemout
systemerr
err
code
timeseconds
```

- result.timeseconds: tempo di esecuzione
- result.err: se esiste, questo campo contiene l'errore
- result.systemout: standard output dello script eseguito
- result.systemerr: output di errore dello script eseguito
- result.code: exit code

## Assertion predefinita {#default-assertion}

```yaml
result.code ShouldEqual 0
```
