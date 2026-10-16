package evals

// Agregació del banc (P1 de docs/research/harness-techniques.md). Un sol
// PASS/FAIL per tasca no serveix per comparar dos prompts o dos models
// locals: el soroll d'una tirada a l'altra és més gran que la millora.
// Amb --repeat N es dona, per tasca, la taxa d'èxit, pass@k i la mediana
// amb el rang interquartílic dels passos, el temps i els tokens, a més de
// les mètriques que validen les altres propostes (caché, edició, crides
// mal formades, repeticions, compactacions).

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// Dist és la mediana i els quartils d'una mètrica entre repeticions.
type Dist struct {
	Median float64 `json:"median"`
	Q1     float64 `json:"q1"`
	Q3     float64 `json:"q3"`
	IQR    float64 `json:"iqr"`
}

// Metrics són els comptadors sumats d'un conjunt d'execucions i les taxes
// que se'n deriven. Les taxes es calculen sobre les sumes (no és la
// mitjana de les taxes de cada execució: una tirada de 2 edicions no pesa
// com una de 40).
type Metrics struct {
	PromptTokens      int `json:"prompt_tokens"`
	CachedTokens      int `json:"cached_tokens"`
	CompletionTokens  int `json:"completion_tokens"`
	LLMCalls          int `json:"llm_calls"`
	EditCalls         int `json:"edit_calls"`
	EditFailed        int `json:"edit_failed"`
	ToolCallInvalid   int `json:"tool_call_invalid"`
	ToolCallRecovered int `json:"tool_call_recovered"`
	DoomHits          int `json:"doom_hits"`
	Compactions       int `json:"compactions"`
	// CacheHitRate = cached_tokens / prompt_tokens (només tokens declarats
	// pel proveïdor). EditFailRate = edit_failed / edit_calls. Totes dues
	// són 0 quan el denominador és 0.
	CacheHitRate float64 `json:"cache_hit_rate"`
	EditFailRate float64 `json:"edit_fail_rate"`
}

func (m *Metrics) add(r Result) {
	m.PromptTokens += r.PromptTokens
	m.CachedTokens += r.CachedTokens
	m.CompletionTokens += r.Down
	m.LLMCalls += r.LLMCalls
	m.EditCalls += r.EditCalls
	m.EditFailed += r.EditFailed
	m.ToolCallInvalid += r.ToolCallInvalid
	m.ToolCallRecovered += r.ToolCallRecovered
	m.DoomHits += r.DoomHits
	m.Compactions += r.Compactions
}

func (m *Metrics) rates() {
	m.CacheHitRate = ratio(m.CachedTokens, m.PromptTokens)
	m.EditFailRate = ratio(m.EditFailed, m.EditCalls)
}

func ratio(a, b int) float64 {
	if b <= 0 {
		return 0
	}
	return round4(float64(a) / float64(b))
}

func round4(x float64) float64 { return math.Round(x*1e4) / 1e4 }

// TaskStats és l'agregat de les repeticions d'una tasca.
type TaskStats struct {
	Task   string `json:"task"`
	Runs   int    `json:"runs"`
	Passes int    `json:"passes"`
	// SuccessRate = passes / runs. Verdict és PASS si almenys la meitat
	// de les execucions passen: és el que es compara amb baseline.json
	// (amb una sola execució, el veredicte de sempre).
	SuccessRate float64 `json:"success_rate"`
	Verdict     string  `json:"verdict"`
	// PassAtK és l'estimador sense biaix de Chen et al. (2021): la
	// probabilitat que almenys una de k execucions triades passi.
	PassAtK    map[string]float64 `json:"pass_at_k"`
	Steps      Dist               `json:"steps"`
	Secs       Dist               `json:"secs"`
	PromptDist Dist               `json:"prompt_tokens_dist"`
	CallsDist  Dist               `json:"llm_calls_dist"`
	Metrics
}

// Totals és l'agregat de tot el banc.
type Totals struct {
	Runs        int     `json:"runs"`
	Passes      int     `json:"passes"`
	SuccessRate float64 `json:"success_rate"`
	Metrics
}

// Summary és el contingut de results/summary.json.
type Summary struct {
	Repeat int         `json:"repeat"`
	Totals Totals      `json:"totals"`
	Tasks  []TaskStats `json:"tasks"`
	Runs   []Result    `json:"runs"`
}

// Summarize agrega les execucions per tasca (en l'ordre de primera
// aparició) i per al banc sencer.
func Summarize(results []Result, repeat int) Summary {
	if repeat < 1 {
		repeat = 1
	}
	s := Summary{Repeat: repeat, Runs: results, Tasks: Aggregate(results)}
	for _, r := range results {
		s.Totals.Runs++
		if r.Verdict == "PASS" {
			s.Totals.Passes++
		}
		s.Totals.add(r)
	}
	s.Totals.SuccessRate = ratio(s.Totals.Passes, s.Totals.Runs)
	s.Totals.rates()
	return s
}

// Aggregate agrupa les execucions per tasca.
func Aggregate(results []Result) []TaskStats {
	var ordre []string
	per := map[string][]Result{}
	for _, r := range results {
		if _, ok := per[r.Task]; !ok {
			ordre = append(ordre, r.Task)
		}
		per[r.Task] = append(per[r.Task], r)
	}
	out := make([]TaskStats, 0, len(ordre))
	for _, t := range ordre {
		rs := per[t]
		st := TaskStats{Task: t, Runs: len(rs)}
		var steps, secs, prompt, calls []float64
		senseAssert := true
		for _, r := range rs {
			if r.Verdict == "PASS" {
				st.Passes++
			}
			if r.Verdict != "SENSE-ASSERT" {
				senseAssert = false
			}
			st.add(r)
			steps = append(steps, float64(r.Steps))
			secs = append(secs, float64(r.Secs))
			prompt = append(prompt, float64(r.PromptTokens))
			calls = append(calls, float64(r.LLMCalls))
		}
		st.rates()
		st.SuccessRate = ratio(st.Passes, st.Runs)
		switch {
		case senseAssert:
			st.Verdict = "SENSE-ASSERT"
		case 2*st.Passes >= st.Runs:
			st.Verdict = "PASS"
		case st.Runs == 1:
			st.Verdict = rs[0].Verdict // FAIL o ERROR, com sempre
		default:
			st.Verdict = "FAIL"
		}
		st.PassAtK = map[string]float64{}
		for _, k := range ksPassAt(st.Runs) {
			st.PassAtK[strconv.Itoa(k)] = round4(PassAtK(st.Runs, st.Passes, k))
		}
		st.Steps, st.Secs, st.PromptDist, st.CallsDist = dist(steps), dist(secs), dist(prompt), dist(calls)
		out = append(out, st)
	}
	return out
}

// ksPassAt són les k que val la pena donar per a n execucions.
func ksPassAt(n int) []int {
	var ks []int
	for _, k := range []int{1, 3, 5, 10, 20, 50, 100} {
		if k < n {
			ks = append(ks, k)
		}
	}
	return append(ks, n)
}

// PassAtK és 1 - C(n-c, k) / C(n, k): de n execucions amb c èxits, la
// probabilitat que en triar-ne k a l'atzar n'hi hagi almenys una de bona.
// Es calcula com a producte per no desbordar.
func PassAtK(n, c, k int) float64 {
	if n <= 0 || k <= 0 {
		return 0
	}
	if k > n {
		k = n
	}
	if n-c < k {
		return 1
	}
	p := 1.0
	for i := n - c + 1; i <= n; i++ {
		p *= 1 - float64(k)/float64(i)
	}
	return 1 - p
}

// dist calcula la mediana i els quartils (interpolació lineal, el
// mètode 7 de Hyndman-Fan, el de numpy i R per defecte).
func dist(xs []float64) Dist {
	if len(xs) == 0 {
		return Dist{}
	}
	s := append([]float64(nil), xs...)
	sort.Float64s(s)
	q1, med, q3 := quantil(s, 0.25), quantil(s, 0.5), quantil(s, 0.75)
	return Dist{Median: round4(med), Q1: round4(q1), Q3: round4(q3), IQR: round4(q3 - q1)}
}

func quantil(sorted []float64, p float64) float64 {
	h := p * float64(len(sorted)-1)
	lo := int(math.Floor(h))
	hi := int(math.Ceil(h))
	return sorted[lo] + (h-float64(lo))*(sorted[hi]-sorted[lo])
}

// Baseline és tasca → veredicte de la passada de referència
// (evals/baseline.json). Amb --repeat és el veredicte agregat.
type Baseline map[string]string

// LoadBaseline llegeix <dir>/baseline.json (nil si no hi és).
func LoadBaseline(dir string) Baseline {
	raw, err := os.ReadFile(filepath.Join(dir, "baseline.json"))
	if err != nil {
		return nil
	}
	var b Baseline
	if json.Unmarshal(raw, &b) != nil {
		return nil
	}
	return b
}

// SaveBaseline desa els veredictes (agregats per tasca) com a referència nova.
func SaveBaseline(dir string, results []Result) error {
	b := Baseline{}
	for _, st := range Aggregate(results) {
		b[st.Task] = st.Verdict
	}
	raw, _ := json.MarshalIndent(b, "", "  ")
	return os.WriteFile(filepath.Join(dir, "baseline.json"), raw, 0o644)
}

// Report pinta la taula de cada execució, l'agregat per tasca si hi ha
// repeticions, els totals i les diferències amb la referència. passed i
// total compten execucions; una regressió és una tasca que a la baseline
// passava i ara, agregada, no.
func Report(w io.Writer, results []Result, base Baseline) (passed, total int, regressions []string) {
	fmt.Fprintf(w, "%-6s %-12s %6s %8s %6s %5s %4s %4s %4s %5s  %-28s %s\n",
		"tasca", "veredicte", "passos", "tokens", "cache", "edit✗", "inv", "doom", "cmp", "s", "eines", "detall")
	repetit := false
	for _, r := range results {
		eines := strings.Join(r.Tools, ",")
		if len(eines) > 28 {
			eines = eines[:27] + "…"
		}
		nom := r.Task
		if r.Run > 0 {
			repetit = true
			nom = fmt.Sprintf("%s#%d", r.Task, r.Run)
		}
		fmt.Fprintf(w, "%-6s %-12s %6d %8d %6s %5s %4d %4d %4d %5d  %-28s %s\n", nom, r.Verdict, r.Steps, r.Up+r.Down,
			pct(r.CachedTokens, r.PromptTokens), fmt.Sprintf("%d/%d", r.EditFailed, r.EditCalls),
			r.ToolCallInvalid, r.DoomHits, r.Compactions, r.Secs, eines, r.Detail)
		total++
		if r.Verdict == "PASS" {
			passed++
		}
	}
	stats := Aggregate(results)
	if repetit {
		fmt.Fprintf(w, "\n%-6s %-9s %7s %7s %-15s %-15s %-17s %6s %6s\n",
			"tasca", "èxit", "pass@1", "pass@n", "passos med[IQR]", "s med[IQR]", "prompt med[IQR]", "cache", "edit✗")
		for _, st := range stats {
			fmt.Fprintf(w, "%-6s %-9s %7.2f %7.2f %-15s %-15s %-17s %6s %6s\n", st.Task,
				fmt.Sprintf("%d/%d", st.Passes, st.Runs), st.PassAtK["1"], st.PassAtK[strconv.Itoa(st.Runs)],
				fmtDist(st.Steps), fmtDist(st.Secs), fmtDist(st.PromptDist),
				pct(st.CachedTokens, st.PromptTokens), pct(st.EditFailed, st.EditCalls))
		}
	}
	if base != nil {
		for _, st := range stats {
			if was, ok := base[st.Task]; ok && was == "PASS" && st.Verdict != "PASS" {
				regressions = append(regressions, st.Task)
			}
		}
	}
	tot := Summarize(results, 1).Totals
	fmt.Fprintf(w, "\n%d/%d PASS · cache %s · edit✗ %s · crides invàlides %d · doom %d · compactacions %d · %d crides LLM",
		passed, total, pct(tot.CachedTokens, tot.PromptTokens), pct(tot.EditFailed, tot.EditCalls),
		tot.ToolCallInvalid, tot.DoomHits, tot.Compactions, tot.LLMCalls)
	if base != nil {
		if len(regressions) > 0 {
			fmt.Fprintf(w, " · REGRESSIONS respecte baseline: %s", strings.Join(regressions, ", "))
		} else {
			fmt.Fprintf(w, " · sense regressions respecte baseline")
		}
	} else {
		fmt.Fprintf(w, " · sense baseline (desa-la amb --eval-baseline)")
	}
	fmt.Fprintln(w)
	return passed, total, regressions
}

// pct és a/b en percentatge, o «-» si no hi ha denominador.
func pct(a, b int) string {
	if b <= 0 {
		return "-"
	}
	return fmt.Sprintf("%.0f%%", 100*float64(a)/float64(b))
}

func fmtDist(d Dist) string {
	return fmt.Sprintf("%s[%s]", strconv.FormatFloat(d.Median, 'f', -1, 64), strconv.FormatFloat(d.IQR, 'f', -1, 64))
}
