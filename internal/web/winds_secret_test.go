package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"gregal/internal/tools"
)

// Formulari segur: el valor va al magatzem, la decisió no el guarda i les
// ordres el reben com a variable d'entorn, tapat a la sortida.

func TestSecretIDAndPlaceholder(t *testing.T) {
	if got := secretID("  Clau OpenWeather!! "); got != "clau_openweather" {
		t.Fatalf("secretID = %q", got)
	}
	if secretEnvName("openweather_key") != "GREGAL_SECRET_OPENWEATHER_KEY" {
		t.Fatal("env name")
	}
	if !secretPlaceholder.MatchString("curl -H 'k: {{secret:openweather_key}}'") {
		t.Fatal("placeholder")
	}
	if got := maskSecrets("token=abcd1234 ok abcd1234", []string{"abcd1234"}); got != "token=[secret] ok [secret]" {
		t.Fatalf("mask = %q", got)
	}
}

func TestSecretAnswerGoesToTheVaultNotTheDecision(t *testing.T) {
	s := goalTestServer(t)
	// Una decisió «secret» pendent d'aquesta persona.
	rec := &windDecisionRec{Proc: windProc, windAskReq: windAskReq{ID: "wa-secrettest1", Key: "wa-secrettest1", Type: "secret", Name: "openweather_key", Query: "Clau d'OpenWeather", State: "pending", Owner: s.remindOwner(), Expires: time.Now().Add(time.Hour).UnixMilli(), ExpiresAt: time.Now().Add(time.Hour).UTC().Format(time.RFC3339)}}
	if err := s.withDecisions(func(st *decisionDir) error { return st.save(rec) }); err != nil {
		t.Fatal(err)
	}
	body := `{"key":"wa-secrettest1","approve":true,"secret":"sk-super-secret-42"}`
	w := httptest.NewRecorder()
	s.handleWindsAnswer(w, httptest.NewRequest(http.MethodPost, "/api/winds/answer", strings.NewReader(body)))
	if w.Code != 200 {
		t.Fatalf("answer = %d %s", w.Code, w.Body)
	}
	v, err := s.secretVault().Get(s.remindOwner(), secretRefPrefix+"openweather_key")
	if err != nil || string(v) != "sk-super-secret-42" {
		t.Fatalf("vault = %q, %v", v, err)
	}
	t.Cleanup(func() { _ = s.secretVault().Delete(s.remindOwner(), secretRefPrefix+"openweather_key") })
	var saved *windDecisionRec
	_ = s.withDecisions(func(st *decisionDir) error { saved, _ = st.load("wa-secrettest1"); return nil })
	raw, _ := json.Marshal(saved)
	if strings.Contains(string(raw), "sk-super-secret-42") {
		t.Fatalf("la decisió desada no pot portar el valor: %s", raw)
	}
	if saved.State != "approved" {
		t.Fatalf("state = %s", saved.State)
	}
	// La llista només ensenya l'id.
	w = httptest.NewRecorder()
	s.handleWindSecrets(w, httptest.NewRequest(http.MethodGet, "/api/winds/secrets", nil))
	if !strings.Contains(w.Body.String(), "openweather_key") || strings.Contains(w.Body.String(), "sk-super") {
		t.Fatalf("secrets = %s", w.Body)
	}

	// Una ordre amb la referència: variable d'entorn, mai el valor al text.
	ctx, args, vals, err := s.withSecrets(context.Background(), "bash", `{"command":"echo {{secret:openweather_key}}"}`)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(args, "sk-super") || !strings.Contains(args, "$GREGAL_SECRET_OPENWEATHER_KEY") {
		t.Fatalf("args = %s", args)
	}
	if env := tools.ExtraEnv(ctx); len(env) != 1 || env[0] != "GREGAL_SECRET_OPENWEATHER_KEY=sk-super-secret-42" {
		t.Fatalf("env = %v", env)
	}
	if len(vals) != 1 {
		t.Fatalf("vals = %v", vals)
	}
	if _, _, _, err := s.withSecrets(context.Background(), "bash", `{"command":"echo {{secret:no_existeix}}"}`); err == nil || !strings.Contains(err.Error(), "secret tool") {
		t.Fatalf("missing = %v", err)
	}
	if _, _, _, err := s.withSecrets(context.Background(), "bash_background", `{"command":"echo {{secret:openweather_key}}"}`); err == nil {
		t.Fatal("bash_background no pot fer servir secrets")
	}
	// Una decisió que no és «secret» no desa res.
	if ok, _ := s.storeSecretAnswer("wa-no-existeix", "x"); ok {
		t.Fatal("decisió inexistent")
	}
}
