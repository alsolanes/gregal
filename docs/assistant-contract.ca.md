# Contracte de l'assistent (B0)

Revisió 1 · 11 de novembre de 2026 · desenvolupa la §9 de [la renovació completa](android-assistent-experiencia-v2.ca.md)

Aquest document és la base dels paquets B1–B9. Fa tres coses: diu què ja existeix i on viu cada concepte, fixa els identificadors i la propietat de les dades, i llista què ha fet B0. Si un paquet necessita canviar res d'aquí, ho demana a l'integrador i no ho canvia pel seu compte.

## 1. Inventari: concepte → què hi ha avui → què falta

| Concepte | Avui | On | Què falta (paquet) |
|---|---|---|---|
| **Missatge** | `llm.Message` als fils dels vents; `capMsg` al fil del Ventet (l'assistent principal). **Des de B0 tots dos porten `id`.** | `internal/llm/client.go`, `internal/web/winds_cap.go`, `petHistoryJSON` a `companion_petchat.go` | `relatedRunId`, `resultRefs` (B4/B1). Android encara indexa el fil per posició: canviar la clau de `LazyColumn` a `id` (B1) |
| **Novetat** (Avui, notificació) | `windNovetat` amb text copiat. **Des de B0 porta `message_id`** quan neix d'un missatge publicat (cor, recordatori) | `internal/web/winds_novetats.go`, `GET /api/winds/novetats` | Obrir el fil a `message_id` (B7); deixar de mostrar el text duplicat a Avui (B1) |
| **Encàrrec** | No hi és com a entitat. Hi ha `runs.Run` (cua en memòria; durabilitat via esdeveniments), `jobs.Job` (programats), colles (`/api/winds/crew/*`) i el torn del Ventet (`/api/winds/cap`, `/cap/go`) | `internal/runs/runs.go`, `internal/jobs/jobs.go`, `internal/web/winds_crew*.go` | Entitat durable amb `objective`, `state`, `allowedScope`, `resultRefs` sobre l'ID de run (B4) |
| **Decisió** | `windAskReq` en memòria (`map` + canal), `windAskMax = 20 min`; `GET /api/winds/pending`, `POST /api/winds/answer` | `internal/web/winds_ask.go` | Persistir, alliberar el goroutine, revisió de l'efecte, idempotència (B4) |
| **Responsabilitat** | Cor del vent = `jobs.Job` `wind-<id>` kind `event`; recordatoris `remind`; rutines | `winds_heartbeat.go`, `winds_remind.go`, `winds_rutines.go` | Objectiu, fonts, accions permeses, pressupost i pausa (B6) |
| **Connexió** | `accountConnections` (ICS, IMAP amb **contrasenya en clar al fitxer**, briefing) a `/api/connections`; MCP remot amb OAuth/PKCE a `/api/mcp/*` | `internal/web/connections.go`, `internal/mcp/oauth.go`, `internal/web/mcp_web.go` | Model únic amb `authRef` opac, estat comprovat, diversos comptes; treure secrets del payload (B5) |
| **Ordinador** | Navegador propi amb CDP i perfil a disc; handoff només en mode visible | `internal/tools/browser*.go`, `browser_handoff.go` | Worker dedicat, salut, lease de control, visor (B2/B3) |
| **Esdeveniment** | Registre durable amb cursor, replay i filtre per usuari, sessió i run | `internal/events/store.go`, `/api/v2/events`, `/api/v2/events/stream`, `/api/v2/runs/{id}/cancel` | Reutilitzar-lo per a encàrrecs, decisions i resultats; **no** crear un segon bus (B4/B7) |

Els estats vàlids de cada concepte nou són a `internal/web/assistant_contract.go` i són els mateixos que ha d'usar Android:

- **Encàrrec:** `queued · running · waiting_user · waiting_connection · waiting_computer · completed · failed · cancelled`.
- **Decisió:** `pending · approved · rejected · expired · superseded`.
- **Connexió:** `available · connecting · connected · needs_session · error · disconnected`.
- **Ordinador:** salut `starting · running · stopped · unreachable`; control `agent · user · none`.

## 2. Identificadors

| Peça | Format | Regla |
|---|---|---|
| Missatge nou | `m-<unixnano base36><10 hex>` | El crea el servidor quan el missatge entra al fil (`newMessageID`). Ordenable per temps |
| Missatge antic | `h-<16 hex>` | Derivat de `rol + ts + text` (`messageID`). El mateix missatge té el mateix id a totes les rutes i entre reinicis, sense migració. Al Ventet, el primer desat el fa persistent |
| Run | enter (`runs.Run.ID`, `events.Event.RunID`) | Ja existent; és la clau de l'encàrrec |
| Novetat → missatge | `message_id` | Buit per a novetats que no tenen missatge al fil (p. ex. canvis del Racó) |
| Connexió | `con-…` + `authRef` `auth:…` | `authRef` és opac: el secret mai surt del servidor |

L'`id` no s'envia mai al proveïdor del model; es treu igual que `ts` i `speaker_*` (`marshalMessage` amb `sanitize`).

## 3. Propietat i aïllament

- El propietari és l'usuari del servidor: `s.user.Name`, o buit en mode d'un sol usuari (`remindOwner`). L'estat dels vents ja viu per usuari (`windStateDir` → `userStateDir`), i els esdeveniments es filtren per usuari (`AfterUser`).
- Tota entitat nova porta `owner` i es llegeix filtrant per ell. Una prova amb una segona identitat és obligatòria a cada paquet que n'afegeixi (B9 la repeteix sencera).
- `connections.go` desa la contrasenya IMAP en clar dins `accountConnections`; la lectura ja només retorna `has_password`. Es pot mantenir com a via existent, però B5 l'ha de moure darrere d'un `authRef` i no la pot posar en cap payload nou.

## 4. Compatibilitat amb les dades actuals

- **Historials existents:** cap migració. Els missatges sense `id` el reben derivat en llegir-los.
- **Clients antics** (Android o web sense aquest canvi): ignoren `id` i `message_id`, que són camps nous i opcionals.
- **Servidors antics:** l'app nova rep `id` buit i ha de continuar funcionant. Mentre hi hagi servidors sense aquest canvi, B1 fa servir l'índex com a clau només quan `id` és buit.
- **Cache fora de línia d'Android** (`VentsOffline`): ara desa i recupera `id`. Les entrades antigues el porten buit.
- **Cor dels vents:** les dates `NEXT` que ja estaven desades no es toquen. A la propera execució s'apliquen els límits nous.

## 5. Fixtures

Els fitxers són a `docs/assistant-contract/fixtures/` i els valida `TestAssistantContractFixtures`: camps imprescindibles, estats vàlids, `authRef` opac i cap camp de secret.

| Fitxer | Cas |
|---|---|
| `encarrec-esperant.json` | Encàrrec del viatge en `waiting_user` amb una decisió pendent i un resultat |
| `decisio-pendent.json` | Enviar un correu a l'hotel, revisió 1, amb caducitat |
| `connexio-caducada.json` | Compte de correu en `needs_session` |
| `ordinador-aturat.json` | Worker `stopped`, ningú amb el control |
| `missatge-proactiu.json` | Aportació del cor lligada al run i al resultat |

Android (B1) les pot llegir des dels tests unitaris amb el camí relatiu `../../docs/assistant-contract/fixtures` des de `android/app`.

## 6. Fitxers reservats a l'integrador

`GregalViewModel.kt`, `GregalApi.kt`, `CompanyScreen.kt`, `MainActivity.kt`, `AppStrings.kt`, `Strings.kt`, `Models.kt`, `Cap.kt`, `docs/api-contract.md`, aquest document i `internal/web/assistant_contract.go`. Qui necessiti un camp o una crida nous lliura el canvi exacte a l'integrador.

## 7. Què ha fet B0

- **F01:** el cor treu primer la línia `NEXT` i després comprova el silenci. Una resposta «(res)» amb `NEXT` (el que demana el prompt) ja no es publica ni genera sortida de notificació. `windStripNext` treu les línies `NEXT` amb el mateix criteri amb què es llegeixen (majúscules, cometes, espais). Tests: `TestWindHeartbeatSilenceWithNext`, `TestWindHeartbeatStripNextVariants`.
- **F02:** `windClampNext` limita la propera mirada a entre 30 min i 14 dies, i posa 24 h per defecte quan no hi ha `NEXT`. Test: `TestWindClampNext`.
- **Identificadors de missatge:** `id` als fils dels vents i del Ventet, i `message_id` a les novetats publicades, servidor i Android. Tests: `TestMessageIDPersistsWithoutProviderWire`, `TestMessageIDDerivedIsStable`, `TestProactiveMessageNovetatPointsToThreadMessage`.
- **Vocabularis d'estat i fixtures** validats.

Queda per a cada paquet: tot el que diu la columna «Què falta» de la §1. Pendent de paritat: la web encara no fa servir `id` ni `message_id` (els rep i els ignora). Quan B1/B7 hi construeixin comportament, l'han d'afegir també a `internal/web/app`.
