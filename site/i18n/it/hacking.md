---
translates: hacking
title: Hacking
description: Come lavorare su venom stesso.
fingerprint: 14fa6d0c590f57d86e6112e8fe3972d3ef1a258de0438f7d48aaaa38eca8425f
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# Hacking {#hacking}

[Come scrivere il proprio executor?](https://github.com/ovh/venom/tree/master/executors#venom-executor)

## Come compilare? {#how-to-compile}
```bash
$ make build
```

## Come testare? {#how-to-test}

### Unit test: {#unit-tests}

```bash
make test
```

### Test di integrazione: {#integration-tests}

Prepara lo stack:

```bash
make build OS=linux ARCH=amd64
cp dist/venom.linux-amd64 tests/venom
cd tests
make start-test-stack  # (wait a few seconds)
make build-test-binary-docker
```

Esegui i test di integrazione:

```bash
make run-test
```

Esegui le suite da `tests/`: le suite raggiungono lo stack tramite `{{.stackHost}}`, il cui valore predefinito (`localhost`) si trova in
`tests/.venomrc`, e venom legge quel file dalla directory corrente. Per eseguirle contro uno stack su un'altra
macchina, passa il suo indirizzo: `make start-test-stack STACK_HOST=<address>` e `make run-test STACK_HOST=<address>`.

Pulizia:

```bash
make clean
make stop-test-stack
```
