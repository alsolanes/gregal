package tools

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// Checkpoints amb git.
//
// El Journal només veu el que passa per write/edit/patch/office: un `sed -i`,
// un `npm install` o un `rm` fets amb bash queden fora i el rewind no els
// desfà. Aquí, als repos git, es fa una foto de TOT el working tree (fitxers
// seguits + nous no ignorats, respectant .gitignore) a l'inici de cada torn i
// abans de cada ordre de shell que no és de només lectura.
//
// La foto es fa amb plomeria i no toca res de l'usuari: un índex temporal
// (GIT_INDEX_FILE, sembrat amb una còpia de l'índex real perquè git aprofiti
// la caché de stat) + `add -A` + `write-tree` + `commit-tree`, i el commit es
// penja d'un ref amagat (refs/gregal/checkpoints/<sessió>/<n>). HEAD, l'índex,
// les branques i l'stash no es toquen mai.
//
// Restaurar escriu només els fitxers que difereixen de la foto (amb un altre
// índex temporal + checkout-index) i esborra els que s'han creat després i no
// són ignorats. El Journal continua sent el pla B fora de git.

// GitCheckpoint és una foto del working tree.
type GitCheckpoint struct {
	N int `json:"n"`
	// Seq és el número d'esdeveniment del Journal quan es va fer la foto:
	// tornar-hi desfà també el journal fins aquí.
	Seq int `json:"seq"`
	// ConvoLen és la llargada de la conversa a l'inici del torn (-1 a les
	// fotos de pas: tornar-hi no retalla la conversa).
	ConvoLen int       `json:"convo_len"`
	Kind     string    `json:"kind"` // turn | step
	Label    string    `json:"label"`
	Commit   string    `json:"commit"`
	Ref      string    `json:"ref"`
	At       time.Time `json:"at"`
	Root     string    `json:"root"`
	Prefix   string    `json:"prefix,omitempty"`
	tree     string
}

// GitCheckpointStore guarda les fotos per propietari (sessió). Com el
// Journal, és un de sol per procés.
type GitCheckpointStore struct {
	mu      sync.Mutex
	run     string
	byOwner map[string][]GitCheckpoint
	next    map[string]int
	journal *Journal
	// Enabled i Keep venen del config (git.checkpoints, git.checkpoint_keep).
	Enabled bool
	Keep    int
	lastErr error
	// refs és quants refs de checkpoint té cada repo segons l'últim prune
	// més els creats després. Si no passen de Keep, el prune no cal
	// (s'estalvia un for-each-ref a cada foto, ~60 ms a Windows).
	refs map[string]int
}

// LastError és l'últim error d'una foto (nil si l'última ha anat bé).
func (st *GitCheckpointStore) LastError() error {
	st.mu.Lock()
	defer st.mu.Unlock()
	return st.lastErr
}

func (st *GitCheckpointStore) setErr(err error) error {
	st.mu.Lock()
	st.lastErr = err
	st.mu.Unlock()
	return err
}

// Valors per defecte i límits de la foto.
const (
	DefaultGitCheckpointKeep = 50
	gitCPMaxUntracked        = 5000
	gitCPMaxFileBytes        = 50 << 20
	gitCPTimeout             = 60 * time.Second
	worktreesRel             = ".gregal/worktrees"
)

// NewGitCheckpointStore crea un magatzem lligat a un journal (per al Seq).
func NewGitCheckpointStore(j *Journal) *GitCheckpointStore {
	return &GitCheckpointStore{
		run:     strconv.FormatInt(time.Now().UnixNano()/int64(time.Millisecond), 36),
		byOwner: map[string][]GitCheckpoint{},
		next:    map[string]int{},
		refs:    map[string]int{},
		journal: j,
		Enabled: true,
		Keep:    DefaultGitCheckpointKeep,
	}
}

// GitCPs és el magatzem global (lligat a Active). Sota `go test` comença
// apagat: els tests corren amb el directori del paquet com a workspace, que
// és dins del repo de debò, i una foto o un rewind hi tocarien la feina
// viva. Els tests que el volen l'encenen amb un repo temporal.
var GitCPs = func() *GitCheckpointStore {
	st := NewGitCheckpointStore(Active)
	st.Enabled = !testing.Testing()
	return st
}()

// ConfigureGitCheckpoints aplica el config a l'arrencada.
func ConfigureGitCheckpoints(enabled bool, keep int) {
	GitCPs.mu.Lock()
	defer GitCPs.mu.Unlock()
	GitCPs.Enabled = enabled
	if keep <= 0 {
		keep = DefaultGitCheckpointKeep
	}
	GitCPs.Keep = keep
}

// gitRun executa git a dir amb env extra i stdin opcional. L'error duu el
// stderr de git (sense, «exit status 128» no diu res).
func gitRun(dir string, env []string, stdin []byte, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), gitCPTimeout)
	defer cancel()
	c := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...)
	if len(env) > 0 {
		c.Env = append(os.Environ(), env...)
	}
	if stdin != nil {
		c.Stdin = bytes.NewReader(stdin)
	}
	var out, errb bytes.Buffer
	c.Stdout, c.Stderr = &out, &errb
	if err := c.Run(); err != nil {
		msg := strings.TrimSpace(errb.String())
		if msg == "" {
			msg = strings.TrimSpace(out.String())
		}
		if len(msg) > 400 {
			msg = msg[:400] + "…"
		}
		return out.String(), fmt.Errorf("git %s: %v: %s", args[0], err, msg)
	}
	return out.String(), nil
}

// GitText executa git a dir i en torna la sortida (per a qui no és del
// paquet: la web).
func GitText(dir string, args ...string) (string, error) { return gitRun(dir, nil, nil, args...) }

// gitIdentityEnv evita que commit-tree falli en màquines sense user.name.
var gitIdentityEnv = []string{
	"GIT_AUTHOR_NAME=gregal", "GIT_AUTHOR_EMAIL=gregal@localhost",
	"GIT_COMMITTER_NAME=gregal", "GIT_COMMITTER_EMAIL=gregal@localhost",
}

// gitRepoOf torna l'arrel del repo i el prefix de dir dins seu ("" si dir
// és l'arrel; acabat en "/" si no). ok=false fora de git.
func gitRepoOf(dir string) (root, prefix string, ok bool) {
	if dir == "" {
		dir, _ = os.Getwd()
	}
	if r, ok := repoCache.get(dir); ok {
		return r.root, r.prefix, true
	}
	out, err := gitRun(dir, nil, nil, "rev-parse", "--show-toplevel", "--show-prefix", "--git-path", "index")
	if err != nil {
		return "", "", false
	}
	lines := strings.Split(strings.ReplaceAll(out, "\r", ""), "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) == "" {
		return "", "", false
	}
	root = filepath.FromSlash(strings.TrimSpace(lines[0]))
	if len(lines) > 1 {
		prefix = strings.TrimSpace(lines[1])
	}
	index := ""
	if len(lines) > 2 {
		// --git-path és relatiu al directori on s'ha cridat git (dir).
		if index = filepath.FromSlash(strings.TrimSpace(lines[2])); index != "" && !filepath.IsAbs(index) {
			index = filepath.Join(dir, index)
		}
	}
	repoCache.put(dir, repoInfo{root: root, prefix: prefix, index: index})
	return root, prefix, true
}

// repoInfo és el que gitRepoOf sap d'un directori: l'arrel, el prefix i
// on és l'índex. Cada foto en feia dos rev-parse (un procés de git a
// Windows són ~60 ms); el repo d'un directori no canvia durant una sessió.
// Només es recorden els positius (un `git init` a mig torn s'ha de veure)
// i es tornen a validar mirant que l'arrel encara tingui .git.
type repoInfo struct{ root, prefix, index string }

type repoInfoCache struct {
	mu sync.Mutex
	m  map[string]repoInfo
}

var repoCache = &repoInfoCache{m: map[string]repoInfo{}}

func (c *repoInfoCache) get(dir string) (repoInfo, bool) {
	c.mu.Lock()
	r, ok := c.m[dir]
	c.mu.Unlock()
	if !ok {
		return r, false
	}
	if _, err := os.Stat(filepath.Join(r.root, ".git")); err != nil {
		c.mu.Lock()
		delete(c.m, dir)
		c.mu.Unlock()
		return r, false
	}
	return r, true
}

func (c *repoInfoCache) put(dir string, r repoInfo) {
	c.mu.Lock()
	c.m[dir] = r
	c.mu.Unlock()
}

// indexOf és l'índex real del repo de root (de la cache si se sap).
func indexOf(root string) (string, error) {
	if r, ok := repoCache.get(root); ok && r.index != "" {
		return r.index, nil
	}
	gp, err := gitRun(root, nil, nil, "rev-parse", "--git-path", "index")
	if err != nil {
		return "", err
	}
	src := filepath.FromSlash(strings.TrimSpace(gp))
	if !filepath.IsAbs(src) {
		src = filepath.Join(root, src)
	}
	return src, nil
}

// cpPathspec limita la foto al workspace (prefix) i deixa fora els worktrees
// de gregal, que són repos sencers penjats de .gregal/worktrees.
func cpPathspec(prefix string) []string {
	pos := "."
	if prefix != "" {
		pos = prefix
	}
	return []string{pos, ":(exclude)" + worktreesRel}
}

// addPathspec és cpPathspec per a `git add`, que falla («paths are ignored»)
// si l'exclusió anomena un camí ignorat. WorktreeCreate posa
// .gregal/worktrees a info/exclude: llavors (o si no existeix) l'exclusió
// sobra i es treu.
func addPathspec(root, prefix string) []string {
	spec := cpPathspec(prefix)
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(worktreesRel))); err != nil {
		return spec[:1]
	}
	if _, err := gitRun(root, nil, nil, "check-ignore", "-q", worktreesRel+"/"); err == nil {
		return spec[:1]
	}
	return spec
}

// tempIndex crea un índex temporal; seed=true hi copia l'índex real.
func tempIndex(root string, seed bool) (string, func(), error) {
	f, err := os.CreateTemp("", "gregal-cp-*.idx")
	if err != nil {
		return "", func() {}, err
	}
	path := f.Name()
	f.Close()
	cleanup := func() { os.Remove(path); os.Remove(path + ".lock") }
	if seed {
		if src, err := indexOf(root); err == nil {
			if raw, err := os.ReadFile(src); err == nil {
				if err := os.WriteFile(path, raw, 0o600); err != nil {
					cleanup()
					return "", func() {}, err
				}
				return path, cleanup, nil
			}
		}
	}
	// Índex inexistent: git l'accepta buit si el fitxer no hi és.
	os.Remove(path)
	return path, cleanup, nil
}

// checkUntracked rebutja fotos desmesurades: milers de fitxers nous no
// ignorats (un node_modules sense .gitignore) o binaris enormes acabarien
// a la base d'objectes del repo.
func checkUntracked(root, prefix string) error {
	out, err := gitRun(root, nil, nil, append([]string{"ls-files", "--others", "--exclude-standard", "-z", "--"}, cpPathspec(prefix)...)...)
	if err != nil {
		return err
	}
	n := 0
	for _, rel := range strings.Split(out, "\x00") {
		if rel == "" {
			continue
		}
		n++
		if n > gitCPMaxUntracked {
			return fmt.Errorf("massa fitxers nous sense ignorar (> %d): afegeix-los al .gitignore", gitCPMaxUntracked)
		}
		if st, err := os.Lstat(filepath.Join(root, filepath.FromSlash(rel))); err == nil && st.Size() > gitCPMaxFileBytes {
			return fmt.Errorf("fitxer nou massa gran per a la foto: %s (%d MB)", rel, st.Size()>>20)
		}
	}
	return nil
}

// SnapshotTree escriu el working tree (dins del prefix) com un tree de git
// i en torna el hash. No toca l'índex de l'usuari.
func snapshotTree(root, prefix string) (string, error) {
	if err := checkUntracked(root, prefix); err != nil {
		return "", err
	}
	idx, cleanup, err := tempIndex(root, true)
	if err != nil {
		return "", err
	}
	defer cleanup()
	env := []string{"GIT_INDEX_FILE=" + idx}
	// Sense --ignore-errors: si add falla, write-tree escriuria l'índex
	// sembrat (l'estat de l'usuari, no el del disc) i restaurar-lo
	// desfaria feina bona. Millor no tenir foto.
	if _, err := gitRun(root, env, nil, append([]string{"add", "-A", "--"}, addPathspec(root, prefix)...)...); err != nil {
		return "", err
	}
	out, err := gitRun(root, env, nil, "write-tree")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// refKey fa un component de ref vàlid a partir del propietari.
func (st *GitCheckpointStore) refKey(owner string) string {
	if owner == "" {
		owner = "main"
	}
	var b strings.Builder
	for _, r := range owner {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}
	k := strings.Trim(b.String(), "-")
	if k == "" {
		k = "s"
	}
	if len(k) > 40 {
		k = k[:40]
	}
	return k + "-" + st.run
}

// Snapshot fa una foto del workspace dir per al propietari. kind és "turn"
// (inici de torn, amb convoLen) o "step" (abans d'una ordre de shell).
// Torna nil sense error fora de git, amb els checkpoints apagats o quan la
// foto de pas seria idèntica a l'anterior.
func (st *GitCheckpointStore) Snapshot(owner, dir, kind, label string, convoLen int) (*GitCheckpoint, error) {
	st.mu.Lock()
	on := st.Enabled
	st.mu.Unlock()
	if !on {
		return nil, nil
	}
	root, prefix, ok := gitRepoOf(dir)
	if !ok {
		return nil, nil
	}
	tree, err := snapshotTree(root, prefix)
	if err != nil {
		return nil, st.setErr(err)
	}
	seq := 0
	if st.journal != nil {
		seq = st.journal.Seq()
	}
	if kind != "turn" {
		kind = "step"
		convoLen = -1
	}
	st.mu.Lock()
	list := st.byOwner[owner]
	if kind == "step" && len(list) > 0 {
		last := list[len(list)-1]
		if last.tree == tree && last.Root == root && last.Prefix == prefix && last.Seq == seq {
			st.mu.Unlock()
			return nil, nil
		}
	}
	st.next[owner]++
	n := st.next[owner]
	keep := st.Keep
	ref := "refs/gregal/checkpoints/" + st.refKey(owner) + "/" + strconv.Itoa(n)
	st.mu.Unlock()

	args := []string{"commit-tree", tree, "-m", "gregal checkpoint " + strconv.Itoa(n) + ": " + oneLine(label, 120)}
	if head, err := gitRun(root, nil, nil, "rev-parse", "-q", "--verify", "HEAD^{commit}"); err == nil && strings.TrimSpace(head) != "" {
		args = append(args, "-p", strings.TrimSpace(head))
	}
	out, err := gitRun(root, gitIdentityEnv, nil, args...)
	if err != nil {
		return nil, st.setErr(err)
	}
	commit := strings.TrimSpace(out)
	if _, err := gitRun(root, nil, nil, "update-ref", ref, commit); err != nil {
		return nil, st.setErr(err)
	}
	st.mu.Lock()
	if n, ok := st.refs[root]; ok {
		st.refs[root] = n + 1
	}
	st.mu.Unlock()
	st.setErr(nil)
	cp := GitCheckpoint{N: n, Seq: seq, ConvoLen: convoLen, Kind: kind, Label: oneLine(label, 160),
		Commit: commit, Ref: ref, At: time.Now(), Root: root, Prefix: prefix, tree: tree}
	st.mu.Lock()
	st.byOwner[owner] = append(st.byOwner[owner], cp)
	st.mu.Unlock()
	st.prune(root, keep)
	return &cp, nil
}

func oneLine(s string, n int) string {
	s = strings.Join(strings.Fields(s), " ")
	if len([]rune(s)) > n {
		s = string([]rune(s)[:n]) + "…"
	}
	return s
}

// prune deixa només els keep refs més nous del repo (de totes les
// sessions, també d'execucions anteriors) i oblida els que han caigut.
func (st *GitCheckpointStore) prune(root string, keep int) {
	if keep <= 0 {
		keep = DefaultGitCheckpointKeep
	}
	st.mu.Lock()
	n, sabut := st.refs[root]
	st.mu.Unlock()
	if sabut && n <= keep {
		return
	}
	out, err := gitRun(root, nil, nil, "for-each-ref", "--sort=-creatordate", "--format=%(refname)", "refs/gregal/checkpoints/")
	if err != nil {
		return
	}
	refs := strings.Fields(out)
	st.mu.Lock()
	st.refs[root] = min(len(refs), keep)
	st.mu.Unlock()
	if len(refs) <= keep {
		return
	}
	var stdin bytes.Buffer
	gone := map[string]bool{}
	for _, r := range refs[keep:] {
		stdin.WriteString("delete " + r + "\n")
		gone[r] = true
	}
	if _, err := gitRun(root, nil, stdin.Bytes(), "update-ref", "--stdin"); err != nil {
		return
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	for owner, list := range st.byOwner {
		kept := list[:0]
		for _, cp := range list {
			if cp.Root == root && gone[cp.Ref] {
				continue
			}
			kept = append(kept, cp)
		}
		st.byOwner[owner] = kept
	}
}

// List torna les fotos del propietari, en ordre.
func (st *GitCheckpointStore) List(owner string) []GitCheckpoint {
	st.mu.Lock()
	defer st.mu.Unlock()
	return append([]GitCheckpoint(nil), st.byOwner[owner]...)
}

// Get torna la foto n del propietari.
func (st *GitCheckpointStore) Get(owner string, n int) (GitCheckpoint, bool) {
	st.mu.Lock()
	defer st.mu.Unlock()
	for _, cp := range st.byOwner[owner] {
		if cp.N == n {
			return cp, true
		}
	}
	return GitCheckpoint{}, false
}

// Baseline és la primera foto del propietari per al workspace dir (l'estat
// d'abans de tota la feina de la sessió en aquell directori).
func (st *GitCheckpointStore) Baseline(owner, dir string) (GitCheckpoint, bool) {
	root, prefix, ok := gitRepoOf(dir)
	if !ok {
		return GitCheckpoint{}, false
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	for _, cp := range st.byOwner[owner] {
		if samePath(cp.Root, root) && cp.Prefix == prefix {
			return cp, true
		}
	}
	return GitCheckpoint{}, false
}

// Forget oblida les fotos del propietari (els refs es poden pel prune).
func (st *GitCheckpointStore) Forget(owner string) {
	st.mu.Lock()
	defer st.mu.Unlock()
	delete(st.byOwner, owner)
}

func samePath(a, b string) bool {
	if strings.EqualFold(filepath.Clean(a), filepath.Clean(b)) {
		return true
	}
	// El mateix directori escrit diferent: noms curts 8.3 de Windows
	// (RUNNER~1 vs runneradmin, que és el que torna git) o enllaços
	// simbòlics. EvalSymlinks els resol tots dos al nom real.
	ra, ea := filepath.EvalSymlinks(a)
	rb, eb := filepath.EvalSymlinks(b)
	return ea == nil && eb == nil && strings.EqualFold(filepath.Clean(ra), filepath.Clean(rb))
}

// Restore torna els fitxers del workspace a la foto n i oblida les fotos
// posteriors. El resum té el mateix format que el del Journal.
func (st *GitCheckpointStore) Restore(owner string, n int) (string, error) {
	cp, ok := st.Get(owner, n)
	if !ok {
		return "", fmt.Errorf("checkpoint git g%d inexistent", n)
	}
	restored, deleted, err := restoreTo(cp.Root, cp.Prefix, cp.Commit)
	st.mu.Lock()
	list := st.byOwner[owner]
	var drop []string
	kept := list[:0]
	for _, c := range list {
		if c.N > n {
			drop = append(drop, c.Ref)
			continue
		}
		kept = append(kept, c)
	}
	st.byOwner[owner] = kept
	st.mu.Unlock()
	if len(drop) > 0 {
		var stdin bytes.Buffer
		for _, r := range drop {
			stdin.WriteString("delete " + r + "\n")
		}
		_, _ = gitRun(cp.Root, nil, stdin.Bytes(), "update-ref", "--stdin")
	}
	return summaryOf(restored, deleted), err
}

func summaryOf(restored, deleted []string) string {
	var parts []string
	if len(restored) > 0 {
		parts = append(parts, "restaurats: "+strings.Join(restored, ", "))
	}
	if len(deleted) > 0 {
		parts = append(parts, "esborrats: "+strings.Join(deleted, ", "))
	}
	return strings.Join(parts, " · ")
}

// restoreTo deixa el working tree (dins del prefix) com al commit: escriu
// els fitxers que difereixen i esborra els creats després (no ignorats).
// Els ignorats i els submòduls no es toquen.
func restoreTo(root, prefix, commit string) (restored, deleted []string, err error) {
	if _, err := gitRun(root, nil, nil, "cat-file", "-e", commit+"^{commit}"); err != nil {
		return nil, nil, fmt.Errorf("la foto %s ja no existeix al repo", shortHash(commit))
	}
	cur, err := snapshotTree(root, prefix)
	if err != nil {
		return nil, nil, err
	}
	raw, err := gitRun(root, nil, nil, append([]string{"diff-tree", "-r", "-z", "--no-renames", commit + "^{tree}", cur, "--"}, cpPathspec(prefix)...)...)
	if err != nil {
		return nil, nil, err
	}
	var toWrite []string
	toks := strings.Split(raw, "\x00")
	for i := 0; i+1 < len(toks); i += 2 {
		meta, path := toks[i], toks[i+1]
		f := strings.Fields(strings.TrimPrefix(meta, ":"))
		if len(f) < 5 || path == "" {
			continue
		}
		srcMode, dstMode, status := f[0], f[1], f[4]
		switch status[:1] {
		case "A": // existeix ara i no a la foto → fora
			if dstMode == "160000" {
				continue
			}
			full := filepath.Join(root, filepath.FromSlash(path))
			if err := os.Remove(full); err != nil && !os.IsNotExist(err) {
				return restored, deleted, err
			}
			deleted = append(deleted, path)
			removeEmptyParents(root, filepath.Dir(full))
		default: // D, M, T → torna el contingut de la foto
			if srcMode == "160000" {
				continue
			}
			toWrite = append(toWrite, path)
		}
	}
	if len(toWrite) > 0 {
		idx, cleanup, err := tempIndex(root, false)
		if err != nil {
			return restored, deleted, err
		}
		defer cleanup()
		env := []string{"GIT_INDEX_FILE=" + idx}
		if _, err := gitRun(root, env, nil, "read-tree", commit+"^{tree}"); err != nil {
			return restored, deleted, err
		}
		stdin := []byte(strings.Join(toWrite, "\x00") + "\x00")
		if _, err := gitRun(root, env, stdin, "checkout-index", "-f", "-z", "--stdin"); err != nil {
			return restored, deleted, err
		}
		restored = append(restored, toWrite...)
	}
	return restored, deleted, nil
}

func removeEmptyParents(root, dir string) {
	root = filepath.Clean(root)
	for d := filepath.Clean(dir); len(d) > len(root) && strings.HasPrefix(strings.ToLower(d), strings.ToLower(root)); d = filepath.Dir(d) {
		if os.Remove(d) != nil {
			return
		}
	}
}

func shortHash(h string) string {
	if len(h) > 10 {
		return h[:10]
	}
	return h
}

// RewindTo torna a la foto n: primer desfà el journal fins al seu Seq
// (filtrat com sempre) i després restaura el working tree amb git, que mana.
// convoLen és fins on s'ha de retallar la conversa (-1 = no tocar-la).
func (st *GitCheckpointStore) RewindTo(owner string, n int, keep func(string) bool) (summary string, convoLen int, err error) {
	cp, ok := st.Get(owner, n)
	if !ok {
		return "", -1, fmt.Errorf("checkpoint git g%d inexistent", n)
	}
	var parts []string
	if st.journal != nil && cp.Seq <= st.journal.Seq() {
		if out, jerr := st.journal.RewindFiltered(cp.Seq, keep); jerr != nil {
			return out, -1, jerr
		} else if out != "" && out != "res a desfer" {
			parts = append(parts, out)
		}
	}
	out, err := st.Restore(owner, n)
	if out != "" {
		parts = append(parts, out)
	}
	return strings.Join(parts, " · "), cp.ConvoLen, err
}

// RewindAll és el rewind total: journal a zero i, si n'hi ha, el workspace
// a la primera foto de la sessió. Fora de git és exactament el Journal.
func (st *GitCheckpointStore) RewindAll(owner, dir string, keep func(string) bool) (string, error) {
	out, err := st.journal.RewindFiltered(0, keep)
	if err != nil {
		return out, err
	}
	base, ok := st.Baseline(owner, dir)
	if !ok {
		return out, nil
	}
	gout, gerr := st.Restore(owner, base.N)
	// La foto base ja és l'estat d'abans: també s'oblida, i el torn
	// següent en farà una de nova.
	st.mu.Lock()
	list := st.byOwner[owner]
	kept := list[:0]
	for _, c := range list {
		if c.N != base.N {
			kept = append(kept, c)
		}
	}
	st.byOwner[owner] = kept
	st.mu.Unlock()
	var parts []string
	if out != "" && out != "res a desfer" {
		parts = append(parts, out)
	}
	if gout != "" {
		parts = append(parts, gout)
	}
	if len(parts) == 0 {
		return "res a desfer", gerr
	}
	return strings.Join(parts, " · "), gerr
}

// ChangedSinceBaseline llista els fitxers (relatius a l'arrel del repo) que
// han canviat des de la primera foto de la sessió al workspace dir, bash
// inclòs. Sense foto, recau en els fitxers del Journal dins de dir.
func (st *GitCheckpointStore) ChangedSinceBaseline(owner, dir string) (root string, paths []string, err error) {
	root, prefix, ok := gitRepoOf(dir)
	if !ok {
		return "", nil, errors.New("no és un repo git")
	}
	if base, ok := st.Baseline(owner, dir); ok {
		cur, err := snapshotTree(root, prefix)
		if err != nil {
			return root, nil, err
		}
		out, err := gitRun(root, nil, nil, append([]string{"diff-tree", "-r", "-z", "--no-renames", "--name-only", base.Commit + "^{tree}", cur, "--"}, cpPathspec(prefix)...)...)
		if err != nil {
			return root, nil, err
		}
		for _, p := range strings.Split(out, "\x00") {
			if p != "" {
				paths = append(paths, p)
			}
		}
		return root, paths, nil
	}
	if st.journal != nil {
		seen := map[string]bool{}
		for _, f := range st.journal.Files() {
			if abs, aerr := filepath.Abs(f); aerr == nil {
				f = abs
			}
			rel, err := filepath.Rel(root, f)
			if err != nil || strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
				continue
			}
			rel = filepath.ToSlash(rel)
			if prefix != "" && !strings.HasPrefix(rel, prefix) {
				continue
			}
			if !seen[rel] {
				seen[rel] = true
				paths = append(paths, rel)
			}
		}
	}
	return root, paths, nil
}
