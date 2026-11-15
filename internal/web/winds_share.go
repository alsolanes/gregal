package web

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"html"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"gregal/internal/companion"
	"gregal/internal/tema"
)

// Compartir un missatge, una targeta, un pla, un menú o un gràfic.
//
//	POST /api/winds/clip  {text,title,from?}     → enllaç públic de només lectura
//	POST /api/winds/send  {to:[nom],text,title,from?} → a la safata d'una altra persona de l'app
//	GET  /api/winds/inbox                         → {items:[…], unread:n}
//	POST /api/winds/inbox {id, action:read|forget}
//
// L'enllaç públic és una web publicada (share_site.go) amb Clip=true: una
// pàgina que pinta el markdown amb el mateix md.js de l'app (targetes, fotos i
// gràfics), sense cap dada del compte. Les fotos fetes pels vents (que l'API
// només dona amb clau) s'hi copien al costat.
//
// La safata viu fora dels directoris personals (<config>/shared-inbox), com
// les rotllanes compartides: el que envies és una còpia, no un accés a res teu.
// Les fotos dels vents s'hi copien i /api/winds/image les troba allà també.

const (
	shareMaxText  = 64 << 10
	inboxMax      = 200
	shareMaxTitle = 120
)

var (
	shareWindImg = regexp.MustCompile(`/api/winds/image\?id=([a-f0-9]{24}\.(?:jpg|png|webp))`)
	// Un document d'un vent només s'obre amb la clau de qui l'ha fet: fora
	// d'aquest compte queda el títol com a text.
	shareDocLink = regexp.MustCompile(`\[([^\]\n]+)\]\(/api/(?:winds/doc|office/download)\?[^)\s]*\)`)
	shareCard    = regexp.MustCompile("(?s)```(?:gregal-[a-z]+|chart)[ \\t]*\\n(.*?)```")
	inboxMu      sync.Mutex
)

// InboxItem és una cosa que algú de l'app t'ha enviat.
type InboxItem struct {
	ID       string `json:"id"`
	From     string `json:"from"`
	WindName string `json:"wind_name,omitempty"`
	Title    string `json:"title"`
	Text     string `json:"text"`
	Ts       int64  `json:"ts"`
	Read     bool   `json:"read,omitempty"`
}

type shareReq struct {
	To    []string `json:"to"`
	Text  string   `json:"text"`
	Title string   `json:"title"`
	From  string   `json:"from"` // nom del vent que ho va dir
}

func readShareReq(r *http.Request) (shareReq, error) {
	var req shareReq
	if err := json.NewDecoder(io.LimitReader(r.Body, shareMaxText+8<<10)).Decode(&req); err != nil {
		return req, errors.New("invalid json")
	}
	req.Text = strings.TrimSpace(req.Text)
	if req.Text == "" {
		return req, errors.New("nothing to share")
	}
	if len(req.Text) > shareMaxText {
		return req, errors.New("the message is too long to share")
	}
	req.Title = shareTitle(req.Title, req.Text)
	req.From = clipRunes(strings.TrimSpace(req.From), 60)
	return req, nil
}

// shareTitle: el que diu el client, o el títol de la primera targeta, o la
// primera línia amb text.
func shareTitle(title, text string) string {
	if t := strings.TrimSpace(title); t != "" {
		return clipRunes(t, shareMaxTitle)
	}
	// El títol de la targeta (el de dalt del JSON, no el d'un pas).
	if m := shareCard.FindStringSubmatch(text); m != nil {
		var head struct {
			Title string `json:"title"`
		}
		if json.Unmarshal([]byte(m[1]), &head) == nil && strings.TrimSpace(head.Title) != "" {
			return clipRunes(strings.TrimSpace(head.Title), shareMaxTitle)
		}
	}
	for _, l := range strings.Split(savedAnyFence.ReplaceAllString(text, ""), "\n") {
		l = strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(l), "#>*-• "))
		l = strings.NewReplacer("**", "", "__", "", "`", "").Replace(l)
		if l != "" && !strings.HasPrefix(l, "![") {
			return clipRunes(l, 80)
		}
	}
	return "Gregal"
}

// shareImages són les fotos dels vents que surten al text (sense repetir).
func shareImages(text string) []string {
	var out []string
	seen := map[string]bool{}
	for _, m := range shareWindImg.FindAllStringSubmatch(text, -1) {
		if !seen[m[1]] {
			seen[m[1]] = true
			out = append(out, m[1])
		}
	}
	return out
}

// --- Enllaç públic ---------------------------------------------------------

func (s *Server) handleWindClip(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	req, err := readShareReq(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	id, err := newShareID()
	if err != nil {
		http.Error(w, "random source unavailable", http.StatusInternalServerError)
		return
	}
	text := shareDocLink.ReplaceAllString(req.Text, "$1")
	var files []siteFile
	for _, name := range shareImages(text) {
		raw, err := os.ReadFile(filepath.Join(s.windImagesDir(), name))
		if err != nil {
			continue
		}
		files = append(files, siteFile{"img/" + name, raw})
	}
	// Les que no hi són (esborrades) es treuen: millor sense foto que trencada.
	text = shareWindImg.ReplaceAllStringFunc(text, func(m string) string {
		name := shareWindImg.FindStringSubmatch(m)[1]
		for _, f := range files {
			if f.rel == "img/"+name {
				return "img/" + name
			}
		}
		return "img/missing"
	})
	files = append(files, siteFile{"index.html", []byte(clipPage(req.Title, req.From, s.sharedMe(), text))})
	now := time.Now().UTC()
	m := shareMeta{ID: id, Kind: "site", Clip: true, Owner: s.usuari(), Created: now, Updated: &now, Title: req.Title, Files: len(files)}
	for _, f := range files {
		m.Bytes += len(f.data)
	}
	if err := writeSite(m, files); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	out := siteResponse(m)
	link := out["url"].(string)
	if link == "" {
		link = strings.TrimRight(requestBase(r), "/") + out["path"].(string)
	}
	out["link"] = link
	// Qui el rebi per WhatsApp ha de poder obrir-lo: si l'adreça és de casa, ho diem.
	out["private"] = linkIsPrivate(link)
	writeJSON(w, out)
}

// clipPage és la pàgina pública: el text va com a JSON (escapat per a HTML)
// i md.js el pinta igual que a l'app. Sense JavaScript es llegeix el text.
func clipPage(title, wind, person, text string) string {
	data, _ := json.Marshal(map[string]string{"title": title, "wind": wind, "person": person, "text": text})
	by := "Gregal"
	if wind != "" {
		by = wind
	}
	if person != "" {
		by += " · " + person
	}
	desc := clipRunes(strings.Join(strings.Fields(savedAnyFence.ReplaceAllString(text, " ")), " "), 160)
	t := html.EscapeString(title)
	bodyClass := "gclip"
	if shareCard.MatchString(text) {
		bodyClass += " gclip-hascard" // la targeta ja porta el títol
	}
	return "<!doctype html><html lang=\"ca\"><head><meta charset=\"utf-8\">" +
		"<meta name=\"viewport\" content=\"width=device-width,initial-scale=1\"><title>" + t + "</title>" +
		"<meta property=\"og:title\" content=\"" + t + "\"><meta property=\"og:description\" content=\"" + html.EscapeString(desc) + "\">" +
		"<meta name=\"robots\" content=\"noindex\"><style>" + tema.CSSRoot() + "</style>" +
		"<link rel=\"stylesheet\" href=\"/app/calma.css\"><link rel=\"stylesheet\" href=\"/app/winds-workspace.css\"><link rel=\"stylesheet\" href=\"/app/plots.css\"><link rel=\"stylesheet\" href=\"/app/clip.css\">" +
		"</head><body class=\"" + bodyClass + "\"><main class=\"gclip-main\"><header class=\"gclip-head\"><span class=\"gclip-mark\">gregal</span><span class=\"gclip-by\">" + html.EscapeString(by) + "</span></header>" +
		"<h1 class=\"gclip-title\">" + t + "</h1>" +
		"<article id=\"gclip-body\" class=\"gclip-body vw-message-text md\"><pre class=\"gclip-raw\">" + html.EscapeString(text) + "</pre></article>" +
		"<footer class=\"gclip-foot\">Compartit des de Gregal · només lectura</footer></main>" +
		"<script type=\"application/json\" id=\"gclip-data\">" + string(data) + "</script>" +
		"<script src=\"/app/md.js\"></script><script src=\"/app/plots-core.js\"></script><script src=\"/app/clip-view.js\"></script></body></html>"
}

// --- Safata: enviar a una altra persona de l'app ---------------------------

func (s *Server) inboxDir() string { return filepath.Join(goalDirFor(s.cfgPath), "shared-inbox") }

func (s *Server) inboxPath(name string) string {
	return filepath.Join(s.inboxDir(), hex.EncodeToString([]byte(name))+".json")
}

func (s *Server) inboxLoad(name string) []InboxItem {
	raw, err := os.ReadFile(s.inboxPath(name))
	if err != nil {
		return nil
	}
	var list []InboxItem
	_ = json.Unmarshal(raw, &list)
	return list
}

func (s *Server) inboxSave(name string, list []InboxItem) error {
	if len(list) > inboxMax {
		list = list[len(list)-inboxMax:]
	}
	if err := os.MkdirAll(s.inboxDir(), 0o700); err != nil {
		return err
	}
	raw, err := json.Marshal(list)
	if err != nil {
		return err
	}
	return writeFileAtomic(s.inboxPath(name), raw)
}

// inboxImagePath: una foto d'un vent que algú t'ha enviat.
func (s *Server) inboxImagePath(id string) string { return filepath.Join(s.inboxDir(), "img", id) }

func (s *Server) handleWindSend(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	me := s.sharedMe()
	if me == "" {
		http.Error(w, s.tr("per enviar a algú calen comptes al servidor", "sending to someone needs accounts on the server"), http.StatusNotFound)
		return
	}
	req, err := readShareReq(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	known := map[string]bool{}
	if s.users != nil {
		for _, n := range s.users.Noms() {
			known[n] = true
		}
	}
	var to []string
	for _, n := range req.To {
		n = strings.TrimSpace(n)
		if n != "" && n != me && known[n] && !containsStr(to, n) {
			to = append(to, n)
		}
	}
	if len(to) == 0 {
		http.Error(w, s.tr("tria a qui ho envies", "choose who to send it to"), http.StatusBadRequest)
		return
	}
	text := shareDocLink.ReplaceAllString(req.Text, "$1")
	// Les fotos dels vents van amb la còpia: l'altra persona no pot llegir les teves.
	for _, name := range shareImages(text) {
		raw, err := os.ReadFile(filepath.Join(s.windImagesDir(), name))
		if err != nil {
			continue
		}
		_ = os.MkdirAll(filepath.Dir(s.inboxImagePath(name)), 0o700)
		_ = os.WriteFile(s.inboxImagePath(name), raw, 0o600)
	}
	now := time.Now()
	inboxMu.Lock()
	for _, n := range to {
		id, _ := newShareID()
		item := InboxItem{ID: id[:20], From: me, WindName: req.From, Title: req.Title, Text: text, Ts: now.Unix()}
		if err := s.inboxSave(n, append(s.inboxLoad(n), item)); err != nil {
			inboxMu.Unlock()
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	inboxMu.Unlock()
	for _, n := range to {
		s.inboxNotify(n, me, req.Title)
	}
	writeJSON(w, map[string]any{"sent": to})
}

// inboxNotify avisa qui ho rep amb una novetat del seu Gregal (sona al mòbil
// si té push o quan l'app consulta). Best-effort.
func (s *Server) inboxNotify(to, from, title string) {
	if s.hub == nil || s.users == nil {
		return
	}
	u := s.users.Usuari(to)
	if u == nil || u.Tothom {
		return
	}
	rs := s.hub.SessionFor(u, "wind-routines")
	if rs == nil || rs.winds == nil {
		return
	}
	list, err := rs.winds.List()
	if err != nil || len(list) == 0 {
		return
	}
	var wind companion.Wind = list[0]
	for _, x := range list {
		if x.ID == "gregal" {
			wind = x
		}
	}
	text := rs.tr(from+" t'ha enviat «"+title+"». És a Desat.", from+" sent you “"+title+"”. It's in Saved.")
	recordNovetat(rs, wind, "shared", text)
}

func (s *Server) handleWindInbox(w http.ResponseWriter, r *http.Request) {
	me := s.sharedMe()
	switch r.Method {
	case http.MethodGet:
		items := []InboxItem{}
		unread := 0
		if me != "" {
			inboxMu.Lock()
			items = append(items, s.inboxLoad(me)...)
			inboxMu.Unlock()
		}
		sort.SliceStable(items, func(a, b int) bool { return items[a].Ts > items[b].Ts })
		for _, it := range items {
			if !it.Read {
				unread++
			}
		}
		writeJSON(w, map[string]any{"items": items, "unread": unread})
	case http.MethodPost:
		var req struct {
			ID     string `json:"id"`
			Action string `json:"action"`
		}
		if json.NewDecoder(io.LimitReader(r.Body, 1<<12)).Decode(&req) != nil || me == "" {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}
		inboxMu.Lock()
		defer inboxMu.Unlock()
		list := s.inboxLoad(me)
		out := list[:0]
		found := false
		for _, it := range list {
			if it.ID == req.ID || (req.ID == "*" && req.Action == "read") {
				found = true
				if req.Action == "forget" {
					continue
				}
				it.Read = true
			}
			out = append(out, it)
		}
		if !found {
			http.NotFound(w, r)
			return
		}
		if err := s.inboxSave(me, out); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]any{"ok": true})
	default:
		http.Error(w, "GET or POST", http.StatusMethodNotAllowed)
	}
}
