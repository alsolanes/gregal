# Manual d'usuari de Gregal

[English](manual.md)

Gregal és un agent de programació amb interfície de terminal, escriptori i
web. Envia les peticions al model a l'endpoint configurat per al rol actiu.
Aquesta guia cobreix la configuració habitual i els límits de seguretat; per
als detalls que depenen de la compilació, consulta l'ajuda integrada i les
referències enllaçades.

## Requisits

- Go 1.27.1 o posterior per executar-lo des del codi font.
- Un endpoint de model compatible amb OpenAI i identificadors de models
  disponibles en aquell endpoint.

L'endpoint i el model han d'admetre les capacitats que vulguis fer servir,
com ara l'ús d'eines. La validació de la configuració no prova la connexió,
les credencials ni la disponibilitat del model.

## Inicialització i configuració

Des de l'arrel del repositori, crea la configuració inicial:

```sh
go run . init
```

En sistemes Unix, la ruta predeterminada és
`~/.config/gregal/config.yaml` i el fitxer es crea amb permisos `0600`.
L'ordre també crea `AGENTS.md` al directori del projecte actual si encara no
existeix. Tria una altra ruta amb `go run . init --config=/path/to/config.yaml`.

Defineix l'URL del proveïdor i els identificadors dels models per als rols
`chat`, `think`, `code` i `reviewer`. Les URL `local` i `local-direct`
generades fan referència a serveis de loopback que Gregal no inicia. L'URL
`cloud` generada és un exemple: substitueix-la, i també els identificadors de
model, pels valors que admeti el teu endpoint. Pots assignar diversos rols a
un mateix proveïdor si té les capacitats que necessiten.

El proveïdor cloud d'exemple llegeix la clau d'una variable d'entorn:

```yaml
providers:
  cloud:
    base_url: https://api.example.org/v1 # substitueix-ho pel teu endpoint
    api_key: ${GREGAL_CLOUD_API_KEY}
```

Defineix la variable a l'entorn des d'on iniciaràs Gregal, o fes servir un
gestor de secrets. No desis credencials ni configuració privada al repositori.

## Interfície de terminal

Valida la configuració i inicia Gregal:

```sh
go run . --check-config
go run .
```

`--check-config` només comprova l'estructura i les referències de la
configuració. A la interfície de terminal, `/help` mostra les ordres
disponibles i `/model` obre un selector amb els models anunciats pels
endpoints configurats.

## Interfície web local

Inicia el servidor a loopback:

```sh
go run . --serve --addr 127.0.0.1:8097
```

Obre `http://127.0.0.1:8097/` al navegador. Gregal rebutja adreces que no
siguin de loopback si no s'ha configurat un token o autenticació d'usuari. El
servidor no ofereix TLS; configura HTTPS abans de fer-lo accessible fora
d'una màquina local de confiança.

## Dades i execució d'eines

Els prompts, el context de conversa i el codi o els fitxers inclosos en una
petició al model s'envien a l'endpoint configurat per al rol. Un proveïdor
anomenat `local` només és local si el servei s'executa realment a la teva
màquina; un proxy local pot reenviar les peticions a un altre lloc. La consulta
web i els connectors MCP configurats també poden fer peticions de xarxa.

Les eines de fitxers i de shell s'executen a la màquina host dins del projecte
seleccionat. Els permisos i les confirmacions de Gregal són controls de
l'aplicació, no un sandbox del sistema operatiu. Revisa els canvis i les
ordres proposats. Per executar codi no fiable, fes servir un checkout
temporal o l'aïllament del sistema operatiu.

## Referències

- [Primers passos](getting-started.ca.md)
- [Contracte de l'API HTTP](api-contract.md)
- [Notes de compatibilitat](compatibility.md)
- [Política de seguretat](../SECURITY.md)
- [Com contribuir](../CONTRIBUTING.md)
