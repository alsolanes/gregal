package web

import (
	"gregal/internal/companion"
	"gregal/internal/llm"
	"gregal/internal/session"
	"testing"
)

// A server restart must use the same account's persisted files, while another
// account must not gain access to them.
func TestServerAccountStateSurvivesRestart(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	h := testHubWithUsers(t)
	a := h.SessionFor(&User{Name: "anna"}, "phone")
	if _, err := a.winds.Save(companion.Wind{Name: "Tramuntana"}); err != nil {
		t.Fatal(err)
	}
	group, err := a.saveRotllana(Rotllana{Name: "Idees", Members: []string{"tramuntana"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := session.SaveSession(a.sessDir(), session.Session{Name: "prova", Convo: []llm.Message{{Role: "user", Content: "Hola"}}}); err != nil {
		t.Fatal(err)
	}
	if err := session.SaveModelDefault("anna", "chat", "provider/model"); err != nil {
		t.Fatal(err)
	}
	restarted := New(h.def.cfg, h.def.cfgPath).Hub()
	again := restarted.SessionFor(&User{Name: "anna"}, "desktop")
	if _, ok := again.winds.ByID("tramuntana"); !ok {
		t.Fatal("wind lost after restart")
	}
	if _, ok := again.rotllanaByID(group.ID); !ok {
		t.Fatal("group lost after restart")
	}
	saved, err := session.Load(again.sessDir(), "prova")
	if err != nil || len(saved.Convo) != 1 || saved.Convo[0].Content != "Hola" {
		t.Fatalf("conversation lost: %v", err)
	}
	models, err := session.LoadModelDefaults("anna")
	if err != nil || models["chat"] != "provider/model" {
		t.Fatalf("model preference lost: %v", err)
	}
	other := restarted.SessionFor(&User{Name: "bert"}, "desktop")
	if _, ok := other.winds.ByID("tramuntana"); ok {
		t.Fatal("wind visible to another account")
	}
	if _, err := session.Load(other.sessDir(), "prova"); err == nil {
		t.Fatal("conversation visible to another account")
	}
}
