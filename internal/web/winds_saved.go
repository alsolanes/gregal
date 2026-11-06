package web

import (
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"gregal/internal/companion"
	"gregal/internal/llm"
)

// Desat: tot el que els vents han fet per a l'usuari, recollit de les seves
// converses (xat propi i rotllanes on han parlat) perquè no s'hagi de pujar
// pel fil per trobar-ho. Documents creats, webs publicades, gràfics, imatges,
// fitxes de referència i les fonts que han citat.
//
//	GET /api/winds/saved[?wind=<id>] → {items:[…], counts:{kind:n}}
//
// No hi ha cap magatzem nou: la conversa és la font i cada element en sap
// l'autor i el moment. Els documents que ja no existeixen no surten.

type SavedItem struct {
	Kind     string `json:"kind"` // doc | web | chart | image | card | link
	Title    string `json:"title"`
	URL      string `json:"url,omitempty"`
	Spec     string `json:"spec,omitempty"` // gràfic: el JSON del bloc
	ID       string `json:"id,omitempty"`   // fitxa
	Wind     string `json:"wind"`
	WindName string `json:"wind_name"`
	Where    string `json:"where,omitempty"` // nom de la rotllana; buit = xat propi
	Ts       int64  `json:"ts"`
}

var (
	savedFence    = regexp.MustCompile("(?s)```(chart|gregal-plot)[ \\t]*\\n(.*?)```")
	savedAnyFence = regexp.MustCompile("(?s)```.*?```")
	savedImage    = regexp.MustCompile(`!\[([^\]\n]*)\]\(([^)\s]+)\)`)
	savedLink     = regexp.MustCompile(`\[([^\]\n]+)\]\(([^)\s]+)\)`)
)

// savedFromText treu d'un missatge del vent el que val la pena desar.
func savedFromText(text string) []SavedItem {
	var out []SavedItem
	for _, m := range savedFence.FindAllStringSubmatch(text, -1) {
		spec := strings.TrimSpace(m[2])
		var head struct {
			Title string `json:"title"`
		}
		if json.Unmarshal([]byte(spec), &head) != nil {
			continue
		}
		out = append(out, SavedItem{Kind: "chart", Title: strings.TrimSpace(head.Title), Spec: spec})
	}
	// El codi no és desat: un enllaç dins d'un bloc de codi és text d'exemple.
	text = savedAnyFence.ReplaceAllString(text, " ")
	for _, m := range savedImage.FindAllStringSubmatch(text, -1) {
		if u := m[2]; strings.HasPrefix(u, "http") || strings.HasPrefix(u, "/") {
			out = append(out, SavedItem{Kind: "image", Title: strings.TrimSpace(m[1]), URL: u})
		}
	}
	text = savedImage.ReplaceAllString(text, " ")
	for _, m := range savedLink.FindAllStringSubmatch(text, -1) {
		title, u := strings.TrimSpace(m[1]), m[2]
		kind := ""
		switch {
		case strings.Contains(u, "/api/winds/doc?") || strings.Contains(u, "/api/office/download?"):
			kind = "doc"
		case strings.Contains(u, "/share/"):
			kind = "web"
		case strings.HasPrefix(u, "http://") || strings.HasPrefix(u, "https://"):
			kind = "link"
		}
		if kind != "" {
			out = append(out, SavedItem{Kind: kind, Title: title, URL: u})
		}
	}
	return out
}

// savedDocExists: un document de vent esborrat o un adjunt caducat no surt.
func (s *Server) savedDocExists(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	q := u.Query()
	if strings.HasSuffix(u.Path, "/api/winds/doc") {
		name := filepath.Base(q.Get("name"))
		if name == "" || name == "." || strings.HasPrefix(name, ".") {
			return false
		}
		_, err := os.Stat(filepath.Join(s.windDocsDir(), name))
		return err == nil
	}
	if strings.HasSuffix(u.Path, "/api/office/download") {
		officeStore.Lock()
		defer officeStore.Unlock()
		d, ok := officeStore.docs[q.Get("id")]
		if !ok || (d.owner != "" && d.owner != s.usuari()) {
			return false
		}
		_, err := os.Stat(d.path)
		return err == nil
	}
	return true
}

// savedMax és el sostre de la llista: la conversa pot ser molt llarga.
const savedMax = 400

func (s *Server) collectSaved(only string) ([]SavedItem, error) {
	list, err := s.winds.List()
	if err != nil {
		return nil, err
	}
	winds := map[string]companion.Wind{}
	for _, w := range list {
		if only == "" || w.ID == only {
			winds[w.ID] = w
		}
	}
	var items []SavedItem
	add := func(w companion.Wind, where string, msgs []llm.Message) {
		for _, m := range msgs {
			if m.Role != "assistant" {
				continue
			}
			for _, it := range savedFromText(m.Content) {
				it.Wind, it.WindName, it.Where, it.Ts = w.ID, w.Name, where, m.Ts
				items = append(items, it)
			}
		}
	}
	for _, w := range winds {
		h := windHistory(s, w.ID)
		h.mu.Lock()
		msgs := append([]llm.Message(nil), h.messages...)
		h.mu.Unlock()
		add(w, "", msgs)
	}
	if rots, err := s.listRotllanes(); err == nil {
		for _, rot := range rots {
			h := rotllanaHistory(s, rot.ID)
			h.mu.Lock()
			msgs := append([]llm.Message(nil), h.messages...)
			h.mu.Unlock()
			bySpeaker := map[string][]llm.Message{}
			for _, m := range msgs {
				if _, ok := winds[m.SpeakerID]; ok {
					bySpeaker[m.SpeakerID] = append(bySpeaker[m.SpeakerID], m)
				}
			}
			for id, ms := range bySpeaker {
				add(winds[id], firstNonEmpty(rot.Name, rot.ID), ms)
			}
		}
	}
	// Les fitxes de referència són desades per definició.
	windCardsMu.Lock()
	for _, w := range winds {
		if !validWindSkillName(w.ID) {
			continue
		}
		dir := filepath.Join(s.windStateDir(), "wind-cards", w.ID)
		entries, _ := os.ReadDir(dir)
		for _, e := range entries {
			id := strings.TrimSuffix(e.Name(), ".json")
			if e.IsDir() || !windCardID.MatchString(id) {
				continue
			}
			c, err := readCard(filepath.Join(dir, e.Name()))
			if err != nil {
				continue
			}
			ts := int64(0)
			if t, err := time.Parse(time.RFC3339, c.Updated); err == nil {
				ts = t.Unix()
			}
			items = append(items, SavedItem{Kind: "card", Title: c.Title, ID: c.ID, Wind: w.ID, WindName: w.Name, Ts: ts})
		}
	}
	windCardsMu.Unlock()

	// El més nou primer i cada cosa un sol cop (el mateix document citat dues
	// vegades és un sol document).
	sort.SliceStable(items, func(a, b int) bool { return items[a].Ts > items[b].Ts })
	seen := map[string]bool{}
	out := make([]SavedItem, 0, len(items))
	for _, it := range items {
		key := it.Kind + "\x00" + it.URL + "\x00" + it.Spec + "\x00" + it.ID
		if seen[key] {
			continue
		}
		seen[key] = true
		if it.Kind == "doc" && !s.savedDocExists(it.URL) {
			continue
		}
		if it.Title == "" {
			it.Title = savedTitleFromURL(it.URL)
		}
		out = append(out, it)
		if len(out) == savedMax {
			break
		}
	}
	return out, nil
}

// savedTitleFromURL dona un títol a un element que no en porta.
func savedTitleFromURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || raw == "" {
		return ""
	}
	if n := u.Query().Get("name"); n != "" {
		return n
	}
	if u.Host != "" {
		return strings.TrimPrefix(u.Host, "www.")
	}
	return filepath.Base(u.Path)
}

func (s *Server) handleWindsSaved(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "GET required", http.StatusMethodNotAllowed)
		return
	}
	items := []SavedItem{}
	if s.winds != nil {
		got, err := s.collectSaved(strings.TrimSpace(r.URL.Query().Get("wind")))
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		items = append(items, got...)
	}
	base := requestBase(r)
	counts := map[string]int{}
	for i := range items {
		items[i].URL = savedURLFor(items[i].Kind, items[i].URL, base)
		counts[items[i].Kind]++
	}
	writeJSON(w, map[string]any{"items": items, "counts": counts})
}

// savedURLFor: els documents van relatius (les apps els baixen amb la seva
// clau, no com una web); la resta, absoluts amb l'adreça per on ha arribat la
// petició, perquè el mòbil els pugui obrir al navegador.
func savedURLFor(kind, raw, base string) string {
	if raw == "" {
		return raw
	}
	if kind == "doc" {
		if u, err := url.Parse(raw); err == nil && u.IsAbs() {
			return u.RequestURI()
		}
		return raw
	}
	if strings.HasPrefix(raw, "/") && base != "" {
		return strings.TrimRight(base, "/") + raw
	}
	return raw
}
