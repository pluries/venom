---
translates: skill
title: Usarlo desde Claude Code
description: Instale la skill venom-pluries para que Claude Code configure Venom Pluries en su proyecto.
fingerprint: d7f89518bebf2fcf0f7a15898d97748d2257db0cefe23caabcf8a0dd26d922b6
drafted_by: claude
drafted_on: 2026-11-27
status: machine-drafted
---
La skill `venom-pluries` permite que Claude Code configure Venom Pluries en su proyecto. Instala una versión verificada, escribe un primer conjunto de pruebas, añade el paso a su build y comprueba las recetas de páginas web y móviles. Usted aprueba cada comando que quiera ejecutar.

## Instalarla {#install-it}

Indique una etiqueta de versión. No instale nunca desde la rama por defecto, que cambia.

```bash
$ npx skills add pluries/venom@skill-v1.0.1 --skill venom-pluries
```

La herramienta pregunta para qué agente instalar: elija Claude Code. Coloca la skill en la carpeta `venom-pluries` (bajo `skills`, dentro de `.claude`) de su proyecto y escribe al lado un archivo `skills-lock.json`. La herramienta envía datos de uso anónimos, salvo que antes defina `DISABLE_TELEMETRY=1` (o `DO_NOT_TRACK=1`).

Las skills se ejecutan con los permisos de su agente. Lea el `SKILL.md` de la carpeta instalada antes de usarla. La skill muestra cada comando y espera su aprobación, y nunca le pide desactivar las aprobaciones.

¿Prefiere ver qué se copia? Clone la etiqueta y copie la carpeta usted mismo:

```bash
$ git clone --depth 1 --branch skill-v1.0.1 https://github.com/pluries/venom venom-skill
```

Copie la carpeta `skills/venom-pluries/` de ese clon en la carpeta `venom-pluries` (bajo `skills`, dentro de `.claude`) de su proyecto y borre el clon.

## Comprobar que está {#check-that-it-is-there}

Inicie Claude Code en el proyecto y pregunte qué skills tiene: `venom-pluries` aparece en la lista. La fila `Skill version` al principio de su `SKILL.md` indica su versión, que coincide con la etiqueta elegida.

## Usarla {#use-it}

Pida con sus propias palabras, por ejemplo «configura venom en este proyecto y escribe una primera prueba», «ejecútalo en GitHub Actions» o «comprueba mi receta». La skill instala la versión de venom a la que apunta (escrita en su `SKILL.md` y en su `.venom-version`), comprueba la suma de verificación, la firma y el firmante antes de ejecutar nada, y le dice qué hizo y qué no hizo.

## Lo que los mantenedores no ejecutaron {#what-the-maintainers-did-not-run}

El archivo de GitLab CI nunca se ejecutó en un servidor GitLab real. La skill lo dice, y la primera ejecución es suya. Para las recetas móviles la skill no instala ni arranca un host de dispositivos: usted aporta un servidor Appium y un emulador o un dispositivo.

## Actualizarla {#update-it}

Instale la nueva etiqueta del mismo modo. Si el `SIGNER_SHA256` de `release.env` difiere del que usa su proyecto, el certificado de firma se renovó. La skill le dice entonces que copie los nuevos scripts, mostrando la diferencia.
