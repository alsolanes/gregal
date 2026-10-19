// Package skills són trossos de coneixement que l'agent carrega quan li
// fan falta, en comptes de portar-los sempre al prompt.
//
// N'hi ha de dues menes. Les integrades (integrades/*.md, dins del
// binari) expliquen el gregal a si mateix: on desa les sessions, què hi
// ha al config, què vol dir cada mode. Sense això, demanar-li «esborra
// les sessions velles» no anava enlloc, perquè el model no sabia ni que
// existien ni on eren. Les altres les escriu qui vulgui: un projecte pot
// posar les seves a .gregal/skills/ i una persona les seves a
// ~/.config/gregal/skills/, amb el mateix format.
//
// Al prompt hi va només l'índex (nom i una línia per skill); el cos
// arriba quan el model crida l'eina `skill`. Dotze skills senceres al
// prompt serien milers de tokens a cada pas per res.
package skills

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

//go:embed integrades/*.md
var integrades embed.FS

//go:embed curades/*.md
var curades embed.FS

var habilitadesMu sync.RWMutex
var habilitades = map[string]bool{
	"anti-ai-slop": true, "data-science": true, "statistics": true,
	"ai-engineering": true, "product-design": true,
}
var nomsCurades = []string{"anti-ai-slop", "data-science", "statistics", "ai-engineering", "product-design"}

// PerDefecte són les habilitats especialistes activades en una instal·lació nova.
func PerDefecte() []string { return append([]string(nil), nomsCurades...) }

// Skill és un document amb nom i una descripció d'una línia.
type Skill struct {
	Nom        string
	Titol      string
	Descripcio string
	Cos        string
	// Font diu d'on surt: "integrada" o la ruta del fitxer.
	Font string
}

// Curades són les habilitats especialistes del producte que l'usuari pot
// activar. El cos només es retorna quan l'eina skill n'obre una.
func Curades() []Skill {
	entrades, err := curades.ReadDir("curades")
	if err != nil {
		return nil
	}
	var out []Skill
	for _, e := range entrades {
		raw, err := curades.ReadFile("curades/" + e.Name())
		if err != nil {
			continue
		}
		if s, ok := Parse(string(raw), "curada"); ok {
			out = append(out, s)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Nom < out[j].Nom })
	return out
}

// NomsCurades retorna els ids que es poden commutar des dels ajustos.
func NomsCurades() []string { return append([]string(nil), nomsCurades...) }

// Habilitades retorna les skills optatives activades, en ordre estable.
func Habilitades() []string {
	habilitadesMu.RLock()
	defer habilitadesMu.RUnlock()
	out := make([]string, 0, len(habilitades))
	for name, on := range habilitades {
		if on {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}

// DefineixHabilitades valida i canvia les skills optatives actives.
func DefineixHabilitades(names []string) error {
	if err := ValidaHabilitades(names); err != nil {
		return err
	}
	next := make(map[string]bool, len(names))
	for _, raw := range names {
		next[Normalitza(raw)] = true
	}
	habilitadesMu.Lock()
	habilitades = next
	habilitadesMu.Unlock()
	return nil
}

// ValidaHabilitades comprova noms sense canviar la selecció activa.
func ValidaHabilitades(names []string) error {
	permeses := make(map[string]bool, len(nomsCurades))
	for _, name := range nomsCurades {
		permeses[name] = true
	}
	for _, raw := range names {
		name := Normalitza(raw)
		if !permeses[name] {
			return fmt.Errorf("no hi ha cap skill optativa que es digui %q", raw)
		}
	}
	return nil
}

// EsCurada indica si el nom correspon a una habilitat optativa del producte.
func EsCurada(name string) bool {
	name = Normalitza(name)
	for _, candidate := range nomsCurades {
		if candidate == name {
			return true
		}
	}
	return false
}

// Habilitada indica si una skill optativa està activa; les altres sempre ho són.
func Habilitada(name string) bool {
	if !EsCurada(name) {
		return true
	}
	habilitadesMu.RLock()
	defer habilitadesMu.RUnlock()
	return habilitades[Normalitza(name)]
}

// Integrades són les skills que van dins del binari.
func Integrades() []Skill {
	entrades, err := integrades.ReadDir("integrades")
	if err != nil {
		return nil
	}
	var out []Skill
	for _, e := range entrades {
		raw, err := integrades.ReadFile("integrades/" + e.Name())
		if err != nil {
			continue
		}
		if s, ok := Parse(string(raw), "integrada"); ok {
			out = append(out, s)
		}
	}
	return out
}

// DeDir llegeix els *.md d'un directori. Un directori que no hi és no és
// cap error: la majoria de projectes no en tindran.
func DeDir(dir string) []Skill {
	if strings.TrimSpace(dir) == "" {
		return nil
	}
	rutes, err := filepath.Glob(filepath.Join(dir, "*.md"))
	if err != nil {
		return nil
	}
	var out []Skill
	for _, r := range rutes {
		raw, err := os.ReadFile(r)
		if err != nil {
			continue
		}
		if s, ok := Parse(string(raw), r); ok {
			out = append(out, s)
		}
	}
	return out
}

// Cataleg són les integrades més les dels directoris donats, per nom.
// Una skill d'un directori tapa la integrada que es digui igual: així un
// projecte pot corregir el que el gregal creu saber de si mateix, i
// l'últim directori mana sobre els anteriors.
func Cataleg(dirs ...string) []Skill {
	per := map[string]Skill{}
	for _, s := range Integrades() {
		per[s.Nom] = s
	}
	for _, s := range Curades() {
		per[s.Nom] = s
	}
	for _, d := range dirs {
		for _, s := range DeDir(d) {
			per[s.Nom] = s
		}
	}
	out := make([]Skill, 0, len(per))
	for _, s := range per {
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Nom < out[j].Nom })
	return out
}

// Per busca una skill pel nom (sense distingir majúscules).
func Per(nom string, dirs ...string) (Skill, bool) {
	nom = Normalitza(nom)
	for _, s := range Cataleg(dirs...) {
		if s.Nom == nom {
			return s, true
		}
	}
	return Skill{}, false
}

// Noms són els noms del catàleg, ordenats.
func Noms(dirs ...string) []string {
	cat := Cataleg(dirs...)
	out := make([]string, 0, len(cat))
	for _, s := range cat {
		out = append(out, s.Nom)
	}
	return out
}

// Index és el bloc que va al prompt: una línia per skill. Buit si no
// n'hi ha cap, perquè el prompt no porti una capçalera sense res a sota.
func Index(cat []Skill) string {
	if len(cat) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("SKILLS (knowledge you can open when you need it: call the `skill` tool with the name to read it in full, and do it BEFORE saying you don't know something or guessing it):\n")
	for _, s := range cat {
		if !Habilitada(s.Nom) {
			continue
		}
		fmt.Fprintf(&b, "- %s: %s\n", s.Nom, s.Descripcio)
	}
	return strings.TrimRight(b.String(), "\n")
}

// Normalitza deixa el nom com el guarda el catàleg.
func Normalitza(nom string) string {
	return strings.ToLower(strings.TrimSpace(strings.TrimSuffix(nom, ".md")))
}

// Parse llegeix un fitxer de skill: capçalera entre «---» amb nom i
// descripcio, i la resta és el cos. Sense capçalera no és una skill: val
// més ignorar-la que posar un README qualsevol a l'índex del prompt.
func Parse(raw, font string) (Skill, bool) {
	text := strings.ReplaceAll(raw, "\r\n", "\n")
	if !strings.HasPrefix(text, "---\n") {
		return Skill{}, false
	}
	fi := strings.Index(text[4:], "\n---")
	if fi < 0 {
		return Skill{}, false
	}
	cap := text[4 : 4+fi]
	cos := strings.TrimLeft(text[4+fi+len("\n---"):], "\n")
	s := Skill{Font: font, Cos: strings.TrimSpace(cos)}
	for _, linia := range strings.Split(cap, "\n") {
		clau, valor, tallat := strings.Cut(linia, ":")
		if !tallat {
			continue
		}
		valor = strings.Trim(strings.TrimSpace(valor), `"'`)
		switch strings.ToLower(strings.TrimSpace(clau)) {
		case "nom", "name":
			s.Nom = Normalitza(valor)
		case "title", "titol", "títol":
			s.Titol = valor
		case "descripcio", "descripció", "description":
			s.Descripcio = valor
		}
	}
	if s.Nom == "" || s.Descripcio == "" || s.Cos == "" {
		return Skill{}, false
	}
	return s, true
}
