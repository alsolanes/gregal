package web

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"gregal/internal/tools"
)

// G2 — workspaces. Fins ara el directori de treball es fixava a l'arrencada
// amb os.Getwd() i no es podia canviar sense reiniciar el servidor. Ara és
// per sessió: dues pestanyes poden treballar en dos repositoris alhora.
//
// La llista de workspaces recents es desa a
// ~/.local/share/gregal/workspaces.json perquè l'escriptori pugui oferir
// "obre projecte" sense tornar a navegar el disc cada vegada.

type workspaceEntry struct {
	Path string    `json:"path"`
	Name string    `json:"name"`
	Last time.Time `json:"last"`
}

var wsMu sync.Mutex

func workspacesPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "workspaces.json"
	}
	return filepath.Join(home, ".local", "share", "gregal", "workspaces.json")
}

func loadWorkspaces() []workspaceEntry {
	raw, err := os.ReadFile(workspacesPath())
	if err != nil {
		return nil
	}
	var out []workspaceEntry
	if json.Unmarshal(raw, &out) != nil {
		return nil
	}
	return out
}

// rememberWorkspace puja el directori al capdamunt dels recents (màxim 20).
func rememberWorkspace(dir string) {
	wsMu.Lock()
	defer wsMu.Unlock()
	list := loadWorkspaces()
	out := make([]workspaceEntry, 0, len(list)+1)
	out = append(out, workspaceEntry{Path: dir, Name: filepath.Base(dir), Last: time.Now()})
	for _, e := range list {
		if e.Path != dir {
			out = append(out, e)
		}
	}
	if len(out) > 20 {
		out = out[:20]
	}
	p := workspacesPath()
	if os.MkdirAll(filepath.Dir(p), 0o700) != nil {
		return
	}
	b, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return
	}
	tmp := p + ".tmp"
	if os.WriteFile(tmp, b, 0o600) == nil {
		_ = os.Rename(tmp, p)
	}
}

// resolveWorkspace normalitza i valida un directori de treball.
func resolveWorkspace(dir string) (string, error) {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return "", errors.New("cal un directori")
	}
	if strings.HasPrefix(dir, "~") {
		if home, err := os.UserHomeDir(); err == nil {
			dir = filepath.Join(home, strings.TrimPrefix(dir, "~"))
		}
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	st, err := os.Stat(abs)
	if err != nil {
		return "", errors.New("no existeix: " + abs)
	}
	if !st.IsDir() {
		return "", errors.New("no és un directori: " + abs)
	}
	return abs, nil
}

// setWorkspace canvia el directori d'aquesta sessió. Els permisos de
// l'agent el segueixen: ProjectDir és el que decideix si write/edit són
// automàtics o demanen permís.
func (s *Server) setWorkspace(dir string) error {
	abs, err := resolveWorkspace(dir)
	if err != nil {
		return err
	}
	// Ningú no planta la sessió fora de la seva carpeta: des d'aquí en
	// pengen l'arbre, els fitxers, els diffs i les ordres del terminal.
	if err := s.user.Allow(abs); err != nil {
		return err
	}
	queued := s.queueHasActiveTurn()
	s.mu.Lock()
	if abs != s.cwd && (queued || s.agentBusy || s.planRunning) {
		s.mu.Unlock()
		return errors.New("stop the current run before changing its working folder, or open a new project")
	}
	s.cwd = abs
	if s.policy != nil {
		s.policy.ProjectDir = abs
	}
	s.mu.Unlock()
	rememberWorkspace(abs)
	return nil
}

// handleWorkspaces llista els recents (GET) o canvia el de la sessió (POST).
func (s *Server) handleWorkspaces(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		var req struct {
			Path string `json:"path"`
		}
		if json.NewDecoder(r.Body).Decode(&req) != nil {
			http.Error(w, "json invàlid", 400)
			return
		}
		if err := s.setWorkspace(req.Path); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
	}
	s.mu.Lock()
	cur := s.cwd
	s.mu.Unlock()
	wsMu.Lock()
	list := loadWorkspaces()
	wsMu.Unlock()
	seen := map[string]bool{}
	out := make([]workspaceEntry, 0, len(list)+1)
	add := func(e workspaceEntry) {
		if e.Path == "" || seen[e.Path] {
			return
		}
		seen[e.Path] = true
		out = append(out, e)
	}
	add(workspaceEntry{Path: cur, Name: filepath.Base(cur), Last: time.Now()})
	for _, e := range list {
		add(e)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Last.After(out[j].Last) })
	writeJSON(w, map[string]any{
		"current": cur, "project": filepath.Base(cur),
		"branch": tools.GitBranch(cur), "workspaces": out,
	})
}

// rewindFilter dona el filtre de rewind d'aquesta sessió: nil (desfés-ho
// tot, com sempre) mentre només hi hagi una sessió viva, i el filtre per
// workspace quan n'hi ha més d'una. Així el comportament d'un sol client
// no canvia gens i el multi-sessió no es trepitja.
func (s *Server) rewindFilter() func(string) bool {
	if s.hub == nil {
		return nil
	}
	s.hub.mu.RLock()
	n := len(s.hub.sess)
	s.hub.mu.RUnlock()
	if n <= 1 {
		return nil
	}
	return s.inWorkspace
}

// inWorkspace diu si path cau dins del workspace de la sessió. El journal
// de canvis és de tot el procés: amb diverses sessions obertes, el rewind
// s'ha de limitar als fitxers del workspace propi o una pestanya desfaria
// la feina de l'altra.
func (s *Server) inWorkspace(path string) bool {
	s.mu.Lock()
	root := s.cwd
	s.mu.Unlock()
	if root == "" {
		return true
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return true
	}
	rel, err := filepath.Rel(root, abs)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
