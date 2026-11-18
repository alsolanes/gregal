package agent

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"gregal/internal/backlog"
	"gregal/internal/config"
	"gregal/internal/llm"
)

// repoAmbTasca fa un repo amb un commit, una tasca a revisió amb Base en
// aquell commit i un canvi fet després.
func repoAmbTasca(t *testing.T) (string, *backlog.Store) {
	t.Helper()
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	dir := t.TempDir()
	git := func(args ...string) {
		cmd := exec.Command("git", append([]string{"-c", "user.email=t@t", "-c", "user.name=t"}, args...)...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	escriu := func(nom, text string) {
		if err := os.WriteFile(filepath.Join(dir, nom), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	ok := func(_ backlog.Item, err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	git("init", "-q")
	escriu("README", "x\n")
	git("add", ".")
	git("commit", "-q", "-m", "init")
	st, err := backlog.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	ok(st.Create(backlog.Item{Type: "task", Title: "Greeting", Why: "w", Criteria: []backlog.Criterion{{Text: "hello.txt says hola"}, {Text: "README mentions it"}}}, ""))
	ok(st.Move(1, "ready", "", ""))
	ok(st.Move(1, "in_progress", "", ""))
	ok(st.SetWork(1, dir, GitHead(dir)))
	escriu("hello.txt", "hola\n")
	git("add", "hello.txt")
	git("commit", "-q", "-m", "hello")
	ok(st.Check(1, 1, true, "cat hello.txt", "agent"))
	ok(st.Check(1, 2, true, "edited README", "agent"))
	ok(st.Move(1, "in_review", "", "agent"))
	return dir, st
}

func ambVerificador(t *testing.T, resposta string, vist *string) {
	t.Helper()
	abans := backlogVerifier
	backlogVerifier = func(ctx context.Context, _ *llm.Client, _ *config.Config, dir, prompt string) (string, error) {
		*vist = prompt
		return resposta, nil
	}
	t.Cleanup(func() { backlogVerifier = abans })
}

func TestRevisioDelBacklogNoSuperada(t *testing.T) {
	dir, st := repoAmbTasca(t)
	var prompt string
	ambVerificador(t, "VERDICT: NOT MET\nCRITERIA:\n- [MET] hello.txt says hola :: hello.txt line 1\n- [NOT MET] README mentions it :: README unchanged in the diff\nMISSING: mention hello.txt in the README", &prompt)
	it, err := ReviewBacklogItem(context.Background(), nil, &config.Config{}, dir, 1)
	if err != nil {
		t.Fatal(err)
	}
	// El revisor ha vist el commit fet després de començar i les afirmacions com a afirmacions.
	for _, want := range []string{"CHANGES SINCE THE WORK STARTED", "hello.txt", "+hola", "claims to check, not evidence", "1. hello.txt says hola"} {
		if !strings.Contains(prompt, want) {
			t.Errorf("al revisor li falta %q", want)
		}
	}
	if it.Status != backlog.StatusInProgress || it.Review.Status != backlog.ReviewNotMet || it.Review.Summary != "mention hello.txt in the README" {
		t.Fatalf("%+v %+v", it.Status, it.Review)
	}
	if !it.Criteria[0].Done || it.Criteria[1].Done {
		t.Fatalf("només es desmarca el que no s'ha demostrat: %+v", it.Criteria)
	}
	_ = st
}

func TestRevisioDelBacklogSuperada(t *testing.T) {
	dir, st := repoAmbTasca(t)
	var prompt string
	ambVerificador(t, "VERDICT: MET\nCRITERIA:\n- [MET] hello.txt says hola :: hello.txt: hola\n- [MET] README mentions it :: README line 2\nMISSING: none", &prompt)
	it, err := ReviewBacklogItem(context.Background(), nil, &config.Config{}, dir, 1)
	if err != nil || it.Status != backlog.StatusInReview || it.Review.Status != backlog.ReviewMet {
		t.Fatalf("%v %+v", err, it.Review)
	}
	if _, err := st.Move(1, "done", "", "you"); err != nil {
		t.Fatalf("superada, es pot tancar: %v", err)
	}
}

func TestRevisioSenseVeredicteNoAprova(t *testing.T) {
	dir, _ := repoAmbTasca(t)
	var prompt string
	ambVerificador(t, "Looks good to me!", &prompt)
	it, _ := ReviewBacklogItem(context.Background(), nil, &config.Config{}, dir, 1)
	if it.Review.Status != backlog.ReviewNotMet || it.Status != backlog.StatusInProgress {
		t.Fatalf("sense veredicte explícit no hi ha aprovació: %+v", it.Review)
	}
}

func TestRevisioAmbComprovacioQueFalla(t *testing.T) {
	dir, _ := repoAmbTasca(t)
	cridat := ""
	ambVerificador(t, "VERDICT: MET", &cridat)
	cfg := &config.Config{Agent: config.AgentCfg{Autonomous: config.AutonomousCfg{Verify: []string{"exit 3"}}}}
	it, _ := ReviewBacklogItem(context.Background(), nil, cfg, dir, 1)
	if cridat != "" {
		t.Fatal("amb una comprovació que falla no cal el model")
	}
	if it.Review.Status != backlog.ReviewNotMet || !strings.Contains(it.Review.Summary, "exit 3") || len(it.Review.Checks) != 1 || it.Review.Checks[0].Passed {
		t.Fatalf("%+v", it.Review)
	}
}

func TestComandaDeComprovacioDelProjecte(t *testing.T) {
	dir := t.TempDir()
	if got := BacklogChecks(nil, dir); got != nil {
		t.Fatalf("sense res: %v", got)
	}
	if err := os.WriteFile(filepath.Join(dir, "Makefile"), []byte("build:\n\tgo build\ncheck: build\n\tgo test ./...\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := BacklogChecks(nil, dir); len(got) != 1 || got[0] != "make check" {
		t.Fatalf("%v", got)
	}
	cfg := &config.Config{Agent: config.AgentCfg{Autonomous: config.AutonomousCfg{Verify: []string{"go vet ./..."}}}}
	if got := BacklogChecks(cfg, dir); got[0] != "go vet ./..." {
		t.Fatalf("el config mana: %v", got)
	}
}
