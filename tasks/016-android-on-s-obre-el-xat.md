---
title: "Android: on s'obre el xat i on queda quan acaba"
status: done
depends_on: [015]
---

## Why

Avui tots els xats s'obren pel primer missatge, el més antic, i en tornar-hi es perd on eres; quan la resposta d'un vent acaba, la llista baixa sola al final encara que l'estiguessis llegint des de dalt. Les apps de xat ho fan d'una manera previsible i aquí no s'entén.

## Acceptance criteria

- [x] Obrir una conversa per primer cop (xat de Codi, vent, Ventet) la mostra pel final: l'últim missatge sencer a la vista, sense animació.
- [x] Si la conversa té missatges nous que no has vist (d'un vent que t'ha escrit mentre no hi eres), s'obre pel primer missatge nou, amb una ratlla «Nous» damunt seu.
- [x] Sortir d'una conversa (a una altra pantalla o a una altra conversa) i tornar-hi dins la mateixa execució de l'app la deixa exactament on era (primer element visible i desplaçament).
- [x] Quan una resposta acaba, la llista no es mou.
- [x] Hi ha un test per a cadascun dels quatre casos.

## Out of scope

- Desar la posició entre execucions de l'app.
- El comportament mentre s'escriu (tasca 015).

## Notes

- Avui cada pantalla fa servir `rememberLazyListState()` sense clau ni `SaveableStateHolder` (`Screens.kt` ~410, `CompanyScreen.kt` ~1213, `VentetChat.kt` ~54), i `shouldFollowOnGrowth` pausa el seguiment en el primer layout, per això s'obre a dalt (a1dcb400).
- El salt al final en acabar és de c619a37b (`ReadReplyFromStart`, `CompanyScreen.kt` ~2603-2616); cal treure'l.
- Una posició per conversa en memòria (clau = id de la conversa) n'hi ha prou; Telegram i WhatsApp obren pel primer no llegit, ChatGPT i Claude pel final.
- Saber què és nou: els vents ja marquen les novetats com a llegides en obrir-los (POST de llegit); cal recordar l'últim missatge vist abans d'aquest POST.
- Fet: `rememberChatList(id, ready, firstNew)` (`CompanyScreen.kt`) substitueix `rememberLazyListState()` al xat de Codi (`"code"`), als vents (`"wind:<id>"`) i al Ventet (`"ventet"`, `"ventet-home"`). Desa la posició en un mapa en memòria (`ChatPositions`) en sortir; el primer cop, quan hi ha contingut, va al final (o al primer missatge nou) amb `scrollToItem`, sense animació.
- Novetats dels vents: `openVentsChat` desa quantes n'hi havia (`windUnreadAtOpen`) abans de marcar-les llegides; el fil s'obre a `msgs.size − n` amb la ratlla «Nous» / «New» (`threadCopy(lang, "new")`). Per saber que el servidor ja les ha donat, `loadWindChat` ara marca `windChatLoading` també quan ja hi ha fil a la mà (l'esquelet només surt amb el fil buit, com abans).
- El salt al final en acabar una resposta (`ReadReplyFromStart`) ja es va treure a la tasca 015; aquí només s'hi afegeix el test.
- Tests: `ChatOpenTest` (un per cas). La ratlla «Nous» i `windUnreadAtOpen` no tenen test propi: només es prova que la llista s'obre a l'índex del primer missatge nou.
- Follow-up: el Ventet ja té el seu separador «Una novetat per a tu» però no s'hi obre (s'obre pel final).
- Follow-up: el xat de Codi fa servir una sola clau (`"code"`) per a totes les sessions; canviar de sessió no recorda la posició de cadascuna.
- Follow-up: la rotllana, #retro i els xats compartits encara s'obren pel primer missatge.
- Follow-up: si les novetats són tan poques que no omplen la pantalla, el fil queda al final (no es pot portar el primer nou a dalt) i la ratlla es veu igualment.
