---
translates: overview
title: Überblick
description: Was venom leistet und wie eine Suite aufgebaut ist.
fingerprint: faf2593150cb4767a598a4183a06a215b68307ec7c26da0cc429744c250d4622
drafted_by: claude
drafted_on: 2026-11-16
status: reviewed
reviewed_by: Christian Stephan
reviewed_on: 2026-11-16
reviewed_fingerprint: faf2593150cb4767a598a4183a06a215b68307ec7c26da0cc429744c250d4622
---
# Überblick {#overview}

Mit Venom behandeln Sie Integrationstests auf dieselbe Weise, wie Sie Ihre Anwendung programmieren.
Mit Venom werden Testcases als Code verwaltet: Weil die Tests gut lesbar sind, werden sie Teil der Code-Reviews. Dadurch wird das Schreiben und Ausführen von Testsuites für Entwicklerinnen, Entwickler und Teams einfacher.

Konkret schreiben Sie eine Testsuite in eine YAML-Datei.
Venom führt Executors aus (Skripte, HTTP-Requests, Web, IMAP usw.) und wendet Assertions an.
Außerdem kann es xUnit-Ergebnisdateien erzeugen.

<img src="./venom.gif" alt="Venom Demonstration">
