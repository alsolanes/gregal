package agent

import (
	"fmt"
	"time"

	"gregal/internal/config"
	"gregal/internal/llm"
)

// Topalls del mode autònom, en un sol lloc.
//
// El headless els comprovava tots tres (eines, temps, cost) al principi
// de cada pas; el TUI només el de cost. O sigui que una sessió autònoma
// del terminal no respectava ni `max_tool_steps` ni `max_minutes`: els
// dos límits que eviten que una tasca mal plantejada es passi la tarda
// cremant tokens. Ara els dos bucles criden aquesta funció.

// EstatAutonom és el que cal saber per decidir si continuar.
type EstatAutonom struct {
	// Execs són les eines executades de debò al torn (no les repetides
	// ni les bloquejades).
	Execs int
	// Inici és quan ha començat el torn autònom.
	Inici time.Time
	// Hist és l'historial, per estimar el cost acumulat.
	Hist []llm.Message
	// Provider i Model són els del rol que treballa (per al preu).
	Provider string
	Model    string
	// Usage és el comptador de tokens reals del client (opcional).
	Usage *llm.UsageMeter
}

// TopallAutonom retorna el motiu pel qual cal aturar-se, o "" per
// continuar. El text ja és llegible per a la persona i prou curt per a
// una línia d'estat.
func TopallAutonom(cfg *config.Config, e EstatAutonom) string {
	if cfg == nil {
		return ""
	}
	a := cfg.AutonomousConfig()
	if a.MaxToolSteps > 0 && e.Execs >= a.MaxToolSteps {
		return fmt.Sprintf("límit d'eines (%d)", a.MaxToolSteps)
	}
	if a.MaxMinutes > 0 && !e.Inici.IsZero() && time.Since(e.Inici) >= time.Duration(a.MaxMinutes)*time.Minute {
		return fmt.Sprintf("límit de temps (%d min)", a.MaxMinutes)
	}
	if a.MaxCostUSD > 0 {
		if cost, known := e.cost(); known && cost >= a.MaxCostUSD {
			return fmt.Sprintf("límit de cost (%s)", FmtCost(cost))
		}
	}
	return ""
}

// cost és el que ha costat el torn fins ara. Amb el comptador del client
// i usage de l'API, el real: cada pas torna a enviar tot el prompt, i
// l'estimació sobre l'historial final (un sol cop) es quedava un o dos
// ordres de magnitud curta, de manera que el topall de cost no saltava
// mai a temps. Sense usage, l'estimació de sempre.
func (e EstatAutonom) cost() (float64, bool) {
	if e.Usage != nil {
		if u, _, amb := e.Usage.Total(); amb > 0 {
			return CostUSD(e.Provider, e.Model, u.PromptTokens, u.CompletionTokens)
		}
	}
	return AutonomousCost(e.Hist, e.Provider, e.Model)
}
