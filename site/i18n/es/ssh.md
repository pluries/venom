---
translates: ssh
title: SSH
description: Cómo usar el executor SSH en una suite de pruebas de venom.
fingerprint: 44c8a00c687f093c007b2632c39692ac92400c71c058513ec8972df353563c5d
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# Venom - Executor SSH {#venom---executor-ssh}

Paso para ejecutar un script en un servidor remoto mediante SSH


## Entrada {#input}

En su archivo YAML puede usar:

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

**Comprobación de la clave del host.** Por defecto, no se define ni `knownhosts` ni `hostkey`, y la clave del host **no se comprueba**:
se acepta cualquier servidor. Es el comportamiento del executor anterior a la existencia de estas dos claves. Resulta adecuado para hosts de prueba
efímeros cuya clave cambia en cada ejecución. Defina una de las dos claves para comprobar el host:

- `knownhosts`: un archivo con el formato de `~/.ssh/known_hosts`, por ejemplo la salida de `ssh-keyscan -p 2222 192.0.2.10`. Una
  ruta relativa se lee desde el directorio de la suite de pruebas; una ruta absoluta se usa tal cual. El paso falla si el host
  no está en el archivo o presenta otra clave. El archivo también puede confiar en una autoridad certificadora con una línea como
  `@cert-authority [192.0.2.10]:2222 ssh-ed25519 AAAAC3Nza…`: un host que presente un certificado de host firmado por esa
  autoridad pasa la comprobación, siempre que los principals del certificado, cuando enumera alguno, incluyan el host tal como se escribe
  en `host` (sin el puerto).
- `hostkey`: una línea como `ssh-ed25519 AAAAC3Nza… [comment]`, por ejemplo el contenido de
  `/etc/ssh/ssh_host_ed25519_key.pub` del host. El paso falla si el host presenta otra clave.

El executor pide al host una clave del tipo que usted indicó, o un certificado de host cuando el archivo enumera una
autoridad para ese host. Por tanto, un host con varios tipos de clave (RSA, ECDSA, Ed25519) pasa la comprobación si el archivo o
`hostkey` contiene cualquiera de sus claves. Definir ambas claves, un archivo que no se puede leer o un `hostkey` que no es una clave
pública hace fallar el paso antes de conectar. Lo mismo ocurre con un `knownhosts` o un `hostkey` con un valor vacío, como una variable
que se expande a nada: un valor vacío es un error, no equivale a omitir la clave, de modo que no puede desactivar
la comprobación. Igual que un fallo de conexión, el motivo aparece en `result.err`.

Ejemplo

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
*NB: la opción sudo usa un pseudotty*

## Salida {#output}

```yaml
systemout
systemerr
err
code
timeseconds
```

- result.timeseconds: tiempo de ejecución
- result.err: si existe, este campo contiene el error
- result.systemout: salida estándar del script ejecutado
- result.systemerr: salida de error del script ejecutado
- result.code: código de salida

## Aserción por defecto {#default-assertion}

```yaml
result.code ShouldEqual 0
```
