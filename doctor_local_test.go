package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gregal/internal/config"
)

func liniesText(ls []liniaDoctor) string {
	var b strings.Builder
	for _, l := range ls {
		b.WriteString(l.estat + " " + l.detall + "\n")
	}
	return b.String()
}

func TestDoctorLocalLlamaCpp(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/props" {
			fmt.Fprint(w, `{"default_generation_settings":{"n_ctx":8192,"params":{"context_shift":true}},"total_slots":1,"chat_template":"{{ messages }}"}`)
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()
	out := liniesText(checksLocal(context.Background(), "en", localTarget{nom: "local", base: srv.URL + "/v1"}))
	for _, want := range []string{
		"ok local: llama.cpp, n_ctx 8",
		"warn local: set `type: llamacpp`",
		"--no-context-shift",
		"Generic format",
		"below 16",
		"--cache-reuse 256", "-ctk q8_0", "--spec-type ngram-mod",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("falta %q a:\n%s", want, out)
		}
	}
}

func TestDoctorLocalOllamaPerV1(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/ps":
			fmt.Fprint(w, `{"models":[{"name":"qwen3:latest","context_length":4096}]}`)
		case "/api/show":
			fmt.Fprint(w, `{"capabilities":["completion"]}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	out := liniesText(checksLocal(context.Background(), "ca", localTarget{nom: "ollama", base: srv.URL + "/v1", models: []string{"qwen3"}}))
	for _, want := range []string{"posa `type: ollama`", "num_ctx 4", "per sota de 16", "no admet eines", "OLLAMA_KV_CACHE_TYPE=q8_0"} {
		if !strings.Contains(out, want) {
			t.Errorf("falta %q a:\n%s", want, out)
		}
	}
}

func TestLocalTargetsNomésLocalsITipats(t *testing.T) {
	cfg := &config.Config{
		Providers: map[string]config.Provider{
			"ollama": {Type: "ollama", BaseURL: "http://gpu-box:11434/v1"},
			"local":  {BaseURL: "http://127.0.0.1:8080/v1"},
			"cloud":  {BaseURL: "https://api.openai.com/v1"},
			"ant":    {Type: "anthropic", BaseURL: "http://localhost:9999/v1"},
		},
		Roles: map[string]config.Role{
			"chat":  {Provider: "ollama", Model: "qwen3", FallbackProvider: "cloud", FallbackModel: "gpt"},
			"code":  {Provider: "local", Model: "q"},
			"other": {Provider: "ant", Model: "claude"},
		},
	}
	ts := localTargets(cfg)
	noms := map[string]bool{}
	for _, t := range ts {
		noms[t.nom] = true
	}
	if len(ts) != 2 || !noms["ollama"] || !noms["local"] {
		t.Fatalf("targets=%+v", ts)
	}
}
