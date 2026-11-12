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

## Fet a la segona passada

1. **Gregal ensenya què fa mentre treballa.** El fil dels vents ja enviava
   els passos (`activity`, només el nom de l'eina, sense arguments ni rutes);
   ara la web el diu en llenguatge pla («Llegint fitxers», «Cercant al web»)
   i deixa sota l'estat una línia amb els passos fets (✓ Llegint fitxers ✓
   Cercant al web), com dots i Duet (`despres-gregal-passos.png`). Android
   ja mostrava el pas en curs; s'hi han afegit les mateixes etiquetes
   (navegar, recordatori, vigilància, calendari, correu, especialista).
   Gap: Android no mostra encara la llista de passos fets.
2. **Menú lateral com el d'Android:** Gregal, Feina · Canals (especialistes,
   amb la seva cara) · Recursos (Ordinador, Notes, Desat, Més).
3. **Icona de notes solta** fora de la capçalera de Gregal (Ctrl+Alt+N i el
   menú la mantenen); el mapa de preguntes, tènue fins que hi passes el ratolí.
4. **«# Retro» i «Replay»** surten en passar el ratolí pel resum (a pantalles
   tàctils, sempre).
5. **Vora del compositor en mode sense preguntes**, suau: el xip groc ja ho diu.
6. **«Aquí comencen les bones idees»** ja no surt sota una conversa real.

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
