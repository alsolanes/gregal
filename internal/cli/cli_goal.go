package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"

	"gregal/internal/agent"
	"gregal/internal/backlog"
	"gregal/internal/config"
	"gregal/internal/goal"
	"gregal/internal/llm"
	"gregal/internal/lsp"
	"gregal/internal/mcp"
)

// goalFlags són els flags de `gregal --goal`.
type goalFlags struct {
	Spec        string // número d'una tasca del backlog (#3), "last" o el text de l'objectiu
	Output      string // text | json
	Mode        string
	MaxRounds   int
	MaxSteps    int // per ronda (0 = el del config)
	MaxMinutes  int
	MaxCostUSD  float64
	VerifyCmd   string
	CfgPath     string
	AutoApprove bool
}

// resolGoal torna la tasca del backlog que diu spec («#3», «3» o «last»,
// la oberta més recent) com a objectiu per al bucle, o en fa un d'inline
// amb el text. item és la tasca (0 si és inline).
func resolGoal(cwd, spec string) (g goal.Goal, item int) {
	spec = strings.TrimSpace(spec)
	ref := strings.TrimPrefix(spec, "#")
	if spec == "last" {
		ref = ""
	}
	if _, err := strconv.Atoi(ref); err == nil || spec == "last" {
		if it, err := goal.Resolve(cwd, ref); err == nil {
			return goal.ForLoop(it, cwd), it.ID
		}
	}
	title := strings.Join(strings.Fields(spec), " ")
	if r := []rune(title); len(r) > 80 {
		title = string(r[:80]) + "…"
	}
	return goal.Goal{Title: title, Body: "task: " + spec, Project: filepath.Base(cwd), CWD: cwd, Status: goal.StatusEsborrany}, 0
}

// startItem posa la tasca en curs abans del bucle (o diu per què no pot).
func startItem(cwd string, id int) error {
	if id == 0 {
		return nil
	}
	_, _, err := goal.Start(cwd, id, "gregal --goal")
	return err
}

// finishItem apunta el resultat del bucle a la tasca. El bucle ja ha passat
// el verificador independent ronda a ronda: si el dona per complert, la
// tasca va a revisió amb aquell veredicte (superada); si no, queda en curs
// amb el que falta al log.
func finishItem(cwd string, id int, res agent.GoalResult) {
	if id == 0 {
		return
	}
	st, err := backlog.Open(cwd)
	if err != nil {
		return
	}
	var last agent.GoalRound
	if n := len(res.Rounds); n > 0 {
		last = res.Rounds[n-1]
	}
	if !res.Met {
		que := strings.TrimSpace(last.Verdict.Missing)
		if que == "" {
			que = last.Verdict.Reason
		}
		_, _ = st.AddLog(id, fmt.Sprintf("gregal --goal stopped (%s) after %d round(s): %s", res.Stop, len(res.Rounds), que), "gregal --goal")
		return
	}
	// Sense avís de revisió: el veredicte ja el porta el bucle.
	avis := backlog.OnInReview
	backlog.OnInReview = nil
	_, err = st.Move(id, backlog.StatusInReview, "", "gregal --goal")
	backlog.OnInReview = avis
	if err != nil {
		return
	}
	r := backlog.Review{Status: backlog.ReviewMet}
	for _, c := range last.Verdict.Criteria {
		r.Criteria = append(r.Criteria, backlog.ReviewCriterion{Text: c.Text, Status: c.Status, Evidence: c.Evidence})
	}
	for _, c := range last.Checks {
		r.Checks = append(r.Checks, backlog.ReviewCheck{Command: c.Command, Passed: c.Code == 0})
	}
	_, _ = st.SetReview(id, r, "reviewer")
}

// runGoalHeadless executa un objectiu en bucle fins que un verificador el
// confirma amb proves (agent.RunGoalLoop) o s'arriba a un límit. Torna el
// codi de sortida: 0 complert, 3 no complert (límit), 1 error.
func runGoalHeadless(cfg *config.Config, cfgPath string, f goalFlags) int {
	lang := cliLanguage(cfg)
	cwd, _ := os.Getwd()
	mode := f.Mode
	if mode == "" {
		// Un objectiu llarg no té ningú que aprovi: el mode autònom no pregunta mai.
		mode = agent.ModeAutonomous
	}
	if err := validaHeadless(cfg, f.Output, mode, f.AutoApprove, f.MaxSteps, lang); err != nil {
		fmt.Fprintln(os.Stderr, "gregal:", err)
		return exitUsage
	}
	if f.Output == "stream-json" {
		fmt.Fprintln(os.Stderr, "gregal: --goal supports --output text or json")
		return exitUsage
	}
	if cfgPath != "" {
		_, _ = goal.MigrateToBacklog(filepath.Dir(cfgPath))
	}
	g, item := resolGoal(cwd, f.Spec)
	if err := startItem(cwd, item); err != nil {
		fmt.Fprintf(os.Stderr, "gregal: backlog item #%d: %v\n", item, err)
		return exitUsage
	}

	mcp.Setup(toMCPServers(cfg))
	agent.SetPostEditHook(cfg.Hooks.PostEdit)
	agent.SetDiagMode(cfg.Hooks.Diag)
	agent.SetLifecycleHooks(cfg.Hooks)
	agent.SetupPrices(cfg)
	defer lsp.Default.Close()
	cli := llm.New()
	cfg.ConfigureClient(cli)
	agent.SetupDelegate(cfg, cli)

	var checks []string
	if c := strings.TrimSpace(f.VerifyCmd); c != "" {
		checks = []string{c}
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	fmt.Fprintf(os.Stderr, "goal: %s\n", g.Title)
	res := agent.RunGoalLoop(ctx, cli, cfg, agent.GoalLoopOpts{
		Run: agent.RunOpts{
			Mode: mode, MaxSteps: f.MaxSteps, HardStepLimit: f.MaxSteps > 0, AutoApprove: f.AutoApprove, Dir: cwd,
			OnStep: func(step int, tools []string) {
				fmt.Fprintln(os.Stderr, cliText(lang, "headless.step_tools", step, strings.Join(tools, ", ")))
			},
		},
		Goal: g, Checks: checks,
		MaxRounds: f.MaxRounds, MaxMinutes: f.MaxMinutes, MaxCostUSD: f.MaxCostUSD,
		OnProgress: func(s string) { fmt.Fprintln(os.Stderr, s) },
		OnRound: func(r agent.GoalRound) {
			fmt.Fprintf(os.Stderr, "round %d: %s\n", r.N, goalRoundLine(r))
		},
	})
	finishItem(cwd, item, res)
	if f.Output == "json" {
		raw, _ := json.MarshalIndent(res, "", "  ")
		fmt.Println(string(raw))
	} else {
		fmt.Println(res.Summary)
	}
	switch {
	case res.Met:
		return exitOK
	case res.Stop == agent.GoalStopError || res.Stop == agent.GoalStopCancelled || res.Stop == agent.GoalStopVerifier:
		return exitError
	}
	return exitVerifyFailed
}

func goalRoundLine(r agent.GoalRound) string {
	switch {
	case r.Verdict.Met:
		return "verifier: MET"
	case !r.Verified && len(r.Checks) > 0:
		return "a check failed, the verifier was not asked"
	case len(r.Verdict.Criteria) > 0:
		open := 0
		for _, c := range r.Verdict.Criteria {
			if c.Status != "MET" {
				open++
			}
		}
		return fmt.Sprintf("verifier: NOT MET (%d of %d criteria open)", open, len(r.Verdict.Criteria))
	}
	return "verifier: NOT MET (" + r.Verdict.Reason + ")"
}
