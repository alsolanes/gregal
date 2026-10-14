package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gregal/internal/agent"
	"gregal/internal/config"
	"gregal/internal/llm"
)

func TestParseExportArgs(t *testing.T) {
	a, err := parseExportArgs([]string{"html", "out.html", "--redact"})
	if err != nil || a.format != "html" || a.path != "out.html" || !a.redact || a.emails {
		t.Fatalf("%+v %v", a, err)
	}
	a, _ = parseExportArgs(nil)
	if a.format != "md" || a.path != "" || a.redact {
		t.Fatalf("per defecte: %+v", a)
	}
	if a, _ := parseExportArgs([]string{"--emails"}); !a.redact || !a.emails {
		t.Fatalf("--emails implica redact: %+v", a)
	}
	if _, err := parseExportArgs([]string{"--xyz"}); err == nil {
		t.Fatal("opció desconeguda")
	}
}

func TestExportConversaEscriuFitxer(t *testing.T) {
	dir := t.TempDir()
	convo := []llm.Message{
		{Role: "user", Content: "Fix it\n\n" + agent.MarcaContext + " x]\nmapfile.go"},
		{Role: "assistant", Content: "Done with key literal-tui-key-0099"},
	}
	cfg := &config.Config{Providers: map[string]config.Provider{"p": {APIKey: "literal-tui-key-0099"}}}
	now := time.Date(2026, 10, 13, 15, 4, 5, 0, time.UTC)
	path, err := exportConversa(convo, dir, cfg, exportArgs{format: "md", redact: true}, now)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(path) != dir || filepath.Base(path) != "gregal-fix-it-20261013-150405.md" {
		t.Fatalf("camí: %s", path)
	}
	raw, _ := os.ReadFile(path)
	s := string(raw)
	if strings.Contains(s, "mapfile.go") || strings.Contains(s, "literal-tui-key-0099") || !strings.Contains(s, "## User\n\nFix it") {
		t.Fatalf("contingut:\n%s", s)
	}
	// Camí relatiu i HTML.
	path, err = exportConversa(convo, dir, cfg, exportArgs{format: "html", path: "x.html"}, now)
	if err != nil || path != filepath.Join(dir, "x.html") {
		t.Fatalf("%s %v", path, err)
	}
	if _, err := exportConversa(nil, dir, cfg, exportArgs{format: "md"}, now); err == nil {
		t.Fatal("conversa buida")
	}
}
