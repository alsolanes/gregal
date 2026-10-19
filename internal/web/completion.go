package web

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"gregal/internal/llm"
)

const (
	inlineCompletionMaxRequest = 16 * 1024
	inlineCompletionMaxChars   = 9000
	inlineCompletionTokens     = 128
)

var inlineCompletionSlots = make(chan struct{}, 4)

type inlineCompletionRequest struct {
	Language  string `json:"language"`
	Prefix    string `json:"prefix"`
	Suffix    string `json:"suffix"`
	Workspace string `json:"workspace"`
}

// handleInlineCompletion is deliberately separate from /api/chat and /api/agent:
// suggestions do not enter conversation history and cannot invoke tools.
func (s *Server) handleInlineCompletion(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "només POST", http.StatusMethodNotAllowed)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, inlineCompletionMaxRequest)
	var req inlineCompletionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "sol·licitud de compleció no vàlida", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(req.Language) == "" || strings.TrimSpace(req.Workspace) == "" || len(req.Prefix)+len(req.Suffix) > inlineCompletionMaxChars {
		http.Error(w, "cal indicar llenguatge, projecte i un fragment de codi acotat", http.StatusBadRequest)
		return
	}
	workspace, err := resolveWorkspace(req.Workspace)
	if err != nil || s.user.Allow(workspace) != nil {
		http.Error(w, "projecte no disponible", http.StatusForbidden)
		return
	}
	select {
	case inlineCompletionSlots <- struct{}{}:
		defer func() { <-inlineCompletionSlots }()
	case <-r.Context().Done():
		return
	default:
		http.Error(w, "hi ha massa complecions en curs", http.StatusTooManyRequests)
		return
	}

	s.mu.Lock()
	roleName := "code"
	if _, ok := s.cfg.Roles[roleName]; !ok {
		roleName = s.role
	}
	p, role := s.roleRefFor(roleName)
	s.mu.Unlock()
	if strings.TrimSpace(p.BaseURL) == "" || strings.TrimSpace(role.Model) == "" {
		http.Error(w, "no hi ha cap model de codi configurat", http.StatusServiceUnavailable)
		return
	}
	if len(req.Prefix) > 7000 {
		req.Prefix = req.Prefix[len(req.Prefix)-7000:]
	}
	if len(req.Suffix) > 1500 {
		req.Suffix = req.Suffix[:1500]
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	ctx = llm.WithSideCall(ctx)
	prompt := "Complete the code at the cursor. Return only the new code to insert, with no Markdown, explanation, or repeated prefix. Use the suffix as context and do not duplicate it. Keep the completion short and preserve the file's style. If no useful completion is clear, return an empty string."
	input := "Language: " + req.Language + "\n\nCode before cursor:\n" + req.Prefix + "\n\n<|cursor|>\n\nCode after cursor:\n" + req.Suffix
	reply, err := s.client.ChatStream(ctx, p.BaseURL, p.APIKey, role.Model,
		[]llm.Message{{Role: "system", Content: prompt}, {Role: "user", Content: input}},
		0.15, inlineCompletionTokens, nil, nil)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			http.Error(w, "compleció cancel·lada o massa lenta", http.StatusGatewayTimeout)
			return
		}
		http.Error(w, "el model no ha pogut completar el codi", http.StatusBadGateway)
		return
	}
	if strings.TrimSpace(reply) == "" {
		reply = ""
	} else if fenced := strings.TrimSpace(reply); strings.HasPrefix(fenced, "```") {
		if newline := strings.IndexByte(fenced, '\n'); newline >= 0 {
			fenced = fenced[newline+1:]
		}
		if end := strings.LastIndex(fenced, "```"); end >= 0 {
			fenced = fenced[:end]
		}
		reply = strings.TrimRight(fenced, "\r\n")
	}
	if runes := []rune(reply); len(runes) > 4000 {
		reply = string(runes[:4000])
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]string{"completion": reply})
}
