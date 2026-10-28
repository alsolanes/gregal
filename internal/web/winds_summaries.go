package web

import (
	"net/http"
	"strings"
)

// Resums per a la llista de converses dels Vents: l'últim missatge i quan,
// per cada vent, cada rotllana i el Ventet. Abans el mòbil carregava
// l'historial sencer de totes les converses per pintar la portada; ara
// carrega aquests resums i cada fil només quan s'obre.
//
//	GET /api/winds/summaries → {items:[{key, from, speaker, text, ts, count}]}
//
// key és "wind:<id>", "rot:<id>" o "cap". ts en segons (0 = sense hora).

type convoSummary struct {
	Key     string `json:"key"`
	From    string `json:"from,omitempty"`
	Speaker string `json:"speaker,omitempty"`
	Text    string `json:"text,omitempty"`
	Ts      int64  `json:"ts,omitempty"`
	Count   int    `json:"count"`
}

func summaryOf(key string, history []map[string]any) convoSummary {
	out := convoSummary{Key: key, Count: len(history)}
	for i := len(history) - 1; i >= 0; i-- {
		m := history[i]
		text, _ := m["text"].(string)
		if strings.TrimSpace(text) == "" {
			continue
		}
		out.Text = oneLine(text, 160)
		out.From, _ = m["from"].(string)
		out.Speaker, _ = m["speaker_name"].(string)
		switch ts := m["ts"].(type) {
		case int64:
			out.Ts = ts
		case int:
			out.Ts = int64(ts)
		case float64:
			out.Ts = int64(ts)
		}
		break
	}
	return out
}

func (s *Server) handleWindsSummaries(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "GET required", http.StatusMethodNotAllowed)
		return
	}
	items := []convoSummary{}
	if winds, err := s.winds.List(); err == nil {
		for _, wd := range winds {
			h := windHistory(s, wd.ID)
			h.mu.Lock()
			msgs := petHistoryJSON(h.messages)
			h.mu.Unlock()
			items = append(items, summaryOf("wind:"+wd.ID, msgs))
		}
	}
	if rots, err := s.listRotllanes(); err == nil {
		for _, rot := range rots {
			h := rotllanaHistory(s, rot.ID)
			h.mu.Lock()
			msgs := s.rotllanaHistoryJSON(h.messages)
			h.mu.Unlock()
			items = append(items, summaryOf("rot:"+rot.ID, msgs))
		}
	}
	capMu.Lock()
	st := s.loadCap()
	capMu.Unlock()
	ventet := convoSummary{Key: "cap", Count: len(st.History)}
	for i := len(st.History) - 1; i >= 0; i-- {
		if m := st.History[i]; strings.TrimSpace(m.Text) != "" {
			ventet.From, ventet.Text, ventet.Ts = m.From, oneLine(m.Text, 160), m.Ts
			break
		}
	}
	items = append(items, ventet)
	writeJSON(w, map[string]any{"items": items})
}
