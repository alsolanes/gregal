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
)

//go:embed integrades/*.md
var integrades embed.FS

// Skill és un document amb nom i una descripció d'una línia.
type Skill struct {
	Nom        string
	Descripcio string
	Cos        string
	// Font diu d'on surt: "integrada" o la ruta del fitxer.
	Font string
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
	b.WriteString("SKILLS (coneixement que pots obrir quan et faci falta; crida l'eina `skill` amb el nom per llegir-la sencera, i fes-ho ABANS de dir que no saps una cosa o de deduir-la):\n")
	for _, s := range cat {
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
		case "descripcio", "descripció", "description":
			s.Descripcio = valor
		}
	}
	if s.Nom == "" || s.Descripcio == "" || s.Cos == "" {
		return Skill{}, false
	}
	return s, true
}
