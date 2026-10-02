package telegram

import (
	"context"
	"strings"
	"testing"
	"time"

	"gregal/internal/runs"
)

// TestAraTallaCuaITornEnMarxa: CancelSession (el cor de /ara) talla el
// torn en marxa i tots els pendents del xat, i cap queda viu.
func TestAraTallaCuaITornEnMarxa(t *testing.T) {
	b := &Bot{queue: runs.New()}
	sess := tgScope(42)
	started := make(chan struct{})
	release := make(chan struct{})
	block := func(ctx context.Context, _ runs.Run) error {
		closeOnce(started)
		select {
		case <-release:
			return nil
		case <-ctx.Done():
			return runs.ErrCancelled
		}
	}
	// Un en marxa i dos pendents.
	first, _, _ := b.queue.Submit(block, runs.Run{Session: sess, Workspace: runs.WorkspaceKey("p")})
	second, _, _ := b.queue.Submit(block, runs.Run{Session: sess, Workspace: runs.WorkspaceKey("p")})
	third, _, _ := b.queue.Submit(block, runs.Run{Session: sess, Workspace: runs.WorkspaceKey("p")})
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("el primer torn no ha començat")
	}

	if n := b.queue.CancelSession(sess); n != 3 {
		t.Fatalf("CancelSession hauria de tocar 3, ha tocat %d", n)
	}
	close(release)
	for _, id := range []int64{first.ID, second.ID, third.ID} {
		if got := waitTelegramRun(t, b.queue, id); got.State != runs.Cancelled {
			t.Fatalf("torn %d estat: %s", id, got.State)
		}
	}
	// Cap torn queda viu a la sessió.
	for _, r := range b.queue.ListSession(sess) {
		if r.State.Active() {
			t.Fatalf("cap torn hauria de seguir viu: %+v", r)
		}
	}
}

// TestAraAmbTascaNovaSExecuta: amb el xat lliure, /ara és una tasca nova
// més; sense text i sense feina, avisa.
func TestAraAmbTascaNovaSExecuta(t *testing.T) {
	tg := newFakeTG(t)
	llm := newFakeLLM(t, replyText("feta la nova"))
	bot, _, _ := testBot(t, tg, llm, nil)

	// Cas 1: res en marxa → /ara es comporta com una tasca nova, sense dir que talla.
	bot.handleMessage(context.Background(), msg(1234567, "/ara fes la cosa nova"))
	m, ok := tg.waitFor("feta la nova", 4*time.Second)
	if !ok {
		t.Fatalf("la tasca nova no s'ha executat: %+v", tg.all())
	}
	if strings.Contains(m.Text, "Talla") {
		t.Errorf("sense feina anterior no hauria de dir que talla: %q", m.Text)
	}

	// Cas 2: /ara sense text i sense feina → avís. El torn anterior ha
	// d'haver acabat del tot (turn és bloquejant amb el xat lliure, així
	// que quan handleMessage torna, la cua ja és buida).
	bot.handleMessage(context.Background(), msg(1234567, "/ara"))
	if _, ok := tg.find("No hi ha res en marxa"); !ok {
		t.Fatalf("sense text i sense feina hauria d'avisar: %+v", tg.all())
	}
}
