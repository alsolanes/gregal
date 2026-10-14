package tools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWorktreeCreateListDiffApplyRemove(t *testing.T) {
	dir := tempRepo(t)
	writeTmp(t, filepath.Join(dir, ".env"), "SECRET=1\n")
	writeTmp(t, filepath.Join(dir, ".gitignore"), "*.log\n.env\n")
	gitT(t, dir, "commit", "-qam", "ignore env")
	writeTmp(t, filepath.Join(dir, ".worktreeinclude"), "# fitxers locals\n.env\n")
	headBefore := gitT(t, dir, "rev-parse", "HEAD")

	wt, err := WorktreeCreate(dir, "feat-x")
	if err != nil {
		t.Fatal(err)
	}
	if wt.Branch != "gregal/feat-x" || !strings.HasSuffix(filepath.ToSlash(wt.Path), ".gregal/worktrees/feat-x") {
		t.Fatalf("wt=%+v", wt)
	}
	if got := readTmp(t, filepath.Join(wt.Path, ".env")); got != "SECRET=1\n" {
		t.Fatalf(".env no copiat: %q", got)
	}
	if WorktreeNameOf(filepath.Join(wt.Path, "sub")) != "feat-x" {
		t.Fatal("WorktreeNameOf")
	}
	if got, want := filepath.Clean(WorktreeRepoOf(filepath.Join(wt.Path, "sub"))), filepath.Clean(dir); !strings.EqualFold(filepath.ToSlash(got), filepath.ToSlash(want)) {
		t.Fatalf("WorktreeRepoOf = %q, vull %q", got, want)
	}
	if WorktreeRepoOf(dir) != "" || !InGitRepo(wt.Path) || InGitRepo(t.TempDir()) {
		t.Fatal("WorktreeRepoOf/InGitRepo fora d'un worktree")
	}
	// El repo principal no veu el worktree com a fitxer nou.
	if st := gitT(t, dir, "status", "--porcelain"); strings.Contains(st, ".gregal") {
		t.Fatalf("status principal: %q", st)
	}
	// Tornar-lo a crear amb el mateix nom el reutilitza.
	if again, err := WorktreeCreate(dir, "feat-x"); err != nil || again.Path != wt.Path {
		t.Fatalf("recrear: %v %v", again, err)
	}
	if _, err := WorktreeCreate(dir, "../evil"); err == nil {
		t.Fatal("nom invàlid acceptat")
	}

	// Feina al worktree: un fitxer modificat i un de nou, sense commit.
	writeTmp(t, filepath.Join(wt.Path, "a.txt"), "from worktree\n")
	writeTmp(t, filepath.Join(wt.Path, "new.txt"), "nou\n")
	list, err := WorktreeList(dir)
	if err != nil || len(list) != 1 || !list[0].Dirty {
		t.Fatalf("list=%+v err=%v", list, err)
	}
	patch, _, err := WorktreeDiff(dir, "feat-x")
	if err != nil || !strings.Contains(patch, "new.txt") || !strings.Contains(patch, "+from worktree") {
		t.Fatalf("diff: %v\n%s", err, patch)
	}
	// merge sense commit: refusat.
	if _, err := WorktreeApply(dir, "feat-x", "merge"); err == nil {
		t.Fatal("merge amb canvis sense commit hauria de fallar")
	}
	out, err := WorktreeApply(dir, "feat-x", "patch")
	if err != nil {
		t.Fatalf("patch: %v", err)
	}
	if got := readTmp(t, filepath.Join(dir, "a.txt")); got != "from worktree\n" {
		t.Fatalf("a.txt principal=%q (%s)", got, out)
	}
	if got := readTmp(t, filepath.Join(dir, "new.txt")); got != "nou\n" {
		t.Fatalf("new.txt principal=%q", got)
	}
	if gitT(t, dir, "rev-parse", "HEAD") != headBefore {
		t.Fatal("el pegat no ha de fer commit")
	}
	if got := gitT(t, dir, "diff", "--cached", "--name-only"); got != "" {
		t.Fatalf("el pegat no ha de tocar l'índex: %q", got)
	}
	// Treure amb canvis: git s'hi nega (mai --force).
	if _, err := WorktreeRemove(dir, "feat-x", false); err == nil {
		t.Fatal("remove d'un worktree brut hauria de fallar")
	}
	gitT(t, wt.Path, "add", "-A")
	gitT(t, wt.Path, "commit", "-qm", "wt work")
	msg, err := WorktreeRemove(dir, "feat-x", true)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(msg, "es conserva") {
		t.Fatalf("branca no fusionada s'hauria de conservar: %s", msg)
	}
	if _, err := os.Stat(wt.Path); !os.IsNotExist(err) {
		t.Fatal("el directori del worktree hauria d'haver marxat")
	}
}

func TestWorktreeMerge(t *testing.T) {
	dir := tempRepo(t)
	wt, err := WorktreeCreate(dir, "m1")
	if err != nil {
		t.Fatal(err)
	}
	writeTmp(t, filepath.Join(wt.Path, "merged.txt"), "m\n")
	gitT(t, wt.Path, "add", "-A")
	gitT(t, wt.Path, "commit", "-qm", "feature")
	// Principal brut (fitxer seguit modificat): refusat.
	writeTmp(t, filepath.Join(dir, "a.txt"), "dirty\n")
	if _, err := WorktreeApply(dir, "m1", "merge"); err == nil {
		t.Fatal("merge amb el principal brut hauria de fallar")
	}
	gitT(t, dir, "checkout", "--", "a.txt")
	if _, err := WorktreeApply(dir, "m1", "merge"); err != nil {
		t.Fatal(err)
	}
	if got := readTmp(t, filepath.Join(dir, "merged.txt")); got != "m\n" {
		t.Fatalf("merged.txt=%q", got)
	}
	if msg, err := WorktreeRemove(dir, "m1", true); err != nil || !strings.Contains(msg, "esborrada") {
		t.Fatalf("remove: %q %v", msg, err)
	}
}

// Amb un worktree creat (i .gregal/worktrees a info/exclude), la foto del
// repo principal continua funcionant i no hi entra el worktree.
func TestSnapshotWithWorktreePresent(t *testing.T) {
	dir := tempRepo(t)
	if _, err := WorktreeCreate(dir, "w"); err != nil {
		t.Fatal(err)
	}
	st := NewGitCheckpointStore(NewJournal())
	cp, err := st.Snapshot("o", dir, "turn", "t", 0)
	if err != nil || cp == nil {
		t.Fatalf("snapshot amb worktree: %v", err)
	}
	if out := gitT(t, dir, "ls-tree", "-r", "--name-only", cp.Commit); strings.Contains(out, ".gregal") {
		t.Fatalf("el worktree ha entrat a la foto: %s", out)
	}
	writeTmp(t, filepath.Join(dir, "x.txt"), "x")
	if _, err := st.Restore("o", cp.N); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".gregal", "worktrees", "w", "a.txt")); err != nil {
		t.Fatal("restaurar no pot tocar el worktree")
	}
}
