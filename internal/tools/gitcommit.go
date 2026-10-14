package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// git_commit i gh_pr_create: el final del cicle (canvis → commit → PR) sense
// passar per un bash genèric. Els arguments són tancats (res de flags
// arbitraris) i la política per defecte és «ask»; en els modes de només
// lectura es deneguen (agent.Policy.Decide).

// GHRunnerIn és GHRunner amb directori: gh ha de veure el repo del
// workspace de la sessió, no el del procés. Injectable per als tests.
var GHRunnerIn = runGHIn

func runGHIn(ctx context.Context, dir string, args ...string) (string, error) {
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, DefaultTimeout)
		defer cancel()
	}
	cmd := exec.CommandContext(ctx, "gh", args...)
	cmd.Dir = dir
	return commandOutput(cmd)
}

// GitPushRunner fa el push (injectable: els tests no tenen remot de debò).
var GitPushRunner = func(ctx context.Context, root, remote, branch string) (string, error) {
	return gitRun(root, nil, nil, "push", "-u", remote, branch)
}

var literalPathspecs = []string{"GIT_LITERAL_PATHSPECS=1"}

// GitCommitArgs són els arguments de git_commit.
type GitCommitArgs struct {
	Message string   `json:"message"`
	Paths   []string `json:"paths"`
	All     bool     `json:"all"`
}

// GitCommit fa commit al repo del workspace dir.
//   - paths: aquests camins (relatius al workspace o absoluts dins del repo).
//   - all: tots els canvis del workspace (git add -A dins del directori).
//   - cap dels dos: els fitxers que l'agent ha canviat en aquesta sessió
//     (foto git d'inici de sessió vs ara, bash inclòs; si no n'hi ha, el
//     journal de write/edit).
//
// Fa `git add -A -- <camins>` i `git commit -- <camins>`: el que l'usuari
// tingui staged fora d'aquests camins no entra al commit. Els hooks de git
// (pre-commit…) s'executen com sempre.
func GitCommit(owner, dir string, a GitCommitArgs) (string, error) {
	msg := strings.TrimSpace(a.Message)
	if msg == "" {
		return "", errors.New("git_commit necessita message")
	}
	if len(msg) > 10000 {
		return "", errors.New("missatge massa llarg (màx 10000 caràcters)")
	}
	if dir == "" {
		dir, _ = os.Getwd()
	}
	root, prefix, ok := gitRepoOf(dir)
	if !ok {
		return "", errors.New("el workspace no és un repo git")
	}
	var paths []string
	switch {
	case len(a.Paths) > 0:
		for _, p := range a.Paths {
			rel, err := repoRel(root, dir, p)
			if err != nil {
				return "", err
			}
			paths = append(paths, rel)
		}
	case a.All:
		if prefix == "" {
			paths = []string{"."}
		} else {
			paths = []string{strings.TrimSuffix(prefix, "/")}
		}
	default:
		_, changed, err := GitCPs.ChangedSinceBaseline(owner, dir)
		if err != nil {
			return "", err
		}
		paths = changed
	}
	if len(paths) == 0 {
		return "", errors.New("no hi ha canvis de l'agent en aquesta sessió: indica paths o all=true")
	}
	addArgs := append([]string{"add", "-A", "--"}, paths...)
	if _, err := gitRun(root, literalPathspecs, nil, addArgs...); err != nil {
		return "", err
	}
	staged, err := gitRun(root, literalPathspecs, nil, append([]string{"diff", "--cached", "--name-only", "--"}, paths...)...)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(staged) == "" {
		return "", errors.New("res a fer commit: els camins no tenen canvis")
	}
	commitArgs := append([]string{"commit", "-m", msg, "--"}, paths...)
	if _, err := gitRun(root, literalPathspecs, nil, commitArgs...); err != nil {
		return "", err
	}
	out, _ := gitRun(root, nil, nil, "log", "-1", "--stat", "--format=commit %h%n%s%n")
	return "commit fet a " + GitBranch(root) + "\n" + truncate(strings.TrimSpace(out), 4000), nil
}

// repoRel converteix un camí (relatiu al workspace o absolut) en relatiu a
// l'arrel del repo, amb barres /. Fora del repo → error.
func repoRel(root, dir, p string) (string, error) {
	p = strings.TrimSpace(p)
	if p == "" {
		return "", errors.New("camí buit")
	}
	if !filepath.IsAbs(p) {
		p = filepath.Join(dir, p)
	}
	rel, err := filepath.Rel(root, filepath.Clean(p))
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("%s és fora del repo", p)
	}
	return filepath.ToSlash(rel), nil
}

// GitCommitTool és l'entrada de l'eina (JSON del model).
func GitCommitTool(owner, dir, argsJSON string) (string, error) {
	var a GitCommitArgs
	if err := json.Unmarshal([]byte(argsJSON), &a); err != nil {
		return "", fmt.Errorf("arguments git_commit il·legibles: %w", err)
	}
	return GitCommit(owner, dir, a)
}

// PRArgs són els arguments de gh_pr_create.
type PRArgs struct {
	Title  string `json:"title"`
	Body   string `json:"body"`
	Base   string `json:"base"`
	Draft  bool   `json:"draft"`
	Branch string `json:"branch"`
}

var branchSlugRe = regexp.MustCompile(`[^a-z0-9]+`)

func branchSlug(title string) string {
	s := strings.Trim(branchSlugRe.ReplaceAllString(strings.ToLower(title), "-"), "-")
	if len(s) > 40 {
		s = strings.TrimRight(s[:40], "-")
	}
	if s == "" {
		s = "changes"
	}
	return s
}

var safeRefRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._/-]{0,99}$`)

func validBranchName(b string) bool {
	return safeRefRe.MatchString(b) && !strings.Contains(b, "..") && !strings.HasSuffix(b, ".lock") &&
		!strings.HasSuffix(b, "/") && !strings.Contains(b, "//")
}

// defaultBranch pregunta a gh la branca per defecte; si no pot, mira
// origin/HEAD i, en últim terme, main.
func defaultBranch(ctx context.Context, root string) string {
	if out, err := GHRunnerIn(ctx, root, "repo", "view", "--json", "defaultBranchRef", "-q", ".defaultBranchRef.name"); err == nil {
		if b := strings.TrimSpace(out); b != "" && validBranchName(b) {
			return b
		}
	}
	if out, err := gitRun(root, nil, nil, "symbolic-ref", "--short", "refs/remotes/origin/HEAD"); err == nil {
		if b := strings.TrimPrefix(strings.TrimSpace(out), "origin/"); b != "" {
			return b
		}
	}
	return "main"
}

// GHPRCreate obre una PR amb gh des del repo del workspace. A la branca per
// defecte (o amb HEAD separat) no la fa des d'allà: crea primer una branca
// nova (branch o gregal/<títol>) amb els commits i el working tree tal com
// són. Push sense --force. Torna la URL de la PR.
func GHPRCreate(ctx context.Context, dir string, a PRArgs) (string, error) {
	title := strings.TrimSpace(a.Title)
	if title == "" {
		return "", errors.New("gh_pr_create necessita title")
	}
	if len(title) > 256 || len(a.Body) > 60000 {
		return "", errors.New("title o body massa llargs")
	}
	if dir == "" {
		dir, _ = os.Getwd()
	}
	root, _, ok := gitRepoOf(dir)
	if !ok {
		return "", errors.New("el workspace no és un repo git")
	}
	base := strings.TrimSpace(a.Base)
	if base != "" && !validBranchName(base) {
		return "", fmt.Errorf("base invàlida: %q", base)
	}
	if a.Branch != "" && !validBranchName(a.Branch) {
		return "", fmt.Errorf("branch invàlida: %q", a.Branch)
	}
	def := defaultBranch(ctx, root)
	if base == "" {
		base = def
	}
	cur := GitBranch(root) // "" = HEAD separat
	var notes []string
	if cur == "" || cur == def || cur == base {
		nb := strings.TrimSpace(a.Branch)
		if nb == "" {
			nb = "gregal/" + branchSlug(title)
			for i := 2; i < 50; i++ {
				if _, err := gitRun(root, nil, nil, "rev-parse", "-q", "--verify", "refs/heads/"+nb); err != nil {
					break
				}
				nb = "gregal/" + branchSlug(title) + fmt.Sprintf("-%d", i)
			}
		}
		if nb == def || nb == base {
			return "", fmt.Errorf("la PR no pot sortir de %s cap a ella mateixa: indica una branch nova", nb)
		}
		if _, err := gitRun(root, nil, nil, "switch", "-c", nb); err != nil {
			return "", fmt.Errorf("no s'ha pogut crear la branca %s: %w", nb, err)
		}
		notes = append(notes, "branca nova "+nb+" (eres a "+firstOf(cur, "HEAD separat")+")")
		cur = nb
	}
	remote := "origin"
	if _, err := gitRun(root, nil, nil, "remote", "get-url", remote); err != nil {
		return "", errors.New("el repo no té remot origin: no es pot fer push")
	}
	// Hi ha d'haver commits per proposar.
	// (Sense la branca remota a la vista no es pot saber: gh ho dirà.)
	cmpBase := remote + "/" + base
	if _, err := gitRun(root, nil, nil, "rev-parse", "-q", "--verify", "refs/remotes/"+cmpBase); err == nil {
		if n, err := gitRun(root, nil, nil, "rev-list", "--count", cmpBase+"..HEAD"); err == nil && strings.TrimSpace(n) == "0" {
			return strings.Join(notes, "\n"), fmt.Errorf("la branca %s no té commits respecte a %s: fes git_commit primer", cur, base)
		}
	}
	if _, err := GitPushRunner(ctx, root, remote, cur); err != nil {
		return strings.Join(notes, "\n"), fmt.Errorf("push: %w", err)
	}
	args := []string{"pr", "create", "--title", title, "--body", a.Body, "--base", base, "--head", cur}
	if a.Draft {
		args = append(args, "--draft")
	}
	out, err := GHRunnerIn(ctx, root, args...)
	if err != nil {
		return strings.Join(notes, "\n"), fmt.Errorf("gh pr create: %w", err)
	}
	notes = append(notes, "PR: "+strings.TrimSpace(out))
	return strings.Join(notes, "\n"), nil
}

// GHPRCreateTool és l'entrada de l'eina (JSON del model).
func GHPRCreateTool(ctx context.Context, dir, argsJSON string) (string, error) {
	var a PRArgs
	if err := json.Unmarshal([]byte(argsJSON), &a); err != nil {
		return "", fmt.Errorf("arguments gh_pr_create il·legibles: %w", err)
	}
	return GHPRCreate(ctx, dir, a)
}

func firstOf(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
