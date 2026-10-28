package web

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"gregal/internal/agent"
)

func postAccess(t *testing.T, s *Server, body string) (int, map[string]string) {
	t.Helper()
	w := httptest.NewRecorder()
	s.handleAccess(w, httptest.NewRequest("POST", "/api/access", bytes.NewReader([]byte(body))))
	var out map[string]string
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	return w.Code, out
}

func TestAccessTriaElPermisDActua(t *testing.T) {
	s := goalTestServer(t)
	s.mode = agent.ModeChat
	s.permissive = true

	if code, out := postAccess(t, s, `{"access":"ask"}`); code != 200 || out["mode"] != agent.ModeCode || out["access"] != agent.AccessAsk {
		t.Fatalf("ask: %d %v", code, out)
	}
	if !s.cfg.Permissions.AskEdits() || !s.policy.AskEdits || s.permissive {
		t.Fatal("ask ha de quedar al config i a la política, i treure el mode permissiu antic")
	}
	if code, out := postAccess(t, s, `{"access":"full"}`); code != 200 || s.mode != agent.ModeAutonomous || out["access"] != agent.AccessFull {
		t.Fatalf("full: %d %v mode=%s", code, out, s.mode)
	}
	if !s.permissiveNow() {
		t.Fatal("l'autonomia total també ha de deixar de preguntar al torn en curs")
	}
	// Passar per Planifica i tornar a Actua recupera l'autonomia total.
	for _, m := range []string{"plan", "act"} {
		if rec := postJSON(t, s.handleMode, `{"mode":"`+m+`"}`); rec.Code != 200 {
			t.Fatalf("%s: %d", m, rec.Code)
		}
		if m == "plan" && s.permissiveNow() {
			t.Fatal("Planifica no pot quedar en mode permissiu")
		}
	}
	if s.mode != agent.ModeAutonomous {
		t.Fatalf("act després de plan = %s, want autonomous", s.mode)
	}
	// Els noms antics del menú de permisos continuen valent.
	if code, out := postAccess(t, s, `{"access":"manual"}`); code != 200 || s.mode != agent.ModeCode || out["access"] != agent.AccessEdits {
		t.Fatalf("manual: %d %v", code, out)
	}
	if s.cfg.Permissions.AskEdits() || s.policy.AskEdits {
		t.Fatal("edits treu el «demana abans de canviar»")
	}
	if code, _ := postAccess(t, s, `{"access":"whatever"}`); code != 400 {
		t.Fatalf("valor desconegut: %d", code)
	}
	if code, _ := postAccess(t, s, `{}`); code != 400 {
		t.Fatalf("buit: %d", code)
	}
}

func TestModeAcceptaElsNomsDelSelector(t *testing.T) {
	s := goalTestServer(t)
	for word, want := range map[string]string{"plan": agent.ModePlan, "ask": agent.ModeChat, "act": agent.ModeCode} {
		w := httptest.NewRecorder()
		s.handleMode(w, httptest.NewRequest("POST", "/api/mode", bytes.NewReader([]byte(`{"mode":"`+word+`"}`))))
		if w.Code != 200 || s.mode != want {
			t.Fatalf("%s: %d mode=%s", word, w.Code, s.mode)
		}
	}
}
