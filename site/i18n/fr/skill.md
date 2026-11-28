---
translates: skill
title: L'utiliser depuis Claude Code
description: Installez la compétence venom-pluries pour que Claude Code configure Venom Pluries dans votre projet.
fingerprint: d7f89518bebf2fcf0f7a15898d97748d2257db0cefe23caabcf8a0dd26d922b6
drafted_by: claude
drafted_on: 2026-11-27
status: machine-drafted
---
La compétence `venom-pluries` permet à Claude Code de configurer Venom Pluries dans votre projet. Elle installe une version vérifiée, écrit une première suite de tests, ajoute l'étape à votre build et vérifie les recettes des pages web et mobiles. Vous approuvez chaque commande qu'elle veut exécuter.

## L'installer {#install-it}

Indiquez une étiquette de version. N'installez jamais depuis la branche par défaut, qui bouge.

```bash
$ npx skills add pluries/venom@skill-v1.0.1 --skill venom-pluries
```

L'outil demande pour quel agent installer : choisissez Claude Code. Il place la compétence dans le dossier `venom-pluries` (sous `skills`, dans `.claude`) de votre projet et écrit un fichier `skills-lock.json` à côté. L'outil envoie des données d'usage anonymes, sauf si vous définissez d'abord `DISABLE_TELEMETRY=1` (ou `DO_NOT_TRACK=1`).

Les compétences s'exécutent avec les droits de votre agent. Lisez le `SKILL.md` du dossier installé avant de l'utiliser. La compétence affiche chaque commande et attend votre approbation, et elle ne vous demande jamais de désactiver les approbations.

Vous préférez voir ce qui est copié ? Clonez l'étiquette et copiez le dossier vous-même :

```bash
$ git clone --depth 1 --branch skill-v1.0.1 https://github.com/pluries/venom venom-skill
```

Copiez le dossier `skills/venom-pluries/` de ce clone dans le dossier `venom-pluries` (sous `skills`, dans `.claude`) de votre projet, puis supprimez le clone.

## Vérifier qu'elle est là {#check-that-it-is-there}

Démarrez Claude Code dans le projet et demandez quelles compétences il a : `venom-pluries` figure dans la liste. La ligne `Skill version` en haut de son `SKILL.md` indique sa version, qui correspond à l'étiquette choisie.

## L'utiliser {#use-it}

Demandez avec vos propres mots, par exemple « configure venom dans ce projet et écris un premier test », « lance-le dans GitHub Actions » ou « vérifie ma recette ». La compétence installe la version de venom qu'elle cible (écrite dans son `SKILL.md` et dans votre `.venom-version`), vérifie la somme de contrôle, la signature et le signataire avant d'exécuter quoi que ce soit, et vous dit ce qu'elle a fait et ce qu'elle n'a pas fait.

## Ce que les mainteneurs n'ont pas exécuté {#what-the-maintainers-did-not-run}

Le fichier GitLab CI n'a jamais été exécuté sur un vrai serveur GitLab. La compétence le dit, et le premier essai est le vôtre. Pour les recettes mobiles, la compétence n'installe ni ne démarre d'hôte d'appareil : vous fournissez un serveur Appium et un émulateur ou un appareil.

## La mettre à jour {#update-it}

Installez la nouvelle étiquette de la même façon. Si le `SIGNER_SHA256` de `release.env` diffère de celui de votre projet, le certificat de signature a été renouvelé. La compétence vous dit alors de copier les nouveaux scripts, avec la différence affichée.
