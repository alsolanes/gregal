# Gregal

[English](README.md) | [Català](README.ca.md)

Gregal és un agent de programació de codi obert amb interfícies de terminal,
escriptori i web, i clients per a Android, Telegram i VS Code. Es connecta a
endpoints de models compatibles amb OpenAI i inclou eines de projecte,
aprovacions, checkpoints, MCP i sessions persistents.

L'anglès és la llengua predeterminada de la interfície i de les respostes de
l'agent. El català està disponible amb `lang: ca` als clients que admeten
aquesta opció.

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
