package connexions

import (
	"encoding/json"
	"errors"
	"os"
	"strings"
	"time"
)

// Autoritzacions OAuth començades: el callback arriba del navegador sense
// cap sessió de Gregal i només porta el state. El que cal per acabar-la
// (owner, connexió, verificador PKCE, client) es desa xifrat a disc: un
// reinici del servidor o tancar l'app durant el consentiment no la perd.

// PendingTTL és quant temps val una autorització començada.
var PendingTTL = 30 * time.Minute

const pendingOwner = "\x00pending" // espai propi: mai coincideix amb un nom d'usuari

type pendingEnvelope struct {
	Created time.Time       `json:"created"`
	Data    json.RawMessage `json:"data"`
}

// PutPending desa una autorització començada sota state.
func (v *Vault) PutPending(state string, x any) error {
	if strings.TrimSpace(state) == "" {
		return errors.New("connexions: empty state")
	}
	raw, err := json.Marshal(x)
	if err != nil {
		return err
	}
	env, _ := json.Marshal(pendingEnvelope{Created: time.Now(), Data: raw})
	v.sweepPending()
	return v.Put(pendingOwner, "pend:"+state, env)
}

// TakePending consumeix (d'un sol ús) l'autorització de state.
func (v *Vault) TakePending(state string, x any) error {
	if strings.TrimSpace(state) == "" {
		return ErrNoSecret
	}
	ref := "pend:" + state
	raw, err := v.Get(pendingOwner, ref)
	if err != nil {
		return err
	}
	_ = v.Delete(pendingOwner, ref)
	var env pendingEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return err
	}
	if time.Since(env.Created) > PendingTTL {
		return errors.New("the authorization took too long; start it again")
	}
	return json.Unmarshal(env.Data, x)
}

// HasPending diu si hi ha una autorització pendent amb aquest state (sense
// consumir-la): el callback compartit decideix així a qui toca.
func (v *Vault) HasPending(state string) bool {
	if strings.TrimSpace(state) == "" {
		return false
	}
	_, err := v.Get(pendingOwner, "pend:"+state)
	return err == nil
}

// sweepPending treu les pendents caducades (no s'acumulen).
func (v *Vault) sweepPending() {
	v.mu.Lock()
	defer v.mu.Unlock()
	f, err := v.read(pendingOwner)
	if err != nil || len(f.Items) < 20 {
		return
	}
	g, err := v.aead()
	if err != nil {
		return
	}
	changed := false
	for ref := range f.Items {
		raw, err := openSealed(g, f.Items[ref], pendingOwner, ref)
		var env pendingEnvelope
		if err != nil || json.Unmarshal(raw, &env) != nil || time.Since(env.Created) > PendingTTL {
			delete(f.Items, ref)
			changed = true
		}
	}
	if changed {
		_ = v.write(pendingOwner, f)
	}
}

// RemoveOwner esborra tot el magatzem d'un owner (compte esborrat).
func (v *Vault) RemoveOwner(owner string) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	err := os.Remove(v.path(owner))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}
