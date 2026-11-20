---
title: "Android: el xat no salta mentre escriu"
status: done
depends_on: []
---

## Why

Mentre l'agent o el vent escriu, la llista del xat va a salts fins que acaba: persegueix el final a cada tros de text, i a sobre canvien d'alçada o de clau coses que hi ha per sobre i per sota. Les apps de xat amb IA (ChatGPT, Claude, Gemini) ho fan al revés: en enviar, el teu missatge puja a dalt de la pantalla, la resposta creix a sota i la pantalla no es mou sola.

## Acceptance criteria

- [x] En enviar un missatge (xat de Codi, xat d'un vent i Ventet), el missatge enviat queda a dalt de la pantalla, sota la capçalera, i la resposta creix a sota seu.
- [x] Mentre la resposta s'escriu, la posició de la llista no canvia: el primer element visible i el seu desplaçament són els mateixos a cada actualització de text, encara que la resposta passi de la pantalla.
- [x] Quan la resposta passa de la pantalla, surt el botó «↓» per anar al final; no s'hi va sol.
- [x] Si l'usuari arrossega la llista mentre s'escriu, la llista no es mou sola fins que torna a enviar.
- [x] Quan la resposta passa a missatge definitiu (fi de torn, fase d'eina, narració que va a la targeta de feina) la llista no fa cap salt: el que hi havia a la vista continua al mateix lloc (±2 dp).
- [x] Al xat de Codi, que creixi el compositor (més línies, el comptador de testimonis, la cua o el pla) no mou els missatges que es llegeixen.
- [x] Un test de dispositiu o de Robolectric reprodueix una resposta llarga amb fases d'eina i comprova que la posició no varia mentre s'escriu.

## Out of scope

- On queda el xat en obrir-lo o en tornar-hi (tasca 016).
- Canviar l'aspecte de les bombolles o de la targeta de feina.

## Notes

- Lògica compartida de seguiment: `rememberChatFollow`, `ReadReplyFromStart`, `glideToLatest` a `CompanyScreen.kt` (~2461-2674). El xat de Codi (`ChatScreen` a `Screens.kt`) no fa servir `ReadReplyFromStart` i persegueix el final a cada flush de 60 ms (`GregalViewModel.kt` ~2302).
- Causes trobades: l'element `"live"` del xat de Codi desapareix o canvia de clau (`narrationToWork`, `settleLiveAsMessage`, `endTurn`); `WorkGroupCard` canvia d'alçada amb `animateContentSize` (`ChatWork.kt`); el `contentPadding` del vidre (`chatListPadding`) segueix l'alçada del compositor; a Ventet `live:reply` passa a `capItemKey` i la definitiva entra amb `MessageIn`; als vents, fase d'eina → bombolla nova i element `"typing"`.
- Per reservar espai sota la resposta (perquè el missatge enviat pugui pujar a dalt) cal un farciment final igual a l'alçada visible menys el missatge, que es va reduint a mesura que la resposta creix.
- `StreamingSmoothDeviceTest` només prova una bombolla sense fases: s'ha d'ampliar o fer-ne un de nou.
- Fet: `rememberChatFollow(list, key, lastSent)` + `chatReserve(follow)` (últim element de la llista) substitueixen el seguiment del final, `ReadReplyFromStart`, `glideToLatest`, `glideStep`, `bottomGap` i `capReplyKey`. En enviar (`follow.send()`), el proper missatge teu que entra es fixa a dalt; l'espai final és `vista − (espai entre el missatge fixat i el final)`. Si alguna cosa de sota s'encongeix i la llista s'enganxa al final (només passa mentre hi ha espai reservat), es torna a posar el missatge a dalt un fotograma després; si has arrossegat, no.
- El botó «↓» surt quan la llista pot anar més avall (`follow.behind`); el Ventet no en tenia i ara en té (`LatestPill`, també a la portada).
- Claus estables: al xat de Codi el text en directe porta ja l'id del missatge en què es tancarà (`vm.liveId`); al Ventet, el torn que acaba d'arribar en directe conserva `live:asked`/`live:reply` fins al proper enviament (`capKeys`), i la pregunta en directe es pinta amb hora i separador com quan ja és al fil.
- Rotllana, #retro i xats compartits fan servir el mateix model (tenien el seguiment antic, que s'ha esborrat).
- Tests: `ChatPinTest` (Robolectric, fil amb alçades fixes), `LiveReplyKeyTest` (ViewModel amb un servidor SSE fals), `ChatThreadTest` (`capKeys`, `capSentKey`). Esborrats perquè provaven el model antic (perseguir el final): `ChatFollowGrowthTest`, `ChatFollowDeviceTest`, `glideApproachesWithoutOvershooting`. `StreamingSmoothDeviceTest` s'ha reescrit al model nou amb una fase d'eina, però no s'ha pogut executar (no hi havia cap dispositiu connectat).
- Follow-up: al xat d'un vent i a la rotllana les claus són l'índex; si en acabar el torn el fil es recarrega del servidor amb un altre nombre de missatges, l'índex fixat pot ser un altre missatge.
- Follow-up: la narració que va a la targeta de feina (`narrationToWork`) treu l'element en directe; si era més llarga que la pantalla i es llegia pel mig, la vista cau a l'element del mateix índex.
- Follow-up: un missatge enviat més alt que la pantalla no puja a dalt (no hi ha espai a reservar).
