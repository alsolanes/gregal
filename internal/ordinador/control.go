package ordinador

// Control humà de l'ordinador (paquet B3, pla v2 §5 i §6 «Camí B»).
//
//   - Lease de control: qui té el control interactiu, la persona o ningú
//     (l'agent el té implícitament quan actua). Mentre la persona el té,
//     l'agent no actua: les eines del navegador esperen i no fan captures
//     (tools.SetBrowserControlGuard, Manager.Exec). La lease és curta i es
//     renova mentre el visor és a la vista; si l'app es tanca, caduca sola i
//     l'ordinador es pot tornar a desar. Es desa a disc: un reinici de
//     l'amfitrió no la perd ni la deixa penjada (té data de caducitat).
//   - Sessions del visor: un testimoni d'un sol ús i de vida curta (lligat al
//     propietari) es bescanvia per una sessió del visor (galeta HttpOnly) que
//     també caduca. El visor mai no s'exposa sense sessió.

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Durades per defecte. Variables perquè els tests les escurcin.
var (
	// UserLeaseTTL: la lease de la persona dura això des de l'última
	// renovació (el visor la renova cada ~30 s mentre és a la vista).
	UserLeaseTTL = 2 * time.Minute
	// ViewerTicketTTL: el testimoni d'un sol ús caduca si no es fa servir.
	ViewerTicketTTL = 60 * time.Second
	// ViewerSessionTTL: la sessió del visor caduca sense batec.
	ViewerSessionTTL = 5 * time.Minute
	// ViewerSessionMax: topall absolut d'una sessió del visor.
	ViewerSessionMax = 2 * time.Hour
)

// ErrUserHasControl: la persona té el control de l'ordinador i l'agent no
// hi pot actuar fins que el retorni.
var ErrUserHasControl = errors.New("ordinador: the person has taken control of the computer; the agent waits until they give it back")

// Lease és qui té el control d'un ordinador.
type Lease struct {
	// Key és la clau del propietari (tools.OwnerKeyFor).
	Key     string    `json:"key"`
	Holder  string    `json:"holder"` // user
	Since   time.Time `json:"since"`
	Expires time.Time `json:"expires"`
	// Reason: viewer (la persona l'ha pres des del visor) o handoff (una
	// feina li ha passat per iniciar sessió).
	Reason string `json:"reason,omitempty"`
	// Handoff: la decisió d'espera lligada, si n'hi ha.
	Handoff string `json:"handoff,omitempty"`
}

// Leases és el registre de leases per propietari, desat a disc.
type Leases struct {
	mu   sync.Mutex
	path string
	m    map[string]*Lease
	now  func() time.Time
}

// NewLeases crea el registre; amb path, el llegeix i hi desa els canvis.
func NewLeases(path string) *Leases {
	l := &Leases{path: path, m: map[string]*Lease{}, now: time.Now}
	if path != "" {
		if raw, err := os.ReadFile(path); err == nil {
			var list []*Lease
			if json.Unmarshal(raw, &list) == nil {
				for _, x := range list {
					if x != nil && x.Key != "" {
						l.m[x.Key] = x
					}
				}
			}
		}
	}
	return l
}

// SetClock substitueix el rellotge (tests).
func (l *Leases) SetClock(now func() time.Time) {
	l.mu.Lock()
	l.now = now
	l.mu.Unlock()
}

func (l *Leases) saveLocked() {
	if l.path == "" {
		return
	}
	list := make([]*Lease, 0, len(l.m))
	for _, x := range l.m {
		list = append(list, x)
	}
	b, _ := json.Marshal(list)
	_ = os.MkdirAll(filepath.Dir(l.path), 0o700)
	tmp := l.path + ".tmp"
	if os.WriteFile(tmp, b, 0o600) == nil {
		_ = os.Rename(tmp, l.path)
	}
}

// currentLocked torna la lease vigent (i oblida la caducada).
func (l *Leases) currentLocked(key string) *Lease {
	x := l.m[key]
	if x == nil {
		return nil
	}
	if !l.now().Before(x.Expires) {
		delete(l.m, key)
		l.saveLocked()
		return nil
	}
	return x
}

// Get torna una còpia de la lease vigent (nil si ningú no té el control).
func (l *Leases) Get(key string) *Lease {
	if l == nil {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	x := l.currentLocked(key)
	if x == nil {
		return nil
	}
	cp := *x
	return &cp
}

// HeldByUser diu si la persona té ara el control.
func (l *Leases) HeldByUser(key string) bool { return l.Get(key) != nil }

// Take dona el control a la persona (o el renova si ja el té).
func (l *Leases) Take(key, reason, handoff string) Lease {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	x := l.currentLocked(key)
	if x == nil {
		x = &Lease{Key: key, Holder: "user", Since: now}
		l.m[key] = x
	}
	if reason != "" {
		x.Reason = reason
	}
	if handoff != "" {
		x.Handoff = handoff
	}
	x.Expires = now.Add(UserLeaseTTL)
	l.saveLocked()
	return *x
}

// Renew allarga la lease si la persona la té; no en crea cap.
func (l *Leases) Renew(key string) (Lease, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	x := l.currentLocked(key)
	if x == nil {
		return Lease{}, false
	}
	x.Expires = l.now().Add(UserLeaseTTL)
	l.saveLocked()
	return *x, true
}

// Release torna el control a l'agent. Torna cert si la persona el tenia.
func (l *Leases) Release(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.currentLocked(key) == nil {
		return false
	}
	delete(l.m, key)
	l.saveLocked()
	return true
}

// ViewerSession és una sessió del visor oberta per un propietari.
type ViewerSession struct {
	ID string
	// Key és la clau del propietari; Owner el nom del compte (buit en
	// mode local) i Computer l'ordinador (buit en mode local).
	Key, Owner, Computer string
	Created, Seen        time.Time
}

type viewerTicket struct {
	sess    ViewerSession
	expires time.Time
}

// Viewers guarda els testimonis d'un sol ús i les sessions del visor. En
// memòria: després d'un reinici n'hi ha prou de tornar a obrir el visor.
type Viewers struct {
	mu      sync.Mutex
	tickets map[string]viewerTicket
	sess    map[string]*ViewerSession
	now     func() time.Time
}

// NewViewers crea el registre de sessions del visor.
func NewViewers() *Viewers {
	return &Viewers{tickets: map[string]viewerTicket{}, sess: map[string]*ViewerSession{}, now: time.Now}
}

// SetClock substitueix el rellotge (tests).
func (v *Viewers) SetClock(now func() time.Time) {
	v.mu.Lock()
	v.now = now
	v.mu.Unlock()
}

func randomID() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

func (v *Viewers) sweepLocked() {
	now := v.now()
	for k, t := range v.tickets {
		if !now.Before(t.expires) {
			delete(v.tickets, k)
		}
	}
	for k, s := range v.sess {
		if !now.Before(s.Seen.Add(ViewerSessionTTL)) || !now.Before(s.Created.Add(ViewerSessionMax)) {
			delete(v.sess, k)
		}
	}
}

// Issue crea un testimoni d'un sol ús per a aquest propietari.
func (v *Viewers) Issue(key, owner, computer string) (ticket string, expires time.Time) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.sweepLocked()
	ticket = randomID()
	expires = v.now().Add(ViewerTicketTTL)
	v.tickets[ticket] = viewerTicket{sess: ViewerSession{Key: key, Owner: owner, Computer: computer}, expires: expires}
	return ticket, expires
}

// Redeem bescanvia el testimoni per una sessió nova. El testimoni deixa de
// valer en el mateix moment, l'hagi bescanviat qui sigui.
func (v *Viewers) Redeem(ticket string) (ViewerSession, bool) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.sweepLocked()
	t, ok := v.tickets[ticket]
	if !ok || ticket == "" {
		return ViewerSession{}, false
	}
	delete(v.tickets, ticket)
	now := v.now()
	s := t.sess
	s.ID, s.Created, s.Seen = randomID(), now, now
	v.sess[s.ID] = &s
	return s, true
}

// Session torna la sessió vigent; touch en renova la caducitat (batec).
func (v *Viewers) Session(id string, touch bool) (ViewerSession, bool) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.sweepLocked()
	s, ok := v.sess[id]
	if !ok || id == "" {
		return ViewerSession{}, false
	}
	if touch {
		s.Seen = v.now()
	}
	return *s, true
}

// End tanca la sessió.
func (v *Viewers) End(id string) {
	v.mu.Lock()
	delete(v.sess, id)
	v.mu.Unlock()
}
