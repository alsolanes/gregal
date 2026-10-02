package agent

import (
	"fmt"
	"strings"
	"sync"

	"gregal/internal/config"
	"gregal/internal/llm"
)

// Price és USD per milió de tokens (in = input, out = output).
type Price struct {
	In, Out float64
}

var (
	costMu sync.RWMutex
	prices = map[string]Price{}
)

// SetPrices desa la taula de preus (clau: nom de model, o "provider/model").
// Mapa buit = sense preus = no es mostra cap cost (mai s'inventa).
func SetPrices(p map[string]Price) {
	costMu.Lock()
	defer costMu.Unlock()
	prices = map[string]Price{}
	for k, v := range p {
		prices[strings.ToLower(strings.TrimSpace(k))] = v
	}
}

// SetupPrices carrega la taula del config (agent.SetupPrices a l'arrencada).
func SetupPrices(cfg *config.Config) {
	p := map[string]Price{}
	for k, v := range cfg.Cost {
		p[k] = Price{In: v.In, Out: v.Out}
	}
	SetPrices(p)
}

// CostUSD calcula el cost d'una sessió. ok=false si el model no té preu
// (locals i desconeguts: no mostrar res abans que un número inventat).
func CostUSD(provider, model string, up, down int) (usd float64, ok bool) {
	costMu.RLock()
	defer costMu.RUnlock()
	pr, ok := prices[strings.ToLower(provider+"/"+model)]
	if !ok {
		pr, ok = prices[strings.ToLower(model)]
	}
	if !ok {
		return 0, false
	}
	return float64(up)/1e6*pr.In + float64(down)/1e6*pr.Out, true
}

// FmtCost pinta $0.0042 / $0.023 / $1.20 (mai notació científica).
func FmtCost(usd float64) string {
	switch {
	case usd < 0.01:
		return fmt.Sprintf("$%.4f", usd)
	case usd < 10:
		return fmt.Sprintf("$%.3f", usd)
	default:
		return fmt.Sprintf("$%.2f", usd)
	}
}

// HistCost compta tokens d'un historial i el seu cost si el model té preu.
// És el mateix càlcul que el tancament de run.go (up = no-assistant).
func HistCost(hist []llm.Message, provider, model string) (up, down int, usd float64, ok bool) {
	for _, m := range hist {
		n := llm.EstimateTokens([]llm.Message{{Role: m.Role, Content: m.Content, ToolCalls: m.ToolCalls}})
		if m.Role == "assistant" {
			down += n
		} else {
			up += n
		}
	}
	usd, ok = CostUSD(provider, model, up, down)
	return up, down, usd, ok
}
