---
translates: ssh
title: SSH
description: Den SSH-executor in einer venom-Test-Suite verwenden.
fingerprint: 44c8a00c687f093c007b2632c39692ac92400c71c058513ec8972df353563c5d
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# Venom - Executor SSH {#venom---executor-ssh}

Schritt zum Ausführen eines Skripts auf einem entfernten Server über SSH


## Eingabe {#input}

In Ihrer YAML-Datei können Sie Folgendes verwenden:

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

**Prüfung des Host-Keys.** Standardmäßig ist weder `knownhosts` noch `hostkey` gesetzt, und der Schlüssel des Hosts wird **nicht geprüft**:
Jeder Server wird akzeptiert. So verhielt sich der executor, bevor es die beiden Schlüssel gab. Das passt zu kurzlebigen
Testhosts, deren Schlüssel sich bei jedem Lauf ändert. Setzen Sie einen der beiden Schlüssel, um den Host zu prüfen:

- `knownhosts`: eine Datei im Format von `~/.ssh/known_hosts`, zum Beispiel die Ausgabe von `ssh-keyscan -p 2222 192.0.2.10`. Ein
  relativer Pfad wird ab dem Verzeichnis der Test-Suite gelesen; ein absoluter Pfad wird unverändert verwendet. Der Schritt schlägt fehl, wenn der Host
  nicht in der Datei steht oder einen anderen Schlüssel vorlegt. Die Datei kann auch einer Zertifizierungsstelle vertrauen, mit einer Zeile wie
  `@cert-authority [192.0.2.10]:2222 ssh-ed25519 AAAAC3Nza…`: Ein Host, der ein von dieser
  Zertifizierungsstelle signiertes Host-Zertifikat vorlegt, besteht die Prüfung, sofern die Principals des Zertifikats, falls es welche aufführt, den Host so enthalten, wie er
  in `host` geschrieben ist (ohne den Port).
- `hostkey`: eine Zeile wie `ssh-ed25519 AAAAC3Nza… [comment]`, zum Beispiel der Inhalt der Datei
  `/etc/ssh/ssh_host_ed25519_key.pub` des Hosts. Der Schritt schlägt fehl, wenn der Host einen anderen Schlüssel vorlegt.

Der executor fragt den Host nach einem Schlüssel des aufgeführten Typs oder nach einem Host-Zertifikat, wenn die Datei eine
Zertifizierungsstelle für den Host aufführt. Ein Host mit mehreren Schlüsseltypen (RSA, ECDSA, Ed25519) besteht die Prüfung daher, wenn die Datei oder
`hostkey` irgendeinen seiner Schlüssel enthält. Wenn beide Schlüssel gesetzt sind, eine Datei nicht gelesen werden kann oder ein `hostkey` kein öffentlicher
Schlüssel ist, schlägt der Schritt fehl, noch bevor eine Verbindung besteht. Dasselbe gilt für ein `knownhosts` oder `hostkey`, das auf einen leeren Wert gesetzt ist, etwa eine Variable,
die zu nichts expandiert: Ein leerer Wert ist ein Fehler und nicht dasselbe wie das Weglassen des Schlüssels, sodass er die Prüfung nicht
abschalten kann. Wie bei einem Verbindungsfehler steht der Grund in `result.err`.

Beispiel

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
*NB: Die Option sudo verwendet ein Pseudo-TTY*

## Ausgabe {#output}

```yaml
systemout
systemerr
err
code
timeseconds
```

- result.timeseconds: Ausführungszeit
- result.err: Falls vorhanden, enthält dieses Feld den Fehler
- result.systemout: Standardausgabe des ausgeführten Skripts
- result.systemerr: Fehlerausgabe des ausgeführten Skripts
- result.code: Exit-Code

## Standard-Assertion {#default-assertion}

```yaml
result.code ShouldEqual 0
```
