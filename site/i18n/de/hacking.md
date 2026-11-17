---
translates: hacking
title: Hacking
description: So arbeiten Sie an venom selbst.
fingerprint: 14fa6d0c590f57d86e6112e8fe3972d3ef1a258de0438f7d48aaaa38eca8425f
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# Hacking {#hacking}

[Wie schreibe ich einen eigenen Executor?](https://github.com/ovh/venom/tree/master/executors#venom-executor)

## Wie wird kompiliert? {#how-to-compile}
```bash
$ make build
```

## Wie wird getestet? {#how-to-test}

### Unit-Tests: {#unit-tests}

```bash
make test
```

### Integrationstests: {#integration-tests}

Den Stack vorbereiten:

```bash
make build OS=linux ARCH=amd64
cp dist/venom.linux-amd64 tests/venom
cd tests
make start-test-stack  # (wait a few seconds)
make build-test-binary-docker
```

Integrationstests ausführen:

```bash
make run-test
```

Die Suites aus `tests/` ausführen: Die Suites erreichen den Stack über `{{.stackHost}}`, dessen Standardwert (`localhost`) in
`tests/.venomrc` steht, und venom liest diese Datei aus dem aktuellen Verzeichnis. Um sie gegen einen Stack auf einem anderen
Rechner auszuführen, übergeben Sie dessen Adresse: `make start-test-stack STACK_HOST=<address>` und `make run-test STACK_HOST=<address>`.

Aufräumen:

```bash
make clean
make stop-test-stack
```
