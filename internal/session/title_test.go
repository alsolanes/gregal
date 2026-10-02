package session

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gregal/internal/llm"
)

// El títol surt del primer missatge de l'usuari: una conversa sense nom no
// es torna a obrir mai, perquè no saps quina és.
func TestTitleFrom(t *testing.T) {
	casos := []struct {
		nom   string
		convo []llm.Message
		vol   string
	}{
		{"primer missatge d'usuari", []llm.Message{
			{Role: "system", Content: "ets un agent"},
			{Role: "user", Content: "revisa el pilar 3"},
			{Role: "assistant", Content: "va"},
		}, "revisa el pilar 3"},
		{"només la primera línia", []llm.Message{
			{Role: "user", Content: "arregla el TUI\nque no fa scroll\ni salta"},
		}, "arregla el TUI"},
		{"sense el fitxer adjunt", []llm.Message{
			{Role: "user", Content: "resumeix això\n\n[@informe.docx]\nun munt de text"},
		}, "resumeix això"},
		{"sense el prefix de l'agent", []llm.Message{
			{Role: "user", Content: "🤖 afegeix tests"},
		}, "afegeix tests"},
		{"sense missatges d'usuari", []llm.Message{
			{Role: "assistant", Content: "hola"},
		}, ""},
		{"conversa buida", nil, ""},
	}
	for _, c := range casos {
		if got := TitleFrom(c.convo); got != c.vol {
			t.Errorf("%s: TitleFrom = %q, volia %q", c.nom, got, c.vol)
		}
	}
}

// Un primer missatge llarg es retalla amb «…», que la llista de l'esquerra
// és estreta.
func TestTitleFromRetalla(t *testing.T) {
	llarg := strings.Repeat("paraula ", 30)
	got := TitleFrom([]llm.Message{{Role: "user", Content: llarg}})
	if r := []rune(got); len(r) > 60 {
		t.Fatalf("títol de %d runes: %q", len(r), got)
	}
	if !strings.HasSuffix(got, "…") {
		t.Fatalf("el títol retallat ha d'acabar en «…»: %q", got)
	}
}

// Desar amb el mateix nom reescriu la mateixa conversa en comptes de deixar
// còpies, i el títol viatja al llistat.
func TestSaveMateixNomNoDuplica(t *testing.T) {
	dir := t.TempDir()
	convo := []llm.Message{{Role: "user", Content: "revisa el pilar 3"}}
	p1, err := Save(dir, "sessio-20260915-101010", "code", convo)
	if err != nil {
		t.Fatal(err)
	}
	convo = append(convo, llm.Message{Role: "assistant", Content: "fet"})
	p2, err := Save(dir, "sessio-20260915-101010", "code", convo)
	if err != nil {
		t.Fatal(err)
	}
	if p1 != p2 {
		t.Fatalf("dos fitxers per a la mateixa conversa: %s i %s", filepath.Base(p1), filepath.Base(p2))
	}
	infos, err := List(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(infos) != 1 {
		t.Fatalf("volia 1 conversa, en tinc %d", len(infos))
	}
	if infos[0].Title != "revisa el pilar 3" {
		t.Fatalf("títol=%q", infos[0].Title)
	}
	if infos[0].Msgs != 2 {
		t.Fatalf("msgs=%d (no s'ha actualitzat)", infos[0].Msgs)
	}
}

// Les sessions d'abans del títol no en tenen al fitxer: se'n treu del
// contingut perquè la llista no quedi plena de «sessio-2026…».
func TestTitolDeSessionsVelles(t *testing.T) {
	dir := t.TempDir()
	if _, err := Save(dir, "vella", "code", []llm.Message{{Role: "user", Content: "una tasca antiga"}}); err != nil {
		t.Fatal(err)
	}
	// Simulem el format d'abans: sense camp title.
	raw := `{"version":1,"name":"vella","saved_at":"2026-01-01T10:00:00Z","role":"code",` +
		`"convo":[{"role":"user","content":"una tasca antiga"}]}`
	if err := writeFile(filepath.Join(dir, "vella.json"), raw); err != nil {
		t.Fatal(err)
	}
	infos, _ := List(dir)
	if len(infos) != 1 || infos[0].Title != "una tasca antiga" {
		t.Fatalf("infos=%+v", infos)
	}
}

func writeFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0o600)
}
