# Changelog

## Unreleased

- **El xat de grup ja no duplica el nom de qui parla.** La resposta de cada vent
  es desa sense el prefix «Nom: » (la identitat viatja per dins del missatge i
  la interfície ja mostra qui parla): s'acaben els «Fredolic: Fredolic: …».
  Les converses antigues es segueixen veient netes.
- **Les veus d'un grup es responen entre elles.** La consigna del grup demana
  una conversa real — una veu pot estar d'acord amb una altra, corregir-la o
  afegir-hi alguna cosa nova — en lloc de monòlegs paral·lels que repeteixen
  el mateix consell. I res de llistes d'enllaços: curt i pla, com als xats
  individuals.

## v1.53.5 - 2026-11-04 - Nom del model i nivell de raonament

- **Nom llegible del model i nivell de raonament, com a OpenCode.** El model surt
  com «DeepSeek V4.1 Flash» (nom del catàleg models.dev, amb un derivat de l'id si
  no hi és) a la web i a Android; l'id complet queda al tooltip. Nou selector de
  raonament (thinking): una píndola al costat del model a la web i una secció al
  menú del model d'Android, amb Per defecte, Auto, Sense i els nivells que el
  model declara al catàleg (p. ex. Baix, Alt, Màxim); si el model no raona,
  s'amaga. Es desa per usuari (`POST /api/think`) i no afecta el revisor ni els
  vents.
## v1.53.4 - 2026-11-04 - Cua de missatges a Android

- **Android: cua de missatges amb l'agent treballant.** Escriure mentre l'agent
  treballa posa el missatge a la cua (visible sobre el compositor, amb «Ara» i ×);
  tocar-lo o «Ara» el hi diu ja (steer), mantenir premut el botó d'enviar
  interromp, i en acabar el torn el següent de la cua s'envia sol. «Stop» buida la
  cua. També es corregeix que el botó d'enviar quedava desactivat durant un torn.
- **Els dumps de peticions fallides ja no embruten el projecte.** Anaven a un camí
  relatiu quan HOME és buit (Windows); ara a `GREGAL_DATA_DIR` o al perfil.
## v1.53.3 - 2026-11-04 - El mesurador de context, de debò

- **El mesurador de context deixa de quedar-se curt.** Només comptava els
  missatges guardats de la conversa i no el que l'agent llegeix amb les eines ni
  les instruccions del sistema: «explora tot el codi» marcava 2.366 tokens quan la
  crida en duia desenes de milers. Ara segueix els tokens de l'última crida real
  al model, també en viu durant el torn.
## v1.53.2 - 2026-11-04 - Respostes de l'agent més netes

- **Respostes de l'agent més netes.** El codi en línia deixa de dur vora i caixeta
  (ara és un ressalt suau), el text és una mica més gran i respira més, i la
  capçalera dels blocs de codi és més lleugera.
## v1.53.1 - 2026-11-04 - Recordatoris amb feina, més polits al mòbil

- **Recordatoris amb feina, més polits al mòbil.** El missatge d'un recordatori
  apareix al xat obert sense haver de sortir-ne; en segon pla l'app fa una segona
  passada dos minuts després de l'hora, així la notificació d'una feina amb cerca
  web no espera 15 minuts. El resultat arriba en l'idioma en què has demanat la
  feina. Si rebutges una aprovació, el vent ho sap i ho diu clar. Sense servidor
  configurat l'app ja no consulta les aprovacions.
## v1.53.0 - 2026-11-04 - Preguntes, aprovacions i recordadoris que fan feina
