package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"golang.org/x/term"

	"gregal/internal/config"
	"gregal/internal/mcpimport"
)

// runMCPImport implementa `gregal mcp import`: llegeix els servidors MCP de
// Claude Code, Codex i opencode, ensenya què s'afegiria (sense secrets) i
// només escriu al config si la persona ho confirma (o amb --yes).
func runMCPImport(args []string) {
	fs := flag.NewFlagSet("mcp import", flag.ContinueOnError)
	cfgPath := fs.String("config", "", "configuration path (default ~/.config/gregal/config.yaml)")
	from := fs.String("from", "all", "where to read from: claude, codex, opencode (comma separated) or all")
	only := fs.String("only", "", "import only these server names (comma separated)")
	yes := fs.Bool("yes", false, "write without asking")
	dry := fs.Bool("dry-run", false, "only show what would be added")
	if err := fs.Parse(args); err != nil {
		os.Exit(2)
	}
	cfg, path, err := config.Load(*cfgPath)
	if err != nil {
		fatalCfg(err, configLanguage(path))
	}
	home, _ := os.UserHomeDir()
	cwd, _ := os.Getwd()
	res := mcpimport.Discover(home, cwd, strings.Split(*from, ",")...)
	plan := mcpimport.MakePlan(cfg.MCP, res.Found)
	fmt.Print(mcpimport.Report(res, plan))
	if len(plan.Add) == 0 {
		fmt.Println("\nNothing to import.")
		return
	}
	if *dry {
		return
	}
	var pick []string
	if s := strings.TrimSpace(*only); s != "" {
		pick = strings.Split(s, ",")
		for i := range pick {
			pick[i] = strings.TrimSpace(pick[i])
		}
	}
	if !*yes {
		if !term.IsTerminal(int(os.Stdin.Fd())) {
			fmt.Fprintln(os.Stderr, "\nNot writing anything: run in a terminal to confirm, or pass --yes.")
			os.Exit(2)
		}
		fmt.Printf("\nAdd these to %s? Environment values and headers are copied as they are into that file. [y/N] ", path)
		if !readYes() {
			fmt.Println("Cancelled; nothing written.")
			return
		}
	}
	added := mcpimport.Apply(cfg, plan, pick)
	if len(added) == 0 {
		fmt.Println("None of the --only names matched; nothing written.")
		return
	}
	if err := cfg.Save(path); err != nil {
		fmt.Fprintln(os.Stderr, "gregal: not saved:", err)
		os.Exit(1)
	}
	fmt.Printf("Added %d server(s) to %s: %s\nThey connect the next time Gregal starts (or with /mcp in the TUI).\n", len(added), path, strings.Join(added, ", "))
}
