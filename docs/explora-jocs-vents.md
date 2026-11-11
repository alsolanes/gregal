# Exploració: jocs perquè els vents aprenguin jugant

> Estat: exploració, no hi ha res implementat. Objectiu: que cada vent jugui a jocs del seu ofici i que millori
> partida rere partida amb un bucle tipus *reinforcement learning*, sense entrenar pesos.

## Idea de fons: RL sense tocar el model

Els pesos del model no es poden entrenar, però el context de cada partida sí que el podem controlar.
La recerca actual fa servir aquest patró: el jugador no guarda estat entre partides, i el que millora
és una **memòria d'experiència** que el vent llegeix abans de jugar.

- **Reflexion**: després de cada episodi el model escriu una reflexió verbal, i la següent partida la llegeix.
- **ExpeL / ReasoningBank / Dynamic Cheatsheet**: les estratègies dels encerts i dels errors es destil·len en
  "lliçons" reutilitzables. Les que funcionen es reforcen i les que no, es descarten.
- **Voyager**: les solucions que funcionen es guarden en una biblioteca d'habilitats i es reutilitzen.
- **Arenes amb Elo** (Game Reasoning Arena, Kaggle Game Arena): enfrontaments per parelles amb un
  rànquing Bradley-Terry/Elo.
- Avís de la recerca: a [Learn2Play Bench](https://arxiv.org/html/2610.08215) i
  [MemoPilot](https://arxiv.org/html/2606.08656v1) es veu que la millora depèn sobretot de **com
  s'actualitza la memòria**. Una memòria que només acumula pot empitjorar el resultat.

Per a Gregal, doncs, el "RL" seria aquest cicle:

```
jugar (amb les N millors lliçons al prompt)
  → recompensa objectiva (verificador, resultat real o vot humà; mai el vent jutjant-se a si mateix)
  → reflexió curta (1 crida, model lateral): «què he de repetir / evitar», màx. 2 lliçons
  → les lliçons usades guanyen o perden punts segons el resultat
  → poda: es queden les que tenen millor taxa d'encert, no les més noves
```

Per demostrar que la millora és real i no una il·lusió, **1 de cada ~5 partides es juga sense lliçons**
(partida de control). La gràfica compara les dues corbes. Cada partida també desa el model que l'ha
jugat: si el proveïdor canvia de model, el salt no s'ha de confondre amb aprenentatge.

## Conceptes de joc

### 1. La Porra dels Vents (prediccions amb recompensa real)
Cada vent fa prediccions del seu terreny que es resolen soles amb el temps:
- Garbí: resultats dels partits de l'equip de l'usuari;
- Migjorn: la temperatura de demà o la tendència d'una sèrie;
- Marinada: si un producte vigilat baixarà de preu aquesta setmana;
- Xaloc: si un disc entrarà a la llista.

- **Per què és divertit**: l'usuari també aposta i hi ha una classificació «tu contra els vents». Quan es
  resol, el vent surt amb la bombolla de «novetat» a la pantalla d'inici («He encertat el 2-1! 🎯»), o
  admet l'error amb humor.
- **Recompensa**: puntuació de Brier sobre la probabilitat que el vent ha declarat. Premia encertar i
  també estar ben calibrat.
- **Què guarda**: lliçons del tipus «quan l'equip juga fora després de Champions, rebaixa la confiança»
  i l'historial de prediccions.
- **Com es veu que millora**: una gràfica de Brier mòbil (amb lliçons vs. control) i una corba de
  calibratge: «quan dic 70 %, encerto el 68 %».

### 2. Duel d'ofici (preferència humana → Elo)
Dos o tres vents responen al mateix repte curt dins d'una rotllana. Exemples:
- «menú per a 4 amb el que hi ha a la nevera, 20 min»;
- «3 llibres per a unes vacances de pluja»;
- «playlist per córrer 30 min».

L'usuari tria el guanyador amb un toc. Opcionalment, més tard: «l'he cuinat: 4/5».
- **Per què és divertit**: és un concurs de tele en petit. Els vents es piquen amb gràcia («Ponent, un
  altre Murakami?») i l'usuari és el jurat.
- **Recompensa**: el vot de l'usuari (parella guanyador/perdedor). És RLHF en versió memòria.
- **Què guarda**: lliçons de preferència («li agraden els plats d'una sola cassola», «odia els spoilers
  fins i tot a la contraportada») i un Elo per vent i per categoria.
- **Com es veu que millora**: la taxa de victòries contra el campió anterior i l'Elo que puja. El vent
  pot dir «He guanyat 7 dels darrers 10 duels de sopars».

### 3. El Banc de Proves (reptes verificables per a vents tècnics)
Trencaclosques amb un verificador automàtic que no fa servir el model:
- Mestral / Llevant: arreglar una funció petita fins que passin uns tests amagats;
- Cerç / Terral: respondre una pregunta sobre un CSV, i la resposta es compara amb el càlcul real;
- Fogony: trobar la vulnerabilitat plantada en un fragment.

Hi ha currículum per nivells: el nivell puja quan el vent encerta el 80 % dels reptes del nivell.
- **Per què és divertit**: a la cara del vent es veuen els estats `working → done/failed` que ja tenim.
  L'usuari pot proposar un repte o competir contra rellotge amb el vent.
- **Recompensa**: tests passats / resposta exacta, i menys intents és millor.
- **Què guarda**: una **biblioteca d'habilitats** a l'estil Voyager, amb fitxes de patró («off-by-one
  en finestres temporals: comprova els extrems») i lliçons dels errors.
- **Com es veu que millora**: nivell del currículum, intents mitjans per repte i % de primer intent.

### 4. Endevina l'ofici (autojoc entre vents, l'usuari s'hi pot afegir)
És un «20 preguntes» temàtic dins una rotllana. Un vent pensa una cosa del seu ofici (un plat, un
jugador, un concepte d'estadística) i els altres pregunten sí/no. L'usuari pot jugar com a endevinador
o com a amagador.
- **Per què és divertit**: és la rotllana fent soroll i té l'animació `discussing`. Es pot mirar amb
  l'app oberta com un programa.
- **Recompensa**: quantes preguntes ha calgut (com menys, millor) per a qui endevina, i per a qui amaga,
  que sigui difícil però justa (s'acaba endevinant).
- **Què guarda**: estratègies de pregunta («primer, el país d'origen») i Elo entre vents.
- **Com es veu que millora**: preguntes mitjanes per endevinar i una classificació a la rotllana.
- **Atenció**: és el més car (vegeu els costos) i el més fàcil de fer trampa: qui amaga pot canviar la
  resposta a mitja partida. Cal que la resposta quedi segellada al servidor abans de començar.

## Com encaixa amb el que ja hi ha

| Peça existent | Ús per als jocs |
|---|---|
| Cervell (`internal/companion/windbrain.go`) | **No s'hi poden barrejar les lliçons.** `windLearn` (`internal/web/windbrain_web.go`) només accepta fets amb un domini font que surti a la conversa, i el fitxer es poda per davant (24 KB, mana el més recent). Cal un fitxer germà, p. ex. `wind-lessons/<id>.md` o `.json`, amb un comptador d'usos i d'encerts per lliçó, i un `WindLessonsSystem()` anàleg a `WindBrainSystem()` que només s'injecti a les partides (o al xat, si l'usuari ho vol). |
| Extracció en segon pla (`sideRoleRef`, `windLearnLoop`) | La reflexió de després de cada partida és el mateix patró: crida petita, model lateral, timeout de 60 s, i si falla no trenca res. |
| Rotllanes (`winds_rotllanes.go`) | Són la taula de joc dels duels i de l'«Endevina»: ja tenen història persistent, torns i veus. Només cal un «moderador» de joc que no sigui un vent (codi, sense model) que reparteixi torns i puntuï. |
| Mascota Cap (`winds_cap_pet.go`, `CapGames.kt`) | Ja hi ha `game:<id>:<punts>` → XP, nivells i roba desbloquejable. Es pot reaprofitar el mateix model de progrés per als vents: guanyar un duel dona XP i un accessori de l'ofici. |
| Cares i moods (`winds.js`, `WindFace.kt`) | `thinking/working/done/failed/discussing/news` ja expliquen la partida sense fer res més. Faltaria un mood «celebració/derrota». |
| Novetats (`/api/winds/novetats`) | Per avisar que la porra s'ha resolt o que el vent ha pujat de nivell. |
| Feines programades (`winds_programats.go`) | Per al repte diari i per resoldre les porres en diferit, al servidor. |
| Blocs ` ```chart ` | Les gràfiques de progrés ja es poden pintar al web i a l'Android (`Plots.kt`). |

## Cost per partida (crides al model)

| Joc | Crides | Notes |
|---|---|---|
| Porra | 1 predicció + 1–3 resolució (cerca web) + 1 reflexió ≈ **3–5** | La resolució pot ser sense model si hi ha una API de resultats. |
| Duel (3 vents) | 3 respostes + 1 comentari + 3 reflexions ≈ **6–7** | Les reflexions, amb el model lateral barat. |
| Banc de Proves | 1–3 intents + 1 reflexió ≈ **2–4** | El verificador és codi (tests/càlcul), no costa crides. |
| Endevina (2–3 vents) | ~2 crides per pregunta × 10–15 + 2 reflexions ≈ **20–30** | El més car: cal posar-hi un límit de preguntes. |

Amb un límit diari configurable (p. ex. 20 crides/dia en jocs autònoms) el cost queda acotat. Els jocs
on hi participa l'usuari no tenen límit.

## Riscos

- **Hackeig de la recompensa / autoavaluació**: si un model jutja un altre model, tots dos aprenen a
  agradar-se. Només recompenses objectives (verificador, món real, vot humà). Un jutge LLM, com a molt,
  per comentar, mai per puntuar.
- **Memòria que s'infla o es contradiu**: tenir massa lliçons empitjora el rendiment (ho diu la
  recerca). Cal un topall (p. ex. 15 lliçons per vent i joc), puntuar-les per resultat i podar per
  utilitat, no per edat.
- **Contaminació del xat normal**: lliçons del tipus «sigues més agosarat» poden canviar la
  personalitat del vent. Per defecte, només dins dels jocs.
- **Progrés fals**: un canvi de model del proveïdor o una variància alta es pot llegir com a
  aprenentatge. Per això hi ha partides de control i el model queda registrat.
- **Execució de codi** (Banc de Proves): cal un entorn aïllat. No ha de córrer al projecte de l'usuari.
- **Resolucions errònies** (Porra): la cerca web pot llegir malament un resultat. L'usuari ha de poder
  corregir-lo, i la correcció torna a puntuar.
- **Pesadesa**: massa avisos o partides autònomes poden cansar. Ritme lent per defecte (1 porra/dia,
  repte setmanal).

## Prototip recomanat: La Porra, amb un sol vent

És el joc més «RL de debò»: la recompensa és objectiva i externa, és barat i no cal executar codi. A més,
la recompensa diferida fa que l'usuari torni a l'app per veure qui ha encertat.

Abast mínim:
1. Un vent amb especialitat verificable (Garbí, esports, o Migjorn, amb la temperatura de demà: més fàcil
   de resoldre sense ambigüitat).
2. Endpoint `POST /api/winds/joc/porra` → el vent fa una predicció amb una probabilitat. L'usuari hi
   posa la seva.
3. Feina programada que resol, puntua (Brier) i llança la reflexió → `wind-lessons/<id>.json`.
4. Una de cada 5 prediccions, sense lliçons (control).
5. Targeta a l'Android/web: marcador «tu vs. vent», gràfica de Brier i bombolla de novetat quan es
   resol.

Si funciona, el **Duel d'ofici** és el segon pas natural: generalitza a tots els vents i reaprofita les
rotllanes.

## Preguntes per a l'usuari

1. Quins vents primer: esports/temps (fàcil de verificar) o cuina/llibres (més divertit, però depèn del
   teu vot)?
2. Vols jugar-hi tu (contra o amb els vents), o sobretot mirar com juguen entre ells?
3. Poden jugar sols, sense tu? Si és així, amb quin pressupost diari de crides?
4. Les lliçons dels jocs poden passar al xat normal, o es queden dins el joc?
5. On ho vols veure primer: la pantalla d'inici de l'Android (tamagotchi) o el web?
6. El Banc de Proves (execució de codi en un entorn aïllat) t'interessa, o el deixem per més endavant?

## Fonts

- [MemoPilot — From Player to Master (arXiv 2606.08656)](https://arxiv.org/html/2606.08656v1)
- [Learn2Play Bench (arXiv 2610.08215)](https://arxiv.org/html/2610.08215)
- [Memento: Fine-tuning LLM Agents without Fine-tuning LLMs (arXiv 2508.16153)](https://arxiv.org/pdf/2508.16153)
- [Megagon — memòria episòdica i semàntica per adaptar agents](https://megagon.ai/memory-driven-llm-agent-adaptation/)
- [Game Reasoning Arena (arXiv 2508.03368)](https://arxiv.org/pdf/2508.03368)
- [Survey on LLM-Based Game Agents (arXiv 2404.02039)](https://arxiv.org/html/2404.02039)
- [LLMs Are Not Good Strategists, Yet Memory-Enhanced Agency Boosts Reasoning (arXiv 2608.12626)](https://arxiv.org/pdf/2608.12626)
