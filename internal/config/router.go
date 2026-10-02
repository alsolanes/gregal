package config

import (
	"strings"
)

// Router: tria de rol per tasca (cost/latència sense perdre qualitat).
// Regles heurístiques, zero crides al model: instantani i gratuït.
// L'override manual (rol fixat per l'usuari a la sessió) sempre guanya;
// el router només decideix quan el rol no està fixat.
type RouterCfg struct {
	Mode       string `yaml:"mode"`        // auto (defecte) | off
	StrongRole string `yaml:"strong_role"` // rol per a feines pesades ("" = code)
	// EscalateAfter: si el revisor diu CAL REVISAR N cops seguits, el pròxim
	// torn canvia sol al StrongRole (0 = apagat). El rol fixat manualment
	// sempre guanya (mai s'escala per sobre del pin).
	EscalateAfter int `yaml:"escalate_after"`
}

// BudgetCfg: pressupost per sessió web (0 = apagat). En superar-lo s'emet
// un avís (una sola vegada per sessió); mai no es bloqueja feina.
type BudgetCfg struct {
	SessionUSD float64 `yaml:"session_usd"`
}

// EscalateTarget diu si cal escalar (streak de CAL REVISAR) i a quin rol.
// Torna ("", false) si no toca: apagat, pin manual, ratxa curta, ja hi som,
// o rol objectiu inexistent.
func (c *Config) EscalateTarget(streak int, pinned bool, current string) (string, bool) {
	if c.Router.EscalateAfter <= 0 || pinned || streak < c.Router.EscalateAfter {
		return "", false
	}
	target := c.Router.StrongRole
	if target == "" {
		target = "code"
	}
	if target == current {
		return "", false
	}
	if _, ok := c.Roles[target]; !ok {
		return "", false
	}
	return target, true
}

// routeQuestion: la tasca sembla una pregunta (resposta curta, sense edits).
func routeQuestion(task string) bool {
	t := strings.ToLower(strings.TrimSpace(task))
	if len([]rune(t)) > 300 {
		return false
	}
	for _, verb := range []string{"refactor", "migra", "reescriu", "reescribe", "implementa", "afegeix codi", "arregla", "crea el fitxer", "canvia el fitxer", "fes un", "fes el"} {
		if strings.Contains(t, verb) {
			return false
		}
	}
	if strings.HasSuffix(t, "?") {
		return true
	}
	for _, q := range []string{"què ", "que ", "com ", "on ", "per què", "perque ", "explica", "resumeix", "què fa", "busca ", "troba ", "quin ", "quins ", "quina ", "quant ", "quants ", "quantes ", "compta ", "llista "} {
		if strings.HasPrefix(t, q) {
			return true
		}
	}
	return false
}

// routeHeavy: feina pesada (molts fitxers, canvis amplis, text llarg).
func routeHeavy(task string) bool {
	t := strings.ToLower(task)
	if len([]rune(t)) > 500 {
		return true
	}
	for _, w := range []string{"refactor", "migra", "reescriu", "tots els", "tot el projecte", "tota la", "multi-fitxer"} {
		if strings.Contains(t, w) {
			return true
		}
	}
	return false
}

// Route tria el rol per a una tasca d'agent. Torna (rol, motiu).
// current és el rol de la sessió; es respecta si el router està off,
// si la tasca no casa amb cap regla, o si cal visió i no hi ha rol vision.
func (c *Config) Route(task string, hasImages bool, current string) (string, string) {
	if c.Router.Mode == "off" {
		return current, ""
	}
	if hasImages {
		if _, ok := c.Roles["vision"]; ok {
			return "vision", "router: amb imatges → rol vision"
		}
		return current, ""
	}
	if current == "chat" || routeQuestion(task) {
		if _, ok := c.Roles["chat"]; ok && current != "chat" {
			return "chat", "router: sembla una pregunta → rol chat (barat)"
		}
		return current, ""
	}
	if routeHeavy(task) {
		if s := c.Router.StrongRole; s != "" {
			if _, ok := c.Roles[s]; ok && s != current {
				return s, "router: feina pesada → rol " + s
			}
		}
		if current != "code" {
			if _, ok := c.Roles["code"]; ok {
				return "code", "router: feina pesada → rol code"
			}
		}
	}
	return current, ""
}
