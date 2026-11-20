---
title: "Android: el xat no salta mentre escriu"
status: todo
depends_on: []
---

## Why

Mentre l'agent o el vent escriu, la llista del xat va a salts fins que acaba: persegueix el final a cada tros de text, i a sobre canvien d'alçada o de clau coses que hi ha per sobre i per sota. Les apps de xat amb IA (ChatGPT, Claude, Gemini) ho fan al revés: en enviar, el teu missatge puja a dalt de la pantalla, la resposta creix a sota i la pantalla no es mou sola.

## Acceptance criteria

- [ ] En enviar un missatge (xat de Codi, xat d'un vent i Ventet), el missatge enviat queda a dalt de la pantalla, sota la capçalera, i la resposta creix a sota seu.
- [ ] Mentre la resposta s'escriu, la posició de la llista no canvia: el primer element visible i el seu desplaçament són els mateixos a cada actualització de text, encara que la resposta passi de la pantalla.
- [ ] Quan la resposta passa de la pantalla, surt el botó «↓» per anar al final; no s'hi va sol.
- [ ] Si l'usuari arrossega la llista mentre s'escriu, la llista no es mou sola fins que torna a enviar.
- [ ] Quan la resposta passa a missatge definitiu (fi de torn, fase d'eina, narració que va a la targeta de feina) la llista no fa cap salt: el que hi havia a la vista continua al mateix lloc (±2 dp).
- [ ] Al xat de Codi, que creixi el compositor (més línies, el comptador de testimonis, la cua o el pla) no mou els missatges que es llegeixen.
- [ ] Un test de dispositiu o de Robolectric reprodueix una resposta llarga amb fases d'eina i comprova que la posició no varia mentre s'escriu.

## Out of scope

- On queda el xat en obrir-lo o en tornar-hi (tasca 016).
- Canviar l'aspecte de les bombolles o de la targeta de feina.

## Notes

- Lògica compartida de seguiment: `rememberChatFollow`, `ReadReplyFromStart`, `glideToLatest` a `CompanyScreen.kt` (~2461-2674). El xat de Codi (`ChatScreen` a `Screens.kt`) no fa servir `ReadReplyFromStart` i persegueix el final a cada flush de 60 ms (`GregalViewModel.kt` ~2302).
- Causes trobades: l'element `"live"` del xat de Codi desapareix o canvia de clau (`narrationToWork`, `settleLiveAsMessage`, `endTurn`); `WorkGroupCard` canvia d'alçada amb `animateContentSize` (`ChatWork.kt`); el `contentPadding` del vidre (`chatListPadding`) segueix l'alçada del compositor; a Ventet `live:reply` passa a `capItemKey` i la definitiva entra amb `MessageIn`; als vents, fase d'eina → bombolla nova i element `"typing"`.
- Per reservar espai sota la resposta (perquè el missatge enviat pugui pujar a dalt) cal un farciment final igual a l'alçada visible menys el missatge, que es va reduint a mesura que la resposta creix.
- `StreamingSmoothDeviceTest` només prova una bombolla sense fases: s'ha d'ampliar o fer-ne un de nou.
