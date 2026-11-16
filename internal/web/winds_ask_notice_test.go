package web

import (
	"context"
	"strings"
	"testing"
	"time"
)

// Un vent que espera el teu permís avisa (novetat «torn», que viatja pel push
// al mòbil i a l'escriptori); quan l'espera s'acaba, l'avís desapareix.
func TestWindAskNotifiesAndForgetsWhenDone(t *testing.T) {
	s := remindTestServer(t)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		s.windAskWait(ctx, nil, &windAskReq{Type: "approval", Name: "bash", What: "npm install"}, `{"command":"npm install"}`)
	}()
	waiting := func() bool {
		for _, n := range loadNovetats(s) {
			if n.Source == "torn" && strings.Contains(n.Text, "npm install") {
				return true
			}
		}
		return false
	}
	deadline := time.Now().Add(3 * time.Second)
	for !waiting() {
		if time.Now().After(deadline) {
			t.Fatal("el vent espera el teu permís i no n'ha avisat")
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	<-done
	if waiting() {
		t.Fatal("l'espera s'ha acabat i l'avís hi continua")
	}
}
