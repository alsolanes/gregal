package tools

// Ordinador personal (pla v2, §5 i paquet B2): el navegador de l'agent és
// de l'usuari/ordinador, no de cada feina.
//
//   - Un perfil per propietari. Abans cada sessió (pestanya del xat, vent,
//     colla) tenia el seu perfil a browser/sessions/<hash de la sessió> i
//     cada feina nova demanava tornar a iniciar sessió als llocs. Ara totes
//     les sessions d'un mateix usuari comparteixen browser/ordinadors/<clau>
//     (o el perfil base en mode local) i dos usuaris no comparteixen mai res.
//   - Un sol Chrome per propietari (Chrome no deixa obrir el mateix perfil
//     dues vegades) i una pestanya per sessió: dues feines no escriuen a
//     cegues a la mateixa pàgina.
//   - Amb GREGAL_BROWSER_CDP (la VM pilot: Chrome visible supervisat per
//     systemd, vegeu deploy/ordinador) també hi ha una pestanya per sessió i
//     una guarda de propietari: l'ordinador només serveix el seu owner.
//
// La lease de control persona/agent és de B3; aquí només es diu si l'agent
// hi té pestanyes obertes.

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"time"

	"gregal/internal/identity"
	"gregal/internal/sandbox"
)

// OrdinadorLocal és la clau del propietari quan el servidor no té usuaris
// (mode local, token únic): és qui fa servir la màquina.
const OrdinadorLocal = "local"

// browserDefaultSession és web.DefaultSession (tools no pot importar web):
// la sessió d'on es migra el perfil antic.
const browserDefaultSession = "default"

// ownerScopeRe reconeix les claus de sessió dels usuaris de debò d'un
// servidor multiusuari: identity.Key(nom) + "-" + id (web.Hub.keyFor). Un id
// de sessió local fa com a màxim 64 caràcters, així que mai pot ser 64 hex
// seguits d'un guió: la clau és inequívoca sense cap registre.
var ownerScopeRe = regexp.MustCompile(`^([0-9a-f]{64})-`)

// BrowserOwnerKey és el propietari del navegador d'una sessió:
// identity.Key de l'usuari o OrdinadorLocal.
func BrowserOwnerKey(scope string) string {
	if m := ownerScopeRe.FindStringSubmatch(scope); m != nil {
		return m[1]
	}
	return OrdinadorLocal
}

// OwnerKeyFor és la clau d'un nom d'usuari ("" = local).
func OwnerKeyFor(name string) string {
	if name == "" {
		return OrdinadorLocal
	}
	return identity.Key(name)
}

// ordinadorProfilePath és on viu el perfil del propietari, sense tocar res.
func ordinadorProfilePath(ownerKey string) string {
	base := browserProfileDir()
	if ownerKey == "" || ownerKey == OrdinadorLocal {
		return base
	}
	return filepath.Join(base, "ordinadors", ownerKey[:32])
}

// legacySessionProfile és el perfil que tenia una sessió abans de B2.
func legacySessionProfile(scope string) string {
	digest := sha256.Sum256([]byte(scope))
	return filepath.Join(browserProfileDir(), "sessions", fmt.Sprintf("%x", digest[:16]))
}

var (
	migrateMu   sync.Mutex
	migrateDone = map[string]bool{}
)

// OrdinadorProfileDir torna el perfil del propietari i, la primera vegada,
// hi porta el login que tenia la seua sessió per defecte.
//
// Migració: només si el perfil nou encara no és un perfil de Chrome (sense
// «Local State») i la sessió default del MATEIX propietari en tenia un. La
// font es deriva de la clau del propietari, així que mai s'hi pot portar el
// perfil d'un altre usuari, i mai es fusionen dos perfils. Els perfils de
// les altres sessions (browser/sessions/…) es deixen on són: no se sap quin
// login és el bo i barrejar-los podria creuar comptes. Es poden esborrar a
// mà (deploy/ordinador/README.ca.md).
func OrdinadorProfileDir(ownerKey string) (string, error) {
	dir := ordinadorProfilePath(ownerKey)
	migrateMu.Lock()
	defer migrateMu.Unlock()
	if !migrateDone[dir] {
		migrateDone[dir] = true
		if err := migrateLegacyProfile(ownerKey, dir); err != nil {
			// No és fatal: el perfil nou comença net i l'antic queda intacte.
			fmt.Fprintf(os.Stderr, "browser: no s'ha pogut migrar el perfil antic (%v); es comença un perfil nou\n", err)
		}
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return dir, nil
}

func isChromeProfile(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, "Local State"))
	return err == nil
}

func migrateLegacyProfile(ownerKey, dir string) error {
	if isChromeProfile(dir) {
		return nil
	}
	scope := browserDefaultSession
	if ownerKey != OrdinadorLocal {
		scope = ownerKey + "-" + browserDefaultSession
	}
	src := legacySessionProfile(scope)
	if !isChromeProfile(src) {
		return nil
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, e := range entries {
		to := filepath.Join(dir, e.Name())
		if _, err := os.Lstat(to); err == nil {
			// El perfil base local conté «sessions» i «ordinadors»: no es
			// trepitja res que ja hi sigui.
			continue
		}
		if err := os.Rename(filepath.Join(src, e.Name()), to); err != nil {
			return err
		}
	}
	_ = os.Remove(src) // només si ha quedat buit
	return nil
}

// ---- un Chrome per propietari ----

// ownerChrome és el Chrome que el gregal ha engegat per a un propietari.
type ownerChrome struct {
	key      string
	port     int
	headless bool
	// users: navegadors de sessió vius que hi tenen pestanya.
	users int
	// claimed: pestanyes que ja són d'una sessió.
	claimed map[string]bool
	kill    func()
	done    <-chan struct{}
}

func (oc *ownerChrome) exited() bool {
	select {
	case <-oc.done:
		return true
	default:
		return false
	}
}

var (
	ownerChromeMu sync.Mutex
	ownerChromes  = map[string]*ownerChrome{}
	// ordinadorAttachTabs: pestanyes que cada sessió ha obert al Chrome
	// supervisat (GREGAL_BROWSER_CDP), per propietari.
	ordinadorAttachUsers = map[string]int{}
)

// startOwnerChrome engega el procés. És variable perquè els tests hi posin
// un CDP fals (cap test engega un navegador de debò).
var startOwnerChrome = func(exe, profileDir string, headless bool) (*ownerChrome, error) {
	port, err := freePort()
	if err != nil {
		return nil, err
	}
	args := []string{
		fmt.Sprintf("--remote-debugging-port=%d", port),
		"--remote-debugging-address=127.0.0.1",
		"--user-data-dir=" + profileDir,
		"--no-first-run", "--no-default-browser-check", "--disable-sync",
		"--disable-background-networking", "--disable-features=Translate,MediaRouter",
		"--window-size=1280,900", "--remote-allow-origins=*",
	}
	if headless {
		args = append(args, "--headless=new")
	}
	args = append(args, "about:blank")
	cmd := exec.Command(exe, args...)
	cmd.Stdout, cmd.Stderr = io.Discard, io.Discard
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("browser: no s'ha pogut engegar %s: %v", filepath.Base(exe), err)
	}
	done := make(chan struct{})
	go func() { _ = cmd.Wait(); close(done) }()
	return &ownerChrome{
		port: port, headless: headless, done: done,
		kill: func() { _ = cmd.Process.Kill(); <-done },
	}, nil
}

// acquireOwnerChrome torna el Chrome del propietari, engegant-lo si cal.
// Si ja corria en un altre mode per a una altra sessió, es reutilitza (no
// es pot obrir el mateix perfil dues vegades) i la nota ho explica.
func acquireOwnerChrome(ownerKey string, headless bool) (*ownerChrome, string, error) {
	ownerChromeMu.Lock()
	defer ownerChromeMu.Unlock()
	if oc := ownerChromes[ownerKey]; oc != nil {
		if !oc.exited() {
			oc.users++
			note := ""
			if oc.headless != headless {
				note = fmt.Sprintf("this computer's browser is already open %s for another task; sharing it (one profile, one tab per task).", map[bool]string{true: "headless", false: "in a window"}[oc.headless])
			}
			return oc, note, nil
		}
		delete(ownerChromes, ownerKey)
	}
	exe := BrowserExecutable()
	if exe == "" {
		return nil, "", fmt.Errorf("browser: no trobo Chrome ni Edge (defineix GREGAL_BROWSER amb la ruta de l'executable)")
	}
	dir, err := OrdinadorProfileDir(ownerKey)
	if err != nil {
		return nil, "", err
	}
	oc, err := startOwnerChrome(exe, dir, headless)
	if err != nil {
		return nil, "", err
	}
	oc.key = ownerKey
	oc.claimed = map[string]bool{}
	oc.users = 1
	ownerChromes[ownerKey] = oc
	return oc, "", nil
}

// releaseOwnerChrome allibera la pestanya de b; l'últim tanca el Chrome
// (el perfil, amb els logins, queda a disc).
func releaseOwnerChrome(oc *ownerChrome, tabID string) {
	ownerChromeMu.Lock()
	delete(oc.claimed, tabID)
	oc.users--
	last := oc.users <= 0
	if last && ownerChromes[oc.key] == oc {
		delete(ownerChromes, oc.key)
	}
	ownerChromeMu.Unlock()
	if last {
		if oc.kill != nil {
			oc.kill()
		}
		return
	}
	closeCDPTab(oc.port, tabID)
}

// claimTab tria la pestanya d'una sessió: una de buida que no sigui de
// ningú (la primera de totes) o una de nova.
func claimTab(oc *ownerChrome, base string) string {
	ownerChromeMu.Lock()
	defer ownerChromeMu.Unlock()
	var tabs []tabInfo
	if resp, err := cdpHTTP.Get(base + "/json/list"); err == nil {
		_ = json.NewDecoder(resp.Body).Decode(&tabs)
		resp.Body.Close()
	}
	for _, t := range tabs {
		id := tabIDFromWS(t.WS)
		if t.Type == "page" && t.WS != "" && !oc.claimed[id] && (t.URL == "about:blank" || t.URL == "") {
			oc.claimed[id] = true
			return t.WS
		}
	}
	return ""
}

func tabIDFromWS(ws string) string { return ws[strings.LastIndex(ws, "/")+1:] }

func closeCDPTab(port int, tabID string) {
	if port == 0 || tabID == "" {
		return
	}
	if resp, err := cdpHTTP.Get(fmt.Sprintf("http://127.0.0.1:%d/json/close/%s", port, tabID)); err == nil {
		resp.Body.Close()
	}
}

// launchOwnerBrowser és launchChromeScoped: el navegador de la sessió scope
// al Chrome del seu propietari, en una pestanya pròpia.
func launchOwnerBrowser(headless bool, scope string) (*Browser, error) {
	key := BrowserOwnerKey(scope)
	oc, note, err := acquireOwnerChrome(key, headless)
	if err != nil {
		return nil, err
	}
	b := &Browser{refs: map[int]string{}, mode: BrowserModeWindow, port: oc.port, oc: oc, note: note}
	if oc.headless {
		b.mode = BrowserModeHeadless
	}
	if err := b.esperaCDP(); err != nil {
		releaseOwnerChrome(oc, "")
		return nil, err
	}
	ws := claimTab(oc, b.httpBase())
	if err := b.connectaWS(ws); err != nil {
		releaseOwnerChrome(oc, tabIDFromWS(ws))
		return nil, err
	}
	ownerChromeMu.Lock()
	oc.claimed[b.tabID] = true
	ownerChromeMu.Unlock()
	return b, nil
}

// ---- Chrome supervisat (GREGAL_BROWSER_CDP) ----

// OrdinadorOwnerName és el propietari declarat de l'ordinador
// (GREGAL_ORDINADOR_OWNER): a la VM pilot, l'únic usuari que el pot usar.
func OrdinadorOwnerName() string { return strings.TrimSpace(os.Getenv("GREGAL_ORDINADOR_OWNER")) }

// attachAllowed: al Chrome supervisat només hi entren el propietari
// declarat i el principal local de la màquina. Un sol Chrome té un sol
// perfil: si un compte d'un servidor multiusuari s'hi connecta sense
// propietari declarat, es denega, perquè el següent compte trobaria els
// logins del primer (deploy/ordinador/README.ca.md).
func attachAllowed(scope string) error {
	key := BrowserOwnerKey(scope)
	if key == OrdinadorLocal {
		return nil
	}
	owner := OrdinadorOwnerName()
	if owner == "" {
		return errors.New("browser: GREGAL_BROWSER_CDP is shared by every account; set GREGAL_ORDINADOR_OWNER to the account that owns this browser")
	}
	if key != identity.Key(owner) {
		return errors.New("browser: this computer belongs to another account")
	}
	return nil
}

func ordinadorAttachAddr() string {
	if browserAttach != "" {
		return browserAttach
	}
	return strings.TrimSpace(os.Getenv("GREGAL_BROWSER_CDP"))
}

// ---- salut de l'ordinador ----

// OrdinadorCheck és una comprovació de salut, tal com surt per l'API.
type OrdinadorCheck struct {
	Name   string `json:"name"`
	OK     bool   `json:"ok"`
	Detail string `json:"detail,omitempty"`
}

// OrdinadorProbe és l'estat real de l'ordinador d'un propietari.
type OrdinadorProbe struct {
	Health       string           `json:"health"`
	Capabilities []string         `json:"capabilities"`
	ControlOwner string           `json:"controlOwner"`
	Checks       []OrdinadorCheck `json:"checks"`
	// BrowserMode: attach (Chrome supervisat), launched (l'engega el
	// gregal) o on_demand (s'engegarà amb la primera acció).
	BrowserMode string `json:"browserMode"`
}

// procStart: amb el procés recent i el navegador encara sense respondre,
// la salut és «starting» i no «stopped».
var procStart = time.Now()

const ordinadorStartGrace = 30 * time.Second

// startGrace: GREGAL_ORDINADOR_START_GRACE (p. ex. «90s», «0») per a
// màquines lentes o tests; per defecte 30 s.
func startGrace() time.Duration {
	if v := strings.TrimSpace(os.Getenv("GREGAL_ORDINADOR_START_GRACE")); v != "" {
		if v == "0" {
			return 0
		}
		if d, err := time.ParseDuration(v); err == nil && d >= 0 {
			return d
		}
	}
	return ordinadorStartGrace
}

// ProbeOrdinador fa les comprovacions de debò (cap és declarativa):
// navegador/CDP, perfil escrivible, carpeta de fitxers, visor noVNC i
// terminal. filesDir és la carpeta de resultats ("" = no se'n declara).
func ProbeOrdinador(ctx context.Context, ownerKey, filesDir string) OrdinadorProbe {
	p := OrdinadorProbe{Capabilities: []string{}, ControlOwner: "none"}
	browserOK, browserDown := false, false

	if attach := ordinadorAttachAddr(); attach != "" {
		p.BrowserMode = "attach"
		c := OrdinadorCheck{Name: "browser"}
		if !loopbackAddr(attach) {
			c.Detail = "GREGAL_BROWSER_CDP must be a loopback address; CDP is never exposed"
		} else if err := probeCDP(ctx, attach); err != nil {
			c.Detail = err.Error()
			browserDown = connRefused(err)
		} else {
			c.OK, c.Detail = true, "CDP reachable on "+attach
		}
		browserOK = c.OK
		p.Checks = append(p.Checks, c)
		ownerChromeMu.Lock()
		if ordinadorAttachUsers[ownerKey] > 0 {
			p.ControlOwner = "agent"
		}
		ownerChromeMu.Unlock()
	} else {
		ownerChromeMu.Lock()
		oc := ownerChromes[ownerKey]
		if oc != nil && oc.exited() {
			oc = nil
		}
		port := 0
		if oc != nil {
			port = oc.port
			if oc.users > 0 {
				p.ControlOwner = "agent"
			}
		}
		ownerChromeMu.Unlock()
		c := OrdinadorCheck{Name: "browser"}
		switch {
		case port != 0:
			p.BrowserMode = "launched"
			if err := probeCDP(ctx, fmt.Sprintf("127.0.0.1:%d", port)); err != nil {
				c.Detail = err.Error()
				browserDown = connRefused(err)
			} else {
				c.OK, c.Detail = true, "CDP reachable"
			}
		case BrowserExecutable() != "":
			p.BrowserMode = "on_demand"
			c.OK, c.Detail = true, "browser installed; starts with the first action"
		default:
			p.BrowserMode = "on_demand"
			c.Detail = "no Chrome/Edge found (set GREGAL_BROWSER)"
			browserDown = true
		}
		browserOK = c.OK
		p.Checks = append(p.Checks, c)
	}

	profOK := false
	pc := OrdinadorCheck{Name: "profile"}
	if dir, err := ordinadorCheckedProfile(ownerKey, p.BrowserMode); err != nil {
		pc.Detail = err.Error()
	} else if err := writable(dir); err != nil {
		pc.Detail = "profile directory not writable: " + err.Error()
	} else {
		pc.OK, profOK = true, true
	}
	p.Checks = append(p.Checks, pc)
	if browserOK && profOK {
		p.Capabilities = append(p.Capabilities, "browser")
	}

	if filesDir != "" {
		fc := OrdinadorCheck{Name: "files"}
		if err := writable(filesDir); err != nil {
			fc.Detail = "files directory not writable: " + err.Error()
		} else {
			fc.OK = true
			p.Capabilities = append(p.Capabilities, "files")
		}
		p.Checks = append(p.Checks, fc)
	}

	tc := OrdinadorCheck{Name: "terminal"}
	if st := sandbox.Current(); st.Refuses {
		tc.Detail = "sandbox is strict and has no backend: shell commands are refused"
	} else {
		tc.OK, tc.Detail = true, "sandbox backend: "+st.Backend
		p.Capabilities = append(p.Capabilities, "terminal")
	}
	p.Checks = append(p.Checks, tc)

	if vnc := strings.TrimSpace(os.Getenv("GREGAL_ORDINADOR_VNC")); vnc != "" {
		vc := OrdinadorCheck{Name: "viewer"}
		if !loopbackAddr(vnc) {
			vc.Detail = "GREGAL_ORDINADOR_VNC must be a loopback address; the viewer goes through the authenticated gateway"
		} else if conn, err := (&net.Dialer{Timeout: 2 * time.Second}).DialContext(ctx, "tcp", vnc); err != nil {
			vc.Detail = "noVNC not reachable: " + err.Error()
		} else {
			conn.Close()
			vc.OK, vc.Detail = true, "noVNC listening on "+vnc
			p.Capabilities = append(p.Capabilities, "viewer")
		}
		p.Checks = append(p.Checks, vc)
	}

	switch {
	case browserOK && profOK:
		p.Health = "running"
	case !browserOK && time.Since(procStart) < startGrace() && p.BrowserMode == "attach":
		p.Health = "starting"
	case browserDown && profOK:
		p.Health = "stopped"
	default:
		p.Health = "unreachable"
	}
	return p
}

// ordinadorCheckedProfile és el perfil que fa servir de debò el navegador:
// amb el Chrome supervisat, el seu (GREGAL_ORDINADOR_PROFILE, al volum
// persistent de la VM); si no, el del propietari.
func ordinadorCheckedProfile(ownerKey, mode string) (string, error) {
	if v := strings.TrimSpace(os.Getenv("GREGAL_ORDINADOR_PROFILE")); v != "" && mode == "attach" {
		return v, nil
	}
	return OrdinadorProfileDir(ownerKey)
}

func loopbackAddr(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return false
	}
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func probeCDP(ctx context.Context, addr string) error {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+addr+"/json/version", nil)
	if err != nil {
		return err
	}
	resp, err := cdpHTTP.Do(req)
	if err != nil {
		return fmt.Errorf("CDP not reachable: %w", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("CDP answered %d", resp.StatusCode)
	}
	return nil
}

// connRefused: no hi ha res escoltant (el navegador està aturat), a
// diferència d'una xarxa que no respon (inabastable).
func connRefused(err error) bool {
	return errors.Is(err, syscall.ECONNREFUSED) || strings.Contains(strings.ToLower(err.Error()), "refused")
}

func writable(dir string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".gregal-probe-*")
	if err != nil {
		return err
	}
	name := f.Name()
	f.Close()
	return os.Remove(name)
}
