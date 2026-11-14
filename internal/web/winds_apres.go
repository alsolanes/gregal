package web

import (
	"net/http"
	"strings"
	"time"

	"gregal/internal/companion"
	"gregal/internal/llm"
)

// Aprèn les teves preferències amb el temps (com dots i Duet), i ho diu.
// La memòria per vent ja s'omplia sola (startWindMemoryLearn), però en
// silenci i amagada al perfil de cada vent. Ara:
//
//   - quan aprèn alguna cosa d'un torn, el fil ho diu amb un missatge
//     «memory» («📝 Ho recordaré: …») que la persona pot desfer;
//   - «Què sap de tu» (GET/POST /api/winds/apres) ensenya en un sol lloc el
//     que sap la memòria personal i la de cada vent, i ho deixa oblidar.
//
// Els missatges «memory» no van mai al model (windChatTurns els salta).

const memoryKind = "memory"

// startWindMemoryLearnInThread és startWindMemoryLearn i, si aprèn res nou,
// ho deixa dit al fil. Agafa el pany dels torns del vent: un torn que
// comencés mentre s'afegeix l'avís no es desaria (commitWindTurn descarta
// les escriptures amb una generació vella).
func (s *Server) startWindMemoryLearnInThread(w companion.Wind, question, answer string, hist *windHistoryEntry) {
	revision := companion.WindMemoryRevision(s.windStateDir(), w.ID)
	goBackground(func() {
		before := map[string]bool{}
		for _, n := range companion.WindMemoryNotes(s.windStateDir(), w.ID) {
			before[n] = true
		}
		s.windMemoryLearnRevision(w, question, answer, revision)
		var learned []string
		for _, n := range companion.WindMemoryNotes(s.windStateDir(), w.ID) {
			if !before[n] {
				learned = append(learned, n)
			}
		}
		if len(learned) == 0 || hist == nil {
			return
		}
		turn := windDirectTurn(s, w.ID)
		turn.Lock()
		defer turn.Unlock()
		_ = s.appendWindThread(w, hist, llm.Message{Role: "assistant", Kind: memoryKind, Content: strings.Join(learned, "\n"),
			SpeakerID: w.ID, SpeakerName: w.Name, Ts: time.Now().Unix()})
	})
}

type apresWind struct {
	ID    string   `json:"id"`
	Name  string   `json:"name"`
	Notes []string `json:"notes"`
}

// handleWindApres: GET → {personal:[…], winds:[{id,name,notes}]};
// POST {wind?, text} oblida aquell fet (wind buit = memòria personal).
func (s *Server) handleWindApres(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		out := []apresWind{}
		list, _ := s.winds.List()
		for _, wd := range list {
			if notes := companion.WindMemoryNotes(s.windStateDir(), wd.ID); len(notes) > 0 {
				out = append(out, apresWind{ID: wd.ID, Name: wd.Name, Notes: notes})
			}
		}
		personal := s.personalNotes()
		if personal == nil {
			personal = []string{}
		}
		writeJSON(w, map[string]any{"personal": personal, "winds": out})
	case http.MethodPost:
		var req struct {
			Wind string   `json:"wind"`
			Text string   `json:"text"`
			All  []string `json:"all"`
		}
		if !decodeSmall(w, r, &req) {
			return
		}
		texts := req.All
		if t := strings.TrimSpace(req.Text); t != "" {
			texts = append(texts, t)
		}
		if len(texts) == 0 {
			http.Error(w, "text", http.StatusBadRequest)
			return
		}
		forgot := 0
		if req.Wind == "" {
			for _, t := range texts {
				n, err := s.forgetPersonal(t)
				if err != nil {
					http.Error(w, "could not forget", http.StatusInternalServerError)
					return
				}
				forgot += n
			}
		} else {
			if _, ok := s.winds.ByID(req.Wind); !ok {
				http.Error(w, "unknown wind", http.StatusBadRequest)
				return
			}
			drop := map[string]bool{}
			for _, t := range texts {
				drop[strings.TrimSpace(t)] = true
			}
			windLibraryMu.Lock()
			var keep []string
			for _, n := range companion.WindMemoryNotes(s.windStateDir(), req.Wind) {
				if drop[strings.TrimSpace(n)] {
					forgot++
					continue
				}
				keep = append(keep, "- "+n)
			}
			err := companion.ReplaceWindMemory(s.windStateDir(), req.Wind, strings.Join(keep, "\n"))
			windLibraryMu.Unlock()
			if err != nil {
				http.Error(w, "could not forget", http.StatusInternalServerError)
				return
			}
		}
		writeJSON(w, map[string]any{"ok": true, "forgot": forgot})
	default:
		http.Error(w, "GET or POST", http.StatusMethodNotAllowed)
	}
}
