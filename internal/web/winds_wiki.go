package web

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gregal/internal/agent"
	"gregal/internal/companion"
	"gregal/internal/llm"
	"gregal/internal/session"
)

// Wiki de l'espai vents: pàgines de resum que qualsevol vent (o l'usuari)
// pot anar guardant de les converses, amb gràfics i imatges inclosos, per
// tenir «el tema en una pàgina» des del mòbil o l'escriptori.
//
//	GET    /api/winds/wiki              → {pages:[{nom,titol,descripcio,updated}]}
//	POST   /api/winds/wiki              → desa {nom,titol,descripcio,cos}
//	GET    /api/winds/wiki/{nom}        → {page:{nom,titol,descripcio,cos,updated}}
//	DELETE /api/winds/wiki/{nom}        → {ok:true}
//	POST   /api/winds/wiki/summary      → {wind|rotllana,titol?} resumeix l'última
//	                                       conversa i la desa com a pàgina nova
//
// Emmagatzematge: fitxers markdown editables a mà, el mateix patró que els
// coneixements del vent:
//
//	wind-wiki/<nom>.md
//
//	---
//	titol: Obres del Camp Nou
//	descripcio: Estat de les obres i dates clau.
//	updated: 2026-10-25T10:30:00Z
//	---
//	<cos en markdown>

// wikiPage és una pàgina de la wiki dels vents.
type wikiPage struct {
	Nom         string `json:"nom"`
	Titol       string `json:"titol"`
	Descripcio string `json:"descripcio"`
	Cos         string `json:"cos,omitempty"`
	Updated     string `json:"updated,omitempty"`
}

func (s *Server) windWikiDir() string {
	return filepath.Join(s.windStateDir(), "wind-wiki")
}

func validWikiName(name string) bool {
	if strings.TrimSpace(name) == "" || len(name) > 160 || name == "." || name == ".." || strings.ContainsAny(name, "/\\<>:\"|?*\x00\r\n") {
		return false
	}
	for _, r := range name {
		if r < 32 {
			return false
		}
	}
	return true
}

// parseWikiPage llegeix el frontmatter d'una pàgina.
func parseWikiPage(nom, raw string) wikiPage {
	p := wikiPage{Nom: nom}
	cos := raw
	if strings.HasPrefix(raw, "---") {
		if _, after, ok := strings.Cut(raw, "\n---"); ok {
			head := strings.TrimPrefix(raw, "---")
			head = strings.TrimSuffix(head, "\n")
			cos = strings.TrimLeft(after, "-\n")
			for _, ln := range strings.Split(head, "\n") {
				ln = strings.TrimSpace(ln)
				if v, okf := strings.CutPrefix(ln, "titol:"); okf {
					p.Titol = strings.TrimSpace(v)
				} else if v, okf := strings.CutPrefix(ln, "descripcio:"); okf {
					p.Descripcio = strings.TrimSpace(v)
				} else if v, okf := strings.CutPrefix(ln, "updated:"); okf {
					p.Updated = strings.TrimSpace(v)
				}
			}
		}
	}
	p.Cos = strings.TrimSpace(cos)
	return p
}

func (s *Server) wikiList() []wikiPage {
	dir := s.windWikiDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var pages []wikiPage
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		nom := strings.TrimSuffix(e.Name(), ".md")
		p := parseWikiPage(nom, string(raw))
		p.Cos = "" // la llista no porta el cos
		pages = append(pages, p)
	}
	sort.Slice(pages, func(i, j int) bool { return pages[i].Nom < pages[j].Nom })
	return pages
}

// saveWikiPage desa (o reescriu) una pàgina.
func (s *Server) saveWikiPage(p wikiPage) (wikiPage, error) {
	p.Nom = strings.TrimSpace(p.Nom)
	if !validWikiName(p.Nom) {
		return wikiPage{}, skillError("nom de pàgina no vàlid")
	}
	p.Titol = strings.TrimSpace(p.Titol)
	p.Descripcio = strings.TrimSpace(p.Descripcio)
	p.Cos = strings.TrimSpace(p.Cos)
	if p.Titol == "" || p.Cos == "" {
		return wikiPage{}, errBadSkill
	}
	p.Updated = time.Now().UTC().Format(time.RFC3339)
	var b strings.Builder
	b.WriteString("---\ntitol: " + p.Titol + "\ndescripcio: " + p.Descripcio + "\nupdated: " + p.Updated + "\n---\n\n")
	b.WriteString(p.Cos)
	if err := os.MkdirAll(s.windWikiDir(), 0o755); err != nil {
		return wikiPage{}, err
	}
	if err := os.WriteFile(filepath.Join(s.windWikiDir(), p.Nom+".md"), []byte(b.String()), 0o644); err != nil {
		return wikiPage{}, err
	}
	return p, nil
}

func (s *Server) handleWindsWiki(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/api/winds/wiki/") {
		nom := strings.TrimPrefix(r.URL.Path, "/api/winds/wiki/")
		if nom == "" || strings.Contains(nom, "/") {
			http.Error(w, "nom de pàgina no vàlid", http.StatusBadRequest)
			return
		}
		raw, err := os.ReadFile(filepath.Join(s.windWikiDir(), nom+".md"))
		if err != nil {
			http.Error(w, "no trobada", http.StatusNotFound)
			return
		}
		writeJSON(w, map[string]any{"page": parseWikiPage(nom, string(raw))})
		return
	}
	if r.Method == http.MethodDelete {
		nom := strings.TrimSpace(r.URL.Path[len("/api/winds/wiki/"):])
		if !validWikiName(nom) {
			http.Error(w, "nom de pàgina no vàlid", http.StatusBadRequest)
			return
		}
		if err := os.Remove(filepath.Join(s.windWikiDir(), nom+".md")); err != nil {
			http.Error(w, "no trobada", http.StatusNotFound)
			return
		}
		writeJSON(w, map[string]bool{"ok": true})
		return
	}
	if r.Method == http.MethodPost {
		if strings.HasSuffix(r.URL.Path, "/summary") {
			s.handleWikiSummary(w, r)
			return
		}
		var p wikiPage
		if !decodeSmall(w, r, &p) {
			return
		}
		saved, err := s.saveWikiPage(p)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, map[string]any{"page": saved})
		return
	}
	if r.Method == http.MethodGet {
		writeJSON(w, map[string]any{"pages": s.wikiList()})
		return
	}
	http.Error(w, "GET, POST or DELETE required", http.StatusMethodNotAllowed)
}

// wikiSummaryBrief és la consigna de destil·lació: resum útil, no transcripció.
func wikiSummaryBrief(lang, title, who string) string {
	return "You distill a conversation into a WIKI PAGE the user will keep. Write it in " + lang + ", in markdown.\n" +
		"- Start with a one-line description of what the page is about (after the title).\n" +
		"- Then the substance: key facts, decisions, numbers, dates — organized with short sections and bullet points.\n" +
		"- Keep any useful charts (fenced ```chart with real numbers from the conversation) and images ( ![title](url) ) that were shared.\n" +
		"- Include the source URLs that were cited, as a short list at the end if they matter.\n" +
		"- Never invent anything that is not in the conversation.\n" +
		"- Output ONLY the markdown body of the page (no frontmatter, no code fences around everything)."
}

// handleWikiSummary resumeix l'última conversa (vent o rotllana) i la desa com a
// pàgina de wiki. La veu fa el resum amb una crida lateral (barata, sense eines).
func (s *Server) handleWikiSummary(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Wind     string `json:"wind"`
		Rotllana string `json:"rotllana"`
		Titol    string `json:"titol"`
	}
	if !decodeSmall(w, r, &req) {
		return
	}
	var messages []llm.Message
	who := ""
	name := strings.TrimSpace(req.Titol)
	if id := strings.TrimSpace(req.Wind); id != "" {
		wind, ok := s.winds.ByRef(id)
		if !ok {
			http.Error(w, "unknown wind", http.StatusBadRequest)
			return
		}
		if saved, err := session.Load(s.sessDir(), "company-"+wind.ID); err == nil {
			messages = saved.Convo
		}
		who = wind.Name
		if name == "" {
			name = wind.Name
		}
	} else if id := strings.TrimSpace(req.Rotllana); id != "" {
		rot, ok := s.rotllanaByID(id)
		if !ok {
			http.Error(w, "unknown rotllana", http.StatusBadRequest)
			return
		}
		if saved, err := session.Load(s.sessDir(), "rotllana-"+rot.ID); err == nil {
			messages = saved.Convo
		}
		who = rot.Name
		if name == "" {
			name = rot.Name
		}
	} else {
		http.Error(w, "wind o rotllana obligatori", http.StatusBadRequest)
		return
	}
	if len(messages) == 0 {
		http.Error(w, "conversa buida", http.StatusBadRequest)
		return
	}
	transcript := agent.TranscriptDigest(messages, 24000)
	s.mu.Lock()
	p, role := s.roleRefFor(s.role)
	s.mu.Unlock()
	ctx, cancel := context.WithTimeout(r.Context(), 120*time.Second)
	defer cancel()
	msgs := []llm.Message{
		{Role: "system", Content: wikiSummaryBrief(s.cfg.Lang(), name, who)},
		{Role: "user", Content: "Conversation to distill (from " + who + "):\n\n" + transcript},
	}
	out, err := s.client.Chat(llm.WithSideCall(ctx), p.BaseURL, p.APIKey, role.Model, msgs, 0.3, 1400)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	body := strings.TrimSpace(strings.Trim(strings.TrimSpace(out), "`"))
	// títol de pàgina: el que ha posat l'usuari o el primer encapçalament del resum.
	title := name
	for _, ln := range strings.Split(body, "\n") {
		ln = strings.TrimLeft(strings.TrimSpace(ln), "# ")
		if ln != "" {
			title = ln
			break
		}
	}
	desc := ""
	if i := strings.Index(body, "\n"); i > 0 {
		desc = strings.TrimSpace(strings.TrimLeft(body[:i], "# "))
	}
	page, err := s.saveWikiPage(wikiPage{
		Nom:         companion.SlugWind(title),
		Titol:       title,
		Descripcio:  desc,
		Cos:         body,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, map[string]any{"page": page})
}
