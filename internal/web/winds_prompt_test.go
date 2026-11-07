package web

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"gregal/internal/llm"
)

// La conversa arriba com a torns reals: system, usuari/assistent alternats i
// el missatge nou al final; els avisos seguits del vent s'uneixen i mai no
// es comença per l'assistent.
func TestWindTurnMessagesStructure(t *testing.T) {
	prev := []llm.Message{
		{Role: "assistant", Content: "Bon dia! Com ha anat l'entrevista?"},
		{Role: "assistant", Content: "Recordatori: truca al dentista."},
		{Role: "user", Content: "Bé! I ja he trucat."},
		{Role: "tool", Content: "sortida d'eina"},
		{Role: "assistant", Content: "Genial."},
		{Role: "user", Content: "   "},
	}
	got := windTurnMessages("SYS", prev, "Què faig per sopar?")
	roles := []string{}
	for _, m := range got {
		roles = append(roles, m.Role)
	}
	if strings.Join(roles, ",") != "system,user,assistant,user,assistant,user" {
		t.Fatalf("rols: %v", roles)
	}
	if got[0].Content != "SYS" || got[len(got)-1].Content != "Què faig per sopar?" {
		t.Fatalf("extrems: %+v", got)
	}
	if !strings.Contains(got[2].Content, "entrevista") || !strings.Contains(got[2].Content, "dentista") {
		t.Fatalf("els dos avisos seguits han d'anar junts: %q", got[2].Content)
	}
}

// Un missatge d'usuari sense resposta (el torn anterior va fallar) no fa dos
// torns d'usuari seguits: el nou s'hi enganxa.
func TestWindTurnMessagesPendingUser(t *testing.T) {
	got := windTurnMessages("SYS", []llm.Message{{Role: "user", Content: "hola"}}, "hi ets?")
	if len(got) != 2 || got[1].Role != "user" || !strings.Contains(got[1].Content, "hola") || !strings.HasSuffix(got[1].Content, "hi ets?") {
		t.Fatalf("%+v", got)
	}
}

// Les converses llargues es tallen pels més antics i cada missatge té sostre.
func TestWindChatTurnsBudget(t *testing.T) {
	var prev []llm.Message
	for i := 0; i < 60; i++ {
		role := "user"
		if i%2 == 1 {
			role = "assistant"
		}
		prev = append(prev, llm.Message{Role: role, Content: strings.Repeat("x", 5000)})
	}
	prev = append(prev, llm.Message{Role: "user", Content: "l'últim"})
	got := windChatTurns(prev)
	total := 0
	for _, m := range got {
		total += len(m.Content)
		if len([]rune(m.Content)) > windTurnMaxChars+4 {
			t.Fatalf("missatge massa llarg: %d", len(m.Content))
		}
	}
	if total > windTurnsMaxChars+windTurnMaxChars || got[len(got)-1].Content != "l'últim" || got[0].Role != "user" {
		t.Fatalf("total=%d n=%d", total, len(got))
	}
}

// El xat directe en directe envia la conversa estructurada al loop, amb el
// system del vent (sense el límit antic d'1-4 frases) i la pregunta al final.
func TestWindStreamSendsStructuredTurns(t *testing.T) {
	s := remindTestServer(t)
	h := windHistory(s, "xaloc")
	h.mu.Lock()
	h.messages = []llm.Message{{Role: "user", Content: "Què faig per sopar?"}, {Role: "assistant", Content: "Una truita."}}
	h.mu.Unlock()
	old := windLoopStream
	var msgs []llm.Message
	windLoopStream = func(_ *Server, ctx context.Context, _ string, _ func(kind, text string)) (exploracio, error) {
		msgs = windPromptOf(ctx)
		return exploracio{text: "Cigrons amb espinacs."}, nil
	}
	t.Cleanup(func() { waitBackground(); windLoopStream = old })
	req := httptest.NewRequest("POST", "/api/winds/chat/stream", strings.NewReader(`{"wind":"xaloc","message":"I per dinar demà?"}`))
	rec := httptest.NewRecorder()
	s.handleWindChatStream(rec, req)
	if len(msgs) != 4 || msgs[0].Role != "system" || msgs[1].Content != "Què faig per sopar?" || msgs[2].Role != "assistant" || msgs[3].Content != "I per dinar demà?" {
		t.Fatalf("missatges: %+v %s", msgs, rec.Body.String())
	}
	if !strings.Contains(msgs[0].Content, "HOW YOU ANSWER") || strings.Contains(msgs[0].Content, "1-4 sentences") || strings.Contains(msgs[0].Content, "PREVIOUS CONVERSATION") {
		t.Fatalf("system:\n%s", msgs[0].Content)
	}
}

// La poda no fa res sota el sostre; per sobre, conserva els recents i
// comença per usuari (alguns proveïdors ho exigeixen).
func TestPodaHistorialVentSostre(t *testing.T) {
	mk := func(n int) []llm.Message {
		out := make([]llm.Message, 0, n)
		for i := 0; i < n; i++ {
			role := "user"
			if i%2 == 1 {
				role = "assistant"
			}
			out = append(out, llm.Message{Role: role, Content: "m"})
		}
		return out
	}
	petit := mk(50)
	if got := podaHistorialVent(petit); len(got) != 50 {
		t.Fatalf("sota el sostre no es toca: %d", len(got))
	}
	gran := mk(250)
	got := podaHistorialVent(gran)
	if len(got) == 0 || len(got) > windStoreMaxMsgs {
		t.Fatalf("per sobre del sostre cal podar: %d", len(got))
	}
	if got[0].Role != "user" {
		t.Fatalf("ha de començar per usuari: %s", got[0].Role)
	}
	for i, m := range gran[len(gran)-len(got):] {
		if got[i].Content != m.Content || got[i].Role != m.Role {
			t.Fatalf("es conserven els més recents (pos %d)", i)
		}
	}
}
