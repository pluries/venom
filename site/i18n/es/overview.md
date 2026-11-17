---
translates: overview
title: Visión general
description: Qué hace venom y cómo se construye una suite.
fingerprint: faf2593150cb4767a598a4183a06a215b68307ec7c26da0cc429744c250d4622
drafted_by: claude
drafted_on: 2026-11-16
status: reviewed
reviewed_by: Christian Stephan
reviewed_on: 2026-11-16
reviewed_fingerprint: faf2593150cb4767a598a4183a06a215b68307ec7c26da0cc429744c250d4622
---
# Visión general {#overview}

Venom le permite gestionar las pruebas de integración de la misma manera en que programa su aplicación.
Con Venom, los casos de prueba (testcases) se gestionan como código: al ser legibles, las pruebas forman parte de las revisiones de código. Gracias a ello, escribir y ejecutar suites de pruebas (testsuites) resulta más fácil para desarrolladores y equipos.

En concreto, debe escribir una suite de pruebas en un archivo YAML.
Venom ejecuta executors (scripts, solicitudes HTTP, web, IMAP, etc.) y aplica aserciones (assertions).
También puede generar archivos de resultados xUnit.

<img src="./venom.gif" alt="Venom Demonstration">
