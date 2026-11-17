---
translates: hacking
title: Travailler sur venom
description: Comment travailler sur venom lui-même.
fingerprint: 14fa6d0c590f57d86e6112e8fe3972d3ef1a258de0438f7d48aaaa38eca8425f
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# Hacking {#hacking}

[Comment écrire votre propre executor ?](https://github.com/ovh/venom/tree/master/executors#venom-executor)

## Comment compiler ? {#how-to-compile}
```bash
$ make build
```

## Comment tester ? {#how-to-test}

### Tests unitaires : {#unit-tests}

```bash
make test
```

### Tests d'intégration : {#integration-tests}

Préparez la pile de test :

```bash
make build OS=linux ARCH=amd64
cp dist/venom.linux-amd64 tests/venom
cd tests
make start-test-stack  # (wait a few seconds)
make build-test-binary-docker
```

Lancez les tests d'intégration :

```bash
make run-test
```

Lancez les suites de `tests/` : les suites atteignent la pile via `{{.stackHost}}`, dont la valeur par défaut (`localhost`) se trouve
dans `tests/.venomrc`, et venom lit ce fichier depuis le répertoire courant. Pour les lancer contre une pile située sur une autre
machine, indiquez son adresse : `make start-test-stack STACK_HOST=<address>` et `make run-test STACK_HOST=<address>`.

Nettoyage :

```bash
make clean
make stop-test-stack
```
