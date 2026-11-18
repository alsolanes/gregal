package web

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"

	"gregal/internal/backlog"
	"gregal/internal/goal"
	"gregal/internal/identity"
	"gregal/internal/profile"
)

// goalDirFor retorna on es desen els objectius (al costat del config).
func goalDirFor(cfgPath string) string {
	if cfgPath != "" {
		if dir := filepath.Dir(cfgPath); dir != "" && dir != "." {
			return dir
		}
	}
	if base, err := os.UserConfigDir(); err == nil {
		return filepath.Join(base, profile.DirName())
	}
	return "."
}

// userStateDir és el directori d'estat personal d'un usuari: vents,
// rotllanes, objectius. Al costat del config, dins users/.identity-v1/<hash>/. Sense
// usuari no s'ha de cridar mai: l'estat compartit és goalDirFor(cfgPath).
func userStateDir(cfgPath, user string) string {
	// Never claim an old slug automatically: even an ASCII lowercase name
	// could have belonged to another exact identity. Leave old data intact
	// for an administrator to verify and migrate explicitly.
	return filepath.Join(goalDirFor(cfgPath), "users", ".identity-v1", identity.Key(user))
}

// projectName és el nom del projecte actual.
func (s *Server) projectName() string {
	return projectName(s.cwd)
}

// El mode objectiu desa tasques al backlog (goal/backlog.go). Aquí: la
// targeta del xat, la migració dels objectius vells i /api/goal com a vista
// de compatibilitat per a les apps que encara el llegeixen.

// migrats recorda quins directoris d'objectius ja s'han passat al backlog
// en aquest procés: la migració només mira el disc una vegada.
var (
	migratsMu sync.Mutex
	migrats   = map[string]bool{}
)

func (s *Server) migraObjectius() {
	dir := s.goalDir
	migratsMu.Lock()
	fet := migrats[dir]
	migrats[dir] = true
	migratsMu.Unlock()
	if fet || dir == "" {
		return
	}
	if n, err := goal.MigrateToBacklog(dir); n > 0 || err != nil {
		log.Printf("backlog: %d objectius migrats de %s (err=%v)", n, dir, err)
	}
}

// emitGoal desa al backlog la tasca del bloc ```goal de la resposta, n'hi
// deixa el número a sota (perquè el model la pugui revisar amb id:) i
// n'envia la targeta a la UI.
func (s *Server) emitGoal(emit func(string, any), reply string) {
	it, created, err := goal.Record(reply, s.sessionCwd())
	if err != nil {
		return
	}
	note := goal.Note(it, created)
	s.mu.Lock()
	goal.AnnotateLast(s.convo, note)
	s.lastGoalID = strconv.Itoa(it.ID)
	s.mu.Unlock()
	emit("backlog_item", map[string]any{"item": it, "created": created, "note": note})
}

// handleGoal és la vista vella d'objectius damunt del backlog: GET llista
// les tasques obertes amb la forma d'un objectiu, «task» les comença i
// «delete» les descarta.
func (s *Server) handleGoal(w http.ResponseWriter, r *http.Request) {
	s.migraObjectius()
	cwd := s.sessionCwd()
	if r.Method == http.MethodGet {
		open, err := goal.Open(cwd)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		out := make([]map[string]any, 0, len(open))
		for _, it := range open {
			out = append(out, map[string]any{
				"id": strconv.Itoa(it.ID), "title": it.Title, "body": goal.Card(it), "status": it.Status, "project": projectName(backlog.Root(cwd)),
			})
		}
		s.mu.Lock()
		últim := s.lastGoalID
		s.mu.Unlock()
		writeJSON(w, map[string]any{"goals": out, "last": últim, "project": s.projectName()})
		return
	}
	var req struct {
		Action string `json:"action"` // task|delete
		ID     string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "json invàlid", 400)
		return
	}
	it, err := goal.Resolve(cwd, req.ID)
	if err != nil {
		http.Error(w, err.Error(), 404)
		return
	}
	switch req.Action {
	case "task":
		it, task, err := goal.Start(cwd, it.ID, backlogWho(r))
		if err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		writeJSON(w, map[string]string{"id": strconv.Itoa(it.ID), "title": it.Title, "task": task})
	case "delete":
		st, err := backlog.Open(cwd)
		if err == nil {
			_, err = st.Move(it.ID, backlog.StatusDropped, "removed from the goals list", backlogWho(r))
		}
		if err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		writeJSON(w, map[string]string{"deleted": strconv.Itoa(it.ID)})
	default:
		http.Error(w, "acció desconeguda (task|delete)", 400)
	}
}
