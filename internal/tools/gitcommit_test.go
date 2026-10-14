package tools

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

// Per defecte git_commit fa commit només del que l'agent ha canviat (foto
// d'inici de sessió vs ara, bash inclòs) i deixa fora el que l'usuari té
// staged en altres fitxers.
func TestGitCommitAgentChangedOnly(t *testing.T) {
	dir := tempRepo(t)
	writeTmp(t, filepath.Join(dir, "user.txt"), "user staged\n")
	gitT(t, dir, "add", "user.txt")
	old := GitCPs
	GitCPs = NewGitCheckpointStore(NewJournal())
	defer func() { GitCPs = old }()
	if _, err := GitCPs.Snapshot("o", dir, "turn", "t", 0); err != nil {
		t.Fatal(err)
	}
	writeTmp(t, filepath.Join(dir, "agent.txt"), "from bash\n")
	writeTmp(t, filepath.Join(dir, "a.txt"), "edited\n")

	out, err := GitCommitTool("o", dir, `{"message":"agent work"}`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "agent work") {
		t.Fatalf("out=%q", out)
	}
	files := gitT(t, dir, "show", "--name-only", "--format=", "HEAD")
	if !strings.Contains(files, "agent.txt") || !strings.Contains(files, "a.txt") || strings.Contains(files, "user.txt") {
		t.Fatalf("fitxers del commit: %q", files)
	}
	if got := gitT(t, dir, "diff", "--cached", "--name-only"); got != "user.txt" {
		t.Fatalf("el staged de l'usuari s'ha de conservar: %q", got)
	}
	if _, err := GitCommitTool("o", dir, `{"message":""}`); err == nil {
		t.Fatal("sense missatge hauria de fallar")
	}
	if _, err := GitCommitTool("o", dir, `{"message":"x","paths":["../fora.txt"]}`); err == nil {
		t.Fatal("camí fora del repo acceptat")
	}
}

// A la branca per defecte, gh_pr_create crea una branca nova abans del push
// i la PR surt d'allà.
func TestGHPRCreateNewBranchFromDefault(t *testing.T) {
	dir := tempRepo(t)
	bare := t.TempDir()
	gitT(t, bare, "init", "-q", "--bare")
	gitT(t, dir, "remote", "add", "origin", bare)
	gitT(t, dir, "push", "-q", "origin", "main")
	gitT(t, dir, "fetch", "-q", "origin")
	writeTmp(t, filepath.Join(dir, "f.txt"), "x\n")
	gitT(t, dir, "add", "f.txt")
	gitT(t, dir, "commit", "-qm", "feature")

	oldGH, oldPush := GHRunnerIn, GitPushRunner
	defer func() { GHRunnerIn, GitPushRunner = oldGH, oldPush }()
	var prArgs []string
	pushed := ""
	GHRunnerIn = func(ctx context.Context, d string, args ...string) (string, error) {
		if args[0] == "repo" {
			return "main\n", nil
		}
		prArgs = args
		return "https://github.com/o/r/pull/7\n", nil
	}
	GitPushRunner = func(ctx context.Context, root, remote, branch string) (string, error) {
		pushed = remote + " " + branch
		return "", nil
	}
	out, err := GHPRCreateTool(context.Background(), dir, `{"title":"Add F feature!","body":"desc","draft":true}`)
	if err != nil {
		t.Fatal(err)
	}
	if b := GitBranch(dir); b != "gregal/add-f-feature" {
		t.Fatalf("branca=%q", b)
	}
	if pushed != "origin gregal/add-f-feature" {
		t.Fatalf("push=%q", pushed)
	}
	joined := strings.Join(prArgs, " ")
	if !strings.Contains(joined, "--base main") || !strings.Contains(joined, "--head gregal/add-f-feature") || !strings.Contains(joined, "--draft") {
		t.Fatalf("gh args=%q", joined)
	}
	if !strings.Contains(out, "pull/7") {
		t.Fatalf("out=%q", out)
	}

	// Sense commits respecte a la base: refusat abans del push.
	gitT(t, dir, "switch", "-q", "-c", "empty", "origin/main")
	pushed = ""
	if _, err := GHPRCreateTool(context.Background(), dir, `{"title":"nothing","base":"main"}`); err == nil || pushed != "" {
		t.Fatalf("PR sense commits: err=%v push=%q", err, pushed)
	}
	GHRunnerIn = func(ctx context.Context, d string, args ...string) (string, error) { return "", errors.New("no gh") }
	if _, err := GHPRCreateTool(context.Background(), dir, `{"title":""}`); err == nil {
		t.Fatal("sense títol hauria de fallar")
	}
}
