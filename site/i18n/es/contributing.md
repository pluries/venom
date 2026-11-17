---
translates: contributing
title: Contribuir
description: Cómo proponer un cambio en venom.
fingerprint: 0697dcef089c507469ff2eec4d800cba3c5835f38a44669c87c3dc8b868edb8c
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# Contribuir a Venom {#contributing-to-venom}

Este proyecto acepta contribuciones. Para contribuir, debe prestar
atención a algunos puntos:

1. su código debe seguir las reglas de estilo de codificación
2. su código debe estar completamente documentado
3. su trabajo debe estar firmado
4. Pull Requests de GitHub

## Estilo de codificación y de documentación: {#coding-and-documentation-style}

- El código debe estar formateado con `go fmt`
- El código debe pasar `go vet`
- El código debe pasar `golint`

## Envío de modificaciones: {#submitting-modifications}

Las contribuciones deben ser Pull Requests de GitHub. Las directrices son las mismas
que las del envío de parches del kernel de Linux, salvo por el DCO, que
se define más abajo. Las directrices están definidas en el
archivo 'submitting-patches', disponible en la ruta de directorio 'Documentation/process'
del árbol de código fuente del kernel de Linux.

También se puede consultar en línea:

https://www.kernel.org/doc/Documentation/process/submitting-patches.rst

Puede enviar sus parches a través de GitHub

## Licencia de los archivos nuevos: {#licensing-for-new-files}

Venom se distribuye bajo una licencia BSD (modificada). Todo lo que se aporte a
Venom debe publicarse bajo esta licencia.

Al introducir un archivo nuevo en el proyecto, asegúrese de que tenga una
cabecera de copyright que indique con claridad bajo qué licencia se publica.

## Certificado de origen del desarrollador: {#developer-certificate-of-origin}

```
To improve tracking of contributions to this project we will use a
process modeled on the modified DCO 1.1 and use a "sign-off" procedure
on patches that are being contributed.

The sign-off is a simple line at the end of the explanation for the
patch, which certifies that you wrote it or otherwise have the right
to pass it on as an open-source patch. The rules are pretty simple:
if you can certify the below:

By making a contribution to this project, I certify that:

(a) The contribution was created in whole or in part by me and I have
    the right to submit it under the open source license indicated in
    the file; or

(b) The contribution is based upon previous work that, to the best of
    my knowledge, is covered under an appropriate open source License
    and I have the right under that license to submit that work with
    modifications, whether created in whole or in part by me, under
    the same open source license (unless I am permitted to submit
    under a different license), as indicated in the file; or

(c) The contribution was provided directly to me by some other person
    who certified (a), (b) or (c) and I have not modified it.

(d) The contribution is made free of any other party''s intellectual
    property claims or rights.

(e) I understand and agree that this project and the contribution are
    public and that a record of the contribution (including all
    personal information I submit with it, including my sign-off) is
    maintained indefinitely and may be redistributed consistent with
    this project or the open source license(s) involved.

```
