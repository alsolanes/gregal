package catalog

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"
)

const mostra = `{
 "opencode-go": {"api": "https://opencode.ai/inference/go/openai/v1", "models": {
   "deepseek-v4.1-flash": {"tool_call": true, "reasoning": true, "attachment": true,
     "limit": {"context": 1000000, "output": 384000}, "cost": {"input": 0.15, "output": 0.6, "cache_read": 0.003}}}},
 "opencode": {"api": "https://opencode.ai/inference/openai/v1", "models": {
   "deepseek-v4.1-flash": {"tool_call": true, "limit": {"context": 1000000, "output": 384000}, "cost": {"input": 0.2, "output": 0.8}}}},
 "openrouter": {"api": "https://openrouter.ai/api/v1", "models": {
   "deepseek/deepseek-v4.1-flash": {"tool_call": true, "limit": {"context": 900000, "output": 100000}, "cost": {"input": 0.3, "output": 1}}}},
 "openai": {"models": {"gpt-9": {"tool_call": true, "limit": {"context": 400000, "output": 128000}, "cost": {"input": 2, "output": 8}}}},
 "altre": {"api": "https://altre.example/v1", "models": {"gpt-9": {"tool_call": true, "limit": {"context": 300000, "output": 64000}}}}
}`

func carrega(t *testing.T) {
	t.Helper()
	d, err := Parse([]byte(mostra))
	if err != nil {
		t.Fatal(err)
	}
	Set(d)
	t.Cleanup(Reset)
}

// opencode zen: la base del config (opencode.ai/zen/go/v1) no és la del
// catàleg, però la màquina i el tros «go» porten a opencode-go.
func TestLookupPerAdreca(t *testing.T) {
	carrega(t)
	m, pid, ok := Lookup("https://opencode.ai/zen/go/v1", "deepseek-v4.1-flash")
	if !ok || pid != "opencode-go" || m.Context != 1000000 || m.Output != 384000 || m.In != 0.15 || !m.HasCost || !m.ToolCall {
		t.Fatalf("zen go: %+v %q %v", m, pid, ok)
	}
	m, pid, _ = Lookup("https://openrouter.ai/api/v1", "deepseek/deepseek-v4.1-flash")
	if pid != "openrouter" || m.Context != 900000 {
		t.Fatalf("openrouter: %+v %q", m, pid)
	}
}

// Sense proveïdor identificat: el model per nom, amb la finestra més
// petita i sense preu (no se sap qui cobra).
func TestLookupPerNom(t *testing.T) {
	carrega(t)
	m, pid, ok := Lookup("https://proxy.example/v1", "gpt-9")
	if !ok || pid != "" || m.Context != 300000 || m.HasCost {
		t.Fatalf("per nom: %+v %q %v", m, pid, ok)
	}
	// «proveïdor/model» que no hi és sencer: es prova la part final.
	if m, _, ok = Lookup("https://proxy.example/v1", "acme/gpt-9"); !ok || m.Context != 300000 {
		t.Fatalf("part final: %+v %v", m, ok)
	}
	if _, _, ok = Lookup("https://proxy.example/v1", "no-existeix"); ok {
		t.Fatal("un model desconegut no ha de sortir")
	}
}

// Desactivat (models_catalog: off, o sense Configure) no hi ha res.
func TestDesactivat(t *testing.T) {
	carrega(t)
	Configure(false, "")
	if _, _, ok := Lookup("https://opencode.ai/zen/go/v1", "deepseek-v4.1-flash"); ok {
		t.Fatal("desactivat no ha de trobar res")
	}
}

// Primera vegada sense còpia: es baixa, es desa, i la següent arrencada la
// llegeix del disc sense xarxa.
func TestBaixaIDesa(t *testing.T) {
	crides := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		crides++
		w.Write([]byte(mostra))
	}))
	defer srv.Close()
	vell := SourceURL
	SourceURL = srv.URL
	PathOverride = filepath.Join(t.TempDir(), "models-catalog.json")
	t.Cleanup(func() { SourceURL, PathOverride = vell, ""; Reset() })

	Configure(true, "")
	if m, _, ok := Lookup("https://opencode.ai/zen/go/v1", "deepseek-v4.1-flash"); !ok || m.Context != 1000000 {
		t.Fatalf("primera baixada: %+v %v", m, ok)
	}
	// Espera que el fitxer quedi escrit (es desa dins de la mateixa
	// goroutine que ha baixat).
	time.Sleep(100 * time.Millisecond)
	srv.Close()
	Configure(true, "")
	if m, _, ok := Lookup("https://opencode.ai/zen/go/v1", "deepseek-v4.1-flash"); !ok || m.Context != 1000000 {
		t.Fatalf("des del disc: %+v %v", m, ok)
	}
	if crides != 1 {
		t.Fatalf("crides=%d", crides)
	}
}
