package web

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"gregal/internal/tools"
)

// G3 — API de fitxers. Fins ara l'única manera de veure un fitxer des de
// la web era demanar-li a l'agent que el llegís (i gastar-hi un torn).
// Amb /api/tree, /api/file i /api/diff l'escriptori pot pintar l'arbre del
// projecte, obrir un fitxer i revisar els canvis sense passar pel model.

// skipDirs són els directoris que no val la pena mostrar mai.
var skipDirs = map[string]bool{
	".git": true, "node_modules": true, "dist": true, "target": true,
	"vendor": true, ".venv": true, "__pycache__": true, ".next": true,
	"build": true, ".gradle": true, ".idea": true,
}

const (
	maxTreeEntries = 4000
	maxFileBytes   = 2 << 20 // 2 MB: prou per a codi, prou poc per al navegador
)

type treeNode struct {
	Name     string     `json:"name"`
	Path     string     `json:"path"` // relatiu al workspace, amb /
	Dir      bool       `json:"dir"`
	Size     int64      `json:"size,omitempty"`
	Status   string     `json:"status,omitempty"` // git: M, A, D, ??
	Children []treeNode `json:"children,omitempty"`
}

// safeJoin resol rel dins del workspace i rebutja qualsevol sortida (..,
// enllaços simbòlics cap enfora, rutes absolutes).
func (s *Server) safeJoin(rel string) (string, error) {
	s.mu.Lock()
	root := s.cwd
	s.mu.Unlock()
	rel = strings.TrimSpace(rel)
	rel = strings.TrimPrefix(filepath.ToSlash(rel), "./")
	if rel == "" || rel == "." {
		return root, nil
	}
	if tools.IsRooted(rel) || strings.HasPrefix(rel, "~") {
		return "", errors.New("només rutes relatives al projecte")
	}
	abs := filepath.Join(root, filepath.FromSlash(rel))
	r, err := filepath.Rel(root, abs)
	if err != nil || r == ".." || strings.HasPrefix(r, ".."+string(filepath.Separator)) {
		return "", errors.New("fora del projecte")
	}
	// La comprovació lèxica anterior no detecta un enllaç simbòlic que
	// apuntés fora del workspace (p. ex. `docs/secrets -> C:\\Users\\...`).
	// Resol també el primer pare existent per cobrir fitxers nous dins d'un
	// directori enllaçat. Això és important perquè safeJoin és el guard únic
	// que comparteixen /api/tree, /api/file i el descart de fitxers nous.
	if !pathWithinReal(root, abs) {
		return "", errors.New("fora del projecte")
	}
	return abs, nil
}

// pathWithinReal comprova contenció després de resoldre symlinks. realPath
// també resol el pare existent quan target encara no existeix, de manera que
// un write futur no pot aprofitar un directori enllaçat per escapar.
func pathWithinReal(root, target string) bool {
	rr := realPath(root)
	rt := realPath(target)
	rel, err := filepath.Rel(rr, rt)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

// resolveFilePath resol una ruta que pot ser del workspace (relativa),
// del home (~) o absoluta, i després comprova que l'usuari hi pugui anar.
// És el pas obligat de l'arbre i dels fitxers: el guard és aquí i no a
// cada handler perquè no se n'oblidi cap.
func (s *Server) resolveFilePath(p string) (string, error) {
	abs, err := s.resolveFilePathRaw(p)
	if err != nil {
		return "", err
	}
	if err := s.user.Allow(abs); err != nil {
		return "", err
	}
	return abs, nil
}

// resolveFilePathRaw fa la resolució sense mirar permisos: rutes del
// workspace, del home (~) o absolutes del disc.
func (s *Server) resolveFilePathRaw(p string) (string, error) {
	p = strings.TrimSpace(p)
	if p == "" || p == "." {
		s.mu.Lock()
		root := s.cwd
		s.mu.Unlock()
		return root, nil
	}
	if strings.HasPrefix(p, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		cleanHome := strings.TrimPrefix(p[1:], "/")
		cleanHome = strings.TrimPrefix(cleanHome, "\\")
		return filepath.Clean(filepath.Join(home, filepath.FromSlash(cleanHome))), nil
	}
	if tools.IsRooted(p) || filepath.IsAbs(p) {
		return filepath.Clean(filepath.FromSlash(p)), nil
	}
	return s.safeJoin(p)
}

// gitStatuses mapeja ruta relativa → estat curt de git (M, A, ??…).
func gitStatuses(dir string) map[string]string {
	out := map[string]string{}
	raw := tools.GitStatusShort(dir)
	for _, line := range strings.Split(raw, "\n") {
		if len(line) < 4 {
			continue
		}
		code := strings.TrimSpace(line[:2])
		p := strings.TrimSpace(line[3:])
		if i := strings.Index(p, " -> "); i >= 0 {
			p = p[i+4:]
		}
		out[strings.Trim(p, `"`)] = code
	}
	return out
}

// handleTree llista un directori del workspace (un nivell, o recursiu amb
// depth). Sempre relatiu al workspace de la sessió, o una carpeta externa si s'indica.
func (s *Server) handleTree(w http.ResponseWriter, r *http.Request) {
	rel := r.URL.Query().Get("path")
	depth, _ := strconv.Atoi(r.URL.Query().Get("depth"))
	if depth < 1 {
		depth = 1
	}
	if depth > 6 {
		depth = 6
	}
	base, err := s.resolveFilePath(rel)
	if err != nil {
		// Fora de la seva carpeta és 403, no un error de sintaxi.
		if errors.Is(err, ErrForaArrel) {
			http.Error(w, err.Error(), http.StatusForbidden)
			return
		}
		http.Error(w, err.Error(), 400)
		return
	}
	s.mu.Lock()
	root := s.cwd
	s.mu.Unlock()
	st := gitStatuses(base)
	n := 0
	nodes := walkTree(root, base, depth, st, &n)
	displayPath := rel
	if displayPath == "" {
		displayPath = filepath.ToSlash(strings.TrimPrefix(strings.TrimPrefix(base, root), string(filepath.Separator)))
	}
	writeJSON(w, map[string]any{
		"path": filepath.ToSlash(displayPath),
		"root": root, "project": filepath.Base(base), "entries": nodes,
		"truncated": n >= maxTreeEntries,
	})
}

func walkTree(root, dir string, depth int, st map[string]string, n *int) []treeNode {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	out := make([]treeNode, 0, len(ents))
	for _, e := range ents {
		if *n >= maxTreeEntries {
			break
		}
		name := e.Name()
		if skipDirs[name] {
			continue
		}
		abs := filepath.Join(dir, name)
		rel, _ := filepath.Rel(root, abs)
		rel = filepath.ToSlash(rel)
		node := treeNode{Name: name, Path: rel, Dir: e.IsDir(), Status: st[rel]}
		if !e.IsDir() {
			if info, err := e.Info(); err == nil {
				node.Size = info.Size()
			}
		} else if depth > 1 {
			node.Children = walkTree(root, abs, depth-1, st, n)
		}
		*n++
		out = append(out, node)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Dir != out[j].Dir {
			return out[i].Dir
		}
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	return out
}

// handleFile retorna el contingut d'un fitxer del workspace (només lectura).
func (s *Server) handleFile(w http.ResponseWriter, r *http.Request) {
	reqPath := r.URL.Query().Get("path")
	abs, err := s.resolveFilePath(reqPath)
	if err != nil {
		if errors.Is(err, ErrForaArrel) {
			http.Error(w, err.Error(), http.StatusForbidden)
			return
		}
		http.Error(w, err.Error(), 400)
		return
	}
	info, err := os.Stat(abs)
	if err != nil {
		http.Error(w, "no existeix", 404)
		return
	}
	if info.IsDir() {
		http.Error(w, "és un directori (fes servir /api/tree)", 400)
		return
	}
	if info.Size() > maxFileBytes {
		writeJSON(w, map[string]any{"path": reqPath, "size": info.Size(), "too_big": true})
		return
	}
	raw, err := os.ReadFile(abs)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	if !utf8.Valid(raw) || strings.IndexByte(string(raw), 0) >= 0 {
		writeJSON(w, map[string]any{"path": reqPath, "size": info.Size(), "binary": true})
		return
	}
	writeJSON(w, map[string]any{
		"path": filepath.ToSlash(reqPath), "size": info.Size(),
		"content": string(raw), "lines": strings.Count(string(raw), "\n") + 1,
	})
}

// handleDiff retorna els canvis del workspace en JSON (fitxers → hunks).
func (s *Server) handleDiff(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	root := s.cwd
	s.mu.Unlock()
	files, err := tools.GitDiffStructured(root)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	only := strings.TrimSpace(r.URL.Query().Get("path"))
	if only != "" {
		var keep []tools.DiffFile
		for _, f := range files {
			if f.Path == only {
				keep = append(keep, f)
			}
		}
		files = keep
	}
	add, del := 0, 0
	for _, f := range files {
		add += f.Added
		del += f.Removed
	}
	writeJSON(w, map[string]any{
		"files": files, "added": add, "removed": del,
		"branch": tools.GitBranch(root), "project": filepath.Base(root),
	})
}

// handleDiffDiscard descarta un canvi: un hunk concret (path+hunk), un
// fitxer sencer (path) o tot (res). És el "Descarta" de la pestanya Canvis.
func (s *Server) handleDiffDiscard(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "només POST", 405)
		return
	}
	var req struct {
		Path string `json:"path"`
		Hunk *int   `json:"hunk"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil {
		http.Error(w, "json invàlid", 400)
		return
	}
	s.mu.Lock()
	root := s.cwd
	s.mu.Unlock()
	files, err := tools.GitDiffStructured(root)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	var target *tools.DiffFile
	for i := range files {
		if files[i].Path == req.Path {
			target = &files[i]
			break
		}
	}
	if target == nil {
		http.Error(w, "aquest fitxer no té canvis", 404)
		return
	}
	// Un fitxer nou no es pot revertir amb git apply: s'esborra.
	if target.Untracked && (req.Hunk == nil || len(target.Hunks) <= 1) {
		abs, err := s.safeJoin(target.Path)
		if err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		if err := os.Remove(abs); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		writeJSON(w, map[string]any{"ok": true, "summary": "esborrat " + target.Path})
		return
	}
	patches := []string{}
	if req.Hunk != nil {
		i := *req.Hunk
		if i < 0 || i >= len(target.Hunks) {
			http.Error(w, "hunk inexistent", 400)
			return
		}
		patches = append(patches, target.Hunks[i].Patch)
	} else {
		// Fitxer sencer: de l'últim hunk al primer, per no moure les línies.
		for i := len(target.Hunks) - 1; i >= 0; i-- {
			patches = append(patches, target.Hunks[i].Patch)
		}
	}
	for _, p := range patches {
		if err := tools.GitApplyReverse(root, p); err != nil {
			http.Error(w, err.Error(), 409)
			return
		}
	}
	summary := "descartat " + target.Path
	if req.Hunk != nil {
		summary += " (hunk " + strconv.Itoa(*req.Hunk+1) + ")"
	}
	writeJSON(w, map[string]any{"ok": true, "summary": summary})
}
