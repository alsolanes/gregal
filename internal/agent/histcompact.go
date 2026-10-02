package agent

// Fer lloc a l'historial de l'agent a mig torn, per aquest ordre:
//
//  1. Retallar les sortides d'eina VELLES (les que ja no són dels últims
//     KeepRecentToolOutputs resultats): un `read` de 8000 caràcters del pas
//     tres no cal sencer al pas vint; en queda el principi i una nota per
//     tornar-lo a demanar. Això sol és barat, no perd cap decisió i
//     acostuma a recuperar la meitat del context.
//  2. Si encara no n'hi ha prou, resumir amb el model tot el que no siguin
//     els últims passos (Summarize) i continuar amb «resum + cua»: el
//     model sap què s'ha fet, què s'ha decidit i què falta. És el que fan
//     opencode i Claude Code; abans aquí es tallava cegament als quatre
//     últims missatges i el model perdia el fil.
//  3. Si el resum falla (model local penjat, timeout), retall segur: mai
//     es comença la cua per un missatge tool sense la crida que el demana,
//     que és un 400 en molts proveïdors.
//
// L'historial pot dur el missatge system al davant (headless, web) o no
// (TUI, que l'afegeix a cada pas): es conserva on és i no es toca.

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"gregal/internal/llm"
)

const (
	// KeepRecentToolOutputs són els últims resultats d'eina que es
	// conserven sencers; els anteriors es retallen a OldToolOutputMax.
	KeepRecentToolOutputs = 6
	OldToolOutputMax      = 600
	// KeepRecentOnCompact són els missatges de cua que sobreviuen sencers
	// a un resum (a més del resum i la tasca).
	KeepRecentOnCompact = 6
)

// errResACompactar: la capçalera només té la tasca; no hi ha passos a resumir.
var errResACompactar = errors.New("res a resumir")

const notaRetall = "\n…[sortida antiga retallada per fer lloc al context: torna a cridar l'eina si la necessites sencera]"

// PruneToolOutputs retalla les sortides d'eina velles. Torna l'historial
// nou (còpia) i quants missatges ha retallat.
func PruneToolOutputs(hist []llm.Message) ([]llm.Message, int) {
	return pruneToolOutputs(hist, KeepRecentToolOutputs)
}

// PruneToolOutputsUntil retalla primer les velles i, si fits encara diu
// que no, va retallant també les recents de la més antiga a la més nova
// fins a deixar-ne només dues de senceres. Passa quan un sol pas porta
// quatre lectures grosses: cap és «vella» i tot i així no hi caben.
func PruneToolOutputsUntil(hist []llm.Message, fits func([]llm.Message) bool) ([]llm.Message, int) {
	out, n := pruneToolOutputs(hist, KeepRecentToolOutputs)
	for keep := KeepRecentToolOutputs - 1; keep >= 2 && !fits(out); keep-- {
		var m int
		out, m = pruneToolOutputs(out, keep)
		n += m
	}
	return out, n
}

// RecoverContext retalla un historial quan el provider rebutja el prompt.
// Usa la mida real reportada per calibrar l'estimació (que no compta els
// esquemes d'eines) i deixa un marge del 35% per al pròxim pas.
func RecoverContext(hist []llm.Message, system string, window, reportedPrompt int) ([]llm.Message, int, bool) {
	if window <= 0 {
		window = DefaultWindow
	}
	estimate := func(h []llm.Message) int {
		if system != "" && (len(h) == 0 || h[0].Role != "system") {
			h = append([]llm.Message{{Role: "system", Content: system}}, h...)
		}
		return llm.EstimateTokens(h)
	}
	base := estimate(hist)
	scale := 1.0
	if base > 0 && reportedPrompt > base {
		scale = float64(reportedPrompt) / float64(base)
	}
	target := int(float64(window) * 0.65)
	fits := func(h []llm.Message) bool { return float64(estimate(h))*scale <= float64(target) }

	trimmed, pruned := PruneToolOutputsUntil(hist, fits)
	for keep := KeepRecentOnCompact; !fits(trimmed) && keep >= 0; keep-- {
		trimmed = TrimKeepSafe(trimmed, keep)
	}
	removed := len(hist) - len(trimmed)
	changed := removed > 0 || pruned > 0
	return trimmed, removed + pruned, changed
}

func pruneToolOutputs(hist []llm.Message, keepRecent int) ([]llm.Message, int) {
	out := append([]llm.Message(nil), hist...)
	recents := 0
	pruned := 0
	for i := len(out) - 1; i >= 0; i-- {
		if out[i].Role != "tool" {
			continue
		}
		recents++
		if recents <= keepRecent {
			continue
		}
		if len(out[i].Images) > 0 {
			out[i].Images = nil
			pruned++
		}
		if strings.HasSuffix(out[i].Content, notaRetall) {
			continue
		}
		if r := []rune(out[i].Content); len(r) > OldToolOutputMax {
			mida := len(out[i].Content)
			out[i].Content = string(r[:OldToolOutputMax]) + fmt.Sprintf(" (%d caràcters en total)", mida) + notaRetall
			pruned++
		}
	}
	return out, pruned
}

// SafeCut torna l'índex a partir del qual la cua té almenys keep
// missatges i no comença per un tool ni per un assistant amb tool_calls
// sense els seus resultats a dins (sí que pot començar per un assistant
// amb tool_calls, perquè els tool que el segueixen queden a la cua).
// Amb el system al davant, la cua mai l'inclou (es tracta a part).
func SafeCut(hist []llm.Message, keep int) int {
	inici := 0
	if len(hist) > 0 && hist[0].Role == "system" {
		inici = 1
	}
	cut := len(hist) - keep
	if cut < inici {
		cut = inici
	}
	for cut > inici && hist[cut].Role == "tool" {
		cut--
	}
	return cut
}

// tascaDe troba l'última consigna real de l'usuari al tros que es
// resumeix (no les guies internes), per repetir-la literal sota el resum:
// el resum pot parafrasejar-la i el model ha de tenir la petició exacta.
func tascaDe(cap []llm.Message) string {
	for i := len(cap) - 1; i >= 0; i-- {
		m := cap[i]
		if m.Role != "user" {
			continue
		}
		c := strings.TrimSpace(m.Content)
		if c == "" || c == GuiaTodosPendents || c == GuiaEinaText || strings.HasPrefix(c, "La crida al model ha fallat") || strings.HasPrefix(c, "[Resum") {
			continue
		}
		return c
	}
	return ""
}

// CompactHist resumeix tot menys els últims keep missatges i torna
// «(system) + resum + tasca + cua». Si el resum falla, torna l'error i
// l'historial intacte: qui crida decideix el retall de recanvi.
func CompactHist(ctx context.Context, client *llm.Client, prim llm.Target, fb *llm.Target, hist []llm.Message, keep int) ([]llm.Message, error) {
	cut := SafeCut(hist, keep)
	inici := 0
	if len(hist) > 0 && hist[0].Role == "system" {
		inici = 1
	}
	if cut <= inici+1 {
		return hist, errResACompactar
	}
	capçalera := hist[inici:cut]
	resum, err := Summarize(ctx, client, prim, fb, capçalera)
	if err != nil {
		return hist, err
	}
	var b strings.Builder
	b.WriteString("[Resum del que s'ha fet fins ara en aquest torn — continua des d'aquí, sense repetir feina feta]\n")
	b.WriteString(resum)
	if t := tascaDe(capçalera); t != "" {
		b.WriteString("\n\nTasca original de l'usuari:\n" + capRunes(t, 1500))
	}
	out := make([]llm.Message, 0, len(hist)-cut+2)
	out = append(out, hist[:inici]...)
	out = append(out, llm.Message{Role: "user", Content: b.String()})
	out = append(out, hist[cut:]...)
	return out, nil
}

// TrimKeepSafe és TrimKeep amb tall segur: conserva el system i, si el
// tall es menja la consigna de l'usuari, la torna a posar davant de la
// cua (sense cap missatge user molts servidors responen 400 «No user
// query found», vist en viu amb llama.cpp).
func TrimKeepSafe(hist []llm.Message, keep int) []llm.Message {
	cut := SafeCut(hist, keep)
	inici := 0
	if len(hist) > 0 && hist[0].Role == "system" {
		inici = 1
	}
	out := make([]llm.Message, 0, len(hist)-cut+inici+1)
	out = append(out, hist[:inici]...)
	if t := tascaDe(hist[inici:cut]); t != "" && !teUser(hist[cut:]) {
		out = append(out, llm.Message{Role: "user", Content: "[Tasca en curs — els passos anteriors s'han retallat per context]\n" + capRunes(t, 1500)})
	}
	return append(out, hist[cut:]...)
}

func teUser(h []llm.Message) bool {
	for _, m := range h {
		if m.Role == "user" {
			return true
		}
	}
	return false
}

// RoomResult diu què ha calgut fer per fer lloc.
type RoomResult struct {
	Hist       []llm.Message
	Before     int // tokens estimats abans
	After      int // tokens estimats després
	Pruned     int // sortides d'eina retallades
	Summarized bool
	Trimmed    bool // retall de recanvi (el resum ha fallat)
	Err        error
}

// Changed diu si s'ha tocat res.
func (r RoomResult) Changed() bool { return r.Pruned > 0 || r.Summarized || r.Trimmed }

// Note és una línia per a la UI ("" si no s'ha fet res).
func (r RoomResult) Note() string {
	if !r.Changed() {
		return ""
	}
	parts := []string{}
	if r.Pruned > 0 {
		parts = append(parts, fmt.Sprintf("%d sortides velles retallades", r.Pruned))
	}
	if r.Summarized {
		parts = append(parts, "passos anteriors resumits")
	}
	if r.Trimmed {
		motiu := "sense model per resumir"
		if r.Err != nil {
			motiu = "el resum ha fallat: " + truncaRunes(r.Err.Error(), 120)
		}
		parts = append(parts, "retall dels passos més antics ("+motiu+")")
	}
	return fmt.Sprintf("context de l'agent %s → %s: %s", llm.FmtCount(r.Before), llm.FmtCount(r.After), strings.Join(parts, ", "))
}

// MakeRoom aplica les tres etapes fins que l'historial cap a la finestra.
// sys és el system prompt si NO és dins de hist (TUI); buit si ja hi és.
// client pot ser nil: aleshores no hi ha resum i es passa al retall.
func MakeRoom(ctx context.Context, client *llm.Client, prim llm.Target, fb *llm.Target, sys string, hist []llm.Message, window int) RoomResult {
	est := func(h []llm.Message) int {
		if sys != "" {
			h = append([]llm.Message{{Role: "system", Content: sys}}, h...)
		}
		return llm.EstimateTokens(h)
	}
	res := RoomResult{Hist: hist}
	res.Before = est(hist)
	res.After = res.Before
	window = WindowOf(window)
	fits := func(h []llm.Message) bool { return !NeedsCompact(est(h), window) }
	if fits(hist) {
		return res
	}
	// 1. Sortides d'eina velles.
	res.Hist, res.Pruned = PruneToolOutputs(hist)
	if fits(res.Hist) {
		res.After = est(res.Hist)
		return res
	}
	// 2. Amb pocs passos (o sense model) no val la pena resumir: es
	// retallen també les sortides recents, de la més antiga a la més nova.
	if len(res.Hist) <= 12 || client == nil {
		var n int
		res.Hist, n = PruneToolOutputsUntil(res.Hist, fits)
		res.Pruned += n
		if fits(res.Hist) || client == nil && len(res.Hist) <= KeepRecentOnCompact+1 {
			res.After = est(res.Hist)
			return res
		}
	}
	// 3. Resum dels passos anteriors amb el model.
	if client != nil && len(res.Hist) > KeepRecentOnCompact+1 {
		if h, err := CompactHist(ctx, client, prim, fb, res.Hist, KeepRecentOnCompact); err == nil {
			res.Hist, res.Summarized = h, true
			if !fits(res.Hist) {
				var n int
				res.Hist, n = PruneToolOutputsUntil(res.Hist, fits)
				res.Pruned += n
			}
			res.After = est(res.Hist)
			return res
		} else if !errors.Is(err, errResACompactar) {
			res.Err = err
		}
	}
	// 4. Retall segur de recanvi (només si hi ha passos vells a treure).
	if len(res.Hist) > KeepRecentOnCompact+1 && (res.Err != nil || client == nil) {
		res.Hist = TrimKeepSafe(res.Hist, KeepRecentOnCompact)
		res.Trimmed = true
	}
	if !fits(res.Hist) {
		var n int
		res.Hist, n = PruneToolOutputsUntil(res.Hist, fits)
		res.Pruned += n
	}
	res.After = est(res.Hist)
	return res
}

// CallPlan és la decisió prèvia sobre una crida d'eina d'un pas: resposta
// fixa (bloquejada o repetida), cal permís, o s'executa.
type CallPlan struct {
	Fixed  string // si no és buit, és la resposta i no s'executa
	Ask    bool   // cal permís de l'usuari (o --auto-approve)
	Reason string
}

// PlanCalls aplica, en ordre i d'una sola manera per a tots els bucles, la
// política i el detector de repeticions a les crides d'un pas. doom pot
// ser nil (subagents). La pregunta (question) no es planifica: cada UI la
// tracta a la seva manera.
func PlanCalls(pol *Policy, mode string, doom *DoomTracker, calls []llm.ToolCall) []CallPlan {
	out := make([]CallPlan, len(calls))
	for i, c := range calls {
		dec, reason := pol.Decide(mode, c.Function.Name, c.Function.Arguments)
		switch dec {
		case "deny":
			out[i] = CallPlan{Fixed: "EINA BLOQUEJADA: " + reason, Reason: reason}
			continue
		case "ask":
			out[i] = CallPlan{Ask: true, Reason: reason}
		}
		if doom != nil && doom.Note(DoomSig(c.Function.Name, c.Function.Arguments)) {
			out[i] = CallPlan{Fixed: DoomGuide, Reason: "repetida"}
		}
	}
	return out
}
