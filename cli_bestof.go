package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"strings"

	"gregal/internal/agent"
	"gregal/internal/config"
	"gregal/internal/llm"
	"gregal/internal/mcp"
)

// runBestOfHeadless és `gregal -p "…" --best-of N`: la tasca N vegades en
// worktrees separats, verificació i puntuació de cada intent, i la taula de
// comparació. Els worktrees queden vius perquè l'usuari triï (la web els
// aplica amb /api/bestof/apply; aquí s'imprimeixen les ordres git). Codi 0
// si almenys un intent ha acabat bé.
func runBestOfHeadless(cfg *config.Config, task, output string, n int, models string, autoApprove bool, maxSteps int, mode, verifyCmd string, apply bool) int {
	if n < agent.BestOfMin || n > agent.BestOfMax {
		fmt.Fprintf(os.Stderr, "gregal: --best-of must be between %d and %d (got %d)\n", agent.BestOfMin, agent.BestOfMax, n)
		return exitUsage
	}
	if mode == "" {
		mode = cfg.Mode
	}
	if maxSteps == 0 {
		maxSteps = cfg.Agent.MaxSteps
	}
	var specs []string
	for _, m := range strings.Split(models, ",") {
		if m = strings.TrimSpace(m); m != "" {
			specs = append(specs, m)
		}
	}
	var verify []string
	if strings.TrimSpace(verifyCmd) != "" {
		verify = []string{verifyCmd}
	}
	mcp.Setup(toMCPServers(cfg))
	agent.SetPostEditHook(cfg.Hooks.PostEdit)
	agent.SetDiagMode(cfg.Hooks.Diag)
	agent.SetLifecycleHooks(cfg.Hooks)
	agent.SetupPrices(cfg)
	cli := llm.New()
	cfg.ConfigureClient(cli)
	agent.SetupDelegate(cfg, cli)
	cwd, _ := os.Getwd()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	res, err := agent.RunBestOf(ctx, cli, cfg, agent.BestOfOpts{
		Task: task, N: n, Models: specs, Dir: cwd, Mode: mode, MaxSteps: maxSteps,
		AutoApprove: autoApprove, Verify: verify, Score: agent.BestOfScorer(cli, cfg),
		OnUpdate: func(a agent.BestOfAttempt) {
			fmt.Fprintf(os.Stderr, "best-of #%d (%s): %s\n", a.Index, a.Model, a.State)
		},
	})
	if output == "json" {
		raw, _ := json.MarshalIndent(res, "", "  ")
		fmt.Println(string(raw))
	} else if len(res.Attempts) > 0 {
		fmt.Print(agent.RenderBestOf(res))
		if res.Recommended > 0 {
			a := res.Attempts[res.Recommended-1]
			fmt.Printf("\nrecommended: #%d\napply it (working tree only, no commit):\n  git -C %q add -A && git -C %q diff --cached --binary HEAD | git apply\n", a.Index, a.Path, a.Path)
		}
		fmt.Println("discard an attempt: git worktree remove --force <path> && git branch -D <branch>")
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "gregal:", err)
		return exitError
	}
	if apply {
		if res.Recommended <= 0 {
			fmt.Fprintln(os.Stderr, "gregal: no attempt to apply (none finished with changes)")
			return exitError
		}
		msg, _, aerr := agent.BestOfApply(cwd, res.Attempts, res.Recommended, false)
		if aerr != nil {
			fmt.Fprintln(os.Stderr, "gregal: apply:", aerr)
			return exitError
		}
		fmt.Fprintln(os.Stderr, msg)
	}
	for _, a := range res.Attempts {
		if a.State == agent.BestOfDone {
			return exitOK
		}
	}
	return exitError
}
