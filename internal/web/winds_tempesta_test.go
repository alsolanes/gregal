package web

import (
	"context"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"gregal/internal/config"
	"gregal/internal/llm"
)

// Tempesta: tres agents en paral·lel, cadascun amb el seu angle; la síntesi
// rep els tres esborranys i la resposta del torn és la síntesi. El client veu
// l'estat de cada agent.
func TestTempestaRunsAgentsInParallelAndMerges(t *testing.T) {
	s := remindTestServer(t)
	oldLoop, oldChat := windLoop, petChatter
	t.Cleanup(func() { waitBackground(); windLoop = oldLoop; petChatter = oldChat })

	var mu sync.Mutex
	angles := map[string]bool{}
	running, maxRunning := 0, 0
	release := make(chan struct{})
	windLoop = func(_ *Server, ctx context.Context, brief string, _ ...*routedRole) (exploracio, error) {
		mu.Lock()
		running++
		if running > maxRunning {
			maxRunning = running
		}
		for _, a := range tempestaAngles {
			if strings.Contains(brief, a.brief) {
				angles[a.Key] = true
			}
		}
		n := running
		mu.Unlock()
		if n == tempestaAgentCount {
			close(release) // tots tres alhora: el darrer obre la porta
		}
		<-release
		mu.Lock()
		running--
		mu.Unlock()
		switch {
		case strings.Contains(brief, "ANGLE: be the sceptic"):
			return exploracio{text: "Compte: des del gener el preu és 12 €, no 10 €."}, nil
		case strings.Contains(brief, "ANGLE: evidence first"):
			return exploracio{text: "Segons la web oficial costa 12 € [web](https://ex.cat)."}, nil
		}
		return exploracio{text: "Costa 10 €."}, nil
	}
	var synthInput string
	petChatter = func(_ context.Context, _ *llm.Client, _ *config.Config, _ config.Provider, _ config.Role, msgs []llm.Message) (string, error) {
		synthInput = msgs[len(msgs)-1].Content
		return "Costa **12 €** des del gener [web](https://ex.cat).", nil
	}

	req := httptest.NewRequest("POST", "/api/winds/chat/stream", strings.NewReader(`{"wind":"xaloc","message":"Quant costa l'entrada?","tempesta":true}`))
	rec := httptest.NewRecorder()
	s.handleWindChatStream(rec, req)
	raw := rec.Body.String()

	if maxRunning != tempestaAgentCount || len(angles) != 3 {
		t.Fatalf("agents en paral·lel: %d, angles: %v", maxRunning, angles)
	}
	for _, want := range []string{"DRAFT 1", "DRAFT 2", "DRAFT 3", "12 €", "10 €"} {
		if !strings.Contains(synthInput, want) {
			t.Fatalf("la síntesi no rep %q:\n%s", want, synthInput)
		}
	}
	if !strings.Contains(raw, "event: tempesta") || !strings.Contains(raw, `"state":"done"`) {
		t.Fatalf("el client no veu els agents:\n%s", raw)
	}
	if !strings.Contains(raw, "event: done") || !strings.Contains(raw, "Costa **12 €** des del gener") {
		t.Fatalf("la resposta ha de ser la síntesi:\n%s", raw)
	}
}

// Si la síntesi falla, la resposta és l'esborrany més complet (mai res buit).
func TestTempestaFallsBackToTheBestDraft(t *testing.T) {
	s := remindTestServer(t)
	oldLoop, oldChat := windLoop, petChatter
	t.Cleanup(func() { waitBackground(); windLoop = oldLoop; petChatter = oldChat })
	windLoop = func(_ *Server, _ context.Context, brief string, _ ...*routedRole) (exploracio, error) {
		if strings.Contains(brief, "ANGLE: evidence first") {
			return exploracio{text: "La resposta llarga i amb fonts [web](https://ex.cat), amb tots els detalls."}, nil
		}
		return exploracio{text: "Curta."}, nil
	}
	petChatter = func(context.Context, *llm.Client, *config.Config, config.Provider, config.Role, []llm.Message) (string, error) {
		return "", context.DeadlineExceeded
	}
	req := httptest.NewRequest("POST", "/api/winds/chat/stream", strings.NewReader(`{"wind":"xaloc","message":"i ara?","tempesta":true}`))
	rec := httptest.NewRecorder()
	s.handleWindChatStream(rec, req)
	if raw := rec.Body.String(); !strings.Contains(raw, "La resposta llarga i amb fonts") {
		t.Fatalf("sense síntesi, l'esborrany més complet:\n%s", raw)
	}
}

// «A fons» és per investigar: si el missatge demana fer una cosa, el torn és
// normal, amb les eines d'escriure, i la resposta diu per què.
func TestTempestaWorkRequestRunsAsNormalTurn(t *testing.T) {
	for _, lang := range []string{"ca", "en"} {
		t.Run(lang, func(t *testing.T) {
			t.Setenv("GREGAL_DATA_DIR", t.TempDir())
			s, _ := companionTestServer(t)
			s.cwd = t.TempDir()
			s.mode = "code"
			s.cfg.Language = lang
			provider := windPermissionProvider(t, "write", `{"path":"index.html","content":"<h1>hola</h1>"}`)
			s.cfg.Providers["p"] = config.Provider{BaseURL: provider.URL + "/v1"}
			makeRotllanaTrio(t, s)
			rec := postJSON(t, s.handleWindChatStream, `{"wind":"tramuntana","message":"fes-me una web de la meva cuina","tempesta":true}`)
			raw := rec.Body.String()
			if data, err := os.ReadFile(filepath.Join(s.cwd, "index.html")); err != nil || string(data) != "<h1>hola</h1>" {
				t.Fatalf("el write no s'ha fet: %v %q\n%s", err, data, raw)
			}
			if strings.Contains(raw, "event: tempesta") {
				t.Fatalf("no ha de ser Tempesta:\n%s", raw)
			}
			want := s.tr("«A fons» és per investigar", "«Deep» is for research")
			if !strings.Contains(raw, "event: done") || !strings.Contains(raw, want) {
				t.Fatalf("la resposta ha de dir %q:\n%s", want, raw)
			}
		})
	}
}

// Les consignes dels agents i de la síntesi diuen que «A fons» només
// investiga i que, si cal fer una cosa, ho diguin així, mai que el servidor
// els bloqueja.
func TestTempestaBriefsSayDeepOnlyResearches(t *testing.T) {
	s := remindTestServer(t)
	oldLoop, oldChat := windLoop, petChatter
	t.Cleanup(func() { waitBackground(); windLoop = oldLoop; petChatter = oldChat })
	var mu sync.Mutex
	var briefs []string
	windLoop = func(_ *Server, _ context.Context, brief string, _ ...*routedRole) (exploracio, error) {
		mu.Lock()
		briefs = append(briefs, brief)
		mu.Unlock()
		return exploracio{text: "esborrany"}, nil
	}
	var synthSystem string
	petChatter = func(_ context.Context, _ *llm.Client, _ *config.Config, _ config.Provider, _ config.Role, msgs []llm.Message) (string, error) {
		synthSystem = msgs[0].Content
		return "resposta", nil
	}
	req := httptest.NewRequest("POST", "/api/winds/chat/stream", strings.NewReader(`{"wind":"xaloc","message":"Quin temps farà demà?","tempesta":true}`))
	s.handleWindChatStream(httptest.NewRecorder(), req)
	if len(briefs) != tempestaAgentCount {
		t.Fatalf("agents: %d", len(briefs))
	}
	for i, text := range append(briefs, synthSystem) {
		for _, want := range []string{"Deep mode only researches", "turn Deep off", "never say the server blocked"} {
			if !strings.Contains(text, want) {
				t.Fatalf("la consigna %d no diu %q:\n%s", i, want, text)
			}
		}
	}
}

// «A fons» sap que li demanes fer una cosa encara que no facis servir els
// verbs de la colla («fes», «crea»): «Vull una pàgina web…», «Escriu-me un
// fitxer…». Una pregunta o una recerca segueixen sent «A fons».
func TestTempestaWantsWorkBeyondCrewVerbs(t *testing.T) {
	for _, m := range []string{
		"Vull una pàgina web de la recepta de rius, desada a rius2/index.html",
		"Escriu-me un fitxer amb la llista de la compra",
		"Necessito un document Word amb l'informe trimestral",
		"Genera un script que reanomeni les fotos",
		"I want a landing page for my bakery",
		"Write me a report.md about the meeting",
		"Quiero una página web para mi tienda",
	} {
		if !tempestaWantsWork(m) {
			t.Errorf("hauria de ser feina: %q", m)
		}
	}
	for _, m := range []string{
		"Com funciona una pàgina web per dins?",
		"Vull saber què diu la premsa de la web de l'ajuntament",
		"Compara els millors frameworks web d'aquest any",
		"Investiga quins jocs han sortit aquest mes",
		"No vull que escriguis cap fitxer, només explica'm la idea",
		"What do experts say about remote work?",
	} {
		if tempestaWantsWork(m) {
			t.Errorf("hauria de ser recerca: %q", m)
		}
	}
}
