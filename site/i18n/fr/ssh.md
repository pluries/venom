---
translates: ssh
title: SSH
description: Comment utiliser l'executor SSH dans une testsuite venom.
fingerprint: 44c8a00c687f093c007b2632c39692ac92400c71c058513ec8972df353563c5d
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# Venom - Executor SSH {#venom---executor-ssh}

Étape qui exécute un script sur un serveur distant via SSH


## Entrée {#input}

Dans votre fichier YAML, vous pouvez utiliser :

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

**Vérification de la clé d'hôte.** Par défaut, ni `knownhosts` ni `hostkey` n'est renseigné, et la clé de l'hôte n'est **pas vérifiée** :
n'importe quel serveur est accepté. C'est le comportement de l'executor d'avant l'existence de ces deux clés. Il convient aux
hôtes de test éphémères dont la clé change à chaque exécution. Renseignez l'une des deux clés pour vérifier l'hôte :

- `knownhosts` : un fichier au format de `~/.ssh/known_hosts`, par exemple la sortie de `ssh-keyscan -p 2222 192.0.2.10`. Un
  chemin relatif est lu depuis le répertoire de la testsuite ; un chemin absolu est utilisé tel quel. L'étape échoue si l'hôte
  n'est pas dans le fichier ou présente une autre clé. Le fichier peut aussi faire confiance à une autorité de certification avec une ligne comme
  `@cert-authority [192.0.2.10]:2222 ssh-ed25519 AAAAC3Nza…` : un hôte qui présente un certificat d'hôte signé par cette
  autorité passe la vérification, à condition que les principals du certificat, s'il en liste, incluent l'hôte tel qu'écrit
  dans `host` (sans le port).
- `hostkey` : une ligne comme `ssh-ed25519 AAAAC3Nza… [comment]`, par exemple le contenu du fichier
  `/etc/ssh/ssh_host_ed25519_key.pub` de l'hôte. L'étape échoue si l'hôte présente une autre clé.

L'executor demande à l'hôte une clé du type que vous avez listé, ou un certificat d'hôte quand le fichier liste une
autorité pour l'hôte. Un hôte qui a plusieurs types de clés (RSA, ECDSA, Ed25519) passe donc la vérification si le fichier ou
`hostkey` contient l'une quelconque de ses clés. Renseigner les deux clés, un fichier illisible ou un `hostkey` qui n'est pas une clé
publique fait échouer l'étape avant la connexion. Il en va de même pour un `knownhosts` ou un `hostkey` à valeur vide, par exemple une variable
qui se développe en rien : une valeur vide est une erreur, ce n'est pas la même chose que d'omettre la clé, donc elle ne peut pas
désactiver la vérification. Comme pour un échec de connexion, la raison se trouve dans `result.err`.

Exemple

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
*NB : l'option sudo utilise un pseudotty*

## Sortie {#output}

```yaml
systemout
systemerr
err
code
timeseconds
```

- result.timeseconds : durée d'exécution
- result.err : s'il existe, ce champ contient l'erreur
- result.systemout : sortie standard du script exécuté
- result.systemerr : sortie d'erreur du script exécuté
- result.code : code de sortie

## Assertion par défaut {#default-assertion}

```yaml
result.code ShouldEqual 0
```
