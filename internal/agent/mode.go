package agent

import "gregal/internal/goal"

// Modes de treball. "code" executa eines amb la política normal (escriptures
// dins del projecte passen soles; la resta demana, amb opció de recordar per
// sessió), "chat" és conversa sense agent, "inspect" consulta el projecte
// automàticament sense escriure i "goal" concreta un objectiu abans
// d'executar-lo.
const (
	ModeCode       = "code"
	ModeChat       = "chat"
	ModeInspect    = "inspect"
	ModeGoal       = "goal"
	ModeAutonomous = "autonomous"
)

// ValidMode diu si m és un mode suportat.
func ValidMode(m string) bool {
	return m == ModeCode || m == ModeChat || m == ModeInspect || m == ModeGoal || m == ModeAutonomous
}

// PromptFor afegeix al prompt base els fets de l'entorn i les regles del
// mode. Els fets van aquí, i no als set llocs que criden PromptFor, perquè
// n'hi hauria hagut prou que un s'oblidés: el TUI, la finestra, el
// headless, el Telegram i els subagents treballen a la mateixa màquina i
// tots hi ensopeguen igual.
func PromptFor(base, mode string) string {
	if f := FetsEntorn(); f != "" {
		base += "\n\n" + f
	}
	base += reglesMode(mode)
	// La data va al final per mantenir estable el prefix reutilitzable pel KV
	// cache dels models locals; només canvia una vegada al dia.
	if d := DataAra(); d != "" {
		base += "\n\n" + d
	}
	return base
}

// reglesMode és el text específic de cada mode.
func reglesMode(mode string) string {
	switch mode {
	case ModeChat:
		return "\nMode xat: conversa amb lectura — POTS llegir el disc amb read/glob/grep quan calgui (fes-ho directament, sense demanar permís ni anunciar límits). No escriguis ni modifiquis res; si cal editar, digues que passin a /mode code. Si necessites una decisió de l'usuari per continuar (dues opcions incompatibles, un gust que no pots deduir), crida l'eina question amb 2-4 opcions curtes ABANS de respondre: surt seleccionable. No facis la pregunta en text pla si la pots fer amb question, ni preguntis el que puguis comprovar llegint el disc."
	case ModeInspect:
		return "\nMode consulta: inspecciona i explica el projecte sense modificar-lo. Utilitza read, grep, glob, web_search, web_fetch i només comandes de shell clarament de lectura. No demanis permisos: si una acció no és segura o només lectura, no l'executis i explica el límit."
	case ModeGoal:
		return goal.Prompt()
	case ModeAutonomous:
		return autonomousRules
	case ModeCode:
		return codeRules
	default:
		return ""
	}
}

// codeRules és el que faltava del mode code: no en tenia cap, i l'únic que
// llegia el model era «primer la solució», que empeny a actuar sempre.
// Resultat: davant d'una petició ambigua endevinava i es posava a escriure,
// i l'eina «question» —amb tot el circuit fet a la UI— no la cridava mai.
//
// La regla és estreta a posta. Un agent que pregunta per tot és pitjor que
// un que endevina: el que no pot fer és endevinar una decisió que canvia la
// forma de la feina i que ell no pot deduir mirant el repositori.
// Mesurat contra el model real amb el projecte ja explorat (la decisió és
// ara), 16 mostres per variant:
//
//	sense aquestes regles ...........  0/16 crida question
//	només el primer paràgraf ........ 14/28 (50%)
//	amb la prohibició d'escriure .... 11/16 (69%)
//
// I amb una tasca inequívoca, 0/12 en tots dos casos: no es posa a
// preguntar per tot. La meitat de les vegades que no preguntava, el que
// feia era triar ell (SQLite, JSON, una interfície) i escriure el fitxer;
// per això el segon paràgraf parla de fitxers i no de preguntes.
//
// La descripció de l'eina, en canvi, no mou res: provada en català llarga,
// curta i en anglès, totes donen el mateix. El que decideix és això.
//
// La regla del todowrite depèn del MODEL, i val la pena saber-ho abans de
// reescriure-la: Nex-2.5-mini 4/4 (0/4 sense la regla), Qwen38-Flash-FAST
// 1/4, i els dos Ornith 0/4 amb quatre formulacions diferents. Si el
// checklist no surt a la pantalla, el que falla és el model triat per al
// rol code, no aquest text.
//
// I una cosa que val per a totes: provades SOLES, sense competència, les
// eines es criden 3/3 (todowrite, patch, delegate, gh_issue,
// bash_background incloses). El que es perd, es perd contra les altres
// 22 —no per l'esquema ni per la descripció.
const codeRules = `
Quan una decisió canviï la FORMA de la feina i no la puguis deduir del
projecte, crida question amb 2-4 opcions curtes en comptes d'endevinar:
per exemple quina biblioteca, quin format de sortida, o quin de dos
comportaments incompatibles vol. Pregunta ABANS d'escriure res.
No la facis servir per a res que puguis comprovar tu mateix (com es fa
aquí, quin estil segueix el codi, què fan els tests): això es mira, no es
pregunta. Una pregunta per decisió, no una llista d'interrogatori.
No escriguis ni editis cap fitxer que introdueixi una dependència nova,
un format d'emmagatzematge o una API pública que l'usuari no hagi triat:
això es pregunta primer amb question.
Si la feina té més de tres passos, publica'ls amb todowrite abans de
començar i torna a cridar-lo cada cop que n'acabis un, amb un de sol
marcat com a working. És l'única manera que qui mira la pantalla sàpiga
per on vas.
El nombre de passos no és un límit: quan s'acaba un tros se't pregunta
si continues, i mentre avancis se te'n concedeix un altre. No deixis una
tasca a mitges per estalviar passos: acaba el checklist, passa les
comprovacions i després respon.
Quan hagis realitzat les edicions o comprovacions demanades, NO continuis
cridant eines innecessàriament: respon directament en text a l'usuari amb
un resum clar de la feina feta per donar la tasca per completada. El resum
diu què has canviat (fitxer i què), com ho has verificat (quina ordre i
quin resultat) i què queda pendent o en vermell, si n'hi ha. No hi tornis
a escriure el codi que ja és al fitxer ni hi posis capçaleres: qui llegeix
té el diff, i cada línia de més és espera.
Fes el que demana la tasca: no hi afegeixis funcionalitat, opcions ni
refactors que ningú no ha demanat, que són més feina i més risc d'errar.
Tria el canvi més petit i directe que arregli la causa, seguint el que el
codi ja declara (docstrings, comentaris, noms, tipus). Un arreglo amb
heurístiques i casos especials és més fràgil que el canvi directe, i
falla amb l'entrada que el test no mira.
Respecta els noms de fitxer EXACTES tal com surten a la tasca o al
repositori: no els tradueixis, no els canviïs ni n'inventis de nous.
Si la tasca demana una dada del projecte (quants fitxers, què fan, quin
valor), comprova-la amb read/glob/grep/bash abans de respondre: no
responguis mai de memòria.
Si necessites diverses lectures o cerques independents, demana-les totes en
un sol bloc del mateix pas: s'executen en paral·lel i cada bloc t'estalvia
una espera sencera de model.
I no tanquis el torn amb una comprovació en vermell: si l'últim test, build
o vet falla, arregla-ho amb eines abans de respondre. Respondre «fet» amb
el vermell a la pantalla no és acabar, és deixar-ho a mitges. Però no
confonguis el teu vermell amb el preexistent: si la comprovació falla
IGUAL amb els teus canvis apartats (git stash, reprodueix, git stash
pop), ja fallava abans de tocar res — explica a l'usuari quina
comprovació és, l'error exacte i que és preexistent, i tanca el torn.
Perseguir un vermell que no pots arreglar des d'aquesta tasca és deixar
la feina a mitges també.`

// autonomousRules afegeix al mode code un contracte de llarg abast. El loop
// continua sent el mateix: el mode canvia el criteri de finalització i obliga
// el model a treballar per fites, verificar i demanar només decisions reals.
const autonomousRules = codeRules + `

Mode autònom de llarg abast:
Tracta aquesta petició com una tasca amb un contracte, no com un sol torn.
Primer concreta mentalment l'objectiu, les fites i els criteris d'acceptació;
si falta una decisió de producte o hi ha dues opcions incompatibles, usa
question abans de modificar res.
Treballa per fites petites. Després de cada fita, actualitza todowrite i
executa una comprovació adequada (tests, build, vet, lint o una lectura del
resultat). No donis la tasca per acabada només perquè una fase ha acabat.
Quan el pressupost d'un tram s'esgoti, mira el checklist, el diff i les
verificacions: respon CONTINUA mentre hi hagi progrés i FINAL només quan els
criteris estiguin complerts o hi hagi un bloqueig que requereixi l'usuari.
Mantén l'abast original; si una millora interessant queda fora del contracte,
anota-la com a pendent i continua amb la feina principal.
Fitxers grans (HTML/CSS/JS d'una sola pàgina, documents, fitxers de dades):
la primera versió completa en UN SOL write (encara que sigui llarg), i els
retocs després amb patch d'un o uns quants blocs. No reconstrueixis mai un
fitxer per trossos amb writes successius: cada write gran torna a passar
pel context i el torn es menja els passos i els minuts. Si un write hagués
de sortir tallat, verifica amb read el que hi ha i continua amb patch.`
