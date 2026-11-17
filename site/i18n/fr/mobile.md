---
translates: mobile
title: Mobile
description: Comment utiliser l'executor Mobile dans une suite de tests venom.
fingerprint: b5add67e3549852f87f31f8dbe9f13cb7b412228964257c765cad39d54a9af9c
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# Venom - Executor Mobile {#venom---executor-mobile}

Étape qui pilote une application native ou hybride Android ou iOS depuis une suite venom : installer une application, trouver des éléments, taper, saisir du texte, lire du texte, faire défiler, prendre une capture d'écran, ouvrir un lien profond, basculer dans une vue web, répondre à une alerte système iOS et vérifier ce que vous voyez. Elle dialogue avec un serveur [Appium](https://appium.io) 3 qui tourne sur la même
machine que venom, ou sur un Mac que venom atteint par un tunnel SSH. Elle utilise le pilote UiAutomator2 pour un
émulateur Android et le pilote XCUITest pour un simulateur iOS.

Statut : jalons M1 (#2396), M2 (#2397), M3 (#2398) et M4 (#2399). Android sur un émulateur et iOS sur un simulateur, sur la même machine ou sur un Mac atteint par SSH ; vues web, coordonnées, liens profonds et alertes. Les appels iOS sont testés contre un serveur scripté et ont été exécutés sur un simulateur le 2026-10-28 et le 2026-10-29 avec l'application Réglages intégrée et avec UIKitCatalog d'Appium (une installation `.zip`, des alertes, `scroll` avec `until`, une vue web). Non exécutés sur un simulateur : un dossier `.app`, les coordonnées, `openURL`, `hideKeyboard` et le libellé `button` de l'alerte (voir « iOS » ; les notes détaillées d'exécution ne sont pas publiées).

Pour vérifier que cela fonctionne, lancez les tests hermétiques (`go test ./executors/mobile/ -count=1`, aucun appareil nécessaire), puis le
test réel contre votre propre Appium et votre émulateur (voir « Lancer les tests »). Les tests hermétiques dialoguent avec un faux serveur scripté, donc
ils ne peuvent pas prouver qu'Appium 3 accepte toutes les requêtes : seul le test réel le prouve.

## Prérequis de l'hôte {#host-prerequisites}

Pour préparer un Mac pour Android et iOS, voir « Préparer le Mac » plus bas.

- Appium 3 avec le pilote UiAutomator2 (testé avec Appium 3.9.0 et uiautomator2 8.7.2 ; Appium 2.x n'est pas vérifié), démarré de façon à **n'écouter que sur l'adresse de bouclage** (127.0.0.1).
- Un émulateur Android (ou un appareil) visible par Appium.
- Les build-tools du SDK Android (`sdkmanager "build-tools;34.0.0"`) lorsque `mobile.app` désigne un `.apk` : UiAutomator2 a besoin de `aapt2` pour le lire. Appium met en cache sa recherche d'outils, donc redémarrez Appium après les avoir installés. `ANDROID_HOME` doit être défini dans le shell qui démarre Appium.
- venom sur la même machine. M1 installe l'application depuis un chemin de fichier local qu'Appium lit, donc les deux doivent partager un hôte.

L'executor n'a besoin d'aucun drapeau `--relaxed-security` sur le serveur Appium.

## Variables {#variables}

Définissez-les sous `vars: mobile:` dans la suite (ou dans un `.venomrc`, ou avec `--var`). La session s'ouvre une fois par cas de test,
avant sa première étape `mobile`, et se ferme après sa dernière étape, y compris lorsqu'une étape a échoué.

```yaml
vars:
  mobile:
    url: http://127.0.0.1:4723
    platform: android
    device: emulator-5554
    app: ./build/app-debug.apk
```

| Variable | Requise | Signification |
|---|---|---|
| `mobile.url` | oui | Adresse du serveur Appium, `http` ou `https`, sans identifiants dedans. |
| `mobile.platform` | oui | `android` ou `ios`. |
| `mobile.device` | oui | L'appareil ou l'émulateur. Sur Android, il est envoyé à Appium à la fois comme `appium:deviceName` et comme `appium:udid`. Sur iOS, c'est un nom de simulateur (envoyé comme `appium:deviceName` seulement) ou un UDID (envoyé aussi comme `appium:udid`). L'executor ne choisit jamais un appareil à votre place. Utilisez le numéro de série adb, tel que `emulator-5554` ; un nom d'AVD tel que `venom_test` est refusé par Appium (« not in the list of connected devices »). |
| `mobile.osVersion` | non | Envoyé comme `appium:platformVersion`. Le rapport affiche la version que répond le serveur, sinon cette valeur. |
| `mobile.app` | non | Chemin de l'application à installer à l'ouverture de la session : un `.apk` sur Android, un dossier `.app` de simulateur ou son `.zip` sur iOS. Un chemin relatif est résolu par rapport au dossier de la suite. |
| `mobile.appSHA256` | non | SHA-256 attendu du `.apk` ou du `.zip`, en hexadécimal (majuscules ou minuscules). Nécessite `mobile.app`. Refusé avec un dossier `.app`. |
| `mobile.appId` | non | Identifiant de paquet (Android) ou de bundle (iOS) de l'application. Sans `mobile.app`, la session lance cette application déjà installée, et sur iOS il est alors obligatoire. Avec les deux, il sert aussi de valeur par défaut pour `launch` et `terminate`. |
| `mobile.ssh.host` | non | Hôte (nom ou adresse) du Mac. Lorsqu'il est défini, venom atteint le serveur Appium par un tunnel SSH vers le bouclage de l'hôte lui-même, au port de `mobile.url`, et y copie l'application. Sans lui, `mobile.url` est utilisé tel quel. |
| `mobile.ssh.user` | avec `ssh.host` | Utilisateur SSH. |
| `mobile.ssh.key` | avec `ssh.host` | Chemin de la clé privée. Authentification par clé uniquement, pas de mot de passe. |
| `mobile.ssh.passphrase` | non | Phrase secrète de la clé. Déclarez-la sous `secrets`. |
| `mobile.ssh.port` | non | Port SSH, 22 par défaut. |
| `mobile.ssh.knownhosts` / `mobile.ssh.hostkey` | non | Vérification de la clé d'hôte, comme dans l'executor ssh : un fichier known_hosts, ou une clé fixe. Pas les deux. Avec aucune des deux, venom avertit une fois par exécution et ne vérifie pas la clé d'hôte. |
| `mobile.capabilities` | non | Capabilities Appium supplémentaires, sous forme de map. Voir plus bas. |
| `mobile.syncTimeout` | non | Secondes pendant lesquelles l'executor attend un élément, `10` par défaut. |
| `mobile.sessionTimeout` | non | Secondes accordées pour ouvrir la session, `180` par défaut sur Android et `300` sur iOS, où la première session construit WebDriverAgent. |
| `mobile.idleTimeout` | non | Secondes pendant lesquelles le serveur garde la session ouverte sans commande, `120` par défaut (envoyé comme `appium:newCommandTimeout`). |

Un nombre négatif pour l'un des trois délais est refusé. Une valeur incorrecte est refusée avant l'envoi de toute requête.

### Le fichier de l'application et son SHA-256 {#the-app-file-and-its-sha-256}

Avec `mobile.app`, venom lit le fichier une fois et calcule son SHA-256. Si `mobile.appSHA256` est défini et diffère, le
cas de test échoue **avant l'ouverture de toute session**, et le message donne les deux empreintes. L'empreinte calculée est toujours affichée dans
le rapport (`appsha256`), de sorte qu'un rapport indique quelle version a tourné. `mobile.appSHA256` sans `mobile.app` est refusé.

Sans tunnel SSH, l'empreinte porte sur les octets que venom a lus, et Appium installe ensuite depuis le chemin : un fichier remplacé
entre-temps ne serait pas détecté. Avec `mobile.ssh.*`, le fichier est copié sur le Mac et l'empreinte porte sur les octets envoyés
(voir « Atteindre le Mac par SSH »).

### Capabilities supplémentaires {#extra-capabilities}

`mobile.capabilities` ajoute des capabilities à la requête de session. Une capability que l'executor définit lui-même est refusée
avant toute requête, avec un message qui nomme la variable qui la gère. Les noms sont comparés sans le préfixe
`appium:`, et à l'intérieur de `appium:options` aussi.

| Capability | Définie par |
|---|---|
| `platformName` | `mobile.platform` |
| `deviceName`, `udid` | `mobile.device` |
| `platformVersion` | `mobile.osVersion` |
| `app` | `mobile.app` |
| `appPackage`, `bundleId` | `mobile.appId` |
| `automationName` | l'executor (UiAutomator2 sur Android, XCUITest sur iOS) |
| `newCommandTimeout` | `mobile.idleTimeout` |
| `autoAcceptAlerts`, `autoDismissAlerts` | l'action alert : l'étape gère les alertes, donc la session ne doit pas le faire |

## Entrée {#input}

```yaml
  - action (mandatory): one of the 17 actions below
  - accessibilityId, text, id, xpath, uiautomator, css, name, predicate, classChain (one of them, see "Locators"): which element the step targets
  - choice (alert): accept (default) or dismiss, in any case
  - button (alert): the label of the button to press, instead of the default one
  - syncTimeout (optional): seconds to wait for the element in this step, overrides mobile.syncTimeout
  - value (fill): the text to type
  - clear (fill): true to clear the field first
  - attribute (read): read this attribute instead of the text
  - state (waitFor): present (default) or absent
  - direction (swipe, scroll): up, down, left or right
  - until (scroll): true to scroll until the element is visible
  - percent (swipe, scroll): the part of the area to cover, above 0 and at most 1, default 0.75
  - x, y (tap, swipe): the point to tap, or where the swipe starts; 0 is valid. Not with a locator
  - toX, toY (swipe): where a swipe from a point ends; give both, and no direction or percent
  - unit (tap, swipe): the unit of x, y, toX, toY: pixels (default, also px) or percent. Percent is of the window rect (0 to 100)
  - appId (launch, terminate): the package id, default mobile.appId
  - path (screenshot): the file to write, relative to the directory venom runs in
  - context (context): native, a web view name starting with WEBVIEW, or empty for the first web view
  - url (openURL): the deep link, scheme://..., at most 2048 bytes, no spaces or control characters
```

`text` est un **localisateur** (il trouve un élément par son texte visible). Le texte que vous saisissez est `value`.

### Localisateurs {#locators}

Une étape qui cible un élément donne **exactement un** localisateur. Zéro ou deux est une erreur avant toute requête.

| Champ | Trouve l'élément par |
|---|---|
| `accessibilityId` | son identifiant d'accessibilité (description du contenu) |
| `text` | son texte visible exact (un `text` de UiSelector) ; les guillemets et les barres obliques inverses qu'il contient sont échappés |
| `id` | son identifiant de ressource |
| `xpath` | un XPath sur la hiérarchie des vues |
| `uiautomator` | un sélecteur UiAutomator, envoyé tel qu'écrit |
| `css` | un sélecteur CSS, uniquement dans une vue web |
| `name` | iOS uniquement : le nom de l'élément |
| `predicate` | iOS uniquement : une chaîne NSPredicate, envoyée telle qu'écrite |
| `classChain` | iOS uniquement : une class chain iOS, envoyée telle qu'écrite |

`id` et `uiautomator` sont réservés à Android ; `name`, `predicate` et `classChain` sont réservés à iOS. Un type de l'autre plateforme
est refusé avant toute requête, avec un message qui indique quoi utiliser à la place. La correspondance iOS est décrite dans « iOS ».

Un élément est attendu : l'executor réinterroge toutes les 500 ms jusqu'à ce qu'il apparaisse ou que l'attente soit écoulée (10 s par défaut,
ou le `syncTimeout` de l'étape, ou `mobile.syncTimeout`). Si l'élément n'apparaît jamais, l'échec nomme le localisateur et
l'attente. Un élément trouvé du premier coup n'ajoute aucun délai.

## Actions et résultats {#actions-and-results}

| Action | Nécessite | Ce qu'elle fait | Clés de résultat qu'elle renseigne |
|---|---|---|---|
| `tap` | un localisateur, ou `x` et `y` (avec `unit` facultatif) | clique sur l'élément, ou touche le point | `found` (avec un localisateur) |
| `fill` | un localisateur, `value`, `clear` facultatif | saisit `value` dans l'élément, après l'avoir vidé lorsque `clear: true`. Une `value` vide ne saisit rien | `found` |
| `clear` | un localisateur | vide l'élément | `found` |
| `read` | un localisateur, `attribute` facultatif | lit le texte de l'élément, ou l'attribut | `found`, `text` (sans attribut) ou `value` (attribut ; un attribut absent se lit comme vide) |
| `count` | un localisateur | compte les correspondances à cet instant, sans attendre. Zéro est une réponse, pas un échec | `count`, `found` (vrai lorsque supérieur à zéro) |
| `waitFor` | un localisateur, `state` facultatif | attend que l'élément soit présent (par défaut) ou absent | `found` |
| `swipe` | `direction`, localisateur et `percent` facultatifs ; ou `x`, `y`, `toX`, `toY` (avec `unit` facultatif) pour glisser d'un point à un autre | balaie à l'intérieur de l'élément, ou à l'intérieur de la zone visible de l'appareil lorsqu'il n'y a pas de localisateur ; cette zone est la fenêtre réduite de 10 pour cent de chaque côté, afin de ne jamais toucher les bords des gestes système | `found` lorsqu'un localisateur a été donné |
| `scroll` | `direction`, localisateur et `percent` facultatifs | fait défiler à l'intérieur de l'élément ou de la zone visible (la fenêtre réduite de 10 pour cent de chaque côté, afin de ne jamais toucher les bords des gestes système). Avec `until: true` et un localisateur, il fait défiler la zone visible jusqu'à ce que cet élément soit à l'écran, 10 défilements au plus | `found` lorsqu'un localisateur a été donné |
| `back` | rien | appuie sur le bouton Retour | aucune |
| `hideKeyboard` | rien | masque le clavier virtuel. Si aucun n'est affiché, ce n'est pas un échec | aucune |
| `launch` | `appId` ou `mobile.appId` | amène l'application au premier plan (`mobile: activateApp`) | aucune |
| `terminate` | `appId` ou `mobile.appId` | arrête l'application (`mobile: terminateApp`) | aucune |
| `screenshot` | `path` facultatif | prend une capture d'écran. Avec `path`, y écrit le PNG (mode 0600) | `found`, `screenshot` (le chemin) |
| `source` | rien | renvoie la hiérarchie des vues en XML | `found`, `source` |
| `context` | `context` : `native`, un nom de vue web (`WEBVIEW_...`), ou vide pour la première vue web | bascule entre l'application native et une vue web. Avec une vue web, il attend (l'attente de l'étape) qu'elle soit listée. Un localisateur `css` nécessite une vue web | `context` (l'actif), `contexts` (la liste donnée par le serveur ; vide après un retour en natif) |
| `openURL` | `url` | ouvre un lien profond : `url` scheme://..., restreint au paquet de l'application lorsqu'il est connu (`mobile: deepLink`) | aucune |
| `alert` | `choice` et `button` facultatifs | iOS uniquement : accepte ou rejette l'alerte système ouverte (voir « iOS ») | `text` (le texte de l'alerte) |

Une action inconnue est refusée avec la liste des actions prises en charge. Lorsqu'une action diffère sur iOS, voir « Différences avec Android ».

## Sortie {#output}

```yaml
  - result.text: the text read (read)
  - result.value: the attribute read (read with attribute)
  - result.count: the number of matches (count)
  - result.found: true when the step found its element, or the screen content
  - result.screenshot: the path written (screenshot with path)
  - result.source: the view hierarchy as XML (source)
  - result.timeseconds: duration of the step
  - result.err: the failure of the step, empty when it worked
  - result.platform, result.device, result.osversion: where the test ran
  - result.app, result.appsha256, result.appid: what ran
```

Les faits de l'exécution (`platform`, `device`, `osversion`, `app`, `appsha256`, `appid`) sont répétés dans **chaque** résultat, de sorte que chaque
cas de test d'un rapport indique quelle version a tourné sur quel appareil. Une clé sans valeur est absente du résultat.

Assertion par défaut : `result.err ShouldBeEmpty`.

### Erreurs {#errors}

- Un **mauvais usage** (deux localisateurs, une action inconnue, une `direction` incorrecte, un paramètre manquant) est une erreur venom avant l'envoi de toute
  requête. L'étape échoue avec un message qui nomme la cause.
- Un **échec sur le serveur ou l'appareil** (élément introuvable, commande refusée, serveur injoignable, appel qui
  se bloque) arrive dans `result.err` sous la forme `action <name>: <message>`. L'étape échoue sur l'assertion par défaut et l'exécution continue.
  La trace de pile d'Appium n'est jamais placée dans le résultat.

## Vues web et applications hybrides {#web-views-and-hybrid-apps}

Une application hybride affiche des pages web à l'intérieur d'un écran natif. L'action `context` bascule entre les deux.

- `context: native` revient à l'application native.
- `context: WEBVIEW_<package>` bascule vers cette vue web.
- Un `context` vide bascule vers la première vue web que le serveur liste.

Pour une vue web, l'executor attend jusqu'à l'attente de l'étape (`syncTimeout`, `mobile.syncTimeout`) qu'elle soit listée, de sorte qu'une
page qui se charge tard est trouvée. Si aucune n'apparaît à temps, l'échec liste les contextes que le serveur a bien signalés. Le
résultat contient `context` (l'actif) et `contexts` (la liste donnée par le serveur).

Dans une vue web, la recherche est gérée par chromedriver, donc `css` et `xpath` sont les localisateurs à utiliser. `text`,
`accessibilityId` et `uiautomator` sont des stratégies natives et sont refusés par le serveur dans une vue web
(vérifié sur un appareil, 2026-10-28 : `accessibilityId` est revenu en erreur) ; ce que signifie `id` à cet endroit (l'id HTML) n'est pas vérifié. Après
`context: native`, un localisateur natif fonctionne comme avant. `css` dans le contexte natif est refusé avec un message avant toute recherche. `source` renvoie le HTML de la page dans une vue web et la hiérarchie
XML dans le contexte natif.

```yaml
      - type: mobile
        action: context
        context: WEBVIEW_com.example.app
      - type: mobile
        action: fill
        css: "#email"
        value: demo-user
      - type: mobile
        action: tap
        css: "button[type=submit]"
      - type: mobile
        action: context
        context: native
      - type: mobile
        action: waitFor
        accessibilityId: welcome_banner
```

### Ce dont la vue web a besoin {#what-the-web-view-needs}

- **Le débogage doit être activé.** L'application doit activer le débogage des WebView. Sans lui, le serveur ne liste que `NATIVE_APP`, et
  un basculement vers une vue web échoue en donnant cette liste.
- **Un chromedriver correspondant.** UiAutomator2 pilote une vue web par un chromedriver dont la version correspond à celle de la WebView sur
  l'appareil. Une discordance fait échouer le basculement sur le serveur. Définissez `appium:chromedriverExecutable` (un chemin sur l'hôte
  Appium) par `mobile.capabilities`, ou dans la configuration propre du serveur :

  ```yaml
  vars:
    mobile:
      capabilities:
        appium:chromedriverExecutable: /opt/chromedrivers/chromedriver-114
  ```
- **Le téléchargement automatique est le choix de l'opérateur.** Le serveur Appium peut récupérer lui-même un chromedriver
  (`chromedriver_autodownload`). Cela exige une option de serveur que l'opérateur définit sur le serveur Appium. venom ne
  l'envoie jamais et n'a jamais besoin de `--relaxed-security` ni de `--allow-insecure`.

## Applications sans étiquettes et coordonnées {#unlabelled-apps-and-coordinates}

Une application qui ne publie aucune étiquette d'accessibilité (certaines versions release de Flutter, des jeux, des vues dessinées sur mesure) ne peut être
atteinte que par des coordonnées. Si l'application a des étiquettes, des ids ou du texte, préférez un localisateur : une coordonnée casse lorsque la mise en page,
la taille d'écran ou la police change.

`tap` et `swipe` prennent un point au lieu d'un localisateur :

| Champ | Signification |
|---|---|
| `x`, `y` | le point à toucher, ou le point de départ du balayage. 0 est une valeur valide |
| `toX`, `toY` | le point d'arrivée du balayage. Donnez les deux, et ni `direction` ni `percent` |
| `unit` | `pixels` (par défaut, aussi `px`) ou `percent` |

Avec `unit: percent`, le point est une part, de 0 à 100, du rectangle de la fenêtre : `x: 50` et `y: 50` est le centre. Sur une
fenêtre de 1080x2400, cela envoie le point 540, 1200. Les pixels sont envoyés tels quels et le serveur les borne. Un point avec un
localisateur, `x` sans `y`, une valeur négative, NaN ou un infini, une valeur en pixels supérieure à 10 millions, ou un pourcentage supérieur à 100
est refusé avant toute requête.

Les coordonnées en pourcentage et `swipe` ou `scroll` sans localisateur lisent la taille de la fenêtre auprès du serveur. Dans une vue web, cette
taille peut être exprimée dans les unités du navigateur, et non en pixels de l'appareil : revenez en natif (`context: native`) avant d'utiliser
des coordonnées en pourcentage ou un `swipe` et un `scroll` sans localisateur. Non vérifié sur un appareil : l'essai manuel a utilisé des coordonnées en pixels et en pourcentage
dans le contexte natif uniquement.

```yaml
      - type: mobile
        action: tap
        x: 50
        y: 50
        unit: percent
      - type: mobile
        action: swipe
        x: 540
        y: 1800
        toX: 540
        toY: 600
```

## Liens profonds {#deep-links}

`openURL` ouvre un lien dans l'application, par le `mobile: deepLink` d'Appium :

```yaml
      - type: mobile
        action: openURL
        url: myapp://orders/42
```

- `url` doit ressembler à `scheme://...`, sans espaces ni caractères de contrôle, d'au plus 2048 octets. Tout autre cas est
  refusé avant toute requête, et le message ne répète pas le lien (il peut contenir un jeton).
- Le lien est restreint au paquet de l'application (le `appId` de l'étape, sinon `mobile.appId`) lorsqu'il est connu.
- Les schémas personnalisés sont acceptés : c'est tout l'intérêt d'un lien profond. Le lien est transmis au `mobile: deepLink` d'Appium, qui
  l'exécute par `am start` ; venom ne construit aucune commande shell. Dans la pile testée (Appium 3.9.0, uiautomator2 8.7.2,
  émulateur Android 14), cinq liens contenant des métacaractères du shell ont atteint le serveur, ont échoué faute de gestionnaire,
  et n'ont créé aucun fichier sur l'appareil. Un Appium ou un pilote plus ancien peut ne pas les protéger par des guillemets : gardez les liens profonds issus d'entrées non fiables
  hors des suites.

## Limites {#limits}

| Limite | Valeur |
|---|---|
| Un appel au serveur | 60 s |
| Ouverture de la session | 180 s (`mobile.sessionTimeout`) |
| Session inactive sur le serveur | 120 s (`mobile.idleTimeout`) |
| Pause entre deux tentatives de recherche d'un élément | 500 ms |
| Attente par défaut d'un élément | 10 s (`mobile.syncTimeout`, `syncTimeout`) |
| Défilements de `scroll` avec `until` | 10 |
| Une capture d'écran décodée | 12 MiB |
| Fermeture de la session | 15 s |
| Chacune des deux requêtes de diagnostic | 15 s |

Une redirection du serveur n'est pas suivie. Le `timeout:` propre à l'étape s'applique toujours en plus de ces limites.

## Diagnostics d'une étape en échec {#diagnostics-on-a-failing-step}

Lorsqu'une étape échoue sur le serveur ou l'appareil, l'executor enregistre deux fichiers dans le répertoire où venom s'exécute :

- `screen.png` : une capture d'écran de l'appareil ;
- `screen.dump.xml` : la hiérarchie des vues.

**Attention : ces deux fichiers montrent ce qui est à l'écran, y compris un secret que vous avez saisi ou que l'application affiche.** Ils ne sont pas
couverts par le masquage des secrets des rapports de venom. Ils sont écrits avec le mode 0600 dans le dossier de travail. Ne les publiez pas
comme artefacts de build. L'étape en échec suivante les remplace ; une étape réussie n'écrit ni ne supprime rien. L'échec de la récupération d'un
fichier est un avertissement dans le journal et ne masque pas l'échec propre de l'étape. Une erreur de mauvais usage, ou une session
que le serveur n'a plus, n'écrit rien.

Deux exécutions de venom lancées dans le **même répertoire** écrasent mutuellement leurs fichiers. Lancez-les depuis des répertoires différents.

## Atteindre le Mac par SSH {#reaching-the-mac-through-ssh}

Laissez Appium écouter sur le bouclage du Mac (`127.0.0.1:4723`) ; il n'a pas de mot de passe propre. Définissez `mobile.ssh.host`, `mobile.ssh.user` et `mobile.ssh.key` et gardez `mobile.url` comme l'adresse que voit le Mac (`http://127.0.0.1:4723`) : venom ouvre le tunnel au démarrage du cas de test et le ferme à sa fin. Le tunnel n'écoute que sur `127.0.0.1` et appelle toujours le bouclage du Mac.

- **Clé d'hôte :** définissez `mobile.ssh.knownhosts` ou `mobile.ssh.hostkey`. Une clé qui ne correspond pas refuse la connexion avant toute requête.
- **Le fichier de l'application :** avec un tunnel, `mobile.app` est copié vers `/tmp/venom-mobile-<random>/app.apk` (`app.zip` sur iOS) sur le Mac, installé à partir de là et supprimé à la fin de la session. Aucune valeur de la suite n'apparaît dans une commande exécutée sur le Mac. `mobile.appSHA256` est vérifié sur les octets envoyés ; une discordance fait échouer avant toute session et supprime la copie. Une exécution tuée peut laisser le dossier derrière elle : cherchez `/tmp/venom-mobile-*`.
- **Limites :** une URL `https://` passant par le tunnel est vérifiée par rapport à `127.0.0.1`, donc utilisez `http://` dans le tunnel. La connexion et la poignée de main SSH sont bornées par `mobile.sessionTimeout` (180 s par défaut), de sorte qu'un hôte qui ne répond pas fait échouer le Setup au lieu de le bloquer.
- **Http simple sans tunnel :** une URL `http://` vers une autre machine affiche un avertissement par exécution ; une URL de bouclage n'en affiche aucun.

## Notes de sécurité {#security-notes}

- **Appium doit écouter uniquement sur le bouclage.** Un serveur Appium exposé sur le réseau permet à quiconque l'atteint de contrôler
  l'émulateur et la machine. En M1, venom se connecte à Appium sur le même hôte.
- L'application est installée à partir d'un **fichier local**, donc Appium et venom doivent partager un hôte en M1.
- **Les captures d'écran, le dump et `result.source` montrent ce qui est à l'écran, y compris un secret que vous avez saisi.** Ils ne sont PAS
  couverts par le masquage des secrets des rapports de venom, la même limite que pour la trace de l'executor web. Gardez `screen.png` et
  `screen.dump.xml` hors des artefacts publiés et des dossiers de build partagés.
- `fill` ne répète jamais le texte saisi dans un résultat ni dans le journal d'appels de `-v`. Avec `-v`, chaque appel au serveur est journalisé (méthode, chemin, durée ; jamais un corps).
- Seuls les points d'accès d'Appium 3 sont utilisés, et l'executor n'a besoin d'aucun `--relaxed-security`.
- **Le contenu de la page d'une vue web et le texte qui y est saisi peuvent apparaître dans les captures d'écran et dans `result.source`**, comme tout
  écran natif, et le masquage des secrets ne les couvre pas. Le texte d'une page lu avec `read` est une valeur de résultat comme une autre.
- **Une URL de lien profond est transmise au `mobile: deepLink` d'Appium, qui l'exécute par `am start`.** venom ne
  construit jamais lui-même de commande shell ; le lien est contrôlé pour sa forme et sa longueur, et restreint au paquet de l'application lorsqu'il est connu.
  Appium a protégé par des guillemets les métacaractères du shell dans la pile testée (Appium 3.9.0, uiautomator2 8.7.2, émulateur Android 14 arm64,
  mesuré le 2026-10-28). Un Appium ou un pilote plus ancien peut ne pas le faire, donc gardez les liens profonds issus d'entrées non fiables hors des suites.
- **Le chromedriver tourne sur l'hôte Appium**, et le choix du binaire reste celui de l'opérateur. venom n'envoie aucune
  option qui active `--relaxed-security` ou `--allow-insecure`, et le serveur n'a besoin d'aucune des deux.
- **Le texte d'une alerte iOS** va dans `result.text` et nulle part ailleurs. Il n'est écrit ni dans une ligne de journal ni dans une erreur, mais
  c'est une valeur de résultat comme une autre, donc un secret qu'il contient n'est masqué que s'il s'agit d'un secret déclaré.
- **Un dossier `.app` est copié vers le Mac sous le nom `app.zip`**, dans un dossier nommé par venom (`/tmp/venom-mobile-<random>`, mode 0700),
  sans aucune valeur de la suite dans les commandes exécutées là. venom refuse un `.app` qui contient un lien symbolique, et ne calcule aucune empreinte d'un
  dossier : son intégrité n'est pas vérifiée. Donnez un `.zip` et `mobile.appSHA256` lorsqu'elle doit l'être.
- **Le pilote XCUITest et WebDriverAgent tournent sur le Mac** et relèvent de l'installation de l'opérateur. venom ne les récupère ni ne les construit ;
  la première session y construit WebDriverAgent.

## iOS {#ios}

Avec `mobile.platform: ios`, la même suite pilote une application sur un **simulateur** iOS par le pilote XCUITest. Un vrai
iPhone n'est pas couvert. Les appels iOS sont implémentés d'après le comportement documenté du pilote XCUITest et testés contre un
serveur scripté. Ils ont été exécutés sur un simulateur (iPhone 17, iOS 26.5, Appium 3.9.0, pilote XCUITest 12.22.3), en deux exécutions :

- 2026-10-28, l'application Réglages intégrée lancée par son bundle id, suite `examples/handrun-settings.yml`, 3 cas de test sur 3
  réussis : ouverture de session, `waitFor`, `tap`, balayage par direction, `terminate`, `launch`, les localisateurs `text`, `name`,
  `predicate` et `classChain`, `count`, et `alert` sans alerte ouverte.
- 2026-10-29, UIKitCatalog v4.0.5 d'Appium fourni sous la forme `UIKitCatalog.zip` avec `mobile.appSHA256`, par le tunnel SSH,
  suite `examples/handrun-uikitcatalog.yml`, statut final PASS : l'installation du `.zip`, `alert` accepter (le texte de l'alerte
  est revenu dans `result.text`) et rejeter, `scroll` avec `until`, et une vue web (`context` a signalé un contexte
  `WEBVIEW`, `css: body` a compté plus de 0, `context: native` est revenu). La vue web iOS n'a pas eu besoin de chromedriver.

Non exécutés sur un simulateur : un dossier `.app`, les coordonnées, `openURL`, `hideKeyboard`, le libellé `button` de l'alerte, les
coordonnées en pourcentage. Les notes détaillées d'exécution ne sont pas publiées.
Les lignes d'une liste de type Réglages ne sont pas des `XCUIElementTypeCell` sur iOS 26 : écrivez une class chain sous la forme `**/*[...]`, et non avec un type de cellule.
La première session a pris plusieurs minutes (construction de WebDriverAgent), ce qui explique que le délai de session par défaut soit de 300 s.

### Prérequis sur le Mac {#prerequisites-on-the-mac}

- Xcode et ses outils en ligne de commande, et au moins un simulateur.
- Appium 3 avec le pilote XCUITest : `appium driver install xcuitest`. Laissez Appium sur l'adresse de bouclage, comme pour Android.
- La première session construit WebDriverAgent et prend plusieurs minutes. C'est pourquoi `mobile.sessionTimeout` vaut 300 s par défaut sur iOS.
- Un simulateur n'a pas besoin de signature de code.
- Sur un Mac Apple Silicon, l'application doit avoir une tranche arm64, et les images d'émulateur Android y sont aussi arm64. Un `.app`
  iOS qui n'a que des tranches i386 ou x86_64 est refusé par Appium (« does not support the arm64 Simulator architecture ») ; un
  APK qui n'a que des bibliothèques `armeabi-v7a` et `x86` échoue avec `INSTALL_FAILED_NO_MATCHING_ABIS` (il lui faut `arm64-v8a`, ou
  aucune bibliothèque native). TheApp v1.10.0 (2019) de Cloud Grey a échoué des deux façons sur le Mac du mainteneur.
- Un exemple prêt à l'emploi pour essayer iOS : UIKitCatalog d'Appium (`UIKitCatalog.zip`, une version simulateur avec des tranches arm64 et x86_64,
  bundle id `com.example.apple-samplecode.UICatalog`), depuis la page des releases de
  <https://github.com/appium/ios-uicatalog>. La suite `examples/handrun-uikitcatalog.yml` s'y exécute.

### Paramètres qui changent sur iOS {#settings-that-change-on-ios}

| Paramètre | Sur iOS |
|---|---|
| `mobile.device` | Un nom de simulateur (`iPhone 16`), ou son UDID. Un UDID est aussi envoyé comme `appium:udid` ; un nom seulement comme `appium:deviceName`. |
| `mobile.osVersion` | Envoyé comme `appium:platformVersion`, comme sur Android. |
| `mobile.app` | Un dossier `.app` de simulateur, ou son `.zip`. Un **`.ipa` est une version pour appareil et ne s'installe pas sur un simulateur** : il est refusé avant toute requête. Construisez plutôt un `.app` de simulateur. |
| `mobile.appSHA256` | Uniquement pour un `.zip`. Un dossier `.app` n'a pas de fichier unique à hacher, donc le paramètre est refusé avec lui. |
| `mobile.appId` | Le bundle id. Sans `mobile.app`, l'application installée est lancée par son bundle id et `mobile.appId` est alors obligatoire. |
| `mobile.sessionTimeout` | 300 s par défaut. |

Par SSH, un dossier `.app` est compressé en zip sur la machine qui exécute venom et copié vers le Mac sous le nom `app.zip` ; un `.zip` est copié
tel quel. Voir « Atteindre le Mac par SSH ».

### Localisateurs sur iOS {#locators-on-ios}

| Champ | Trouve l'élément par |
|---|---|
| `accessibilityId` | son identifiant d'accessibilité |
| `text` | son libellé : envoyé comme le prédicat `label == "<text>"`, les guillemets, barres obliques inverses et sauts de ligne du texte étant échappés |
| `name` | son nom |
| `predicate` | une chaîne NSPredicate, envoyée telle qu'écrite |
| `classChain` | une class chain iOS, envoyée telle qu'écrite |
| `xpath` | un XPath sur la hiérarchie des vues |
| `css` | un sélecteur CSS, uniquement dans une vue web |

`id` et `uiautomator` sont refusés sur iOS ; `name`, `predicate` et `classChain` sont refusés sur Android. Comme sur Android, une
étape donne exactement un localisateur.

### Différences avec Android {#differences-from-android}

| Action | Sur iOS |
|---|---|
| `tap`, `fill`, `clear`, `read`, `count`, `waitFor`, `screenshot`, `source`, `context` | Les mêmes commandes que sur Android. |
| `tap` et `swipe` par `x`, `y` | Le point est en points, et non en pixels. `tap` utilise `mobile: tap`, `swipe` utilise `mobile: dragFromToForDuration` sur une demi-seconde. `unit: percent` se rapporte au rectangle de la fenêtre, que iOS exprime en points. |
| `swipe`, `scroll` par `direction` | `mobile: swipe` et `mobile: scroll`, sur l'élément ou sur toute l'application. `percent` est refusé : XCUITest n'a pas un tel argument. |
| `scroll` avec `until` | Répète `mobile: scroll` jusqu'à 10 fois. Le serveur n'indique pas quand la liste se termine, donc la boucle s'arrête sur l'élément ou sur la borne. |
| `back` | Refusé : iOS n'a pas de touche retour. Touchez le contrôle de retour propre à l'application. |
| `hideKeyboard` | `mobile: hideKeyboard`. Un clavier qui n'est pas affiché n'est pas un échec. |
| `launch`, `terminate` | `mobile: activateApp` et `mobile: terminateApp`, en désignant l'application par `bundleId` (`appId` ou `mobile.appId`). |
| `openURL` | `mobile: deepLink` avec l'`url`, et le `bundleId` lorsqu'il est connu. |
| `alert` | iOS uniquement. Sur Android, elle est refusée. |

### Alertes {#alerts}

Une application iOS peut déclencher une alerte système, par exemple une demande d'autorisation. L'action `alert` lit son texte puis l'accepte
ou la rejette :

```yaml
      - type: mobile
        action: tap
        accessibilityId: enable_location
      - type: mobile
        action: alert
        choice: accept
        assertions:
          - result.err ShouldBeEmpty
          - result.text ShouldContainSubstring Location
      - type: mobile
        action: alert
        choice: dismiss
        button: Don't Allow
```

- `choice` vaut `accept` (par défaut) ou `dismiss`. Toute autre valeur est refusée avant toute requête.
- Sans `button`, l'étape répond par la commande simple d'acceptation ou de rejet. Avec `button`, elle appuie sur le bouton portant
  ce libellé (`mobile: alert`).
- `result.text` contient le texte de l'alerte. **Une alerte peut montrer un secret** (un code, un nom) : le texte n'est jamais écrit dans une ligne
  de journal ni dans un message d'erreur, et le masquage des secrets des rapports de venom ne couvre que les secrets déclarés.
- Sans alerte ouverte, l'étape échoue avec `no alert open` dans `result.err`.
- La session ne doit pas répondre elle-même aux alertes : `autoAcceptAlerts` et `autoDismissAlerts` dans `mobile.capabilities` sont
  refusés, car l'étape `alert` les gère.
- Un lien profond sur un simulateur peut déclencher une alerte système de confirmation (« Open in ...? »). Faites suivre un `openURL` d'une étape `alert`
  lorsque votre application s'ouvre de cette façon, faute de quoi l'étape suivante trouvera l'alerte sur son chemin.

## Pas encore {#not-yet}

Prévu, refusé ou absent aujourd'hui :

- Un vrai appareil iOS : seuls les simulateurs sont dans le périmètre.
- Flutter sur le simulateur iOS n'existe qu'en version debug : Flutter ne construit aucune application release pour le simulateur.
- Les applications sans tranche arm64, sur un Mac Apple Silicon.
- Connexion par mot de passe sur SSH : authentification par clé uniquement.
- Un chromedriver téléchargé pour vous : pour une vue web Android, l'opérateur en fournit un correspondant.
- Pas encore exécutés sur un simulateur (voir « iOS » ; les notes détaillées d'exécution ne sont pas publiées) : l'installation d'un dossier `.app` (seul un
  `.zip` a tourné), les coordonnées `x`/`y` et en pourcentage, `openURL`, `hideKeyboard`, et le libellé `button` de l'alerte. La suite
  `examples/handrun-ios.yml` est écrite pour l'installation d'un dossier `.app`.

## Exemple {#example}

Chaque étape ci-dessous est une forme que les tests de cet executor exécutent.

```yaml
name: Mobile smoke
vars:
  mobile:
    url: http://127.0.0.1:4723
    platform: android
    device: emulator-5554
    osVersion: "14"
    app: ./build/app-debug.apk
    appSHA256: <sha-256 of the build, in hex>
    appId: com.example.app
    syncTimeout: 5

testcases:
  - name: the home screen is usable
    steps:
      - type: mobile
        action: waitFor
        accessibilityId: login_button
        syncTimeout: 20
      - type: mobile
        action: count
        id: com.example.app:id/item
        assertions:
          - result.count ShouldBeGreaterThan 0
      - type: mobile
        action: fill
        id: com.example.app:id/username
        value: demo-user
        clear: true
      - type: mobile
        action: hideKeyboard
      - type: mobile
        action: tap
        accessibilityId: login_button
      - type: mobile
        action: read
        id: com.example.app:id/title
        assertions:
          - result.err ShouldBeEmpty
          - result.text ShouldEqual Welcome
      - type: mobile
        action: scroll
        direction: down
        until: true
        text: Settings
      - type: mobile
        action: screenshot
        path: home.png
      - type: mobile
        action: back
      - type: mobile
        action: terminate
      - type: mobile
        action: launch
```

## Préparer le Mac {#set-up-the-mac}

Utilisez ceci lorsque venom tourne sur une autre machine (Windows, Linux) et que l'émulateur et le simulateur tournent sur un Mac. Tout ce qui suit
s'installe sur le Mac. venom n'en installe rien.

1. **Appium 3** avec les deux pilotes : `appium driver install uiautomator2` et `appium driver install xcuitest`. Démarrez-le de façon
   qu'il n'écoute que sur l'adresse de bouclage : `appium --address 127.0.0.1 --port 4723`. Définissez `ANDROID_HOME` dans le
   shell qui le démarre.
2. **Android :** le SDK Android (platform-tools, `build-tools;34.0.0`) et une image d'émulateur **arm64** sur un Mac Apple Silicon.
   Démarrez l'émulateur avant l'exécution.
3. **iOS :** Xcode, ses outils en ligne de commande et au moins un simulateur. Démarrez-le, ou laissez Appium le démarrer. La première session
   construit WebDriverAgent et prend plusieurs minutes.
4. **Vues web Android :** un chromedriver qui correspond à la WebView de l'émulateur, et son chemin dans
   `appium:chromedriverExecutable` (voir « Ce dont la vue web a besoin »). Les vues web iOS n'en ont pas besoin.
5. **SSH :** une clé que le Mac accepte pour l'utilisateur que vous donnez dans `mobile.ssh.user`. La connexion par mot de passe n'est pas prise en charge.

Pièges rencontrés sur un vrai Mac :

- **La première exécution d'un outil peut se bloquer sur SSH.** Gatekeeper de macOS peut demander une confirmation que personne ne voit dans une
  session SSH, et le binaire attend. Lancez `flutter --version` (ou l'outil concerné) une fois dans un Terminal sur le Mac lui-même
  et répondez à l'invite. Les outils installés avec un cask Homebrew peuvent aussi porter l'indicateur de quarantaine. Le retirer avec
  `xattr -dr com.apple.quarantine <folder>` est votre décision : ne le faites que pour un dossier de confiance.
- **Les applications ont besoin d'une tranche arm64** sur un Mac Apple Silicon (voir « Prérequis sur le Mac »).
- **Flutter ne construit que des applications debug pour le simulateur iOS.** Une version release de Flutter n'est possible que sur Android.
- **CocoaPods a besoin d'une locale UTF-8** (`export LANG=en_US.UTF-8 LC_ALL=en_US.UTF-8`), par exemple lorsqu'il s'exécute depuis un
  script lancé par SSH.

## Applications d'exemple {#sample-apps}

Des applications publiques utilisées par les essais du mainteneur. Chacune est prête pour arm64.

| Application | Plateforme | Provenance |
|---|---|---|
| ApiDemos | Android | `ApiDemos-debug.apk` d'Appium, depuis les releases de <https://github.com/appium/android-apidemos> |
| UIKitCatalog v4.0.5 | simulateur iOS | `UIKitCatalog.zip`, depuis les releases de <https://github.com/appium/ios-uicatalog> |
| Application compteur Flutter | les deux | `flutter create`, construite par `examples/build-sample-apps.sh` (APK release Android, application debug pour simulateur iOS) |
| Application React Native | les deux | `react-native init`, construite par le même script (APK release Android, application Release pour simulateur iOS) |

Le script s'exécute sur le Mac. Il installe le SDK Flutter et CocoaPods s'ils manquent, construit les quatre applications à partir des
modèles propres aux frameworks et les liste à la fin. Lancez d'abord `flutter --version` une fois à la main (voir « Préparer le Mac »).

Les suites sont dans `examples/` : `examples/flutter.yml` et `examples/rn.yml` s'exécutent sans modification sur les deux plateformes, avec le fichier de variables de
chaque plateforme (`examples/android.vars.example.yml`, `examples/ios.vars.example.yml` : renseignez les valeurs `<...>`) ; `examples/failing.yml` échoue
exprès pour montrer les fichiers de diagnostic. Les notes détaillées d'exécution ne sont pas publiées.

## Lancer les tests {#run-the-tests}

Tests hermétiques (sans appareil ni réseau ; ils font partie de `go test ./...`) :

```bash
go test ./executors/mobile/ -count=1
```

Test réel, derrière le build tag `mobile`. Il nécessite Appium 3 avec UiAutomator2 sur le bouclage et un émulateur, et il lit
ses paramètres dans l'environnement. Il s'ignore avec un message lorsqu'un paramètre n'est pas défini.

```bash
export VENOM_MOBILE_URL=http://127.0.0.1:4723
export VENOM_MOBILE_DEVICE=emulator-5554
export VENOM_MOBILE_APP=/path/to/sample.apk        # or VENOM_MOBILE_APPID=<package id of an installed app>
export VENOM_MOBILE_LOCATOR_KIND=accessibilityId   # accessibilityId, text, id, xpath or uiautomator
export VENOM_MOBILE_LOCATOR_VALUE=<an element on the first screen>
export VENOM_MOBILE_EXPECT_TEXT=<its text>         # optional
export VENOM_MOBILE_OSVERSION=14                   # optional
go test -tags mobile ./executors/mobile/ -run Real -v -count=1
```

Il ouvre une session, trouve et lit l'élément, vérifie qu'une étape réussie ne laisse aucun diagnostic, fait échouer une étape exprès
et vérifie que `screen.png` et `screen.dump.xml` apparaissent, puis ferme la session.

Deux autres tests réels partagent les variables de session ci-dessus (`VENOM_MOBILE_URL`, `_DEVICE`, `_APP` ou `_APPID`) :

| Test | Variables supplémentaires | Ce qu'il vérifie |
|---|---|---|
| `TestReal_CoordinatesAndDeepLink` | aucune | des balayages en pourcentage et en pixels et un toucher à 50 % / 50 % réussissent ; `openURL venomtest://nothing/handles/this` revient avec un `result.err` non vide |
| `TestReal_IOSSession` | `VENOM_MOBILE_PLATFORM=ios`, `VENOM_MOBILE_DEVICE` un nom de simulateur, `VENOM_MOBILE_APPID` un bundle id (ou `VENOM_MOBILE_APP`), `VENOM_MOBILE_IOS_LOCATOR` (identifiant d'accessibilité sur le premier écran, `General` par défaut pour l'application Réglages) | `waitFor`, balayage vers le haut, `terminate`, `launch`, et `alert` sans alerte ouverte répond `no alert open` ; ignoré sauf si `VENOM_MOBILE_PLATFORM` vaut `ios` |
| `TestReal_WebViewContext` | `VENOM_MOBILE_CHROMEDRIVER` (chemin d'un chromedriver correspondant sur l'hôte Appium) et `VENOM_MOBILE_WEBVIEW_LOCATOR` (identifiant d'accessibilité qui ouvre un écran de vue web) | `context` atteint un `WEBVIEW...`, `css: body` compte plus de 0, `context: native` renvoie `NATIVE_APP` ; ignoré sauf si les deux sont définis |
