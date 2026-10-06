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

Al compositor, tria **Fer una tasca** per treballar amb eines o **Conversar**
per parlar sense modificar fitxers. Els modes d'objectiu i autònom són dins
del desplegable de modes avançats. Els permisos es mostren al costat: canviar
de mode no els amplia. La selecció manual del rol és una opció avançada del
selector de models; Gregal pot triar-lo automàticament segons la feina.

## Taller 2D

La vista **Taller 2D** de la barra lateral representa les sessions obertes
com estacions de treball. Cada estació mostra el projecte i si la sessió
treballa, està en repòs o espera una resposta. Clica una estació per obrir
la seva conversa; **Qui em necessita?** obre la primera amb una interacció
pendent coneguda pel navegador actual.

Les estacions reutilitzen les sessions existents. Les alertes depenen de
les preguntes i aprovacions rebudes en aquest client; no són un registre
global de totes les interaccions obertes des d'altres navegadors.

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

Per defecte, el mode autònom permet escriure fitxers dins del projecte
seleccionat. Les accions que requereixen aprovació continuen esperant una
persona, encara que la sessió estigui en mode permissiu. Els permisos explícits
de les eines a la configuració tenen prioritat. En una execució sense
interfície, les accions que encara requereixen aprovació es deneguen.

Els subagents utilitzen el projecte i la sessio actius, hereten les restriccions
configurades i s'aturen quan es cancella el torn pare. Les ordres de segon pla
iniciades per eines de l'agent pertanyen al torn i s'aturen quan acaba o es
cancella; els processos iniciats manualment al terminal son independents.

Aprovar una ordre shell o permetre explicitament una eina dona acces real amb
els privilegis de l'usuari del backend. Els permisos d'escriptura dins del
projecte no limiten les ordres shell, la xarxa ni les lectures a aquest projecte.
No executis repositoris no fiables ni tasques desateses amb permisos amplis en
una maquina amb dades sensibles; utilitza un entorn aillat.

## Referències

### Prototips Web

A Equip, tria Crea una web, adapta la tasca, configura un model i inicia
l'equip. Els rols col.laboren amb text i codi, sense eines externes. Selecciona
el constructor o el revisor per obrir un HTML complet al panell lateral o
descarregar-lo com `website.html`. Els documents parcials no ofereixen preview.
La demo local es una simulacio i no utilitza cap provider.

La preview executa l'HTML en un iframe sense acces al mateix origen, amb CSP
que restringeix recursos externs, fetch i formularis. No es un sandbox complet
ni un desplegament. Revisa el codi abans d'obrir l'HTML descarregat fora de la
preview o publicar-lo.

- [Primers passos](getting-started.ca.md)
- [Contracte de l'API HTTP](api-contract.md)
- [Notes de compatibilitat](compatibility.md)
- [Política de seguretat](../SECURITY.md)
- [Com contribuir](../CONTRIBUTING.md)
## Limits Temporals Del Model

Els passos del model tenen un limit de 20 minuts en Autonomous i de quatre
minuts en els modes interactius. `agent.model_timeout_s` permet configurar-lo
entre 1 i 1200 segons; `0` selecciona el valor automatic segons el mode.
Els reintents comparteixen el mateix termini i la cancel·lacio continua activa.
Les peticions autonomes tambe respecten el pressupost restant de `max_minutes`.
Quan s'esgota, es descarten les eines pendents i es demana un resum final per
separat, amb el seu limit temporal existent. Un pas que excedeix el termini
no es repeteix automaticament.
