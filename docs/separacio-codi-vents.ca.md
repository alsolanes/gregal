# Gregal Codi i Gregal Vents: dues interfícies, un sol backend

Proposta del 3 de novembre de 2026.

## El problema

Ara la web i l'escriptori són una sola pàgina (`internal/web/index.html`, unes
4.400 línies). Hi conviuen al mateix rail:
- l'agent de codi (xat, fitxers, canvis, terminal, navegador);
- els vents;
- l'Arena, els jocs i el Racó;
- Office, gràfics, GitHub, objectius, Kanban.

Qui ve a programar hi troba jocs. Qui ve pels vents hi troba un terminal.

## La idea

Mateix servidor, mateixes dades i mateix login, però **dues interfícies**:

| | Gregal Codi | Gregal Vents |
| --- | --- | --- |
| Per a què | Programar, com Claude Code | Assistents que s'encarreguen de les coses, com Grok o dots |
| Adreça | `/` | `/vents/` |
| Pantalla principal | Xat de l'agent amb el projecte: eines plegades, diffs, pla, aprovacions | Converses amb els vents, la colla i la benvinguda |
| Laterals | Projectes i converses; Fitxers, Canvis, Terminal i Navegador en un panell | Converses, grups i directori de vents |
| Hi viu | Objectius, MCP, GitHub, Kanban, Office, gràfics | Automatitzacions (rutines, recordatoris, resum del matí), calendari i correu, feines, Arena, jocs, Racó, biblioteca |
| Mòbil | Gregal (Android de codi) | Gregal Vents (el que ja hi ha) |

Les dues es poden obrir des de l'altra amb un sol botó («Obre Vents ↗» /
«Obre Codi ↗»), però no es barregen.

## Com, per fases

1. **Separar (sense redissenyar).**
   - `/vents/` serveix una pàgina pròpia (`vents.html`) que reaprofita els
     mòduls que ja hi ha: `winds-workspace.js`, l'Arena, els jocs, el Racó i les
     preferències.
   - A Codi, el rail perd Vents, l'Arena, els jocs i el Racó, i hi queda un
     enllaç a Vents.
   - A l'escriptori, el mateix instal·lador té dues finestres i dues dreceres
     («Gregal» i «Gregal Vents»), cadascuna amb la seva adreça.
   - Ni l'API ni les dades canvien.
2. **Codi al nivell de Claude Code.**
   - Transcripció neta, amb les eines plegades en una línia («Llegit 3 fitxers»,
     «Editat x.go +12 −3») que s'obren amb el diff.
   - Llista de tasques viva, aprovacions en línia, compositor amb @fitxers i /ordres.
   - Tipografia i espais sobris, i res de cap altre producte a la vista.
3. **Vents a la web al nivell del mòbil i de dots.**
   - Porta a la web el disseny del Gregal Vents d'Android: portada de
     converses, benvinguda, suggeriments, cares vives.
   - La secció Automatitzacions amb historial i «Executa ara».
   - Calendari, correu i el resum del matí.
   - L'Arena i els jocs com a plugins de Vents.

Cada fase es pot publicar sola. La 1 és la que treu l'estranyesa i té poc
risc. La 2 i la 3 són de disseny i es poden fer en paral·lel.
