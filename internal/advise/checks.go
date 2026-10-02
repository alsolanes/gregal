package advise

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// runCmd executa una ordre externa amb timeout propi i la mata en
// excedir-lo. Torna la sortida combinada; qualsevol error (ordre
// inexistent, timeout, codi de sortida) es tracta com a "no disponible".
func runCmd(ctx context.Context, timeout time.Duration, dir, name string, args ...string) ([]byte, error) {
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(cctx, name, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	// Entorn net d'herències fràgils: sense això, un GOFLAGS o GOPROXY
	// estrany de l'usuari canviaria el resultat del detector.
	cmd.Env = filteredEnv()
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	if err := cmd.Run(); err != nil {
		return buf.Bytes(), err
	}
	return buf.Bytes(), nil
}

func filteredEnv() []string {
	keep := map[string]bool{
		"PATH": true, "PATHEXT": true, "SystemRoot": true, "TMP": true,
		"TEMP": true, "TMPDIR": true, "HOME": true, "USERPROFILE": true,
		"GOPROXY": true, "GOSUMDB": true, "GOFLAGS": true, "GOPATH": true,
		"GOROOT": true, "LANG": true, "LC_ALL": true,
	}
	out := []string{}
	for _, kv := range os.Environ() {
		if i := strings.IndexByte(kv, '='); i > 0 && keep[kv[:i]] {
			out = append(out, kv)
		}
	}
	return out
}

func readFile(root string, parts ...string) ([]byte, bool) {
	raw, err := os.ReadFile(filepath.Join(append([]string{root}, parts...)...))
	if err != nil {
		return nil, false
	}
	return raw, true
}

// goDirective llegeix la versió de la directiva "go" del go.mod.
func goDirective(raw []byte) (major, minor int, ok bool) {
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "go ") {
			continue
		}
		ver := strings.TrimSpace(strings.TrimPrefix(line, "go"))
		pts := strings.SplitN(ver, ".", 3)
		if len(pts) < 2 {
			return 0, 0, false
		}
		ma, err1 := strconv.Atoi(pts[0])
		mi, err2 := strconv.Atoi(pts[1])
		if err1 != nil || err2 != nil {
			return 0, 0, false
		}
		return ma, mi, true
	}
	return 0, 0, false
}

// runtimeGo parseja runtime.Version() ("go1.27.1").
func runtimeGo() (major, minor int, ok bool) {
	ver := strings.TrimPrefix(runtime.Version(), "go")
	pts := strings.SplitN(ver, ".", 3)
	if len(pts) < 2 {
		return 0, 0, false
	}
	ma, err1 := strconv.Atoi(pts[0])
	mi, err2 := strconv.Atoi(pts[1])
	if err1 != nil || err2 != nil {
		return 0, 0, false
	}
	return ma, mi, true
}

// checkGoToolchain proposa actualitzar Go si el toolchain instal·lat és
// més vell que el que demana el go.mod. Tot local: funciona sense xarxa.
func checkGoToolchain(ctx context.Context, root string) ([]Proposal, Outcome) {
	const name = "go-toolchain"
	raw, ok := readFile(root, "go.mod")
	if !ok {
		return nil, Outcome{Name: name, Status: "no-aplica", Reason: "sense go.mod"}
	}
	ma, mi, ok := goDirective(raw)
	if !ok {
		return nil, Outcome{Name: name, Status: "no-disponible", Reason: "directiva go il·legible"}
	}
	ra, ri, ok := runtimeGo()
	if !ok {
		return nil, Outcome{Name: name, Status: "no-disponible", Reason: "versió del toolchain il·legible"}
	}
	if ra > ma || (ra == ma && ri >= mi) {
		return nil, Outcome{Name: name, Status: "ok"}
	}
	return []Proposal{newProposal("toolchain", Avis,
		fmt.Sprintf("Go %d.%d instal·lat, però go.mod demana %d.%d", ra, ri, ma, mi),
		"El toolchain és més vell que la directiva del mòdul: la compilació pot fallar o usar una versió antiga del llenguatge.",
		"petit", "go.mod")}, Outcome{Name: name, Status: "proposta"}
}

// checkNpmLock proposa resincronitzar el lockfile si la versió no
// coincideix amb la del package.json. Tot local: funciona sense xarxa.
func checkNpmLock(ctx context.Context, root string) ([]Proposal, Outcome) {
	const name = "npm-lock"
	pkg, ok := readFile(root, "desktop", "package.json")
	if !ok {
		return nil, Outcome{Name: name, Status: "no-aplica", Reason: "sense desktop/package.json"}
	}
	lock, ok := readFile(root, "desktop", "package-lock.json")
	if !ok {
		return nil, Outcome{Name: name, Status: "no-aplica", Reason: "sense package-lock.json"}
	}
	var pj, pl struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(pkg, &pj); err != nil || pj.Version == "" {
		return nil, Outcome{Name: name, Status: "no-disponible", Reason: "package.json il·legible"}
	}
	if err := json.Unmarshal(lock, &pl); err != nil || pl.Version == "" {
		return nil, Outcome{Name: name, Status: "no-disponible", Reason: "package-lock.json il·legible"}
	}
	if pj.Version == pl.Version {
		return nil, Outcome{Name: name, Status: "ok"}
	}
	return []Proposal{newProposal("dependencia", Suggeriment,
		fmt.Sprintf("package-lock.json desincronitzat (%s, el paquet diu %s)", pl.Version, pj.Version),
		"El lockfile no reflecteix la versió del paquet: un `npm install` el resincronitza i evita instal·lacions impredictibles.",
		"petit", "desktop/package-lock.json")}, Outcome{Name: name, Status: "proposta"}
}

// checkBackendFresc proposa reconstruir el backend empaquetat si és més
// vell que l'últim commit. Tot local: funciona sense xarxa.
func checkBackendFresc(ctx context.Context, root string) ([]Proposal, Outcome) {
	const name = "backend-fresc"
	exe := "gregal"
	if runtime.GOOS == "windows" {
		exe = "gregal.exe"
	}
	bin := filepath.Join(root, "desktop", "backend", exe)
	fi, err := os.Stat(bin)
	if err != nil {
		return nil, Outcome{Name: name, Status: "no-aplica", Reason: "sense backend empaquetat"}
	}
	raw, err := runCmd(ctx, 15*time.Second, root, "git", "log", "-1", "--format=%ct")
	if err != nil {
		return nil, Outcome{Name: name, Status: "no-disponible", Reason: "git no disponible"}
	}
	ts, err := strconv.ParseInt(strings.TrimSpace(string(raw)), 10, 64)
	if err != nil {
		return nil, Outcome{Name: name, Status: "no-disponible", Reason: "data del commit il·legible"}
	}
	head := time.Unix(ts, 0)
	if !fi.ModTime().Before(head.Add(-2 * time.Minute)) {
		return nil, Outcome{Name: name, Status: "ok"}
	}
	return []Proposal{newProposal("fresco", Suggeriment,
		fmt.Sprintf("backend empaquetat desfasat (%s, últim commit %s)", fi.ModTime().Format("02/01 15:04"), head.Format("02/01 15:04")),
		"El binari de desktop/backend és més vell que HEAD: el portable provaria codi antic. Reconstrueix-lo amb `npm run dist-win` (o `dist`).",
		"mitjà", "desktop/backend")}, Outcome{Name: name, Status: "proposta"}
}

// checkHigieneRepo detecta binaris grossos seguits per git i directoris
// nous sense seguiment. Tot local: funciona sense xarxa.
func checkHigieneRepo(ctx context.Context, root string) ([]Proposal, Outcome) {
	const name = "higiene-repo"
	raw, err := runCmd(ctx, 15*time.Second, root, "git", "ls-files")
	if err != nil {
		return nil, Outcome{Name: name, Status: "no-disponible", Reason: "git no disponible"}
	}
	var props []Proposal
	for _, f := range strings.Split(string(raw), "\n") {
		f = strings.TrimSpace(f)
		if f == "" {
			continue
		}
		lower := strings.ToLower(f)
		if strings.HasSuffix(lower, ".exe") && !strings.Contains(filepath.ToSlash(f), "testdata") {
			if fi, err := os.Stat(filepath.Join(root, f)); err == nil && fi.Size() > 1024*1024 {
				props = append(props, newProposal("higiene", Info,
					"Binari seguit per git: "+f,
					"Els executables haurien de sortir de dist/ i les releases, no del repo: cada actualització hi afegeix megabytes d'historial.",
					"petit", "git"))
				break
			}
		}
	}
	raw, err = runCmd(ctx, 15*time.Second, root, "git", "status", "--porcelain")
	if err != nil {
		if len(props) > 0 {
			return props, Outcome{Name: name, Status: "proposta"}
		}
		return nil, Outcome{Name: name, Status: "no-disponible", Reason: "git status no disponible"}
	}
	// Només directoris sense seguiment (acaben en "/"): els fitxers
	// solts són feina en curs normal i el seu directori pare ja indica
	// el context. S'informa el camí relatiu real, no el primer
	// component: "internal/advise", no "internal".
	untracked := map[string]bool{}
	for _, line := range strings.Split(string(raw), "\n") {
		if len(line) < 4 || !strings.HasPrefix(line, "??") {
			continue
		}
		p := strings.TrimSpace(line[2:])
		if !strings.HasSuffix(p, "/") {
			continue
		}
		p = strings.TrimSuffix(p, "/")
		top := p
		if i := strings.IndexAny(p, "/\\"); i > 0 {
			top = p[:i]
		}
		if top == "dist" {
			continue
		}
		untracked[p] = true
	}
	names := []string{}
	for n := range untracked {
		names = append(names, n)
	}
	sortStrings(names)
	if len(names) > 3 {
		names = names[:3]
	}
	for _, n := range names {
		props = append(props, newProposal("higiene", Info,
			"Directori sense seguiment: "+n,
			"O s'integra amb un commit o s'afegeix al .gitignore: el que penja sense decidir contamina els builds i els diffs.",
			"petit", "git"))
	}
	if len(props) == 0 {
		return nil, Outcome{Name: name, Status: "ok"}
	}
	return props, Outcome{Name: name, Status: "proposta"}
}

// checkGoDesfasat llista mòduls Go actualitzables. Necessita xarxa:
// sense ella (o amb GOPROXY=off) torna "no-disponible", mai un error.
func checkGoDesfasat(ctx context.Context, root string) ([]Proposal, Outcome) {
	const name = "go-desfasat"
	if _, ok := readFile(root, "go.mod"); !ok {
		return nil, Outcome{Name: name, Status: "no-aplica", Reason: "sense go.mod"}
	}
	raw, err := runCmd(ctx, 60*time.Second, root, "go", "list", "-m", "-u", "all")
	if err != nil {
		return nil, Outcome{Name: name, Status: "no-disponible", Reason: "sense xarxa o proxy Go (go list -m -u ha fallat)"}
	}
	type item struct{ mod, from, to string }
	var items []item
	for _, line := range strings.Split(string(raw), "\n") {
		f := strings.Fields(line)
		// Format: mòdul versió [disponible]. Sense claudàtors = al dia.
		if len(f) == 3 && strings.HasPrefix(f[2], "[") && strings.HasSuffix(f[2], "]") {
			items = append(items, item{mod: f[0], from: f[1], to: strings.Trim(f[2], "[]")})
		}
	}
	if len(items) == 0 {
		return nil, Outcome{Name: name, Status: "ok"}
	}
	const max = 5
	var props []Proposal
	for _, it := range items {
		if len(props) >= max {
			break
		}
		props = append(props, newProposal("dependencia", Suggeriment,
			fmt.Sprintf("Go: %s actualitzable (%s → %s)", it.mod, it.from, it.to),
			"Hi ha una versió nova al proxy de mòduls. Revisa el changelog i puja-la amb `go get "+it.mod+"@"+it.to+"` si convé.",
			"petit", "go list -m -u"))
	}
	if extra := len(items) - len(props); extra > 0 {
		props = append(props, newProposal("dependencia", Info,
			fmt.Sprintf("i %d mòduls més actualitzables", extra),
			"La llista completa surt amb `go list -m -u all`.",
			"petit", "go list -m -u"))
	}
	return props, Outcome{Name: name, Status: "proposta"}
}

// checkNpmDesfasat llista paquets npm actualitzables. Necessita xarxa i
// npm: sense ells torna "no-disponible", mai un error.
func checkNpmDesfasat(ctx context.Context, root string) ([]Proposal, Outcome) {
	const name = "npm-desfasat"
	if _, ok := readFile(root, "desktop", "package.json"); !ok {
		return nil, Outcome{Name: name, Status: "no-aplica", Reason: "sense desktop/package.json"}
	}
	// npm outdated surt amb codi 1 quan hi ha paquets vells: no és un error.
	raw, _ := runCmd(ctx, 60*time.Second, filepath.Join(root, "desktop"), "npm", "outdated", "--json")
	var data map[string]struct {
		Current string `json:"current"`
		Wanted  string `json:"wanted"`
		Latest  string `json:"latest"`
	}
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return nil, Outcome{Name: name, Status: "no-disponible", Reason: "npm no disponible o sense xarxa"}
	}
	if err := json.Unmarshal(trimmed, &data); err != nil {
		// npm imprimeix avisos abans del JSON quan falla la xarxa.
		if i := bytes.LastIndexByte(trimmed, '{'); i >= 0 {
			if err2 := json.Unmarshal(trimmed[i:], &data); err2 != nil {
				return nil, Outcome{Name: name, Status: "no-disponible", Reason: "resposta de npm il·legible (sense xarxa?)"}
			}
		} else {
			return nil, Outcome{Name: name, Status: "no-disponible", Reason: "resposta de npm il·legible (sense xarxa?)"}
		}
	}
	if len(data) == 0 {
		return nil, Outcome{Name: name, Status: "ok"}
	}
	names := make([]string, 0, len(data))
	for n := range data {
		names = append(names, n)
	}
	sortStrings(names)
	const max = 5
	var props []Proposal
	for _, n := range names {
		if len(props) >= max {
			break
		}
		d := data[n]
		props = append(props, newProposal("dependencia", Suggeriment,
			fmt.Sprintf("npm: %s actualitzable (%s → %s)", n, d.Current, d.Latest),
			"Hi ha una versió nova al registre. Revisa-la i puja-la amb `npm install "+n+"@latest` a desktop/ si convé.",
			"petit", "npm outdated"))
	}
	if extra := len(names) - len(props); extra > 0 {
		props = append(props, newProposal("dependencia", Info,
			fmt.Sprintf("i %d paquets més actualitzables", extra),
			"La llista completa surt amb `npm outdated` a desktop/.",
			"petit", "npm outdated"))
	}
	return props, Outcome{Name: name, Status: "proposta"}
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}
