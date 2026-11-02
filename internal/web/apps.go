package web

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"gregal/internal/agent"
	"gregal/internal/procs"
	"gregal/internal/session"
)

// Apps gestionades: el que un vent (o l'agent) construeix i necessita un
// servidor propi (node, python, go, un servidor de joc).
//
// Abans el vent improvisava `python server.py &`: el procés moria en acabar
// l'ordre, el port canviava a cada intent, i matar-lo demanava un permís
// que el xat del vent no pot donar («REQUIRES APPROVAL»). Aquí el servidor
// de Gregal fa de supervisor:
//
//   - cada app té un id de 160 bits i un port lliure de 127.0.0.1; rep PORT,
//     HOST i BASE_PATH a l'entorn;
//   - s'hi arriba per /apps/<id>/ (un proxy invers del mateix Gregal, també
//     des del mòbil i amb WebSockets), sense token: l'id no és endevinable;
//   - si cau, es torna a engegar (fins a 5 caigudes en 2 minuts); les que
//     corrien tornen a arrencar amb el servidor (apps.json);
//   - start/stop/restart/status/logs per a l'eina `app` i GET/POST /api/apps.
//
// Seguretat: l'ordre passa pels mateixos filtres que bash (classificador,
// guarda de serveis, sandbox opcional) i la carpeta ha de ser dins del
// projecte (WindAutoAllow) o aprovada (política de l'agent). El proxy treu
// les credencials de Gregal (Authorization, galetes) de les peticions,
// no deixa posar galetes a l'origen de Gregal i serveix amb la mateixa CSP
// sandbox que les webs publicades: els scripts de l'app corren en un
// origen opac i no poden llegir el token del web de Gregal.

const (
	appPrefix         = "/apps/"
	appMaxPerOwner    = 20
	appCrashWindow    = 2 * time.Minute
	appMaxCrashes     = 5
	appReadyTimeout   = 25 * time.Second
	appLogTailDefault = 40
)

var reAppName = regexp.MustCompile(`[^a-z0-9-]+`)

// appRecord és el que es desa a apps.json.
type appRecord struct {
	ID      string    `json:"id"`
	Name    string    `json:"name"`
	Owner   string    `json:"owner,omitempty"`
	Dir     string    `json:"dir"`
	Command string    `json:"command"`
	Port    int       `json:"port"`
	Want    bool      `json:"want"` // ha de córrer (també després de reiniciar)
	Created time.Time `json:"created"`
}

// appRun és l'estat viu d'una app.
type appRun struct {
	rec      appRecord
	proc     *procs.Proc
	gen      int
	crashes  []time.Time
	restarts int
	gaveUp   bool
	lastExit string
}

type appManager struct {
	mu     sync.Mutex
	store  *procs.Store
	apps   map[string]*appRun
	once   sync.Once
	dialer func(port int) bool
}

var appsMgr = &appManager{store: procs.New(), apps: map[string]*appRun{}, dialer: portOpen}

func appsFile() string { return filepath.Join(filepath.Dir(session.DefaultDir()), "apps.json") }

// portOpen diu si algú escolta al port de 127.0.0.1.
func portOpen(port int) bool {
	c, err := net.DialTimeout("tcp", "127.0.0.1:"+strconv.Itoa(port), 400*time.Millisecond)
	if err != nil {
		return false
	}
	_ = c.Close()
	return true
}

// freePort demana un port lliure al sistema.
func freePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

// normAppName fa del nom un slug curt (també és el que es veu a la llista).
func normAppName(name string) string {
	n := reAppName.ReplaceAllString(strings.ToLower(strings.TrimSpace(name)), "-")
	n = strings.Trim(n, "-")
	if len(n) > 40 {
		n = strings.Trim(n[:40], "-")
	}
	return n
}

// boot carrega apps.json (un sol cop).
func (m *appManager) boot() {
	m.once.Do(func() {
		raw, err := os.ReadFile(appsFile())
		if err != nil {
			return
		}
		var recs []appRecord
		if json.Unmarshal(raw, &recs) != nil {
			return
		}
		m.mu.Lock()
		defer m.mu.Unlock()
		for _, r := range recs {
			m.apps[r.ID] = &appRun{rec: r}
		}
	})
}

// autostart torna a engegar les apps que corrien quan el servidor es va
// aturar. Només el crida Hub.Run: carregar el registre (boot) no engega res.
func (m *appManager) autostart() {
	m.boot()
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, run := range m.apps {
		if run.rec.Want && run.proc == nil {
			if info, err := os.Stat(run.rec.Dir); err == nil && info.IsDir() {
				_ = m.launchLocked(run)
			}
		}
	}
}

// saveLocked desa el registre (sense l'estat viu).
func (m *appManager) saveLocked() {
	recs := make([]appRecord, 0, len(m.apps))
	for _, a := range m.apps {
		recs = append(recs, a.rec)
	}
	sort.Slice(recs, func(i, j int) bool { return recs[i].Created.Before(recs[j].Created) })
	raw, err := json.MarshalIndent(recs, "", "  ")
	if err != nil {
		return
	}
	_ = os.MkdirAll(filepath.Dir(appsFile()), 0o700)
	tmp := appsFile() + ".tmp"
	if os.WriteFile(tmp, raw, 0o600) == nil {
		_ = os.Rename(tmp, appsFile())
	}
}

// launchLocked engega el procés de l'app i el vigila.
func (m *appManager) launchLocked(run *appRun) error {
	if run.rec.Port == 0 || m.dialer(run.rec.Port) {
		// El port d'abans l'ocupa un altre: un de nou (l'enllaç no canvia).
		p, err := freePort()
		if err != nil {
			return err
		}
		run.rec.Port = p
	}
	env := []string{
		"PORT=" + strconv.Itoa(run.rec.Port),
		"HOST=127.0.0.1",
		"BASE_PATH=" + appPrefix + run.rec.ID + "/",
	}
	p, err := m.store.StartEnv(context.Background(), "app:"+run.rec.ID, run.rec.Dir, run.rec.Command, env)
	if err != nil {
		return err
	}
	run.gen++
	run.proc = p
	run.gaveUp = false
	gen := run.gen
	go m.supervise(run.rec.ID, p, gen)
	return nil
}

// supervise espera que el procés acabi i, si no l'ha aturat ningú, el torna
// a engegar amb espera creixent; massa caigudes seguides → «crashed».
func (m *appManager) supervise(id string, p *procs.Proc, gen int) {
	for !p.Wait(time.Hour) {
	}
	m.mu.Lock()
	run := m.apps[id]
	if run == nil || run.gen != gen || !run.rec.Want {
		m.mu.Unlock()
		return
	}
	_, _, exit := p.Output(1 << 30)
	if exit != nil {
		run.lastExit = "exit " + strconv.Itoa(*exit)
	}
	now := time.Now()
	kept := run.crashes[:0]
	for _, t := range run.crashes {
		if now.Sub(t) < appCrashWindow {
			kept = append(kept, t)
		}
	}
	run.crashes = append(kept, now)
	if len(run.crashes) >= appMaxCrashes {
		run.gaveUp = true
		m.mu.Unlock()
		return
	}
	wait := time.Duration(1<<uint(len(run.crashes)-1)) * time.Second
	m.mu.Unlock()
	time.Sleep(wait)
	m.mu.Lock()
	defer m.mu.Unlock()
	if run := m.apps[id]; run != nil && run.gen == gen && run.rec.Want {
		run.restarts++
		_ = m.launchLocked(run)
	}
}

// status és l'estat que es veu: running, starting, crashed o stopped.
func (m *appManager) statusLocked(run *appRun) string {
	switch {
	case run.proc != nil && run.proc.IsRunning():
		if m.dialer(run.rec.Port) {
			return "running"
		}
		return "starting"
	case run.gaveUp:
		return "crashed"
	case run.rec.Want && run.proc != nil:
		return "restarting"
	}
	return "stopped"
}

func (m *appManager) find(owner, ref string) *appRun {
	ref = strings.TrimSpace(ref)
	if run, ok := m.apps[ref]; ok && run.rec.Owner == owner {
		return run
	}
	name := normAppName(ref)
	for _, run := range m.apps {
		if run.rec.Owner == owner && run.rec.Name == name {
			return run
		}
	}
	return nil
}

// start engega (o torna a engegar amb l'ordre nova) l'app «name».
func (m *appManager) start(owner, name, dir, command string) (*appRun, error) {
	m.boot()
	name = normAppName(name)
	if name == "" {
		name = normAppName(filepath.Base(dir))
	}
	if name == "" {
		return nil, errors.New("the app needs a name")
	}
	if strings.TrimSpace(command) == "" {
		return nil, errors.New("the app needs a command (e.g. node server.js)")
	}
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		return nil, fmt.Errorf("dir %q is not a folder", dir)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	run := m.find(owner, name)
	if run == nil {
		n := 0
		for _, r := range m.apps {
			if r.rec.Owner == owner {
				n++
			}
		}
		if n >= appMaxPerOwner {
			return nil, fmt.Errorf("at most %d apps: stop and remove one first", appMaxPerOwner)
		}
		id, err := newShareID()
		if err != nil {
			return nil, err
		}
		run = &appRun{rec: appRecord{ID: id, Name: name, Owner: owner, Created: time.Now().UTC()}}
		m.apps[id] = run
	} else if run.proc != nil {
		run.gen++ // el supervisor de l'anterior no l'ha de reengegar
		run.proc.Kill()
		run.proc.Wait(5 * time.Second)
		run.proc = nil
	}
	run.rec.Dir, run.rec.Command, run.rec.Want = dir, strings.TrimSpace(command), true
	run.crashes, run.gaveUp, run.lastExit = nil, false, ""
	if err := m.launchLocked(run); err != nil {
		run.rec.Want = false
		m.saveLocked()
		return nil, err
	}
	m.saveLocked()
	return run, nil
}

// waitReady espera que l'app escolti (o que caigui).
func (m *appManager) waitReady(run *appRun, d time.Duration) string {
	deadline := time.Now().Add(d)
	for {
		m.mu.Lock()
		st := m.statusLocked(run)
		m.mu.Unlock()
		if st == "running" || st == "crashed" || st == "stopped" || time.Now().After(deadline) {
			return st
		}
		time.Sleep(300 * time.Millisecond)
	}
}

func (m *appManager) stop(owner, ref string, remove bool) (*appRun, error) {
	m.boot()
	m.mu.Lock()
	defer m.mu.Unlock()
	run := m.find(owner, ref)
	if run == nil {
		return nil, fmt.Errorf("no app called %q", ref)
	}
	run.rec.Want = false
	run.gen++
	if run.proc != nil {
		p := run.proc
		m.mu.Unlock()
		p.Kill()
		p.Wait(5 * time.Second)
		m.mu.Lock()
	}
	if remove {
		delete(m.apps, run.rec.ID)
	}
	m.saveLocked()
	return run, nil
}

func (m *appManager) restart(owner, ref string) (*appRun, error) {
	m.mu.Lock()
	run := m.find(owner, ref)
	m.mu.Unlock()
	if run == nil {
		return nil, fmt.Errorf("no app called %q", ref)
	}
	return m.start(owner, run.rec.Name, run.rec.Dir, run.rec.Command)
}

// logs torna les últimes n línies de sortida.
func (m *appManager) logs(run *appRun, n int) string {
	m.mu.Lock()
	p := run.proc
	m.mu.Unlock()
	if p == nil {
		return "(no output: the app is not running)"
	}
	lines, _, _ := p.Output(0)
	if n <= 0 {
		n = appLogTailDefault
	}
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	var b strings.Builder
	for _, l := range lines {
		if l.Stream == "err" {
			b.WriteString("! ")
		}
		b.WriteString(l.Text)
		b.WriteByte('\n')
	}
	if b.Len() == 0 {
		return "(no output yet)"
	}
	return strings.TrimRight(b.String(), "\n")
}

// appView és la vista d'una app per a l'API i l'eina.
func (m *appManager) view(run *appRun, base string) map[string]any {
	m.mu.Lock()
	defer m.mu.Unlock()
	return map[string]any{
		"id": run.rec.ID, "name": run.rec.Name, "dir": run.rec.Dir, "command": run.rec.Command,
		"port": run.rec.Port, "status": m.statusLocked(run), "restarts": run.restarts,
		"last_exit": run.lastExit, "path": appPrefix + run.rec.ID + "/", "url": absLink(base, appPrefix+run.rec.ID+"/"),
		"created": run.rec.Created,
	}
}

func (m *appManager) list(owner string) []*appRun {
	m.boot()
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []*appRun
	for _, r := range m.apps {
		if r.rec.Owner == owner {
			out = append(out, r)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].rec.Created.Before(out[j].rec.Created) })
	return out
}

// --- enllaços absoluts ---

// lastClientBase és l'adreça per on ha entrat l'últim client de l'API: els
// torns del xat principal no porten la petició al context.
var lastClientBase atomic.Value

func noteClientBase(r *http.Request) {
	if b := requestBase(r); b != "" {
		lastClientBase.Store(b)
	}
}

// runServerTool executa les eines que només té el servidor web.
func (s *Server) runServerTool(ctx context.Context, workspace, name, argsJSON string) (string, error) {
	if name == "wind" {
		return s.runWindTool(ctx, argsJSON)
	}
	return s.runBuildTool(ctx, workspace, name, argsJSON)
}

// runBuildTool són les eines del servidor que també tenen els vents.
func (s *Server) runBuildTool(ctx context.Context, workspace, name, argsJSON string) (string, error) {
	switch name {
	case "app":
		return s.runAppTool(ctx, workspace, argsJSON)
	case "publish":
		return s.runPublishTool(ctx, workspace, argsJSON)
	}
	return "", fmt.Errorf("unknown tool %q", name)
}

// absLink fa absoluta una ruta del servidor: amb GREGAL_PUBLIC_URL si n'hi
// ha, si no amb l'adreça per on ha entrat el client del torn. Sense cap de
// les dues, la ruta tal qual (el client hi posa la seva adreça).
func absLink(base, p string) string {
	if b := publicBase(); b != "" {
		return b + p
	}
	if base != "" {
		return base + p
	}
	if b, _ := lastClientBase.Load().(string); b != "" {
		return b + p
	}
	return p
}

type linkBaseKey struct{}

// withLinkBase recorda per on ha entrat el client (http://10.0.2.2:8097 des
// de l'emulador, la IP de la xarxa des del mòbil): els enllaços que torna el
// vent s'han de poder obrir des d'aquell mateix aparell.
func withLinkBase(ctx context.Context, r *http.Request) context.Context {
	if r == nil {
		return ctx
	}
	return context.WithValue(ctx, linkBaseKey{}, requestBase(r))
}

func linkBaseOf(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	b, _ := ctx.Value(linkBaseKey{}).(string)
	return b
}

// requestBase és scheme://host de la petició (respectant un proxy davant).
func requestBase(r *http.Request) string {
	host := r.Host
	if h := strings.TrimSpace(r.Header.Get("X-Forwarded-Host")); h != "" {
		host = strings.TrimSpace(strings.Split(h, ",")[0])
	}
	if host == "" || strings.ContainsAny(host, " /\\@") {
		return ""
	}
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if p := strings.ToLower(strings.TrimSpace(r.Header.Get("X-Forwarded-Proto"))); p == "https" || p == "http" {
		scheme = p
	}
	return scheme + "://" + host
}

// --- eina `app` ---

// runAppTool executa l'eina app per a un torn (vent o agent).
func (s *Server) runAppTool(ctx context.Context, workspace, argsJSON string) (string, error) {
	var a struct {
		Action  string `json:"action"`
		Name    string `json:"name"`
		Dir     string `json:"dir"`
		Command string `json:"command"`
		Lines   int    `json:"lines"`
	}
	if err := json.Unmarshal([]byte(agent.ResolveArgs(workspace, argsJSON)), &a); err != nil {
		return "", errors.New("unreadable arguments")
	}
	owner := s.usuari()
	base := linkBaseOf(ctx)
	switch strings.TrimSpace(a.Action) {
	case "start":
		dir := strings.TrimSpace(a.Dir)
		if dir == "" {
			dir = workspace
		}
		if dir == "" {
			return "", errors.New("dir is required: the project folder")
		}
		if s.user != nil {
			if err := s.user.Allow(dir); err != nil {
				return "", err
			}
		}
		run, err := appsMgr.start(owner, a.Name, dir, a.Command)
		if err != nil {
			return "", err
		}
		st := appsMgr.waitReady(run, appReadyTimeout)
		v := appsMgr.view(run, base)
		var b strings.Builder
		switch st {
		case "running":
			fmt.Fprintf(&b, "App %q is running.\nLink (works on the phone too): %s\nLocal test: curl -s http://127.0.0.1:%d/\nFolder: %s\nIt keeps running after this turn and restarts if it crashes.", run.rec.Name, v["url"], run.rec.Port, run.rec.Dir)
		default:
			fmt.Fprintf(&b, "App %q did NOT come up (status: %s). It must listen on 127.0.0.1 at the port in $PORT (%d). Fix it and call app start again.", run.rec.Name, st, run.rec.Port)
		}
		b.WriteString("\n\nLast output:\n" + appsMgr.logs(run, 20))
		return b.String(), nil
	case "stop":
		run, err := appsMgr.stop(owner, firstNonEmpty(a.Name, a.Dir), false)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("App %q stopped (the link answers again when it is started).", run.rec.Name), nil
	case "restart":
		run, err := appsMgr.restart(owner, a.Name)
		if err != nil {
			return "", err
		}
		st := appsMgr.waitReady(run, appReadyTimeout)
		return fmt.Sprintf("App %q restarted: %s at %s\n\nLast output:\n%s", run.rec.Name, st, appsMgr.view(run, base)["url"], appsMgr.logs(run, 20)), nil
	case "status", "logs":
		appsMgr.boot()
		appsMgr.mu.Lock()
		run := appsMgr.find(owner, a.Name)
		appsMgr.mu.Unlock()
		if run == nil {
			return "", fmt.Errorf("no app called %q (use action list)", a.Name)
		}
		v := appsMgr.view(run, base)
		out := fmt.Sprintf("App %q: %s, port %v, %v restarts, link %s, folder %s", run.rec.Name, v["status"], v["port"], v["restarts"], v["url"], run.rec.Dir)
		if a.Action == "logs" || v["status"] != "running" {
			out += "\n\nLast output:\n" + appsMgr.logs(run, a.Lines)
		}
		return out, nil
	case "list", "":
		runs := appsMgr.list(owner)
		if len(runs) == 0 {
			return "No apps yet.", nil
		}
		var b strings.Builder
		for _, run := range runs {
			v := appsMgr.view(run, base)
			fmt.Fprintf(&b, "- %s: %s · %s · %s\n", run.rec.Name, v["status"], v["url"], run.rec.Dir)
		}
		return strings.TrimRight(b.String(), "\n"), nil
	}
	return "", fmt.Errorf("unknown action %q (start, status, logs, restart, stop, list)", a.Action)
}

// --- API ---

// handleApps: GET llista les apps de l'usuari; POST {action,id|name,dir,command}
// les engega, atura, reinicia o esborra; GET ?id=…&logs=1 en dona la sortida.
func (s *Server) handleApps(w http.ResponseWriter, r *http.Request) {
	owner := s.usuari()
	base := requestBase(r)
	switch r.Method {
	case http.MethodGet:
		if id := strings.TrimSpace(r.URL.Query().Get("id")); id != "" {
			appsMgr.boot()
			appsMgr.mu.Lock()
			run := appsMgr.find(owner, id)
			appsMgr.mu.Unlock()
			if run == nil {
				http.NotFound(w, r)
				return
			}
			v := appsMgr.view(run, base)
			if r.URL.Query().Get("logs") != "" {
				n, _ := strconv.Atoi(r.URL.Query().Get("lines"))
				if n <= 0 || n > 500 {
					n = 200
				}
				v["logs"] = appsMgr.logs(run, n)
			}
			writeJSON(w, map[string]any{"app": v})
			return
		}
		items := []map[string]any{}
		for _, run := range appsMgr.list(owner) {
			items = append(items, appsMgr.view(run, base))
		}
		writeJSON(w, map[string]any{"apps": items})
	case http.MethodPost:
		var req struct {
			Action  string `json:"action"`
			ID      string `json:"id"`
			Name    string `json:"name"`
			Dir     string `json:"dir"`
			Command string `json:"command"`
		}
		if !decodeSmall(w, r, &req) {
			return
		}
		ref := firstNonEmpty(req.ID, req.Name)
		var run *appRun
		var err error
		switch req.Action {
		case "start":
			// Des de l'API (la persona mateixa): ordre i carpeta seves, però
			// la carpeta ha de ser seva i l'ordre passa la mateixa guarda.
			dir, derr := s.resolveFilePath(req.Dir)
			if derr != nil {
				http.Error(w, derr.Error(), http.StatusForbidden)
				return
			}
			run, err = appsMgr.start(owner, firstNonEmpty(req.Name, filepath.Base(dir)), dir, req.Command)
			if err == nil {
				appsMgr.waitReady(run, 10*time.Second)
			}
		case "stop":
			run, err = appsMgr.stop(owner, ref, false)
		case "remove":
			run, err = appsMgr.stop(owner, ref, true)
		case "restart":
			run, err = appsMgr.restart(owner, ref)
			if err == nil {
				appsMgr.waitReady(run, 10*time.Second)
			}
		default:
			http.Error(w, "action must be start, stop, restart or remove", http.StatusBadRequest)
			return
		}
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, map[string]any{"app": appsMgr.view(run, base)})
	default:
		http.Error(w, "GET or POST", http.StatusMethodNotAllowed)
	}
}

// --- proxy /apps/<id>/ ---

// storageShim: als orígens opacs (CSP sandbox) localStorage i sessionStorage
// llancen SecurityError i un joc que hi desa el rècord es trenca en
// carregar. El shim els substitueix per un magatzem en memòria només si
// l'accés falla.
const storageShim = `<script>(function(){function m(){var d={};return{getItem:function(k){return Object.prototype.hasOwnProperty.call(d,k)?d[k]:null},setItem:function(k,v){d[k]=String(v)},removeItem:function(k){delete d[k]},clear:function(){d={}},key:function(i){var k=Object.keys(d);return i<k.length?k[i]:null},get length(){return Object.keys(d).length}}}["localStorage","sessionStorage"].forEach(function(n){try{window[n].getItem("_")}catch(e){try{Object.defineProperty(window,n,{value:m(),configurable:true})}catch(_){}}})})();</script>`

var reHeadOpen = regexp.MustCompile(`(?i)<head(\s[^>]*)?>`)
var reScript = regexp.MustCompile(`(?i)<script`)
var reHTMLOpen = regexp.MustCompile(`(?i)<html(\s[^>]*)?>`)

// injectStorageShim posa el shim al principi del <head> (o del document),
// només a les pàgines que ja porten scripts.
func injectStorageShim(page []byte) []byte {
	// Sense scripts no hi ha res que es pugui trencar (i una pàgina que
	// Gregal mateix ha fet a partir de markdown ha de quedar sense scripts).
	if !reScript.Match(page) || bytes.Contains(page, []byte(`"localStorage","sessionStorage"`)) {
		return page
	}
	for _, re := range []*regexp.Regexp{reHeadOpen, reHTMLOpen} {
		if loc := re.FindIndex(page); loc != nil {
			out := make([]byte, 0, len(page)+len(storageShim))
			out = append(out, page[:loc[1]]...)
			out = append(out, storageShim...)
			return append(out, page[loc[1]:]...)
		}
	}
	return append([]byte(storageShim), page...)
}

// handleAppProxy serveix /apps/<id>/<camí> fent de proxy cap a l'app.
func handleAppProxy(w http.ResponseWriter, r *http.Request) {
	appsMgr.boot()
	rest := strings.TrimPrefix(r.URL.Path, appPrefix)
	id, sub, hasSlash := strings.Cut(rest, "/")
	if !reShareID.MatchString(id) {
		http.NotFound(w, r)
		return
	}
	appsMgr.mu.Lock()
	run := appsMgr.apps[id]
	var st string
	var port int
	if run != nil {
		st, port = appsMgr.statusLocked(run), run.rec.Port
	}
	appsMgr.mu.Unlock()
	if run == nil {
		http.NotFound(w, r)
		return
	}
	if !hasSlash {
		http.Redirect(w, r, appPrefix+id+"/", http.StatusFound)
		return
	}
	prefix := appPrefix + id
	h := w.Header()
	h.Set("Access-Control-Allow-Origin", "*")
	if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
		h.Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		if hdr := r.Header.Get("Access-Control-Request-Headers"); hdr != "" {
			h.Set("Access-Control-Allow-Headers", hdr)
		}
		h.Set("Access-Control-Max-Age", "600")
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if st != "running" {
		h.Set("Content-Type", "text/html; charset=utf-8")
		h.Set("Retry-After", "3")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = io.WriteString(w, `<!doctype html><meta charset="utf-8"><meta name="viewport" content="width=device-width"><title>App</title><body style="font-family:sans-serif;padding:2em"><p>Aquesta app ara no està en marxa (`+st+`). Torna-ho a provar d'aquí a una estona.</p><p>This app is not running right now (`+st+`).</p>`)
		return
	}
	target := &url.URL{Scheme: "http", Host: "127.0.0.1:" + strconv.Itoa(port)}
	proxy := &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.SetURL(target)
			pr.Out.URL.Path = "/" + sub
			pr.Out.URL.RawPath = ""
			pr.Out.Host = target.Host
			// Les credencials de Gregal no surten mai cap a l'app.
			pr.Out.Header.Del("Authorization")
			pr.Out.Header.Del("Cookie")
			pr.Out.Header.Del("Accept-Encoding") // el transport descomprimeix: es pot injectar el shim
			pr.Out.Header.Set("X-Forwarded-Prefix", prefix)
			pr.SetXForwarded()
		},
		ModifyResponse: func(res *http.Response) error {
			res.Header.Del("Set-Cookie")
			res.Header.Set("Content-Security-Policy", siteCSP)
			// ACAO ja és a la resposta (h.Set de dalt) i el proxy hi AFEGEIX
			// les capçaleres de l'app: si l'app també el posa, sortiria «*, *»
			// i el navegador rebutja la resposta.
			res.Header.Del("Access-Control-Allow-Origin")
			res.Header.Set("X-Content-Type-Options", "nosniff")
			res.Header.Set("Referrer-Policy", "no-referrer")
			if loc := res.Header.Get("Location"); strings.HasPrefix(loc, "/") && !strings.HasPrefix(loc, "//") {
				res.Header.Set("Location", prefix+loc)
			}
			if strings.HasPrefix(strings.ToLower(res.Header.Get("Content-Type")), "text/html") && res.Header.Get("Content-Encoding") == "" && res.StatusCode != http.StatusSwitchingProtocols {
				body, err := io.ReadAll(io.LimitReader(res.Body, 20<<20))
				_ = res.Body.Close()
				if err != nil {
					return err
				}
				body = injectStorageShim(body)
				res.Body = io.NopCloser(bytes.NewReader(body))
				res.ContentLength = int64(len(body))
				res.Header.Set("Content-Length", strconv.Itoa(len(body)))
			}
			return nil
		},
		ErrorHandler: func(w http.ResponseWriter, _ *http.Request, err error) {
			http.Error(w, "the app did not answer", http.StatusBadGateway)
		},
	}
	proxy.ServeHTTP(w, r)
}
