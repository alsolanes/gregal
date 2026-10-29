package agent

import (
	"encoding/json"

	"gregal/internal/llm"
	"gregal/internal/tools"
)

// browserGate aplica el permís per lloc del navegador (tools.BrowserGate) a
// una crida del torn: el primer cop a cada lloc es pregunta; les accions
// irreversibles i el Chrome de l'usuari es pregunten encara que hi hagi
// aprovació automàtica. El que el mode ja ha denegat no arriba aquí.
func (t *Torn) browserGate(c llm.ToolCall, pl *CallPlan) {
	var args map[string]any
	if json.Unmarshal([]byte(c.Function.Arguments), &args) != nil || args == nil {
		return
	}
	d, reason, force := tools.BrowserGate(t.o.Sessio, args, t.autonom())
	switch d {
	case "deny":
		pl.Fixed, pl.Reason = "EINA BLOQUEJADA: "+reason, reason
	case "ask":
		pl.Ask, pl.Reason = true, reason
		if force {
			pl.ForceAsk = true
			if t.forcats == nil {
				t.forcats = map[string]bool{}
			}
			t.forcats[c.ID] = true
		}
	default:
		// Lloc ja permès: el permís per acció de la política no cal, tret
		// que el config demani preguntar sempre pel navegador.
		if t.o.Pol == nil || (t.o.Pol.Tools["browser"] != "ask" && len(t.o.Pol.rulesFor("browser")) == 0) {
			pl.Ask = false
		}
	}
}

// Motiu és per què la crida pendent demana permís (per a la targeta).
func (t *Torn) Motiu(id string) string { return t.motius[id] }

// Forcada diu si la crida s'ha de confirmar sí o sí (no es pot recordar ni
// aprovar automàticament).
func (t *Torn) Forcada(id string) bool { return t.forcats[id] }
