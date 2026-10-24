package web

import (
	"strings"

	"gregal/internal/companion"
)

// Els vents propis no tenen "specialty" de colla: el seu ofici és text lliure
// al Role ("Música i descobertes", "Futbol i Barça"). Però la cara li posa
// l'eina del seu ofici (una pilota, uns auriculars, una proveta) segons el kit.
//
// kitOf és el punt únic on es decideix quina eina porta un vent: la seva
// especialitat de colla si la té, o la que endevina el rol. inferKit és una
// aposta conservadora: si no ho té clar, cap objecte (cara neta). No es desa
// enlloc; es calcula en llistar els vents, així un vent ja creat rep la seva
// eina el mateix moment que n'hi hagi una de nova.

// kitOf torna l'objecte d'ofici que duu un vent: la seva especialitat de
// colla si la té, o la inferida del seu rol.
func kitOf(w companion.Wind) string {
	if w.Specialty != "" {
		return w.Specialty
	}
	return inferKit(w.Role)
}

// inferKit endevina el kit a partir del text del rol, en català o anglès.

// Llista ordenada (els mapes Go iteren en ordre aleatori): l'específic mana
// sobre el general — seguretat i música abans que tecnologia, perquè
// "Seguretat informàtica" és seguretat, no tecnologia.
var kitKeywords = []struct {
	kit    string
	words  []string
}{
	{"security", []string{"seguretat", "security", "hack", "vulnerab", "privacitat", "privacy"}},
	{"music", []string{"música", "musica", "music", "canç", "song", "disc", "concert", "banda", "guitarra"}},
	{"sports", []string{"esport", "sport", "futbol", "fútbol", "football", "basket", "bàsquet", "pilota", "lliga", "barça", "barca", "goal", "partit"}},
	{"books", []string{"llibre", "llibres", "book", "novel", "lectura", "reading", "literatura", "poesia"}},
	{"shopping", []string{"compress", "compra", "shopping", "botiga", "preu", "price"}},
	{"organisation", []string{"organitza", "organiza", "organi", "agenda", "calendar", "planific"}},
	{"research", []string{"recerca", "research", "ciència", "ciencia", "science", "experiment", "lab", "provet", "descobr", "discover", "investig"}},
	{"frontend", []string{"codi", "code", "programa", "program", "desenvolup", "develop", "app", "web", "frontend"}},
	{"ml", []string{"machine learning", " aprenentatge", "learning", "intel·ligència artificial", "inteligència artificial", "intel·ligencia", "model de llenguatge", "llm"}},
	{"tech", []string{"tecnolog", "technolog", "tech", "novetat", "gadget", "digital", "internet", "informàt", "software", "xip", "chip"}},
	{"testing", []string{"testeig", "testing", "qualitat de codi", "qa "}},
	{"delivery", []string{"entrega", "delivery", "enviaments", "shipping"}},
}

// inferKit torna el kit de l'ofici que duu el rol, o "" si cap encaixa.
func inferKit(role string) string {
	r := " " + strings.ToLower(strings.TrimSpace(role)) + " "
	for _, kw := range kitKeywords {
		for _, w := range kw.words {
			if strings.Contains(r, w) {
				return kw.kit
			}
		}
	}
	return ""
}
