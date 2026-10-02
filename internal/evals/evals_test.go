package evals

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"gregal/internal/config"
	"gregal/internal/llm"
)

// Banc de dues tasques amb un proveïdor fals: una amb assert que passa
// (el model escriu el fitxer) i una sense assert. Comprova el circuit:
// còpia de la fixture, torn, log, assert amb sh, taula i baseline.
func TestRunBancFals(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sense sh")
	}
	dir := t.TempDir()
	tasks := filepath.Join(dir, "tasks")
	os.MkdirAll(filepath.Join(tasks, "t1"), 0o755)
	os.WriteFile(filepath.Join(tasks, "t1", "base.txt"), []byte("hola\n"), 0o600)
	os.WriteFile(filepath.Join(tasks, "t1.prompt"), []byte("escriu sortida.txt amb el text ok"), 0o600)
	os.WriteFile(filepath.Join(tasks, "t1.assert"), []byte("#!/bin/sh\ngrep -q ok sortida.txt && test -f base.txt\n"), 0o755)
	os.WriteFile(filepath.Join(tasks, "t2.prompt"), []byte("digues hola"), 0o600)

	n := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		n++
		w.Header().Set("Content-Type", "application/json")
		if n == 1 {
			w.Write([]byte(`{"choices":[{"message":{"content":"","tool_calls":[{"id":"c1","type":"function","function":{"name":"write","arguments":"{\"path\":\"sortida.txt\",\"content\":\"ok\\n\"}"}}]},"finish_reason":"tool_calls"}]}`))
			return
		}
		w.Write([]byte(`{"choices":[{"message":{"content":"fet"},"finish_reason":"stop"}]}`))
	}))
	defer srv.Close()
	cfg := &config.Config{
		Providers: map[string]config.Provider{"p": {BaseURL: srv.URL}},
		Roles:     map[string]config.Role{"code": {Provider: "p", Model: "m", MaxTokens: 100}, "chat": {Provider: "p", Model: "m", MaxTokens: 100}},
		Verify:    config.VerifyCfg{Mode: "off"},
		Agent:     config.AgentCfg{MaxSteps: 5},
		Mode:      "code",
	}
	llista, err := Load(dir)
	if err != nil || len(llista) != 2 || llista[0].Fixture == "" || llista[0].Assert == "" || llista[1].Fixture != "" {
		t.Fatalf("Load: %v %+v", err, llista)
	}
	var events []string
	res, err := Run(context.Background(), cfg, llm.New(), dir, llista, Options{MaxSteps: 5, OnEvent: func(s string) { events = append(events, s) }})
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 2 || res[0].Verdict != "PASS" || res[1].Verdict != "SENSE-ASSERT" {
		t.Fatalf("resultats: %+v", res)
	}
	if _, err := os.Stat(filepath.Join(dir, "results", "t1", "sortida.txt")); err != nil {
		t.Fatal("el fitxer s'havia d'escriure a la còpia, no a la fixture")
	}
	if _, err := os.Stat(filepath.Join(tasks, "t1", "sortida.txt")); err == nil {
		t.Fatal("la fixture no s'ha de tocar")
	}
	if _, err := os.Stat(filepath.Join(dir, "results", "summary.json")); err != nil {
		t.Fatal("falta summary.json")
	}
	var out bytes.Buffer
	passed, total, regs := Report(&out, res, nil)
	if passed != 1 || total != 2 || len(regs) != 0 || !strings.Contains(out.String(), "1/2 PASS") {
		t.Fatalf("report: %d/%d %v\n%s", passed, total, regs, out.String())
	}
	// Baseline: si abans t2 passava i ara no, és regressió.
	if err := SaveBaseline(dir, []Result{{Task: "t1", Verdict: "PASS"}, {Task: "t2", Verdict: "PASS"}}); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	_, _, regs = Report(&out, res, LoadBaseline(dir))
	if len(regs) != 1 || regs[0] != "t2" || !strings.Contains(out.String(), "REGRESSIONS") {
		t.Fatalf("regressions: %v\n%s", regs, out.String())
	}
}
