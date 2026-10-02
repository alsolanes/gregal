package agent

import (
	"context"
	"net/http"
	"net/http/httptest"
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
