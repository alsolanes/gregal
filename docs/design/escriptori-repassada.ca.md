# Repassada visual de l'escriptori (Vents i mode codi), 2026-11-12

Captures reals (servidor de proves, model deepseek-v4.1-flash, un projecte
Python petit), a `docs/design/escriptori/`. Què es veia malament, què s'ha
arreglat ara i què queda, per ordre d'impacte.

## Arreglat ara

| Problema | Abans | Ara |
|---|---|---|
| Targeta de resultat (Gregal) | Nom, descripció i preu en tres columnes estretes, tot en negreta; fonts repetides («idealo · QCY H3», «idealo · Sony…») | Com a Android: nom a dalt, descripció suau a sota (2 línies), preu a la dreta; cada web una vegada (`abans-gregal-targeta.png` → `despres-gregal-targeta.png`) |
| Mode codi: «treballant» tres vegades | Capçalera del grup + franja groga amb el pas, la velocitat i les dreceres + peu amb «PP 3.2k · TG 144 t/s» | La franja només diu estat i temps (el pas ja és a la capçalera del grup); la velocitat, «128 t/s», només al peu |
| Rutes absolutes a les eines | `Edit C:\Users\…\Temp\b14\proj\app.py` | `Edit app.py` (la sencera al títol) |
| Etiqueta «bet 90% ✗» en vermell | Píndola amb vora, paraula críptica | «90%» discret; què vol dir, al títol |
| «Task completed» | Targeta gran amb ombra que repetia la resposta | Una franja: ✓ Tasca completada · fitxers · Revisa canvis (`despres-codi-acabat.png`) |
| Avís que tapava el camp d'escriure (Vents) | A baix, sobre el compositor | A dalt, sota la cara flotant |
| Resposta llarga que «salta» | S'escrivia dins d'una bombolla i en acabar passava a superfície de lectura | Si és llarga (taula, codi, >900 caràcters) ja s'escriu sense bombolla |

## Queda per fer (proposta, per ordre)

1. **Gregal no ensenya què fa mentre treballa.** A Vents només surt
   «Preparant la resposta…» encara que llegeixi fitxers, navegui o programi un
   recordatori (`gregal-escrivint.png`). dots i Duet ensenyen els passos en
   una línia viva («Llegint app.py», «A renfe.com»), plegable. Cal que el
   stream del fil dels vents porti els passos d'eina com ja fa el del mode
   codi, i pintar-los com una línia sota la cara.
2. **Menú lateral de Vents massa llarg i diferent del d'Android.** Ara:
   Vents, Racó, Automatitzacions / Nova conversa / Inici, Feina, Colla, Notes,
   Desats, Més / Converses / Projectes. Android ja té el menú curt (Gregal,
   Feina, Vigilant, Recordatoris · Canals · Recursos). Paritat.
3. **Artefactes flotants a Vents:** la icona de document solta a dalt a
   l'esquerra del fil i el «minimapa» de ratlles a la dreta (navegació entre
   preguntes) es veuen com errors. Amagar-los fins que hi passis el ratolí.
4. **Mode codi: el resum del grup** porta «# Retro» i «Replay» com a xips de
   colors al costat de «Ha treballat 16 s · 3 eines»: moure'ls a un menú ⋯ o
   mostrar-los en passar-hi el ratolí.
5. **Compositor del mode codi** amb vora groga quan és en mode autònom
   («No questions»): és informació útil però crida massa; un punt de color al
   xip ja ho diu.
6. **El fil de la conversa nova** a la llista mostra «Good ideas start here.»
   a sota d'una conversa real: és el text de buit i no hi hauria de ser.

## Com actualitzar l'escriptori de Vents

- **Versió instal·lada (`Gregal-Setup-*.exe`)**: s'actualitza sola amb
  electron-updater, que llegeix `latest.yml` de l'última release publicada.
  Per forçar-ho: Preferències → «Comprova si hi ha actualitzacions».
- **Versió portable (`.exe` solt)**: no s'actualitza sola (l'actualitzador
  diu «portable»). Recomanació: passar a l'instal·lador una vegada; la
  configuració i les converses es conserven perquè viuen a `~/.config/gregal`
  i `~/.local/share/gregal`, no a la carpeta del programa.
- Cal que hi hagi una release publicada amb binaris: la v1.72.4 té
  l'etiqueta pujada, però encara no els instal·ladors.
