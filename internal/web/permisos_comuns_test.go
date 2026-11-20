package web

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"gregal/internal/agent"
	"gregal/internal/config"
)

// El nivell d'accés d'Actua és un de sol per a Codi i per als Vents: la
// mateixa acció rep la mateixa decisió als dos modes. Codi decideix amb la
// política del torn (la que runTurn copia de s.policy); el vent, amb el seu
// torn interactiu sencer (handleWindChat), on una acció que cal aprovar queda
// sense fer amb la nota «cal el teu permís».
func TestMateixaReglaDePermisosACodiIVents(t *testing.T) {
	fora := filepath.ToSlash(filepath.Join(t.TempDir(), "fora.txt"))
	bash := func(cmd string) string { b, _ := json.Marshal(map[string]string{"command": cmd}); return string(b) }
	cases := []struct {
		nom, tool, args string
		ask, edits      string // decisió amb «Demana abans de canviar» i amb «edicions»
	}{
		{"escriure dins", "write", `{"path":"nou.txt","content":"x"}`, "ask", "allow"},
		{"editar dins", "edit", `{"path":"nou.txt","old_string":"x","new_string":"y"}`, "ask", "allow"},
		{"ordre de desenvolupament", "bash", bash("npm install"), "ask", "allow"},
		{"commit local", "git_commit", `{"message":"canvi"}`, "ask", "allow"},
		{"push", "bash", bash("git push origin main"), "ask", "ask"},
		{"PR", "gh_pr_create", `{"title":"t","body":"b"}`, "ask", "ask"},
		{"escriure fora", "write", `{"path":"` + fora + `","content":"x"}`, "ask", "ask"},
		{"instal·lar (porta dura)", "bash", bash("go install example.com/eina@latest"), "ask", "ask"},
		{"esborrar fora", "bash", bash("rm -rf ../altre"), "ask", "ask"},
		{"denegat per la política", "bash", bash("sudo ls"), "deny", "deny"},
	}
	for _, access := range []string{agent.AccessAsk, agent.AccessEdits} {
		for _, c := range cases {
			want := c.edits
			if access == agent.AccessAsk {
				want = c.ask
			}
			t.Run(access+"/"+c.nom, func(t *testing.T) {
				t.Setenv("GREGAL_DATA_DIR", t.TempDir())
				s, _ := companionTestServer(t)
				s.cwd = t.TempDir()
				makeRotllanaTrio(t, s)
				if code, out := postAccess(t, s, `{"access":"`+access+`"}`); code != 200 {
					t.Fatalf("access: %d %v", code, out)
				}

				pol := *s.policy
				pol.ProjectDir = s.cwd
				pol.AskEdits = s.cfg.Permissions.AskEdits()
				if got, why := pol.Decide(s.mode, c.tool, c.args); got != want {
					t.Errorf("Codi: %s (%s), volia %s", got, why, want)
				}

				if got, resposta := windDecision(t, s, c.tool, c.args); got != want {
					t.Errorf("Vents: %s, volia %s; resposta=%s", got, want, resposta)
				}
			})
		}
	}
}

// windDecision fa demanar l'eina a un vent i diu què n'ha decidit el torn:
// allow (s'ha executat), ask (no s'ha fet perquè cal el permís) o deny.
func windDecision(t *testing.T, s *Server, tool, args string) (string, string) {
	t.Helper()
	var mu sync.Mutex
	executada := false
	old := windToolExec
	windToolExec = func(_ context.Context, _, _, name, _ string) (string, []string, error) {
		mu.Lock()
		defer mu.Unlock()
		if name == tool {
			executada = true
		}
		return "fet", nil, nil
	}
	t.Cleanup(func() { windToolExec = old })
	provider := windPermissionProvider(t, tool, args)
	s.cfg.Providers["p"] = config.Provider{BaseURL: provider.URL + "/v1"}
	rec := postJSON(t, s.handleWindChat, `{"wind":"tramuntana","message":"fes-ho"}`)
	if rec.Code != 200 {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	mu.Lock()
	defer mu.Unlock()
	switch {
	case executada:
		return "allow", body
	case strings.Contains(body, "cal el teu permís") || strings.Contains(body, "it needs your permission"):
		return "ask", body
	}
	return "deny", body
}

// Els casos propis dels vents no canvien amb la regla comuna: amb qualsevol
// nivell d'accés, els que parlen després del primer a la rotllana (i una
// revisió) només llegeixen, l'estudi només escriu a la seva carpeta i els xats
// compartits només tenen les eines de web.
func TestElsCasosPropisDelsVentsEsMantenen(t *testing.T) {
	for _, access := range []string{agent.AccessAsk, agent.AccessEdits} {
		t.Run(access, func(t *testing.T) {
			s, _ := companionTestServer(t)
			s.cwd = t.TempDir()
			if code, out := postAccess(t, s, `{"access":"`+access+`"}`); code != 200 {
				t.Fatalf("access: %d %v", code, out)
			}
			dins := `{"path":"nou.txt","content":"x"}`
			decide := func(ctx context.Context, args string) string {
				scope := ctx.Value(interactiveWindKey{}).(windTurnScope)
				d, _ := scope.policy.Decide(scope.mode, "write", args)
				return d
			}
			if d := decide(s.readOnlyWindCtx(s.interactiveWindCtx(context.Background())), dins); d != "deny" {
				t.Errorf("rotllana després del primer / revisió: write=%s, volia deny", d)
			}
			practica := t.TempDir()
			study := s.windStudyCtx(context.Background(), practica)
			if d := decide(study, `{"path":"`+filepath.ToSlash(filepath.Join(practica, "a.go"))+`","content":"x"}`); d != "allow" {
				t.Errorf("estudi a la seva carpeta: write=%s, volia allow", d)
			}
			if d := decide(study, `{"path":"`+filepath.ToSlash(filepath.Join(s.cwd, "a.go"))+`","content":"x"}`); d == "allow" {
				t.Errorf("estudi fora de la seva carpeta: write=%s", d)
			}
			if sharedWebTool("write") || sharedWebTool("bash") || !sharedWebTool("web_search") {
				t.Error("un xat compartit només té les eines de web")
			}
		})
	}
}
