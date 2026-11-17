---
translates: contributing
title: Beitragen
description: So schlagen Sie eine Änderung an venom vor.
fingerprint: 0697dcef089c507469ff2eec4d800cba3c5835f38a44669c87c3dc8b868edb8c
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# Zu Venom beitragen {#contributing-to-venom}

Dieses Projekt nimmt Beiträge an. Um beizutragen, sollten Sie
auf ein paar Dinge achten:

1. Ihr Code muss den Regeln des Coding-Stils folgen
2. Ihr Code muss vollständig dokumentiert sein
3. Ihre Arbeit muss signiert sein
4. GitHub Pull Requests

## Coding- und Dokumentationsstil: {#coding-and-documentation-style}

- Der Code muss mit `go fmt` formatiert sein
- Der Code muss `go vet` bestehen
- Der Code muss `golint` bestehen

## Änderungen einreichen: {#submitting-modifications}

Beiträge sollten als GitHub Pull Requests eingereicht werden. Es gelten dieselben Richtlinien
wie für das Einreichen von Patches für den Linux-Kernel, mit Ausnahme des DCO, das
weiter unten definiert ist. Die Richtlinien stehen in der Datei
„submitting-patches“ im Verzeichnispfad „Documentation/process“
des Linux-Kernel-Quellbaums.

Sie ist auch online abrufbar:

https://www.kernel.org/doc/Documentation/process/submitting-patches.rst

Sie können Ihre Patches über GitHub einreichen

## Lizenzierung neuer Dateien: {#licensing-for-new-files}

Venom steht unter einer (modifizierten) BSD-Lizenz. Alles, was zu
Venom beigetragen wird, muss unter dieser Lizenz veröffentlicht werden.

Wenn Sie eine neue Datei in das Projekt aufnehmen, achten Sie bitte darauf, dass sie
einen Copyright-Header enthält, der klarstellt, unter welcher Lizenz sie veröffentlicht wird.

## Developer Certificate of Origin: {#developer-certificate-of-origin}

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

