package web

import (
	"log"
	"net/http"
	"strings"

	"gregal/internal/agent"
)

// noteAI és la crida al model; els tests la substitueixen.
var noteAI = agent.NoteAI

// handleNotesAI: POST {id, action, instruction?, body?} → {body} amb la
// proposta del model per a la nota (netejar, organitzar, resumir, tasques o
// el que l'usuari demani). No desa res: la finestra ensenya la proposta i
// l'usuari decideix. body, si hi és, és el text que hi ha ara a l'editor
// (pot no estar desat encara).
func (s *Server) handleNotesAI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		ID          string  `json:"id"`
		Action      string  `json:"action"`
		Instruction string  `json:"instruction"`
		Title       *string `json:"title"`
		Body        *string `json:"body"`
	}
	if !decodeSmall(w, r, &req) {
		return
	}
	if !agent.NoteAIAction(req.Action) {
		http.Error(w, "unknown action", http.StatusBadRequest)
		return
	}
	title, body := "", ""
	if req.ID != "" {
		notesMu.Lock()
		n, err := s.readNoteLocked(req.ID)
		notesMu.Unlock()
		if err != nil {
			http.Error(w, "note not found", http.StatusNotFound)
			return
		}
		title, body = n.Title, n.Body
	}
	if req.Title != nil {
		title = *req.Title
	}
	if req.Body != nil {
		body = *req.Body
	}
	if len(body) > notesMaxBody {
		http.Error(w, "note too long", http.StatusRequestEntityTooLarge)
		return
	}
	p, role, ok := s.sideRoleRef()
	if !ok {
		http.Error(w, "configure an active model first", http.StatusBadRequest)
		return
	}
	s.mu.Lock()
	cfg, client := s.cfg, s.client
	s.mu.Unlock()
	out, err := noteAI(r.Context(), client, cfg, p, role, req.Action, req.Instruction, title, body)
	if err != nil {
		log.Printf("note ai error: %s", redactError(err))
		msg := "the model could not edit the note"
		if strings.Contains(err.Error(), "empty") || strings.Contains(err.Error(), "say what") {
			msg = err.Error()
		}
		http.Error(w, msg, http.StatusBadGateway)
		return
	}
	writeJSON(w, map[string]any{"body": out, "changed": out != strings.TrimSpace(body)})
}
