package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"gregal/internal/config"
	"gregal/internal/llm"
)

// fakeUsageServer és fakeAgentServer però declarant usage a cada resposta,
// com fan els proveïdors OpenAI-compatibles: el segon pas torna a enviar
// tot el prompt, per això en declara més.
func fakeUsageServer(t *testing.T, file string) *httptest.Server {
	t.Helper()
	n := 0
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		n++
		if n == 1 {
			inner, _ := json.Marshal(map[string]string{"path": file})
			argsField, _ := json.Marshal(string(inner))
			fmt.Fprintf(w, `{"choices":[{"message":{"role":"assistant","content":"","tool_calls":[{"id":"c1","type":"function","function":{"name":"read","arguments":%s}}]},"finish_reason":"tool_calls"}],"usage":{"prompt_tokens":1000,"completion_tokens":20}}`, argsField)
			return
		}
		fmt.Fprint(w, `{"choices":[{"message":{"role":"assistant","content":"fet i llegit"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1500,"completion_tokens":30,"prompt_tokens_details":{"cached_tokens":800}}}`)
	}))
}

// Amb usage del proveïdor, el resultat headless és la suma real de totes
// les crides (no l'estimació sobre l'historial final) i el cost en surt.
func TestRunNonInteractiveUsageReal(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "d.txt")
	if err := os.WriteFile(f, []byte("DADES"), 0o600); err != nil {
		t.Fatal(err)
	}
	srv := fakeUsageServer(t, f)
	defer srv.Close()
	cfg := testCfg(srv.URL)
	cfg.Cost = map[string]config.CostPrice{"m": {In: 1, Out: 4}}
	SetPrices(nil)
	SetupPrices(cfg)
	defer SetPrices(nil)
	res, err := RunNonInteractive(context.Background(), llm.New(), cfg, "llegeix", "code", 5, false)
	if err != nil {
		t.Fatal(err)
	}
	if res.TokensSource != "api" || res.UpTokens != 2500 || res.DownTokens != 50 || res.CachedTokens != 800 || res.LLMCalls != 2 {
		t.Fatalf("res=%+v", res)
	}
	want := 2500.0/1e6*1 + 50.0/1e6*4
	if res.CostUSD == nil || *res.CostUSD != want {
		t.Fatalf("cost=%v, volia %v", res.CostUSD, want)
	}
	// El JSON conserva els camps d'abans i afegeix l'origen.
	raw, _ := json.Marshal(res)
	var m map[string]any
	_ = json.Unmarshal(raw, &m)
	for _, k := range []string{"up_tokens", "down_tokens", "tokens_source", "cached_tokens", "cost_usd"} {
		if _, ok := m[k]; !ok {
			t.Fatalf("falta %q al JSON: %s", k, raw)
		}
	}
}

// Sense usage (llama.cpp antic, proxies que no el passen) es torna a
// l'estimació i es diu.
func TestRunNonInteractiveUsageEstimat(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "d.txt")
	if err := os.WriteFile(f, []byte("DADES"), 0o600); err != nil {
		t.Fatal(err)
	}
	srv := fakeAgentServer(t, f)
	defer srv.Close()
	res, err := RunNonInteractive(context.Background(), llm.New(), testCfg(srv.URL), "llegeix", "code", 5, false)
	if err != nil {
		t.Fatal(err)
	}
	if res.TokensSource != "estimate" || res.UpTokens < 1 || res.DownTokens < 1 || res.CachedTokens != 0 || res.LLMCalls != 2 {
		t.Fatalf("res=%+v", res)
	}
}
