---
translates: quickstart
title: Démarrage rapide
description: Installez venom, écrivez une suite et lancez-la.
fingerprint: ff09c7857b2e31620d2ae8d74003a0e186283bb52cf0d3334055d5d020980dc4
drafted_by: claude
drafted_on: 2026-11-24
status: reviewed
reviewed_by: Christian Stephan
reviewed_on: 2026-11-25
reviewed_fingerprint: ff09c7857b2e31620d2ae8d74003a0e186283bb52cf0d3334055d5d020980dc4
---
Cette page vous conduit de zéro à une suite de tests qui passe. Il vous faut un terminal sous Linux, macOS ou Windows.

## Installation {#install}

Téléchargez l'exécutable de la dernière release pour votre plateforme. Voici l'exemple Linux. La page [installation](installing.html) liste toutes les plateformes, et la page [vérifier une version](verify-release.html) montre comment contrôler la signature :

```
curl -L -o venom https://github.com/pluries/venom/releases/latest/download/venom.linux-amd64
chmod +x venom
```

Les commandes produisent un exécutable `venom` dans le dossier courant.

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
