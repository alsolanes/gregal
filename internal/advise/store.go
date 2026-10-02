package advise

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Store desa propostes en un JSON amb escriptura atòmica. És tolerant:
// si el fitxer falta, parteix de buit; si és corrupte, el conserva com a
// .corrupte-<data> i parteix de buit. Mai perd dades de l'usuari.
type Store struct {
	path string
}

// Open prepara la botiga al camí donat (es crea en desar).
func Open(path string) *Store {
	return &Store{path: path}
}

// Load llegeix les propostes desades i en poda les caducades.
func (s *Store) Load() ([]Proposal, error) {
	raw, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var props []Proposal
	if err := json.Unmarshal(raw, &props); err != nil {
		backup := s.path + fmt.Sprintf(".corrupte-%s", time.Now().UTC().Format("20060102-150405"))
		_ = os.Rename(s.path, backup)
		return nil, fmt.Errorf("advise: fitxer corrupte (%s), s'ha conservat a %s i es parteix de buit: %w", s.path, backup, err)
	}
	return prune(props, time.Now().UTC()), nil
}

// Save desa la llista de forma atòmica (temporal + rename) amb
// permisos 0600: les propostes poden citar camins privats.
func (s *Store) Save(props []Proposal) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(props, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".nou"
	if err := os.WriteFile(tmp, append(raw, '\n'), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

// Upsert fusiona propostes noves: les existents (mateix ID) es
// refresquen sense duplicar, i les caducades es poden.
func (s *Store) Upsert(noves []Proposal) ([]Proposal, error) {
	actuals, err := s.Load()
	if err != nil {
		// Fitxer corrupte: Load ja l'ha apartat; continuem amb buit
		// però avisem a qui crida.
		actuals = nil
	}
	byID := map[string]int{}
	for i, p := range actuals {
		byID[p.ID] = i
	}
	for _, n := range noves {
		if i, ok := byID[n.ID]; ok {
			actuals[i] = n
			continue
		}
		byID[n.ID] = len(actuals)
		actuals = append(actuals, n)
	}
	actuals = prune(actuals, time.Now().UTC())
	if serr := s.Save(actuals); serr != nil {
		return actuals, serr
	}
	return actuals, err
}

func prune(props []Proposal, now time.Time) []Proposal {
	out := props[:0]
	for _, p := range props {
		if p.ExpiresAt.IsZero() || p.ExpiresAt.After(now) {
			out = append(out, p)
		}
	}
	return out
}
