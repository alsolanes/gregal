package web

import (
	"context"
	"net/http"
	"strings"
)

// Aturar un torn: igual que Esc a Claude Code o opencode.
//
// El torn d'un vent o d'una colla corre amb un context propi (no el de la
// petició HTTP): si el client tanca la connexió, la feina continua i es desa,
// que és el que vols al mòbil. Però quan la persona VEU que el vent s'ha
// encallat o ja no li cal la resposta, ha de poder parar-lo; sense això
// l'única sortida era esperar el límit dur (cinc minuts amb els passos nous).
//
// Cada torn en curs es registra per (tipus, id) amb el seu cancel·lador. Un
// torn nou sobre el mateix vent substitueix l'anterior. Cancel·lar no esborra
// res: el que ja s'ha fet queda fet i el torn acaba amb «aturat».

type turnKey struct{ kind, id string }

// turnHandle identifica un torn concret: tancar un torn vell no ha de
// desregistrar el que l'ha substituït (es compara el punter).
type turnHandle struct{ cancel context.CancelFunc }

// beginWindTurn registra el torn i en torna el context i el tancament.
func (s *Server) beginWindTurn(kind, id string, parent context.Context) (context.Context, func()) {
	ctx, cancel := context.WithCancel(parent)
	key := turnKey{kind: kind, id: id}
	h := &turnHandle{cancel: cancel}
	s.mu.Lock()
	if s.windTurns == nil {
		s.windTurns = map[turnKey]*turnHandle{}
	}
	prev := s.windTurns[key]
	s.windTurns[key] = h
	s.mu.Unlock()
	if prev != nil && prev != h {
		prev.cancel() // un torn nou sobre el mateix vent substitueix el vell
	}
	done := func() {
		s.mu.Lock()
		if s.windTurns[key] == h {
			delete(s.windTurns, key)
		}
		s.mu.Unlock()
		cancel()
	}
	return ctx, done
}

// cancelWindTurn atura el torn en curs d'un vent o rotllana. Torna fals si no
// n'hi havia cap (l'operació és idempotent: la UI no ha de fallar).
func (s *Server) cancelWindTurn(kind, id string) bool {
	s.mu.Lock()
	h := s.windTurns[turnKey{kind: kind, id: id}]
	s.mu.Unlock()
	if h == nil {
		return false
	}
	h.cancel()
	return true
}

// tornAturat diu si l'error d'un torn és que l'han aturat (i no una fallada).
func tornAturat(err error) bool {
	return err != nil && (err == context.Canceled || strings.Contains(strings.ToLower(err.Error()), "context canceled"))
}

// handleWindCancel atura el torn d'un vent o d'una rotllana.
// POST {"wind":"id"} o {"rotllana":"id"}.
func (s *Server) handleWindCancel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "només POST", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Wind     string `json:"wind"`
		Rotllana string `json:"rotllana"`
	}
	if !decodeSmall(w, r, &req) {
		return
	}
	wind := strings.TrimSpace(req.Wind)
	rot := strings.TrimSpace(req.Rotllana)
	if wind == "" && rot == "" {
		http.Error(w, "cal un vent o una rotllana", http.StatusBadRequest)
		return
	}
	aturat := false
	if wind != "" {
		aturat = s.cancelWindTurn("wind", wind) || aturat
	}
	if rot != "" {
		aturat = s.cancelWindTurn("rotllana", rot) || aturat
	}
	writeJSON(w, map[string]bool{"ok": true, "aturat": aturat})
}
