package web

import (
	"encoding/json"
	"gregal/internal/config"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

const modelsDevSample = `{
 "opencode-go": {"name":"OpenCode Go","models":{
   "deepseek-v4.1-flash":{"id":"deepseek-v4.1-flash","name":"DeepSeek V4.1 Flash","reasoning":true,
      "reasoning_options":[{"type":"effort","values":["low","high","max"]}]},
   "plain":{"id":"plain-model","name":"Plain Model","reasoning":false}}},
 "other": {"name":"Other","models":{"deepseek-v4.1-flash":{"id":"deepseek-v4.1-flash","name":"Duplicat","reasoning":true}}}
}`

func useModelsDev(t *testing.T, raw string) {
	t.Helper()
	idx, err := parseModelsDev([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	modelsDev.mu.Lock()
	old, oldAt := modelsDev.idx, modelsDev.fetched
	modelsDev.idx, modelsDev.fetched = idx, time.Now()
	modelsDev.mu.Unlock()
	t.Cleanup(func() { modelsDev.mu.Lock(); modelsDev.idx, modelsDev.fetched = old, oldAt; modelsDev.mu.Unlock() })
}

func TestModelsDevGivesNamesAndEfforts(t *testing.T) {
	useModelsDev(t, modelsDevSample)
	if got := modelDisplayName("zen/deepseek-v4.1-flash"); got != "DeepSeek V4.1 Flash" {
		t.Fatalf("nom = %q", got)
	}
	if got := modelDisplayName("inventat"); got != "" {
		t.Fatalf("un model desconegut no té nom: %q", got)
	}
	if got := strings.Join(thinkOptionsFor("deepseek-v4.1-flash"), ","); got != "default,auto,no,low,high,max" {
		t.Fatalf("opcions = %s", got)
	}
	if got := thinkOptionsFor("plain-model"); got != nil {
		t.Fatalf("un model sense raonament no ofereix nivells: %v", got)
	}
	if got := strings.Join(thinkOptionsFor("desconegut"), ","); got != "default,auto,no,low,medium,high,max" {
		t.Fatalf("model desconegut: %s", got)
	}
}

func TestModelsDevDuplicateProviderSelectionIsDeterministic(t *testing.T) {
	for i := 0; i < 100; i++ {
		idx, err := parseModelsDev([]byte(modelsDevSample))
		if err != nil {
			t.Fatal(err)
		}
		got, ok := idx["deepseek-v4.1-flash"]
		if !ok || got.Name != "DeepSeek V4.1 Flash" {
			t.Fatalf("iteration %d selected duplicate provider record: %+v, found=%v", i, got, ok)
		}
		if strings.Join(got.Efforts, ",") != "low,high,max" {
			t.Fatalf("iteration %d changed duplicate provider effort data: %+v", i, got.Efforts)
		}
	}
}

func TestThinkLevelOverridesTheRoleButNotTheReviewer(t *testing.T) {
	s, _ := companionTestServer(t)
	s.mu.Lock()
	s.cfg.Roles["code"] = withThink(s.cfg.Roles["code"], "auto")
	s.cfg.Roles["reviewer"] = withThink(s.cfg.Roles["reviewer"], "no")
	s.mu.Unlock()
	post := func(level string) int {
		w := httptest.NewRecorder()
		s.handleThink(w, httptest.NewRequest("POST", "/api/think", strings.NewReader(`{"level":"`+level+`"}`)))
		return w.Code
	}
	if post("tremendous") != 400 {
		t.Fatal("un nivell desconegut s'ha de rebutjar")
	}
	if post("high") != 200 {
		t.Fatal("high ha de valer")
	}
	s.mu.Lock()
	_, code := s.roleRefFor("code")
	_, rev := s.roleRefFor("reviewer")
	s.mu.Unlock()
	if code.Think != "high" {
		t.Fatalf("code.Think = %q", code.Think)
	}
	if rev.Think != "no" {
		t.Fatalf("el revisor no segueix el nivell: %q", rev.Think)
	}
	post("default")
	s.mu.Lock()
	_, code = s.roleRefFor("code")
	s.mu.Unlock()
	if code.Think != "auto" {
		t.Fatalf("«default» torna al config: %q", code.Think)
	}
	w := httptest.NewRecorder()
	s.handleState(w, httptest.NewRequest("GET", "/api/state", nil))
	var st map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &st); err != nil {
		t.Fatal(err)
	}
	if _, ok := st["think_options"]; !ok {
		t.Fatal("l'estat ha de portar think_options")
	}
}

func withThink(r config.Role, level string) config.Role { r.Think = level; return r }

// Els tests no baixen mai el catàleg real.
func init() { os.Setenv("GREGAL_NO_MODELS_DEV", "1") }
