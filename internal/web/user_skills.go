package web

import (
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"gregal/internal/skills"
)

// Skills pròpies de cada usuari: es llegeixen a users/<nom>/skills/ i es
// poden actualitzar des de l'app o des dels vents (quan aprenen alguna
// cosa que val la pena recordar, la desen aquí com una skill).
//
//	GET    /api/skills           → {skills:[{nom,titol,descripcio}]} (només l'índex de l'usuari)
//	GET    /api/skills?nom=x     → {nom,titol,descripcio,cos}
//	POST   /api/skills           → crea/actualitza {nom,titol,descripcio,cos}
//	DELETE /api/skills           → {nom}
//
// Les skills globals (integrades, de la persona, del projecte) no es toquen
// des d'aquí: això és el bloc de notes personal de cada usuari.

// slugSkill converteix un títol qualsevol en nom de fitxer de skill:
// minúscules, accents resolts, espais en guions.
func slugSkill(nom string) string {
	n := strings.ToLower(strings.TrimSpace(strings.TrimSuffix(nom, ".md")))
	repl := map[rune]rune{'à': 'a', 'á': 'a', 'ä': 'a', 'ç': 'c', 'è': 'e', 'é': 'e', 'ë': 'e', 'í': 'i', 'ï': 'i', 'ò': 'o', 'ó': 'o', 'ö': 'o', 'ú': 'u', 'ù': 'u', 'ü': 'u', 'ñ': 'n'}
	var b strings.Builder
	for _, r := range n {
		if v, ok := repl[r]; ok {
			r = v
		}
		switch {
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			b.WriteRune(r)
		case r == ' ' || r == '_' || r == '-':
			b.WriteRune('-')
		}
	}
	return strings.Trim(b.String(), "-")
}

// skillFileName valida i normalitza el nom d'una skill d'usuari.
func skillFileName(nom string) (string, bool) {
	n := slugSkill(nom)
	if n == "" || len(n) > 64 {
		return "", false
	}
	return n + ".md", true
}

func (s *Server) userSkillsDir() string {
	if s.user == nil || s.user.Tothom {
		return ""
	}
	return filepath.Join(userStateDir(s.cfgPath, s.user.Name), "skills")
}

var userSkillsMu sync.Mutex

func (s *Server) handleUserSkills(w http.ResponseWriter, r *http.Request) {
	dir := s.userSkillsDir()
	if dir == "" {
		http.Error(w, "aquest servidor no té usuaris: les skills globals ja hi són", http.StatusBadRequest)
		return
	}
	switch r.Method {
	case http.MethodGet:
		nom := strings.TrimSpace(r.URL.Query().Get("nom"))
		if nom != "" {
			file, ok := skillFileName(nom)
			if !ok {
				http.Error(w, "nom invàlid", http.StatusBadRequest)
				return
			}
			raw, err := os.ReadFile(filepath.Join(dir, file))
			if err != nil {
				http.Error(w, "skill inexistent", http.StatusNotFound)
				return
			}
			sk, _ := skills.Parse(string(raw), "usuari")
			writeJSON(w, map[string]any{"nom": sk.Nom, "titol": sk.Titol, "descripcio": sk.Descripcio, "cos": sk.Cos})
			return
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			if os.IsNotExist(err) {
				writeJSON(w, map[string]any{"skills": []map[string]any{}})
				return
			}
			http.Error(w, err.Error(), 500)
			return
		}
		out := make([]map[string]string, 0, len(entries))
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
				continue
			}
			raw, err := os.ReadFile(filepath.Join(dir, e.Name()))
			if err != nil {
				continue
			}
			if sk, ok := skills.Parse(string(raw), "usuari"); ok {
				out = append(out, map[string]string{"nom": sk.Nom, "titol": sk.Titol, "descripcio": sk.Descripcio})
			}
		}
		sort.Slice(out, func(i, j int) bool { return out[i]["nom"] < out[j]["nom"] })
		writeJSON(w, map[string]any{"skills": out})
	case http.MethodPost:
		var req struct {
			Nom        string `json:"nom"`
			Titol      string `json:"titol"`
			Descripcio string `json:"descripcio"`
			Cos        string `json:"cos"`
		}
		if !decodeSmall(w, r, &req) {
			return
		}
		file, ok := skillFileName(req.Nom)
		if !ok {
			http.Error(w, "nom invàlid: minúscules, xifres i guions", http.StatusBadRequest)
			return
		}
		if strings.TrimSpace(req.Cos) == "" {
			http.Error(w, "cos buit", http.StatusBadRequest)
			return
		}

		var b strings.Builder
		slug := slugSkill(req.Nom)
		// Parse exigeix descripció: si no ve, en posem una del títol.
		if strings.TrimSpace(req.Descripcio) == "" {
			req.Descripcio = strings.TrimSpace(req.Titol)
			if req.Descripcio == "" {
				req.Descripcio = "Skill de l'usuari " + slug
			}
		}
		b.WriteString("---\n")
		b.WriteString("nom: " + slug + "\n")
		titol := strings.TrimSpace(req.Titol)
		if titol == "" {
			titol = slug
		}
		b.WriteString("titol: " + titol + "\n")
		if d := strings.TrimSpace(req.Descripcio); d != "" {
			b.WriteString("descripcio: " + d + "\n")
		}
		b.WriteString("---\n\n")
		b.WriteString(strings.ReplaceAll(strings.TrimSpace(req.Cos), "\r\n", "\n"))
		b.WriteString("\n")
		userSkillsMu.Lock()
		defer userSkillsMu.Unlock()
		if err := os.MkdirAll(dir, 0o755); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		if err := os.WriteFile(filepath.Join(dir, file), []byte(b.String()), 0o644); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		writeJSON(w, map[string]bool{"ok": true})
	case http.MethodDelete:
		var req struct {
			Nom string `json:"nom"`
		}
		if !decodeSmall(w, r, &req) {
			return
		}
		file, ok := skillFileName(req.Nom)
		if !ok {
			http.Error(w, "nom invàlid", http.StatusBadRequest)
			return
		}
		if err := os.Remove(filepath.Join(dir, file)); err != nil {
			http.Error(w, "skill inexistent", http.StatusNotFound)
			return
		}
		writeJSON(w, map[string]bool{"ok": true})
	default:
		http.Error(w, "GET, POST or DELETE required", http.StatusMethodNotAllowed)
	}
}
