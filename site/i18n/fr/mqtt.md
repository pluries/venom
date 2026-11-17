---
translates: mqtt
title: MQTT
description: Comment utiliser l'executor MQTT dans une suite de tests venom.
fingerprint: 9ae821f545fded1165c473e275c91dc01e721875681bba4d6c8f7a000da248fd
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# Venom - Executor MQTT {#venom---executor-mqtt}

Étape pour lire et écrire des topics MQTT.

Vous trouverez un exemple dans `tests/mqtt.yml`

## Input {#input}

ClientType peut prendre l'une des valeurs suivantes :
* "publisher"
* "subscriber"
* "persistent_queue"

Comme son nom l'indique, "persistent_queue" crée une file persistante en positionnant la propriété session_clean, afin que des étapes "subscriber" ultérieures puissent récupérer les données. "persistent_queue" va de pair avec "persistSubscription", qui peut valoir true ou false et qui demande ou libère l'abonnement persistant au topic.
Il est important que persistent_queue et subscriber utilisent le même identifiant de client, pour que le broker puisse suivre l'état d'une connexion à l'autre. Pensez à supprimer l'enregistrement du topic une fois terminé.
Notez l'emploi du nom "persistent_queue" plutôt que du clean_session plus habituel de MQTT. Cela réduit les comportements inattendus lorsqu'on omet l'option "persistent_queue" dans la configuration de l'étape.

## Limitations and Future Improvements {#limitations-and-future-improvements}

### Limitations {#limitations}

Comme venom démarre chaque tâche sans conserver d'état d'une étape à l'autre, et comme mqtt perdrait des messages si l'abonnement avait lieu après l'envoi d'un message, il faut un mécanisme pour enregistrer des topics persistants en vue d'une étape mqtt ultérieure. Les tests doivent donc ajouter quelques étapes supplémentaires pour pré-enregistrer l'abonnement à un topic persistant et le désenregistrer à la fin d'une séquence d'étapes.

MQTT peut délivrer des messages après la connexion, mais avant que le service ait eu le temps d'associer un gestionnaire d'abonnement. Pour les étapes "subscriber", nous attachons un gestionnaire avec AddRoute avant la connexion, puis nous nous abonnons après la connexion. Cela doit garantir qu'aucun message n'est perdu et que les abonnements se comportent comme vous le souhaitez. C'est nécessaire pour éviter la perte de messages lorsqu'un abonnement persistant est utilisé.

### Future work {#future-work}

* Ajouter la possibilité d'obtenir le contenu des messages depuis des fichiers plutôt que depuis la configuration yaml. Cela n'a pas besoin d'être propre à cet executor
* Ajouter la prise en charge de codecs pour gérer d'autres formats de sérialisation que json. Ce devrait être une capacité dont n'importe quel executor peut profiter, plutôt qu'une solution à écrire pour chacun
* Ajouter la prise en charge de TLS pour l'interface MQTT
