package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"gregal/internal/agent"
	"gregal/internal/config"
	"gregal/internal/llm"
)

// runMemory és `gregal memory tidy`: proposa una llista consolidada de les
// notes del projecte (.gregal/memory.md) amb el rol de xat, n'ensenya el
// diff i només l'escriu si l'usuari ho confirma (o amb --yes). Mai no
// escriu sense ensenyar abans el diff.
func runMemory(args []string) {
	if len(args) == 0 || args[0] != "tidy" {
		fmt.Fprintln(os.Stderr, "usage: gregal memory tidy [--config path] [--dir path] [--yes] [--dry-run]")
		os.Exit(2)
	}
	fs := flag.NewFlagSet("memory tidy", flag.ContinueOnError)
	cfgPath := fs.String("config", "", "configuration path")
	dir := fs.String("dir", "", "project directory (default: current directory)")
	yes := fs.Bool("yes", false, "write the tidied notes without asking (the diff is still shown)")
	dry := fs.Bool("dry-run", false, "show the diff and never write")
	if err := fs.Parse(args[1:]); err != nil {
		os.Exit(2)
	}
	cwd := *dir
	if cwd == "" {
		cwd, _ = os.Getwd()
	}
	before := config.Notes(cwd, "")
	if len(before) < 2 {
		fmt.Println("Nothing to tidy: fewer than two project notes in", config.ProjectMemoryPath(cwd))
		return
	}
	cfg, _, err := config.Load(*cfgPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "gregal memory: config:", err)
		os.Exit(1)
	}
	_, role, ok := agent.SideRole(cfg)
	p, known := cfg.Providers[role.Provider]
	if !ok || !known {
		fmt.Fprintln(os.Stderr, "gregal memory: no chat or code role with a configured provider")
		os.Exit(1)
	}
	client := llm.New()
	cfg.ConfigureClient(client)
	hash := config.NotesHash(cwd)
	after, err := agent.TidyMemory(context.Background(), client, cfg, p, role, before)
	if err != nil {
		fmt.Fprintln(os.Stderr, "gregal memory: tidy failed:", err)
		os.Exit(1)
	}
	if strings.Join(before, "\n") == strings.Join(after, "\n") {
		fmt.Println("Nothing to merge or drop.")
		return
	}
	fmt.Println(agent.NotesDiff(before, after))
	fmt.Printf("\n%d → %d notes\n", len(before), len(after))
	if *dry {
		return
	}
	if !*yes {
		fmt.Print("Write these notes to " + config.ProjectMemoryPath(cwd) + "? [y/N] ")
		line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
		if a := strings.ToLower(strings.TrimSpace(line)); a != "y" && a != "yes" && a != "s" && a != "si" && a != "sí" {
			fmt.Println("Not written.")
			return
		}
	}
	if config.NotesHash(cwd) != hash {
		fmt.Fprintln(os.Stderr, "gregal memory: the notes changed meanwhile; run tidy again")
		os.Exit(1)
	}
	if err := config.ReplaceNotes(cwd, after); err != nil {
		fmt.Fprintln(os.Stderr, "gregal memory:", err)
		os.Exit(1)
	}
	fmt.Println("Written.")
}
