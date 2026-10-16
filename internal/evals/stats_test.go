package evals

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"gregal/internal/config"
	"gregal/internal/llm"
)

func aprop(a, b float64) bool { return math.Abs(a-b) < 1e-4 }

func TestPassAtK(t *testing.T) {
	cases := []struct {
		n, c, k int
		want    float64
	}{
		{10, 0, 1, 0},
		{10, 10, 1, 1},
		{10, 3, 1, 0.3},
		{5, 2, 2, 1 - 3.0/10}, // 1 - C(3,2)/C(5,2)
		{5, 2, 3, 1 - 1.0/10}, // 1 - C(3,3)/C(5,3)
		{5, 2, 4, 1},          // n-c < k
		{4, 1, 4, 1},          // k = n amb algun èxit
		{3, 0, 3, 0},          // cap èxit
		{3, 1, 9, 1},          // k > n es retalla a n
	}
	for _, c := range cases {
		if got := PassAtK(c.n, c.c, c.k); !aprop(got, c.want) {
			t.Errorf("pass@%d(n=%d,c=%d)=%v, volia %v", c.k, c.n, c.c, got, c.want)
		}
	}
}

func TestDist(t *testing.T) {
	d := dist([]float64{7, 1, 3, 5})
	// numpy: percentile([1,3,5,7],[25,50,75]) = 2.5, 4, 5.5
	if !aprop(d.Median, 4) || !aprop(d.Q1, 2.5) || !aprop(d.Q3, 5.5) || !aprop(d.IQR, 3) {
		t.Fatalf("%+v", d)
	}
	if d := dist([]float64{9}); d.Median != 9 || d.IQR != 0 {
		t.Fatalf("un sol valor: %+v", d)
	}
	if d := dist(nil); d != (Dist{}) {
		t.Fatalf("buit: %+v", d)
	}
}

// Agregació per tasca: taxa d'èxit, veredicte per majoria, pass@k i les
// taxes calculades sobre les sumes.
func TestAggregate(t *testing.T) {
	res := []Result{
		{Task: "t1", Run: 1, Verdict: "PASS", Steps: 4, Secs: 10, PromptTokens: 1000, CachedTokens: 500, LLMCalls: 4, EditCalls: 2, EditFailed: 1},
		{Task: "t1", Run: 2, Verdict: "FAIL", Steps: 8, Secs: 20, PromptTokens: 3000, CachedTokens: 1500, LLMCalls: 8, EditCalls: 2, EditFailed: 0, ToolCallInvalid: 2, DoomHits: 1},
		{Task: "t1", Run: 3, Verdict: "PASS", Steps: 6, Secs: 30, PromptTokens: 2000, CachedTokens: 0, LLMCalls: 6, Compactions: 1},
		{Task: "t2", Run: 1, Verdict: "FAIL"},
		{Task: "t2", Run: 2, Verdict: "ERROR"},
		{Task: "t2", Run: 3, Verdict: "PASS"},
		{Task: "t3", Run: 1, Verdict: "SENSE-ASSERT"},
	}
	st := Aggregate(res)
	if len(st) != 3 || st[0].Task != "t1" || st[1].Task != "t2" {
		t.Fatalf("%+v", st)
	}
	a := st[0]
	if a.Runs != 3 || a.Passes != 2 || !aprop(a.SuccessRate, 0.6667) || a.Verdict != "PASS" {
		t.Fatalf("t1: %+v", a)
	}
	if !aprop(a.PassAtK["1"], 2.0/3) || a.PassAtK["3"] != 1 {
		t.Fatalf("pass@k: %v", a.PassAtK)
	}
	if a.Steps.Median != 6 || a.Steps.IQR != 2 || a.Secs.Median != 20 || a.PromptDist.Median != 2000 || a.CallsDist.Median != 6 {
		t.Fatalf("dist: %+v %+v %+v", a.Steps, a.Secs, a.PromptDist)
	}
	// 2000 cached / 6000 prompt; 1 fallada / 4 edicions.
	if a.PromptTokens != 6000 || !aprop(a.CacheHitRate, 0.3333) || !aprop(a.EditFailRate, 0.25) ||
		a.ToolCallInvalid != 2 || a.DoomHits != 1 || a.Compactions != 1 || a.LLMCalls != 18 {
		t.Fatalf("mètriques: %+v", a.Metrics)
	}
	if st[1].Verdict != "FAIL" || st[1].Passes != 1 {
		t.Fatalf("t2 (1/3) ha de ser FAIL: %+v", st[1])
	}
	if st[2].Verdict != "SENSE-ASSERT" {
		t.Fatalf("t3: %+v", st[2])
	}
	s := Summarize(res, 3)
	if s.Totals.Runs != 7 || s.Totals.Passes != 3 || s.Totals.PromptTokens != 6000 || !aprop(s.Totals.EditFailRate, 0.25) {
		t.Fatalf("totals: %+v", s.Totals)
	}
	// Baseline: t1 passa per majoria, t2 no; si abans passava, regressió.
	dir := t.TempDir()
	if err := SaveBaseline(dir, res); err != nil {
		t.Fatal(err)
	}
	b := LoadBaseline(dir)
	if b["t1"] != "PASS" || b["t2"] != "FAIL" {
		t.Fatalf("baseline: %v", b)
	}
	var out bytes.Buffer
	_, _, regs := Report(&out, res, Baseline{"t1": "PASS", "t2": "PASS"})
	if len(regs) != 1 || regs[0] != "t2" {
		t.Fatalf("regressions: %v", regs)
	}
	for _, want := range []string{"t1#2", "pass@1", "6[2]", "cache 33%", "edit✗ 25%", "REGRESSIONS"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("falta %q a l'informe:\n%s", want, out.String())
		}
	}
	// Sense denominador, les taxes són 0 i l'informe diu «-».
	var m Metrics
	m.rates()
	if m.CacheHitRate != 0 || m.EditFailRate != 0 || pct(0, 0) != "-" {
		t.Fatal("divisió per zero")
	}
}

// El banc de bench/ es carrega amb les fixtures de fixtures/<t> i els
// asserts de tasks/<t>.assert.
func TestLoadBench(t *testing.T) {
	llista, err := Load(filepath.Join("..", "..", "bench"))
	if err != nil {
		t.Fatal(err)
	}
	var noms []string
	for _, tk := range llista {
		noms = append(noms, tk.Name)
		if tk.Fixture == "" || tk.Assert == "" || tk.Prompt == "" {
			t.Fatalf("%s incompleta: %+v", tk.Name, tk)
		}
		if filepath.Base(filepath.Dir(tk.Fixture)) != "fixtures" {
			t.Fatalf("%s: fixture %s", tk.Name, tk.Fixture)
		}
	}
	if got := strings.Join(noms, ","); got != "t1,t3,t4,t5,t6,t7,t8,t9" {
		t.Fatalf("tasques: %s", got)
	}
}

// --repeat amb proveïdor fals: cada repetició té la seva còpia i el seu
// log, l'assert veu GREGAL_EVAL_DIR i GREGAL_EVAL_FIXTURE, i summary.json
// porta l'agregat amb les mètriques noves.
func TestRunRepeat(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sense sh")
	}
	dir := t.TempDir()
	tasks := filepath.Join(dir, "tasks")
	os.MkdirAll(tasks, 0o755)
	os.MkdirAll(filepath.Join(dir, "fixtures", "t1"), 0o755)
	os.MkdirAll(filepath.Join(dir, "hidden"), 0o755)
	os.WriteFile(filepath.Join(dir, "fixtures", "t1", "base.txt"), []byte("hola\n"), 0o600)
	os.WriteFile(filepath.Join(dir, "hidden", "ok.txt"), []byte("ok\n"), 0o600)
	os.WriteFile(filepath.Join(tasks, "t1.prompt"), []byte("escriu sortida.txt amb el text ok"), 0o600)
	os.WriteFile(filepath.Join(tasks, "t1.assert"), []byte("#!/bin/sh\nset -e\ncmp -s \"$GREGAL_EVAL_FIXTURE/base.txt\" base.txt\ngrep -q ok \"$GREGAL_EVAL_DIR/hidden/ok.txt\"\ngrep -q ok sortida.txt\n"), 0o755)

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
		// Cada execució: una escriptura i la resposta. La segona
		// execució escriu una altra cosa i falla.
		switch k {
		case 1:
			w.Write([]byte(`{"choices":[{"message":{"content":"","tool_calls":[{"id":"c1","type":"function","function":{"name":"write","arguments":"{\"path\":\"sortida.txt\",\"content\":\"ok\\n\"}"}}]},"finish_reason":"tool_calls"}],"usage":{"prompt_tokens":1000,"completion_tokens":10}}`))
		case 3:
			w.Write([]byte(`{"choices":[{"message":{"content":"","tool_calls":[{"id":"c1","type":"function","function":{"name":"write","arguments":"{\"path\":\"sortida.txt\",\"content\":\"no\\n\"}"}}]},"finish_reason":"tool_calls"}],"usage":{"prompt_tokens":1000,"completion_tokens":10}}`))
		default:
			w.Write([]byte(`{"choices":[{"message":{"content":"fet"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1200,"completion_tokens":5,"prompt_tokens_details":{"cached_tokens":900}}}`))
		}
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
	if err != nil {
		t.Fatal(err)
	}
	res, err := Run(context.Background(), cfg, llm.New(), dir, llista, Options{MaxSteps: 5, Repeat: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 2 || res[0].Run != 1 || res[1].Run != 2 || res[0].Verdict != "PASS" || res[1].Verdict != "FAIL" {
		raw, _ := os.ReadFile(filepath.Join(dir, "results", "t1.r1.assert.log"))
		t.Fatalf("resultats: %+v\n%s", res, raw)
	}
	for _, p := range []string{"t1.r1/sortida.txt", "t1.r2/sortida.txt", "t1.r1.log", "t1.r2.log"} {
		if _, err := os.Stat(filepath.Join(dir, "results", p)); err != nil {
			t.Fatalf("falta results/%s", p)
		}
	}
	raw, err := os.ReadFile(filepath.Join(dir, "results", "summary.json"))
	if err != nil {
		t.Fatal(err)
	}
	var s Summary
	if err := json.Unmarshal(raw, &s); err != nil {
		t.Fatal(err)
	}
	if s.Repeat != 2 || len(s.Tasks) != 1 || len(s.Runs) != 2 {
		t.Fatalf("summary: %s", raw)
	}
	st := s.Tasks[0]
	if st.Passes != 1 || st.SuccessRate != 0.5 || st.PassAtK["1"] != 0.5 || st.PassAtK["2"] != 1 || st.Verdict != "PASS" {
		t.Fatalf("agregat: %+v", st)
	}
	if st.PromptTokens != 4400 || st.CachedTokens != 1800 || !aprop(st.CacheHitRate, 0.4091) || st.LLMCalls != 4 {
		t.Fatalf("mètriques: %+v", st.Metrics)
	}
	for _, k := range []string{"cache_hit_rate", "prompt_tokens", "edit_fail_rate", "tool_call_invalid", "doom_hits", "compactions", "llm_calls", "pass_at_k", "steps"} {
		if !strings.Contains(string(raw), `"`+k+`"`) {
			t.Fatalf("falta %q a summary.json", k)
		}
	}
}
