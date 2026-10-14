package web

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"gregal/internal/config"
	"gregal/internal/knowledge"
)

// API de coneixement (pàgina «Knowledge»): col·leccions per abast, fonts,
// notes, reindexació amb progrés, cerca de prova i les memòries que ja
// existien (memòria d'usuari, notes del projecte, AGENTS.md) per veure-les
// i editar-les. Les rutes són de sessió: el projecte i la sessió són els
// de la pestanya que pregunta.

func registerKnowledgeRoutes(h *Hub, handle func(string, http.HandlerFunc)) {
	handle("/api/knowledge", h.route((*Server).handleKnowledge))
	handle("/api/knowledge/search", h.route((*Server).handleKnowledgeSearch))
	handle("/api/knowledge/memory", h.route((*Server).handleKnowledgeMemory))
	handle("/api/knowledge/{id}", h.route((*Server).handleKnowledgeItem))
	handle("/api/knowledge/{id}/sources", h.route((*Server).handleKnowledgeSources))
	handle("/api/knowledge/{id}/reindex", h.route((*Server).handleKnowledgeReindex))
	handle("/api/knowledge/{id}/notes", h.route((*Server).handleKnowledgeNotes))
}

func (s *Server) knowledgeWhere() knowledge.Where {
	s.mu.Lock()
	defer s.mu.Unlock()
	return knowledge.Where{Project: s.cwd, Session: s.id}
}

// knowledgeView és una col·lecció amb el que la UI necessita al costat.
type knowledgeView struct {
	knowledge.Collection
	Stats    knowledge.Stats     `json:"stats"`
	Progress knowledge.Progress  `json:"progress"`
	Paths    map[string]string   `json:"source_paths,omitempty"` // id → ruta absoluta
	Missing  []string            `json:"missing_sources,omitempty"`
	Docs     []knowledge.DocInfo `json:"docs,omitempty"`
	Error    string              `json:"index_error,omitempty"`
	Notes    []knowledge.Note    `json:"notes"`
	Sources  []knowledge.Source  `json:"sources"`
}

func viewOf(w knowledge.Where, c knowledge.Collection, withDocs bool) knowledgeView {
	v := knowledgeView{Collection: c, Progress: knowledge.ProgressOf(c.ID), Paths: map[string]string{}, Notes: c.Notes, Sources: c.Sources}
	if v.Notes == nil {
		v.Notes = []knowledge.Note{}
	}
	if v.Sources == nil {
		v.Sources = []knowledge.Source{}
	}
	for _, src := range c.Sources {
		p := knowledge.SourcePath(c, src, w)
		v.Paths[src.ID] = p
		if _, err := os.Stat(p); err != nil {
			v.Missing = append(v.Missing, src.ID)
		}
	}
	ix, err := knowledge.LoadIndex(w, c)
	if err != nil {
		v.Error = err.Error()
	}
	if ix != nil {
		v.Stats = ix.Stats()
		if withDocs {
			v.Docs = ix.Docs
		}
	}
	return v
}

// knowledgeAllowed: en un servidor amb usuaris, les col·leccions globals
// i la memòria d'usuari són de tots; només l'administrador les toca.
func knowledgeAllowed(r *http.Request, scope string) bool {
	if scope != knowledge.ScopeGlobal {
		return true
	}
	u := userFrom(r)
	return u == nil || u.Tothom || u.Admin
}

func knowledgeError(w http.ResponseWriter, err error) {
	code := http.StatusBadRequest
	switch {
	case errors.Is(err, knowledge.ErrNotFound):
		code = http.StatusNotFound
	case errors.Is(err, knowledge.ErrDuplicate), errors.Is(err, knowledge.ErrBusy):
		code = http.StatusConflict
	}
	writeJSONStatus(w, code, map[string]string{"error": err.Error()})
}

func decodeKnowledge(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(v); err != nil {
		writeJSONStatus(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return false
	}
	return true
}

// handleKnowledge: GET llista; POST crea {name, description, scope,
// path?, include?, exclude?} (amb path, hi afegeix la font i indexa).
func (s *Server) handleKnowledge(w http.ResponseWriter, r *http.Request) {
	where := s.knowledgeWhere()
	switch r.Method {
	case http.MethodGet:
		list, err := knowledge.List(where)
		if err != nil {
			knowledgeError(w, err)
			return
		}
		views := make([]knowledgeView, 0, len(list))
		for _, c := range list {
			views = append(views, viewOf(where, c, false))
		}
		set := knowledge.Current()
		writeJSON(w, map[string]any{
			"collections": views,
			"project":     where.Project,
			"session":     where.Session,
			"embeddings":  set.EmbeddingsOn(),
			"embed_model": set.Embed.Model,
			"limits":      map[string]int{"max_files": set.MaxFiles, "max_mb": set.MaxTotalMB, "max_file_mb": set.MaxFileMB},
			"can_global":  knowledgeAllowed(r, knowledge.ScopeGlobal),
		})
	case http.MethodPost:
		var req struct {
			Name        string   `json:"name"`
			Description string   `json:"description"`
			Scope       string   `json:"scope"`
			Path        string   `json:"path"`
			Include     []string `json:"include"`
			Exclude     []string `json:"exclude"`
		}
		if !decodeKnowledge(w, r, &req) {
			return
		}
		if req.Scope == "" {
			req.Scope = knowledge.ScopeProject
		}
		if !knowledgeAllowed(r, req.Scope) {
			writeJSONStatus(w, http.StatusForbidden, map[string]string{"error": "only the administrator can manage global knowledge"})
			return
		}
		if strings.TrimSpace(req.Path) != "" && s.guardKnowledgePath(w, where, req.Path) {
			return
		}
		c, err := knowledge.Create(where, req.Name, req.Description, req.Scope)
		if err != nil {
			knowledgeError(w, err)
			return
		}
		if strings.TrimSpace(req.Path) != "" {
			if _, err := knowledge.AddSource(where, c.ID, req.Path, req.Include, req.Exclude); err != nil {
				_ = knowledge.Delete(where, c.ID)
				knowledgeError(w, err)
				return
			}
			_ = knowledge.StartReindex(where, c.ID, false)
			if got, err := knowledge.Get(where, c.ID); err == nil {
				c = got
			}
		}
		writeJSON(w, viewOf(where, *c, false))
	default:
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// guardKnowledgePath no deixa enllaçar carpetes de fora de les arrels de
// l'usuari (servidor amb usuaris).
func (s *Server) guardKnowledgePath(w http.ResponseWriter, where knowledge.Where, path string) bool {
	p := strings.TrimSpace(path)
	if !filepath.IsAbs(p) && where.Project != "" && !strings.HasPrefix(p, "~") {
		p = filepath.Join(where.Project, p)
	}
	return s.guardPath(w, p)
}

func (s *Server) knowledgeCollection(w http.ResponseWriter, r *http.Request, write bool) (knowledge.Where, *knowledge.Collection, bool) {
	where := s.knowledgeWhere()
	c, err := knowledge.Get(where, r.PathValue("id"))
	if err != nil {
		knowledgeError(w, err)
		return where, nil, false
	}
	if write && !knowledgeAllowed(r, c.Scope) {
		writeJSONStatus(w, http.StatusForbidden, map[string]string{"error": "only the administrator can manage global knowledge"})
		return where, nil, false
	}
	return where, c, true
}

// handleKnowledgeItem: GET (amb documents), PATCH {name?, description?,
// enabled?}, DELETE.
func (s *Server) handleKnowledgeItem(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		where, c, ok := s.knowledgeCollection(w, r, false)
		if ok {
			writeJSON(w, viewOf(where, *c, true))
		}
	case http.MethodPatch:
		where, c, ok := s.knowledgeCollection(w, r, true)
		if !ok {
			return
		}
		var req struct {
			Name        *string `json:"name"`
			Description *string `json:"description"`
			Enabled     *bool   `json:"enabled"`
		}
		if !decodeKnowledge(w, r, &req) {
			return
		}
		got, err := knowledge.Update(where, c.ID, func(c *knowledge.Collection) error {
			if req.Name != nil {
				c.Name = *req.Name
			}
			if req.Description != nil {
				c.Description = strings.TrimSpace(*req.Description)
			}
			if req.Enabled != nil {
				c.Enabled = *req.Enabled
			}
			return nil
		})
		if err != nil {
			knowledgeError(w, err)
			return
		}
		writeJSON(w, viewOf(where, *got, false))
	case http.MethodDelete:
		where, c, ok := s.knowledgeCollection(w, r, true)
		if !ok {
			return
		}
		if knowledge.ProgressOf(c.ID).Running {
			knowledgeError(w, knowledge.ErrBusy)
			return
		}
		if err := knowledge.Delete(where, c.ID); err != nil {
			knowledgeError(w, err)
			return
		}
		writeJSON(w, map[string]bool{"ok": true})
	default:
		w.Header().Set("Allow", "GET, PATCH, DELETE")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleKnowledgeSources: POST {path, include?, exclude?}; DELETE {id}.
// Tots dos tornen a indexar en segon pla.
func (s *Server) handleKnowledgeSources(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodDelete {
		w.Header().Set("Allow", "POST, DELETE")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	where, c, ok := s.knowledgeCollection(w, r, true)
	if !ok {
		return
	}
	var req struct {
		ID      string   `json:"id"`
		Path    string   `json:"path"`
		Include []string `json:"include"`
		Exclude []string `json:"exclude"`
	}
	if !decodeKnowledge(w, r, &req) {
		return
	}
	if r.Method == http.MethodPost {
		if s.guardKnowledgePath(w, where, req.Path) {
			return
		}
		if _, err := knowledge.AddSource(where, c.ID, req.Path, req.Include, req.Exclude); err != nil {
			knowledgeError(w, err)
			return
		}
	} else if err := knowledge.RemoveSource(where, c.ID, req.ID); err != nil {
		knowledgeError(w, err)
		return
	}
	_ = knowledge.StartReindex(where, c.ID, false)
	got, err := knowledge.Get(where, c.ID)
	if err != nil {
		knowledgeError(w, err)
		return
	}
	writeJSON(w, viewOf(where, *got, false))
}

// handleKnowledgeReindex: POST {force?, wait?}. Sense wait torna de
// seguida amb el progrés; la UI el va consultant amb GET /api/knowledge/{id}.
func (s *Server) handleKnowledgeReindex(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	where, c, ok := s.knowledgeCollection(w, r, true)
	if !ok {
		return
	}
	var req struct {
		Force bool `json:"force"`
		Wait  bool `json:"wait"`
	}
	if r.ContentLength != 0 && !decodeKnowledge(w, r, &req) {
		return
	}
	if req.Wait {
		if _, err := knowledge.Reindex(r.Context(), where, c.ID, req.Force); err != nil {
			knowledgeError(w, err)
			return
		}
	} else if err := knowledge.StartReindex(where, c.ID, req.Force); err != nil {
		knowledgeError(w, err)
		return
	}
	writeJSON(w, viewOf(where, *c, false))
}

// handleKnowledgeNotes: POST {title?, text}; DELETE {id}.
func (s *Server) handleKnowledgeNotes(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodDelete {
		w.Header().Set("Allow", "POST, DELETE")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	where, c, ok := s.knowledgeCollection(w, r, true)
	if !ok {
		return
	}
	var req struct {
		ID    string `json:"id"`
		Title string `json:"title"`
		Text  string `json:"text"`
	}
	if !decodeKnowledge(w, r, &req) {
		return
	}
	var err error
	if r.Method == http.MethodPost {
		author := "user"
		if u := userFrom(r); u != nil && u.Name != "" && !u.Tothom {
			author = u.Name
		}
		_, err = knowledge.AddNote(where, c.ID, req.Title, req.Text, author)
	} else {
		err = knowledge.RemoveNote(where, c.ID, req.ID)
	}
	if err != nil {
		knowledgeError(w, err)
		return
	}
	got, err := knowledge.Get(where, c.ID)
	if err != nil {
		knowledgeError(w, err)
		return
	}
	writeJSON(w, viewOf(where, *got, false))
}

// handleKnowledgeSearch: GET ?q=&collection=&k= o POST {query, collection, k}.
func (s *Server) handleKnowledgeSearch(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Query      string `json:"query"`
		Collection string `json:"collection"`
		K          int    `json:"k"`
	}
	switch r.Method {
	case http.MethodGet:
		q := r.URL.Query()
		req.Query, req.Collection = q.Get("q"), q.Get("collection")
		req.K, _ = strconv.Atoi(q.Get("k"))
	case http.MethodPost:
		if !decodeKnowledge(w, r, &req) {
			return
		}
	default:
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if req.K <= 0 {
		req.K = 8
	}
	hits, err := knowledge.Search(r.Context(), s.knowledgeWhere(), req.Query, knowledge.SearchOpts{Collection: req.Collection, K: req.K})
	if err != nil {
		knowledgeError(w, err)
		return
	}
	if hits == nil {
		hits = []knowledge.Hit{}
	}
	writeJSON(w, map[string]any{"query": req.Query, "hits": hits})
}

// ---- memòries que ja existien ----

type memoryView struct {
	Kind     string `json:"kind"`
	Path     string `json:"path"`
	Text     string `json:"text"`
	Hash     string `json:"hash"`
	Exists   bool   `json:"exists"`
	Editable bool   `json:"editable"`
}

func memoryHash(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:8])
}

func readMemory(kind, path string, editable bool) memoryView {
	v := memoryView{Kind: kind, Path: path, Editable: editable}
	if path == "" {
		v.Editable = false
		return v
	}
	if raw, err := os.ReadFile(path); err == nil {
		v.Text, v.Exists = string(raw), true
	}
	v.Hash = memoryHash(v.Text)
	return v
}

func projectRulesFile(project string) string {
	if project == "" {
		return ""
	}
	for _, name := range []string{"AGENTS.md", "CLAUDE.md"} {
		p := filepath.Join(project, name)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

// handleKnowledgeMemory: GET les tres memòries; PUT {kind: user|project,
// text, hash}. El hash és el del text que es va llegir: si el fitxer ha
// canviat entremig (/remember, /nota, l'agent), 409 amb el text actual en
// comptes de trepitjar-lo.
func (s *Server) handleKnowledgeMemory(w http.ResponseWriter, r *http.Request) {
	where := s.knowledgeWhere()
	canUser := knowledgeAllowed(r, knowledge.ScopeGlobal)
	views := func() []memoryView {
		return []memoryView{
			readMemory("user", config.UserMemoryPath(), canUser),
			readMemory("project", config.ProjectMemoryPath(where.Project), where.Project != ""),
			readMemory("rules", projectRulesFile(where.Project), false),
		}
	}
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, map[string]any{"memories": views(), "project": where.Project})
	case http.MethodPut, http.MethodPost:
		var req struct {
			Kind string `json:"kind"`
			Text string `json:"text"`
			Hash string `json:"hash"`
		}
		if !decodeKnowledge(w, r, &req) {
			return
		}
		var path string
		switch req.Kind {
		case "user":
			if !canUser {
				writeJSONStatus(w, http.StatusForbidden, map[string]string{"error": "only the administrator can edit the user memory on a shared server"})
				return
			}
			path = config.UserMemoryPath()
		case "project":
			path = config.ProjectMemoryPath(where.Project)
			if path == "" {
				writeJSONStatus(w, http.StatusBadRequest, map[string]string{"error": "this session has no project folder"})
				return
			}
			if s.guardPath(w, where.Project) {
				return
			}
		default:
			writeJSONStatus(w, http.StatusBadRequest, map[string]string{"error": "kind must be user or project"})
			return
		}
		knowledgeMemMu.Lock()
		defer knowledgeMemMu.Unlock()
		cur := readMemory(req.Kind, path, true)
		if req.Hash != cur.Hash {
			writeJSONStatus(w, http.StatusConflict, map[string]any{"error": "the memory changed since you opened it", "current": cur})
			return
		}
		text := strings.ReplaceAll(req.Text, "\r\n", "\n")
		if strings.TrimSpace(text) != "" && !strings.HasSuffix(text, "\n") {
			text += "\n"
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			knowledgeError(w, err)
			return
		}
		if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
			knowledgeError(w, err)
			return
		}
		writeJSON(w, map[string]any{"memories": views(), "project": where.Project})
	default:
		w.Header().Set("Allow", "GET, PUT")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// knowledgeMemMu serialitza les escriptures de memòria (comprova i escriu).
var knowledgeMemMu sync.Mutex
