---
translates: overview
title: Vue d'ensemble
description: Ce que fait venom et comment une suite est construite.
fingerprint: faf2593150cb4767a598a4183a06a215b68307ec7c26da0cc429744c250d4622
drafted_by: claude
drafted_on: 2026-11-16
status: reviewed
reviewed_by: Christian Stephan
reviewed_on: 2026-11-16
reviewed_fingerprint: faf2593150cb4767a598a4183a06a215b68307ec7c26da0cc429744c250d4622
---
# Vue d'ensemble {#overview}

Venom vous permet de gérer vos tests d'intégration comme vous codez votre application.
Avec Venom, les testcases sont gérés comme du code : leur lisibilité fait que les tests entrent dans les revues de code. Écrire et exécuter des testsuites devient ainsi plus simple pour les développeurs et les équipes.

Concrètement, vous écrivez une testsuite dans un fichier YAML.
Venom exécute des executors (scripts, requêtes HTTP, web, IMAP, etc.) et applique des assertions.
Il peut aussi générer des fichiers de résultats xUnit.

<img src="./venom.gif" alt="Venom Demonstration">
