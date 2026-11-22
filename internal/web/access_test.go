package web

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"gregal/internal/agent"
	"gregal/internal/config"
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

// Un «access: ask» antic al config no pot sobreviure a triar l'accés complet:
// el menú diria «complet» mentre els vents (que segueixen AskEdits) preguntarien.
func TestAccesCompletEsborraElDemanaDesat(t *testing.T) {
	s := goalTestServer(t)
	s.cfg.Permissions.Access = config.AccessAsk
	s.policy.AskEdits = true
	if code, out := postAccess(t, s, `{"access":"full"}`); code != 200 || out["access"] != agent.AccessFull {
		t.Fatalf("full: %d %v", code, out)
	}
	if s.cfg.Permissions.AskEdits() || s.policy.AskEdits {
		t.Fatal("full ha de treure el «demana abans de canviar» del config i de la política")
	}
	desat, err := os.ReadFile(s.cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(desat), config.AccessAsk) {
		t.Fatalf("el config desat encara diu ask:\n%s", desat)
	}
}

// Després de triar l'accés complet, el torn d'un vent deixa passar sola la
// feina dins del projecte; push i instal·lacions globals continuen preguntant.
func TestAccesCompletDeixaTreballarElsVents(t *testing.T) {
	s, _ := companionTestServer(t)
	s.cwd = t.TempDir()
	s.cfg.Permissions.Access = config.AccessAsk
	s.policy.AskEdits = true
	if code, out := postAccess(t, s, `{"access":"full"}`); code != 200 {
		t.Fatalf("access: %d %v", code, out)
	}
	scope := s.interactiveWindCtx(context.Background()).Value(interactiveWindKey{}).(windTurnScope)
	bash := func(cmd string) string { b, _ := json.Marshal(map[string]string{"command": cmd}); return string(b) }
	for _, c := range []struct{ nom, tool, args, want string }{
		{"escriure dins", "write", `{"path":"nou.txt","content":"x"}`, "allow"},
		{"npm test", "bash", bash("npm test"), "allow"},
		{"git push", "bash", bash("git push origin main"), "ask"},
		{"npm i -g", "bash", bash("npm i -g eina"), "ask"},
	} {
		if got, why := scope.policy.Decide(scope.mode, c.tool, c.args); got != c.want {
			t.Errorf("%s: %s (%s), volia %s", c.nom, got, why, c.want)
		}
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
