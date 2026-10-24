package web

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
)

// El resum mecànic d'un torn aturat mai no és una resposta del vent: ni al
// xat directe (frase curta amb el motiu amable) ni a la rotllana (fallback
// al xat pla). Bloqueja la regressió del «Resposta parcial…» a la bombolla.
func TestEsRespostaMecanica(t *testing.T) {
	casos := map[string]bool{
		"Resposta parcial: el torn s'ha aturat abans de la resposta final (DNS).": true,
		"Partial answer: the turn stopped before the final answer.":              true,
		"Resposta parcial: el torn s'ha aturat":                                  true,
		"  Resposta parcial: el torn s'ha aturat  ":                              true,
		"Que t'ho preparo amb calma?": false,
		"": false,
	}
	for text, want := range casos {
		if got := esRespostaMecanica(text); got != want {
			t.Errorf("esRespostaMecanica(%q) = %v, want %v", text, got, want)
		}
	}
}

func TestFraseTornAturat(t *testing.T) {
	ca := fraseTornAturat("provider https://api.example.com: dial tcp: lookup api.example.com: no such host (host not found: check the provider base_url)", "ca")
	if strings.Contains(ca, "example.com") || strings.Contains(ca, "dial tcp") || strings.Contains(ca, "Resposta parcial") {
		t.Fatalf("la frase CA filtra detalls internals: %q", ca)
	}
	if !strings.Contains(ca, "prove\u00efdor de models") {
		t.Errorf("la frase CA hauria de portar el motiu amable: %q", ca)
	}
	en := fraseTornAturat("connection refused", "en")
	if !strings.Contains(en, "try again") || !strings.Contains(en, "provider") {
		t.Errorf("frase EN inesperada: %q", en)
	}
	// «connection refused» es tradueix a motiu amable en anglès net, sense
	// text cru de Go ni adreces.
	if strings.Contains(en, "dial tcp") || strings.Contains(en, "http://") {
		t.Errorf("la frase EN filtra detalls interns: %q", en)
	}
}

// El xat directe d'un vent amb el torn mort a mitges ha de tornar la frase
// curta, mai el resum mecànic (comprovació de la resposta del servei).
func TestWindChatStreamTornAturatFraseCurta(t *testing.T) {
	s := remindTestServer(t)
	old := windLoopStream
	defer func() { waitBackground(); windLoopStream = old }()
	hist0 := windHistory(s, "xaloc")
	hist0.mu.Lock()
	hist0.messages = nil
	hist0.mu.Unlock()
	windLoopStream = func(s2 *Server, ctx context.Context, brief string, emit func(kind, text string)) (exploracio, error) {
		return exploracio{
			text:    "Resposta parcial: el torn s'ha aturat abans de la resposta final (no s'ha pogut arribar al prove\u00efdor de models (DNS); comprova la connexió a internet).",
			parcial: true,
			motiu:   "provider https://api.example.com: dial tcp: no such host",
		}, nil
	}
	req := httptest.NewRequest("POST", "/api/winds/chat/stream", strings.NewReader(`{"wind":"xaloc","message":"hola"}`))
	rec := httptest.NewRecorder()
	s.handleWindChatStream(rec, req)
	raw := rec.Body.String()
	if strings.Contains(raw, "Resposta parcial") {
		t.Fatalf("el resum mecanic ha arribat al xat:\n%s", raw)
	}
	// El servidor de test corre en anglès: la frase curta ha de sortir en
	// la llengua del config, mai el resum mecànic.
	if !strings.Contains(raw, "try again") {
		t.Errorf("falta la frase curta del vent:\n%s", raw)
	}
	if strings.Contains(raw, "example.com") {
		t.Errorf("la URL del proveidor ha filtrat a la conversa:\n%s", raw)
	}
	if !strings.Contains(raw, `"parcial":true`) {
		t.Errorf("el done ha de marcar la resposta com a parcial perquè l'oferiuca \u00abContinua\u00bb segueixi existint:\n%s", raw)
	}
}
