package agent

// Subagents en paral·lel: quants en poden córrer alhora, com s'espera un
// lloc sense quedar invisible i com es parteix la investigació d'inici de
// torn en àrees independents (un scout per àrea, tots alhora).
//
// Els límits: MaxDelegateTasks per torn (o agent.parallel_subagents),
// MaxDelegateLiveTotal per procés, i 1 quan els subagents van a un
// servidor local: Ollama, llama.cpp o LM Studio tenen sovint un sol slot, i
// quatre fills alhora només hi farien cua (i cremarien el temps d'espera
// de cadascun).

import (
	"context"
	"regexp"
	"strings"
	"sync/atomic"

	"gregal/internal/config"
	"gregal/internal/llm"
)

// MaxScouts topa els investigadors paral·lels d'inici de torn.
const MaxScouts = 3

// delegateParallelDefault és el sostre per crida quan no hi ha semàfor de
// torn (TUI, headless, Telegram): el fixa SetupDelegate. 0 = MaxDelegateTasks.
var delegateParallelDefault atomic.Int32

// ParallelSubagents diu quants subagents poden córrer alhora en un torn.
func ParallelSubagents(cfg *config.Config) int {
	if cfg != nil && cfg.Agent.ParallelSubagents > 0 {
		if cfg.Agent.ParallelSubagents > MaxDelegateLiveTotal {
			return MaxDelegateLiveTotal
		}
		return cfg.Agent.ParallelSubagents
	}
	if subagentsLocal(cfg) {
		return 1
	}
	return MaxDelegateTasks
}

// subagentsLocal diu si els subagents (el rol delegate, o code, i el rol
// petit que fa servir el scout) van a un servidor local.
func subagentsLocal(cfg *config.Config) bool {
	if cfg == nil {
		return false
	}
	names := []string{"code"}
	if _, ok := cfg.Roles["delegate"]; ok {
		names = []string{"delegate"}
	}
	if name, _, _, ok := SmallRole(cfg); ok {
		names = append(names, name)
	}
	for _, name := range names {
		r, ok := cfg.Roles[name]
		if !ok {
			continue
		}
		p, ok := cfg.Providers[r.Provider]
		if !ok {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(p.Type)) {
		case "ollama", "llamacpp":
			return true
		}
		if llm.IsLocalBase(p.BaseURL) {
			return true
		}
	}
	return false
}

// acquireSlot pren un lloc de sem (nil = sense sostre). Si li toca esperar,
// abans ho diu amb un event «queued»: un fill que espera no ha de semblar
// que treballa ni desaparèixer. Torna false si el context s'acaba abans.
func acquireSlot(ctx context.Context, sem chan struct{}, maxSteps int) bool {
	if tryAcquire(sem) {
		return true
	}
	emitDelegateEvent(ctx, DelegateEvent{Status: DelegateQueued, Message: queuedMessage, MaxSteps: maxSteps})
	return waitSlot(ctx, sem)
}

const queuedMessage = "queued: waiting for a free subagent slot"

// tryAcquire pren un lloc sense esperar (nil = sempre n'hi ha).
func tryAcquire(sem chan struct{}) bool {
	if sem == nil {
		return true
	}
	select {
	case sem <- struct{}{}:
		return true
	default:
		return false
	}
}

// waitSlot espera un lloc; false si el context s'acaba abans.
func waitSlot(ctx context.Context, sem chan struct{}) bool {
	if sem == nil {
		return true
	}
	select {
	case sem <- struct{}{}:
		return true
	case <-ctx.Done():
		return false
	}
}

// Marques d'una llista: «(1)», «1.», «1)», «a)», «-», «*», «•» a l'inici
// d'una línia o, les numerades entre parèntesis, també dins d'una frase.
var (
	focusInline = regexp.MustCompile(`\(\s*(\d{1,2}|[a-h])\s*\)\s*`)
	focusLine   = regexp.MustCompile(`(?m)^\s*(?:\d{1,2}[.)]|[a-h]\)|[-*•])\s+`)
	// Un camí o fitxer del projecte: «internal/team», «web/app/x.js», «main.go».
	focusTail = regexp.MustCompile(`\.\s+\p{Lu}|\n`)
	// Talls de frase per buscar-hi enumeracions, i la conjunció final.
	focusClause   = regexp.MustCompile(`[()\[\]:;?!\n]|\.\s`)
	focusResearch = regexp.MustCompile(`(?i)\b(how|where|why|what|which|review|compare|explain|investigate|audit|analy[sz]e|map|explore|com|on|quin|quina|quins|revisa|compara|explica|investiga|analitza|explora|como|donde)\b|per què`)
	focusConj     = regexp.MustCompile(`,?\s+(?:and|or|i|o|y|et)\s+`)
	focusURL      = regexp.MustCompile(`\w+://\S+`)
	focusPath     = regexp.MustCompile(`(?:[\w.-]+/)+[\w.-]+|\b[\w-]+\.(?:go|py|js|ts|tsx|jsx|rs|java|kt|cs|cpp|c|h|rb|php|css|html|sql|sh|md)\b`)
)

// ScoutFocuses parteix una tasca en àrees independents que es poden
// investigar alhora: una llista explícita («(1) … (2) …», línies
// numerades o amb guions) o, si no n'hi ha, camins de carpetes diferents.
// Menys de dues àrees = nil (un sol scout, com sempre). Mai més de
// MaxScouts: la resta s'ajunta a l'última.
func ScoutFocuses(task string) []string {
	t := strings.TrimSpace(task)
	if t == "" {
		return nil
	}
	var parts []string
	for _, re := range []*regexp.Regexp{focusInline, focusLine} {
		idx := re.FindAllStringIndex(t, -1)
		if len(idx) < 2 {
			continue
		}
		for i, m := range idx {
			end := len(t)
			if i+1 < len(idx) {
				end = idx[i+1][0]
			} else if cut := focusTail.FindStringIndex(t[m[1]:]); cut != nil {
				// L'últim element acaba on acaba la seva frase o línia: el
				// que ve després («Un paràgraf per àrea.») és per a tots.
				end = m[1] + cut[0]
			}
			if p := cleanFocus(t[m[1]:end]); p != "" {
				parts = append(parts, p)
			}
		}
		if len(parts) >= 2 {
			break
		}
		parts = nil
	}
	if len(parts) < 2 {
		parts = pathFocuses(t)
	}
	// Una enumeració dins d'una frase només compta en una pregunta o una
	// revisió: «arregla el bug, passa els tests i fes commit» són passos
	// d'una feina, no àrees per investigar.
	if len(parts) < 2 && focusResearch.MatchString(t) {
		parts = listFocuses(t)
	}
	if len(parts) < 2 {
		return nil
	}
	if len(parts) > MaxScouts {
		rest := strings.Join(parts[MaxScouts-1:], "; ")
		parts = append(parts[:MaxScouts-1:MaxScouts-1], rest)
	}
	return parts
}

// cleanFocus treu la puntuació que enllaça els elements d'una llista
// («…, i (2)») i retalla el que és massa llarg per a una etiqueta.
func cleanFocus(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimRight(s, " ,;:")
	for _, w := range []string{" and", " i", " y", " et"} {
		s = strings.TrimSuffix(s, w)
	}
	s = strings.TrimSpace(strings.TrimRight(s, " ,;:"))
	if r := []rune(s); len(r) > 300 {
		s = string(r[:300]) + "…"
	}
	return s
}

// listFocuses troba una enumeració curta dins d'una frase: «the TUI, the
// web server and the Telegram bot», «el parser, el lexer i la impressora».
// Tres elements com a mínim (dos és massa comú: «llegeix i resumeix») i
// cap de més de sis paraules, perquè una frase amb comes no ho sembli.
func listFocuses(t string) []string {
	for _, seg := range focusClause.Split(t, -1) {
		locs := focusConj.FindAllStringIndex(seg, -1)
		if len(locs) == 0 {
			continue
		}
		last := locs[len(locs)-1]
		items := strings.Split(seg[:last[0]], ",")
		tail := strings.TrimSpace(strings.TrimRight(seg[last[1]:], ". "))
		if len(items) < 2 || tail == "" || len(strings.Fields(tail)) > 6 {
			continue
		}
		var out []string
		for i, it := range items {
			it = strings.TrimSpace(it)
			// El primer element porta el començament de la frase («compara
			// com gestionen els errors el client»): se'n queden les dues
			// últimes paraules, que és el que s'enumera.
			if w := strings.Fields(it); i == 0 && len(w) > 6 {
				it = strings.Join(w[len(w)-2:], " ")
			}
			if it == "" || len(strings.Fields(it)) > 6 {
				out = nil
				break
			}
			out = append(out, it)
		}
		if out != nil {
			return append(out, tail)
		}
	}
	return nil
}

// pathFocuses agrupa els camins de la tasca per carpeta (els dos primers
// trams): dues carpetes diferents són dues àrees.
func pathFocuses(t string) []string {
	groups := map[string][]string{}
	var order []string
	seen := map[string]bool{}
	for _, p := range focusPath.FindAllString(focusURL.ReplaceAllString(t, " "), -1) {
		p = strings.Trim(p, "./")
		if p == "" || seen[p] {
			continue
		}
		seen[p] = true
		segs := strings.Split(p, "/")
		key := segs[0]
		if len(segs) > 2 {
			key = segs[0] + "/" + segs[1]
		} else if len(segs) == 1 {
			key = "."
		}
		if _, ok := groups[key]; !ok {
			order = append(order, key)
		}
		groups[key] = append(groups[key], p)
	}
	if len(order) < 2 {
		return nil
	}
	out := make([]string, 0, len(order))
	for _, k := range order {
		out = append(out, strings.Join(groups[k], ", "))
	}
	return out
}
