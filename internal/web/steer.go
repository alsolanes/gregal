package web

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
)

// Missatges escrits mentre el torn treballa (POST /api/steer). Cada sessió
// té una bústia: el bucle del torn (conduirTorn) la buida a cada volta i
// passa els missatges al motor (agent.Torn.Interjeccio), que els afegeix
// al següent punt segur. Amb interrupt, a més, es talla la crida al model
// que estigui en curs perquè el pas es refaci ja amb el missatge; les eines
// que corren no es tallen mai (deixarien fitxers a mitges).

type steerBox struct {
	mu    sync.Mutex
	live  bool // hi ha un torn d'aquesta sessió dins conduirTorn
	inbox []string
	stop  context.CancelFunc // talla la crida al model en curs
	abort bool               // l'última crida s'ha tallat per un interrupt
}

// steerBegin marca el torn com a viu; el que torna s'ha de cridar en acabar
// i dona els missatges que no s'han pogut lliurar.
func (s *Server) steerBegin() func() []string {
	b := &s.steer
	b.mu.Lock()
	b.live, b.inbox, b.stop, b.abort = true, nil, nil, false
	b.mu.Unlock()
	return func() []string {
		b.mu.Lock()
		defer b.mu.Unlock()
		left := b.inbox
		b.live, b.inbox, b.stop, b.abort = false, nil, nil, false
		return left
	}
}

// queueSteer deixa un missatge per al torn en curs. false si no n'hi ha.
func (s *Server) queueSteer(text string, interrupt bool) bool {
	b := &s.steer
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.live {
		return false
	}
	b.inbox = append(b.inbox, text)
	if interrupt && b.stop != nil {
		b.abort = true
		b.stop()
		b.stop = nil
	}
	return true
}

func (s *Server) drainSteer() []string {
	b := &s.steer
	b.mu.Lock()
	defer b.mu.Unlock()
	out := b.inbox
	b.inbox = nil
	return out
}

// steerModelCall registra com tallar la crida al model que comença; el que
// torna s'ha de cridar quan acaba i diu si s'ha tallat per un interrupt.
func (s *Server) steerModelCall(cancel context.CancelFunc) func() bool {
	b := &s.steer
	b.mu.Lock()
	b.stop, b.abort = cancel, false
	b.mu.Unlock()
	return func() bool {
		b.mu.Lock()
		defer b.mu.Unlock()
		b.stop = nil
		aborted := b.abort
		b.abort = false
		return aborted
	}
}

// handleSteer: POST {text, interrupt?}. 200 si el torn el rebrà; 409 si no
// hi ha cap torn treballant (el client l'envia llavors com a missatge nou).
func (s *Server) handleSteer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Text      string `json:"text"`
		Interrupt bool   `json:"interrupt"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	text := strings.TrimSpace(req.Text)
	if text == "" {
		w.WriteHeader(http.StatusBadRequest)
		writeJSON(w, map[string]string{"error": "empty message"})
		return
	}
	if !s.queueSteer(text, req.Interrupt) {
		w.WriteHeader(http.StatusConflict)
		writeJSON(w, map[string]any{"ok": false, "error": "no turn is running; send it as a new message"})
		return
	}
	writeJSON(w, map[string]any{"ok": true, "interrupt": req.Interrupt})
}
