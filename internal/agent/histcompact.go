package agent

// Fer lloc a l'historial de l'agent a mig torn, per aquest ordre:
//
//  1. Emmascarar les sortides d'eina VELLES (P5): la sortida d'un `read`
//     del pas tres es canvia per «[output of `read foo.go` cleared …; call
//     the tool again if you need it]». Es pot recuperar (tornant a cridar
//     l'eina), no perd cap decisió i acostuma a recuperar la major part del
//     context. Es fa PER BLOCS i amb histèresi: quan es passa el llindar,
//     s'emmascara tot fins a les últimes KeepRecentToolOutputs d'un sol
//     cop, i encara més si cal per baixar a MaskLowWater. Així la caché de
//     prefix només es trenca de tant en tant, no a cada pas (abans es
//     retallava a 600 caràcters tot el que no fos dels sis últims i la
//     frontera es movia a cada pas).
//  2. Només si emmascarar no basta, resumir amb el model (Summarize) tot
//     el que no siguin els últims passos i continuar amb «resum + cua»: el
//     resum és un traspàs (objectiu, fet, pendent, decisions, fitxers,
//     errors oberts).
//  3. Si el resum falla (model local penjat, timeout), retall segur: mai
//     es comença la cua per un missatge tool sense la crida que el demana,
//     que és un 400 en molts proveïdors.
//
// Cap etapa no separa mai una crida d'eina del seu resultat: emmascarar
// canvia el contingut però deixa els dos missatges, i els talls passen
// per SafeCut.
//
// L'historial pot dur el missatge system al davant (headless, web) o no
// (TUI, que l'afegeix a cada pas): es conserva on és i no es toca.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"gregal/internal/config"
	"gregal/internal/llm"
)

const (
	// KeepRecentToolOutputs són els últims resultats d'eina que queden
	// sencers quan s'emmascara un bloc; els anteriors es canvien per una
	// nota recuperable (els de delegate, que no es poden recuperar, es
	// retallen a OldToolOutputMax).
	KeepRecentToolOutputs = 10
	OldToolOutputMax      = 600
	// MinKeepHisteresi és el mínim de resultats recents que es conserven
	// per baixar fins a MaskLowWater; per cabre a la finestra, si cal, se
	// n'emmascaren fins a deixar-ne dos. No baixa de 6: `read` pot respondre
	// «sense canvis» a un rang llegit fa poques crides, i aquella sortida ha
	// de seguir sencera.
	MinKeepHisteresi = 6
	// MaskLowWater és la fracció del llindar de compactació on es vol
	// deixar l'historial després d'emmascarar: prou lluny perquè el pas
	// següent no hi torni a arribar i hagi de moure la frontera.
	MaskLowWater = 0.75
	// MaskMinChars: una sortida més curta no val la pena emmascarar-la (la
	// nota ja fa uns 150 caràcters i la caché es trencaria per res).
	MaskMinChars = 400
	// KeepRecentOnCompact són els missatges de cua que sobreviuen sencers
	// a un resum (a més del resum i la tasca).
	KeepRecentOnCompact = 6
)

// errResACompactar: la capçalera només té la tasca; no hi ha passos a resumir.
var errResACompactar = errors.New("res a resumir")

// notaRetall és la nota d'abans (cap de 600 caràcters): les sessions
// desades encara en porten i no es tornen a tocar.
const notaRetall = "\n…[old output truncated to make room in the context: call the tool again if you need it in full]"

// notaRetallFill és la poda per a resums de subagents: re-delegar executa
// una altra cerca, no la mateixa, així que "torna a cridar" seria mentida.
const notaRetallFill = "\n…[subagent summary truncated to make room in the context: it cannot be fetched again; continue with the summary]"

// Les notes d'abans (en català) encara són a sessions desades: una sortida
// que ja en porta una no es torna a retallar.
const (
	notaRetallCA     = "\n…[sortida antiga retallada per fer lloc al context: torna a cridar l'eina si la necessites sencera]"
	notaRetallFillCA = "\n…[resum de subagent retallat per fer lloc al context: no es pot reobtenir; continua amb el resum]"
)

// Nota que substitueix una sortida emmascarada.
const (
	prefixEmmascarada = "[output of `"
	sufixEmmascarada  = "call the tool again if you need it]"
)

func notaEmmascarada(desc string, mida int) string {
	return fmt.Sprintf("%s%s` cleared to free context (%d characters); %s", prefixEmmascarada, desc, mida, sufixEmmascarada)
}

// jaPodada diu si una sortida ja porta una nota (emmascarada o retallada).
func jaPodada(c string) bool {
	return strings.HasPrefix(c, prefixEmmascarada) && strings.HasSuffix(c, sufixEmmascarada) ||
		strings.HasSuffix(c, notaRetall) || strings.HasSuffix(c, notaRetallFill) ||
		strings.HasSuffix(c, notaRetallCA) || strings.HasSuffix(c, notaRetallFillCA)
}

// descCrida és «read foo.go», «bash go test ./...»: el que cal per saber
// què es va demanar i tornar-ho a demanar.
func descCrida(name, args string) string {
	var v map[string]any
	if json.Unmarshal([]byte(args), &v) == nil {
		for _, k := range []string{"path", "file_path", "command", "pattern", "url", "query", "name"} {
			if s, ok := v[k].(string); ok && strings.TrimSpace(s) != "" {
				return name + " " + truncaRunes(strings.Join(strings.Fields(s), " "), 80)
			}
		}
	}
	return name
}

// PruneToolOutputs emmascara les sortides d'eina velles (totes menys les
// últimes KeepRecentToolOutputs) d'un sol cop. Torna l'historial nou
// (còpia) i quantes n'ha tocat.
func PruneToolOutputs(hist []llm.Message) ([]llm.Message, int) {
	return pruneToolOutputs(hist, KeepRecentToolOutputs)
}

// PruneToolOutputsUntil emmascara primer el bloc vell i, si fits encara
// diu que no, va emmascarant també les recents de la més antiga a la més
// nova fins a deixar-ne només dues de senceres. Passa quan un sol pas
// porta quatre lectures grosses: cap és «vella» i tot i així no hi caben.
func PruneToolOutputsUntil(hist []llm.Message, fits func([]llm.Message) bool) ([]llm.Message, int) {
	return pruneAmbHisteresi(hist, fits, nil)
}

// pruneAmbHisteresi és PruneToolOutputsUntil amb una segona meta: un cop
// es toca l'historial, continua fins que baix (si n'hi ha) també diu que
// sí, sense deixar menys de MinKeepHisteresi sortides recents per aquesta
// raó. Així el bloc emmascarat queda prou lluny del llindar.
func pruneAmbHisteresi(hist []llm.Message, fits, baix func([]llm.Message) bool) ([]llm.Message, int) {
	out, n := pruneToolOutputs(hist, KeepRecentToolOutputs)
	ok := func(h []llm.Message, keep int) bool {
		if !fits(h) {
			return false
		}
		return baix == nil || n == 0 || keep <= MinKeepHisteresi || baix(h)
	}
	for keep := KeepRecentToolOutputs - 1; keep >= 2 && !ok(out, keep+1); keep-- {
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
	crides := map[string]llm.ToolCall{}
	for _, m := range out {
		for _, c := range m.ToolCalls {
			crides[c.ID] = c
		}
	}
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
		tocat := false
		if len(out[i].Images) > 0 {
			out[i].Images = nil
			tocat = true
		}
		c := out[i].Content
		switch {
		case jaPodada(c):
		case out[i].Name == "delegate":
			// El resum d'un subagent no es pot tornar a demanar: se'n
			// queda el principi, com abans.
			if r := []rune(c); len(r) > OldToolOutputMax {
				out[i].Content = string(r[:OldToolOutputMax]) + fmt.Sprintf(" (%d characters in total)", len(c)) + notaRetallFill
				tocat = true
			}
		case len([]rune(c)) > MaskMinChars:
			name, args := out[i].Name, ""
			if cr, ok := crides[out[i].ToolCallID]; ok {
				name, args = cr.Function.Name, cr.Function.Arguments
			}
			if name == "" {
				name = "tool"
			}
			out[i].Content = notaEmmascarada(descCrida(name, args), len(c))
			tocat = true
		}
		if tocat {
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
// Les guies i capçaleres es reconeixen en anglès i en el català d'abans
// (sessions desades abans que els prompts passessin a l'anglès).
func tascaDe(cap []llm.Message) string {
	for i := len(cap) - 1; i >= 0; i-- {
		m := cap[i]
		if m.Role != "user" {
			continue
		}
		c := strings.TrimSpace(m.Content)
		if c == "" || c == GuiaTodosPendents || c == GuiaEinaText ||
			strings.HasPrefix(c, PrefixErrorServidor) || strings.HasPrefix(c, "La crida al model ha fallat") ||
			strings.HasPrefix(c, "[Summary") || strings.HasPrefix(c, "[Resum") || strings.HasPrefix(c, MarcaResum) {
			continue
		}
		return c
	}
	return ""
}

// CompactHist resumeix tot menys els últims keep missatges i torna
// «(system) + resum + tasca + cua». Si el resum falla, torna l'error i
// l'historial intacte: qui crida decideix el retall de recanvi. lang és la
// llengua del resum (la de la sessió).
func CompactHist(ctx context.Context, client *llm.Client, prim llm.Target, fb *llm.Target, hist []llm.Message, keep int, lang string) ([]llm.Message, error) {
	cut := SafeCut(hist, keep)
	inici := 0
	if len(hist) > 0 && hist[0].Role == "system" {
		inici = 1
	}
	if cut <= inici+1 {
		return hist, errResACompactar
	}
	capçalera := hist[inici:cut]
	resum, err := Summarize(ctx, client, prim, fb, capçalera, lang)
	if err != nil {
		return hist, err
	}
	var b strings.Builder
	b.WriteString(MarcaResum + "\n[Handoff note of the work done so far in this turn — continue from here without repeating finished work]\n")
	b.WriteString(resum)
	if t := tascaDe(capçalera); t != "" {
		b.WriteString("\n\nOriginal user task:\n" + capRunes(SenseMapa(t), 1500))
	}
	b.WriteString("\n" + MarcaResumFinal)
	out := make([]llm.Message, 0, len(hist)-cut+2)
	out = append(out, hist[:inici]...)
	out = append(out, llm.Message{Role: "user", Content: b.String()})
	// La cua perd els mapes vells (tots de cop, ara que la compactació ja
	// reescriu el principi): el torn següent en posarà un de sencer.
	out = append(out, senseMapesVells(hist[cut:])...)
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
		out = append(out, llm.Message{Role: "user", Content: "[Task in progress — earlier steps were trimmed to fit the context]\n" + capRunes(t, 1500)})
	}
	return append(out, hist[cut:]...)
}

// senseMapesVells treu els mapes del projecte de tots els missatges menys
// el més nou, si aquest és un mapa sencer (un delta sense el seu mapa base
// no serveix i també se'n va).
func senseMapesVells(h []llm.Message) []llm.Message {
	darrer := -1
	for i := len(h) - 1; i >= 0; i-- {
		if h[i].Role == "user" && SenseMapa(h[i].Content) != h[i].Content {
			darrer = i
			break
		}
	}
	out := SenseMapesAntics(h)
	if darrer >= 0 && !strings.Contains(h[darrer].Content, " delta] ") {
		out[darrer] = h[darrer]
	}
	return out
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
// client pot ser nil: aleshores no hi ha resum i es passa al retall. La
// mida es calcula amb el calibratge del model de prim (EstimaCalibrat).
func MakeRoom(ctx context.Context, client *llm.Client, prim llm.Target, fb *llm.Target, sys string, hist []llm.Message, window int) RoomResult {
	est := func(h []llm.Message) int {
		if sys != "" {
			h = append([]llm.Message{{Role: "system", Content: sys}}, h...)
		}
		return EstimaCalibrat(prim.Model, h)
	}
	res := RoomResult{Hist: hist}
	res.Before = est(hist)
	res.After = res.Before
	window = WindowOf(window)
	fits := func(h []llm.Message) bool { return !NeedsCompact(est(h), window) }
	if fits(hist) {
		return res
	}
	baix := func(h []llm.Message) bool {
		return float64(est(h)) <= float64(window)*CompactThreshold*MaskLowWater
	}
	// 1. Emmascarar per blocs, amb histèresi: és recuperable i barat.
	res.Hist, res.Pruned = pruneAmbHisteresi(hist, fits, baix)
	if fits(res.Hist) || client == nil && len(res.Hist) <= KeepRecentOnCompact+1 {
		res.After = est(res.Hist)
		return res
	}
	// 2. Resum dels passos anteriors amb el model (darrer recurs), en la
	// llengua de la directiva que tanca el system prompt (sigui a sys o
	// dins de hist).
	if client != nil && len(res.Hist) > KeepRecentOnCompact+1 {
		lang := config.LangOfPrompt(sys)
		if sys == "" && len(res.Hist) > 0 && res.Hist[0].Role == "system" {
			lang = config.LangOfPrompt(res.Hist[0].Content)
		}
		if h, err := CompactHist(ctx, client, prim, fb, res.Hist, KeepRecentOnCompact, lang); err == nil {
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
	// 3. Retall segur de recanvi (només si hi ha passos vells a treure).
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
	// ForceAsk: un hook pre_tool ha demanat confirmació; una aprovació
	// recordada de la sessió no la salta.
	ForceAsk bool
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
