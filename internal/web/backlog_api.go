package web

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"gregal/internal/agent"
	"gregal/internal/backlog"
)

// /api/backlog: la llista de feina del projecte de la sessió (vegeu
// internal/backlog). Les regles (què pot estar ready, què demana motiu,
// què demana proves) són al paquet; aquí només es tradueix la petició.

type backlogReq struct {
	Action   string `json:"action"` // create|edit|move|check|log|to_repo|work
	ID       int    `json:"id"`
	N        int    `json:"n"`
	Done     *bool  `json:"done"`
	Status   string `json:"status"`
	Reason   string `json:"reason"`
	Evidence string `json:"evidence"`
	Text     string `json:"text"`
	Session  string `json:"session"`
	// Dry, amb work, només comprova que es pot començar: la UI ho pregunta
	// abans d'obrir el worktree, perquè no en quedi cap d'orfe.
	Dry        bool     `json:"dry"`
	Type       *string  `json:"type"`
	Title      *string  `json:"title"`
	Why        *string  `json:"why"`
	OutOfScope *string  `json:"out_of_scope"`
	Priority   *string  `json:"priority"`
	Size       *string  `json:"size"`
	Parent     *int     `json:"parent"`
	DependsOn  []int    `json:"depends_on"`
	Criteria   []string `json:"criteria"`
}

func backlogWho(r *http.Request) string {
	if u := userFrom(r); u != nil && strings.TrimSpace(u.Name) != "" {
		return u.Name
	}
	// Sense comptes, qui fa servir la UI és la persona de la màquina: el log
	// ho deixa en blanc i la UI hi posa «tu».
	return ""
}

func (q backlogReq) aplica(it *backlog.Item) {
	set := func(dst *string, src *string) {
		if src != nil {
			*dst = *src
		}
	}
	set(&it.Type, q.Type)
	set(&it.Title, q.Title)
	set(&it.Why, q.Why)
	set(&it.OutOfScope, q.OutOfScope)
	set(&it.Priority, q.Priority)
	set(&it.Size, q.Size)
	if q.Parent != nil {
		it.Parent = *q.Parent
	}
	if q.DependsOn != nil {
		it.DependsOn = q.DependsOn
	}
	if q.Criteria != nil {
		prev := map[string]backlog.Criterion{}
		for _, c := range it.Criteria {
			prev[c.Text] = c
		}
		it.Criteria = nil
		for _, t := range q.Criteria {
			c, ok := prev[strings.TrimSpace(t)]
			if !ok {
				c = backlog.Criterion{Text: t}
			}
			it.Criteria = append(it.Criteria, c)
		}
	}
}

func (s *Server) handleBacklog(w http.ResponseWriter, r *http.Request) {
	st, err := backlog.Open(s.sessionCwd())
	if err != nil {
		writeJSONStatus(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	switch r.Method {
	case http.MethodGet:
		s.backlogList(w, st)
	case http.MethodPost:
		var q backlogReq
		if err := json.NewDecoder(r.Body).Decode(&q); err != nil {
			writeJSONStatus(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
			return
		}
		out, err := s.backlogAction(st, q, backlogWho(r))
		if err != nil {
			code := http.StatusBadRequest
			if errors.Is(err, backlog.ErrNotFound) {
				code = http.StatusNotFound
			}
			writeJSONStatus(w, code, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, out)
	default:
		http.Error(w, "GET o POST", http.StatusMethodNotAllowed)
	}
}

func (s *Server) backlogList(w http.ResponseWriter, st *backlog.Store) {
	items, err := st.List()
	if err != nil {
		writeJSONStatus(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if items == nil {
		items = []backlog.Item{}
	}
	writeJSON(w, map[string]any{
		"root": st.Root, "project": projectName(st.Root), "storage": st.Storage, "items": items,
		"types": backlog.Types, "statuses": backlog.Statuses, "priorities": backlog.Priorities, "sizes": backlog.Sizes,
	})
}

// backlogAction fa una acció i torna el cos de la resposta.
func (s *Server) backlogAction(st *backlog.Store, q backlogReq, who string) (any, error) {
	var it backlog.Item
	var err error
	switch q.Action {
	case "create":
		nou := backlog.Item{}
		q.aplica(&nou)
		it, err = st.Create(nou, who)
	case "edit":
		it, err = st.Edit(q.ID, who, q.aplica)
	case "move":
		it, err = st.Move(q.ID, q.Status, q.Reason, who)
	case "check":
		done := q.Done == nil || *q.Done
		it, err = st.Check(q.ID, q.N, done, q.Evidence, who)
	case "log":
		it, err = st.AddLog(q.ID, q.Text, who)
	case "to_repo":
		if err := st.MoveToRepo(); err != nil {
			return nil, err
		}
		return map[string]string{"storage": st.Storage, "path": st.Path}, nil
	case "work":
		return s.backlogWork(st, q, who)
	case "review":
		// Torna a passar el revisor (per exemple si l'anterior va fallar o
		// es va quedar a mitges). La resposta no l'espera: la UI ho segueix.
		it, err = agent.StartBacklogReview(s.client, s.cfg, st.Root, q.ID)
	default:
		return nil, errors.New("unknown action: create, edit, move, check, log, to_repo, work or review")
	}
	if err != nil {
		return nil, err
	}
	return map[string]any{"item": it}, nil
}

// backlogWork comença la tasca des de la sessió que la demana. La UI ja ha
// obert la sessió (en un worktree si el projecte és un repo de git) i crida
// des d'allà; amb dry només comprova que es pot començar, abans d'obrir el
// worktree, perquè no en quedi cap d'orfe.
func (s *Server) backlogWork(st *backlog.Store, q backlogReq, who string) (any, error) {
	it, err := st.Get(q.ID)
	if err != nil {
		return nil, err
	}
	if q.Dry {
		if it.Status != backlog.StatusInProgress {
			all, _ := st.List()
			if err := backlog.CheckMove(it, backlog.StatusInProgress, "", all); err != nil {
				return nil, err
			}
		}
		return map[string]any{"item": it, "branch": backlog.Slug(it)}, nil
	}
	if it.Status != backlog.StatusInProgress {
		if _, err := st.Move(q.ID, backlog.StatusInProgress, "", who); err != nil {
			return nil, err
		}
	}
	cwd := s.sessionCwd()
	base := ""
	if it.Base == "" {
		base = agent.GitHead(cwd)
	}
	if _, err := st.SetWork(q.ID, cwd, base); err != nil {
		return nil, err
	}
	on := "session " + strings.TrimSpace(q.Session)
	if cwd != st.Root {
		on += " in " + cwd
	}
	if it, err = st.AddLog(q.ID, "work started: "+on, who); err != nil {
		return nil, err
	}
	return map[string]any{"item": it, "task": backlog.Task(it)}, nil
}
