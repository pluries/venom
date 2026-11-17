---
translates: contributing
title: Contribuire
description: Come proporre una modifica a venom.
fingerprint: 0697dcef089c507469ff2eec4d800cba3c5835f38a44669c87c3dc8b868edb8c
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# Contribuire a Venom {#contributing-to-venom}

Questo progetto accetta contributi. Per contribuire, devi
prestare attenzione ad alcuni aspetti:

1. il tuo codice deve rispettare le regole di stile di codifica
2. il tuo codice deve essere documentato per intero
3. il tuo lavoro deve essere firmato
4. Pull Request su GitHub

## Stile di codifica e di documentazione: {#coding-and-documentation-style}

- Il codice deve essere formattato con `go fmt`
- Il codice deve superare `go vet`
- Il codice deve superare `golint`

## Invio delle modifiche: {#submitting-modifications}

I contributi devono essere Pull Request di GitHub. Le linee guida sono le stesse
dell'invio di patch per il kernel Linux, ad eccezione del DCO, che
è definito più avanti. Le linee guida sono definite nel file
'submitting-patches', disponibile nel percorso 'Documentation/process'
dell'albero dei sorgenti del kernel Linux.

È consultabile anche online:

https://www.kernel.org/doc/Documentation/process/submitting-patches.rst

Puoi inviare le tue patch tramite GitHub

## Licenza dei nuovi file: {#licensing-for-new-files}

Venom è distribuito con una licenza BSD (modificata). Tutto ciò che viene contribuito a
Venom deve essere rilasciato con questa licenza.

Quando introduci un nuovo file nel progetto, assicurati che abbia
un'intestazione di copyright che chiarisca con quale licenza viene rilasciato.

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

