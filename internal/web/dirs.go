package web

// /api/dirs: navegador de carpetes per triar el projecte des de la
// finestra sense escriure rutes a mà. Torna els subdirectoris d'una
// carpeta (i si són repositoris git), el pare, la carpeta d'usuari i,
// a Windows, les unitats. Només llegeix noms: no obre cap fitxer.

import (
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
)

type dirEntry struct {
	Name string `json:"name"`
	Path string `json:"path"`
	Git  bool   `json:"git,omitempty"`
}

// maxDirEntries evita que una carpeta amb milers de subcarpetes (node_modules)
// ofegui el panell.
const maxDirEntries = 400

func (s *Server) handleDirs(w http.ResponseWriter, r *http.Request) {
	home, _ := os.UserHomeDir()
	path := strings.TrimSpace(r.URL.Query().Get("path"))
	if path == "" {
		s.mu.Lock()
		path = s.cwd
		s.mu.Unlock()
	}
	if path == "" || path == "~" {
		path = home
	}
	if strings.HasPrefix(path, "~"+string(filepath.Separator)) || strings.HasPrefix(path, "~/") {
		path = filepath.Join(home, path[2:])
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	// El navegador de carpetes només ensenya el que és de l'usuari.
	if s.guardPath(w, abs) {
		return
	}
	st, err := os.Stat(abs)
	if err != nil || !st.IsDir() {
		http.Error(w, "no és un directori: "+abs, 400)
		return
	}
	entries, err := os.ReadDir(abs)
	if err != nil {
		http.Error(w, "no es pot llegir: "+err.Error(), 400)
		return
	}
	dirs := make([]dirEntry, 0, 64)
	amagats := 0
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		n := e.Name()
		if strings.HasPrefix(n, ".") || strings.HasPrefix(n, "$") || n == "node_modules" || n == "__pycache__" {
			amagats++
			continue
		}
		p := filepath.Join(abs, n)
		_, gerr := os.Stat(filepath.Join(p, ".git"))
		dirs = append(dirs, dirEntry{Name: n, Path: p, Git: gerr == nil})
		if len(dirs) >= maxDirEntries {
			break
		}
	}
	sort.Slice(dirs, func(i, j int) bool {
		// Repositoris primer: és el que es ve a buscar.
		if dirs[i].Git != dirs[j].Git {
			return dirs[i].Git
		}
		return strings.ToLower(dirs[i].Name) < strings.ToLower(dirs[j].Name)
	})
	parent := filepath.Dir(abs)
	if parent == abs {
		parent = ""
	}
	_, gerr := os.Stat(filepath.Join(abs, ".git"))
	writeJSON(w, map[string]any{
		"path":    abs,
		"name":    filepath.Base(abs),
		"git":     gerr == nil,
		"parent":  parent,
		"home":    home,
		"roots":   arrels(),
		"dirs":    dirs,
		"hidden":  amagats,
		"truncat": len(dirs) >= maxDirEntries,
	})
}

// arrels torna els punts d'entrada del disc: a Windows les unitats amb
// lletra que existeixen; a la resta, "/".
func arrels() []string {
	if runtime.GOOS != "windows" {
		return []string{"/"}
	}
	return unitatsVives()
}

// provaArrel diu si una unitat hi és. esperaArrel és el que s'espera, com a
// molt, que contestin totes: un disc local respon en mil·lisegons, i una
// unitat lenta que es quedi fora surt a la llista següent (no es desa res).
// Variables perquè els tests les canviïn.
var (
	provaArrel = func(d string) bool {
		_, err := os.Stat(d)
		return err == nil
	}
	esperaArrel = 300 * time.Millisecond
)

// unitatsVives prova totes les lletres alhora i no espera la que no contesta:
// una unitat de xarxa desconnectada triga segons a fallar, i provades d'una
// en una el selector de carpetes s'hi quedava aturat cada cop que s'obria.
func unitatsVives() []string {
	const lletres = 'Z' - 'C' + 1
	type resposta struct {
		unitat string
		viva   bool
	}
	ch := make(chan resposta, lletres) // amb lloc per a totes: la que arriba tard no es queda penjada
	prova := provaArrel                // llegida aquí: les proves que acaben tard no miren la variable
	for c := 'C'; c <= 'Z'; c++ {
		d := string(c) + `:\`
		go func() { ch <- resposta{d, prova(d)} }()
	}
	var out []string
	limit := time.After(esperaArrel)
espera:
	for i := 0; i < lletres; i++ {
		select {
		case r := <-ch:
			if r.viva {
				out = append(out, r.unitat)
			}
		case <-limit:
			break espera
		}
	}
	sort.Strings(out)
	return out
}
