package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"gregal/internal/config"
	"gregal/internal/session"
)

func fitxerStdin(t *testing.T, contingut string) *os.File {
	t.Helper()
	p := filepath.Join(t.TempDir(), "stdin")
	if err := os.WriteFile(p, []byte(contingut), 0o600); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(p)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { f.Close() })
	return f
}

// `git diff | gregal -p "revisa"`: el que arriba per stdin va amb la tasca.
func TestTascaAmbStdin(t *testing.T) {
	got, err := tascaAmbStdin("review this", fitxerStdin(t, "diff --git a/x b/x\n+hola\n"))
	if err != nil || !strings.HasPrefix(got, "review this\n\nInput piped on stdin:\n<stdin>\ndiff --git") || !strings.HasSuffix(got, "+hola\n</stdin>") {
		t.Fatalf("got %q, %v", got, err)
	}
	// -p - : la tasca sencera ve de stdin.
	if got, err = tascaAmbStdin("-", fitxerStdin(t, "explain main.go\n")); err != nil || got != "explain main.go" {
		t.Fatalf("-p -: %q, %v", got, err)
	}
	if _, err = tascaAmbStdin("-", fitxerStdin(t, "")); err == nil {
		t.Fatal("-p - amb stdin buit ha de fallar")
	}
	// Stdin buit no canvia la tasca.
	if got, _ = tascaAmbStdin("task", fitxerStdin(t, "")); got != "task" {
		t.Fatalf("stdin buit: %q", got)
	}
	// Massa llarg: es talla i es diu.
	got, _ = tascaAmbStdin("t", fitxerStdin(t, strings.Repeat("x", maxStdin+10)))
	if !strings.Contains(got, "truncated to the first 256 KB") || strings.Count(got, "x") != maxStdin {
		t.Fatalf("tall: %d x", strings.Count(got, "x"))
	}
}

func TestAplicaModel(t *testing.T) {
	cfg := &config.Config{
		Providers: map[string]config.Provider{"zen": {}, "or": {}},
		Roles: map[string]config.Role{
			"code": {Provider: "zen", Model: "a"}, "chat": {Provider: "zen", Model: "b"},
			"reviewer": {Provider: "zen", Model: "r"},
		},
	}
	if err := aplicaModel(cfg, "or/anthropic/claude-x"); err != nil {
		t.Fatal(err)
	}
	if r := cfg.Roles["code"]; r.Provider != "or" || r.Model != "anthropic/claude-x" {
		t.Fatalf("code=%+v", r)
	}
	if r := cfg.Roles["reviewer"]; r.Model != "r" {
		t.Fatalf("el revisor no es toca: %+v", r)
	}
	// Sense proveïdor conegut davant, el text sencer és el model.
	if err := aplicaModel(cfg, "deepseek/v4"); err != nil || cfg.Roles["chat"].Model != "deepseek/v4" || cfg.Roles["chat"].Provider != "or" {
		t.Fatalf("chat=%+v, %v", cfg.Roles["chat"], err)
	}
	if err := aplicaModel(cfg, "zen/"); err == nil {
		t.Fatal("zen/ sense model ha de fallar")
	}
}

// La tirada es desa com a sessió; -c la continua amb la conversa
// d'abans, i --resume NOM també. Sense sessió a la carpeta, -c falla.
func TestRunHeadlessDesaIContinua(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	t.Setenv("GREGAL_MEMORY", t.TempDir()+"/memory.md")
	var mu sync.Mutex
	var rebuts [][]map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		raw, _ := io.ReadAll(r.Body)
		var body struct {
			Messages []map[string]any `json:"messages"`
		}
		json.Unmarshal(raw, &body)
		mu.Lock()
		rebuts = append(rebuts, body.Messages)
		n := len(rebuts)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"choices":[{"message":{"role":"assistant","content":"resposta %d"},"finish_reason":"stop"}]}`, n)
	}))
	defer srv.Close()
	cfg := func() *config.Config {
		return &config.Config{
			Mode:      "code",
			Providers: map[string]config.Provider{"t": {BaseURL: srv.URL}},
			Roles:     map[string]config.Role{"code": {Provider: "t", Model: "m", MaxTokens: 100}},
			Verify:    config.VerifyCfg{Mode: "off"},
			Router:    config.RouterCfg{Mode: "off"},
			Agent:     config.AgentCfg{MaxSteps: 5},
		}
	}
	wd, _ := os.Getwd()
	dir := t.TempDir()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(wd)

	if code := runHeadless(cfg(), "primera pregunta sobre el projecte", "text", false, 0, "", "", 1, headlessExtra{Desa: true, Continua: true}); code != exitUsage {
		t.Fatalf("-c sense cap sessió: codi %d", code)
	}
	if code := runHeadless(cfg(), "primera pregunta sobre el projecte", "text", false, 0, "", "", 1, headlessExtra{Desa: true}); code != exitOK {
		t.Fatalf("primera tirada: %d", code)
	}
	infos, _ := session.List(session.DefaultDir())
	if len(infos) != 1 || !mateixaCarpeta(infos[0].Workspace, dir) {
		t.Fatalf("sessions=%+v", infos)
	}
	nom := infos[0].Name
	if code := runHeadless(cfg(), "i la segona?", "text", false, 0, "", "", 1, headlessExtra{Desa: true, Continua: true}); code != exitOK {
		t.Fatalf("-c: %d", code)
	}
	mu.Lock()
	darrer := rebuts[len(rebuts)-1]
	mu.Unlock()
	var text []string
	for _, m := range darrer {
		if c, _ := m["content"].(string); c != "" {
			text = append(text, c)
		}
	}
	tot := strings.Join(text, "\n")
	if !strings.Contains(tot, "primera pregunta") || !strings.Contains(tot, "resposta 1") || !strings.Contains(tot, "i la segona?") {
		t.Fatalf("-c no porta la conversa d'abans:\n%s", tot)
	}
	// Es desa al mateix fitxer, no en un de nou.
	infos, _ = session.List(session.DefaultDir())
	s, _ := session.Load(session.DefaultDir(), nom)
	if len(infos) != 1 || len(s.Convo) != 4 {
		t.Fatalf("sessions=%d, convo=%d", len(infos), len(s.Convo))
	}
	if code := runHeadless(cfg(), "tercera", "text", false, 0, "", "", 1, headlessExtra{Desa: true, Resume: nom}); code != exitOK {
		t.Fatalf("--resume: %d", code)
	}
	if s, _ = session.Load(session.DefaultDir(), nom); len(s.Convo) != 6 {
		t.Fatalf("--resume no ha continuat: %d missatges", len(s.Convo))
	}
	if code := runHeadless(cfg(), "x", "text", false, 0, "", "", 1, headlessExtra{Desa: true, Resume: "no-existeix"}); code != exitUsage {
		t.Fatalf("--resume inexistent: %d", code)
	}
}

// --output stream-json: una línia per pas, crida i resultat, i el
// resultat final amb "type":"result".
func TestRunHeadlessStreamJSON(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	t.Setenv("GREGAL_MEMORY", t.TempDir()+"/memory.md")
	var mu sync.Mutex
	n := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		mu.Lock()
		n++
		k := n
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if k == 1 {
			fmt.Fprint(w, `{"choices":[{"message":{"role":"assistant","content":"","tool_calls":[{"id":"c1","type":"function","function":{"name":"read","arguments":"{\"path\":\"nota.txt\"}"}}]},"finish_reason":"tool_calls"}]}`)
			return
		}
		fmt.Fprint(w, `{"choices":[{"message":{"role":"assistant","content":"diu hola"},"finish_reason":"stop"}]}`)
	}))
	defer srv.Close()
	cfg := &config.Config{
		Mode:      "code",
		Providers: map[string]config.Provider{"t": {BaseURL: srv.URL}},
		Roles:     map[string]config.Role{"code": {Provider: "t", Model: "m", MaxTokens: 100}},
		Verify:    config.VerifyCfg{Mode: "off"},
		Router:    config.RouterCfg{Mode: "off"},
		Agent:     config.AgentCfg{MaxSteps: 5},
	}
	wd, _ := os.Getwd()
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "nota.txt"), []byte("hola\n"), 0o600)
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(wd)

	r, w, _ := os.Pipe()
	old := os.Stdout
	os.Stdout = w
	code := runHeadless(cfg, "què diu nota.txt?", "stream-json", false, 0, "", "", 1, headlessExtra{})
	w.Close()
	os.Stdout = old
	raw, _ := io.ReadAll(r)
	if code != exitOK {
		t.Fatalf("codi %d\n%s", code, raw)
	}
	var tipus []string
	var darrer map[string]any
	for _, l := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		var v map[string]any
		if err := json.Unmarshal([]byte(l), &v); err != nil {
			t.Fatalf("línia no JSON %q: %v", l, err)
		}
		tipus = append(tipus, v["type"].(string))
		darrer = v
	}
	got := strings.Join(tipus, ",")
	if !strings.Contains(got, "tool_call") || !strings.Contains(got, "tool_result") || !strings.HasSuffix(got, "result") {
		t.Fatalf("tipus: %s", got)
	}
	if darrer["answer"] != "diu hola" || darrer["steps"] == nil {
		t.Fatalf("resultat: %v", darrer)
	}
}
