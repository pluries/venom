---
translates: web
title: Web
description: Comment utiliser l'executor Web dans une suite de tests venom.
fingerprint: 841e8ee00f470e50ad60416a4c8b072458dd7b33e432bb145851d952a4649966
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# Venom - Executor Web {#venom---executor-web}

Naviguer dans une application web

Cas d'usage : vous avez une application web et vous voulez vérifier certains comportements.

L'executor web vous permet de naviguer dans votre application web et d'y exécuter des actions.

## Certificats HTTPS {#https-certificates}

Par défaut, l'executor web ignore les erreurs de certificat HTTPS (le contexte du navigateur est créé avec `IgnoreHttpsErrors`).
Une étape web réussit face à un site dont le certificat est expiré, auto-signé ou émis pour un autre hôte : ne comptez donc pas sur elle pour vérifier
les certificats. Pour que le navigateur rejette un certificat invalide, réglez la variable `verifyTls` sur `true` ; l'étape
échoue alors. L'executor web ne peut pas faire confiance à une autorité de certification privée : Playwright n'offre pas cette option pour un contexte de navigateur.

## Navigateurs {#browsers}

L'executor web pilote un navigateur via [Playwright](https://playwright.dev) (binding Go
`github.com/mxschmitt/playwright-go`). Playwright gère ses propres versions de navigateurs : aucun `chromedriver` ni `geckodriver`
à installer, et aucune version à faire correspondre à celle du navigateur.

Installez une fois par machine le driver Playwright et le ou les navigateurs utilisés par vos suites, avec leurs paquets
du système d'exploitation (`--with-deps` demande `sudo` sous Linux) :

```bash
go run github.com/mxschmitt/playwright-go/cmd/playwright@v0.6201.1 install --with-deps chromium   # or firefox, webkit
```

venom ne télécharge jamais un navigateur de lui-même : une suite qui en a besoin d'un qui n'est pas installé échoue à l'initialisation, et
le message d'erreur donne cette commande.

### Arguments {#arguments}
* Chromium : [liste des arguments](https://peter.sh/experiments/chromium-command-line-switches/). Un argument sans
  tiret initial reçoit `--`, comme le faisait chromedriver.
* Firefox : [liste des arguments](https://wiki.mozilla.org/Firefox/CommandLineOptions),
  [liste des préférences](https://searchfox.org/mozilla-central/source/modules/libpref/init/all.js)

## Variables {#variables}

Vous pouvez définir des paramètres pour configurer le navigateur utilisé pendant la suite de tests. Tous les paramètres sont facultatifs :
* driver (défaut : `chrome`) : navigateur à utiliser :
  * `chrome` : Chromium
  * `gecko` : Firefox (la version propre à Playwright)
  * `webkit` : WebKit, le moteur de Safari
* width, height : taille de la page. Sans eux, la page garde la taille propre de la fenêtre du navigateur
* args : arguments du navigateur
* prefs : préférences Firefox (`gecko` uniquement)
* timeout (défaut : `180`) : durée maximale, en secondes, pour le chargement d'une page (`navigate`, `historyAction`, la réponse à une fenêtre popup),
  et pour qu'une navigation lancée par une étape se termine
* proxy : proxy à utiliser pour interroger le site web (format : `[server]:[port]`)
* verifyTls (défaut : `false`) : fait rejeter par le navigateur les pages dont le certificat HTTPS est invalide, de sorte que l'étape échoue. Sans lui,
  les erreurs de certificat sont ignorées, comme elles l'ont toujours été
* headless (défaut : `false`) : exécute le navigateur sans fenêtre. Utile dans un pipeline CI/CD
* binaryPath : exécutable du navigateur à utiliser à la place de celui de Playwright (`chrome` uniquement)
* trace (défaut : `false`) : enregistre une trace Playwright de chaque étape en échec dans `page.trace.zip`. Voir
  [Diagnostic des échecs](#failure-diagnostics)

Chromium s'exécute avec son bac à sable activé, comme sous chromedriver. Une suite qui doit le désactiver (dans un conteneur exécuté en
root, par exemple) passe `--no-sandbox` dans `args`. Un hôte qui restreint les espaces de noms utilisateur non privilégiés, comme le font
Ubuntu 23.10 et les versions suivantes en natif, ne peut pas démarrer le bac à sable du Chromium de Playwright : l'initialisation échoue et indique les solutions —
`--no-sandbox` dans `args`, `binaryPath` pointant vers un Chrome du système, ou l'autorisation de ces espaces de noms
(`sysctl kernel.apparmor_restrict_unprivileged_userns=0`).

Les erreurs de certificat HTTPS sont ignorées par défaut, comme elles l'étaient avec la session WebDriver que cet executor utilisait auparavant
(`acceptInsecureCerts`). Donnez une valeur à `verifyTls` pour les vérifier (voir Certificats HTTPS ci-dessus).

Toujours acceptés, mais **ignorés avec un avertissement**, car ils n'ont pas d'équivalent Playwright :
* driverPath, driverPort : il n'y a plus de processus driver
* detach : Playwright ne peut pas laisser le navigateur en marche après la suite
* debug, logLevel : ils réglaient le niveau de journalisation de l'ancien client WebDriver ; utilisez plutôt l'option `-v` de venom
* prefs avec `chrome`, binaryPath avec `gecko` : Playwright pilote sa propre version de Firefox

## Action {#action}
Les actions vous permettent de manipuler le navigateur

Une action sur un élément (`click`, `fill`, `select`, `uploadFile`, `selectFrame`) attend que l'élément soit prêt,
jusqu'à son `syncTimeout` ou 30 secondes. `find` n'attend jamais : il compte les éléments présents à l'instant, de sorte que
`result.find ShouldEqual 0` reste rapide. Après une action qui peut charger une page (`click`, `fill`, `select`,
`uploadFile`, `execute`, la réponse à une popup), l'étape attend que la page ait fini de se charger avant de lire
`result.title` et `result.url`. Cela inclut une navigation que la page lance d'elle-même, depuis un script
(`location.href = …`) ou depuis un minuteur sans délai. Une navigation qui ne se termine pas dans le délai `timeout` fait échouer l'étape,
et les suivantes aussi, jusqu'à ce qu'un `navigate` la remplace. Aucun `page.dump.html` n'est écrit pendant que la page navigue.

Un appel à la page qui ne revient jamais fait échouer son étape : après 60 secondes pour une lecture (le titre, la source, un comptage, un
texte ; plus le `syncTimeout` de l'étape pour un élément qu'elle attend), après 30 secondes pour `execute`.
L'appel au navigateur continue de s'exécuter, de sorte que l'étape suivante l'attend avant d'envoyer son propre appel. Un appel qui a une limite
propre (les lectures et `execute`) attend au plus cette limite, puis échoue de la même façon, l'attente se poursuivant
pour l'étape suivante. Une action sans limite propre (un clic, une sélection, une capture d'écran) attend au plus 60 secondes, consigne un
avertissement et continue. `navigate`, `historyAction` et un vidage de minuteur indiquant qu'une navigation est terminée n'attendent jamais, et une boîte de dialogue
ouverte entre-temps est d'abord fermée, comme d'habitude. Une étape en échec n'écrit pas de `page.dump.html` tant que cet appel
s'exécute encore : elle consigne un avertissement à la place.

Liste des valeurs de sortie disponibles :
* result.url : URL de la page courante
* result.timeseconds : durée d'exécution de l'action
* result.title : titre de la page courante

### Navigate {#navigate}
Naviguer vers une URL donnée

#### Input {#input}
* URL : URL vers laquelle naviguer
* Reset : réinitialiser l'état du navigateur

#### Exemple {#example}
```yaml
  - type: web
    action:
      navigate:
        url: https://www.google.fr
```

### Find {#find}
Rechercher un élément à partir d'un sélecteur

[Lien vers un tutoriel sur les sélecteurs CSS](https://developer.mozilla.org/fr/docs/Web/CSS/CSS_Selectors)
[Lien vers un tutoriel sur les sélecteurs XPATH](https://developer.mozilla.org/fr/docs/Web/XPath)

#### Input {#input-1}
* Selector : expression servant à identifier l'élément web
* Locator : localisateur à utiliser pour chercher l'élément (CSS ou XPATH)

#### Output {#output}
* result.find : renvoie le nombre d'objets identifiés

#### Exemple {#example-1}
```yaml
  - type: web
    action:
      find: 
        selector: .gsfi
        locator: CSS
    assertions:
    - result.find ShouldEqual 2

  - type: web
    action:
      find:
        selector: //div[@class='FPdoLc lJ9FBc']/center/input[@value='Recherche Google']
        locator: XPATH
    assertions:
    - result.find ShouldEqual 1
    - result.value ShouldEqual Recherche Google
```

### Click {#click}
Cliquer sur un élément à partir d'un sélecteur

#### Input {#input-2}
* Find : élément à trouver (plus d'informations dans la section find)
* Wait : durée d'attente après le clic (en secondes)
* SyncTimeout : durée maximale, en secondes, d'attente que l'élément soit prêt (présent, visible, activé, immobile, non recouvert). Défaut : `30`

#### Exemple {#example-2}
```yaml
  - type: web
    action:
      click:
        find:
          selector: button
          locator: CSS
```

### Fill {#fill}
Remplir un élément (input ou textarea) avec un texte. Le texte est saisi à la suite du contenu actuel du champ, comme le faisait WebDriver ;
il ne le remplace pas

#### Input {#input-3}
Tableau d'entrées structurées :
* Find : élément à trouver (plus d'informations dans la section find)
* Text : met à jour la valeur de l'élément avec un texte
* Key : met à jour la valeur de l'élément avec une touche
* SyncTimeout : durée maximale, en secondes, d'attente que l'élément soit prêt (présent, visible, activé, immobile, non recouvert). Défaut : `30`

#### Exemple {#example-3}
```yaml
  - type: web
    action:
      fill:
      - find:
          selector: input
          locator: CSS
        text: userName
```

### Select {#select}
Sélectionner une option d'un élément web

#### Input {#input-4}
* Find : élément à trouver (plus d'informations dans la section find)
* Text : option à sélectionner
* Wait : durée d'attente après le clic (en secondes)
* SyncTimeout : durée maximale, en secondes, d'attente que l'élément soit prêt (présent, visible, activé, immobile, non recouvert). Défaut : `30`

#### Exemple {#example-4}
```yaml
  - type: web
    action:
      select:
        find:
          selector: select
          locator: CSS
        text: Option 1
```

### Téléverser un fichier {#upload-file}
Téléverser un fichier sur un élément web

#### Input {#input-5}
* Find : élément à trouver (plus d'informations dans la section find)
* Files : tableau de chaînes définissant les fichiers à téléverser
* Wait : durée d'attente après le clic (en secondes)
* SyncTimeout : durée maximale, en secondes, d'attente que l'élément soit prêt (présent, visible, activé, immobile, non recouvert). Défaut : `30`

#### Exemple {#example-5}
```yaml
  - type: web
    action:
      uploadFile:
        find:
          selector: input
          locator: CSS
        files: 
        - toUpload.csv
```

### Wait {#wait}
Attendre
Une valeur entière qui définit le nombre de secondes à attendre

#### Exemple {#example-6}
```yaml
  - type: web
    action:
      wait: 1
```

### Sélectionner une frame {#select-frame}
Sélectionner une frame présente dans la page courante

#### Input {#input-6}
* Find : frame à trouver (plus d'informations dans la section find)
* SyncTimeout : durée maximale, en secondes, d'attente que l'élément soit prêt (présent, visible, activé, immobile, non recouvert). Défaut : `30`

#### Exemple {#example-7}
```yaml
  - type: web
    action:
      selectFrame:
        find:
          selector: "#iframeResult"
          locator: CSS
```

### Sélectionner la frame racine {#select-root-frame}
Sélectionner la frame racine présente dans la page courante
Une valeur booléenne à mettre à true pour sélectionner la frame racine

#### Exemple {#example-8}
```yaml
  - type: web
    action:
      selectRootFrame: true
```

### Fenêtre suivante {#next-window}
Sélectionner la fenêtre (onglet) suivante du navigateur, en revenant de la dernière à la première ; avec une seule fenêtre, l'étape
reste sur celle-ci
Une valeur booléenne à mettre à true pour sélectionner la fenêtre suivante

#### Exemple {#example-9}
```yaml
  - type: web
    action:
      nextWindow: true
```

### Précédent {#back}
Revenir à la page précédente
Une valeur booléenne à mettre à true pour revenir à la page précédente

#### Exemple {#example-10}
```yaml
  - type: web
    action:
      historyAction: back
    assertions:
    - result.title ShouldStartWith GitHub
    - result.url ShouldEqual https://github.com/
```

### Suivant {#forward}
Avancer vers la page suivante
Une valeur booléenne à mettre à true pour avancer vers la page suivante

#### Exemple {#example-11}
```yaml
  - type: web
    action:
      historyAction: forward
    assertions:
    - result.title ShouldStartWith GitHub
    - result.url ShouldEqual https://github.com/team
```

### Actualiser {#refresh}
Actualiser la page
Une valeur booléenne à mettre à true pour actualiser la page

#### Exemple {#example-12}
```yaml
  - type: web
    action:
      historyAction: refresh
    assertions:
    - result.title ShouldStartWith GitHub
    - result.url ShouldEqual https://github.com/team
```

### Confirmer une popup {#confirm-popup}
Confirmer une boîte de dialogue popup (boîte de dialogue de confirmation et boîte de dialogue d'alerte)

Une boîte de dialogue ouverte par une étape reste ouverte pour que l'étape suivante y réponde avec `confirmPopup` ou `cancelPopup` ; tant qu'elle
est ouverte, `result.title` et `result.url` sont vides. Toute autre action pendant qu'une boîte de dialogue est ouverte la ferme et échoue
avec `unexpected alert open`, comme le faisait WebDriver.
Une valeur booléenne à mettre à true pour confirmer la boîte de dialogue popup

#### Exemple {#example-13}
```yaml
  - type: web
    action:
      confirmPopup: true
```

### Annuler une popup {#cancel-popup}
Annuler une boîte de dialogue popup
Une valeur booléenne à mettre à true pour annuler la boîte de dialogue popup

#### Exemple {#example-14}
```yaml
  - type: web
    action:
      cancelPopup: true
```

### Exécuter du javascript {#execute-javascript}
Exécuter du code javascript. La commande est le corps d'une fonction : `return` et `arguments[0]`, `arguments[1]`… fonctionnent
comme avec WebDriver

#### Input {#input-7}
* Command : code javascript à exécuter
* Args : tableau de chaînes définissant les arguments javascript

#### Output {#output-1}
Ce que le script renvoie, le cas échéant. Quand il renvoie une promesse, l'étape attend sa valeur.
* result.value : la valeur sous forme de texte. Une chaîne reste telle quelle. Toute autre valeur est écrite en JSON : `42`, `true`,
  `[1,"a"]`, `{"user":{"name":"Ada"}}`, avec les clés d'un objet triées
* result.valuejson : la même valeur, typée, comme l'est `result.bodyjson` pour l'executor http. Utilisez-la pour comparer des nombres ou
  pour atteindre l'intérieur d'un objet ou d'une liste : `result.valuejson.user.name`

Le résultat de chaque étape web a une variable `result.valuejson`. Elle est vide, sauf si l'étape exécute un script qui renvoie
une valeur.

Un script qui ne renvoie rien, `undefined` ou `null` laisse les deux vides. Quelques valeurs n'ont pas de forme JSON :
* `NaN`, `Infinity` et `-Infinity` se lisent comme ces mots
* une `Date` se lit comme son heure UTC au format RFC 3339 (`1970-01-01T00:00:00Z`)
* un BigInt, une URL et une RegExp se lisent comme leur texte
* un élément DOM, `document` ou `window` se lit `ref: <Node>`, `ref: <Document>` ou `ref: <Window>`

Une valeur qui se référence elle-même, comme un objet dont une propriété pointe vers l'objet, laisse les deux champs vides.
Il en va de même pour une URL que le navigateur accepte et que venom ne sait pas lire, comme une URL avec un `%` qui n'est pas suivi de deux chiffres hexadécimaux
(`https://a.test/x#50%off`). venom consigne un avertissement et l'étape réussit.

#### Exemple {#example-15}
```yaml
  - type: web
    action:
      execute:
        command: "window.editor.setValue(\"<!DOCTYPE html> <html></html>\"); window.editor.save();"

  - type: web
    action:
      execute:
        command: "return {count: document.querySelectorAll('li').length, first: document.querySelector('li').innerText};"
    assertions:
    - result.valuejson.count ShouldEqual 3
    - result.valuejson.first ShouldEqual Apples
```


Plus d'informations sur les actions (https://github.com/ovh/venom/tree/master/executors/web/types.go)
Pour une action, vous pouvez prendre une capture d'écran du navigateur avec la commande suivante : `screenshot: [fileName].png`

## Simuler des appels réseau {#mock-network-calls}
Une étape peut répondre elle-même aux requêtes réseau de la page, de sorte qu'une suite peut tester un front end sans son backend. Ajoutez une
liste `mock:` à l'étape, à côté de `action:`. Chaque entrée répond à toute requête dont l'URL correspond à son `url`, et la
requête n'atteint jamais le réseau. L'hôte n'a pas besoin d'exister : un mock peut servir la page elle-même, puis les
appels propres de la page.

#### Input {#input-8}
Tableau d'entrées structurées :
* Url : motif des URL auxquelles répondre (obligatoire). `*` correspond à n'importe quels caractères sauf `/`, `**` correspond à n'importe quels caractères,
  `{a,b}` correspond à `a` ou à `b`. Le motif doit correspondre à l'URL entière : `**/api/users` répond à
  `https://app.test/api/users`, pas à `https://app.test/api/users?page=2` ; `**/api/users*` répond aux deux. Chaque `{` exige
  son `}`, sinon l'étape échoue : écrivez `\{` pour une accolade littérale (`\\{` entre guillemets doubles)
* Status : statut HTTP de la réponse. Défaut : `200`
* Headers : en-têtes de la réponse. Écrivez chaque valeur sous forme de chaîne : `X-Total: "3"`, et non `X-Total: 3`
* ContentType : le `Content-Type` de la réponse. Défaut : aucun
* Body : le corps de la réponse, sous forme de texte

Un mock commence à répondre avant l'exécution de l'action de l'étape, et continue de répondre dans les étapes suivantes du même testcase,
dans tous les onglets. Quand plusieurs mocks correspondent à une URL, le plus récent répond. Le testcase suivant démarre sans
aucun mock. Une étape peut contenir `mock:` sans action.

Définir `mock:` pendant qu'une popup est ouverte la ferme et fait échouer l'étape, comme toute autre action. Définissez le mock dans une
étape qui précède celle qui ouvre la popup.

Une requête à laquelle le service worker de la page répond lui-même n'atteint jamais le mock.

#### Exemple {#example-16}
```yaml
  - type: web
    mock:
    - url: "**/app"
      contentType: text/html
      body: "<title>Users</title><h1>Users</h1>"
    - url: "**/api/users"
      status: 200
      contentType: application/json
      headers:
        X-Total: "1"
      body: '[{"name": "Ada"}]'
    action:
      navigate:
        url: http://app.venom.invalid/app
    assertions:
    - result.title ShouldEqual Users

  - type: web
    action:
      execute:
        command: "return fetch('/api/users').then(r => r.json()).then(users => users[0].name);"
    assertions:
    - result.value ShouldEqual Ada
```

## Diagnostic des échecs {#failure-diagnostics}
Quand une étape échoue, l'executor web écrit `page.dump.html` dans le répertoire de travail : la source de la frame courante.
Il ne produit pas ce fichier tant que la page charge encore un nouveau document. Une étape avec `screenshot:` écrit sa capture d'écran,
et le fichier de vidage, une fois son action réussie.

Avec la variable `trace: true`, une étape en échec écrit aussi `page.trace.zip` à côté de `page.dump.html` : une trace Playwright
de cette étape, avec chaque action, un instantané de la page avant et après celle-ci, des captures d'écran et les appels réseau.
Ouvrez-la avec le visualiseur de traces de Playwright :

```bash
go run github.com/mxschmitt/playwright-go/cmd/playwright@v0.6201.1 show-trace page.trace.zip
```

Le visualiseur a besoin du driver Playwright ; la commande le télécharge s'il est absent, comme le fait la commande d'installation.
L'étape en échec suivante remplace les deux fichiers. Une étape qui réussit n'écrit aucune trace.

La trace contient tout ce que l'étape a vu et saisi, **mots de passe compris**, ainsi que les en-têtes et les corps de ses appels
réseau. Le masquage des secrets de venom ne s'y applique pas. venom l'écrit lisible par son seul propriétaire, comme
`page.dump.html`. Ne la publiez pas comme artefact de build et ne la commitez pas. Le traçage
ralentit aussi chaque étape, d'environ 20 ms sur une page simple : activez-le le temps d'analyser un échec.

## Exemple {#example-17}
Un exemple global

```yaml
name: TestSuite Web
vars:
  web:
    driver: chrome
    width: 1920
    height: 1080
    args:
    - 'browser-test'
    prefs:
      profile.default_content_settings.popups: 0
      profile.default_content_setting_values.notifications: 1
    timeout: 60
    debug: true
testcases:
- name: TestCase Google search
  steps:
  - type: web
    action:
      navigate:
        url: https://www.google.fr
    assertions:
    - result.title ShouldEqual Google
    - result.url ShouldEqual https://www.google.fr
  - type: web
    action:
      find: input[name="q"]
    assertions:
     - result.find ShouldEqual 1
  - type: web
    action:
      fill:
      - find: input[name="q"]
        text: "venom ovh"
  - type: web
    action:
      click:
        find: input[value="Recherche Google"]
        wait: 1
    screenshot: googlesearch.png

```

