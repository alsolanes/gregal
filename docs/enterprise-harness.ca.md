# Harness de xat empresarial

[English](enterprise-harness.md)

Gregal implementa el motor de l'agent en Go. El harness permet incrustar i ampliar
el backend HTTP compartit. Telegram és una integració separada, fora del contracte
HTTP del harness. L'SDK Python és un client HTTP, no un motor d'agent Python.

## Vies disponibles

`harness.New` incrusta el mateix runtime Go i handler HTTP que fa servir
`gregal serve`. La web, desktop, Android, VS Code i el TUI connectat consumeixen
el contracte HTTP. El TUI local, el runner headless i el client upstream de
Telegram encara criden directament el motor Go; el harness HTTP no els exposa.

Els paquets `client` de Go i `python/` són clients HTTP del servei Go existent.
L'SDK Python no executa el motor de l'agent. Les extensions disponibles són un
endpoint de model compatible amb OpenAI, connectors MCP stdio al servidor, un
client propi sobre el contracte HTTP i el callback `Options.Authenticate` per
validar identitats d'un proveïdor corporatiu. Les ordres i credencials dels
connectors es queden al servidor.

Executeu un runtime i una configuració per procés. Les integracions d'eines i
les caches dels providers inclouen estat global al procés; algunes API, com les
dels providers i les tasques programades, són globals a l'aplicació. Separeu
desplegaments amb configuracions, directoris de dades, workspaces i permisos de
sistema propis. El runtime actual no és una frontera multi-tenant certificada.

## Arrencada

Copieu `examples/company-chat/config.example.yaml` a un fitxer privat i
substituïu l'endpoint i els models d'exemple. La plantilla apunta a un servei
local compatible amb OpenAI; si no requereix clau, es pot deixar buida. Per a
altres endpoints, `api_key` accepta `${GREGAL_MODEL_API_KEY}`.

Exemple local en un host Unix:

```sh
install -m 600 examples/company-chat/config.example.yaml /private/gregal.yaml
mkdir -p /private/gregal-data && chmod 700 /private/gregal-data
export GREGAL_DATA_DIR=/private/gregal-data
export GREGAL_API_TOKEN="$(openssl rand -hex 32)"
# Si cal, definiu GREGAL_MODEL_API_KEY des d'un gestor de secrets.
go run ./examples/company-chat -config /private/gregal.yaml -addr 127.0.0.1:8097
```

El runner escolta a loopback i serveix la interfície existent per defecte. Amb
`-ui=false` només exposa `/api/`. `GREGAL_API_TOKEN` és una única identitat de
servei/administració; feu-lo servir només amb la interfície local de confiança
o un client de servidor de confiança. No l'envieu a un navegador o mòbil no
confiable. Per a l'accés dels empleats a la interfície inclosa, afegiu usuaris
a `users:` dins la configuració privada. Definiu cada contrasenya amb
`GREGAL_DATA_DIR` apuntant al directori de dades del servei i l'ordre
`gregal --config /private/gregal.yaml --passwd <usuari>`. Per a un client amb
IdP, incrusteu el handler i passeu `Options.Authenticate`. El runner d'exemple
només mostra l'accés amb token compartit; no configura un callback d'IdP. El
callback ha de verificar la sessió/JWT del proveïdor d'identitat, derivar un
namespace estable amb `harness.AccountID(verifiedSubject)` i assignar roots,
home i drets d'administració segons política del servidor. Per a cada
`AccountID`, el runtime fixa les arrels normalitzades, home i drets
d'administració en el primer ús; rebutja els canvis fins que es reinicia. No
deriveu la identitat ni els permisos de capçaleres del navegador.

Per a accés remot, acabeu TLS al proxy d'entrada. Si envia assertions
d'identitat, feu que `Authenticate` les verifiqui amb un emissor de confiança;
no confieu només en capçaleres d'identitat reenviades. Limiteu la lectura de
les claus, la configuració i el directori persistent al compte de servei; feu
les còpies de seguretat amb els mateixos controls. Executeu el
procés amb privilegis mínims. Les arrels de fitxers i els permisos d'eines són
controls de l'aplicació, no un sandbox del sistema operatiu. Aïlleu l'execució
de codi no fiable amb límits explícits de fitxers i xarxa. Reviseu totes les
rutes HTTP i els connectors configurats abans de producció. En tancar
l'aplicació, `CloseContext` cancel·la les execucions actives i encuades i espera
el planificador, però no espera que surtin els executors d'eines ni tanca el
servidor HTTP, que pertany a l'aplicació que l'incrusta. Tanqueu tots dos
components amb un límit de temps adequat al desplegament.

## Client Python

Instal·leu l'SDK HTTP:

```sh
python -m pip install -e ./python
```

Feu servir un token per principal autenticat. L'entrada nativa retorna el token
d'usuari des de `/api/login`; una aplicació incrustada pot emetre'l després de
verificar la identitat IdP. Conserveu-lo al servidor:

```python
import asyncio
import os
import uuid
from gregal_client import Client

async def main():
    async with Client(os.environ["GREGAL_URL"], os.environ["GREGAL_USER_TOKEN"]) as backend:
        await backend.health()
        session = await backend.open_session(title="Pregunta sobre documents")
        request_key = str(uuid.uuid4())
        submitted = await backend.submit(
            session["id"],
            "Resumeix el document aprovat",
            idempotency_key=request_key,
        )
        async for event in backend.stream(session["id"], after=submitted["cursor"]):
            print(event)
            if event.get("kind") in {"run_completed", "run_failed", "run_cancelled"}:
                break

asyncio.run(main())
```

Deseu l'ID de cada esdeveniment consumit i reconnecteu des de l'últim cursor.
Deseu i reutilitzeu la mateixa clau d'idempotència si repetiu una petició quan
se n'ha perdut la resposta. Mostreu els payloads estructurats d'aprovacions i preguntes
i envieu la decisió amb `approve` o `answer`; no aproveu eines automàticament.
Atureu l'SSE persistent quan el consumidor ja no el necessiti i gestioneu les
desconnexions. Feu servir HTTPS fora d'una xarxa local de confiança.

## Modes i connectors

Els modes disponibles són `chat`, `inspect`, `code`, `goal` i `autonomous`.
Manteniu explícits el mode i la política d'eines. MCP està desactivat a `chat` i
`inspect`; la recuperació de fonts aprovades per MCP i l'anàlisi que escriu
gràfics requereixen `code` i permisos explícits. `set_mode` canvia les
capacitats de la sessió; el mode d'un torn només les pot restringir. El control
d'accés a la recuperació i les cites corresponen al servidor o al connector.
Un prompt no és un control d'accés. Reviseu executables, arguments, secrets i
accés a xarxa dels connectors, que s'executen al servidor.

## Compatibilitat del backend

Un client Python pot consumir aquest servei Go amb l'SDK HTTP. Cal preservar el contracte
HTTP de `docs/api-contract.md` i la negociació de capacitats de
`docs/compatibility.md`: identitat i protocol de salut, sessions, execucions
v2, cancel·lació, idempotència, cursors d'esdeveniments persistents, payloads
d'aprovació i preguntes, autenticació, límits de workspace i semàntica de
modes/permisos. Abans de canviar d'implementació, executeu tests de conformitat;
els tests actuals de l'SDK Python cobreixen el transport, no un motor alternatiu.
Manteniu les credencials i la configuració del desplegament fora del control de versions.
