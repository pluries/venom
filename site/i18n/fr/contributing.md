---
translates: contributing
title: Contribuer
description: Comment proposer une modification de venom.
fingerprint: 0697dcef089c507469ff2eec4d800cba3c5835f38a44669c87c3dc8b868edb8c
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# Contribuer à Venom {#contributing-to-venom}

Ce projet accepte les contributions. Pour contribuer, vous devez
veiller à quelques points :

1. votre code doit respecter les règles de style de codage
2. votre code doit être entièrement documenté
3. votre travail doit être signé
4. des Pull Requests GitHub

## Style de code et de documentation {#coding-and-documentation-style}

- Le code doit être formaté avec `go fmt`
- Le code doit passer `go vet`
- Le code doit passer `golint`

## Soumettre des modifications {#submitting-modifications}

Les contributions doivent prendre la forme de Pull Requests GitHub. Les règles sont les mêmes
que pour la soumission de correctifs au noyau Linux, à l'exception du DCO, qui
est défini ci-dessous. Ces règles sont décrites dans le fichier
'submitting-patches', disponible dans le chemin 'Documentation/process'
de l'arborescence des sources du noyau Linux.

Il est aussi consultable en ligne :

https://www.kernel.org/doc/Documentation/process/submitting-patches.rst

Vous pouvez soumettre vos correctifs via GitHub

## Licence des nouveaux fichiers {#licensing-for-new-files}

Venom est distribué sous une licence BSD (modifiée). Tout ce qui est apporté à
Venom doit être publié sous cette licence.

Lorsque vous ajoutez un nouveau fichier au projet, assurez-vous qu'il comporte un
en-tête de copyright indiquant clairement sous quelle licence il est publié.

## Certificat d'origine du développeur {#developer-certificate-of-origin}

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

