package connexions

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Estats d'una connexió (els de internal/web/assistant_contract.go).
const (
	Available    = "available"
	Connecting   = "connecting"
	Connected    = "connected"
	NeedsSession = "needs_session"
	Error        = "error"
	Disconnected = "disconnected"
)

// Serveis (el «tipus» de connexió).
const (
	ServiceMCP      = "mcp"          // connector estructurat: servidor MCP remot amb OAuth
	ServiceICS      = "calendar-ics" // calendari per adreça secreta iCal
	ServiceIMAP     = "mail-imap"    // correu IMAP amb contrasenya d'aplicació
	ServiceBrowser  = "browser"      // sessió iniciada al navegador de l'ordinador
	MethodAPI       = "api"
	MethodBrowser   = "browser"
	CapRead         = "read"
	CapPrepare      = "prepare"
	CapSend         = "send"
	maxPerOwner     = 50
	registryFile    = "connexions.json"
	registryVersion = 1
)

// Connexio és una connexió d'una persona. Res d'aquí és secret: el secret és
// al Vault sota AuthRef.
type Connexio struct {
	ID      string `json:"id"`
	Owner   string `json:"owner"`
	Method  string `json:"method"`
	Service string `json:"service"`
	// Provider és el connector concret (nom del servidor MCP) o el lloc
	// (browser); buit per a ICS/IMAP.
	Provider     string `json:"provider,omitempty"`
	AccountLabel string `json:"accountLabel"`
	// Status és el darrer resultat d'una comprovació de debò (mai optimista).
	Status string `json:"status"`
	// Capabilities: què pot fer l'eina. Allowed: què autoritza la persona.
	Capabilities  []string `json:"capabilities"`
	Allowed       []string `json:"allowed"`
	LastCheckedAt string   `json:"lastCheckedAt,omitempty"`
	AuthRef       string   `json:"authRef"`
	ComputerID    string   `json:"computerId,omitempty"`
	Error         string   `json:"error,omitempty"`
	CreatedAt     string   `json:"createdAt,omitempty"`

	// Configuració no secreta de cada servei (no surt pel payload públic
	// llevat del que diu Public).
	Host   string `json:"host,omitempty"`   // IMAP
	Port   int    `json:"port,omitempty"`   // IMAP
	Folder string `json:"folder,omitempty"` // IMAP
	Hint   string `json:"hint,omitempty"`   // ICS: inici de l'adreça, per reconèixer-la
}

// Effective són els permisos que valen: capacitat de l'eina ∩ autorització.
func (c Connexio) Effective() []string {
	out := []string{}
	for _, cap := range c.Capabilities {
		if contains(c.Allowed, cap) {
			out = append(out, cap)
		}
	}
	return out
}

// Can diu si un permís és efectiu.
func (c Connexio) Can(perm string) bool { return contains(c.Effective(), perm) }

// Public és el payload per a l'API, Android i el model: cap secret, cap
// configuració interna més enllà del que la persona necessita veure.
func (c Connexio) Public() map[string]any {
	caps := c.Capabilities
	if caps == nil {
		caps = []string{}
	}
	allowed := c.Allowed
	if allowed == nil {
		allowed = []string{}
	}
	m := map[string]any{
		"kind": "connexio", "id": c.ID, "owner": c.Owner, "method": c.Method, "service": c.Service,
		"accountLabel": c.AccountLabel, "status": c.Status, "capabilities": caps, "allowed": allowed,
		"effective": c.Effective(), "authRef": c.AuthRef,
	}
	if c.Provider != "" {
		m["provider"] = c.Provider
	}
	if c.LastCheckedAt != "" {
		m["lastCheckedAt"] = c.LastCheckedAt
	}
	if c.ComputerID != "" {
		m["computerId"] = c.ComputerID
	}
	if c.Error != "" {
		m["error"] = c.Error
	}
	if c.Service == ServiceIMAP && c.Host != "" {
		m["host"] = c.Host
	}
	if c.Service == ServiceICS && c.Hint != "" {
		m["hint"] = c.Hint
	}
	return m
}

func contains(list []string, x string) bool {
	for _, v := range list {
		if v == x {
			return true
		}
	}
	return false
}

// MarkChecked desa el resultat d'una comprovació.
func (c *Connexio) MarkChecked(status, errText string, now time.Time) {
	c.Status = status
	c.Error = errText
	c.LastCheckedAt = now.UTC().Format(time.RFC3339)
}

// Store és el registre de connexions d'un owner: <dir>/connexions.json.
type Store struct {
	path  string
	owner string
}

var storeLocks sync.Map // camí → *sync.Mutex

// Open obre el registre d'owner a dir (la carpeta d'estat de la persona).
func Open(dir, owner string) *Store {
	return &Store{path: filepath.Join(dir, registryFile), owner: owner}
}

type registry struct {
	V     int        `json:"v"`
	Items []Connexio `json:"items"`
}

func (s *Store) lock() func() {
	mu, _ := storeLocks.LoadOrStore(s.path, &sync.Mutex{})
	mu.(*sync.Mutex).Lock()
	return mu.(*sync.Mutex).Unlock
}

func (s *Store) read() ([]Connexio, error) {
	raw, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var r registry
	if err := json.Unmarshal(raw, &r); err != nil {
		return nil, err
	}
	// Cada lectura torna a comprovar owner: un fitxer copiat d'una altra
	// carpeta no dona connexions d'un altre.
	out := r.Items[:0]
	for _, c := range r.Items {
		if c.Owner == s.owner {
			out = append(out, c)
		}
	}
	return out, nil
}

func (s *Store) write(items []Connexio) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return err
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].CreatedAt < items[j].CreatedAt })
	raw, err := json.MarshalIndent(registry{V: registryVersion, Items: items}, "", "  ")
	if err != nil {
		return err
	}
	return writeAtomic(s.path, raw)
}

// List torna les connexions de l'owner.
func (s *Store) List() []Connexio {
	unlock := s.lock()
	defer unlock()
	items, _ := s.read()
	return items
}

// Get torna una connexió de l'owner.
func (s *Store) Get(id string) (Connexio, bool) {
	for _, c := range s.List() {
		if c.ID == id {
			return c, true
		}
	}
	return Connexio{}, false
}

// Update modifica el registre sota el bloqueig (llegir, canviar, desar).
// Les connexions noves reben owner, id, authRef i data si no en porten.
func (s *Store) Update(fn func(items []Connexio) ([]Connexio, error)) error {
	unlock := s.lock()
	defer unlock()
	items, err := s.read()
	if err != nil {
		return err
	}
	items, err = fn(items)
	if err != nil {
		return err
	}
	if len(items) > maxPerOwner {
		return errors.New("too many connections for this account")
	}
	now := time.Now().UTC().Format(time.RFC3339)
	for i := range items {
		c := &items[i]
		c.Owner = s.owner
		if c.ID == "" {
			c.ID = NewID()
		}
		if c.AuthRef == "" {
			c.AuthRef = NewAuthRef()
		}
		if c.CreatedAt == "" {
			c.CreatedAt = now
		}
		if c.Status == "" {
			c.Status = Available
		}
		if c.Allowed == nil {
			c.Allowed = []string{CapRead}
		}
	}
	return s.write(items)
}

// Put desa (crea o substitueix per id) una connexió i la torna completa.
func (s *Store) Put(c Connexio) (Connexio, error) {
	var out Connexio
	err := s.Update(func(items []Connexio) ([]Connexio, error) {
		if c.ID == "" {
			c.ID = NewID()
		}
		found := false
		for i := range items {
			if items[i].ID == c.ID {
				items[i] = c
				found = true
			}
		}
		if !found {
			items = append(items, c)
		}
		return items, nil
	})
	if err != nil {
		return out, err
	}
	out, _ = s.Get(c.ID)
	return out, nil
}

// Modify canvia una connexió existent sota el bloqueig.
func (s *Store) Modify(id string, fn func(c *Connexio) error) (Connexio, error) {
	var out Connexio
	err := s.Update(func(items []Connexio) ([]Connexio, error) {
		for i := range items {
			if items[i].ID == id {
				if err := fn(&items[i]); err != nil {
					return nil, err
				}
				out = items[i]
				return items, nil
			}
		}
		return nil, os.ErrNotExist
	})
	return out, err
}

// Remove treu una connexió del registre (el secret l'esborra qui crida).
func (s *Store) Remove(id string) (Connexio, error) {
	var out Connexio
	err := s.Update(func(items []Connexio) ([]Connexio, error) {
		kept := items[:0]
		found := false
		for _, c := range items {
			if c.ID == id {
				out, found = c, true
				continue
			}
			kept = append(kept, c)
		}
		if !found {
			return nil, os.ErrNotExist
		}
		return kept, nil
	})
	return out, err
}

// ValidID diu si id té la forma d'un identificador de connexió.
func ValidID(id string) bool {
	if !strings.HasPrefix(id, "con-") || len(id) > 64 || len(id) < 5 {
		return false
	}
	for _, r := range id[4:] {
		if !(r >= '0' && r <= '9' || r >= 'a' && r <= 'z' || r == '-') {
			return false
		}
	}
	return true
}
