package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"gregal/internal/tools"
)

// Preguntes i aprovacions dels vents.
//
// Un torn de vent no tenia cap lloc on esperar una resposta: les accions que
// demanaven permís es bloquejaven en silenci («falta permís») i les preguntes
// del vent no arribaven a ningú. Ara el torn s'atura de debò: emet
// approve_request / question_request pel mateix flux de la conversa
// (/api/winds/chat/stream i /api/winds/rotllana/stream) i espera la resposta
// a /api/winds/answer. Mentre espera, la petició és a /api/winds/pending: si
// tanques l'app i hi tornes, la trobes esperant.
//
// Tenen els seus propis mapes, apart dels del xat principal (approvals i
// questions): una aprovació d'un vent mai no surt com un fantasma al Codi.

// windAskMax és el màxim que un torn espera una resposta; passat, l'acció no
// es fa i el vent ho sap.
const windAskMax = 20 * time.Minute

type windAskOpt struct {
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
}

// windAskReq és una petició pendent: una aprovació (Type "approve") o una
// pregunta amb opcions (Type "question").
type windAskReq struct {
	Key     string       `json:"key"`
	Type    string       `json:"type"`
	Query   string       `json:"query,omitempty"`
	Options []windAskOpt `json:"options,omitempty"`
	Name    string       `json:"name,omitempty"`   // l'eina
	Kind    string       `json:"kind,omitempty"`   // install | download | delete | system | other
	What    string       `json:"what,omitempty"`   // l'ordre o l'acció, curta
	Reason  string       `json:"reason,omitempty"` // per què es pregunta
	At      int64        `json:"at"`
	Expires int64        `json:"expires"`

	ch chan windAnswer
}

type windAnswer struct {
	Approve bool
	Answer  string
}

func (s *Server) windAskInit() {
	if s.windAsks == nil {
		s.windAsks = map[string]*windAskReq{}
	}
}

// windAskWait publica la petició al flux del torn i espera la resposta.
// Torna (resposta, hiHaResposta): sense resposta (temps, torn aturat) no es
// fa res. emit rep el JSON de la petició.
func (s *Server) windAskWait(ctx context.Context, emit func(kind, text string), r *windAskReq) (windAnswer, bool) {
	s.mu.Lock()
	s.windAskInit()
	s.windAskSeq++
	r.Key = fmt.Sprintf("wa%d", s.windAskSeq)
	r.At = time.Now().UnixMilli()
	r.Expires = time.Now().Add(windAskMax).UnixMilli()
	r.ch = make(chan windAnswer, 1)
	s.windAsks[r.Key] = r
	s.mu.Unlock()

	drop := func() {
		s.mu.Lock()
		delete(s.windAsks, r.Key)
		s.mu.Unlock()
	}
	defer drop()

	if b, err := json.Marshal(r); err == nil && emit != nil {
		emit(r.Type+"_request", string(b))
	}
	timer := time.NewTimer(windAskMax)
	defer timer.Stop()
	var ans windAnswer
	ok := false
	select {
	case ans = <-r.ch:
		ok = true
	case <-ctx.Done():
	case <-timer.C:
	}
	if emit != nil {
		b, _ := json.Marshal(map[string]any{"key": r.Key, "answered": ok})
		emit("ask_resolved", string(b))
	}
	return ans, ok
}

// windApprove demana permís per a una acció i bloqueja el torn fins que la
// persona respon. Torna el text per al model si no es fa.
func (s *Server) windApprove(ctx context.Context, emit func(kind, text string), name, kind, what, reason string) (bool, string) {
	ans, ok := s.windAskWait(ctx, emit, &windAskReq{Type: "approve", Name: name, Kind: kind, What: what, Reason: reason})
	switch {
	case !ok:
		return false, "NOT RUN: the user did not answer in time. Do not retry it; say what you needed and ask again in your reply."
	case !ans.Approve:
		return false, "NOT RUN: the user declined this. Do not retry it and do not look for a way around it; carry on without it or explain what you could not do."
	}
	return true, ""
}

// windQuestion fa la pregunta del vent i torna el text de la resposta per al model.
func (s *Server) windQuestion(ctx context.Context, emit func(kind, text string), query string, opts []tools.QuestionOption) string {
	req := &windAskReq{Type: "question", Query: query}
	for _, o := range opts {
		req.Options = append(req.Options, windAskOpt{Label: o.Label, Description: o.Description})
	}
	ans, ok := s.windAskWait(ctx, emit, req)
	if !ok || strings.TrimSpace(ans.Answer) == "" {
		return "The user did not answer in time. Do not assume an answer: say what you need to know and carry on with what does not depend on it."
	}
	return "The user answered: " + strings.TrimSpace(ans.Answer)
}

// forwardWindAsk passa al client les peticions que surten del bucle del vent.
func forwardWindAsk(emit func(string, any), kind, text string) bool {
	switch kind {
	case "approve_request", "question_request", "ask_resolved":
		emit(kind, json.RawMessage(text))
		return true
	}
	return false
}

// handleWindsPending: GET → les aprovacions i preguntes que esperen resposta,
// de la més antiga a la més nova.
func (s *Server) handleWindsPending(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	list := make([]*windAskReq, 0, len(s.windAsks))
	for _, a := range s.windAsks {
		list = append(list, a)
	}
	s.mu.Unlock()
	sort.Slice(list, func(i, j int) bool { return list[i].At < list[j].At })
	writeJSON(w, map[string]any{"pending": list})
}

// handleWindsAnswer: POST {key, approve?, answer?} desbloqueja el torn.
func (s *Server) handleWindsAnswer(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Key     string `json:"key"`
		Approve bool   `json:"approve"`
		Answer  string `json:"answer"`
	}
	if r.Method != http.MethodPost || !decodeSmall(w, r, &req) {
		if r.Method != http.MethodPost {
			http.Error(w, "POST only", http.StatusMethodNotAllowed)
		}
		return
	}
	s.mu.Lock()
	a, ok := s.windAsks[strings.TrimSpace(req.Key)]
	if ok {
		delete(s.windAsks, a.Key)
	}
	s.mu.Unlock()
	if !ok {
		http.Error(w, "expired or unknown request", http.StatusNotFound)
		return
	}
	if len([]rune(req.Answer)) > 2000 {
		req.Answer = string([]rune(req.Answer)[:2000])
	}
	a.ch <- windAnswer{Approve: req.Approve, Answer: req.Answer}
	writeJSON(w, map[string]bool{"ok": true})
}

// windAskArgsHint: una pista curta (camí, ordre, adreça) de la crida d'una eina,
// perquè l'aprovació digui de què va sense mostrar tots els arguments.
func windAskArgsHint(argsJSON string) string {
	var m map[string]any
	if json.Unmarshal([]byte(argsJSON), &m) != nil {
		return ""
	}
	for _, k := range []string{"command", "path", "file", "dir", "url", "target", "name", "action"} {
		if v, ok := m[k].(string); ok && strings.TrimSpace(v) != "" {
			v = strings.TrimSpace(v)
			if r := []rune(v); len(r) > 100 {
				v = string(r[:100]) + "…"
			}
			return v
		}
	}
	return ""
}
