package ordinador

// Gestor de l'amfitrió: quin ordinador és de qui, encendre'l només quan una
// feina el necessita, desar-lo a disc quan no fa res i dir-ne la salut
// honesta sense despertar-lo mai (pla v2 §5, «Política d'encesa»).
//
// Estat normal: desat a disc (suspensió/hibernació) o aturat. Zero CPU i RAM;
// el perfil del navegador i els fitxers queden al disc.
//
// Registre (GREGAL_ORDINADORS, JSON):
//
//	{"computers": [{
//	  "id": "pc-carla", "owner": "carla",
//	  "agent_url": "http://10.66.0.2:8098",
//	  "token_file": "/etc/gregal/ordinadors/pc-carla.token",
//	  "control": ["/usr/local/lib/gregal/ordinador-hipervisor.sh", "gregal-carla"],
//	  "idle_minutes": 10, "idle_action": "suspend", "wake_timeout_seconds": 90}]}
//
// "control" és l'ordre del hipervisor; el gestor hi afegeix «status»,
// «wake», «suspend» o «stop» com a últim argument (deploy/ordinador). Sense
// "control" l'ordinador no s'encén ni es desa des d'aquí i la salut només
// pot ser running o unreachable.

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"gregal/internal/tools"
)

// Computer és una entrada del registre.
type Computer struct {
	ID        string   `json:"id"`
	Owner     string   `json:"owner"`
	AgentURL  string   `json:"agent_url"`
	TokenFile string   `json:"token_file"`
	Control   []string `json:"control,omitempty"`
	// IdleMinutes: minuts sense feina ni visor abans de desar-lo (0 = 10,
	// -1 = mai).
	IdleMinutes int `json:"idle_minutes,omitempty"`
	// IdleAction: «suspend» (per defecte: desar a disc) o «stop» si el
	// hipervisor no sap suspendre.
	IdleAction         string `json:"idle_action,omitempty"`
	WakeTimeoutSeconds int    `json:"wake_timeout_seconds,omitempty"`

	token string
}

// Report és l'estat que l'amfitrió dona d'un ordinador (camps del contracte
// B0 més les comprovacions i l'ús).
type Report struct {
	ID           string   `json:"id"`
	Owner        string   `json:"owner"`
	Health       string   `json:"health"`
	Capabilities []string `json:"capabilities"`
	ControlOwner string   `json:"controlOwner"`
	// LeaseExpiry: quan caduca la lease de la persona (buit = cap).
	LeaseExpiry string `json:"leaseExpiry,omitempty"`
	LastSeenAt  any    `json:"lastSeenAt"`
	BrowserMode string `json:"browserMode,omitempty"`
	// Power és el que diu el hipervisor: running, saved (desat a disc),
	// stopped, starting o unknown (sense "control" o si no respon).
	Power  string  `json:"power"`
	Usage  Usage   `json:"usage"`
	Checks []Check `json:"checks"`
}

// Usage és la mesura d'ús: quant de temps ha estat encès i per què.
type Usage struct {
	// OnSince: des de quan és encès ara (null si és desat o aturat).
	OnSince any `json:"onSince"`
	// OnSeconds: segons encès en total (registre de l'amfitrió).
	OnSeconds int64 `json:"onSeconds"`
	// Wakes: vegades que l'amfitrió l'ha encès.
	Wakes          int    `json:"wakes"`
	LastWakeReason string `json:"lastWakeReason,omitempty"`
}

// Check és una comprovació, de l'amfitrió o de l'agent.
type Check struct {
	Name   string `json:"name"`
	OK     bool   `json:"ok"`
	Detail string `json:"detail,omitempty"`
}

// ErrNoComputer: el compte no té ordinador assignat.
var ErrNoComputer = errors.New("ordinador: aquest compte no té ordinador")

// ErrComputerUnavailable: l'ordinador no s'ha pogut encendre o no respon.
// La feina ha de passar a waiting_computer (B4), no fallar.
var ErrComputerUnavailable = errors.New("ordinador: no disponible")

// Topalls: la salut no pot penjar mai la petició d'Android.
const (
	powerTimeout  = 3 * time.Second
	healthTimeout = 3 * time.Second
	controlTO     = 60 * time.Second
	// defaultWake: l'objectiu del pilot és ≤ 30 s des de suspensió; el
	// topall deixa marge per a una arrencada en fred (aturat, no desat).
	defaultWake = 90 * time.Second
	defaultIdle = 10
)

// MeterEvent és una línia del registre d'ús (JSONL).
type MeterEvent struct {
	Time      time.Time `json:"time"`
	ID        string    `json:"id"`
	Owner     string    `json:"owner"`
	Event     string    `json:"event"` // wake, suspend, stop
	Reason    string    `json:"reason,omitempty"`
	OnSeconds int64     `json:"on_seconds,omitempty"`
}

// Manager és el gestor d'ordinadors de l'amfitrió.
type Manager struct {
	byOwner map[string]*Computer
	byID    map[string]*Computer
	client  *http.Client
	// runControl executa l'ordre del hipervisor; variable per als tests.
	runControl func(ctx context.Context, argv []string) (string, error)
	now        func() time.Time
	meterPath  string

	mu       sync.Mutex
	lastUse  map[string]time.Time
	lastSeen map[string]time.Time
	waking   map[string]time.Time
	active   map[string]int
	usage    map[string]*usageState

	// leases: qui té el control (B3). Nil = ningú no el pot prendre.
	leases *Leases
}

type usageState struct {
	onSince    time.Time
	onSeconds  int64
	wakes      int
	lastReason string
}

// LoadRegistry llegeix el registre i els testimonis. El registre d'ús va al
// costat (<registre>.uso.jsonl) si no se'n dona cap altre.
func LoadRegistry(path string) (*Manager, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var reg struct {
		Computers []*Computer `json:"computers"`
		MeterFile string      `json:"meter_file,omitempty"`
	}
	if err := json.Unmarshal(raw, &reg); err != nil {
		return nil, fmt.Errorf("ordinador: %s: %v", path, err)
	}
	m, err := NewManager(reg.Computers)
	if err != nil {
		return nil, err
	}
	meter := reg.MeterFile
	if meter == "" {
		meter = strings.TrimSuffix(path, ".json") + ".uso.jsonl"
	}
	m.SetMeterFile(meter)
	return m, nil
}

// NewManager valida les entrades i crea el gestor.
func NewManager(list []*Computer) (*Manager, error) {
	m := &Manager{
		byOwner: map[string]*Computer{}, byID: map[string]*Computer{},
		client:     &http.Client{},
		runControl: runCommand, now: time.Now,
		lastUse: map[string]time.Time{}, lastSeen: map[string]time.Time{}, waking: map[string]time.Time{},
		active: map[string]int{}, usage: map[string]*usageState{},
	}
	for _, c := range list {
		if c.ID == "" || c.Owner == "" {
			return nil, errors.New("ordinador: cada entrada necessita id i owner")
		}
		if m.byID[c.ID] != nil {
			return nil, fmt.Errorf("ordinador: id repetit %q", c.ID)
		}
		if m.byOwner[c.Owner] != nil {
			return nil, fmt.Errorf("ordinador: %q ja té un ordinador", c.Owner)
		}
		switch c.IdleAction {
		case "":
			c.IdleAction = "suspend"
		case "suspend", "stop":
		default:
			return nil, fmt.Errorf("ordinador %s: idle_action ha de ser suspend o stop", c.ID)
		}
		u, err := url.Parse(c.AgentURL)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Port() == "" {
			return nil, fmt.Errorf("ordinador %s: agent_url ha de ser http(s)://IP:port", c.ID)
		}
		if err := CheckPrivateAddr(u.Host); err != nil {
			return nil, fmt.Errorf("ordinador %s: %v", c.ID, err)
		}
		if c.token == "" {
			tok, err := os.ReadFile(c.TokenFile)
			if err != nil {
				return nil, fmt.Errorf("ordinador %s: token_file: %v", c.ID, err)
			}
			c.token = strings.TrimSpace(string(tok))
		}
		if len(c.token) < MinTokenLen {
			return nil, fmt.Errorf("ordinador %s: el testimoni és massa curt", c.ID)
		}
		m.byID[c.ID], m.byOwner[c.Owner] = c, c
		m.usage[c.ID] = &usageState{}
	}
	return m, nil
}

// SetMeterFile fixa el registre d'ús i en recupera els totals.
func (m *Manager) SetMeterFile(path string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.meterPath = path
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var ev MeterEvent
		if json.Unmarshal(sc.Bytes(), &ev) != nil {
			continue
		}
		u := m.usage[ev.ID]
		if u == nil {
			continue
		}
		u.onSeconds += ev.OnSeconds
		if ev.Event == "wake" {
			u.wakes++
			u.lastReason = ev.Reason
		}
	}
}

// meterLocked apunta un esdeveniment d'ús (amb m.mu agafat).
func (m *Manager) meterLocked(c *Computer, event, reason string) {
	u := m.usage[c.ID]
	ev := MeterEvent{Time: m.now().UTC(), ID: c.ID, Owner: c.Owner, Event: event, Reason: reason}
	switch event {
	case "wake":
		u.wakes++
		u.lastReason = reason
		if u.onSince.IsZero() {
			u.onSince = m.now()
		}
	default:
		if !u.onSince.IsZero() {
			ev.OnSeconds = int64(m.now().Sub(u.onSince) / time.Second)
			u.onSeconds += ev.OnSeconds
			u.onSince = time.Time{}
		}
	}
	if m.meterPath == "" {
		return
	}
	f, err := os.OpenFile(m.meterPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o640)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ordinador: registre d'ús: %v\n", err)
		return
	}
	defer f.Close()
	b, _ := json.Marshal(ev)
	f.Write(append(b, '\n'))
}

// For torna l'ordinador del compte.
func (m *Manager) For(owner string) (*Computer, error) {
	if c := m.byOwner[owner]; c != nil {
		return c, nil
	}
	return nil, ErrNoComputer
}

// ByID torna l'ordinador amb aquest id (el que en faci ús ha de comprovar
// el propietari).
func (m *Manager) ByID(id string) *Computer { return m.byID[id] }

func runCommand(ctx context.Context, argv []string) (string, error) {
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = io.Discard
	err := cmd.Run()
	return out.String(), err
}

func (m *Manager) control(ctx context.Context, c *Computer, verb string, to time.Duration) (string, error) {
	if len(c.Control) == 0 {
		return "", errors.New("no hypervisor command configured")
	}
	ctx, cancel := context.WithTimeout(ctx, to)
	defer cancel()
	argv := append(append([]string{}, c.Control...), verb)
	return m.runControl(ctx, argv)
}

// power pregunta al hipervisor (mai desperta la VM). La sortida esperada és
// una paraula: running, saved, stopped o starting.
func (m *Manager) power(ctx context.Context, c *Computer) (string, Check) {
	if len(c.Control) == 0 {
		return "unknown", Check{Name: "hypervisor", OK: true, Detail: "no hypervisor command; power state unknown"}
	}
	out, err := m.control(ctx, c, "status", powerTimeout)
	if err != nil {
		return "unknown", Check{Name: "hypervisor", Detail: "status failed: " + err.Error()}
	}
	word := strings.ToLower(strings.TrimSpace(strings.SplitN(strings.TrimSpace(out)+"\n", "\n", 2)[0]))
	switch word {
	case "running", "stopped", "starting", "saved":
	case "suspended", "hibernated", "pmsuspended":
		word = "saved"
	case "paused", "shut off", "off":
		word = "stopped"
	default:
		return "unknown", Check{Name: "hypervisor", Detail: "unexpected status " + fmt.Sprintf("%q", word)}
	}
	return word, Check{Name: "hypervisor", OK: true, Detail: word}
}

func asleep(power string) bool { return power == "stopped" || power == "saved" }

func (m *Manager) agentReq(ctx context.Context, c *Computer, method, path string, body any) (*http.Response, error) {
	var rd io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(c.AgentURL, "/")+path, rd)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return m.client.Do(req)
}

func (m *Manager) agentHealth(ctx context.Context, c *Computer) (*Health, error) {
	ctx, cancel := context.WithTimeout(ctx, healthTimeout)
	defer cancel()
	resp, err := m.agentReq(ctx, c, http.MethodGet, "/v1/health", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("agent answered %d", resp.StatusCode)
	}
	var h Health
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&h); err != nil {
		return nil, err
	}
	if h.ID != c.ID || h.Owner != c.Owner {
		return nil, fmt.Errorf("the agent at %s is %s/%s, not %s/%s", c.AgentURL, h.ID, h.Owner, c.ID, c.Owner)
	}
	return &h, nil
}

func (c *Computer) wakeTimeout() time.Duration {
	if c.WakeTimeoutSeconds > 0 {
		return time.Duration(c.WakeTimeoutSeconds) * time.Second
	}
	return defaultWake
}

// Status és la salut honesta de l'ordinador. No el desperta mai: amb la VM
// desada o aturada diu stopped sense parlar amb l'agent; mai triga més
// d'uns segons.
func (m *Manager) Status(ctx context.Context, c *Computer) Report {
	r := Report{ID: c.ID, Owner: c.Owner, Capabilities: []string{}, ControlOwner: "none"}
	power, pc := m.power(ctx, c)
	r.Power = power
	r.Checks = append(r.Checks, pc)

	m.mu.Lock()
	wakingSince, waking := m.waking[c.ID]
	if waking && m.now().Sub(wakingSince) > c.wakeTimeout() {
		delete(m.waking, c.ID)
		waking = false
	}
	if power == "running" && m.usage[c.ID].onSince.IsZero() {
		// L'amfitrió s'ha reiniciat amb la VM encesa: compta des d'ara.
		m.usage[c.ID].onSince = m.now()
	}
	if asleep(power) && !waking && !m.usage[c.ID].onSince.IsZero() {
		// S'ha desat o aturat per fora (hipervisor, la persona): es tanca
		// el tram encès.
		m.meterLocked(c, power, "external")
	}
	m.mu.Unlock()

	if asleep(power) && !waking {
		r.Health = "stopped"
		r.LastSeenAt = m.seen(c, false)
		r.Usage = m.usageOf(c)
		m.applyLease(c, &r)
		return r
	}
	h, err := m.agentHealth(ctx, c)
	if err != nil {
		r.Checks = append(r.Checks, Check{Name: "agent", Detail: "agent not reachable: " + err.Error()})
		// Encesa demanada fa poc (o el hipervisor diu que arrenca): starting.
		// Encesa i sense agent: unreachable, mai running.
		if waking || power == "starting" {
			r.Health = "starting"
		} else {
			r.Health = "unreachable"
		}
		r.LastSeenAt = m.seen(c, false)
		r.Usage = m.usageOf(c)
		m.applyLease(c, &r)
		return r
	}
	r.Checks = append(r.Checks, Check{Name: "agent", OK: true})
	for _, ch := range h.Checks {
		r.Checks = append(r.Checks, Check{Name: ch.Name, OK: ch.OK, Detail: ch.Detail})
	}
	r.Health, r.Capabilities, r.ControlOwner, r.BrowserMode = h.Health, h.Capabilities, h.ControlOwner, h.BrowserMode
	if r.Capabilities == nil {
		r.Capabilities = []string{}
	}
	if r.Health == "running" {
		m.mu.Lock()
		delete(m.waking, c.ID)
		m.mu.Unlock()
	}
	r.LastSeenAt = m.seen(c, r.Health == "running")
	r.Usage = m.usageOf(c)
	m.applyLease(c, &r)
	return r
}

func (m *Manager) usageOf(c *Computer) Usage {
	m.mu.Lock()
	defer m.mu.Unlock()
	u := m.usage[c.ID]
	out := Usage{OnSeconds: u.onSeconds, Wakes: u.wakes, LastWakeReason: u.lastReason}
	if !u.onSince.IsZero() {
		out.OnSince = u.onSince.UTC().Format(time.RFC3339)
		out.OnSeconds += int64(m.now().Sub(u.onSince) / time.Second)
	}
	return out
}

func (m *Manager) seen(c *Computer, now bool) any {
	m.mu.Lock()
	defer m.mu.Unlock()
	if now {
		m.lastSeen[c.ID] = m.now().UTC()
	}
	if t, ok := m.lastSeen[c.ID]; ok {
		return t.Format(time.RFC3339)
	}
	return nil
}

// Wake encén (o reprèn del disc) l'ordinador; no espera que arrenqui.
// reason queda al registre d'ús: «task:<run>», «viewer», «user»…
func (m *Manager) Wake(ctx context.Context, c *Computer, reason string) error {
	if strings.TrimSpace(reason) == "" {
		return errors.New("ordinador: cal el motiu de l'encesa")
	}
	if _, err := m.control(ctx, c, "wake", controlTO); err != nil {
		return fmt.Errorf("ordinador %s: wake: %w", c.ID, err)
	}
	m.mu.Lock()
	m.waking[c.ID] = m.now()
	m.lastUse[c.ID] = m.now()
	m.meterLocked(c, "wake", reason)
	m.mu.Unlock()
	return nil
}

// Park desa l'ordinador a disc (idle_action suspend) o l'atura (stop).
func (m *Manager) Park(ctx context.Context, c *Computer, reason string) error {
	return m.sleep(ctx, c, c.IdleAction, reason)
}

// Stop atura l'ordinador del tot (el disc es conserva).
func (m *Manager) Stop(ctx context.Context, c *Computer, reason string) error {
	return m.sleep(ctx, c, "stop", reason)
}

func (m *Manager) sleep(ctx context.Context, c *Computer, verb, reason string) error {
	if _, err := m.control(ctx, c, verb, controlTO); err != nil {
		return fmt.Errorf("ordinador %s: %s: %w", c.ID, verb, err)
	}
	m.mu.Lock()
	delete(m.waking, c.ID)
	m.meterLocked(c, verb, reason)
	m.mu.Unlock()
	return nil
}

// EnsureRunning encén l'ordinador si cal i espera que l'agent digui running,
// fins al topall d'arrencada. Si no hi arriba torna ErrComputerUnavailable:
// la feina passa a waiting_computer, no falla. reason és per què cal
// (només feina que necessita navegador, terminal o fitxers, o el visor).
func (m *Manager) EnsureRunning(ctx context.Context, c *Computer, reason string, poll time.Duration) error {
	m.touch(c)
	r := m.Status(ctx, c)
	if r.Health == "running" {
		return nil
	}
	if asleep(r.Power) || r.Power == "unknown" && len(c.Control) > 0 {
		if err := m.Wake(ctx, c, reason); err != nil {
			return fmt.Errorf("%w: %v", ErrComputerUnavailable, err)
		}
	}
	if poll <= 0 {
		poll = 2 * time.Second
	}
	deadline := m.now().Add(c.wakeTimeout())
	for m.now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(poll):
		}
		if r = m.Status(ctx, c); r.Health == "running" {
			return nil
		}
	}
	return fmt.Errorf("%w: %s is %s", ErrComputerUnavailable, c.ID, r.Health)
}

func (m *Manager) touch(c *Computer) {
	m.mu.Lock()
	m.lastUse[c.ID] = m.now()
	m.mu.Unlock()
}

// Exec envia una eina a l'ordinador del compte. No l'encén: qui crida
// decideix (EnsureRunning) i, si no hi és, posa la feina en espera.
// Mentre una eina corre, l'ordinador no es desa.
func (m *Manager) Exec(ctx context.Context, owner string, req ExecRequest) (ExecResponse, error) {
	c, err := m.For(owner)
	if err != nil {
		return ExecResponse{}, err
	}
	req.Owner = owner
	// La persona té el control: l'agent no toca el navegador ni en fa
	// captures fins que el retorni (B3).
	if req.Tool == "browser" && m.leases.HeldByUser(c.LeaseKey()) {
		return ExecResponse{}, ErrUserHasControl
	}
	m.mu.Lock()
	m.active[c.ID]++
	m.lastUse[c.ID] = m.now()
	m.mu.Unlock()
	defer func() {
		m.mu.Lock()
		m.active[c.ID]--
		m.lastUse[c.ID] = m.now()
		m.mu.Unlock()
	}()
	resp, err := m.agentReq(ctx, c, http.MethodPost, "/v1/exec", req)
	if err != nil {
		return ExecResponse{}, fmt.Errorf("%w: %v", ErrComputerUnavailable, err)
	}
	defer resp.Body.Close()
	var out ExecResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 64<<20)).Decode(&out); err != nil {
		return ExecResponse{}, fmt.Errorf("ordinador %s: resposta il·legible (%d)", c.ID, resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		return out, fmt.Errorf("ordinador %s: %s", c.ID, out.Error)
	}
	return out, nil
}

// Hold marca feina en curs a l'ordinador (p. ex. un encàrrec que fa diverses
// crides seguides): mentre no es cridi la funció que torna, no es desa.
func (m *Manager) Hold(c *Computer) (release func()) {
	m.mu.Lock()
	m.active[c.ID]++
	m.mu.Unlock()
	var once sync.Once
	return func() {
		once.Do(func() {
			m.mu.Lock()
			m.active[c.ID]--
			m.lastUse[c.ID] = m.now()
			m.mu.Unlock()
		})
	}
}

// ParkIdle desa (o atura) els ordinadors encesos que fa més d'idle_minutes
// que no fan cap feina, sense eines en curs ni ningú amb el control o el
// visor obert (controlOwner none). Torna els desats.
func (m *Manager) ParkIdle(ctx context.Context) []string {
	var parked []string
	for _, c := range m.byID {
		if len(c.Control) == 0 || c.IdleMinutes < 0 {
			continue
		}
		idle := time.Duration(c.IdleMinutes) * time.Minute
		if c.IdleMinutes == 0 {
			idle = defaultIdle * time.Minute
		}
		m.mu.Lock()
		last, ok := m.lastUse[c.ID]
		if !ok {
			// Sense ús des que ha arrencat l'amfitrió: compta des d'ara.
			m.lastUse[c.ID] = m.now()
			last = m.now()
		}
		busy := m.active[c.ID] > 0 || m.leases.HeldByUser(c.LeaseKey())
		m.mu.Unlock()
		if busy || m.now().Sub(last) < idle {
			continue
		}
		r := m.Status(ctx, c)
		if asleep(r.Power) || r.ControlOwner != "none" || r.Health == "starting" {
			continue
		}
		if err := m.Park(ctx, c, "idle"); err == nil {
			parked = append(parked, c.ID)
		}
	}
	return parked
}

// RunIdleLoop crida ParkIdle cada minut fins que ctx s'acabi.
func (m *Manager) RunIdleLoop(ctx context.Context) {
	t := time.NewTicker(time.Minute)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			m.ParkIdle(ctx)
		}
	}
}

// SetControlRunner substitueix com s'executa l'ordre del hipervisor (tests
// i integracions que no passen per una ordre externa).
func (m *Manager) SetControlRunner(fn func(ctx context.Context, argv []string) (string, error)) {
	m.runControl = fn
}

// LeaseKey és la clau del propietari per a la lease de control: la mateixa
// que fa servir el navegador (tools.OwnerKeyFor).
func (c *Computer) LeaseKey() string { return tools.OwnerKeyFor(c.Owner) }

// SetLeases connecta el registre de leases de control (B3).
func (m *Manager) SetLeases(l *Leases) { m.leases = l }

// applyLease: si la persona té el control, la salut ho diu (controlOwner
// user i la caducitat de la lease), digui el que digui l'agent.
func (m *Manager) applyLease(c *Computer, r *Report) {
	if x := m.leases.Get(c.LeaseKey()); x != nil {
		r.ControlOwner = "user"
		r.LeaseExpiry = x.Expires.UTC().Format(time.RFC3339)
	}
}

// Touch apunta ús de l'ordinador (el visor és a la vista): compta com a
// feina per al desat per inactivitat. Quan el visor es tanca o l'app passa a
// segon pla, els batecs s'aturen i el compte enrere torna a córrer.
func (m *Manager) Touch(c *Computer) { m.touch(c) }

// ViewerProxy és el pont cap al visor noVNC de la VM, a través de l'agent
// (/v1/viewer/, amb el testimoni de l'ordinador). prefix és el camí públic
// que es treu abans de passar-ho a l'agent. Qui el munta ha d'haver
// comprovat la sessió del visor i el propietari.
func (m *Manager) ViewerProxy(c *Computer, prefix string) http.Handler {
	target, _ := url.Parse(strings.TrimRight(c.AgentURL, "/"))
	return newViewerProxy(target, "/v1/viewer/", prefix, "Bearer "+c.token)
}
