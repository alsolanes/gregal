package web

import (
	"context"
	"strings"
	"testing"

	"gregal/internal/llm"
)

// TestWantsAllVoices: les adreces col·lectives activen el «parlen tots»;
// la resta de missatges no.
func TestWantsAllVoices(t *testing.T) {
	for msg, want := range map[string]bool{
		"M'expliqueu cadascú la situació":            true,
		"que en parlin tots":                        true,
		"un de cada":                                true,
		"Tothom hi està d'acord?":                   false, // «tothom» subjecte, no adreça
		"todos, decid":                              false, // «todos» sol: massa ambigu
		"everyone, what do you think":                true,
		"hola a tothom":                             false, // salutació, no petició
		"què en penseu":                             false,
		"explica'm la situació":                     false,
		"el cadascuníssim no és una paraula":        false,
		"tinc tots els dits adolorits":              false, // «tots els» sol no és adreça
		"hola":                                      false,
		"CADA UN de vosaltres em diu la seva":       true,
	} {
		if got := wantsAllVoices(msg); got != want {
			t.Errorf("wantsAllVoices(%q) = %v, want %v", msg, got, want)
		}
	}
}

// TestRotllanaCollectiveAddressAllMembersSpeak: «M'expliqueu cadascú…»
// fa parlar tots els membres; un missatge normal segueix amb la regla
// justa d'un sol vent.
func TestRotllanaCollectiveAddressAllMembersSpeak(t *testing.T) {
	s, _ := companionTestServer(t)
	id := makeRotllanaTrio(t, s)
	rot, _ := s.rotllanaByID(id)
	tail := []llm.Message(nil)
	// Adreça col·lectiva: tots tres.
	got := s.routeRotllanaSpeakers(context.Background(), rot, "", "M'expliqueu cadascú la situació dels equips", tail)
	if len(got) != 3 {
		t.Fatalf("adreça col·lectiva: esperava 3 vents, got %d", len(got))
	}
	// Missatge normal: un de sol (regla justa).
	got = s.routeRotllanaSpeakers(context.Background(), rot, "", "què en penseu?", tail)
	if len(got) != 1 {
		t.Fatalf("missatge normal: esperava 1 vent, got %d", len(got))
	}
	// Un nomenat guanya a l'adreça col·lectiva: «Garbí, explica'ls-ho tots»
	// és per al Garbí, no per a tothom.
	got = s.routeRotllanaSpeakers(context.Background(), rot, "", "Tramuntana, explica'ls-ho a tots", tail)
	if len(got) != 1 || got[0].ID != "tramuntana" {
		t.Fatalf("nomenat + col·lectiu: got %+v", got)
	}
	_ = strings.TrimSpace
}
