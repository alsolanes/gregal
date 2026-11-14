package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

// TestReplyAwayRingsAndOpensTheMessage: una resposta que acaba amb el client
// ja desconnectat (app tancada) queda com a novetat que sona, fins i tot en
// mode tranquil, apunta al missatge exacte i no fa cap targeta d'«Avui». Amb
// el client connectat no es fa res (ja avisa ell).
func TestReplyAwayRingsAndOpensTheMessage(t *testing.T) {
	rig := sharedTestRig(t)
	s := rig.aleix
	f := &fakePush{done: make(chan struct{}, 4)}
	withFakePush(t, f)
	if rec := postJSON(t, s.handlePush, `{"token":"tok-aleix","platform":"android"}`); rec.Code != 200 {
		t.Fatalf("registre: %d", rec.Code)
	}
	// Mode tranquil: les veus proactives no sonarien.
	if err := os.WriteFile(avuiSettingsPath(s), []byte(`{"level":"calm"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	cardsBefore := len(loadAvui(s))

	live := httptest.NewRequest(http.MethodPost, "/api/winds/chat/stream", nil)
	s.noteReplyIfAway(live, rig.mestral, replySource, "m-live", "Resposta vista")
	if n := countNovetats(s, "m-live"); n != 0 {
		t.Fatalf("amb el client connectat hi ha %d novetats", n)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	gone := httptest.NewRequest(http.MethodPost, "/api/winds/rotllana/stream", nil).WithContext(ctx)
	s.noteReplyIfAway(gone, rig.mestral, replySource+":r-1", "m-gone", "La resposta que no has vist")
	var got windNovetat
	for _, n := range loadNovetats(s) {
		if n.MessageID == "m-gone" {
			got = n
		}
	}
	if got.ID == "" || got.Quiet || got.Card != "" || got.Source != "resposta:r-1" || got.Wind != rig.mestral.ID {
		t.Fatalf("novetat = %+v", got)
	}
	if len(loadAvui(s)) != cardsBefore {
		t.Fatal("una resposta no ha de fer cap targeta d'«Avui»")
	}
	select {
	case <-f.done:
	case <-time.After(5 * time.Second):
		t.Fatal("cap push per a la resposta no vista")
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.sent) != 1 || f.sent[0]["id"] != got.ID {
		t.Fatalf("push = %v", f.sent)
	}
}

func countNovetats(s *Server, messageID string) int {
	n := 0
	for _, x := range loadNovetats(s) {
		if x.MessageID == messageID {
			n++
		}
	}
	return n
}
