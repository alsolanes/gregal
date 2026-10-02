package agent

import (
	"strings"
	"testing"

	"gregal/internal/config"
	"gregal/internal/llm"
)

// El topall de cost autònom compta el consum real quan el client en té:
// cada pas torna a enviar tot el prompt, i l'estimació sobre l'historial
// final (un sol cop) es quedava tan curta que el topall no saltava mai.
func TestTopallCostAmbUsageReal(t *testing.T) {
	SetPrices(map[string]Price{"m": {In: 1, Out: 2}}) // $ per milió de tokens
	defer SetPrices(nil)
	cfg := &config.Config{Agent: config.AgentCfg{Autonomous: config.AutonomousCfg{MaxCostUSD: 0.5}}}
	hist := []llm.Message{{Role: "user", Content: "fes-ho"}, {Role: "assistant", Content: "fet"}}

	// Només amb l'estimació, l'historial és petit: no salta.
	if m := TopallAutonom(cfg, EstatAutonom{Hist: hist, Provider: "p", Model: "m"}); m != "" {
		t.Fatalf("amb l'estimació d'un historial petit no hauria de saltar: %q", m)
	}
	// Amb el consum real (400k tokens de prompt acumulats), sí.
	uso := &llm.UsageMeter{}
	uso.Add(llm.Usage{PromptTokens: 400_000, CompletionTokens: 60_000})
	m := TopallAutonom(cfg, EstatAutonom{Hist: hist, Provider: "p", Model: "m", Usage: uso})
	if !strings.Contains(m, "límit de cost") {
		t.Fatalf("amb el consum real ($0.52) ha de saltar: %q", m)
	}
}
