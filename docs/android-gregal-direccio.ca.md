# Gregal Android: direcció visual i prompt

12 de novembre de 2026. Basat en captures reals de Vents 1.75 a l'emulador (`docs/design/gregal-android/ara/`) i en la maqueta objectiu (`docs/design/gregal-android/gregal-target.html`, imatges `gregal-target-1..4.png`).

## 1. Què es veu avui

| Problema | On | Per què molesta |
|---|---|---|
| La capçalera és una franja opaca d'uns 130 dp amb dos botons rodons amb vora; el fil es talla en sec a sota | `VentsChatHeader(centered, compact)` + `openingBloom` a `CompanyScreen.kt` | Ocupa un 15 % de la pantalla i no sembla «flotant». Hauria de ser transparent amb una cara petita |
| ☰ a la portada no obre el menú | `WindConversation(home = true)`, `onBack = openDrawer` | El menú és l'única manera d'arribar a Feina, Canals, Connexions… |
| Les taules es tallen per la dreta, sense desplaçament ni indici | `Md.kt` (taules dins la bombolla) | Els preus de la darrera columna no es veuen |
| Les fonts surten com a píndoles grosses que també es tallen | xips de fonts sota la resposta | Pesen més que la resposta |
| Cada missatge porta hora + altaveu en una fila pròpia | meta de `WindBubble` | Soroll vertical; la hora ja la dona la ratxa |
| Una resposta llarga és un mur de text | lectura sense bombolla | No hi ha resum ni acció; un resultat hauria de ser una targeta |
| En obrir l'app, un diàleg modal d'actualització tapa tot | `Updater.kt` | Interromp just quan obres l'assistent |
| A Gregal li falta mostrar *què està fent* quan fa servir l'ordinador | franja de feina | Ara només hi ha punts; hi ha d'haver la miniatura en directe |

## 2. La idea

**Gregal és una persona que treballa per tu, no una app de menús.** La pantalla és el fil; tot el que no és el fil s'aparta.

1. **Res fix a dalt.** La barra d'estat és transparent (edge-to-edge); a sota, només la cara de Gregal (34 dp) flotant amb el menú (☰) i l'ordinador (🖥) com a icones soles, sense vores ni fons. El fil passa per sota amb un degradat suau del color de fons.
2. **Obertura amb presència.** En obrir: lluïssor que floreix (≤ 1 s) al voltant de la cara gran, la salutació i 3–4 suggeriments. Al primer missatge, la cara puja a la capçalera i la lluïssor es queda com un rastre.
3. **Els resultats són objectes, no paràgrafs.** Quan Gregal troba coses (trens, productes, opcions), torna una frase curta + una **targeta de resultat**: títol, 3 files destacades (una «recomanada»), fonts com a punts de colors i «Veure tots ›».
4. **Una sola acció clara per decidir.** Les aprovacions són una targeta amb la pregunta, què farà exactament i dos botons («Reserva'l» / «Ara no»).
5. **Veure'l treballar.** Mentre fa servir l'ordinador, sobre el camp d'escriure hi ha una franja amb la **miniatura en directe** (de `/api/ordinador/live`), el lloc i el pas («Clica "Comprar" · pas 3 de 5») i «Mira-ho». Es pot escriure mentre treballa.
6. **Menú curt i amb sentit.** Gregal · Feina · Vigilant · Recordatoris | Canals (els especialistes) | Recursos (Ordinador, Connexions, Webs, Notes) | Què pot fer Gregal · Preferències.

## 3. Pantalles objectiu

| # | Imatge | Què ha de quedar clar |
|---|---|---|
| 1 | `docs/design/gregal-android/gregal-target-1.png` | Obertura: cara gran amb lluïssor, salutació amb el nom, suggeriments, camp a baix |
| 2 | `docs/design/gregal-android/gregal-target-2.png` | Fil: capçalera flotant, bombolles, targeta de resultat amb recomanada, aprovació |
| 3 | `docs/design/gregal-android/gregal-target-3.png` | Treballant (fosc): «treballant…» sota la cara, novetat amb «Per què?», franja amb l'ordinador en directe, camp amb Atura |
| 4 | `docs/design/gregal-android/gregal-target-4.png` | Menú: Gregal, Feina (2), Vigilant (3), Recordatoris (5), Canals, Recursos |

## 4. Especificació

- **Capçalera:** alçada 52 dp sota la barra d'estat, transparent; icones 24 dp dins d'objectius de 48 dp, sense vora ni fons; cara 34 dp; sota la cara, una paraula d'estat només quan treballa. Degradat de fons de 0 → 100 % de transparència en els 56 dp de sota. `enableEdgeToEdge()` i barra d'estat transparent.
- **Fil:** bombolles 22 dp amb cua 6 dp a l'última de la ratxa; cos 16/23 sp; hora només a l'última de la ratxa, 11,5 sp, sense icona d'altaveu (llegir en veu alta va al menú de mantenir premut).
- **Taules:** dins d'una targeta amb desplaçament horitzontal i un degradat a la dreta quan n'hi ha més; o bé convertides a targeta de resultat quan tenen ≤ 4 columnes.
- **Targeta de resultat:** radi 18 dp, vora 1 dp; capçalera amb icona en quadre de 30 dp; files de 44 dp; fila recomanada amb fons verd al 9 % i etiqueta; peu amb fonts (punts de 18 dp superposats), antiguitat i «Veure tots ›». El servidor l'emet com a bloc estructurat (```gregal-result {json}```) perquè no depengui de parsejar markdown.
- **Fonts:** com a punts al peu de la targeta o com a «3 fonts» clicable que obre la llista; mai xips grossos.
- **Aprovació:** targeta amb vora de l'accent al 55 %, pregunta 14,5 sp, línia «què faré» 12 sp, dos botons de 44 dp.
- **Franja de feina:** 58 dp, miniatura 64×42 amb el punt magenta de l'acció (B13), lloc + pas, «Mira-ho» obre el full Ordinador.
- **Actualitzacions:** mai modal en obrir; una línia discreta al menú o una notificació.
- **Clar/fosc:** les dues paletes; contrast AA en bombolles i targetes.

## 5. Prompt per a l'agent que ho implementi

```
You are improving the Gregal Vents Android app (Kotlin/Compose, repo C:\dev\gregal, branch assistent-v2) so its main
screen looks and behaves like a top personal assistant (OpenAI dots / Claude Duet), following
docs/android-gregal-direccio.ca.md. Target images: docs/design/gregal-android/gregal-target-1..4.png (open the HTML
docs/design/gregal-android/gregal-target.html for exact sizes). Current state: docs/design/gregal-android/ara/*.png.

Goal: the home screen is Gregal's thread (WindConversation(home = true) in CompanyScreen.kt). Make it match the mockups:
1. Floating header: edge-to-edge, transparent status bar; no bar/background/borders; menu icon left, Gregal's
   34 dp face centred, computer icon right; content scrolls under it with a soft fade. Show a one-word status
   under the face only while working. Fix: the menu icon must open the drawer on the home screen.
2. Opening: big face with the bloom halo (≤1 s, respect reduced motion), "Bon dia, <name>", 3–4 suggestion chips;
   after the first message the face moves to the header.
3. Messages: bubbles per GregalShape tokens; meta (time) only on the last bubble of a run, no speaker icon row;
   tables scroll horizontally inside a card with an edge fade; source links become a compact "N fonts" row.
4. Result cards: add a structured block the server can emit (```gregal-result {title, subtitle, rows:[{lead,
   text, value, tag}], sources:[{name,url}], more}```), render it as the mockup card, and teach Gregal's persona
   (internal/web/winds_gregal.go) to use it when it returns options or comparisons. Web parity in
   internal/web/app (winds-workspace.js) with the same look.
5. Approvals: inline card (question, what exactly will happen, primary + secondary button) for decisions in
   Gregal's thread.
6. Work strip above the composer while Gregal uses its computer: live thumbnail from GET /api/ordinador/live,
   site + step text from the computer_action events, "Mira-ho" opens OrdinadorSheet. Typing stays possible.
7. Drawer (SimpleVentsDrawer in Drawer.kt): Gregal, Feina (badge), Vigilant, Recordatoris | Canals (specialist
   winds, + Nou especialista) | Ordinador, Connexions, Webs, Notes | Què pot fer Gregal, Preferències.
8. The update prompt must not be a modal on launch (Updater.kt): use a quiet row in the drawer or a notification.

Rules: edit Strings.kt by hand (ca/en parity with internal/web/app/i18n.js), never run regen-strings.py; keep
light and dark palettes AA; set ANDROID_HOME=C:\Users\alsol\AppData\Local\Android\Sdk and run
./gradlew testDebugUnitTest; check on the emulator (Medium_Phone_API_36) against a sandbox server
(GREGAL_DATA_DIR/GREGAL_AUTH/GREGAL_MEMORY in Temp, adb reverse tcp:8097 tcp:<port>) and take screenshots next to
the mockups; no Co-Authored-By lines; do not push or release.
Done when: side-by-side screenshots match the four mockups in light and dark, the drawer opens from home,
a train/price search returns a result card, an approval shows as the inline card, and all tests pass.
```

## 6. Què fan Grok Bot, dots i Duet que nosaltres no fem

Llegit el 12/11 a les fonts: [Grok Bot 101](https://x.ai/bot/guides/grok-bot-101) (guia oficial), [Presentem els dots](https://openai.com/ca-ES/index/introducing-dots/) (anunci oficial) i [What is Claude Duet?](https://zapier.com/blog/claude-duet/) (prova a Zapier). ✅ fet · 🟡 a mitges · ❌ no ho tenim.

| Què fan | Qui | Gregal avui |
|---|---|---|
| **Avisos al mòbil quan acaba una feina o et necessita**, amb l'app tancada | Duet, dots | 🟡 només amb l'app oberta o per comprovació periòdica; el push (FCM) no està configurat |
| **Connectar Gmail, Calendar i Drive d'un toc** a l'alta | Duet, dots, Grok | ❌ correu per IMAP i calendari per enllaç ICS, a mà; OAuth només per a MCP amb client registrat |
| **Resum del matí** al fil principal | Duet | 🟡 existeix el resum, però no arriba al fil de Gregal |
| **Revisa els teus comptes cada poques hores i només parla si cal** («recerca proactiva» només de lectura) | Duet, dots | 🟡 només el que li encarregues amb «vigila…»; no mira els comptes pel seu compte |
| **Pantalla de Tasques** amb la feina de fons i, al principi, «escaneja les teves eines per trobar feina que pugui fer» | Duet | 🟡 «Feina» llista el que corre; no proposa feina |
| **Pestanya Fes (Make)** amb plantilles: llista del dia, triatge de correu… | Duet | ❌ |
| **Resultats com a targetes al fil** (apps connectades, opcions) | Duet | 🟡 hi ha `ResultCard`, però Gregal no l'omple amb dades estructurades |
| **Veure i fer servir el seu ordinador des del mòbil com un escriptori remot** | Grok, dots | 🟡 amb VM: visor noVNC (mai provat amb una real); sense VM: el veus en directe però no el pots controlar des del mòbil |
| **Formulari segur al xat per a una contrasenya o una clau d'API** | Grok | ❌ cal anar a Connexions o iniciar sessió a l'ordinador |
| **Inicia sessió amb contrasenyes desades sense ensenyar-les al model** | dots | ❌ |
| **Regles en llenguatge natural** (què pot fer sol, què demana, què mai) + **un revisor** que les aplica | Grok, dots | ❌ només permisos al YAML i les decisions per a accions externes |
| **Ensenyar una feina gravant-la** i convertir-la en rutina | Grok | 🟡 «fes-ho així la propera vegada» en text; sense gravació |
| **Rutines i disparadors d'esdeveniments d'apps** (un fil de Slack, un PR, un correu nou) | Grok | 🟡 web, agenda, calendari i correu per enquesta; cap disparador d'apps |
| **Trucar-li i parlar-hi en veu** | dots | ❌ només dictat |
| **Parlar-hi des de Slack, Teams o SMS** amb el mateix context | dots | 🟡 hi ha un client de Telegram, no unificat amb el fil de Gregal |
| **Diverses feines alhora sense fils separats** | dots | 🟡 un torn a la vegada al fil; les colles van a part |
| **Bots que es parlen entre ells i xats de grup** | Grok | ✅ Gregal consulta especialistes; rotllanes i colles |
| **Diversos comptes per servei** | Grok | ✅ B5 |
| **Aprovació abans d'enviar, pagar, signar o esborrar** | tots | ✅ decisions de B4 |
| **Aprèn les teves preferències amb el temps** | dots, Duet | 🟡 memòria per vent i correccions; no aprèn sol del feedback |
| **Torna amb captures o un vídeo del que ha fet** | Grok | 🟡 B13 guarda les captures; no arriben al fil |
| **Navegar webs reals (Amazon, bitllets) amb preus** | tots | ✅ provat: el navegador d'aquest PC obre Amazon, Trainline i Renfe sense captcha. Al mòbil no ho veies perquè el servidor encara era la v1.71.0 |

### Ordre proposat (Android primer)

**P0 — perquè sigui i sembli com ells**
1. La pantalla de Gregal com a la maqueta (§2–§4): capçalera flotant, targetes de resultat amb dades estructurades, aprovació en targeta, franja amb l'ordinador en directe, menú curt, sense diàleg d'actualització.
2. Avisos reals al mòbil: FCM configurat (cal un projecte de Firebase teu) i «acabat / et necessito» sempre com a notificació.
3. Connectar Google d'un toc (Gmail, Calendar, Drive) amb OAuth (cal un client OAuth de Google Cloud teu).
4. Resum del matí i revisions cada poques hores al fil de Gregal, només quan hi ha alguna cosa.
5. Controlar el navegador del PC des del mòbil (tocs i text) quan no hi ha VM.

**P1 — confiança i comoditat**
6. Regles en llenguatge natural + revisor (permet / pregunta / mai), a Preferències.
7. Formulari segur al xat per a contrasenyes i claus (van directes al magatzem xifrat).
8. Pestanyes Xat · Tasques · Fes, amb «busca feina que pugui fer» i plantilles.
9. Mode veu: parlar-hi i que respongui en veu.

**P2 — més enllà**
10. Ensenyar gravant la pantalla de l'ordinador; disparadors d'apps; una adreça de correu per a Gregal; Telegram/Slack lligats al mateix fil.
