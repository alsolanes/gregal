package tools

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func gitT(t *testing.T, dir string, args ...string) string {
	t.Helper()
	c := exec.Command("git", append([]string{"-C", dir}, args...)...)
	c.Env = append(os.Environ(), gitIdentityEnv...)
	out, err := c.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// tempRepo crea un repo amb un commit (a.txt, gone.txt, .gitignore).
func tempRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git no disponible")
	}
	dir := t.TempDir()
	gitT(t, dir, "init", "-q", "-b", "main")
	gitT(t, dir, "config", "core.autocrlf", "false")
	// Identitat al repo de prova: el codi provat (git_commit) crida git pel
	// seu compte i a la CI no hi ha cap identitat global.
	gitT(t, dir, "config", "user.name", "Gregal Test")
	gitT(t, dir, "config", "user.email", "test@example.com")
	writeTmp(t, filepath.Join(dir, "a.txt"), "a0\n")
	writeTmp(t, filepath.Join(dir, "gone.txt"), "keep me\n")
	writeTmp(t, filepath.Join(dir, ".gitignore"), "*.log\n")
	gitT(t, dir, "add", ".")
	gitT(t, dir, "commit", "-q", "-m", "init")
	return dir
}

// El cas que el journal no cobreix: canvis fets per bash. La foto d'inici
// de torn els desfà tots (modificat, esborrat, nou) sense tocar els
// ignorats, ni HEAD, ni l'índex de l'usuari, ni les branques.
func TestGitCheckpointRestoresBashChanges(t *testing.T) {
	dir := tempRepo(t)
	// L'usuari té feina a mig fer: un canvi staged i un fitxer nou.
	writeTmp(t, filepath.Join(dir, "a.txt"), "a1 staged\n")
	gitT(t, dir, "add", "a.txt")
	writeTmp(t, filepath.Join(dir, "mine.txt"), "user untracked\n")
	headBefore := gitT(t, dir, "rev-parse", "HEAD")
	branchesBefore := gitT(t, dir, "branch", "-a")
	idxPath := filepath.Join(dir, ".git", "index")
	idxBefore, _ := os.ReadFile(idxPath)

	st := NewGitCheckpointStore(NewJournal())
	cp, err := st.Snapshot("s1", dir, "turn", "turn 1", 0)
	if err != nil || cp == nil {
		t.Fatalf("snapshot: %v %v", cp, err)
	}
	if !strings.HasPrefix(cp.Ref, "refs/gregal/checkpoints/s1-") {
		t.Fatalf("ref %q", cp.Ref)
	}
	// "bash": modifica, esborra, crea (dins d'un directori nou) i un ignorat.
	writeTmp(t, filepath.Join(dir, "a.txt"), "sed -i result\n")
	os.Remove(filepath.Join(dir, "gone.txt"))
	writeTmp(t, filepath.Join(dir, "build", "out", "new.txt"), "generated\n")
	writeTmp(t, filepath.Join(dir, "debug.log"), "ignored\n")

	sum, err := st.Restore("s1", cp.N)
	if err != nil {
		t.Fatalf("restore: %v", err)
	}
	if got := readTmp(t, filepath.Join(dir, "a.txt")); got != "a1 staged\n" {
		t.Fatalf("a.txt=%q", got)
	}
	if got := readTmp(t, filepath.Join(dir, "gone.txt")); got != "keep me\n" {
		t.Fatalf("gone.txt=%q", got)
	}
	if _, err := os.Stat(filepath.Join(dir, "build", "out", "new.txt")); !os.IsNotExist(err) {
		t.Fatal("el fitxer creat per bash hauria d'haver desaparegut")
	}
	if _, err := os.Stat(filepath.Join(dir, "build")); !os.IsNotExist(err) {
		t.Fatal("els directoris buits creats després haurien de marxar")
	}
	if got := readTmp(t, filepath.Join(dir, "mine.txt")); got != "user untracked\n" {
		t.Fatalf("mine.txt=%q (existia a la foto: es conserva)", got)
	}
	if got := readTmp(t, filepath.Join(dir, "debug.log")); got != "ignored\n" {
		t.Fatal("els ignorats no es toquen")
	}
	if !strings.Contains(sum, "restaurats:") || !strings.Contains(sum, "esborrats:") {
		t.Fatalf("resum %q", sum)
	}
	if gitT(t, dir, "rev-parse", "HEAD") != headBefore {
		t.Fatal("HEAD ha canviat")
	}
	if gitT(t, dir, "branch", "-a") != branchesBefore {
		t.Fatal("branques canviades")
	}
	if idxAfter, _ := os.ReadFile(idxPath); string(idxAfter) != string(idxBefore) {
		t.Fatal("l'índex de l'usuari ha canviat")
	}
	if got := gitT(t, dir, "diff", "--cached", "--name-only"); got != "a.txt" {
		t.Fatalf("staged=%q", got)
	}
	if got := gitT(t, dir, "stash", "list"); got != "" {
		t.Fatalf("stash tocat: %q", got)
	}
}

// Les fotos de pas idèntiques no s'acumulen i el prune deixa N refs.
func TestGitCheckpointDedupAndPrune(t *testing.T) {
	dir := tempRepo(t)
	st := NewGitCheckpointStore(NewJournal())
	st.Keep = 3
	if _, err := st.Snapshot("s", dir, "step", "x", 0); err != nil {
		t.Fatal(err)
	}
	if cp, _ := st.Snapshot("s", dir, "step", "y", 0); cp != nil {
		t.Fatal("foto de pas idèntica: no se n'hauria de fer cap")
	}
	for i := 0; i < 5; i++ {
		writeTmp(t, filepath.Join(dir, "a.txt"), strings.Repeat("x", i+1))
		if _, err := st.Snapshot("s", dir, "turn", "t", i); err != nil {
			t.Fatal(err)
		}
	}
	refs := strings.Fields(gitT(t, dir, "for-each-ref", "--format=%(refname)", "refs/gregal/checkpoints/"))
	if len(refs) != 3 {
		t.Fatalf("refs=%d, volia 3", len(refs))
	}
	if got := len(st.List("s")); got != 3 {
		t.Fatalf("en memòria=%d, volia 3", got)
	}
}

// RewindTo combina journal i git: tornar a la foto del torn 2 desfà el
// write del torn 2 (journal) i el fitxer creat per bash (git), i dona la
// llargada de conversa del torn.
func TestGitCheckpointRewindToCombinesJournal(t *testing.T) {
	dir := tempRepo(t)
	j := NewJournal()
	st := NewGitCheckpointStore(j)
	if _, err := st.Snapshot("o", dir, "turn", "t1", 0); err != nil {
		t.Fatal(err)
	}
	a := filepath.Join(dir, "a.txt")
	j.SnapOp(a, "write")
	writeTmp(t, a, "turn1 write\n")
	cp2, err := st.Snapshot("o", dir, "turn", "t2", 2)
	if err != nil || cp2 == nil {
		t.Fatal(err)
	}
	j.SnapOp(a, "write")
	writeTmp(t, a, "turn2 write\n")
	writeTmp(t, filepath.Join(dir, "bash.txt"), "x")
	sum, convo, err := st.RewindTo("o", cp2.N, nil)
	if err != nil {
		t.Fatal(err)
	}
	if convo != 2 {
		t.Fatalf("convo=%d", convo)
	}
	if got := readTmp(t, a); got != "turn1 write\n" {
		t.Fatalf("a=%q", got)
	}
	if _, err := os.Stat(filepath.Join(dir, "bash.txt")); !os.IsNotExist(err) {
		t.Fatalf("bash.txt hauria de marxar (%s)", sum)
	}
	if j.Seq() != 1 {
		t.Fatalf("journal seq=%d", j.Seq())
	}
	// Rewind total: torna a l'estat d'abans de la sessió.
	if _, err := st.RewindAll("o", dir, nil); err != nil {
		t.Fatal(err)
	}
	if got := readTmp(t, a); got != "a0\n" {
		t.Fatalf("a=%q", got)
	}
}

// Workspace en un subdirectori: la foto i la restauració no surten d'allà.
func TestGitCheckpointSubdirScope(t *testing.T) {
	dir := tempRepo(t)
	sub := filepath.Join(dir, "sub")
	writeTmp(t, filepath.Join(sub, "s.txt"), "s0")
	st := NewGitCheckpointStore(NewJournal())
	cp, err := st.Snapshot("o", sub, "turn", "t", 0)
	if err != nil || cp == nil {
		t.Fatal(err)
	}
	writeTmp(t, filepath.Join(sub, "s.txt"), "s1")
	writeTmp(t, filepath.Join(dir, "outside.txt"), "fora")
	if _, err := st.Restore("o", cp.N); err != nil {
		t.Fatal(err)
	}
	if got := readTmp(t, filepath.Join(sub, "s.txt")); got != "s0" {
		t.Fatalf("s.txt=%q", got)
	}
	if _, err := os.Stat(filepath.Join(dir, "outside.txt")); err != nil {
		t.Fatal("un fitxer fora del workspace no s'ha de tocar")
	}
}

func TestGitCheckpointOutsideGit(t *testing.T) {
	st := NewGitCheckpointStore(NewJournal())
	if cp, err := st.Snapshot("o", t.TempDir(), "turn", "t", 0); cp != nil || err != nil {
		t.Fatalf("fora de git: %v %v", cp, err)
	}
}
