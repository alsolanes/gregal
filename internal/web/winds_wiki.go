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
	Nom        string `json:"nom"`
	Titol      string `json:"titol"`
	Descripcio string `json:"descripcio"`
	Cos        string `json:"cos,omitempty"`
	Updated    string `json:"updated,omitempty"`
	// Font és la conversa d'on surt (wind:<id> o rotllana:<id>): tornar a
	// resumir-la actualitza la mateixa pàgina en lloc de fer-ne una de nova.
	Font string `json:"font,omitempty"`
	// Autor és qui l'ha escrita: el vent o la rotllana de la conversa.
	Autor string `json:"autor,omitempty"`
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
				} else if v, okf := strings.CutPrefix(ln, "font:"); okf {
					p.Font = strings.TrimSpace(v)
				} else if v, okf := strings.CutPrefix(ln, "autor:"); okf {
					p.Autor = strings.TrimSpace(v)
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
	// Les més recents primer: és el que s'acaba de resumir o actualitzar.
	sort.SliceStable(pages, func(i, j int) bool {
		if pages[i].Updated != pages[j].Updated {
			return pages[i].Updated > pages[j].Updated
		}
		return pages[i].Nom < pages[j].Nom
	})
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
	b.WriteString("---\ntitol: " + wikiLine(p.Titol) + "\ndescripcio: " + wikiLine(p.Descripcio) + "\nupdated: " + p.Updated + "\n")
	if p.Font != "" {
		b.WriteString("font: " + wikiLine(p.Font) + "\n")
	}
	if p.Autor != "" {
		b.WriteString("autor: " + wikiLine(p.Autor) + "\n")
	}
	b.WriteString("---\n\n")
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
		if !validWikiName(nom) {
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

// wikiLine evita que un títol o una descripció trenquin el frontmatter.
func wikiLine(v string) string { return strings.Join(strings.Fields(v), " ") }

// wikiLanguage dona el nom de la llengua al model: un codi sol ("ca") el confon.
func wikiLanguage(code string) string {
	switch strings.ToLower(strings.TrimSpace(code)) {
	case "ca":
		return "Catalan"
	case "es":
		return "Spanish"
	case "en", "":
		return "English"
	}
	return code
}

// wikiSummaryBrief és la consigna de destil·lació: una pàgina útil, no una transcripció.
func wikiSummaryBrief(lang string, update bool) string {
	b := "You distill a conversation into a WIKI PAGE the user will keep and come back to. Write it in the language the conversation is in (if unclear, " + wikiLanguage(lang) + "), in markdown.\n" +
		"- First line: `# <short title>` naming the subject (not the conversation).\n" +
		"- Second line: one plain sentence saying what the page covers.\n" +
		"- Then the substance: key facts, decisions, numbers, dates and open questions, in short sections with bullet points. Prefer a table when several items share the same attributes.\n" +
		"- When the conversation has numbers that compare or evolve, add a chart as a fenced ```chart block with JSON {\"title\":\"…\",\"type\":\"bar|line|pie\",\"labels\":[\"…\"],\"series\":[{\"name\":\"…\",\"values\":[1,2]}]} using only those numbers.\n" +
		"- Keep the images that were shared ( ![title](url) on their own line ) where they help; never invent an image URL.\n" +
		"- End with a short sources section listing the URLs that were cited, if any.\n" +
		"- Never invent anything that is not in the conversation"
	if update {
		b += " or in the current page.\n- You are UPDATING an existing page: keep what is still true, add what is new, correct what changed, and drop nothing useful. Use the conversation's language even if the current page is in another one.\n"
	} else {
		b += ".\n"
	}
	return b + "- Output ONLY the markdown page (no frontmatter, no code fence around the whole page)."
}

// unwrapMarkdownFence treu una tanca ```markdown que embolcalli TOTA la
// resposta. Les tanques internes (un ```chart al final) es conserven.
func unwrapMarkdownFence(out string) string {
	out = strings.TrimSpace(out)
	if !strings.HasPrefix(out, "```") || !strings.HasSuffix(out, "```") {
		return out
	}
	first, rest, ok := strings.Cut(out, "\n")
	if !ok {
		return out
	}
	lang := strings.TrimSpace(strings.TrimPrefix(first, "```"))
	if lang != "" && lang != "markdown" && lang != "md" {
		return out
	}
	return strings.TrimSpace(strings.TrimSuffix(rest, "```"))
}

// splitWikiSummary separa el títol (# …) i la descripció (primera frase) del cos.
func splitWikiSummary(body string) (title, desc, rest string) {
	lines := strings.Split(strings.TrimSpace(body), "\n")
	i := 0
	if i < len(lines) && strings.HasPrefix(strings.TrimSpace(lines[i]), "#") {
		title = strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(lines[i]), "#"))
		i++
	}
	for i < len(lines) && strings.TrimSpace(lines[i]) == "" {
		i++
	}
	if i < len(lines) {
		ln := strings.TrimSpace(lines[i])
		if ln != "" && !strings.HasPrefix(ln, "#") && !strings.HasPrefix(ln, "-") && !strings.HasPrefix(ln, "|") && !strings.HasPrefix(ln, "!") && !strings.HasPrefix(ln, "```") {
			desc = ln
			i++
		}
	}
	return title, desc, strings.TrimSpace(strings.Join(lines[i:], "\n"))
}

// wikiSummarizer fa la crida lateral de resum; els tests la substitueixen.
var wikiSummarizer = func(s *Server, ctx context.Context, msgs []llm.Message) (string, error) {
	s.mu.Lock()
	p, role := s.roleRefFor(s.role)
	client := s.client
	s.mu.Unlock()
	return client.Chat(llm.WithSideCall(ctx), p.BaseURL, p.APIKey, role.Model, msgs, 0.3, 3500)
}

// wikiPageBySource troba la pàgina que ja resumeix aquesta conversa.
func (s *Server) wikiPageBySource(font string) (wikiPage, bool) {
	for _, p := range s.wikiList() {
		if p.Font == font {
			raw, err := os.ReadFile(filepath.Join(s.windWikiDir(), p.Nom+".md"))
			if err == nil {
				return parseWikiPage(p.Nom, string(raw)), true
			}
		}
	}
	return wikiPage{}, false
}

// handleWikiSummary resumeix una conversa (vent o rotllana) en una pàgina de
// wiki. Si la conversa ja tenia pàgina, l'actualitza amb el que és nou.
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
	who, font := "", ""
	if id := strings.TrimSpace(req.Wind); id != "" {
		wind, ok := s.winds.ByRef(id)
		if !ok {
			http.Error(w, "unknown wind", http.StatusBadRequest)
			return
		}
		if saved, err := session.Load(s.sessDir(), "company-"+wind.ID); err == nil {
			messages = saved.Convo
		}
		who, font = wind.Name, "wind:"+wind.ID
	} else if id := strings.TrimSpace(req.Rotllana); id != "" {
		rot, ok := s.rotllanaByID(id)
		if !ok {
			http.Error(w, "unknown rotllana", http.StatusBadRequest)
			return
		}
		if saved, err := session.Load(s.sessDir(), "rotllana-"+rot.ID); err == nil {
			messages = saved.Convo
		}
		who, font = rot.Name, "rotllana:"+rot.ID
	} else {
		http.Error(w, "wind o rotllana obligatori", http.StatusBadRequest)
		return
	}
	if len(messages) == 0 {
		http.Error(w, "conversa buida", http.StatusBadRequest)
		return
	}
	previous, update := s.wikiPageBySource(font)
	prompt := "Conversation to distill (from " + who + "):\n\n" + agent.TranscriptDigest(messages, 24000)
	if update {
		prompt = "Current page:\n\n# " + previous.Titol + "\n" + previous.Descripcio + "\n\n" + previous.Cos + "\n\n---\n\n" + prompt
	}
	ctx, cancel := context.WithTimeout(r.Context(), 150*time.Second)
	defer cancel()
	out, err := wikiSummarizer(s, ctx, []llm.Message{
		{Role: "system", Content: wikiSummaryBrief(s.cfg.Lang(), update)},
		{Role: "user", Content: prompt},
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	title, desc, body := splitWikiSummary(unwrapMarkdownFence(out))
	if t := strings.TrimSpace(req.Titol); t != "" {
		title = t
	}
	if title == "" {
		title = who
	}
	if body == "" {
		http.Error(w, "el resum ha sortit buit", http.StatusBadGateway)
		return
	}
	nom := companion.SlugWind(title)
	if update {
		nom = previous.Nom // la mateixa pàgina, encara que el títol hagi canviat
	} else if _, err := os.Stat(filepath.Join(s.windWikiDir(), nom+".md")); err == nil {
		nom += "-" + time.Now().Format("20060102-1504") // un altre tema amb el mateix títol no la trepitja
	}
	page, err := s.saveWikiPage(wikiPage{Nom: nom, Titol: title, Descripcio: desc, Cos: body, Font: font, Autor: who})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, map[string]any{"page": page, "updated": update})
}
