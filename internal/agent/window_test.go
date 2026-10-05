package agent

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"gregal/internal/config"
)

func TestDetectWindowsDesDelProveidor(t *testing.T) {
	ResetWindows()
	t.Cleanup(ResetWindows)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" || r.Header.Get("Authorization") != "Bearer clau" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data":[
			{"id":"gran","context_length":131072},
			{"id":"vllm","max_model_len":40960},
			{"id":"llamacpp","meta":{"n_ctx_train":32768}},
			{"id":"mut"}]}`))
	}))
	defer srv.Close()
	cfg := &config.Config{
		Providers: map[string]config.Provider{"p": {BaseURL: srv.URL + "/v1", APIKey: "clau"}},
		Roles: map[string]config.Role{
			"code":  {Provider: "p", Model: "gran"},
			"chat":  {Provider: "p", Model: "mut"},
			"fixat": {Provider: "p", Model: "gran", ContextWindow: 16000},
		},
	}
	DetectWindows(context.Background(), cfg)
	casos := map[string][2]any{
		"code":  {131072, "model"},
		"chat":  {8192, "defecte"}, // el servidor de prova és 127.0.0.1: defecte local
		"fixat": {16000, "config"},
	}
	for rol, vol := range casos {
		n, src := WindowSource(cfg, cfg.Roles[rol])
		if n != vol[0].(int) || src != vol[1].(string) {
			t.Errorf("%s: %d %s (volia %v)", rol, n, src, vol)
		}
	}
	if n := Window(cfg, config.Role{Provider: "p", Model: "vllm"}); n != 40960 {
		t.Errorf("vllm: %d", n)
	}
	if n := Window(cfg, config.Role{Provider: "p", Model: "llamacpp"}); n != 32768 {
		t.Errorf("llamacpp: %d", n)
	}
	// Un error de context excedit ensenya la mida real i guanya al defecte.
	LearnWindow(srv.URL+"/v1", "mut", 4096)
	if n, src := WindowSource(cfg, cfg.Roles["chat"]); n != 4096 || src != "model" {
		t.Errorf("après: %d %s", n, src)
	}
}

func TestDetectRoleWindowsNomésConsultaRolActiuIFallback(t *testing.T) {
	ResetWindows()
	t.Cleanup(ResetWindows)
	var activeCalls, fallbackCalls, unrelatedCalls atomic.Int32
	server := func(calls *atomic.Int32, window int) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			calls.Add(1)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":[{"id":"model","context_length":` + fmt.Sprint(window) + `}]}`))
		}))
	}
	active := server(&activeCalls, 65536)
	fallback := server(&fallbackCalls, 32768)
	unrelated := server(&unrelatedCalls, 131072)
	defer active.Close()
	defer fallback.Close()
	defer unrelated.Close()
	cfg := &config.Config{
		Providers: map[string]config.Provider{
			"active": {BaseURL: active.URL}, "fallback": {BaseURL: fallback.URL}, "other": {BaseURL: unrelated.URL},
		},
		Roles: map[string]config.Role{"other-role": {Provider: "other", Model: "model"}},
	}
	role := config.Role{Provider: "active", Model: "model", FallbackProvider: "fallback", FallbackModel: "model"}
	DetectRoleWindows(context.Background(), cfg, role)
	if activeCalls.Load() != 1 || fallbackCalls.Load() != 1 || unrelatedCalls.Load() != 0 {
		t.Fatalf("requests active/fallback/unrelated = %d/%d/%d, volia 1/1/0", activeCalls.Load(), fallbackCalls.Load(), unrelatedCalls.Load())
	}
	if n, ok := KnownWindow(active.URL, "model"); !ok || n != 65536 {
		t.Fatalf("finestra activa = %d, %v", n, ok)
	}
	if n, ok := KnownWindow(fallback.URL, "model"); !ok || n != 32768 {
		t.Fatalf("finestra fallback = %d, %v", n, ok)
	}
}

func TestDetectRoleWindowsOmetProviderActiuJaConegutPeroDetectaFallback(t *testing.T) {
	ResetWindows()
	t.Cleanup(ResetWindows)
	var activeCalls, fallbackCalls atomic.Int32
	newServer := func(calls *atomic.Int32) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			calls.Add(1)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":[{"id":"model","context_length":32768}]}`))
		}))
	}
	active, fallback := newServer(&activeCalls), newServer(&fallbackCalls)
	defer active.Close()
	defer fallback.Close()
	LearnWindow(active.URL, "model", 65536)
	cfg := &config.Config{Providers: map[string]config.Provider{"a": {BaseURL: active.URL}, "b": {BaseURL: fallback.URL}}}
	DetectRoleWindows(context.Background(), cfg, config.Role{Provider: "a", Model: "model", FallbackProvider: "b", FallbackModel: "model"})
	if activeCalls.Load() != 0 || fallbackCalls.Load() != 1 {
		t.Fatalf("requests active/fallback = %d/%d, volia 0/1", activeCalls.Load(), fallbackCalls.Load())
	}
}

func TestDetectRoleWindowsOmetFinestraExplicita(t *testing.T) {
	ResetWindows()
	t.Cleanup(ResetWindows)
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1) }))
	defer srv.Close()
	cfg := &config.Config{Providers: map[string]config.Provider{"p": {BaseURL: srv.URL}}}
	DetectRoleWindows(context.Background(), cfg, config.Role{Provider: "p", Model: "m", FallbackProvider: "p", FallbackModel: "f", ContextWindow: 16384})
	if calls.Load() != 0 {
		t.Fatalf("s'han fet %d consultes amb la finestra explícita", calls.Load())
	}
}

func TestWindowDefecteLocal(t *testing.T) {
	ResetWindows()
	cfg := &config.Config{Providers: map[string]config.Provider{"local": {BaseURL: "http://127.0.0.1:8089/v1"}}}
	if n := Window(cfg, config.Role{Provider: "local", Model: "x"}); n != 8192 {
		t.Fatalf("local: %d", n)
	}
	if n := Window(nil, config.Role{Provider: "cloud", Model: "x"}); n != DefaultWindow {
		t.Fatalf("desconegut: %d", n)
	}
}
