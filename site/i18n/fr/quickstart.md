---
translates: quickstart
title: Démarrage rapide
description: Installez venom, écrivez une suite et lancez-la.
fingerprint: 86cf4090a52b3ea8d391e1a37a1038fbbf6b86519dd9a23afd4e7391ceb9b402
drafted_by: claude
drafted_on: 2026-11-16
status: reviewed
reviewed_by: Christian Stephan
reviewed_on: 2026-11-16
reviewed_fingerprint: 86cf4090a52b3ea8d391e1a37a1038fbbf6b86519dd9a23afd4e7391ceb9b402
---
Cette page vous conduit de zéro à une suite de tests qui passe. Il vous faut Go et un clone du dépôt venom.

## Installation {#install}

Compilez l'outil en ligne de commande depuis un clone du dépôt :

```
go build -o venom ./cmd/venom
```

La commande produit un binaire `venom` dans le dossier courant. Lorsqu'une release existe, vous pouvez télécharger son binaire au lieu de compiler.

## Écrire une suite {#write-a-suite}

Une suite est un fichier YAML. Celle-ci contient un cas de test avec une seule étape. L'étape exécute une commande shell, et deux assertions vérifient son résultat :

````yaml
name: Quickstart suite
testcases:
  - name: say hello
    steps:
      - type: exec
        script: echo "hello venom"
        assertions:
          - result.systemout ShouldContainSubstring hello
          - result.code ShouldEqual 0
````

- `type: exec` exécute le `script` dans un shell.
- `result.systemout ShouldContainSubstring hello` vérifie que la sortie contient le mot `hello`.
- `result.code ShouldEqual 0` vérifie que la commande s'est terminée sans erreur.

## Lancez-la {#run-it}

Enregistrez le fichier sous le nom `quickstart.yml` et lancez-le :

```
./venom run quickstart.yml
```

## Lire le résultat {#read-the-result}

Venom affiche chaque cas de test et son statut. Lorsque toutes les assertions sont vraies, la dernière ligne est :

```
final status: PASS
```

Une assertion en échec est affichée avec l'étape, l'assertion et la valeur trouvée par venom. Le code de sortie n'est pas nul lorsqu'une suite échoue.

Venom est un fork indépendant de [ovh/venom](https://github.com/ovh/venom). Lisez ensuite la [vue d'ensemble](overview.html) ou la page sur les [suites de tests](testsuites.html).
