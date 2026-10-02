package skills

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Les skills integrades han de ser llegibles totes: una capçalera mal
// escrita no fallaria la compilació i el gregal es quedaria sense saber
// coses de si mateix, sense que ho digués ningú.
func TestLesIntegradesSonValides(t *testing.T) {
	entrades, err := integrades.ReadDir("integrades")
	if err != nil {
		t.Fatal(err)
	}
	cat := Integrades()
	if len(cat) != len(entrades) {
		t.Fatalf("hi ha %d fitxers i només %d skills vàlides", len(entrades), len(cat))
	}
	vistos := map[string]bool{}
	for _, s := range cat {
		if vistos[s.Nom] {
			t.Errorf("nom repetit: %q", s.Nom)
		}
		vistos[s.Nom] = true
		if strings.Contains(s.Descripcio, "\n") || len(s.Descripcio) > 160 {
			t.Errorf("%s: la descripció ha de ser una línia curta (%d)", s.Nom, len(s.Descripcio))
		}
	}
	// Les que expliquen el gregal a si mateix no poden faltar.
	for _, vol := range []string{"gregal", "sessions", "configuracio", "modes", "skills"} {
		if !vistos[vol] {
			t.Errorf("falta la skill integrada %q", vol)
		}
	}
}

// Un projecte pot afegir-ne i tapar-ne una d'integrada; l'últim
// directori mana.
func TestElProjecteTapaLaIntegrada(t *testing.T) {
	dir := t.TempDir()
	escriu(t, dir, "sessions.md", "---\nnom: sessions\ndescripcio: com les desem nosaltres\n---\nA la base de dades.\n")
	escriu(t, dir, "desplegament.md", "---\nnom: desplegament\ndescripcio: com es desplega això\n---\nAmb make deploy.\n")

	cat := Cataleg(dir)
	per := map[string]Skill{}
	for _, s := range cat {
		per[s.Nom] = s
	}
	if per["sessions"].Descripcio != "com les desem nosaltres" {
		t.Fatalf("la del projecte ha de tapar la integrada: %q", per["sessions"].Descripcio)
	}
	if per["desplegament"].Cos != "Amb make deploy." {
		t.Fatalf("la nova ha de ser al catàleg: %+v", per["desplegament"])
	}
	if per["gregal"].Font != "integrada" {
		t.Fatal("les integrades que ningú no tapa segueixen")
	}
	if s, ok := Per("DESPLEGAMENT.md", dir); !ok || s.Nom != "desplegament" {
		t.Fatalf("el nom s'ha de normalitzar: %+v", s)
	}
}

// Sense capçalera no és una skill: si no, qualsevol README del
// directori acabaria a l'índex del prompt.
func TestSenseCapcaleraNoEsUnaSkill(t *testing.T) {
	for _, cas := range []string{
		"# Notes\n\nunes notes qualssevol\n",
		"---\nnom: x\n---\nsense descripció\n",
		"---\ndescripcio: sense nom\n---\ncos\n",
		"---\nnom: x\ndescripcio: y\n---\n",
	} {
		if s, ok := Parse(cas, "prova"); ok {
			t.Errorf("no hauria de valer: %q → %+v", cas, s)
		}
	}
	dir := t.TempDir()
	escriu(t, dir, "README.md", "# Llegeix-me\n\nres a veure\n")
	if n := len(DeDir(dir)); n != 0 {
		t.Fatalf("un README no és una skill: %d", n)
	}
}

// L'índex és el que va al prompt: una línia per skill, amb el nom i per
// a què serveix, i diu com obrir-les.
func TestIndex(t *testing.T) {
	idx := Index(Cataleg())
	if !strings.Contains(idx, "`skill`") {
		t.Fatalf("l'índex ha de dir com obrir-les:\n%s", idx)
	}
	for _, s := range Cataleg() {
		if !strings.Contains(idx, "- "+s.Nom+": "+s.Descripcio) {
			t.Fatalf("falta la línia de %q:\n%s", s.Nom, idx)
		}
	}
	if Index(nil) != "" {
		t.Fatal("sense skills, cap capçalera")
	}
}

func escriu(t *testing.T, dir, nom, cos string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, nom), []byte(cos), 0o600); err != nil {
		t.Fatal(err)
	}
}
