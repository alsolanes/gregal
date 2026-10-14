package main

import (
	"fmt"
	"os"
	"strings"

	"gregal/internal/tools"
)

// entraWorktree implementa --worktree <nom>: crea (o reprèn) el worktree
// aïllat del repo del directori actual i hi entra. Buit = no fa res.
func entraWorktree(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil
	}
	wt, err := tools.WorktreeCreate("", name)
	if err != nil {
		return err
	}
	if err := os.Chdir(wt.Path); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "gregal: worktree %s · %s · branch %s\n", wt.Name, wt.Path, wt.Branch)
	return nil
}
