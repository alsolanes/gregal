package web

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// La web que el vent ha fet en aquest torn surt al xat com a targeta, amb
// l'adreça de la previsualització; la que ja hi era abans del torn, no.
func TestWindWebCardsShowsTheWebMadeThisTurn(t *testing.T) {
	proj, repo := projectesTest(t)
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	s, _ := companionTestServer(t)
	s.mu.Lock()
	s.cwd = repo
	s.mu.Unlock()
	old := filepath.Join(proj, "vella", "index.html")
	_ = os.MkdirAll(filepath.Dir(old), 0o755)
	_ = os.WriteFile(old, []byte("<p>vella"), 0o644)
	past := time.Now().Add(-time.Hour)
	_ = os.Chtimes(old, past, past)

	start := time.Now()
	nova := filepath.Join(proj, "calculadora-propines", "index.html")
	_ = os.MkdirAll(filepath.Dir(nova), 0o755)
	_ = os.WriteFile(nova, []byte("<p>nova"), 0o644)

	got := s.windWebCards(start, "Ja el tens a la vista.")
	if strings.Count(got, "```gregal-web") != 1 {
		t.Fatalf("una targeta, la de la web nova:\n%s", got)
	}
	raw := strings.TrimSuffix(strings.TrimSpace(strings.SplitN(got, "```gregal-web\n", 2)[1]), "```")
	var c webCard
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &c); err != nil {
		t.Fatalf("json: %v\n%s", err, raw)
	}
	if c.Title != "Calculadora propines" || c.Path != "projectes/calculadora-propines/index.html" || c.URL != s.previewBase()+c.Path {
		t.Fatalf("targeta = %+v", c)
	}
	// L'adreça funciona de debò.
	if body := previewFetch(t, s, c.URL); body != "<p>nova" {
		t.Fatalf("la previsualització no serveix la web: %q", body)
	}
	// Si la resposta ja la porta, no es repeteix.
	if again := s.windWebCards(start, got); again != "" {
		t.Fatalf("repetida: %s", again)
	}
}
