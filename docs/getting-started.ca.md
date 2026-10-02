# Primers passos

[English](getting-started.md)

Gregal s'executa a la teva màquina i envia les peticions al model al proveïdor configurat per a cada rol. Admet APIs compatibles amb OpenAI; tant l'endpoint com el model han de tenir les capacitats que necessitis, com ara l'ús d'eines.

## Requisits

- Go 1.27.1 o posterior.
- Un endpoint de model compatible amb OpenAI i els identificadors dels models disponibles.

## Compila i configura

Des de la còpia del repositori, crea la configuració inicial:

```sh
go run . init
```

Per defecte, el fitxer es desa a `~/.config/gregal/config.yaml`; en sistemes Unix es crea amb permisos `0600`. `init` també crea `AGENTS.md` al directori del projecte actual si encara no existeix. Pots triar una altra ruta amb `go run . init --config=/path/to/config.yaml`.

Obre el fitxer i configura l'URL del proveïdor i els identificadors dels models dels rols `chat`, `think`, `code` i `reviewer`. Les URL `local` i `local-direct` apunten a serveis de loopback que Gregal no engega. L'URL `cloud` generada (`https://api.example.org/v1`) és un marcador d'exemple. Substitueix les URL i els noms de model pels que admeti el teu endpoint. Pots assignar tots els rols a un mateix proveïdor si ofereix les funcions necessàries.

La plantilla llegeix la clau del núvol de la variable d'entorn `GREGAL_CLOUD_API_KEY`:

```yaml
providers:
  cloud:
    base_url: https://api.example.org/v1 # substitueix-ho pel teu endpoint
    api_key: ${GREGAL_CLOUD_API_KEY}
```

Defineix aquesta variable a l'entorn des d'on iniciaràs Gregal, o fes servir el teu gestor de secrets. Un endpoint local pot no necessitar clau. No desis credencials ni configuració privada al repositori.

Valida la configuració local i inicia el TUI:

```sh
go run . --check-config
go run .
```

`--check-config` comprova l'estructura i les referències de la configuració; no prova la connexió, les credencials ni la disponibilitat dels models. Al TUI, `/model` obre un selector amb els models que anuncien els endpoints configurats.

## Interfície web local

Inicia la interfície web només a loopback:

```sh
go run . --serve --addr 127.0.0.1:8097
```

Obre `http://127.0.0.1:8097/` al navegador. Per defecte, el servidor només escolta en aquesta màquina. Gregal rebutja les adreces accessibles des de fora de loopback si no hi ha un token o autenticació d'usuari configurats. El servidor no ofereix TLS; configura HTTPS abans de fer-lo accessible fora d'una màquina local de confiança.

## Dades i límits d'execució

Les peticions al model poden incloure prompts, context de conversa i codi o fitxers. S'envien a l'endpoint assignat al rol. Una URL anomenada `local` només és local si el servei de l'adreça s'executa a la teva màquina; un proxy local també pot reenviar les peticions a un altre lloc. La consulta web i els connectors MCP configurats també poden fer peticions de xarxa.

Les eines de fitxers i de shell s'executen a la màquina host dins del projecte seleccionat. Els permisos i les confirmacions de Gregal són controls de l'aplicació, no un sandbox del sistema operatiu. Revisa els canvis i les ordres proposats; per executar codi no fiable, fes servir un checkout temporal o l'aïllament del sistema operatiu.

Els proveïdors i els models tenen capacitats diferents pel que fa a eines, context, imatges i altres funcions. La configuració d'exemple no inclou cap subscripció de models, i validar-la no demostra que l'endpoint sigui compatible. Consulta les condicions del proveïdor per a les dades que hi envies.

## Més informació

- [Contracte de l'API HTTP](api-contract.md)
- [Notes de compatibilitat](compatibility.md)
- [Política de seguretat](../SECURITY.md)
- [Com contribuir](../CONTRIBUTING.md)
