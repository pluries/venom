---
translates: mqtt
title: MQTT
description: So verwenden Sie den MQTT-executor in einer venom-Test-Suite.
fingerprint: 9ae821f545fded1165c473e275c91dc01e721875681bba4d6c8f7a000da248fd
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# Venom - Executor MQTT {#venom---executor-mqtt}

Schritt zum Lesen und Schreiben von MQTT-Topics.

Ein Beispiel finden Sie in `tests/mqtt.yml`

## Input {#input}

ClientType kann einer der folgenden Werte sein:
* "publisher"
* "subscriber"
* "persistent_queue"

Wie der Name sagt, erzeugt "persistent_queue" eine persistente Queue, indem die Eigenschaft session_clean gesetzt wird, sodass spätere "subscriber"-Schritte Daten abrufen können. "persistent_queue" wird mit "persistSubscription" kombiniert, das true oder false sein kann; es fordert das persistente Topic-Abonnement an oder gibt es frei.
Wichtig ist, dass persistent_queue und subscriber dieselbe Client-ID verwenden, damit der Broker den Zustand über Verbindungen hinweg verfolgen kann. Denken Sie daran, die Topic-Registrierung am Ende zu entfernen.
Beachten Sie, dass der Name "persistent_queue" statt des bei MQTT üblicheren clean_session verwendet wird. So wird unerwartetes Verhalten vermieden, wenn man die Option "persistent_queue" in der Schrittkonfiguration weglässt.

## Einschränkungen und künftige Verbesserungen {#limitations-and-future-improvements}

### Einschränkungen {#limitations}

Venom startet jede Aufgabe ohne Zustand, der über Schritte hinweg erhalten bleibt, und MQTT würde Nachrichten verlieren, wenn es sich erst nach dem Senden einer Nachricht anmeldet. Deshalb braucht es einen Mechanismus, um persistente Topics für einen späteren mqtt-Schritt zu registrieren. Tests müssen dafür einige zusätzliche Schritte ergänzen, die ein persistentes Topic-Abonnement vorab registrieren und am Ende einer Schrittfolge wieder abmelden.

MQTT könnte Nachrichten nach dem Verbindungsaufbau, aber vor dem Moment ausliefern, in dem der Dienst einen Handler für das Abonnement binden kann. Bei "subscriber"-Schritten hängen wir vor dem Verbindungsaufbau mit AddRoute einen Handler an und abonnieren erst nach dem Verbindungsaufbau. Das soll sicherstellen, dass keine Nachrichten verloren gehen und Abonnements sich wie gewünscht verhalten. Das ist nötig, um Nachrichtenverlust bei einem persistenten Abonnement zu vermeiden.

### Künftige Arbeiten {#future-work}

* Die Möglichkeit ergänzen, den Nachrichteninhalt aus Dateien statt aus der YAML-Konfiguration zu beziehen. Das muss nicht auf diesen executor beschränkt sein
* Unterstützung für Codecs ergänzen, damit sich andere Serialisierungsformate als JSON unterstützen lassen. Das sollte eigentlich eine Fähigkeit sein, die jeder executor nutzen kann, statt für jeden einzeln gelöst zu werden
* TLS-Unterstützung für die MQTT-Schnittstelle ergänzen
