---
translates: recipes
title: Recettes
description: Écrivez des tests de navigateur et de mobile sous forme de recettes qu'un contrôle et un contrôle négatif peuvent prouver.
fingerprint: 82005e3722eb38017980efc2b70ccd792894888e52d1846d5c7918662e03d91a
drafted_by: claude
drafted_on: 2026-11-19
status: machine-drafted
---
Une recette est une suite de tests venom qui pilote une page web ou une application mobile. Cette page montre comment organiser les recettes, les nommer, prouver qu'elles testent quelque chose, et garder les secrets et les résultats hors des mauvais endroits. Il vous faut venom et, pour les recettes web, un navigateur installé par Playwright. Les pages du [executor web](web.html) et du [executor mobile](mobile.html) listent chaque action.

Une recette est une suite ordinaire. Elle utilise des étapes `web` ou des étapes `mobile`, et rien de nouveau. Voici une petite recette web. L'adresse de la page est la variable `appUrl`, de sorte que le même fichier s'exécute contre n'importe quelle copie de la page :

{{include site/examples/recipes/web-page.yml yaml}}

Lancez-la avec l'adresse de votre page :

```
venom run --var appUrl=http://localhost:8000 web-page.yml
```

## Organisation des dossiers {#folder-layout}

Gardez les recettes d'une fonctionnalité dans un seul dossier sous `tests/`, à côté des autres suites venom. Par exemple :

```
tests/
  site/
    site-languages.yml
  recipes/
    reference/
      mobile/
        counter.yml
```

Un dossier par fonctionnalité. Un ou plusieurs fichiers `.yml` dedans. Un dossier qui contient des sous-dossiers se vérifie en nommant les sous-dossiers. Gardez les fichiers de variables qui contiennent des identifiants hors du dossier, ou ignorés par git (voir Identifiants).

## Noms des cas de test et identifiant de spec {#testcase-names-and-the-spec-id}

Chaque cas de test prouve un scénario d'acceptation d'une spec. Commencez le nom du cas de test par l'identifiant du scénario, puis une courte description : `US1-AS3 an item typed in the input is added to the list`. L'identifiant a la forme `USn-ASm` : user story `n`, scénario d'acceptation `m`.

Nommez la spec une seule fois, dans une variable de la suite :

```
vars:
  spec: 014-web-mobile-recipes
```

La valeur est le nom du dossier de la spec. Le contrôle des recettes lit `spec.md` dans ce dossier et y cherche les identifiants. L'option `--specs-dir` lui indique où se trouvent les dossiers de specs. Deux fichiers d'un même dossier ne doivent pas revendiquer le même scénario.

Pour voir quels scénarios une recette prouve, ajoutez `--coverage` avec l'identifiant de la spec : `venom recipe check --specs-dir <specs folder> --coverage 014-web-mobile-recipes tests/recipes`. Après les constats, il liste chaque scénario de la spec, `US1-AS3 proven: <file> / <testcase>` ou `US1-AS4 not proven`. Un cas de test ne prouve un scénario que s'il n'est pas ignoré et contient une assertion qui lit l'application. L'option ne change pas le code de sortie. Le script de couverture du jeu de développement ne lit pas les fichiers `.yml` : utilisez donc cette option, et non ce script, pour compter les recettes.

## Sélecteurs {#selectors}

Une étape `find`, `click` ou `fill` prend un sélecteur. La clé `locator` indique comment le lire : `CSS`, `XPATH`, `ROLE`, `TEXT`, `LABEL`, `PLACEHOLDER` ou `TESTID`.

Préférez les localisateurs ROLE et TESTID à un chemin CSS. `ROLE` trouve un élément par ce qu'il est et par son nom, comme le voit un utilisateur. `TESTID` le trouve par un identifiant de test que la page définit uniquement pour les tests :

```
find:
  locator: ROLE
  selector: button
  name: Add item
```

```
find:
  locator: TESTID
  selector: add-button
```

Les identifiants et les noms de classe changent quand quelqu'un modifie le style de la page. Un rôle et un nom, ou un identifiant de test, restent. Si la page n'a ni rôle ni identifiant de test exploitable, demandez aux développeurs d'en ajouter avant d'écrire la recette. Ne revenez à CSS ou XPath que si rien d'autre ne correspond.

`TESTID` lit l'attribut `data-testid`. Beaucoup de pages utilisent `data-test` à la place, et `TESTID` ne trouve alors rien. Définissez une fois dans la suite la variable `web.testIdAttribute` avec le nom que la page utilise, par exemple `testIdAttribute: data-test`. La page de l'[executor web](web.html) liste chaque localisateur et ses règles.

`find` n'attend pas. Il compte les éléments qui existent maintenant. Quand la page charge des données après le premier affichage, attendez d'abord avec `wait`, ou utilisez un `click` ou un `fill`, qui attendent leur élément. Dans une étape mobile, définissez exactement un localisateur, comme `accessibilityId`.

## Étapes de preuve {#proof-steps}

Une recette qui passe ne prouve rien tant que vous ne l'avez pas vue échouer. Trois étapes donnent la preuve :

1. Rejeu. Lancez la recette avec `venom run` contre l'application. Chaque cas de test doit passer. Rejouez-la plusieurs fois : un cas de test qui passe une fois et échoue une fois n'est pas utilisable.
2. Contrôle négatif. Pour chaque cas de test, copiez la recette, changez la première valeur attendue qui vient de l'application, et lancez la copie. L'exécution doit se terminer avec le statut 2 et nommer ce cas de test. Laissez le fichier d'origine intact.
3. Contrôle de la recette. Lancez le contrôle, qui lit les fichiers sans navigateur ni appareil :

```
venom recipe check --specs-dir <folder of the specs> tests/site
```

Le contrôle se termine avec 0 quand il ne trouve rien, 1 quand il signale un constat et 2 quand il ne peut pas lire son entrée. Chaque constat nomme le fichier, la ligne, la règle et le cas de test. Il signale un cas de test sans identifiant de scénario, un identifiant répété, un identifiant que la spec n'a pas, un cas de test sans assertion sur une valeur lue dans l'application, une assertion qui ne peut pas échouer, un texte qui ressemble à un identifiant secret et une adresse littérale.

Le contrôle est statique. Il ne peut pas lire l'intérieur d'un script `execute` et ne peut pas savoir si une valeur attendue est juste. Le contrôle négatif couvre les deux.

## Ce que les executors ne savent pas faire {#what-the-executors-cannot-do}

L'executor web a les étapes `hover`, `dragAndDrop` et `press` pour une touche ou un enchaînement tel que `Control+A`, et `waitFor` pour un élément qui apparaît ou disparaît. `find` ne peut pas lire un nombre comme une largeur de défilement. L'action `execute` lance un script dans la page et renvoie une valeur, sur laquelle vous pouvez faire une assertion. Cet exemple lit de combien la page défile sur le côté et combien d'éléments elle liste :

{{include site/examples/recipes/web-execute.yml yaml}}

Un `waitFor` avec l'état `hidden` ou `detached` passe aussitôt sur un sélecteur mal saisi : vérifiez donc d'abord le sélecteur. `press` envoie des touches, pas un texte, et un vidage ou une trace contient ce que `fill` et `press` ont saisi. Si `execute` ne suffit pas, notez le manque et n'écrivez pas de recette pour ce scénario. Ne mettez pas toutes les assertions d'un cas de test sur le résultat d'un seul script sans contrôle négatif : le contrôle ne peut pas lire le script.

Une recette mobile a besoin d'un serveur Appium et d'un appareil ou d'un émulateur. Elle ne s'exécute pas sans eux. Celle-ci porte l'étiquette `# needs: device` sur sa première ligne, de sorte que les tests du site l'ignorent là où aucun appareil n'existe :

{{include site/examples/recipes/mobile-counter.yml yaml}}

## Identifiants {#credentials}

Une recette ne contient ni mot de passe, ni jeton, ni clé. Une connexion vient de variables. Passez-les sur la ligne de commande ou dans un fichier de variables que git ignore :

```
venom run --var-from-file login.vars.yml tests/site/login.yml
```

Dans la recette, écrivez la variable : `text: "{{.password}}"`. Le contrôle signale un texte qui ressemble à un identifiant secret dans n'importe quelle étape ou valeur par défaut.

Les recettes s'exécutent contre un environnement de test, jamais contre la production. L'adresse est une variable que le pipeline définit, et `venom run` ne vous empêche pas de la pointer vers un vrai système. Gardez la liste des hôtes autorisés dans votre pipeline.

## Résultats et artefacts {#results-and-artifacts}

Une étape en échec peut écrire un export de la page, une capture d'écran ou une trace. Ces fichiers peuvent contenir des données de l'application. Écrivez-les dans un dossier de résultats que git ignore, et ne les validez pas. En CI, gardez-les peu de temps, par exemple quelques jours, et donnez l'accès aux seules personnes qui en ont besoin : le stockage est restreint. Ne les copiez pas dans un stockage de longue durée.

## Variables non définies {#undefined-variables}

Une variable que vous oubliez de passer n'arrête pas l'exécution par défaut. Venom laisse `{{.password}}` comme le texte littéral `{{.password}}` et l'étape saisit ce texte. L'exécution n'échoue pas pour autant.

Passez `--strict-variables` pour changer cela. Venom s'arrête alors avec le statut de sortie 2 et le message `unknown variable(s) "password"`. Utilisez l'option dans chaque pipeline qui lance des recettes :

```
venom run --strict-variables --var-from-file login.vars.yml tests/site/login.yml
```
