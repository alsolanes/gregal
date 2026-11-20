---
title: "Android: on s'obre el xat i on queda quan acaba"
status: todo
depends_on: [006]
---

## Why

Avui tots els xats s'obren pel primer missatge, el més antic, i en tornar-hi es perd on eres; quan la resposta d'un vent acaba, la llista baixa sola al final encara que l'estiguessis llegint des de dalt. Les apps de xat ho fan d'una manera previsible i aquí no s'entén.

## Acceptance criteria

- [ ] Obrir una conversa per primer cop (xat de Codi, vent, Ventet) la mostra pel final: l'últim missatge sencer a la vista, sense animació.
- [ ] Si la conversa té missatges nous que no has vist (d'un vent que t'ha escrit mentre no hi eres), s'obre pel primer missatge nou, amb una ratlla «Nous» damunt seu.
- [ ] Sortir d'una conversa (a una altra pantalla o a una altra conversa) i tornar-hi dins la mateixa execució de l'app la deixa exactament on era (primer element visible i desplaçament).
- [ ] Quan una resposta acaba, la llista no es mou.
- [ ] Hi ha un test per a cadascun dels quatre casos.

## Out of scope

- Desar la posició entre execucions de l'app.
- El comportament mentre s'escriu (tasca 006).

## Notes

- Avui cada pantalla fa servir `rememberLazyListState()` sense clau ni `SaveableStateHolder` (`Screens.kt` ~410, `CompanyScreen.kt` ~1213, `VentetChat.kt` ~54), i `shouldFollowOnGrowth` pausa el seguiment en el primer layout, per això s'obre a dalt (a1dcb400).
- El salt al final en acabar és de c619a37b (`ReadReplyFromStart`, `CompanyScreen.kt` ~2603-2616); cal treure'l.
- Una posició per conversa en memòria (clau = id de la conversa) n'hi ha prou; Telegram i WhatsApp obren pel primer no llegit, ChatGPT i Claude pel final.
- Saber què és nou: els vents ja marquen les novetats com a llegides en obrir-los (POST de llegit); cal recordar l'últim missatge vist abans d'aquest POST.
