package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"

	"gregal/internal/agent"
	"gregal/internal/config"
	"gregal/internal/goal"
	"gregal/internal/llm"
	"gregal/internal/lsp"
	"gregal/internal/mcp"
)

// goalFlags són els flags de `gregal --goal`.
type goalFlags struct {
	Spec        string // id d'un objectiu desat, "last" o el text de l'objectiu
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

// resolGoal torna l'objectiu desat que diu spec, o en fa un d'inline amb el
// text. saved diu si ve del fitxer d'objectius (i es pot marcar com a fet).
func resolGoal(dir, project, cwd, spec string) (g goal.Goal, saved bool) {
	spec = strings.TrimSpace(spec)
	if spec == "last" {
		if l, err := goal.List(dir, project); err == nil && len(l) > 0 {
			return l[0], true
		}
	} else if got, err := goal.Get(dir, spec); err == nil {
		return got, true
	}
	title := strings.Join(strings.Fields(spec), " ")
	if r := []rune(title); len(r) > 80 {
		title = string(r[:80]) + "…"
	}
	return goal.Goal{Title: title, Body: "task: " + spec, Project: project, CWD: cwd, Status: goal.StatusEsborrany}, false
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
	goalDir := ""
	if cfgPath != "" {
		goalDir = filepath.Dir(cfgPath)
	}
	g, saved := resolGoal(goalDir, filepath.Base(cwd), cwd, f.Spec)

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
	if res.Met && saved && g.ID != "" {
		g.Status = goal.StatusFet
		_ = goal.Save(goalDir, g)
	}
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
