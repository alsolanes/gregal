package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"gregal/internal/tools"
)

func gitW(t *testing.T, dir string, args ...string) string {
	t.Helper()
	c := exec.Command("git", append([]string{"-C", dir}, args...)...)
	c.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
	out, err := c.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// gitTestServer: servidor amb el workspace en un repo temporal i els
// checkpoints git encesos només durant el test (mai sobre el repo real).
func gitTestServer(t *testing.T) (*Server, string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git no disponible")
	}
	dir := t.TempDir()
	gitW(t, dir, "init", "-q", "-b", "main")
	gitW(t, dir, "config", "core.autocrlf", "false")
	// Identitat al repo de prova: el commit el fa el servidor, no el helper,
	// i a la CI no hi ha cap identitat global.
	gitW(t, dir, "config", "user.name", "Gregal Test")
	gitW(t, dir, "config", "user.email", "test@example.com")
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitW(t, dir, "add", ".")
	gitW(t, dir, "commit", "-qm", "init")
	s := goalTestServer(t)
	s.Hub()
	s.mu.Lock()
	s.cwd = dir
	s.mu.Unlock()
	old := tools.GitCPs
	tools.GitCPs = tools.NewGitCheckpointStore(tools.Active)
	t.Cleanup(func() { tools.GitCPs = old })
	return s, dir
}

func getJSON(t *testing.T, h http.HandlerFunc, url string, v any) int {
	t.Helper()
	rec := httptest.NewRecorder()
	h(rec, httptest.NewRequest(http.MethodGet, url, nil))
	if v != nil && rec.Code == 200 {
		if err := json.Unmarshal(rec.Body.Bytes(), v); err != nil {
			t.Fatal(err)
		}
	}
	return rec.Code
}

// /api/checkpoints llista les fotos git i /api/rewind-to {"git":N} desfà
// també el que ha fet bash.
func TestGitCheckpointsAPI(t *testing.T) {
	s, dir := gitTestServer(t)
	cp, err := tools.GitCPs.Snapshot(s.id, dir, "turn", "before turn", 0)
	if err != nil || cp == nil {
		t.Fatalf("snapshot: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "bash.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	var list []map[string]any
	if code := getJSON(t, s.handleCheckpoints, "/api/checkpoints", &list); code != 200 {
		t.Fatalf("checkpoints=%d", code)
	}
	var git float64
	for _, e := range list {
		if e["op"] == "snapshot" {
			git, _ = e["git"].(float64)
		}
	}
	if int(git) != cp.N {
		t.Fatalf("la foto no surt al llistat: %v", list)
	}
	rec := postJSON(t, s.handleRewindTo, `{"git":`+jsonNum(cp.N)+`}`)
	if rec.Code != 200 {
		t.Fatalf("rewind-to git=%d %s", rec.Code, rec.Body.String())
	}
	if _, err := os.Stat(filepath.Join(dir, "bash.txt")); !os.IsNotExist(err) {
		t.Fatal("bash.txt hauria d'haver desaparegut")
	}
	if rec := postJSON(t, s.handleRewindTo, `{"git":999}`); rec.Code != 400 {
		t.Fatalf("foto inexistent=%d", rec.Code)
	}
}

func jsonNum(n int) string { b, _ := json.Marshal(n); return string(b) }

func TestWorktreesAPI(t *testing.T) {
	s, dir := gitTestServer(t)
	rec := postJSON(t, s.handleWorktrees, `{"name":"w1"}`)
	if rec.Code != 200 {
		t.Fatalf("crear=%d %s", rec.Code, rec.Body.String())
	}
	var created struct {
		Worktree tools.Worktree    `json:"worktree"`
		Session  map[string]string `json:"session"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.Worktree.Branch != "gregal/w1" || created.Session["id"] == "" || created.Session["worktree"] != "w1" {
		t.Fatalf("resposta=%s", rec.Body.String())
	}
	// La pestanya duu la marca del worktree.
	found := false
	for _, item := range s.hub.listFor(nil) {
		if item["id"] == created.Session["id"] && item["worktree"] == "w1" {
			found = true
		}
	}
	if !found {
		t.Fatal("la sessió nova no surt amb worktree=w1")
	}
	var list struct {
		Worktrees []tools.Worktree `json:"worktrees"`
	}
	if code := getJSON(t, s.handleWorktrees, "/api/worktrees", &list); code != 200 || len(list.Worktrees) != 1 {
		t.Fatalf("llista=%d %+v", code, list)
	}
	// Feina al worktree → diff → pegat al principal.
	if err := os.WriteFile(filepath.Join(created.Worktree.Path, "a.txt"), []byte("wt\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rec = postJSON(t, s.handleWorktreeApply, `{"name":"w1","how":"diff"}`)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "+wt") {
		t.Fatalf("diff=%d %s", rec.Code, rec.Body.String())
	}
	rec = postJSON(t, s.handleWorktreeApply, `{"name":"w1","how":"patch"}`)
	if rec.Code != 200 {
		t.Fatalf("patch=%d %s", rec.Code, rec.Body.String())
	}
	if raw, _ := os.ReadFile(filepath.Join(dir, "a.txt")); string(raw) != "wt\n" {
		t.Fatalf("a.txt=%q", raw)
	}
	// Amb la sessió oberta no es pot treure.
	del := httptest.NewRecorder()
	s.handleWorktrees(del, httptest.NewRequest(http.MethodDelete, "/api/worktrees?name=w1", nil))
	if del.Code != 409 {
		t.Fatalf("delete amb sessió=%d", del.Code)
	}
}

func TestGitCommitAPI(t *testing.T) {
	s, dir := gitTestServer(t)
	if err := os.WriteFile(filepath.Join(dir, "n.txt"), []byte("n\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rec := postJSON(t, s.handleGitCommit, `{"message":"add n"}`)
	if rec.Code != 200 {
		t.Fatalf("commit=%d %s", rec.Code, rec.Body.String())
	}
	if got := gitW(t, dir, "log", "-1", "--format=%s"); got != "add n" {
		t.Fatalf("log=%q", got)
	}
	if rec := postJSON(t, s.handleGitCommit, `{"message":""}`); rec.Code != 400 {
		t.Fatalf("sense missatge=%d", rec.Code)
	}
}
