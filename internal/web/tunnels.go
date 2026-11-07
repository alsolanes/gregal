package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"gregal/internal/agent"
	"gregal/internal/procs"
	"gregal/internal/session"
)

// Eina `tunnel`: posa a internet un servei local amb Cloudflare (cloudflared),
// com fa qui té un agent amb terminal al servidor. Dues maneres:
//
//   - Ràpida (sense hostname): un enllaç https://…trycloudflare.com que no
//     demana compte de Cloudflare. Canvia si el túnel es torna a engegar.
//   - Amb domini (hostname): un túnel amb nom (cloudflared tunnel login +
//     create, un cop: `gregalctl tunnel-login`). Gregal hi afegeix la ruta DNS
//     i manté un únic procés amb totes les regles d'entrada (ingress).
//
// El Gregal supervisa els processos com les apps (apps.go): tornen a
// arrencar si cauen i després de reiniciar el servidor. No cal root: la
// configuració viu a la carpeta de dades.

const (
	tunnelMaxPerOwner = 20
	tunnelNamedID     = "named"
)

var reTrycloudflare = regexp.MustCompile(`https://[a-z0-9-]+\.trycloudflare\.com`)

type tunnelRecord struct {
	ID       string    `json:"id"`
	Name     string    `json:"name"`
	Owner    string    `json:"owner,omitempty"`
	Target   string    `json:"target"`             // http://127.0.0.1:PORT
	Hostname string    `json:"hostname,omitempty"` // "" = enllaç ràpid
	Tunnel   string    `json:"tunnel,omitempty"`   // túnel amb nom (només amb hostname)
	Want     bool      `json:"want"`
	Created  time.Time `json:"created"`
}

type tunnelRun struct {
	rec      tunnelRecord
	proc     *procs.Proc
	gen      int
	crashes  []time.Time
	gaveUp   bool
	lastExit string
}

type tunnelManager struct {
	mu    sync.Mutex
	store *procs.Store
	runs  map[string]*tunnelRun // els ràpids, i el procés compartit «named»
	recs  map[string]*tunnelRecord
	once  sync.Once
	// bin troba cloudflared (els tests el substitueixen).
	bin func() (string, error)
	// route afegeix la ruta DNS d'un hostname al túnel amb nom.
	route func(bin, tunnel, hostname string) (string, error)
}

var tunnelsMgr = &tunnelManager{store: procs.New(), runs: map[string]*tunnelRun{}, recs: map[string]*tunnelRecord{},
	bin: cloudflaredBin, route: cloudflaredRoute}

func tunnelsFile() string { return filepath.Join(filepath.Dir(session.DefaultDir()), "tunnels.json") }
func tunnelConfigFile() string {
	return filepath.Join(filepath.Dir(session.DefaultDir()), "cloudflared", "config.yml")
}

// cloudflaredBin troba l'executable: GREGAL_CLOUDFLARED, o el del PATH.
func cloudflaredBin() (string, error) {
	if p := strings.TrimSpace(os.Getenv("GREGAL_CLOUDFLARED")); p != "" {
		return p, nil
	}
	p, err := exec.LookPath("cloudflared")
	if err != nil {
		return "", errors.New("cloudflared is not installed on the server. On Debian/Ubuntu the administrator can run: sudo gregalctl cloudflared-install")
	}
	return p, nil
}

// cloudflaredRoute afegeix la ruta DNS d'un hostname al túnel amb nom. Cal
// passar-hi l'identificador del túnel: amb un nom, `tunnel route dns` d'aquesta
// versió de cloudflared crea el CNAME apuntant a un altre túnel (el primer de la
// llista) i l'amfitrió acaba donant 404 encara que el connector estigui bé.
func cloudflaredRoute(bin, tunnel, hostname string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	tunnel = resolveTunnelID(ctx, bin, tunnel)
	out, err := exec.CommandContext(ctx, bin, "tunnel", "route", "dns", "--overwrite-dns", tunnel, hostname).CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

var reTunnelID = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// resolveTunnelID tradueix el nom d'un túnel al seu identificador consultant la
// llista de túnels. Si ja és un identificador, o no es pot resoldre, torna el
// valor original perquè cloudflared digui què passa amb les seves paraules.
func resolveTunnelID(ctx context.Context, bin, tunnel string) string {
	tunnel = strings.TrimSpace(tunnel)
	if tunnel == "" || reTunnelID.MatchString(tunnel) {
		return tunnel
	}
	out, err := exec.CommandContext(ctx, bin, "tunnel", "list", "--output", "json").Output()
	if err != nil {
		return tunnel
	}
	var list []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	if err := json.Unmarshal(out, &list); err != nil {
		return tunnel
	}
	for _, t := range list {
		if strings.EqualFold(strings.TrimSpace(t.Name), tunnel) && t.ID != "" {
			return t.ID
		}
	}
	return tunnel
}

func shQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'" }

// namedTunnelConfig escriu la configuració del túnel amb nom. Les credencials
// de l'entorn només s'hi escriuen si són del mateix túnel: les credencials
// d'un túnel només valen per al seu propi ID i barrejar-les fa que cloudflared
// no autentiqui mai (amb un missatge d'error que enganya).
func namedTunnelConfig(tunnel string, ingress []tunnelRecord) string {
	var b strings.Builder
	fmt.Fprintf(&b, "tunnel: %s\n", tunnel)
	if creds := strings.TrimSpace(os.Getenv("GREGAL_CF_CREDENTIALS")); creds != "" {
		// Només si les credencials es poden llegir i són d'aquest túnel;
		// si no, cloudflared farà servir el certificat d'origen (cert.pem).
		if c, err := loadTunnelCredsID(creds); err == nil && c != "" && c == tunnel {
			fmt.Fprintf(&b, "credentials-file: %s\n", creds)
		}
	}
	b.WriteString("ingress:\n")
	for _, r := range ingress {
		fmt.Fprintf(&b, "  - hostname: %s\n    service: %s\n", r.Hostname, r.Target)
	}
	b.WriteString("  - service: http_status:404\n")
	return b.String()
}

// loadTunnelCredsID llegeix el TunnelID del fitxer de credencials d'un túnel
// (el fitxer que genera `cloudflared tunnel create`). Això permet comprovar
// que les credencials de l'entorn són realment del túnel que s'engegarà.
func loadTunnelCredsID(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	var c struct {
		TunnelID string `json:"TunnelID"`
	}
	if err := json.Unmarshal(raw, &c); err != nil {
		return "", err
	}
	return strings.TrimSpace(c.TunnelID), nil
}

// boot carrega tunnels.json (un sol cop).
func (m *tunnelManager) boot() {
	m.once.Do(func() {
		raw, err := os.ReadFile(tunnelsFile())
		if err != nil {
			return
		}
		var recs []tunnelRecord
		if json.Unmarshal(raw, &recs) != nil {
			return
		}
		m.mu.Lock()
		defer m.mu.Unlock()
		for i := range recs {
			r := recs[i]
			m.recs[r.ID] = &r
		}
	})
}

// autostart torna a obrir els túnels que estaven oberts.
func (m *tunnelManager) autostart() {
	m.boot()
	m.mu.Lock()
	defer m.mu.Unlock()
	bin, err := m.bin()
	if err != nil {
		return
	}
	named := false
	for _, r := range m.recs {
		if !r.Want {
			continue
		}
		if r.Hostname != "" {
			named = true
			continue
		}
		_ = m.launchQuickLocked(bin, r)
	}
	if named {
		_ = m.relaunchNamedLocked(bin)
	}
}

func (m *tunnelManager) saveLocked() {
	recs := make([]tunnelRecord, 0, len(m.recs))
	for _, r := range m.recs {
		recs = append(recs, *r)
	}
	sort.Slice(recs, func(i, j int) bool { return recs[i].Created.Before(recs[j].Created) })
	raw, err := json.MarshalIndent(recs, "", "  ")
	if err != nil {
		return
	}
	_ = os.MkdirAll(filepath.Dir(tunnelsFile()), 0o700)
	tmp := tunnelsFile() + ".tmp"
	if os.WriteFile(tmp, raw, 0o600) == nil {
		_ = os.Rename(tmp, tunnelsFile())
	}
}

func (m *tunnelManager) launchQuickLocked(bin string, r *tunnelRecord) error {
	run := m.runs[r.ID]
	if run == nil {
		run = &tunnelRun{rec: *r}
		m.runs[r.ID] = run
	}
	run.rec = *r
	cmd := shQuote(bin) + " tunnel --no-autoupdate --url " + shQuote(r.Target)
	return m.launchLocked(run, cmd)
}

// relaunchNamedLocked reescriu la configuració del túnel amb nom amb totes
// les regles i torna a engegar el seu procés (o l'atura si no en queda cap).
func (m *tunnelManager) relaunchNamedLocked(bin string) error {
	var ingress []tunnelRecord
	tunnel := ""
	for _, r := range m.recs {
		if r.Want && r.Hostname != "" {
			ingress = append(ingress, *r)
			if tunnel == "" {
				tunnel = r.Tunnel
			}
		}
	}
	run := m.runs[tunnelNamedID]
	if len(ingress) == 0 {
		if run != nil && run.proc != nil {
			run.gen++
			run.proc.Kill()
			run.proc = nil
		}
		delete(m.runs, tunnelNamedID)
		return nil
	}
	sort.Slice(ingress, func(i, j int) bool { return ingress[i].Hostname < ingress[j].Hostname })
	// El nom s'escriu a la configuració com a identificador: així `run` no depèn
	// de com cloudflared resol els noms (que en aquesta versió falla).
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	tunnel = resolveTunnelID(ctx, bin, tunnel)
	cancel()
	cfg := namedTunnelConfig(tunnel, ingress)
	if err := os.MkdirAll(filepath.Dir(tunnelConfigFile()), 0o700); err != nil {
		return err
	}
	if err := os.WriteFile(tunnelConfigFile(), []byte(cfg), 0o600); err != nil {
		return err
	}
	if run == nil {
		run = &tunnelRun{rec: tunnelRecord{ID: tunnelNamedID, Name: tunnel, Tunnel: tunnel, Want: true}}
		m.runs[tunnelNamedID] = run
	}
	run.rec.Tunnel, run.rec.Name = tunnel, tunnel
	cmd := shQuote(bin) + " tunnel --no-autoupdate --config " + shQuote(tunnelConfigFile()) + " run " + shQuote(tunnel)
	return m.launchLocked(run, cmd)
}

func (m *tunnelManager) launchLocked(run *tunnelRun, cmd string) error {
	if run.proc != nil {
		run.gen++
		run.proc.Kill()
		run.proc.Wait(5 * time.Second)
		run.proc = nil
	}
	p, err := m.store.StartEnv(context.Background(), "tunnel:"+run.rec.ID, filepath.Dir(tunnelsFile()), cmd, nil)
	if err != nil {
		return err
	}
	run.gen++
	run.proc, run.gaveUp, run.lastExit = p, false, ""
	go m.supervise(run.rec.ID, p, run.gen, cmd)
	return nil
}

// supervise torna a engegar el túnel si cau (com les apps).
func (m *tunnelManager) supervise(id string, p *procs.Proc, gen int, cmd string) {
	for !p.Wait(time.Hour) {
	}
	m.mu.Lock()
	run := m.runs[id]
	if run == nil || run.gen != gen {
		m.mu.Unlock()
		return
	}
	if _, _, exit := p.Output(1 << 30); exit != nil {
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
	if run := m.runs[id]; run != nil && run.gen == gen {
		_ = m.launchLocked(run, cmd)
	}
}

// quickURL és l'enllaç trycloudflare que el procés ha escrit (o "").
func quickURL(p *procs.Proc) string {
	if p == nil {
		return ""
	}
	lines, _, _ := p.Output(0)
	for i := len(lines) - 1; i >= 0; i-- {
		if u := reTrycloudflare.FindString(lines[i].Text); u != "" {
			return u
		}
	}
	return ""
}

func (m *tunnelManager) statusLocked(run *tunnelRun) string {
	switch {
	case run == nil:
		return "stopped"
	case run.proc != nil && run.proc.IsRunning():
		return "running"
	case run.gaveUp:
		return "crashed"
	}
	return "stopped"
}

func (m *tunnelManager) find(owner, ref string) *tunnelRecord {
	ref = strings.TrimSpace(ref)
	if r, ok := m.recs[ref]; ok && r.Owner == owner {
		return r
	}
	name := normAppName(ref)
	for _, r := range m.recs {
		if r.Owner == owner && (r.Name == name || (r.Hostname != "" && strings.EqualFold(r.Hostname, ref))) {
			return r
		}
	}
	return nil
}

// tunnelTarget resol el destí: un port, una URL local o una app gestionada.
func tunnelTarget(owner, target string) (string, error) {
	t := strings.TrimSpace(target)
	if t == "" {
		return "", errors.New("target is required: a port (3000), a local URL (http://127.0.0.1:3000) or the name of a managed app")
	}
	if strings.EqualFold(t, "gregal") {
		if gregalListenPort == 0 {
			return "", errors.New("the Gregal server port is unknown")
		}
		return "http://127.0.0.1:" + strconv.Itoa(gregalListenPort), nil
	}
	if n, err := strconv.Atoi(strings.TrimPrefix(t, ":")); err == nil && n > 0 && n < 65536 {
		return "http://127.0.0.1:" + strconv.Itoa(n), nil
	}
	if strings.HasPrefix(t, "http://") || strings.HasPrefix(t, "https://") {
		u, err := url.Parse(t)
		if err != nil || u.Host == "" {
			return "", fmt.Errorf("invalid target URL %q", t)
		}
		host := u.Hostname()
		if ip := net.ParseIP(host); host != "localhost" && (ip == nil || !ip.IsLoopback()) {
			return "", errors.New("a tunnel only exposes services of this machine: use localhost or 127.0.0.1")
		}
		return t, nil
	}
	appsMgr.boot()
	appsMgr.mu.Lock()
	run := appsMgr.find(owner, t)
	port := 0
	if run != nil {
		port = run.rec.Port
	}
	appsMgr.mu.Unlock()
	if port == 0 {
		return "", fmt.Errorf("no managed app called %q (use a port number or start it with the app tool first)", t)
	}
	return "http://127.0.0.1:" + strconv.Itoa(port), nil
}

// gregalListenPort és el port on escolta aquest Gregal (Hub.Run).
var gregalListenPort int

func (m *tunnelManager) start(owner, name, target, hostname, tunnel string) (string, error) {
	m.boot()
	bin, err := m.bin()
	if err != nil {
		return "", err
	}
	tgt, err := tunnelTarget(owner, target)
	if err != nil {
		return "", err
	}
	hostname = strings.ToLower(strings.TrimSpace(hostname))
	if hostname != "" && !regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?(\.[a-z0-9]([a-z0-9-]*[a-z0-9])?)+$`).MatchString(hostname) {
		return "", fmt.Errorf("invalid hostname %q", hostname)
	}
	name = normAppName(name)
	if name == "" {
		name = normAppName(strings.SplitN(hostname, ".", 2)[0])
	}
	if name == "" {
		name = normAppName(strings.TrimPrefix(strings.TrimPrefix(tgt, "http://"), "127.0.0.1:"))
	}
	tunnel = strings.TrimSpace(tunnel)
	if hostname != "" && tunnel == "" {
		tunnel = strings.TrimSpace(os.Getenv("GREGAL_CF_TUNNEL"))
	}
	if hostname != "" && tunnel == "" {
		m.mu.Lock()
		for _, r := range m.recs {
			if r.Tunnel != "" {
				tunnel = r.Tunnel
				break
			}
		}
		m.mu.Unlock()
	}
	if hostname != "" && tunnel == "" {
		return "", errors.New("a hostname needs a named Cloudflare tunnel: give tunnel (its name) or set GREGAL_CF_TUNNEL. The administrator creates one once with: sudo gregalctl tunnel-login && sudo gregalctl tunnel-create NAME. Without a domain, omit hostname for a quick trycloudflare.com link")
	}
	if hostname != "" {
		if out, err := m.route(bin, tunnel, hostname); err != nil {
			return "", fmt.Errorf("cloudflared could not add the DNS route for %s: %s", hostname, firstLines(out, 4))
		}
	}
	m.mu.Lock()
	r := m.find(owner, name)
	if r == nil {
		n := 0
		for _, x := range m.recs {
			if x.Owner == owner {
				n++
			}
		}
		if n >= tunnelMaxPerOwner {
			m.mu.Unlock()
			return "", fmt.Errorf("at most %d tunnels: stop one first", tunnelMaxPerOwner)
		}
		id, err := newShareID()
		if err != nil {
			m.mu.Unlock()
			return "", err
		}
		r = &tunnelRecord{ID: id, Name: name, Owner: owner, Created: time.Now().UTC()}
		m.recs[id] = r
	}
	wasQuick := r.Hostname == "" && m.runs[r.ID] != nil
	r.Target, r.Hostname, r.Tunnel, r.Want = tgt, hostname, tunnel, true
	if hostname != "" && wasQuick {
		// Passa de ràpid a domini: el procés ràpid ja no cal.
		if run := m.runs[r.ID]; run != nil && run.proc != nil {
			run.gen++
			run.proc.Kill()
		}
		delete(m.runs, r.ID)
	}
	if hostname == "" {
		err = m.launchQuickLocked(bin, r)
	} else {
		err = m.relaunchNamedLocked(bin)
	}
	m.saveLocked()
	m.mu.Unlock()
	if err != nil {
		return "", err
	}
	if hostname != "" {
		return "https://" + hostname, nil
	}
	return m.waitQuickURL(r.ID, 25*time.Second)
}

func (m *tunnelManager) waitQuickURL(id string, d time.Duration) (string, error) {
	deadline := time.Now().Add(d)
	for {
		m.mu.Lock()
		run := m.runs[id]
		var p *procs.Proc
		if run != nil {
			p = run.proc
		}
		st := m.statusLocked(run)
		m.mu.Unlock()
		if u := quickURL(p); u != "" {
			return u, nil
		}
		if st != "running" && time.Now().After(deadline.Add(-d+3*time.Second)) {
			return "", fmt.Errorf("cloudflared stopped before giving a link: %s", m.tail(p, 6))
		}
		if time.Now().After(deadline) {
			return "", fmt.Errorf("no trycloudflare link after %s yet (status %s); check with tunnel status. Last output: %s", d, st, m.tail(p, 6))
		}
		time.Sleep(400 * time.Millisecond)
	}
}

func (m *tunnelManager) tail(p *procs.Proc, n int) string {
	if p == nil {
		return "(no output)"
	}
	lines, _, _ := p.Output(0)
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	var b strings.Builder
	for _, l := range lines {
		b.WriteString(l.Text)
		b.WriteByte('\n')
	}
	if b.Len() == 0 {
		return "(no output yet)"
	}
	return strings.TrimRight(b.String(), "\n")
}

func firstLines(s string, n int) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	if len(lines) > n {
		lines = lines[:n]
	}
	if len(lines) == 0 || lines[0] == "" {
		return "(no output)"
	}
	return strings.Join(lines, " / ")
}

func (m *tunnelManager) stop(owner, ref string) (*tunnelRecord, error) {
	m.boot()
	m.mu.Lock()
	defer m.mu.Unlock()
	r := m.find(owner, ref)
	if r == nil {
		return nil, fmt.Errorf("no tunnel called %q", ref)
	}
	delete(m.recs, r.ID)
	if run := m.runs[r.ID]; run != nil && run.proc != nil {
		run.gen++
		run.proc.Kill()
	}
	delete(m.runs, r.ID)
	if r.Hostname != "" {
		if bin, err := m.bin(); err == nil {
			_ = m.relaunchNamedLocked(bin)
		}
	}
	m.saveLocked()
	return r, nil
}

// view és el que veu el model (i l'API) d'un túnel.
func (m *tunnelManager) viewLocked(r *tunnelRecord) map[string]any {
	runID := r.ID
	if r.Hostname != "" {
		runID = tunnelNamedID
	}
	run := m.runs[runID]
	link := ""
	if r.Hostname != "" {
		link = "https://" + r.Hostname
	} else if run != nil {
		link = quickURL(run.proc)
	}
	out := map[string]any{"id": r.ID, "name": r.Name, "target": r.Target, "url": link, "status": m.statusLocked(run)}
	if r.Hostname != "" {
		out["hostname"], out["tunnel"] = r.Hostname, r.Tunnel
	} else {
		out["kind"] = "quick (trycloudflare, changes if restarted)"
	}
	if run != nil && run.lastExit != "" {
		out["last_exit"] = run.lastExit
	}
	return out
}

func (m *tunnelManager) list(owner string) []map[string]any {
	m.boot()
	m.mu.Lock()
	defer m.mu.Unlock()
	var recs []*tunnelRecord
	for _, r := range m.recs {
		if r.Owner == owner {
			recs = append(recs, r)
		}
	}
	sort.Slice(recs, func(i, j int) bool { return recs[i].Created.Before(recs[j].Created) })
	out := make([]map[string]any, 0, len(recs))
	for _, r := range recs {
		out = append(out, m.viewLocked(r))
	}
	return out
}

// runTunnelTool és l'eina `tunnel` dels vents i de l'agent.
func (s *Server) runTunnelTool(ctx context.Context, workspace, argsJSON string) (string, error) {
	var a struct {
		Action   string `json:"action"`
		Name     string `json:"name"`
		Target   string `json:"target"`
		Hostname string `json:"hostname"`
		Tunnel   string `json:"tunnel"`
	}
	if err := json.Unmarshal([]byte(agent.ResolveArgs(workspace, argsJSON)), &a); err != nil {
		return "", errors.New("unreadable arguments")
	}
	owner := s.usuari()
	pretty := func(v any) string { b, _ := json.MarshalIndent(v, "", "  "); return string(b) }
	switch strings.TrimSpace(a.Action) {
	case "start", "":
		link, err := tunnelsMgr.start(owner, a.Name, a.Target, a.Hostname, a.Tunnel)
		if err != nil {
			return "", err
		}
		note := "It stays open after your turn and comes back if it crashes or the server restarts."
		if a.Hostname == "" {
			note += " This quick link changes if the tunnel restarts: for a stable address use a hostname on the user's domain."
		} else {
			note += " DNS can take a minute to propagate."
		}
		return fmt.Sprintf("Tunnel open: %s → %s\n%s", link, strings.TrimSpace(a.Target), note), nil
	case "list":
		l := tunnelsMgr.list(owner)
		if len(l) == 0 {
			return "No tunnels open.", nil
		}
		return pretty(l), nil
	case "status", "logs":
		tunnelsMgr.boot()
		tunnelsMgr.mu.Lock()
		r := tunnelsMgr.find(owner, a.Name)
		if r == nil {
			tunnelsMgr.mu.Unlock()
			return "", fmt.Errorf("no tunnel called %q", a.Name)
		}
		v := tunnelsMgr.viewLocked(r)
		runID := r.ID
		if r.Hostname != "" {
			runID = tunnelNamedID
		}
		var p *procs.Proc
		if run := tunnelsMgr.runs[runID]; run != nil {
			p = run.proc
		}
		tunnelsMgr.mu.Unlock()
		if a.Action == "logs" {
			return tunnelsMgr.tail(p, 30), nil
		}
		return pretty(v), nil
	case "stop":
		r, err := tunnelsMgr.stop(owner, a.Name)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("Tunnel %q closed.", r.Name), nil
	}
	return "", fmt.Errorf("unknown action %q (start, list, status, logs, stop)", a.Action)
}
