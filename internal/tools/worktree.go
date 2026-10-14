package tools

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Worktrees per sessió: una sessió (o un agent) pot treballar en un
// `git worktree` aïllat a <repo>/.gregal/worktrees/<nom>, en una branca nova
// gregal/<nom> creada des de HEAD. El repo principal no es toca fins que
// l'usuari decideix portar-hi la feina: com a pegat (git apply, només el
// working tree) o fusionant la branca (git merge, mai forçat).

// Worktree descriu un worktree de gregal.
type Worktree struct {
	Name   string `json:"name"`
	Path   string `json:"path"`
	Branch string `json:"branch"`
	Head   string `json:"head"`
	// Base és el commit des d'on va sortir (merge-base amb el HEAD del repo
	// principal): el diff es calcula contra aquí.
	Base  string `json:"base"`
	Dirty bool   `json:"dirty"`
	// Ahead: commits de la branca que el repo principal encara no té.
	Ahead int `json:"ahead"`
	// Main és l'arrel del repo principal.
	Main string `json:"main"`
}

var worktreeNameRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,47}$`)

// ValidWorktreeName diu si el nom és acceptable (també com a branca).
func ValidWorktreeName(name string) bool {
	return worktreeNameRe.MatchString(name) && !strings.Contains(name, "..") &&
		!strings.HasSuffix(name, ".lock") && !strings.HasSuffix(name, ".")
}

// worktreeBranch és la branca d'un worktree de gregal.
func worktreeBranch(name string) string { return "gregal/" + name }

// MainRepoRoot torna l'arrel del repo principal encara que dir sigui dins
// d'un worktree (el directori pare del common dir .git).
func MainRepoRoot(dir string) (string, error) {
	if dir == "" {
		dir, _ = os.Getwd()
	}
	out, err := gitRun(dir, nil, nil, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return "", errors.New("no és un repo git: " + dir)
	}
	common := filepath.FromSlash(strings.TrimSpace(out))
	if filepath.Base(common) == ".git" {
		return filepath.Dir(common), nil
	}
	// Repo nu o layout estrany: recau en el toplevel.
	top, err := gitRun(dir, nil, nil, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", err
	}
	return filepath.FromSlash(strings.TrimSpace(top)), nil
}

// WorktreeNameOf diu si path és dins d'un worktree de gregal i com es diu.
func WorktreeNameOf(path string) string {
	p := filepath.ToSlash(filepath.Clean(path))
	const marker = "/" + worktreesRel + "/"
	i := strings.LastIndex(strings.ToLower(p), strings.ToLower(marker))
	if i < 0 {
		return ""
	}
	rest := p[i+len(marker):]
	if j := strings.IndexByte(rest, '/'); j >= 0 {
		rest = rest[:j]
	}
	if !ValidWorktreeName(rest) {
		return ""
	}
	return rest
}

// WorktreeRepoOf torna l'arrel del repo principal d'un worktree de gregal
// (el que hi ha abans de .gregal/worktrees/<nom>), o "" si path no n'és
// cap. Només mira la ruta: no crida git, i la UI la pot demanar sovint.
func WorktreeRepoOf(path string) string {
	if WorktreeNameOf(path) == "" {
		return ""
	}
	p := filepath.ToSlash(filepath.Clean(path))
	const marker = "/" + worktreesRel + "/"
	i := strings.LastIndex(strings.ToLower(p), strings.ToLower(marker))
	if i <= 0 {
		return ""
	}
	return filepath.FromSlash(p[:i])
}

// InGitRepo diu si dir és dins d'un working tree de git.
func InGitRepo(dir string) bool {
	_, err := gitOut(dir, "rev-parse", "--is-inside-work-tree")
	return err == nil
}

// excludeWorktrees afegeix .gregal/worktrees/ a .git/info/exclude perquè el
// Changes, el git status i les fotos no vegin els worktrees com a fitxers
// nous. Idempotent; no toca el .gitignore del projecte.
func excludeWorktrees(main string) {
	out, err := gitRun(main, nil, nil, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return
	}
	path := filepath.Join(filepath.FromSlash(strings.TrimSpace(out)), "info", "exclude")
	raw, _ := os.ReadFile(path)
	line := "/" + worktreesRel + "/"
	for _, l := range strings.Split(string(raw), "\n") {
		if strings.TrimSpace(l) == line {
			return
		}
	}
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	if len(raw) > 0 && !strings.HasSuffix(string(raw), "\n") {
		f.WriteString("\n")
	}
	f.WriteString("# gregal: worktrees aïllats per sessió\n" + line + "\n")
}

// WorktreeCreate crea el worktree <main>/.gregal/worktrees/<name> en una
// branca nova gregal/<name> des de HEAD i hi copia els fitxers de
// .worktreeinclude (p. ex. .env, que no és al repo). Si ja existeix un
// worktree de gregal amb aquest nom, el torna tal com és.
func WorktreeCreate(dir, name string) (Worktree, error) {
	if !ValidWorktreeName(name) {
		return Worktree{}, fmt.Errorf("nom de worktree invàlid %q (lletres, xifres, . _ -; màxim 48)", name)
	}
	main, err := MainRepoRoot(dir)
	if err != nil {
		return Worktree{}, err
	}
	path := filepath.Join(main, filepath.FromSlash(worktreesRel), name)
	if wt, ok := worktreeByName(main, name); ok {
		return wt, nil
	}
	if _, err := os.Stat(path); err == nil {
		return Worktree{}, fmt.Errorf("%s ja existeix i no és un worktree de git", path)
	}
	if _, err := gitRun(main, nil, nil, "rev-parse", "-q", "--verify", "HEAD^{commit}"); err != nil {
		return Worktree{}, errors.New("el repo encara no té cap commit: cal un HEAD per crear el worktree")
	}
	branch := worktreeBranch(name)
	if _, err := gitRun(main, nil, nil, "rev-parse", "-q", "--verify", "refs/heads/"+branch); err == nil {
		return Worktree{}, fmt.Errorf("la branca %s ja existeix: tria un altre nom o esborra-la", branch)
	}
	excludeWorktrees(main)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return Worktree{}, err
	}
	if _, err := gitRun(main, nil, nil, "worktree", "add", "-b", branch, path, "HEAD"); err != nil {
		return Worktree{}, err
	}
	if err := copyWorktreeIncludes(main, path); err != nil {
		return Worktree{}, err
	}
	wt, ok := worktreeByName(main, name)
	if !ok {
		return Worktree{}, errors.New("worktree creat però git no el llista")
	}
	return wt, nil
}

// copyWorktreeIncludes copia al worktree els camins de .worktreeinclude
// (un patró glob per línia, relatiu a l'arrel; # comenta). Només els que el
// worktree no té (els seguits ja hi són). Directoris: sencers.
func copyWorktreeIncludes(main, wt string) error {
	raw, err := os.ReadFile(filepath.Join(main, ".worktreeinclude"))
	if err != nil {
		return nil
	}
	for _, line := range strings.Split(string(raw), "\n") {
		pat := strings.TrimSpace(strings.TrimRight(line, "\r"))
		if pat == "" || strings.HasPrefix(pat, "#") {
			continue
		}
		pat = strings.TrimPrefix(filepath.FromSlash(pat), string(filepath.Separator))
		if strings.Contains(pat, "..") {
			continue
		}
		matches, _ := filepath.Glob(filepath.Join(main, pat))
		for _, src := range matches {
			rel, err := filepath.Rel(main, src)
			if err != nil || strings.HasPrefix(rel, "..") || strings.HasPrefix(filepath.ToSlash(rel), worktreesRel) || filepath.ToSlash(rel) == ".git" {
				continue
			}
			if err := copyTree(src, filepath.Join(wt, rel)); err != nil {
				return fmt.Errorf(".worktreeinclude %s: %w", rel, err)
			}
		}
	}
	return nil
}

func copyTree(src, dst string) error {
	st, err := os.Lstat(src)
	if err != nil {
		return err
	}
	if st.IsDir() {
		ents, err := os.ReadDir(src)
		if err != nil {
			return err
		}
		for _, e := range ents {
			if e.Name() == ".git" {
				continue
			}
			if err := copyTree(filepath.Join(src, e.Name()), filepath.Join(dst, e.Name())); err != nil {
				return err
			}
		}
		return nil
	}
	if !st.Mode().IsRegular() {
		return nil
	}
	if _, err := os.Stat(dst); err == nil {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_EXCL|os.O_WRONLY, st.Mode().Perm())
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// WorktreeList torna els worktrees de gregal del repo de dir.
func WorktreeList(dir string) ([]Worktree, error) {
	main, err := MainRepoRoot(dir)
	if err != nil {
		return nil, err
	}
	return listWorktrees(main)
}

func worktreeByName(main, name string) (Worktree, bool) {
	list, err := listWorktrees(main)
	if err != nil {
		return Worktree{}, false
	}
	for _, wt := range list {
		if wt.Name == name {
			return wt, true
		}
	}
	return Worktree{}, false
}

func listWorktrees(main string) ([]Worktree, error) {
	out, err := gitRun(main, nil, nil, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, err
	}
	mainHead, _ := gitRun(main, nil, nil, "rev-parse", "HEAD")
	mainHead = strings.TrimSpace(mainHead)
	var list []Worktree
	for _, block := range strings.Split(strings.ReplaceAll(out, "\r", ""), "\n\n") {
		var wt Worktree
		for _, l := range strings.Split(block, "\n") {
			switch {
			case strings.HasPrefix(l, "worktree "):
				wt.Path = filepath.FromSlash(strings.TrimPrefix(l, "worktree "))
			case strings.HasPrefix(l, "HEAD "):
				wt.Head = strings.TrimPrefix(l, "HEAD ")
			case strings.HasPrefix(l, "branch "):
				wt.Branch = strings.TrimPrefix(strings.TrimPrefix(l, "branch "), "refs/heads/")
			}
		}
		name := WorktreeNameOf(wt.Path)
		if wt.Path == "" || name == "" {
			continue
		}
		wt.Name, wt.Main = name, main
		if mainHead != "" && wt.Head != "" {
			if mb, err := gitRun(main, nil, nil, "merge-base", mainHead, wt.Head); err == nil {
				wt.Base = strings.TrimSpace(mb)
			}
			if n, err := gitRun(main, nil, nil, "rev-list", "--count", mainHead+".."+wt.Head); err == nil {
				fmt.Sscanf(strings.TrimSpace(n), "%d", &wt.Ahead)
			}
		}
		if st, err := gitRun(wt.Path, nil, nil, "status", "--porcelain"); err == nil {
			wt.Dirty = strings.TrimSpace(st) != ""
		}
		list = append(list, wt)
	}
	return list, nil
}

// WorktreeDiff dona el pegat (binari inclòs) de tota la feina del worktree
// respecte a la seva base: commits de la branca + canvis sense commit +
// fitxers nous no ignorats. És el que es presenta abans de portar-ho al
// repo principal.
func WorktreeDiff(dir, name string) (string, Worktree, error) {
	main, err := MainRepoRoot(dir)
	if err != nil {
		return "", Worktree{}, err
	}
	wt, ok := worktreeByName(main, name)
	if !ok {
		return "", Worktree{}, fmt.Errorf("worktree %q inexistent", name)
	}
	base := wt.Base
	if base == "" {
		base = wt.Head
	}
	tree, err := snapshotTree(wt.Path, "")
	if err != nil {
		return "", wt, err
	}
	out, err := gitRun(wt.Path, nil, nil, "diff", "--binary", "--no-color", "--no-ext-diff", base, tree)
	if err != nil {
		return "", wt, err
	}
	return out, wt, nil
}

// WorktreeApply porta la feina del worktree al repo principal.
//
//   - how="patch": aplica el diff al working tree principal amb `git apply`
//     (sense --index: ni l'índex ni HEAD canvien). Primer --check; si no
//     encaixa, no es toca res.
//   - how="merge": `git merge --no-ff gregal/<nom>` al repo principal. Només
//     amb el working tree principal net (els fitxers no seguits no compten)
//     i amb la feina del worktree ja en commits; si hi ha conflictes es fa
//     `merge --abort` i no queda res a mitges. Mai es força res.
func WorktreeApply(dir, name, how string) (string, error) {
	main, err := MainRepoRoot(dir)
	if err != nil {
		return "", err
	}
	switch how {
	case "", "patch":
		patch, wt, err := WorktreeDiff(dir, name)
		if err != nil {
			return "", err
		}
		if strings.TrimSpace(patch) == "" {
			return "res a aplicar: el worktree " + wt.Name + " no té canvis", nil
		}
		if _, err := gitRun(main, nil, []byte(patch), "apply", "--check", "--whitespace=nowarn", "-"); err != nil {
			return "", fmt.Errorf("el pegat no encaixa amb el repo principal (no s'ha tocat res): %w", err)
		}
		if _, err := gitRun(main, nil, []byte(patch), "apply", "--whitespace=nowarn", "-"); err != nil {
			return "", err
		}
		stat, _ := gitRun(main, nil, []byte(patch), "apply", "--stat", "-")
		return "pegat aplicat al working tree de " + main + " (sense commit)\n" + strings.TrimSpace(stat), nil
	case "merge":
		wt, ok := worktreeByName(main, name)
		if !ok {
			return "", fmt.Errorf("worktree %q inexistent", name)
		}
		if wt.Dirty {
			return "", errors.New("el worktree té canvis sense commit: fes-ne commit allà (git_commit) o porta'ls com a pegat")
		}
		if wt.Ahead == 0 {
			return "res a fusionar: la branca " + wt.Branch + " no té commits nous", nil
		}
		st, err := gitRun(main, nil, nil, "status", "--porcelain", "--untracked-files=no")
		if err != nil {
			return "", err
		}
		if strings.TrimSpace(st) != "" {
			return "", errors.New("el repo principal té canvis sense commit: desa'ls o fes servir el pegat")
		}
		out, err := gitRun(main, gitIdentityFallback(main), nil, "merge", "--no-ff", "--no-edit", wt.Branch)
		if err != nil {
			_, _ = gitRun(main, nil, nil, "merge", "--abort")
			return "", fmt.Errorf("merge amb conflictes o fallit; s'ha avortat i el repo queda com estava: %w", err)
		}
		return strings.TrimSpace(out), nil
	default:
		return "", fmt.Errorf("how ha de ser patch o merge")
	}
}

// gitIdentityFallback dona identitat només si el repo no en té (un merge
// de debò ha de dur l'autor de l'usuari).
func gitIdentityFallback(dir string) []string {
	if out, err := gitRun(dir, nil, nil, "config", "user.email"); err == nil && strings.TrimSpace(out) != "" {
		return nil
	}
	return gitIdentityEnv
}

// WorktreeRemove treu el worktree (git worktree remove, sense --force: si
// té canvis sense commit, git s'hi nega i l'error ho diu). Amb
// deleteBranch també esborra la branca amb -d (només si ja està fusionada).
func WorktreeRemove(dir, name string, deleteBranch bool) (string, error) {
	main, err := MainRepoRoot(dir)
	if err != nil {
		return "", err
	}
	wt, ok := worktreeByName(main, name)
	if !ok {
		return "", fmt.Errorf("worktree %q inexistent", name)
	}
	if _, err := gitRun(main, nil, nil, "worktree", "remove", wt.Path); err != nil {
		return "", fmt.Errorf("no s'ha pogut treure (té canvis sense commit o fitxers nous?): %w", err)
	}
	msg := "worktree " + name + " tret"
	if deleteBranch && wt.Branch != "" {
		if _, err := gitRun(main, nil, nil, "branch", "-d", wt.Branch); err != nil {
			msg += "; la branca " + wt.Branch + " es conserva (no està fusionada)"
		} else {
			msg += "; branca " + wt.Branch + " esborrada"
		}
	} else if wt.Branch != "" {
		msg += "; la branca " + wt.Branch + " es conserva"
	}
	return msg, nil
}
