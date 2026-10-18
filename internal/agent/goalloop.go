package agent

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"gregal/internal/config"
	"gregal/internal/goal"
	"gregal/internal/llm"
	"gregal/internal/verify"
)

// El bucle d'objectiu: executa un objectiu (mode goal) tantes rondes com
// calgui fins que un verificador independent diu que es compleix, amb
// proves. Reutilitza el que ja hi havia: les comprovacions mecàniques del
// mode autònom (RunAutonomousChecks, `--verify`), el rol revisor i el torn
// normal de l'agent.
//
// Cada ronda:
//  1. un torn de l'agent amb l'objectiu (o, a partir de la segona, amb el
//     que el verificador va dir que faltava);
//  2. les comprovacions declarades (verify del config o --verify) corren al
//     workspace. Si en falla alguna, la ronda no es compleix i la sortida
//     guia la següent: no es gasta cap crida de verificador;
//  3. si passen (o no n'hi ha), un fill de només lectura amb el rol revisor
//     mira els fitxers de debò, amb el diff i la sortida de les
//     comprovacions davant, i contesta criteri per criteri (verify/goal.go).
//
// L'objectiu només es dóna per complert quan el verificador diu MET amb
// una prova per criteri. Si no, el bucle s'atura per un límit clar (rondes,
// passos, temps, cost, o dues rondes seguides idèntiques) i torna un resum
// parcial: què es compleix, què no, i per què s'ha parat.

// Motius d'aturada del bucle d'objectiu.
const (
	GoalMet           = "met"
	GoalStopRounds    = "rounds"
	GoalStopSteps     = "steps"
	GoalStopTime      = "time"
	GoalStopCost      = "cost"
	GoalStopStalled   = "stalled"
	GoalStopCancelled = "cancelled"
	GoalStopVerifier  = "verifier_error"
	GoalStopError     = "error"
)

// Límits per defecte del bucle.
const (
	defaultGoalRounds  = 4
	maxGoalRounds      = 10
	defaultGoalSteps   = 300
	defaultGoalMinutes = 60
	goalVerifierSteps  = 12
)

// goalVerifierTools: el verificador mira, no toca. bash passa pel mode
// inspect, que només deixa les ordres de lectura.
var goalVerifierTools = []string{"read", "grep", "glob", "symbols", "lsp", "bash"}

// GoalRound és el que va passar en una ronda.
type GoalRound struct {
	N        int                `json:"n"`
	Steps    int                `json:"steps"`
	Answer   string             `json:"answer,omitempty"`
	Checks   []AutonomousCheck  `json:"checks,omitempty"`
	Verified bool               `json:"verifier_ran"`
	Verdict  verify.GoalVerdict `json:"verdict"`
	CostUSD  *float64           `json:"cost_usd,omitempty"`
	Duration time.Duration      `json:"-"`
	Trace    []TraceCall        `json:"-"`
	Raw      string             `json:"-"`
}

// GoalResult és el resultat del bucle.
type GoalResult struct {
	Met    bool        `json:"met"`
	Stop   string      `json:"stop"`
	Rounds []GoalRound `json:"rounds"`
	// Summary és el resum final (parcial si no es compleix).
	Summary string `json:"summary"`
	// Answer és la resposta de l'agent a l'última ronda.
	Answer  string   `json:"answer"`
	Steps   int      `json:"steps"`
	CostUSD *float64 `json:"cost_usd,omitempty"`
	// CostKnown: cap model de la tirada deixava sense preu.
	Elapsed time.Duration `json:"-"`
}

// GoalLoopOpts configura el bucle.
type GoalLoopOpts struct {
	// Run són les opcions del torn (mode, dir, passos per ronda…); Task
	// se substitueix per la de cada ronda.
	Run  RunOpts
	Goal goal.Goal
	// Checks són les comprovacions mecàniques (buit = agent.autonomous.verify
	// del config; i si tampoc n'hi ha, només verifica el revisor).
	Checks []string
	// Límits (0 = per defecte: 4 rondes, 300 passos, 60 minuts, sense límit de cost).
	MaxRounds  int
	MaxSteps   int
	MaxMinutes int
	MaxCostUSD float64
	// VerifierRole és el rol del verificador (buit = agent.autonomous.reviewer_role).
	VerifierRole string
	// OnRound s'avisa en acabar cada ronda; OnEvent rep una línia de progrés.
	OnRound    func(GoalRound)
	OnProgress func(string)

	// Ganxos de proves.
	runRound func(ctx context.Context, o RunOpts) (RunResult, error)
	verifier func(ctx context.Context, prompt string) (RunResult, error)
}

// RunGoalLoop executa l'objectiu fins que es compleix o s'arriba a un límit.
func RunGoalLoop(ctx context.Context, client *llm.Client, cfg *config.Config, o GoalLoopOpts) GoalResult {
	start := time.Now()
	rounds := clampInt(o.MaxRounds, defaultGoalRounds, maxGoalRounds)
	maxSteps := o.MaxSteps
	if maxSteps <= 0 {
		maxSteps = defaultGoalSteps
	}
	minutes := o.MaxMinutes
	if minutes <= 0 {
		minutes = defaultGoalMinutes
	}
	checks := o.Checks
	if len(checks) == 0 && cfg != nil {
		checks = cfg.AutonomousConfig().Verify
	}
	run := o.runRound
	if run == nil {
		run = func(ctx context.Context, ro RunOpts) (RunResult, error) {
			return RunNonInteractiveOpts(ctx, client, cfg, ro)
		}
	}
	verifier := o.verifier
	if verifier == nil {
		role := o.VerifierRole
		if role == "" && cfg != nil {
			role = cfg.AutonomousConfig().ReviewerRole
		}
		if cfg != nil {
			if r, ok := cfg.Roles[role]; !ok || strings.TrimSpace(r.Model) == "" {
				role = "" // sense rol revisor: el rol per defecte del mode inspect
			}
		}
		dir := o.Run.Dir
		verifier = func(ctx context.Context, prompt string) (RunResult, error) {
			return RunNonInteractiveOpts(ctx, client, cfg, RunOpts{
				Task: prompt, Mode: ModeInspect, MaxSteps: goalVerifierSteps, HardStepLimit: true,
				Dir: dir, Role: role, Tools: goalVerifierTools,
			})
		}
	}
	progress := func(s string) {
		if o.OnProgress != nil {
			o.OnProgress(s)
		}
	}
	lang := ""
	if cfg != nil {
		lang = cfg.Lang()
	}

	deadline := start.Add(time.Duration(minutes) * time.Minute)
	rctx, cancel := context.WithDeadline(ctx, deadline)
	defer cancel()

	res := GoalResult{}
	var cost costSum
	task := goal.Task(o.Goal)
	lastDigest := ""
	finish := func(stop string) GoalResult {
		res.Stop, res.Met = stop, stop == GoalMet
		res.CostUSD = cost.ptr()
		res.Elapsed = time.Since(start)
		res.Summary = goalSummary(o.Goal, res)
		return res
	}

	for n := 1; ; n++ {
		if err := rctx.Err(); err != nil {
			return finish(stopFromCtx(ctx, rctx))
		}
		if res.Steps >= maxSteps {
			return finish(GoalStopSteps)
		}
		if n > rounds {
			return finish(GoalStopRounds)
		}
		if o.MaxCostUSD > 0 && cost.known() && cost.total >= o.MaxCostUSD {
			return finish(GoalStopCost)
		}
		progress(fmt.Sprintf("goal round %d/%d", n, rounds))

		ro := o.Run
		ro.Task = task
		// Passos d'una ronda: els demanats o els del config, i mai més del
		// que queda del total (el torn té un sostre de 200).
		if ro.MaxSteps <= 0 {
			ro.MaxSteps = 40
			if cfg != nil && cfg.Agent.MaxSteps > 0 {
				ro.MaxSteps = cfg.Agent.MaxSteps
			}
		}
		if left := maxSteps - res.Steps; ro.MaxSteps > left {
			ro.MaxSteps = left
		}
		if ro.MaxSteps > 200 {
			ro.MaxSteps = 200
		}
		t0 := time.Now()
		rr, err := run(rctx, ro)
		round := GoalRound{N: n, Steps: rr.Steps, Answer: rr.Answer, Trace: rr.Trace, Duration: time.Since(t0)}
		cost.add(rr.CostUSD)
		round.CostUSD = rr.CostUSD
		res.Steps += rr.Steps
		if rr.Answer != "" {
			res.Answer = rr.Answer
		}
		if err != nil {
			res.Rounds = append(res.Rounds, round)
			if rctx.Err() != nil {
				return finish(stopFromCtx(ctx, rctx))
			}
			out := finish(GoalStopError)
			out.Summary = fmt.Sprintf("round %d failed: %v\n\n%s", n, err, out.Summary)
			return out
		}
		if rctx.Err() != nil { // el temps s'ha acabat durant la ronda: sense verificar, es para
			res.Rounds = append(res.Rounds, round)
			return finish(stopFromCtx(ctx, rctx))
		}

		// 2. Comprovacions mecàniques: la primera que falla guia la ronda següent.
		var failed *AutonomousCheck
		if len(checks) > 0 {
			cctx, ccancel := context.WithTimeout(rctx, verifyTimeout*time.Duration(len(checks)))
			round.Checks = RunAutonomousChecks(cctx, ro.Dir, checks)
			ccancel()
			for i := range round.Checks {
				if round.Checks[i].Code != 0 {
					failed = &round.Checks[i]
					break
				}
			}
		}
		evidence := GoalEvidence(ro.Dir, round.Checks, rr.Trace, rr.Answer)
		digest := digestOf(evidence)

		if failed != nil {
			round.Verdict = verify.GoalVerdict{Reason: fmt.Sprintf("check `%s` failed (exit %d)", failed.Command, failed.Code),
				Missing: fmt.Sprintf("make `%s` pass", failed.Command)}
		} else {
			// 3. Verificador independent, amb les proves davant.
			progress(fmt.Sprintf("goal round %d: verifying", n))
			vr, verr := verifier(rctx, verify.BuildGoalPrompt(o.Goal.Body, evidence, lang))
			cost.add(vr.CostUSD)
			res.Steps += vr.Steps
			round.Verified = true
			round.Raw = vr.Answer
			if verr != nil {
				res.Rounds = append(res.Rounds, round)
				if rctx.Err() != nil {
					return finish(stopFromCtx(ctx, rctx))
				}
				round.Verdict = verify.GoalVerdict{Reason: "the verifier failed: " + verr.Error()}
				res.Rounds[len(res.Rounds)-1] = round
				return finish(GoalStopVerifier)
			}
			round.Verdict = verify.ParseGoalVerdict(vr.Answer)
		}
		round.CostUSD = cost.ptr()
		res.Rounds = append(res.Rounds, round)
		if o.OnRound != nil {
			o.OnRound(round)
		}
		if round.Verdict.Met {
			return finish(GoalMet)
		}
		if digest == lastDigest {
			return finish(GoalStopStalled)
		}
		lastDigest = digest
		task = goalRepairTask(o.Goal, n, round)
	}
}

func clampInt(v, def, max int) int {
	if v <= 0 {
		return def
	}
	if v > max {
		return max
	}
	return v
}

// stopFromCtx distingeix el temps que s'acaba de la cancel·lació de l'usuari.
func stopFromCtx(parent, derived context.Context) string {
	if parent.Err() != nil {
		return GoalStopCancelled
	}
	return GoalStopTime
}

func digestOf(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:8])
}

// costSum suma costos; si algun és desconegut, el total no es pot dir.
type costSum struct {
	total   float64
	unknown bool
	seen    bool
}

func (c *costSum) add(p *float64) {
	c.seen = true
	if p == nil {
		c.unknown = true
		return
	}
	c.total += *p
}

func (c *costSum) known() bool { return c.seen && !c.unknown }

func (c *costSum) ptr() *float64 {
	if !c.known() {
		return nil
	}
	v := c.total
	return &v
}

// goalRepairTask és la tasca de la ronda següent: l'objectiu, el que no es
// compleix i la prohibició d'afirmar èxit sense prova.
func goalRepairTask(g goal.Goal, n int, r GoalRound) string {
	var b strings.Builder
	b.WriteString(goal.Task(g))
	fmt.Fprintf(&b, "\n\nROUND %d RESULT: the goal is NOT met yet.\n", n)
	for _, c := range r.Checks {
		if c.Code != 0 {
			fmt.Fprintf(&b, "The check `%s` FAILED (exit code %d):\n%s\n", c.Command, c.Code, trimTail(c.Output, 2500))
		}
	}
	for _, c := range r.Verdict.Criteria {
		if c.Status != verify.CriterionMet {
			fmt.Fprintf(&b, "- [%s] %s", c.Status, c.Text)
			if c.Evidence != "" {
				b.WriteString(" — " + c.Evidence)
			}
			b.WriteByte('\n')
		}
	}
	if r.Verdict.Missing != "" {
		b.WriteString("Missing: " + r.Verdict.Missing + "\n")
	}
	if r.Verdict.Reason != "" && len(r.Verdict.Criteria) == 0 {
		b.WriteString("Verifier: " + r.Verdict.Reason + "\n")
	}
	b.WriteString("\nThe changes of the previous rounds are already on disk. Do not claim it works: fix what is missing with tools and run the relevant checks yourself. An independent verifier will look at the real files and the check output, so only what is demonstrated counts. In your final answer, list what you changed and how you verified it.")
	return b.String()
}

// GoalEvidence reuneix les proves d'una ronda: les comprovacions, el diff
// i els fitxers canviats, i el que l'agent ha executat (amb el resultat).
// Les paraules de l'agent hi van marcades com a afirmació, no com a prova.
func GoalEvidence(dir string, checks []AutonomousCheck, trace []TraceCall, answer string) string {
	var b strings.Builder
	b.WriteString("CHECKS (run by Gregal after the round):\n")
	if len(checks) == 0 {
		b.WriteString("- none configured\n")
	}
	for _, c := range checks {
		status := "PASS"
		if c.Code != 0 {
			status = fmt.Sprintf("FAIL (exit %d)", c.Code)
		}
		fmt.Fprintf(&b, "- `%s`: %s\n", c.Command, status)
		if out := strings.TrimSpace(c.Output); out != "" {
			b.WriteString(indent(trimTail(out, 1500)) + "\n")
		}
	}
	b.WriteString("\nWORKING TREE CHANGES (git status, stat and diff):\n")
	if d := AutonomousDiff(dir); strings.TrimSpace(d) != "" {
		b.WriteString(d + "\n")
	} else {
		b.WriteString("(no uncommitted changes, or not a git repository: look at the files with your tools)\n")
	}
	var ran []string
	for _, tc := range trace {
		if tc.Tool != "bash" && tc.Tool != "bash_background" {
			continue
		}
		line := "- " + trimTail(oneLine(tc.Args), 160)
		if tc.Failed {
			line += "  [FAILED]"
		}
		if s := strings.TrimSpace(tc.Summary); s != "" {
			line += "\n" + indent(trimTail(s, 300))
		}
		ran = append(ran, line)
	}
	if len(ran) > 12 {
		ran = ran[len(ran)-12:]
	}
	if len(ran) > 0 {
		b.WriteString("\nSHELL COMMANDS THE AGENT RAN THIS ROUND (last ones):\n" + strings.Join(ran, "\n") + "\n")
	}
	if a := strings.TrimSpace(answer); a != "" {
		b.WriteString("\nTHE AGENT'S OWN FINAL ANSWER (a claim, not evidence):\n" + trimTail(a, 1500) + "\n")
	}
	return b.String()
}

func trimTail(s string, max int) string {
	r := []rune(strings.TrimSpace(s))
	if len(r) <= max {
		return string(r)
	}
	return "…" + string(r[len(r)-max:])
}

func oneLine(s string) string { return strings.Join(strings.Fields(s), " ") }

func indent(s string) string {
	lines := strings.Split(s, "\n")
	for i := range lines {
		lines[i] = "    " + lines[i]
	}
	return strings.Join(lines, "\n")
}

var goalStopText = map[string]string{
	GoalStopRounds:    "stopped: the round limit was reached",
	GoalStopSteps:     "stopped: the step limit was reached",
	GoalStopTime:      "stopped: the time limit was reached",
	GoalStopCost:      "stopped: the cost limit was reached",
	GoalStopStalled:   "stopped: two rounds in a row left the project exactly as it was, so more rounds would not change anything",
	GoalStopCancelled: "stopped: cancelled",
	GoalStopVerifier:  "stopped: the verifier could not run",
	GoalStopError:     "stopped: a round failed",
}

// goalSummary és el resum final: complert amb les proves, o parcial amb
// el que es compleix, el que no i per què s'ha parat.
func goalSummary(g goal.Goal, res GoalResult) string {
	var b strings.Builder
	if res.Met {
		fmt.Fprintf(&b, "GOAL MET after %d round(s): %s\n", len(res.Rounds), g.Title)
	} else {
		fmt.Fprintf(&b, "GOAL NOT MET (%s) after %d round(s): %s\n", goalStopText[res.Stop], len(res.Rounds), g.Title)
	}
	if n := len(res.Rounds); n > 0 {
		last := res.Rounds[n-1]
		for _, c := range last.Checks {
			st := "PASS"
			if c.Code != 0 {
				st = fmt.Sprintf("FAIL (exit %d)", c.Code)
			}
			fmt.Fprintf(&b, "check `%s`: %s\n", c.Command, st)
		}
		for _, c := range last.Verdict.Criteria {
			fmt.Fprintf(&b, "- [%s] %s", c.Status, c.Text)
			if c.Evidence != "" {
				b.WriteString(" :: " + c.Evidence)
			}
			b.WriteByte('\n')
		}
		if !res.Met {
			if last.Verdict.Missing != "" {
				b.WriteString("Still missing: " + last.Verdict.Missing + "\n")
			}
			if last.Verdict.Reason != "" {
				b.WriteString("Last verdict: " + last.Verdict.Reason + "\n")
			}
		}
	}
	fmt.Fprintf(&b, "steps: %d · elapsed: %s", res.Steps, res.Elapsed.Round(time.Second))
	if res.CostUSD != nil {
		b.WriteString(" · cost: " + FmtCost(*res.CostUSD))
	}
	if a := strings.TrimSpace(res.Answer); a != "" && !res.Met {
		b.WriteString("\n\nLast agent answer:\n" + trimTail(a, 1200))
	}
	return b.String()
}
