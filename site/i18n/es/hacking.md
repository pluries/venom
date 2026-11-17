---
translates: hacking
title: Desarrollo de venom
description: Cómo trabajar en el propio venom.
fingerprint: 14fa6d0c590f57d86e6112e8fe3972d3ef1a258de0438f7d48aaaa38eca8425f
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# Desarrollo de venom {#hacking}

[¿Cómo escribir su propio executor?](https://github.com/ovh/venom/tree/master/executors#venom-executor)

## ¿Cómo compilar? {#how-to-compile}
```bash
$ make build
```

## ¿Cómo probar? {#how-to-test}

### Pruebas unitarias: {#unit-tests}

```bash
make test
```

### Pruebas de integración: {#integration-tests}

Prepare el stack:

```bash
make build OS=linux ARCH=amd64
cp dist/venom.linux-amd64 tests/venom
cd tests
make start-test-stack  # (wait a few seconds)
make build-test-binary-docker
```

Ejecute las pruebas de integración:

```bash
make run-test
```

Ejecute las suites de `tests/`: las suites acceden al stack a través de `{{.stackHost}}`, cuyo valor predeterminado (`localhost`) está
en `tests/.venomrc`, y venom lee ese archivo desde el directorio actual. Para ejecutarlas contra un stack en otra
máquina, indique su dirección: `make start-test-stack STACK_HOST=<address>` y `make run-test STACK_HOST=<address>`.

Limpieza:

```bash
make clean
make stop-test-stack
```
