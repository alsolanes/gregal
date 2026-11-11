package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gregal/internal/config"
	"gregal/internal/runs"
)

// publishModel: un proveïdor fals que demana publicar la carpeta en públic
// amb nom i, amb el resultat de l'eina, el repeteix.
func publishModel(t *testing.T, dir string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Messages []struct {
				Role    string `json:"role"`
				Content any    `json:"content"`
			} `json:"messages"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		w.Header().Set("Content-Type", "text/event-stream")
		if last := req.Messages[len(req.Messages)-1]; last.Role == "tool" {
			b, _ := json.Marshal("Rebut: " + fmt.Sprint(last.Content))
			fmt.Fprintf(w, "data: {\"choices\":[{\"delta\":{\"content\":%s},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n", b)
			return
		}
		args, _ := json.Marshal(map[string]any{"dir": dir, "name": "receptes", "visibility": "public", "title": "Receptes"})
		wire, _ := json.Marshal(string(args))
		fmt.Fprintf(w, "data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":0,\"id\":\"c1\",\"function\":{\"name\":\"publish\",\"arguments\":%s}}]},\"finish_reason\":\"tool_calls\"}]}\n\ndata: [DONE]\n\n", wire)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// El primer «public» d'un nom nou passa per una decisió durable (B4), també
// quan la política deixaria publicar sol; aprovada, la web queda pública i
// el rebut arriba al fil.
func TestWebsFirstPublicIsAThreadDecision(t *testing.T) {
	shortLive(t)
	t.Setenv("GREGAL_PUBLISH_DOMAIN", "example.org")
	t.Setenv("GREGAL_PUBLISH_SCHEME", "http")
	s := remindTestServer(t)
	s.cwd = t.TempDir()
	site := filepath.Join(s.cwd, "receptes")
	if err := os.MkdirAll(site, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(site, "index.html"), []byte("<h1>Receptes</h1>"), 0o600); err != nil {
		t.Fatal(err)
	}
	model := publishModel(t, site)
	s.cfg.Providers["p"] = config.Provider{BaseURL: model.URL + "/v1"}
	s.cfg.Agent.MaxSteps = 6
	s.cfg.Permissions.Tools = map[string]string{"publish": "allow"}
	wind, _ := s.winds.ByID("xaloc")
	if err := s.clearWindHistory(wind, windHistory(s, "xaloc")); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(s.Hub().Handler())
	t.Cleanup(srv.Close)
	old := webCheckDial
	webCheckDial = func(ctx context.Context, network, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, srv.Listener.Addr().String())
	}
	t.Cleanup(func() { webCheckDial = old })

	raw := windChat(t, s, "Publica les receptes a internet")
	if !strings.Contains(raw, "event: approve_request") {
		t.Fatalf("havia de preguntar:\n%s", raw)
	}
	if getWeb("receptes") != nil {
		t.Fatal("res publicat abans de decidir")
	}
	pend := pendingWindAsks(s)
	if len(pend) != 1 || pend[0].Kind != "publish_public" || !strings.Contains(pend[0].What, "receptes.example.org") {
		t.Fatalf("decisió: %+v", pend)
	}
	if rec := answerWindAsk(t, s, `{"key":"`+pend[0].ID+`","approve":true}`); rec.Code != 200 {
		t.Fatalf("respondre: %d %s", rec.Code, rec.Body)
	}
	if run := waitRun(t, s, pend[0].RunID); run.State != runs.Completed {
		t.Fatalf("run: %+v", run)
	}
	w := getWeb("receptes")
	if w == nil || w.Visibility != webPublic || w.Current != 1 || !w.PublicConfirmed {
		t.Fatalf("web: %+v", w)
	}
	if !strings.Contains(threadText(s), "receptes.example.org") || !strings.Contains(threadText(s), "1") {
		t.Fatalf("el rebut ha d'arribar al fil:\n%s", threadText(s))
	}
	// Actualitzar el mateix nom ja no pregunta.
	raw = windChat(t, s, "Torna-la a publicar")
	if strings.Contains(raw, "event: approve_request") {
		t.Fatalf("una actualització no és una decisió:\n%s", raw)
	}
	waitBackground()
	if w := getWeb("receptes"); w.Current != 2 {
		t.Fatalf("actualització: %+v", w)
	}
}
