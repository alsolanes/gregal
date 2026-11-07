package web

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"

	"gregal/internal/agent"
	"gregal/internal/llm"
)

// Steer dels vents: missatges escrits mentre el torn treballa, com fa el
// motor amb /api/steer (steer.go) però per als torns dels vents, rotllanes
// i colles. Cada torn viu té una bústia (la clau és la mateixa que el
// registre de torns: sessió + kind + id). El bucle la buida abans de cada
// pas i injecta els missatges com a guies (agent.GuiaInterjeccio): el model
// els té en compte al pas següent sense aturar les eines que corren.
//
// POST /api/winds/steer {kind: "wind"|"rotllana", id, text, interrupt?}.
// 200 si el torn el rebrà; 409 si no n'hi ha cap de viu (el client l'ha
// d'enviar llavors com a missatge nou).

type windSteerBox struct {
	mu    sync.Mutex
	inbox []string
	stop  context.CancelFunc // talla la crida al model en curs (interrupt)
	abort bool
}

var windSteerRegistry = struct {
	sync.Mutex
	boxes map[turnKey]*windSteerBox
}{boxes: map[turnKey]*windSteerBox{}}

// windSteerBegin registra la bústia del torn que comença; el que torna s'ha
// de cridar en acabar i retorna els missatges no lliurats.
func windSteerBegin(key turnKey) func() []string {
	b := &windSteerBox{}
	windSteerRegistry.Lock()
	windSteerRegistry.boxes[key] = b
	windSteerRegistry.Unlock()
	return func() []string {
		windSteerRegistry.Lock()
		if windSteerRegistry.boxes[key] == b {
			delete(windSteerRegistry.boxes, key)
		}
		windSteerRegistry.Unlock()
		b.mu.Lock()
		defer b.mu.Unlock()
		left := b.inbox
		b.inbox, b.stop, b.abort = nil, nil, false
		return left
	}
}

// windSteerQueue deixa un missatge per al torn en curs. false si no n'hi ha.
func windSteerQueue(key turnKey, text string, interrupt bool) bool {
	windSteerRegistry.Lock()
	b := windSteerRegistry.boxes[key]
	windSteerRegistry.Unlock()
	if b == nil {
		return false
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.inbox = append(b.inbox, text)
	if interrupt && b.stop != nil {
		b.abort = true
		b.stop()
		b.stop = nil
	}
	return true
}

// windSteerDrain buida la bústia del torn (cridat pel bucle entre passos).
func windSteerDrain(key turnKey) []string {
	windSteerRegistry.Lock()
	b := windSteerRegistry.boxes[key]
	windSteerRegistry.Unlock()
	if b == nil {
		return nil
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	out := b.inbox
	b.inbox = nil
	return out
}

// windSteerModelCall registra com tallar la crida al model que comença; el
// que torna es crida quan acaba i diu si s'ha tallat per un interrupt.
func windSteerModelCall(key turnKey, cancel context.CancelFunc) func() bool {
	windSteerRegistry.Lock()
	b := windSteerRegistry.boxes[key]
	windSteerRegistry.Unlock()
	if b == nil {
		cancel()
		return func() bool { return false }
	}
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

// handleWindSteer: POST {kind, id, text, interrupt?}.
func (s *Server) handleWindSteer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Kind      string `json:"kind"`
		ID        string `json:"id"`
		Wind      string `json:"wind"` // àlies d'ID (kind=wind)
		Rotllana  string `json:"rotllana"`
		Text      string `json:"text"`
		Interrupt bool   `json:"interrupt"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	kind := strings.TrimSpace(req.Kind)
	id := strings.TrimSpace(req.ID)
	if kind == "wind" && id == "" {
		id = strings.TrimSpace(req.Wind)
	}
	if kind == "rotllana" && id == "" {
		id = strings.TrimSpace(req.Rotllana)
	}
	if kind == "" {
		// Compatibilitat: sense kind, wind o rotllana pel camp present.
		if req.Wind != "" {
			kind, id = "wind", strings.TrimSpace(req.Wind)
		} else if req.Rotllana != "" {
			kind, id = "rotllana", strings.TrimSpace(req.Rotllana)
		}
	}
	text := strings.TrimSpace(req.Text)
	if text == "" || id == "" || (kind != "wind" && kind != "rotllana") {
		w.WriteHeader(http.StatusBadRequest)
		writeJSON(w, map[string]string{"error": "cal kind (wind|rotllana), id i text"})
		return
	}
	if !windSteerQueue(turnKey{scope: s.sessDir(), kind: kind, id: id}, text, req.Interrupt) {
		w.WriteHeader(http.StatusConflict)
		writeJSON(w, map[string]any{"ok": false, "error": "cap torn en curs; envia-ho com a missatge nou"})
		return
	}
	writeJSON(w, map[string]any{"ok": true, "interrupt": req.Interrupt})
}

// injectaSteer buida la bústia del torn i afegeix les guies a l'historial.
// Torna l'historial ampliat (o el mateix si no hi ha res). És el punt
// segur: abans d'una crida al model, amb totes les eines del lot tancades.
func injectaSteer(key turnKey, hist []llm.Message) []llm.Message {
	for _, m := range windSteerDrain(key) {
		hist = append(hist, llm.Message{Role: "user", Content: agent.GuiaInterjeccio(m)})
	}
	return hist
}
