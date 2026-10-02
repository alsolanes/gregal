package tools

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func gitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	run("init", "-q")
	base := "un\ndos\ntres\nquatre\ncinc\nsis\nset\nvuit\nnou\ndeu\nonze\ndotze\ntretze\ncatorze\nquinze\nsetze\n"
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte(base), 0o644)
	run("add", "-A")
	run("-c", "user.email=t@t", "-c", "user.name=t", "commit", "-qm", "base")
	return dir
}

// Un canvi al mig surt com un hunk amb línia esborrada i afegida.
func TestGitDiffStructured(t *testing.T) {
	dir := gitRepo(t)
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("un\ndos\nTRES\nquatre\ncinc\nsis\nset\nvuit\nnou\ndeu\nonze\ndotze\ntretze\ncatorze\nquinze\nsetze\n"), 0o644)
	files, err := GitDiffStructured(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files[0].Path != "a.txt" {
		t.Fatalf("fitxers: %+v", files)
	}
	f := files[0]
	if f.Added != 1 || f.Removed != 1 || len(f.Hunks) != 1 {
		t.Fatalf("comptadors: +%d -%d hunks=%d", f.Added, f.Removed, len(f.Hunks))
	}
	h := f.Hunks[0]
	if !strings.HasPrefix(h.Header, "@@") || !strings.Contains(h.Patch, "+TRES") {
		t.Fatalf("hunk: %+v", h)
	}
	var kinds []string
	for _, l := range h.Lines {
		kinds = append(kinds, l.Kind)
	}
	if !strings.Contains(strings.Join(kinds, ","), "del,add") {
		t.Fatalf("línies: %v", kinds)
	}
}

// Els fitxers nous hi surten com a alta sencera, sense necessitat de git add.
func TestGitDiffUntracked(t *testing.T) {
	dir := gitRepo(t)
	os.WriteFile(filepath.Join(dir, "nou.txt"), []byte("hola\nadeu\n"), 0o644)
	files, _ := GitDiffStructured(dir)
	var found *DiffFile
	for i := range files {
		if files[i].Path == "nou.txt" {
			found = &files[i]
		}
	}
	if found == nil || !found.Untracked || found.Added != 2 {
		t.Fatalf("untracked: %+v", files)
	}
}

// Descartar un hunk retorna el fitxer a l'estat anterior.
func TestGitApplyReverseHunk(t *testing.T) {
	dir := gitRepo(t)
	orig, _ := os.ReadFile(filepath.Join(dir, "a.txt"))
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("un\ndos\nTRES\nquatre\ncinc\nsis\nset\nvuit\nnou\ndeu\nonze\ndotze\ntretze\ncatorze\nquinze\nsetze\n"), 0o644)
	files, _ := GitDiffStructured(dir)
	if err := GitApplyReverse(dir, files[0].Hunks[0].Patch); err != nil {
		t.Fatal(err)
	}
	now, _ := os.ReadFile(filepath.Join(dir, "a.txt"))
	if string(now) != string(orig) {
		t.Fatalf("no s'ha restaurat:\n%s", now)
	}
}

// Dos hunks separats: descartar-ne un deixa l'altre intacte.
func TestGitApplyReverseNomesUnHunk(t *testing.T) {
	dir := gitRepo(t)
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("UN\ndos\ntres\nquatre\ncinc\nsis\nset\nvuit\nnou\ndeu\nonze\ndotze\ntretze\ncatorze\nquinze\nSETZE\n"), 0o644)
	files, _ := GitDiffStructured(dir)
	if len(files[0].Hunks) != 2 {
		t.Fatalf("volia 2 hunks, tinc %d", len(files[0].Hunks))
	}
	if err := GitApplyReverse(dir, files[0].Hunks[0].Patch); err != nil {
		t.Fatal(err)
	}
	now, _ := os.ReadFile(filepath.Join(dir, "a.txt"))
	if !strings.HasPrefix(string(now), "un\n") || !strings.Contains(string(now), "SETZE") {
		t.Fatalf("s'havia de desfer només el primer hunk:\n%s", now)
	}
}

// Sense repo git no peta: llista buida.
func TestGitDiffSenseRepo(t *testing.T) {
	files, err := GitDiffStructured(t.TempDir())
	if err != nil || len(files) != 0 {
		t.Fatalf("%v %v", files, err)
	}
}

// Amb core.autocrlf=true (el que ve de sèrie al Git de Windows) descartar un
// hunk reescrivia tot el fitxer amb CRLF: una línia canviada en deixava setze
// de modificades. El pegat ha de tocar només el que diu.
func TestGitApplyReverseNoToquElsFinalsDeLinia(t *testing.T) {
	dir := gitRepo(t)
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	run("config", "core.autocrlf", "true")

	orig, _ := os.ReadFile(filepath.Join(dir, "a.txt"))
	if bytes.Contains(orig, []byte("\r\n")) {
		t.Fatal("la fixture ha de ser LF pur")
	}
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("un\ndos\nTRES\nquatre\ncinc\nsis\nset\nvuit\nnou\ndeu\nonze\ndotze\ntretze\ncatorze\nquinze\nsetze\n"), 0o644)

	files, _ := GitDiffStructured(dir)
	if err := GitApplyReverse(dir, files[0].Hunks[0].Patch); err != nil {
		t.Fatal(err)
	}
	now, _ := os.ReadFile(filepath.Join(dir, "a.txt"))
	if bytes.Contains(now, []byte("\r\n")) {
		t.Fatalf("descartar un hunk ha convertit el fitxer a CRLF:\n%q", now)
	}
	if !bytes.Equal(now, orig) {
		t.Fatalf("byte a byte havia de tornar a l'original:\n%q", now)
	}
}
