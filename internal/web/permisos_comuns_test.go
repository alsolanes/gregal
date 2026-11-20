package web

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"gregal/internal/agent"
	"gregal/internal/config"
)

// El nivell d'accés d'Actua és un de sol per a Codi i per als Vents: la
// mateixa acció rep la mateixa decisió als dos modes. Codi decideix amb la
// política del torn (la que runTurn copia de s.policy) i les seves portes
// dures (codiPortaDura); el vent, amb el seu torn interactiu sencer
// (handleWindChat), on una acció que cal aprovar queda sense fer amb la nota
// «cal el teu permís».
func TestMateixaReglaDePermisosACodiIVents(t *testing.T) {
	fora := filepath.ToSlash(filepath.Join(t.TempDir(), "fora.txt"))
	bash := func(cmd string) string { b, _ := json.Marshal(map[string]string{"command": cmd}); return string(b) }
	fix := func(args string) func(string) string { return func(string) string { return args } }
	publica := func(web string) string {
		b, _ := json.Marshal(map[string]string{"dir": web, "name": "receptes", "visibility": webPublic, "title": "Receptes"})
		return string(b)
	}
	privada := func(web string) string {
		b, _ := json.Marshal(map[string]string{"dir": web, "title": "Receptes"})
		return string(b)
	}
	cases := []struct {
		nom, tool  string
		args       func(web string) string
		ask, edits string // decisió amb «Demana abans de canviar» i amb «edicions»
		// jaPublica: la web «receptes» ja és pública i confirmada abans del torn.
		jaPublica bool
		// fet diu si l'eina s'ha executat quan el torn del vent no passa per
		// windToolExec (publish el fa el servidor web).
		fet func(s *Server) bool
	}{
		{nom: "escriure dins", tool: "write", args: fix(`{"path":"nou.txt","content":"x"}`), ask: "ask", edits: "allow"},
		{nom: "editar dins", tool: "edit", args: fix(`{"path":"nou.txt","old_string":"x","new_string":"y"}`), ask: "ask", edits: "allow"},
		{nom: "ordre de desenvolupament", tool: "bash", args: fix(bash("npm install")), ask: "ask", edits: "allow"},
		{nom: "commit local", tool: "git_commit", args: fix(`{"message":"canvi"}`), ask: "ask", edits: "allow"},
		{nom: "push", tool: "bash", args: fix(bash("git push origin main")), ask: "ask", edits: "ask"},
		{nom: "PR", tool: "gh_pr_create", args: fix(`{"title":"t","body":"b"}`), ask: "ask", edits: "ask"},
		{nom: "escriure fora", tool: "write", args: fix(`{"path":"` + fora + `","content":"x"}`), ask: "ask", edits: "ask"},
		{nom: "instal·lar (porta dura)", tool: "bash", args: fix(bash("go install example.com/eina@latest")), ask: "ask", edits: "ask"},
		{nom: "esborrar fora", tool: "bash", args: fix(bash("rm -rf ../altre")), ask: "ask", edits: "ask"},
		{nom: "denegat per la política", tool: "bash", args: fix(bash("sudo ls")), ask: "deny", edits: "deny"},
		{nom: "publicar la carpeta del projecte (enllaç privat)", tool: "publish", args: privada, ask: "ask", edits: "allow",
			fet: func(s *Server) bool { return len(listShares(s.usuari())) > 0 }},
		{nom: "primera publicació pública (porta dura)", tool: "publish", args: publica, ask: "ask", edits: "ask",
			fet: func(*Server) bool { return getWeb("receptes") != nil }},
		{nom: "actualitzar una web ja pública", tool: "publish", args: publica, ask: "ask", edits: "allow", jaPublica: true,
			fet: func(*Server) bool { w := getWeb("receptes"); return w != nil && len(w.Versions) == 2 }},
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
				args := c.args(webDelProjecte(t, s, c.jaPublica))

				pol := *s.policy
				pol.ProjectDir = s.cwd
				pol.AskEdits = s.cfg.Permissions.AskEdits()
				got, why := pol.Decide(s.mode, c.tool, args)
				if motiu, ask := s.codiPortaDura(c.tool, args); ask && got != "deny" {
					got, why = "ask", motiu
				}
				if got != want {
					t.Errorf("Codi: %s (%s), volia %s", got, why, want)
				}

				if got, resposta := windDecision(t, s, c.tool, args, c.fet); got != want {
					t.Errorf("Vents: %s, volia %s; resposta=%s", got, want, resposta)
				}
			})
		}
	}
}

// webDelProjecte crea una carpeta web dins del projecte i, si cal, la publica
// abans com a «receptes» pública i confirmada. Les comprovacions de la web van
// al servidor de prova, no a internet.
func webDelProjecte(t *testing.T, s *Server, publica bool) string {
	t.Helper()
	t.Setenv("GREGAL_PUBLISH_DOMAIN", "example.org")
	t.Setenv("GREGAL_PUBLISH_SCHEME", "http")
	srv := httptest.NewServer(s.Hub().Handler())
	t.Cleanup(srv.Close)
	old := webCheckDial
	webCheckDial = func(ctx context.Context, network, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, srv.Listener.Addr().String())
	}
	t.Cleanup(func() { webCheckDial = old })
	dir := filepath.Join(s.cwd, "web")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<h1>Receptes</h1>"), 0o600); err != nil {
		t.Fatal(err)
	}
	if publica {
		files, err := s.siteFilesFromDir(dir)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := s.webPublish(webPublishOpts{Name: "receptes", Visibility: webPublic, Title: "Receptes", Source: dir, PublicOK: true}, files); err != nil {
			t.Fatal(err)
		}
	}
	return filepath.ToSlash(dir)
}

// windDecision fa demanar l'eina a un vent i diu què n'ha decidit el torn:
// allow (s'ha executat), ask (no s'ha fet perquè cal el permís) o deny (la
// política l'ha bloquejat). fet, si no és nil, diu si l'eina s'ha executat.
// Qualsevol altre final del torn és «indeterminat», no un deny.
func windDecision(t *testing.T, s *Server, tool, args string, fet func(*Server) bool) (string, string) {
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
	waitBackground()
	body := rec.Body.String()
	mu.Lock()
	defer mu.Unlock()
	if fet != nil {
		executada = fet(s)
	}
	blocked := strings.Contains(body, "TOOL BLOCKED: ")
	permis := strings.Contains(body, "cal el teu permís") || strings.Contains(body, "it needs your permission")
	switch {
	case executada && !blocked && !permis:
		return "allow", body
	case permis && strings.Contains(body, "TOOL BLOCKED: NOT RUN") && !executada:
		return "ask", body
	case blocked && !permis && !executada:
		return "deny", body
	}
	return "indeterminat", body
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

// A Codi, la primera publicació pública d'un nom pregunta encara que publicar
// la carpeta del projecte passi sol amb «edicions», com als vents; amb el sí
// la web queda pública. Actualitzar-la després ja no pregunta.
func TestCodiPrimeraPublicacioPublicaPregunta(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	s := goalTestServer(t)
	s.cwd = t.TempDir()
	s.policy.ProjectDir = s.cwd
	s.cfg.Agent.MaxSteps = 6
	web := webDelProjecte(t, s, false)
	model := publishModel(t, web)
	s.cfg.Providers = map[string]config.Provider{"p": {BaseURL: model.URL + "/v1"}}

	var mu sync.Mutex
	aprovacions, abans := 0, ""
	prou := make(chan struct{})
	vigila := make(chan struct{})
	go func() {
		defer close(vigila)
		for {
			select {
			case <-prou:
				return
			case <-time.After(10 * time.Millisecond):
			}
			s.mu.Lock()
			var ch chan bool
			for k, ar := range s.approvals {
				ch = ar.ch
				delete(s.approvals, k)
			}
			s.mu.Unlock()
			if ch != nil {
				mu.Lock()
				aprovacions++
				if w := getWeb("receptes"); w != nil {
					abans = w.Visibility
				}
				mu.Unlock()
				ch <- true
			}
		}
	}()
	torn := func(task string) string {
		rec := httptest.NewRecorder()
		s.handleAgent(rec, httptest.NewRequest(http.MethodPost, "/api/agent", strings.NewReader(`{"task":"`+task+`","mode":"code"}`)))
		waitBackground()
		return rec.Body.String()
	}
	primer := torn("publica les receptes a internet")
	mu.Lock()
	n, vis := aprovacions, abans
	mu.Unlock()
	if n != 1 || vis != "" {
		t.Fatalf("la primera publicació pública ha de preguntar abans de publicar res: aprovacions=%d visibilitat abans=%q\n%s", n, vis, primer)
	}
	if w := getWeb("receptes"); w == nil || w.Visibility != webPublic || !w.PublicConfirmed {
		t.Fatalf("amb el sí, la web ha de quedar pública: %+v\n%s", w, primer)
	}
	segon := torn("torna-la a publicar")
	close(prou)
	<-vigila
	if aprovacions != 1 {
		t.Fatalf("una actualització no pregunta: aprovacions=%d\n%s", aprovacions, segon)
	}
	if w := getWeb("receptes"); w == nil || len(w.Versions) != 2 {
		t.Fatalf("actualització: %+v\n%s", w, segon)
	}
}
