---
translates: mqtt
title: MQTT
description: Cómo usar el executor MQTT en una suite de pruebas de venom.
fingerprint: 9ae821f545fded1165c473e275c91dc01e721875681bba4d6c8f7a000da248fd
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# Venom - Executor MQTT {#venom---executor-mqtt}

Paso para leer y escribir en topics MQTT.

Puede encontrar un ejemplo en `tests/mqtt.yml`

## Input {#input}

ClientType puede ser uno de:
* "publisher"
* "subscriber"
* "persistent_queue"

Como su nombre indica, "persistent_queue" crea una cola persistente mediante la propiedad session_clean, de modo que los pasos "subscriber" posteriores puedan recuperar los datos. "persistent_queue" va acompañado de "persistSubscription", que puede ser true o false y que solicita o libera la suscripción persistente al topic.
Es importante que persistent_queue y subscriber usen el mismo identificador de cliente para que el broker pueda seguir el estado entre conexiones. Recuerde eliminar el registro del topic cuando termine.
Observe que se usa el nombre "persistent_queue" en lugar del más habitual clean_session de MQTT. Esto reduce los comportamientos inesperados cuando se omite la opción "persistent_queue" en la configuración del paso.

## Limitations and Future Improvements {#limitations-and-future-improvements}

### Limitations {#limitations}

Dado que venom inicia cada tarea sin conservar estado entre pasos, y que MQTT perdería mensajes si la suscripción se hiciera después de enviar un mensaje, se necesita un mecanismo para registrar topics persistentes para un paso mqtt posterior. Esto exige que las pruebas añadan algunos pasos adicionales para registrar de antemano una suscripción persistente a un topic y cancelar ese registro al final de una secuencia de pasos.

MQTT podría entregar mensajes tras la conexión pero antes de que el servicio tenga ocasión de enlazar un manejador de suscripción. En los pasos "subscriber" se asocia un manejador con AddRoute antes de la conexión y se suscribe después de conectar. Así se garantiza que no se pierda ningún mensaje y que las suscripciones se comporten como se necesita. Esto es necesario para evitar la pérdida de mensajes cuando se usa una suscripción persistente.

### Future work {#future-work}

* Añadir la posibilidad de obtener el contenido de los mensajes desde archivos en lugar de hacerlo dentro de la configuración YAML. No tiene por qué ser específico de este executor
* Añadir compatibilidad con códecs para poder admitir formatos de serialización distintos de JSON. Debería ser una capacidad que cualquier executor pueda aprovechar, en lugar de resolverse en cada uno por separado
* Añadir compatibilidad con TLS en la interfaz MQTT
