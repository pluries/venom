---
translates: cli-usage
title: Utilisation de la CLI
description: Les commandes de venom et leurs options.
fingerprint: 02ee0f4bbd022eb88342dc24560b249d66c272730ab0c8285af92241332fa21a
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# Utilisation de la CLI {#cli-usage}

La CLI `venom` se compose de plusieurs commandes :

```bash
$ venom -h
Venom - RUN Integration Tests

Usage:
  venom [command]

Available Commands:
  help        Help about any command
  run         Run Tests
  version     Display Version of venom: venom version

Flags:
  -h, --help   help for venom

Use "venom [command] --help" for more information about a command.
```

Vous pouvez afficher l'aide d'une commande avec `venom [command] -h` :

```bash
$ venom run -h

run integration tests

Usage:
  venom run [flags]

Examples:
  Run all testsuites containing in files ending with *.yml or *.yaml: venom run
  Run a single testsuite: venom run mytestfile.yml
  Run a single testsuite and export the result in JSON format in test/ folder: venom run mytestfile.yml --format=json --output-dir=test
  Run a single testsuite and export the result in XML and HTML formats in test/ folder: venom run mytestfile.yml --format=xml --output-dir=test --html-report
  Run a single testsuite and specify a variable: venom run mytestfile.yml --var="foo=bar"
  Run a single testsuite and load all variables from a file: venom run mytestfile.yml --var-from-file variables.yaml
  Run all testsuites containing in files ending with *.yml or *.yaml with verbosity: VENOM_VERBOSE=2 venom run

  Notice that variables initialized with -var-from-file argument can be overrided with -var argument

  More info: https://github.com/ovh/venom

Flags:
      --format string           --format:json, tap, xml, yaml (default "xml")
  -h, --help                    help for run
      --html-report             Generate HTML Report
      --lib-dir string          Lib Directory: can contain user executors. example:/etc/venom/lib:$HOME/venom.d/lib
      --output-dir string       Output Directory: create tests results file inside this directory
      --parallel int            Number of test files to run at the same time. 1 (default) runs them one after another (default 1)
      --stop-on-failure         Stop running Test Suite on first Test Case failure
      --strict-variables        Fail a step that names an unknown variable
      --var stringArray         --var cds='cds -f config.json' --var cds2='cds -f config.json'
      --var-from-file strings   --var-from-file filename.yaml --var-from-file filename2.yaml: yaml, must contains a dictionary
  -v, --verbose count           verbose. -v (INFO level in venom.log file), -vv to very verbose (DEBUG level) and -vvv to very verbose with CPU Profiling
```

## Exécuter des suites de tests dans un ordre précis {#run-test-suites-in-a-specific-order}

- `venom run 01_foo.yml 02_foo.yml` exécutera 01 avant 02. 
- `venom run 02_foo.yml 01_foo.yml` exécutera 02 avant 01.

Pour trier de nombreux fichiers de suites de tests, vous pouvez utiliser les commandes standard, par exemple :

```bash
venom run `find . -type f -name "*.yml"|sort`
```

## Exécuter plusieurs fichiers de test en même temps {#run-several-test-files-at-the-same-time}

Par défaut, `venom` exécute un fichier de test à la fois. `--parallel N` en exécute jusqu'à `N` simultanément :

```bash
$ venom run --parallel 4
```

Les tests d'intégration passent le plus clair de leur temps à attendre d'autres systèmes, et c'est cette attente que l'option répartit. Sur huit fichiers qui attendent chacun, on a mesuré : **3,4 fois plus rapide avec `--parallel 4`, 5,6 fois avec `--parallel 8`**.

L'unité est le **fichier** : les cas de test d'un fichier, et les étapes d'un cas, s'exécutent toujours l'un après l'autre, car une étape transmet ses valeurs à la suivante. Exécuter un seul fichier avec `--parallel 4` ne change donc rien.

**Le gain dépend de la part de temps que vos fichiers passent réellement à attendre.** Environ 0,35 s de chaque exécution est fixe (démarrage, analyse, écriture des rapports) et aucun nombre de workers ne l'élimine. Sur les suites `tests/` de venom, qui sont courtes et n'attendent presque pas, `--parallel 4` n'est qu'environ 1,2 fois plus rapide. Attendez-vous à un vrai gain lorsque vos fichiers attendent un service, et à peu de chose dans le cas contraire.

Deux choses à savoir :

- **Chaque ligne de la console est étiquetée avec le fichier dont elle provient**, ce qui permet de continuer à lire la sortie de fichiers exécutés en même temps :

  ```
  [exec.yml]   • exec testsuite (tests/exec.yml)
  [vars.yml]   • vars testsuite (tests/vars.yml)
  [exec.yml]   	• stdin PASS
  [vars.yml]   	• set PASS
  ```

  Les lignes de fichiers différents s'entremêlent (elles ne sont pas regroupées), mais chacune indique qui l'a écrite, et une ligne n'est jamais coupée entre deux fichiers. Suivez un seul fichier avec `grep` :
  `venom run --parallel 4 | grep '^\[exec.yml'`. Avec `--parallel 1`, il n'y a aucune étiquette et la sortie est exactement ce qu'elle a toujours été. Le fichier de log n'est pas concerné dans les deux cas : chaque ligne y est déjà étiquetée avec son fichier de test.
- **`--stop-on-failure` arrête toute l'exécution.** Lorsqu'un fichier de test échoue, aucun autre fichier n'est démarré ; les fichiers déjà en cours sont laissés à terminer, de sorte que chaque fichier démarré rapporte quand même un résultat. Les fichiers qui n'ont jamais démarré sont signalés comme ignorés. Dans les versions précédentes, cette option ne faisait qu'ignorer les cas de test restants du fichier en échec, et l'exécution continuait avec le fichier suivant.

`-vvv` ne peut pas être combiné avec `--parallel 2` ou plus : le profilage CPU porte sur tout le processus, il ne peut donc pas produire un profil par fichier exécuté en même temps. L'exécution s'arrête au démarrage et l'indique.

## Prise en charge de globstar {#globstar-support}

La CLI `venom` prend en charge globstar :

```
$ venom run ./foo/b*/**/z*.yml
```

## Variables {#variables}

Pour définir des variables une à une en ligne de commande, utilisez l'option `--var` avec la commande `venom run` :

```bash
$ venom run --var="foo=bar"
$ venom run --var='foo_list=["biz","buz"]'
$ venom run --var='foo={"biz":"bar","biz":"barr"}'
```

L'option `--var` peut être utilisée plusieurs fois dans une même commande.

### Fichiers de définition de variables {#variable-definitions-files}

Pour définir beaucoup de variables, il est plus pratique d'indiquer leurs valeurs dans un fichier de définition de variables. Ce fichier est un dictionnaire YAML. Vous devez le passer en ligne de commande avec `--var-from-file` :

```bash
venom run --var-from-file variables.yaml
```

### Variables d'environnement {#environment-variables}

En dernier recours, après les autres façons de définir des variables, l'outil `venom` cherche dans l'environnement de son propre processus des variables d'environnement nommées `VENOM_VAR_` suivi du nom d'une variable déclarée.

```bash
$ export VENOM_VAR_foo=bar
$ venom run *.yml
```

Vous pouvez aussi définir la variable d'environnement et lancer votre suite de tests en une seule ligne :

```bash
$ VENOM_VAR_foo=bar venom run *.yml
```

## Arguments {#arguments}

Vous pouvez définir des arguments en ligne de commande en utilisant le nom de l'option.

Les options sont listées dans le résultat de la commande d'aide.

Liste des options disponibles pour la commande `venom run` :

```
Flags:
      --format string           --format:json, tap, xml, yaml (default "xml")
  -h, --help                    help for run
      --html-report             Generate HTML Report
      --lib-dir string          Lib Directory: can contain user executors. example:/etc/venom/lib:$HOME/venom.d/lib
      --output-dir string       Output Directory: create tests results file inside this directory
      --parallel int            Number of test files to run at the same time. 1 (default) runs them one after another (default 1)
      --stop-on-failure         Stop running Test Suite on first Test Case failure
      --strict-variables        Fail a step that names an unknown variable
      --var stringArray         --var cds='cds -f config.json' --var cds2='cds -f config.json'
      --var-from-file strings   --var-from-file filename.yaml --var-from-file filename2.yaml: yaml, must contains a dictionary
  -v, --verbose count           verbose. -vv to very verbose and -vvv to very verbose with CPU Profiling
```

### Définir des arguments avec des variables d'environnement {#define-arguments-with-environment-variables}

Vous pouvez aussi définir les arguments avec des variables d'environnement :

```bash
# is the same as
VENOM_FORMAT=json venom run my-test-suite.yml

# is equivalent to
venom run my-test-suite.yml --format=json
```

Les options et leur équivalent en variable d'environnement :

- l'option `--format="json"` équivaut à la variable d'environnement `VENOM_FORMAT="json"`
- l'option `--lib-dir="/etc/venom/lib:$HOME/venom.d/lib"` équivaut à la variable d'environnement `VENOM_LIB_DIR="/etc/venom/lib"`
- l'option `--output-dir="test-results"` équivaut à la variable d'environnement `VENOM_OUTPUT_DIR="test-results"`
- l'option `--stop-on-failure` équivaut à la variable d'environnement `VENOM_STOP_ON_FAILURE=true`
- l'option `--strict-variables` équivaut à la variable d'environnement `VENOM_STRICT_VARIABLES=true`
- l'option `--var foo=bar` équivaut à la variable d'environnement `VENOM_VAR_foo='bar'`
- l'option `--var-from-file fileA.yml fileB.yml` équivaut à la variable d'environnement `VENOM_VAR_FROM_FILE="fileA.yml fileB.yml"`
- l'option `--parallel 4` équivaut à la variable d'environnement `VENOM_PARALLEL=4`
- l'option `-v` équivaut à la variable d'environnement `VENOM_VERBOSE=1`
- l'option `-vv` équivaut à la variable d'environnement `VENOM_VERBOSE=2`

Il est possible de définir la variable d'environnement `NO_COLOR=1` pour désactiver les couleurs dans la sortie.

## Utiliser un fichier de configuration {#use-a-configuration-file}

Vous pouvez définir les paramètres de venom dans un fichier de configuration `.venomrc`. Ce fichier de configuration doit se trouver dans le répertoire courant ou dans le répertoire `home`.

```yml
variables: 
  - foo=bar
variables_files:
  - my_var_file.yaml
stop_on_failure: true
strict_variables: true
format: xml
output_dir: output
lib_dir: lib
verbosity: 3
parallel: 4
```

Notez que les options de la ligne de commande l'emportent sur le fichier de configuration. Le fichier de configuration l'emporte sur les variables d'environnement.

## Modèle de confiance {#trust-model}

Venom n'a pas de bac à sable. Une suite de tests est du code, et elle s'exécute avec les droits de l'utilisateur qui lance venom.

- Les suites de tests, les executors utilisateur de `lib/` et les plugins Go (`lib/*.so`) s'exécutent tous avec vos droits. N'exécutez une suite que si vous exécuteriez aussi un script de même provenance.
- Ne placez jamais le résultat d'une étape, ni aucune valeur que vous ne maîtrisez pas, dans un `script:` ou dans du texte SQL sans le protéger par des guillemets. Le résultat d'une étape peut contenir n'importe quoi, et venom ne l'échappe ni pour un shell ni pour SQL.
- En CI, n'exécutez pas de suites provenant de forks ou de pull requests non relus, et ne rendez aucun secret disponible pour de telles exécutions.
- Un secret déclaré dans `secrets` ou dans `.venomrc` est masqué dans les rapports, les logs et la console. Seules les valeurs déclarées sont masquées.
- Les executors `ssh`, `imap` et `smtp` ne vérifient pas l'identité du serveur par défaut. Venom écrit un avertissement une fois par exécution lorsque c'est le cas. Pour activer la vérification, définissez `knownhosts` ou `hostkey` sur une étape `ssh`, et `verifytls: true` (sous `auth:` pour `imap`) ainsi que `withtls: true` sur une étape `imap` ou `smtp`.
- L'executor `web` ignore par défaut les erreurs de certificat HTTPS. Définissez la variable `verifyTls` à `true` pour que le navigateur rejette un certificat invalide. Voir [executors/web](executors/web/README.md).
