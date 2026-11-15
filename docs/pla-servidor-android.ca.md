# Pla: Gregal servidor 24 h + Android (substituir Hermes)

Revisió crítica del 15/11/2026, amb proves reals al sandbox i lectura del codi.
Objectiu: un servidor Gregal encès sempre, l'app Vents com a client principal
(amb compte), la ubicació i l'hora del mòbil, i que serveixi per preguntar,
comparar, fer webs i servir-les, notícies diàries i compartir coses.

## Què funciona avui

- **Preguntar i comparar.** Prova «Pixel 9a o Galaxy A56?»: busca, llegeix
  botigues i torna la targeta d'opcions amb preus, fonts i una tria. Bé.
- **Webs: el codi hi és.** Un vent pot escriure fitxers, provar i cridar
  `publish`; el servidor ho serveix a `/share/<id>/` o a `<nom>.<domini>`, amb
  versions i desfer, i l'Android té «Webs i apps». Funciona amb el PC apagat.
- **Login al servidor** amb usuari i contrasenya o codi d'invitació.
- **Telegram:** hi ha un bot (`gregal telegram`, internal/telegram/), però
  parla amb l'agent de codi, no amb els vents.

## Què falla (per ordre d'impacte)

1. **Fer una web des del mòbil s'encalla.** La prova «fes-me una web… i
   publica-la» s'ha quedat a «Espero el teu permís: browser eval». En un xat
   de vent no hi hauria d'haver esperes dins del projecte. El guió del vent
   tampoc no li diu que pot construir i publicar.
2. **«Cada matí a les 8, notícies» no es pot fer des del xat.** No hi ha cap
   eina per crear una tasca recurrent: el vent posa una setmana d'avisos
   puntuals i diu que no pot. Només es pot des del formulari de rutines.
3. **Hora del servidor, no la del mòbil.** En un VPS en UTC, els avisos «a les
   9», les rutines diàries i les hores tranquil·les van 1–2 h desfasats; a més,
   l'hora es congela en desar i es mou amb el canvi d'hora.
4. **La ubicació del mòbil només s'envia si toques un botó** a «A prop teu» i
   no s'actualitza mai. El temps fa servir un altre lloc posat a mà.
5. **Menú d'Android desendreçat:** «Avui» (temps, A prop teu, resum del matí)
   no és accessible; «Recordatoris» i «A prop teu» obren la mateixa pantalla;
   «Feina» obre dues pantalles diferents; Webs, Connexions i «Nou
   especialista» surten dues vegades; «Ordinador» no té sentit en un VPS.
6. **Avisos al mòbil:** sense FCM a l'APK, l'app consulta cada 15 min i, si el
   sistema la mata, no arriba res. Telegram ho resoldria del tot.
7. **Compartir:** no hi ha manera de compartir un missatge, una foto, un pla, un
   menú o un gràfic amb altres persones de l'app o per WhatsApp/Telegram.
8. **Servidor remot:** per IP va en HTTP pla (contrasenya i token sense xifrar);
   les IP privades del VPS es confonen amb «xarxa de casa»; els comptes
   necessiten `GREGAL_ACCOUNTS_DIR` i `--addr 0.0.0.0` o fallen sense dir per què.
9. **Comparacions llargues** es queden a mitges (12 passos) i un resultat
   parcial surt amb «[partial: …]» al xat.

## Fases

### Fase 1: que el que demanes funcioni — FETA (15/11)
Provat al sandbox: «cada matí feiner a les 8 les notícies…» crea dues rutines de
debò (feiners i caps de setmana); «fes-me una web… i publica-la» la construeix,
la prova i la publica sense demanar res, i diu clar si l'enllaç només va a casa.
La zona horària no ha calgut: el servidor és al PC de casa.
- Eina `programa` per als vents: rutina diària/feiners/cada N hores des del xat
  («cada matí a les 8, notícies de…»), amb llista i «atura-ho».
- Webs des del mòbil sense esperes: el vent ho sap fer, cap aprovació dins del
  projecte, enllaç públic si n'hi ha (i avís clar si només és de casa).
- L'app envia la zona horària i la ubicació aproximada (si ho has acceptat) en
  obrir-se; el servidor les fa servir per a avisos, rutines, temps i A prop teu.
- Menú d'Android endreçat: Avui accessible, sense duplicats, noms clars.

### Fase 2: compartir — FETA (15/11)
Escriptori: botó de compartir a cada resposta (copia, WhatsApp, Telegram, a algú
de l'app, enllaç públic). Android: prémer llarg una resposta obre el full (text,
imatge de la bombolla, copia, a algú de l'app, enllaç). El que reps surt a
Desat («Rebut») i el teu Gregal t'avisa. L'enllaç pinta les targetes, les fotos i
els gràfics igual que l'app, i avisa si l'adreça només va a casa.
- Botó «Comparteix» a cada missatge i targeta: com a imatge (targeta
  renderitzada) o text, pel full de compartir d'Android (WhatsApp, Telegram…).
- Desa i envia a una altra persona de l'app (pla, menú, gràfic, recepta), que
  ho rep a «Desat» i ho pot obrir com a targeta viva.
- Enllaç públic de només lectura per a una targeta o un pla.

### Fase 3: Telegram com a canal de Gregal
- El bot dins de `--serve`, enllaçat al teu compte: el que escrius a Telegram va
  al fil de Gregal i les respostes, avisos i notícies diàries arriben allà.
- Targetes convertides (taula/text), fotos amb `sendPhoto` i botons inline.

### Fase 4: servidor de debò
- Guia i comprovació d'arrencada per a VPS: HTTPS (domini o túnel cloudflared),
  `GREGAL_PUBLIC_URL`, comptes, i avisos clars al mòbil si falta res.
- Comparacions: més passos i model fort quan cal; res de «[partial]».
