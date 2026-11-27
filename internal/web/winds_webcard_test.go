package web

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gregal/internal/llm"
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

	hist := []llm.Message{{Role: "assistant", ToolCalls: []llm.ToolCall{{Function: struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	}{Name: "write", Arguments: `{"path":"calculadora-propines/index.html"}`}}}}}
	got := s.windWebCards(start, "Ja el tens a la vista.", hist)
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
	if again := s.windWebCards(start, got, hist); again != "" {
		t.Fatalf("repetida: %s", again)
	}
}

// Dos torns alhora: la web que ha fet l'altre vent també és nova, però
// aquesta resposta només porta la targeta de la que ha escrit aquest torn.
func TestWindWebCardsOnlyForTheWebsThisTurnWrote(t *testing.T) {
	proj, repo := projectesTest(t)
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	s, _ := companionTestServer(t)
	s.mu.Lock()
	s.cwd = repo
	s.mu.Unlock()
	start := time.Now()
	for _, d := range []string{"escacs", "cafe-port", "rius"} {
		p := filepath.Join(proj, d, "index.html")
		_ = os.MkdirAll(filepath.Dir(p), 0o755)
		_ = os.WriteFile(p, []byte("<p>"+d), 0o644)
	}
	call := func(name, args string) llm.ToolCall {
		var c llm.ToolCall
		c.Function.Name, c.Function.Arguments = name, args
		return c
	}
	hist := []llm.Message{{Role: "assistant", ToolCalls: []llm.ToolCall{
		call("read", `{"path":"`+filepath.ToSlash(filepath.Join(proj, "rius", "index.html"))+`"}`),
		call("write", `{"path":"`+filepath.ToSlash(filepath.Join(proj, "escacs", "index.html"))+`","content":"<p>escacs"}`),
	}}}
	got := s.windWebCards(start, "Fet.", hist)
	if strings.Count(got, "```gregal-web") != 1 || !strings.Contains(got, "projectes/escacs/index.html") {
		t.Fatalf("només la targeta d'escacs:\n%s", got)
	}
	// Una ruta relativa (escrita des de la carpeta de projectes) o feta amb bash també compta.
	rel := []llm.Message{{Role: "assistant", ToolCalls: []llm.ToolCall{
		call("edit", `{"path":"cafe-port/index.html","old":"a","new":"b"}`),
		call("bash", `{"command":"cat > rius/index.html <<'X'\n<p>rius\nX"}`),
	}}}
	got = s.windWebCards(start, "Fet.", rel)
	if strings.Count(got, "```gregal-web") != 2 || strings.Contains(got, "escacs") {
		t.Fatalf("cafè i rius, no escacs:\n%s", got)
	}
	// Un torn que no ha escrit res no porta cap targeta.
	if none := s.windWebCards(start, "Fet.", nil); none != "" {
		t.Fatalf("cap targeta sense escriptures:\n%s", none)
	}
}
