package web

import (
	"context"
	"strings"
	"testing"
)

// TestWindChatParcial: si el loop es queda sense pressupost, la resposta
// ho diu («parcial»:true) perquè el client pugui oferir «continua».
func TestWindChatParcial(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	s, _ := companionTestServer(t)
	if rec := postJSON(t, s.handleWinds, `{"name":"Mitja","color":"#FF4444"}`); rec.Code != 200 {
		t.Fatalf("save: %d", rec.Code)
	}
	saved := windLoop
	windLoop = func(_ *Server, _ context.Context, _ string) (exploracio, error) {
		return exploracio{text: "primera meitat…", parcial: true}, nil
	}
	t.Cleanup(func() { windLoop = saved })

	rec := postJSON(t, s.handleWindChat, `{"wind":"mitja","message":"explica-m'ho tot"}`)
	if rec.Code != 200 {
		t.Fatalf("chat: %d %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"parcial":true`) {
		t.Errorf("falta parcial:true: %s", rec.Body.String())
	}
}

// TestWindChatContinueBuit: «continua» sense historial no té què
// reprendre → 400, igual que un missatge buit normal.
func TestWindChatContinueBuit(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	s, _ := companionTestServer(t)
	if rec := postJSON(t, s.handleWinds, `{"name":"Buida","color":"#FF4444"}`); rec.Code != 200 {
		t.Fatalf("save: %d", rec.Code)
	}
	if rec := postJSON(t, s.handleWindChat, `{"wind":"buida","message":"","continue":true}`); rec.Code != 400 {
		t.Errorf("continua sense historial: %d", rec.Code)
	}
}
