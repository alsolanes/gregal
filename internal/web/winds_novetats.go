package web

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"gregal/internal/companion"
)

// Novetats dels vents per a l'app: quan un vent parla sol (cor proactiu,
// recordatori), el text entra al seu xat i a més queda apuntat aquí amb
// l'hora. L'app en mode servidor ho consulta de tant en tant
// (GET /api/winds/novetats?des_de=<unix>) i avisa amb notificació local
// si hi ha veus noves. Sense Telegram ni canals externs: tot dins l'app.
//
// El registre viu a winds-novetats.json del dir de sessions (tallat a 100).

const (
	novetatsFile = "winds-novetats.json"
	maxNovetats  = 100
	maxNovetatsOut = 50
)

// windNovetat és una veu proactiva d'un vent: qui, d'on ve i què ha dit.
type windNovetat struct {
	Ts     int64  `json:"ts"`
	Wind   string `json:"wind"`
	Name   string `json:"name"`
	Source string `json:"source"`
	Text   string `json:"text"`
}

var novetatsMu sync.Mutex

func novetatsPath(s *Server) string { return filepath.Join(s.sessDir(), novetatsFile) }

func loadNovetats(s *Server) []windNovetat {
	raw, err := os.ReadFile(novetatsPath(s))
	if err != nil {
		return nil
	}
	var list []windNovetat
	if err := json.Unmarshal(raw, &list); err != nil {
		return nil
	}
	return list
}

func saveNovetats(s *Server, list []windNovetat) {
	if len(list) > maxNovetats {
		list = append([]windNovetat(nil), list[len(list)-maxNovetats:]...)
	}
	raw, err := json.Marshal(list)
	if err != nil {
		return
	}
	// El dir pot no existir (tests, primera veu): best-effort igualment.
	_ = os.MkdirAll(filepath.Dir(novetatsPath(s)), 0o700)
	_ = os.WriteFile(novetatsPath(s), raw, 0o600)
}

// recordNovetat apunta una veu proactiva. Best-effort: mai no falla el torn.
func recordNovetat(s *Server, w companion.Wind, source, text string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	novetatsMu.Lock()
	defer novetatsMu.Unlock()
	list := append(loadNovetats(s), windNovetat{
		Ts:     time.Now().Unix(),
		Wind:   w.ID,
		Name:   w.Name,
		Source: source,
		Text:   text,
	})
	saveNovetats(s, list)
}

// handleWindNovetats torna les veus proactives posteriors a des_de (unix,
// 0 = totes les guardades), de vella a nova. Lleuger: l'app ho crida en
// segon pla sense descarregar historials.
func (s *Server) handleWindNovetats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "només GET", http.StatusMethodNotAllowed)
		return
	}
	var desDe int64
	if raw := strings.TrimSpace(r.URL.Query().Get("des_de")); raw != "" {
		desDe, _ = strconv.ParseInt(raw, 10, 64)
	}
	novetatsMu.Lock()
	all := loadNovetats(s)
	novetatsMu.Unlock()
	out := make([]windNovetat, 0, len(all))
	for _, n := range all {
		if n.Ts > desDe {
			out = append(out, n)
		}
	}
	if len(out) > maxNovetatsOut {
		out = append([]windNovetat(nil), out[len(out)-maxNovetatsOut:]...)
	}
	writeJSON(w, map[string]any{"novetats": out})
}
