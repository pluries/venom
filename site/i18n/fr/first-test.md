---
translates: first-test
title: Écrire et lancer votre première suite de tests
description: Un pas à pas plus détaillé d'une première suite de tests.
fingerprint: fda7e71e5e326bc03d72f8c9ef6f8b95feabec80763f205ae7362a10a3a571d1
drafted_by: claude
drafted_on: 2026-11-16
status: reviewed
reviewed_by: Christian Stephan
reviewed_on: 2026-11-16
reviewed_fingerprint: fda7e71e5e326bc03d72f8c9ef6f8b95feabec80763f205ae7362a10a3a571d1
---
# Écrire et lancer votre première suite de tests {#write-and-run-your-first-test-suite}

Pour comprendre comment fonctionne Venom, créons et lançons ensemble une première testsuite.

Les premières assertions de cette testsuite vérifient si le site à tester (une API REST publique, l'API OVHcloud par exemple) :
- est accessible (répond avec un code de statut 200)
- répond en moins de 5 secondes
- renvoie une réponse valide (au format JSON)

Commencez par créer votre testsuite dans un fichier nommé par exemple `testsuite.yml`.
Ouvrez-le dans votre éditeur ou IDE favori et remplissez-le avec ce contenu :

```yaml
name: APIIntegrationTest

vars:
  url: https://eu.api.ovh.com

testcases:
- name: GET http testcase, with 5 seconds timeout
  steps:
  - type: http
    method: GET
    url: {{.url}}/1.0/
    timeout: 5
    assertions:
    - result.statuscode ShouldEqual 200
    - result.timeseconds ShouldBeLessThan 1
    - result.bodyjson ShouldContainKey apis
    - result.body ShouldContainSubstring /dedicated/server
    - result.body ShouldContainSubstring /ipLoadbalancing
```

Lancez ensuite votre testsuite avec la commande suivante :

```bash
$ venom run 

 • APIIntegrationTest (testsuite.yml)
 	• GET-http-testcase-with-5-seconds-timeout SUCCESS
```

Vous avez écrit et exécuté votre première testsuite avec l'executor HTTP ! :)
