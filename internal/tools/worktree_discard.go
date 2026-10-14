package tools

import (
	"fmt"
	"os"
)

// WorktreeDiscard llença un worktree de gregal amb la seva feina: `git
// worktree remove --force` i `git branch -D` de la seva branca. És per als
// worktrees d'un sol ús (els intents de best-of), on la feina que no s'ha
// triat no s'ha de conservar. WorktreeRemove continua sent el camí prudent
// per als worktrees de sessió (mai --force).
func WorktreeDiscard(dir, name string) error {
	main, err := MainRepoRoot(dir)
	if err != nil {
		return err
	}
	wt, ok := worktreeByName(main, name)
	if !ok {
		return fmt.Errorf("worktree %q inexistent", name)
	}
	if _, err := gitRun(main, nil, nil, "worktree", "remove", "--force", wt.Path); err != nil {
		// Directori ja esborrat a mà o bloquejat: que git oblidi el registre.
		_ = os.RemoveAll(wt.Path)
		if _, perr := gitRun(main, nil, nil, "worktree", "prune"); perr != nil {
			return fmt.Errorf("no s'ha pogut treure el worktree %s: %w", name, err)
		}
	}
	if wt.Branch != "" {
		_, _ = gitRun(main, nil, nil, "branch", "-D", wt.Branch)
	}
	return nil
}
