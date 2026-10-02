package web

import (
	"strings"
	"testing"

	"gregal/internal/agent"
	"gregal/internal/config"
	"gregal/internal/llm"
)

func c2Server(t *testing.T) *Server {
	t.Helper()
	s := goalTestServer(t)
	s.cfg.Router = config.RouterCfg{Mode: "auto", StrongRole: "fort", EscalateAfter: 2}
	s.cfg.Roles["fort"] = config.Role{Provider: "p", Model: "m", ContextWindow: 32768}
	s.cfg.Budget = config.BudgetCfg{SessionUSD: 1}
	return s
}

func collectEmits() (func(string, any), *[]string) {
	var got []string
	return func(ev string, _ any) { got = append(got, ev) }, &got
}

// Dues CAL REVISAR seguides escalen a fort; APROVAT trenca la ratxa.
func TestTrackVerdictEscala(t *testing.T) {
	s := c2Server(t)
	emit, got := collectEmits()
	s.trackVerdict(emit, "CAL REVISAR: brut", "code")
	if s.role != "code" || len(*got) != 0 {
		t.Fatalf("amb 1 no s'escala (rol=%q ev=%v)", s.role, *got)
	}
	s.trackVerdict(emit, "CAL REVISAR: encara brut", "code")
	if s.role != "fort" {
		t.Fatalf("amb 2 cal escalar (rol=%q)", s.role)
	}
	if len(*got) != 1 || (*got)[0] != "escalate" {
		t.Fatalf("cal event escalate: %v", *got)
	}
}

// APROVAT trenca la ratxa; error del revisor no compta.
func TestTrackVerdictAprovaTrenca(t *testing.T) {
	s := c2Server(t)
	emit, _ := collectEmits()
	s.trackVerdict(emit, "CAL REVISAR: x", "code")
	s.trackVerdict(emit, "APROVAT: net", "code")
	s.trackVerdict(emit, "CAL REVISAR: y", "code")
	if s.role == "fort" {
		t.Fatal("l'APROVAT ha de trencar la ratxa")
	}
	s.trackVerdict(emit, "error", "code")
	if s.streak != 1 {
		t.Fatalf("l'error no compta (streak=%d)", s.streak)
	}
}

// Pin manual: mai s'escala.
func TestTrackVerdictPinGuanya(t *testing.T) {
	s := c2Server(t)
	s.rolePinned = true
	emit, got := collectEmits()
	s.trackVerdict(emit, "CAL REVISAR: x", "code")
	s.trackVerdict(emit, "CAL REVISAR: y", "code")
	if len(*got) != 0 {
		t.Fatalf("amb pin no hi ha escalat: %v", *got)
	}
}

// Pressupost: sense preu no hi ha res; amb preu i límit superat, un avís.
func TestTrackTurnCost(t *testing.T) {
	s := c2Server(t)
	emit, got := collectEmits()
	hist := []llm.Message{{Role: "user", Content: "hola"}, {Role: "assistant", Content: "adeu"}}
	agent.SetPrices(map[string]agent.Price{})
	s.trackTurnCost(emit, hist, "code", s.cfg.Roles["code"])
	if s.sessCost != 0 || len(*got) != 0 {
		t.Fatal("sense preu no es compta res")
	}
	agent.SetPrices(map[string]agent.Price{"m": {In: 1e6, Out: 1e6}})
	s.trackTurnCost(emit, hist, "code", s.cfg.Roles["code"])
	if len(*got) != 1 || (*got)[0] != "budget" {
		t.Fatalf("cal avís budget: %v (cost=%v)", *got, s.sessCost)
	}
	s.trackTurnCost(emit, hist, "code", s.cfg.Roles["code"])
	if len(*got) != 1 {
		t.Fatal("l'avís és una sola vegada per sessió")
	}
	agent.SetPrices(map[string]agent.Price{})
}

// strict: CAL REVISAR tanca la porta; l'aprovació l'obre.
func TestGateStrict(t *testing.T) {
	s := c2Server(t)
	s.cfg.Verify.Mode = "strict"
	emit, got := collectEmits()
	s.trackVerdict(emit, "CAL REVISAR: brut", "code")
	if !s.gated {
		t.Fatal("strict ha de tancar la porta")
	}
	if len(*got) != 1 || (*got)[0] != "gated" {
		t.Fatalf("cal event gated: %v", *got)
	}
	// L'aprovació obre.
	rec := postJSON(t, s.handleVerifyApprove, `{"approve":true}`)
	if rec.Code != 200 {
		t.Fatalf("approve=%d", rec.Code)
	}
	if s.gated {
		t.Fatal("la porta ha de quedar oberta")
	}
	// Sense bloqueig, avisa honestament.
	rec = postJSON(t, s.handleVerifyApprove, `{"approve":true}`)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "cap bloqueig") {
		t.Fatalf("sense gate: %d %q", rec.Code, rec.Body.String())
	}
}

// both (no strict): CAL REVISAR no tanca res.
func TestGateNomésStrict(t *testing.T) {
	s := c2Server(t)
	s.cfg.Verify.Mode = "both"
	emit, _ := collectEmits()
	s.trackVerdict(emit, "CAL REVISAR: brut", "code")
	s.trackVerdict(emit, "CAL REVISAR: brut", "code")
	if s.gated {
		t.Fatal("fora de strict no hi ha porta")
	}
}
