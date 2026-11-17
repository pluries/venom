---
translates: overview
title: Panoramica
description: Cosa fa venom e come è fatta una suite.
fingerprint: faf2593150cb4767a598a4183a06a215b68307ec7c26da0cc429744c250d4622
drafted_by: claude
drafted_on: 2026-11-16
status: reviewed
reviewed_by: Christian Stephan
reviewed_on: 2026-11-16
reviewed_fingerprint: faf2593150cb4767a598a4183a06a215b68307ec7c26da0cc429744c250d4622
---
# Panoramica {#overview}

Venom ti permette di gestire i test di integrazione nello stesso modo in cui scrivi la tua applicazione.
Con Venom, i testcase sono gestiti come codice: la leggibilità dei test fa sì che entrino nelle code review. Così scrivere ed eseguire testsuite diventa più semplice per sviluppatori e team.

In concreto, devi scrivere una testsuite in un file YAML.
Venom esegue degli executor (script, richieste HTTP, web, IMAP e così via) e applica delle assertion.
Può anche generare file di risultati xUnit.

<img src="./venom.gif" alt="Dimostrazione di Venom">
