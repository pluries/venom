---
translates: mqtt
title: MQTT
description: Come usare l'executor MQTT in una test suite di venom.
fingerprint: 9ae821f545fded1165c473e275c91dc01e721875681bba4d6c8f7a000da248fd
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# Venom - Executor MQTT {#venom---executor-mqtt}

Step per leggere e scrivere topic MQTT.

Un esempio si trova in `tests/mqtt.yml`

## Input {#input}

ClientType può essere uno tra:
* "publisher"
* "subscriber"
* "persistent_queue"

Come suggerisce il nome, "persistent_queue" crea una coda persistente impostando la proprietà session_clean, così che gli step "subscriber" successivi possano recuperare i dati. "persistent_queue" è abbinato a "persistSubscription", che può essere true o false e che richiede/rilascia la sottoscrizione persistente al topic.
È importante che persistent_queue e subscriber usino lo stesso client id, in modo che il broker possa tracciare lo stato tra una connessione e l'altra. Ricordati di rimuovere la registrazione del topic quando hai finito.
Nota l'uso del nome "persistent_queue" invece del più usuale clean_session di MQTT: serve a ridurre i comportamenti inattesi quando si omette l'opzione "persistent_queue" dalla configurazione dello step.

## Limiti e miglioramenti futuri {#limitations-and-future-improvements}

### Limiti {#limitations}

Dato che venom avvia ogni task senza alcuno stato conservato tra gli step, e che mqtt perderebbe i messaggi se la sottoscrizione avvenisse dopo l'invio di un messaggio, serve un meccanismo per registrare i topic persistenti per un successivo step mqtt. Questo richiede che i test aggiungano alcuni step in più per pre-registrare la sottoscrizione a un topic persistente e annullarne la registrazione alla fine di una sequenza di step.

MQTT potrebbe consegnare messaggi dopo la connessione ma prima che il servizio abbia la possibilità di collegare un handler di sottoscrizione. Per gli step "subscriber" colleghiamo un handler con AddRoute prima della connessione e poi ci sottoscriviamo dopo la connessione. In questo modo nessun messaggio dovrebbe andare perso e le sottoscrizioni si comportano come ti aspetti. È necessario per evitare la perdita di messaggi quando si usa una sottoscrizione persistente.

### Lavoro futuro {#future-work}

* Aggiungere la possibilità di ottenere il contenuto dei messaggi da file invece che dalla configurazione YAML. Non deve essere specifico di questo executor
* Aggiungere il supporto ai codec, per supportare formati di serializzazione diversi da json. Dovrebbe essere una capacità sfruttabile da qualsiasi executor, invece di essere risolta singolarmente per ciascuno
* Aggiungere il supporto TLS per l'interfaccia MQTT
