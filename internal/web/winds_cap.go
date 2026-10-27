package web

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"gregal/internal/companion"
	"gregal/internal/llm"
)

// La mascota, cap de gabinet de la colla. És l'única que l'usuari té a la
// pantalla principal del mòbil: li diu què necessita i ella decideix si ho
// respon ella mateixa, si ho passa a un vent, si obre una rotllana o si és
// una feina llarga per a una colla al servidor. La decisió es veu sempre i
// l'usuari la pot canviar; res no passa fins que diu «endavant».
//
//	GET  /api/winds/cap?lang=ca           → {name,color,style,history,brief}
//	POST /api/winds/cap {message,lang,force} → {action,reply,winds,task,rotllana,history}
//	POST /api/winds/cap/profile {name}    → {name,color,style}
//
// action: self (ho respon ella) | wind (un vent) | rotllana (2-3 vents) |
// crew (feina llarga). force fa la decisió de l'usuari: "self", "wind:<id>",
// "rotllana" o "crew".

const (
	capFile       = "cap.json"
	capMaxHistory = 40
	capName       = "Ventet"
	capColor      = "#FFC857"
	capStyle      = "pto"
)

type capMsg struct {
	From   string   `json:"from"` // user | cap
	Text   string   `json:"text"`
	Ts     int64    `json:"ts"`
	Action string   `json:"action,omitempty"`
	Winds  []string `json:"winds,omitempty"`
}

type capState struct {
	Name    string   `json:"name"`
	Color   string   `json:"color"`
	Style   string   `json:"style"`
	Seen    int64    `json:"seen"` // última visita: el resum compta el que ha passat després
	History []capMsg `json:"history"`
}

type capDecision struct {
	Action   string   `json:"action"`
	Reply    string   `json:"reply"`
	Winds    []string `json:"winds"`
	Task     string   `json:"task,omitempty"`
	Rotllana string   `json:"rotllana,omitempty"`
}

var capMu sync.Mutex

func (s *Server) capPath() string { return filepath.Join(s.windStateDir(), capFile) }

func (s *Server) loadCap() capState {
	st := capState{}
	if raw, err := os.ReadFile(s.capPath()); err == nil {
		_ = json.Unmarshal(raw, &st)
	}
	if strings.TrimSpace(st.Name) == "" {
		st.Name = capName
	}
	if st.Color == "" {
		st.Color = capColor
	}
	if st.Style == "" {
		st.Style = capStyle
	}
	if st.History == nil {
		st.History = []capMsg{}
	}
	return st
}

func (s *Server) saveCap(st capState) {
	if len(st.History) > capMaxHistory {
		st.History = append([]capMsg(nil), st.History[len(st.History)-capMaxHistory:]...)
	}
	raw, err := json.Marshal(st)
	if err != nil {
		return
	}
	_ = os.MkdirAll(filepath.Dir(s.capPath()), 0o700)
	_ = os.WriteFile(s.capPath(), raw, 0o600)
}

// capBrief és el resum de benvinguda: feines acabades des de l'última
// visita, les que corren ara i les veus noves dels vents. Sense model: és
// el que hi ha, dit curt.
type capBrief struct {
	Done    int      `json:"done"`
	Failed  int      `json:"failed"`
	Running int      `json:"running"`
	News    int      `json:"news"`
	Titles  []string `json:"titles,omitempty"` // les acabades, per dir-les pel nom
	Text    string   `json:"text"`
}

func (s *Server) capBriefSince(since int64, ca bool) capBrief {
	var b capBrief
	if records, err := s.windTaskRecords(); err == nil {
		for _, t := range s.windTaskSummaries(records) {
			switch t.Status {
			case "running":
				b.Running++
			case "done", "failed", "interrupted":
				if since > 0 && t.Updated.Unix() > since {
					if t.Status == "done" {
						b.Done++
						if len(b.Titles) < 3 {
							b.Titles = append(b.Titles, oneLine(t.Task, 60))
						}
					} else {
						b.Failed++
					}
				}
			}
		}
	}
	if since > 0 {
		novetatsMu.Lock()
		for _, n := range loadNovetats(s) {
			if n.Ts > since && !strings.HasPrefix(n.Source, "feina:") {
				b.News++
			}
		}
		novetatsMu.Unlock()
	}
	b.Text = capBriefText(b, ca)
	return b
}

func capBriefText(b capBrief, ca bool) string {
	var parts []string
	plural := func(n int, one, many string) string {
		if n == 1 {
			return one
		}
		return strconv.Itoa(n) + " " + many
	}
	if ca {
		if b.Done > 0 {
			if b.Done == 1 && len(b.Titles) == 1 {
				parts = append(parts, "ja està llesta «"+b.Titles[0]+"»")
			} else {
				parts = append(parts, plural(b.Done, "ha acabat una feina", "feines acabades"))
			}
		}
		if b.Failed > 0 {
			parts = append(parts, plural(b.Failed, "una feina s'ha encallat", "feines encallades"))
		}
		if b.Running > 0 {
			parts = append(parts, plural(b.Running, "una feina en marxa", "feines en marxa"))
		}
		if b.News > 0 {
			parts = append(parts, plural(b.News, "un vent t'ha dit una cosa", "veus noves dels vents"))
		}
		if len(parts) == 0 {
			return "Tot tranquil. Què necessites?"
		}
		return capitalise(strings.Join(parts, ", ")) + "."
	}
	if b.Done > 0 {
		if b.Done == 1 && len(b.Titles) == 1 {
			parts = append(parts, "“"+b.Titles[0]+"” is ready")
		} else {
			parts = append(parts, plural(b.Done, "a task finished", "tasks finished"))
		}
	}
	if b.Failed > 0 {
		parts = append(parts, plural(b.Failed, "a task got stuck", "tasks got stuck"))
	}
	if b.Running > 0 {
		parts = append(parts, plural(b.Running, "one task running", "tasks running"))
	}
	if b.News > 0 {
		parts = append(parts, plural(b.News, "a wind has news", "new notes from the winds"))
	}
	if len(parts) == 0 {
		return "All quiet. What do you need?"
	}
	return capitalise(strings.Join(parts, ", ")) + "."
}

func capitalise(s string) string {
	r := []rune(s)
	if len(r) == 0 {
		return s
	}
	return strings.ToUpper(string(r[0])) + string(r[1:])
}

func (s *Server) handleCap(w http.ResponseWriter, r *http.Request) {
	ca := r.URL.Query().Get("lang") == "ca"
	switch r.Method {
	case http.MethodGet:
		capMu.Lock()
		st := s.loadCap()
		brief := s.capBriefSince(st.Seen, ca)
		st.Seen = time.Now().Unix()
		s.saveCap(st)
		capMu.Unlock()
		writeJSON(w, map[string]any{"name": st.Name, "color": st.Color, "style": st.Style, "history": st.History, "brief": brief})
	case http.MethodPost:
		var req struct {
			Message string `json:"message"`
			Lang    string `json:"lang"`
			Force   string `json:"force"`
		}
		if !decodeSmall(w, r, &req) {
			return
		}
		msg := strings.TrimSpace(req.Message)
		if msg == "" || len(msg) > 8000 {
			http.Error(w, "message required", http.StatusBadRequest)
			return
		}
		ca = req.Lang == "ca"
		winds, err := s.winds.List()
		if err != nil {
			http.Error(w, "could not read winds", http.StatusInternalServerError)
			return
		}
		capMu.Lock()
		st := s.loadCap()
		capMu.Unlock()
		ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
		defer cancel()
		d, err := s.capDecide(ctx, st, msg, req.Force, ca, winds)
		if err != nil {
			http.Error(w, redactError(err), http.StatusBadGateway)
			return
		}
		capMu.Lock()
		st = s.loadCap()
		now := time.Now().Unix()
		// Una nova decisió sobre el mateix missatge (l'usuari ha triat un
		// altre camí) substitueix l'anterior: el fil no repeteix la pregunta.
		if n := len(st.History); req.Force != "" && n >= 2 && st.History[n-2].From == "user" && st.History[n-2].Text == msg {
			st.History = st.History[:n-1]
		} else {
			st.History = append(st.History, capMsg{From: "user", Text: msg, Ts: now})
		}
		st.History = append(st.History, capMsg{From: "cap", Text: d.Reply, Ts: now, Action: d.Action, Winds: d.Winds})
		s.saveCap(st)
		capMu.Unlock()
		writeJSON(w, map[string]any{"action": d.Action, "reply": d.Reply, "winds": d.Winds, "task": d.Task, "rotllana": d.Rotllana, "history": st.History})
	default:
		http.Error(w, "GET or POST required", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleCapProfile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Name  string `json:"name"`
		Color string `json:"color"`
		Style string `json:"style"`
	}
	if !decodeSmall(w, r, &req) {
		return
	}
	capMu.Lock()
	defer capMu.Unlock()
	st := s.loadCap()
	if n := oneLine(req.Name, 24); n != "" {
		st.Name = n
	}
	if c := strings.TrimSpace(req.Color); len(c) == 7 && c[0] == '#' {
		st.Color = strings.ToUpper(c)
	}
	if v := strings.TrimSpace(req.Style); len(v) == 3 {
		st.Style = v
	}
	s.saveCap(st)
	writeJSON(w, map[string]string{"name": st.Name, "color": st.Color, "style": st.Style})
}

// capDecider és la crida al model; els tests la substitueixen.
var capDecider = func(s *Server, ctx context.Context, msgs []llm.Message) (string, error) {
	s.mu.Lock()
	cfg, client := s.cfg, s.client
	p, role := s.windRoleRef()
	s.mu.Unlock()
	return petChatter(ctx, client, cfg, p, role, msgs)
}

// capDecide tria el camí. Un vent nomenat va directe, sense model; la resta,
// una sola crida que decideix i, si ho respon la mascota, ja porta la resposta.
func (s *Server) capDecide(ctx context.Context, st capState, msg, force string, ca bool, winds []companion.Wind) (capDecision, error) {
	byID := map[string]companion.Wind{}
	for _, w := range winds {
		byID[w.ID] = w
	}
	switch {
	case strings.HasPrefix(force, "wind:"):
		if w, ok := byID[strings.TrimPrefix(force, "wind:")]; ok {
			return capFinish(s, capDecision{Action: "wind", Winds: []string{w.ID}}, st, byID, ca, msg), nil
		}
		return capDecision{}, errConst("unknown wind")
	case force == "crew":
		return capFinish(s, capDecision{Action: "crew", Task: msg}, st, byID, ca, msg), nil
	case force == "" && len(winds) > 0:
		if w, ok := windNamedIn(winds, msg); ok {
			return capFinish(s, capDecision{Action: "wind", Winds: []string{w.ID}}, st, byID, ca, msg), nil
		}
	}

	lang := "English"
	if ca {
		lang = "Catalan"
	}
	var roster strings.Builder
	for _, w := range winds {
		roster.WriteString("- " + w.ID + " | " + w.Name + " | " + w.Role)
		if w.Specialty != "" {
			roster.WriteString(" | specialty " + w.Specialty)
		}
		var descs []string
		for _, sk := range s.windSkills(w.ID) {
			descs = append(descs, sk.Descripcio)
		}
		if d := strings.Join(descs, "; "); d != "" {
			roster.WriteString(" | knows: " + oneLine(d, 200))
		}
		roster.WriteString("\n")
	}
	if len(winds) == 0 {
		roster.WriteString("(none yet)\n")
	}
	system := "You are " + st.Name + ", the user's small, cheerful mascot and chief of staff of a crew of AI helpers called winds. " +
		"You decide who should handle each request. Write `reply` in the language of the user's current message (" + lang + " if unclear), warmly and briefly, with at most one emoji. Reply with JSON only, no code fences."
	var b strings.Builder
	b.WriteString("WINDS:\n" + roster.String())
	if tail := capTail(st.History, 6); tail != "" {
		b.WriteString("\nRECENT TALK WITH YOU:\n" + tail)
	}
	b.WriteString("\nUSER NOW: " + msg + "\n\n")
	if force == "self" {
		b.WriteString("The user wants YOU to answer this yourself. action must be \"self\".\n")
	} else if force == "rotllana" {
		b.WriteString("The user wants a group chat: action must be \"rotllana\"; pick the 2 or 3 most relevant winds.\n")
	}
	b.WriteString("Pick ONE action:\n" +
		"- \"self\": greetings, small talk, quick facts, a short answer you can give in a few sentences, or when no wind fits. reply = your full answer (max 5 sentences).\n" +
		"- \"wind\": one wind's role or knowledge clearly matches. winds = [its id]. reply = one short playful sentence handing it over, naming the wind (e.g. \"This one is for Marinada 🛒\").\n" +
		"- \"rotllana\": it really needs two or three different viewpoints. winds = 2-3 ids. reply = one short sentence saying you are gathering them.\n" +
		"- \"crew\": long work that produces something: a website, an app, a report, an analysis of data, a plan with many parts. task = the request rewritten as a clear brief. reply = one short sentence saying this needs a crew working on the server.\n" +
		"Never invent wind ids. Prefer \"wind\" over \"self\" when a wind's role is about the topic.\n" +
		`JSON: {"action":"…","winds":["id"],"task":"…","reply":"…"}`)
	raw, err := capDecider(s, ctx, []llm.Message{{Role: "system", Content: system}, {Role: "user", Content: b.String()}})
	if err != nil {
		log.Printf("cap: %s", redactError(err))
		return capDecision{}, err
	}
	var d capDecision
	if obj := firstJSONObject(raw); obj == "" || json.Unmarshal([]byte(obj), &d) != nil {
		// Sense JSON, el model ha contestat en prosa: és una resposta seva.
		d = capDecision{Action: "self", Reply: oneLineKeepBreaks(raw, 1200)}
	}
	if force == "self" {
		d.Action = "self"
	}
	return capFinish(s, d, st, byID, ca, msg), nil
}

// capFinish valida la decisió i omple el que falti: ids existents, la
// frase d'entrega si el model no l'ha escrit, la rotllana que ja existeix
// amb aquests mateixos vents.
func capFinish(s *Server, d capDecision, st capState, byID map[string]companion.Wind, ca bool, msg string) capDecision {
	var ids []string
	seen := map[string]bool{}
	for _, id := range d.Winds {
		id = strings.TrimSpace(id)
		if _, ok := byID[id]; ok && !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	d.Winds = ids
	d.Reply = strings.TrimSpace(d.Reply)
	switch d.Action {
	case "wind":
		if len(ids) == 0 {
			d.Action = "self"
			break
		}
		d.Winds = ids[:1]
		if d.Reply == "" {
			d.Reply = capHandOver(byID[ids[0]].Name, ca)
		}
	case "rotllana":
		if len(ids) > 3 {
			d.Winds = ids[:3]
		}
		if len(d.Winds) < 2 {
			if len(d.Winds) == 1 {
				d.Action = "wind"
				if d.Reply == "" {
					d.Reply = capHandOver(byID[d.Winds[0]].Name, ca)
				}
				break
			}
			d.Action = "self"
			break
		}
		if d.Reply == "" {
			d.Reply = map[bool]string{true: "Faig rotllana: ho mirem entre uns quants 🌀", false: "Let me gather a few of them 🌀"}[ca]
		}
		d.Rotllana = s.capRotllanaWith(d.Winds)
	case "crew":
		d.Winds = nil
		if strings.TrimSpace(d.Task) == "" {
			d.Task = msg
		}
		if d.Reply == "" {
			d.Reply = map[bool]string{true: "Això és feina llarga: munto una colla que hi treballi 🛠️", false: "That's a big job: I'll put a crew on it 🛠️"}[ca]
		}
	default:
		d.Action = "self"
	}
	if d.Action == "self" {
		d.Winds = nil
		if d.Reply == "" {
			d.Reply = map[bool]string{true: "Hmm, no ho he entès del tot. M'ho tornes a dir?", false: "Hmm, I didn't quite get that. Say it again?"}[ca]
		}
	}
	return d
}

func capHandOver(name string, ca bool) string {
	if ca {
		return "Això és cosa de " + name + " 💨"
	}
	return "This one is for " + name + " 💨"
}

// capRotllanaWith torna una rotllana que ja tingui exactament aquests vents.
func (s *Server) capRotllanaWith(ids []string) string {
	list, err := s.listRotllanes()
	if err != nil {
		return ""
	}
	want := map[string]bool{}
	for _, id := range ids {
		want[id] = true
	}
	for _, rot := range list {
		if len(rot.Members) != len(want) {
			continue
		}
		all := true
		for _, m := range rot.Members {
			if !want[m] {
				all = false
				break
			}
		}
		if all {
			return rot.ID
		}
	}
	return ""
}

func capTail(h []capMsg, n int) string {
	if len(h) > n {
		h = h[len(h)-n:]
	}
	var b strings.Builder
	for _, m := range h {
		who := "user"
		if m.From == "cap" {
			who = "you"
		}
		b.WriteString(who + ": " + oneLine(m.Text, 300) + "\n")
	}
	return b.String()
}
