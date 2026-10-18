package agent

import (
	"encoding/json"
	"path/filepath"
	"strings"

	"gregal/internal/config"
	"gregal/internal/tools"
)

// Regles de permís amb glob (config.PermRules) i permisos per subagent.
//
// Què es compara amb el patró:
//   - bash i bash_background: la comanda, etapa per etapa (`&&`, `||`, `;`,
//     `|`, salts de línia). Una comanda amb substitució (`$(…)`, cometes
//     inverses) o redirecció a fitxer no pot quedar permesa per un patró:
//     `git *` no ha de deixar passar `git log > ~/.bashrc`;
//   - eines amb camí (write, edit, read, lsp…): el camí, relatiu al projecte
//     i amb «/»;
//   - web_fetch: l'adreça; la resta: només el patró `*` hi coincideix.
//
// Dins d'una eina mana l'última regla que coincideix. Una regla «allow» mai
// no relaxa un bloqueig dur (sudo, rm -rf /, camins crítics: el classificador
// de shell es mira abans) ni les eines que un mode de només lectura
// denega; sí que pot canviar un «allow» per defecte en «ask» o «deny».
// Al mode autònom, «ask» passa com sempre i «deny» continua denegant.

// PolicyFromConfig és la política de permisos del config per a projectDir.
func PolicyFromConfig(cfg *config.Config, projectDir string) *Policy {
	p := &Policy{ProjectDir: projectDir}
	if cfg == nil {
		return p
	}
	p.Tools = cfg.Permissions.Tools
	p.BashAllow = cfg.Permissions.BashAllow
	p.BashDeny = cfg.Permissions.BashDeny
	p.Rules = cfg.Permissions.Rules
	return p
}

// subagentPolicy és la política d'un subagent: la general amb el que
// permissions.subagents.<nom> hi canvia. Una decisió de tools.<eina> del
// subagent substitueix també les regles generals d'aquella eina; les regles
// del subagent van després de les generals (i per tant manen).
func subagentPolicy(permissions config.PermissionsCfg, name string) *Policy {
	p := &Policy{Tools: permissions.Tools, BashAllow: permissions.BashAllow, BashDeny: permissions.BashDeny, Rules: permissions.Rules}
	o, ok := permissions.Subagents[name]
	if !ok || name == "" {
		return p
	}
	p.Tools = make(map[string]string, len(permissions.Tools)+len(o.Tools))
	for k, v := range permissions.Tools {
		p.Tools[k] = v
	}
	p.Rules = make(config.PermRules, len(permissions.Rules)+len(o.Rules))
	for k, v := range permissions.Rules {
		p.Rules[k] = append([]config.PermRule(nil), v...)
	}
	for k, v := range o.Tools {
		p.Tools[k] = v
		delete(p.Rules, k)
		if k == "bash" || k == "bash_background" {
			delete(p.Rules, "bash")
			delete(p.Rules, "bash_background")
		}
	}
	for k, v := range o.Rules {
		p.Rules[k] = append(p.Rules[k], v...)
	}
	return p
}

// globMatch: `*` (i `**`) coincideix amb qualsevol cosa, també «/»; `?`
// amb un caràcter. La resta és literal.
func globMatch(pattern, s string) bool {
	p, t := []rune(pattern), []rune(s)
	pi, ti, star, mark := 0, 0, -1, 0
	for ti < len(t) {
		switch {
		case pi < len(p) && p[pi] == '*':
			star, mark = pi, ti
			pi++
		case pi < len(p) && (p[pi] == '?' || p[pi] == t[ti]):
			pi++
			ti++
		case star >= 0:
			pi = star + 1
			mark++
			ti = mark
		default:
			return false
		}
	}
	for pi < len(p) && p[pi] == '*' {
		pi++
	}
	return pi == len(p)
}

// ruleTarget és el text de la crida que es compara amb els patrons.
func (p *Policy) ruleTarget(name, argsJSON string) string {
	var a struct {
		Path string `json:"path"`
		URL  string `json:"url"`
	}
	_ = json.Unmarshal([]byte(argsJSON), &a)
	switch {
	case name == "web_fetch":
		return strings.TrimSpace(a.URL)
	case strings.TrimSpace(a.Path) != "":
		return p.relPath(strings.TrimSpace(a.Path))
	}
	return ""
}

func (p *Policy) relPath(path string) string {
	if p != nil && p.ProjectDir != "" && filepath.IsAbs(path) {
		if rel, err := filepath.Rel(p.ProjectDir, path); err == nil && !strings.HasPrefix(rel, "..") {
			path = rel
		}
	}
	return strings.TrimPrefix(filepath.ToSlash(path), "./")
}

func (p *Policy) rulesFor(name string) []config.PermRule {
	if p == nil || len(p.Rules) == 0 {
		return nil
	}
	if name == "bash" || name == "bash_background" {
		return append(append([]config.PermRule(nil), p.Rules["bash"]...), p.Rules["bash_background"]...)
	}
	return p.Rules[name]
}

// lastMatch torna la decisió de l'última regla que coincideix amb target.
func lastMatch(rules []config.PermRule, target string, paths bool) (string, string, bool) {
	d, pat, ok := "", "", false
	for _, r := range rules {
		if globMatch(r.Pattern, target) || (paths && !strings.Contains(r.Pattern, "/") && globMatch(r.Pattern, filepath.Base(target))) {
			d, pat, ok = r.Decision, r.Pattern, true
		}
	}
	return d, pat, ok
}

// ruleDecision aplica les regles amb glob a una crida. matched=false vol dir
// que cap regla no se n'ocupa i toca la decisió de sempre.
func (p *Policy) ruleDecision(name, argsJSON string) (decision, reason string, matched bool) {
	rules := p.rulesFor(name)
	if len(rules) == 0 {
		return "", "", false
	}
	if name == "bash" || name == "bash_background" {
		var a struct {
			Command string `json:"command"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &a); err != nil {
			return "", "", false
		}
		return bashRuleDecision(rules, a.Command)
	}
	target := p.ruleTarget(name, argsJSON)
	d, pat, ok := lastMatch(rules, target, name != "web_fetch" && target != "")
	if !ok {
		return "", "", false
	}
	return d, "permissions rule " + name + " \"" + pat + "\": " + d, true
}

// bashRuleDecision combina les etapes d'una comanda: el més restrictiu
// guanya (deny > ask > allow), i «allow» només si totes les etapes tenen
// una regla que ho permet i la comanda no amaga res en una substitució o
// una redirecció.
func bashRuleDecision(rules []config.PermRule, command string) (string, string, bool) {
	command = strings.TrimSpace(command)
	stages := tools.ShellStages(command)
	opaque := strings.ContainsAny(command, "`") || strings.Contains(command, "$(") || strings.Contains(command, "<(") || tools.HasFileRedirect(command)
	worst, worstPat := "", ""
	rank := map[string]int{"allow": 1, "ask": 2, "deny": 3}
	unmatched := false
	for _, st := range stages {
		d, pat, ok := lastMatch(rules, st, false)
		if !ok {
			unmatched = true
			continue
		}
		if rank[d] > rank[worst] {
			worst, worstPat = d, pat
		}
	}
	// El patró també es mira contra la comanda sencera: una regla «deny»
	// o «ask» sobre `git push *` ha de saltar encara que s'amagui en una
	// substitució.
	if d, pat, ok := lastMatch(rules, command, false); ok && rank[d] > rank[worst] {
		worst, worstPat = d, pat
	}
	switch {
	case worst == "":
		return "", "", false
	case worst == "allow" && (unmatched || opaque):
		return "", "", false
	}
	return worst, "permissions rule bash \"" + worstPat + "\": " + worst, true
}
