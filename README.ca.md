# Gregal

[English](README.md) | [Català](README.ca.md)

Gregal és un agent de programació de codi obert amb interfícies de terminal,
escriptori i web, i clients per a Android, Telegram i VS Code. Es connecta a
endpoints de models compatibles amb OpenAI i inclou eines de projecte,
aprovacions, checkpoints, MCP i sessions persistents.

L'anglès és la llengua predeterminada de la interfície i de les respostes de
l'agent. El català està disponible amb `lang: ca` als clients que admeten
aquesta opció.

## Instal·lació

**App d'escriptori per a Windows (recomanada).** Descarrega
`Gregal-Setup-<versió>-x64.exe` de la
[darrera publicació](https://github.com/alsolanes/gregal/releases/latest) i
executa'l. Porta el backend inclòs i s'actualitza sola des de les publicacions
de GitHub; `Gregal-Portable-<versió>-x64.exe` funciona sense instal·lar (sense
actualitzacions automàtiques). L'instal·lador encara no està signat: si
SmartScreen l'atura, tria **Més informació → Executa igualment**. Des de l'app
també pots preparar l'extensió de Chrome i instal·lar el complement de Word
(Preferències).

**Android (Gregal Vents).** Instal·la `Gregal-Vents-<versió>-arm64-v8a.apk` de
la publicació que el porta (ara
[v1.23.0](https://github.com/alsolanes/gregal/releases/tag/v1.23.0)); a partir
de la 1.35.0 l'app s'actualitza sola.

**Des del codi font (qualsevol sistema).** Vegeu a continuació.

## Primers passos

Cal Go 1.27.1 o posterior. Des de l'arrel del repositori:

```sh
go run . init
```

Configura un endpoint accessible i els identificadors dels models abans de fer
la primera petició; els valors generats són exemples. Consulta la
[guia de primers passos](docs/getting-started.ca.md) per configurar el proveïdor,
provar la interfície web local i entendre el tractament de dades. També hi ha la
[guia en anglès](docs/getting-started.md).

Per iniciar la interfície web local, executa `go run . --serve` i obre l'adreça
que mostra el terminal. Per a la resta de clients i l'API HTTP, consulta el
[contracte de l'API](docs/api-contract.md) i les
[notes de compatibilitat](docs/compatibility.md).

### VS Code

L'instal·lador d'un clic `Gregal-VSCode-Setup-x64.exe` i l'ordre
`install-vscode.ps1` necessiten una publicació amb el paquet `.vsix` i el
backend independent, i les publicacions actuals no els porten (l'última va ser
la v1.13.1). Mentrestant, compila l'extensió des del codi tal com explica la
[guia de VS Code](vscode-gregal/README.md) i connecta-la al backend de l'app
d'escriptori (`http://127.0.0.1:8097`) o a `go run . --serve`.

## Contribuir

Llegeix [CONTRIBUTING.md](CONTRIBUTING.md) per preparar l'entorn de
desenvolupament. Per comunicar problemes de seguretat, segueix
[SECURITY.md](SECURITY.md). Consulta el [manual en català](docs/manual.ca.md)
o el [manual en anglès](docs/manual.md).

## Llicència

Gregal es distribueix amb llicència MIT; consulta el text complet a
[LICENSE](LICENSE). Els components de tercers inclosos conserven les seves
llicències i avisos.

## Preparació de la publicació

Les persones que mantenen el projecte poden seguir la
[llista de publicació a GitHub](docs/github-release.md) i l'
[auditoria de publicació](docs/open-source-audit.md). L'exportació de codi font
sanititzada no inclou l'historial privat de Git. Preparar el codi font no
autoritza a distribuir binaris d'escriptori o mòbil que no s'hagin validat.
