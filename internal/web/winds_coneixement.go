package web

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gregal/internal/companion"
	"gregal/internal/llm"
	"gregal/internal/skills"
)

// Coneixements d'un vent (estil skill): al prompt només hi van les
// descripcions; el cos arriba quan el vent crida l'eina `skill`.
// Cada vent té la seva carpeta dins l'estat de l'usuari:
//
//	wind-skills/<ventID>/<nom>.md
//
//	---
//	nom: cuina-italiana
//	titol: Cuina italiana
//	descripcio: Receptes i tècniques de la cuina italiana.
//	---
//	<cos en markdown>
//
// Markdown a propòsit: l'usuari ho pot llegir i editar a mà, al mòbil
// (perfil del vent) o a l'escriptori, amb els mateixos fitxers.

// windSkillsDir és la carpeta de coneixements d'un vent.
func (s *Server) windSkillsDir(windID string) string {
	return filepath.Join(s.windStateDir(), "wind-skills", windID)
}

// windSkills torna els coneixements del vent ordenats per nom.
func (s *Server) windSkills(windID string) []skills.Skill {
	cat := skills.DeDir(s.windSkillsDir(windID))
	sort.Slice(cat, func(i, j int) bool { return cat[i].Nom < cat[j].Nom })
	return cat
}

// windSkillsSystem és l'índex que va al prompt del vent: només noms i
// descripcions. El cos només arriba via l'eina `skill`.
func (s *Server) windSkillsSystem(windID string) string {
	idx := skills.Index(s.windSkills(windID))
	if idx == "" {
		return ""
	}
	return "\n\n" + idx
}

// windMemorySystem és el que el vent recorda de l'usuari.
func (s *Server) windMemorySystem(windID string) string {
	mem := companion.LoadWindMemory(s.windStateDir(), windID)
	if strings.TrimSpace(mem) == "" {
		return ""
	}
	return "\n\nWHAT YOU REMEMBER ABOUT THE USER (tastes, preferences, durable facts — use it, never repeat it back verbatim):\n" + mem
}

// windSkillFiles llista els fitxers reals (per esborrar amb seguretat).
func windSkillFiles(dir string) []string {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range ents {
		if !e.IsDir() && strings.HasSuffix(strings.ToLower(e.Name()), ".md") {
			out = append(out, e.Name())
		}
	}
	return out
}

// saveWindSkill desa un coneixement (nom normalitzat, escriptura atòmica).
func (s *Server) saveWindSkill(windID string, sk skills.Skill) error {
	if windID == "" || sk.Nom == "" || sk.Descripcio == "" || sk.Cos == "" {
		return errBadSkill
	}
	dir := s.windSkillsDir(windID)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString("nom: " + sk.Nom + "\n")
	if sk.Titol != "" {
		b.WriteString("titol: " + sk.Titol + "\n")
	}
	b.WriteString("descripcio: " + sk.Descripcio + "\n")
	b.WriteString("---\n" + strings.TrimSpace(sk.Cos) + "\n")
	tmp, err := os.CreateTemp(dir, ".skill-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.WriteString(b.String()); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), filepath.Join(dir, sk.Nom+".md"))
}

var errBadSkill = skillError("coneixement incomplet: cal nom, descripció i cos")

type skillError string

func (e skillError) Error() string { return string(e) }

// startWindMemoryLearn extreu en segon pla els fets duradors SOBRE
// L'USUARI (gustos, preferències, fets de converses). Ho fa sol, sense
// demanar permís; l'usuari ho veu i ho pot editar al perfil.
func (s *Server) startWindMemoryLearn(w companion.Wind, question, answer string) {
	revision := companion.WindMemoryRevision(s.windStateDir(), w.ID)
	go s.windMemoryLearnRevision(w, question, answer, revision)
}

func (s *Server) windMemoryLearnRevision(w companion.Wind, question, answer string, revision uint64) {
	defer func() { _ = recover() }() // la memòria no pot tombar el xat
	p, role, ok := s.sideRoleRef()
	if !ok {
		return
	}
	s.mu.Lock()
	cfg, client := s.cfg, s.client
	s.mu.Unlock()
	var b strings.Builder
	b.WriteString("You track what " + w.Name + " knows ABOUT THE USER (tastes, preferences, durable personal facts — never topic facts, never chit-chat).\n")
	b.WriteString("From the exchange below, extract DURABLE facts about the user. Reply ONLY with bullet lines starting with '- ', one fact per line, max 3 lines, in the user's language.\n")
	b.WriteString("If nothing is durable about the user, reply with nothing.\n")
	if known := companion.WindMemoryNotes(s.windStateDir(), w.ID); len(known) > 0 {
		b.WriteString("\nAlready known (do NOT repeat these):\n")
		for i, k := range known {
			if i >= 20 {
				break
			}
			b.WriteString("- " + k + "\n")
		}
	}
	b.WriteString("\nExchange:\nUser: " + question + "\n" + w.Name + ": " + answer)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	out, err := windLearnLoop(s, ctx, client, cfg, p, role, []llm.Message{{Role: "user", Content: b.String()}})
	if err != nil {
		return
	}
	var facts []string
	for _, l := range strings.Split(out, "\n") {
		if l = strings.TrimSpace(l); strings.HasPrefix(l, "- ") && len(facts) < 3 {
			facts = append(facts, strings.TrimPrefix(l, "- "))
		}
	}
	if len(facts) == 0 {
		return
	}
	_ = companion.AppendWindMemoryIfRevision(s.windStateDir(), w.ID, revision, facts)
}

// --- Endpoints (mòbil, escriptori i edició a mà comparteixen fitxers) ---

func (s *Server) handleWindMemoria(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		id := strings.TrimSpace(r.URL.Query().Get("wind"))
		if _, ok := s.winds.ByID(id); !ok {
			http.Error(w, "unknown wind", http.StatusBadRequest)
			return
		}
		writeJSON(w, map[string]any{"wind": id, "memoria": companion.LoadWindMemory(s.windStateDir(), id)})
	case http.MethodPut:
		var req struct {
			Wind    string `json:"wind"`
			Memoria string `json:"memoria"`
		}
		if !decodeSmall(w, r, &req) {
			return
		}
		if _, ok := s.winds.ByID(strings.TrimSpace(req.Wind)); !ok {
			http.Error(w, "unknown wind", http.StatusBadRequest)
			return
		}
		if err := companion.ReplaceWindMemory(s.windStateDir(), strings.TrimSpace(req.Wind), req.Memoria); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, map[string]any{"ok": true})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleWindConeixements(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		id := strings.TrimSpace(r.URL.Query().Get("wind"))
		if _, ok := s.winds.ByID(id); !ok {
			http.Error(w, "unknown wind", http.StatusBadRequest)
			return
		}
		type resum struct {
			Nom        string `json:"nom"`
			Titol      string `json:"titol"`
			Descripcio string `json:"descripcio"`
		}
		var out []resum
		for _, sk := range s.windSkills(id) {
			out = append(out, resum{sk.Nom, sk.Titol, sk.Descripcio})
		}
		writeJSON(w, map[string]any{"wind": id, "coneixements": out})
	case http.MethodPut:
		var req struct {
			Wind       string `json:"wind"`
			Nom        string `json:"nom"`
			Titol      string `json:"titol"`
			Descripcio string `json:"descripcio"`
			Cos        string `json:"cos"`
		}
		if !decodeSmall(w, r, &req) {
			return
		}
		id := strings.TrimSpace(req.Wind)
		if _, ok := s.winds.ByID(id); !ok {
			http.Error(w, "unknown wind", http.StatusBadRequest)
			return
		}
		sk := skills.Skill{
			Nom:        skills.Normalitza(req.Nom),
			Titol:      strings.TrimSpace(req.Titol),
			Descripcio: strings.TrimSpace(req.Descripcio),
			Cos:        strings.TrimSpace(req.Cos),
		}
		if err := s.saveWindSkill(id, sk); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, map[string]any{"ok": true, "nom": sk.Nom})
	case http.MethodDelete:
		var req struct {
			Wind string `json:"wind"`
			Nom  string `json:"nom"`
		}
		if !decodeSmall(w, r, &req) {
			return
		}
		id := strings.TrimSpace(req.Wind)
		if _, ok := s.winds.ByID(id); !ok {
			http.Error(w, "unknown wind", http.StatusBadRequest)
			return
		}
		nom := skills.Normalitza(req.Nom)
		if nom == "" || strings.ContainsAny(nom, "/\\") {
			http.Error(w, "bad nom", http.StatusBadRequest)
			return
		}
		if err := os.Remove(filepath.Join(s.windSkillsDir(id), nom+".md")); err != nil {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		writeJSON(w, map[string]any{"ok": true})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleWindConeixement(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id := strings.TrimSpace(r.URL.Query().Get("wind"))
	nom := skills.Normalitza(r.URL.Query().Get("nom"))
	if _, ok := s.winds.ByID(id); !ok || nom == "" {
		http.Error(w, "unknown wind o nom", http.StatusBadRequest)
		return
	}
	for _, sk := range s.windSkills(id) {
		if sk.Nom == nom {
			writeJSON(w, map[string]any{"wind": id, "nom": sk.Nom, "titol": sk.Titol, "descripcio": sk.Descripcio, "cos": sk.Cos})
			return
		}
	}
	http.Error(w, "not found", http.StatusNotFound)
}

// handleWindsResum serveix tots els vents amb rol i índex de coneixements:
// un sol cop per pintar el selector de membres (nom + de què és expert).
func (s *Server) handleWindsResum(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	winds, _ := s.winds.List()
	out := make([]map[string]any, 0, len(winds))
	for _, wind := range winds {
		breus := make([]map[string]string, 0)
		for _, c := range s.windSkills(wind.ID) {
			breus = append(breus, map[string]string{"nom": c.Nom, "titol": c.Titol, "descripcio": c.Descripcio})
		}
		out = append(out, map[string]any{"id": wind.ID, "nom": wind.Name, "rol": wind.Role, "coneixements": breus})
	}
	writeJSON(w, map[string]any{"vents": out})
}
