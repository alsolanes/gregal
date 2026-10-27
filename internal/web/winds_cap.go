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
	// Task és l'encàrrec d'una colla proposada: si l'usuari respon
	// «endavant», es fa aquest, no «endavant».
	Task string `json:"task,omitempty"`
}

type capState struct {
	Name    string   `json:"name"`
	Color   string   `json:"color"`
	Style   string   `json:"style"`
	Seen    int64    `json:"seen"` // última visita: el resum compta el que ha passat després
	History []capMsg `json:"history"`
	Pet     capPet   `json:"pet"`
}

type capDecision struct {
	Action   string   `json:"action"`
	Reply    string   `json:"reply"`
	Winds    []string `json:"winds"`
	Task     string   `json:"task,omitempty"`
	Rotllana string   `json:"rotllana,omitempty"`
	// Create: especialistes que encara no hi són i la mascota proposa crear.
	Create []crewNew `json:"create"`
	// Novices: vents triats que encara no saben res del seu ofici (ni
	// coneixements ni memòria): l'app proposa fer-los un curs accelerat.
	Novices []string `json:"novices"`
	// Confirm: l'usuari ha dit «endavant» a la proposta anterior. L'app la
	// fa sense tornar a preguntar, amb Message (la petició original) com a
	// missatge per al vent o encàrrec de la colla.
	Confirm bool   `json:"confirm,omitempty"`
	Message string `json:"message,omitempty"`
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
		now := time.Now()
		st.Seen = now.Unix()
		st.Pet.tick(now)
		s.saveCap(st)
		capMu.Unlock()
		writeJSON(w, map[string]any{"name": st.Name, "color": st.Color, "style": st.Style, "history": st.History, "brief": brief, "pet": st.Pet.json(now)})
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
		st.History = append(st.History, capMsg{From: "cap", Text: d.Reply, Ts: now, Action: d.Action, Winds: d.Winds, Task: d.Task})
		if d.Action != "self" {
			capCheer(&st.Pet, time.Now())
		}
		s.saveCap(st)
		capMu.Unlock()
		writeJSON(w, map[string]any{"action": d.Action, "reply": d.Reply, "winds": d.Winds, "task": d.Task, "rotllana": d.Rotllana,
			"create": d.Create, "novices": d.Novices, "confirm": d.Confirm, "message": d.Message,
			"history": st.History, "pet": st.Pet.json(time.Now())})
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
	case force == "":
		// «Endavant», «sí, crea-la»: la resposta a la proposta d'abans. Abans
		// anava al model com un missatge nou, que contestava «fet!» sense
		// fer res o tornava a proposar el mateix.
		if d, ok := capConfirmed(st, msg, byID, ca); ok {
			return d, nil
		}
		if w, ok := windNamedIn(winds, msg); ok && len(winds) > 0 {
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
		"You decide who should handle each request; the app carries out your decision. You cannot create, build or start anything yourself: with \"self\" you only talk, so never claim you created, started or did something. Write `reply` in the language of the user's current message (" + lang + " if unclear), warmly and briefly, with at most one emoji. Reply with JSON only, no code fences."
	var b strings.Builder
	b.WriteString("WINDS:\n" + roster.String())
	if tail := capTail(st.History, 6); tail != "" {
		b.WriteString("\nRECENT TALK WITH YOU:\n" + tail)
	}
	b.WriteString("\nUSER NOW: " + msg + "\n(Write `reply` in the same language as USER NOW, whatever language the earlier talk used.)\n\n")
	if force == "self" {
		b.WriteString("The user wants YOU to answer this yourself. action must be \"self\".\n")
	} else if force == "rotllana" {
		b.WriteString("The user wants a group chat: action must be \"rotllana\"; pick the 2 or 3 most relevant winds.\n")
	}
	b.WriteString("Pick ONE action:\n" +
		"- \"self\": greetings, small talk, quick facts, a short answer you can give in a few sentences, or when no wind fits. reply = your full answer (max 5 sentences).\n" +
		"- \"rotllana\" (the usual choice for any real question): open a chat with the SPECIALISTS of the subject. winds = the 1-3 existing winds whose role or knowledge is specifically about it (generic roles such as organisation or review never count as specialists of a subject). If the subject has no specialist, add up to 2 new specialists in `new`. reply = one short playful sentence saying who you are gathering.\n" +
		"- \"wind\": the user names one wind, or exactly one specialist fits and nothing is missing. winds = [its id]. reply = one short playful sentence handing it over (e.g. \"This one is for Marinada 🛒\").\n" +
		"- \"crew\": long work that produces something: a website, an app, a report, an analysis of data, a plan with many parts. task = the request rewritten as a clear brief. reply = one short sentence saying this needs a crew working on the server.\n" +
		"Never invent wind ids. Prefer specialists over answering yourself whenever the subject has any depth.\n" +
		"New specialists: a short, playful, wind-inspired Catalan name not used above; a role of 2-5 words in " + lang + " saying exactly what it masters; a warm persona (2 sentences, in English) stating its expertise; kit = the closest of [" + strings.Join(crewKits, ", ") + "] or \"\".\n" +
		`JSON: {"action":"…","winds":["id"],"new":[{"name":"…","role":"…","persona":"…","kit":"…"}],"task":"…","reply":"…"}`)
	raw, err := capDecider(s, ctx, []llm.Message{{Role: "system", Content: system}, {Role: "user", Content: b.String()}})
	if err != nil {
		log.Printf("cap: %s", redactError(err))
		return capDecision{}, err
	}
	var d capDecision
	var extra struct {
		New []crewNew `json:"new"`
	}
	if obj := firstJSONObject(raw); obj != "" && json.Unmarshal([]byte(obj), &extra) == nil {
		d.Create = extra.New
	}
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
	if d.Action != "wind" && d.Action != "rotllana" {
		d.Create = nil
	}
	if d.Action == "wind" || d.Action == "rotllana" {
		// Els especialistes nous passen pel mateix filtre que les colles:
		// noms i colors lliures, i si n'hi ha un amb el mateix ofici, aquest.
		if len(d.Create) > 0 {
			winds := make([]companion.Wind, 0, len(byID))
			for _, w := range byID {
				winds = append(winds, w)
			}
			reuse := make([]crewReuse, 0, len(ids))
			for _, id := range ids {
				reuse = append(reuse, crewReuse{ID: id})
			}
			p := crewSanitise(crewProposal{Plan: "-", Reuse: reuse, Create: d.Create}, winds, msg, ca)
			ids = ids[:0]
			for _, r := range p.Reuse {
				ids = append(ids, r.ID)
			}
			d.Winds, d.Create = ids, p.Create
		}
		if len(d.Create) > 2 {
			d.Create = d.Create[:2]
		}
		if len(d.Winds) > 3 {
			d.Winds = d.Winds[:3]
		}
		switch total := len(d.Winds) + len(d.Create); {
		case total == 0:
			d.Action = "self"
		case total == 1:
			d.Action = "wind"
		default:
			d.Action = "rotllana"
		}
	}
	switch d.Action {
	case "wind":
		name := ""
		if len(d.Winds) == 1 {
			name = byID[d.Winds[0]].Name
		} else if len(d.Create) == 1 {
			name = d.Create[0].Name
		}
		if d.Reply == "" {
			d.Reply = capHandOver(name, ca)
		}
	case "rotllana":
		if d.Reply == "" {
			d.Reply = map[bool]string{true: "Faig rotllana amb els que en saben 🌀", false: "Let me gather the experts 🌀"}[ca]
		}
		if len(d.Create) == 0 {
			d.Rotllana = s.capRotllanaWith(d.Winds)
		}
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
		d.Winds, d.Create = nil, nil
		if d.Reply == "" {
			d.Reply = map[bool]string{true: "Hmm, no ho he entès del tot. M'ho tornes a dir?", false: "Hmm, I didn't quite get that. Say it again?"}[ca]
		}
	}
	if d.Create == nil {
		d.Create = []crewNew{}
	}
	d.Novices = []string{}
	for _, id := range d.Winds {
		if s.windIsNovice(id) {
			d.Novices = append(d.Novices, id)
		}
	}
	return d
}

// windIsNovice: el vent encara no sap res del seu ofici, ni per estudi
// (coneixements) ni per conversa (memòria).
func (s *Server) windIsNovice(id string) bool {
	return len(s.windSkills(id)) == 0 && strings.TrimSpace(s.windBrain(id)) == ""
}

// handleCapGo fa el que la mascota ha decidit: crea els especialistes nous,
// obre el xat (un vent sol o una rotllana, la que ja hi hagi amb els mateixos
// vents) i, si l'usuari ho vol, engega el curs accelerat dels que no en saben.
//
//	POST /api/winds/cap/go {message,lang,winds,create,course} → {wind|rotllana, created, studying}
func (s *Server) handleCapGo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Message string    `json:"message"`
		Lang    string    `json:"lang"`
		Winds   []string  `json:"winds"`
		Create  []crewNew `json:"create"`
		Course  bool      `json:"course"`
	}
	if !decodeSmall(w, r, &req) {
		return
	}
	ca := req.Lang == "ca"
	if n := len(req.Winds) + len(req.Create); n == 0 || n > 4 || len(req.Create) > 2 {
		http.Error(w, "1-4 winds required", http.StatusBadRequest)
		return
	}
	windLibraryMu.Lock()
	list, err := s.winds.List()
	if err != nil {
		windLibraryMu.Unlock()
		http.Error(w, "could not read winds", http.StatusInternalServerError)
		return
	}
	reuse := []crewReuse{}
	for _, id := range req.Winds {
		reuse = append(reuse, crewReuse{ID: id})
	}
	p := crewSanitise(crewProposal{Plan: "-", Reuse: reuse, Create: req.Create}, list, req.Message, ca)
	members := []string{}
	for _, r := range p.Reuse {
		members = append(members, r.ID)
	}
	created := []companion.Wind{}
	for _, c := range p.Create {
		wind := companion.Wind{
			ID: "w-" + windRecordID(), Name: c.Name, Role: c.Role, Specialty: c.Kit,
			Color: c.Color, Style: c.Style,
			Persona: c.Persona + " Respond in the user's language. Never claim actions or checks you did not perform.",
		}
		if err := s.winds.Restore(wind); err != nil {
			windLibraryMu.Unlock()
			http.Error(w, "could not create winds", http.StatusInternalServerError)
			return
		}
		created = append(created, wind)
		members = append(members, wind.ID)
	}
	windLibraryMu.Unlock()
	if len(members) == 0 {
		http.Error(w, "unknown winds", http.StatusBadRequest)
		return
	}
	studying := []string{}
	if req.Course {
		for _, id := range members {
			wnd, ok := s.winds.ByID(id)
			if !ok || !s.windIsNovice(id) {
				continue
			}
			if e := engegaExpert(wnd.ID, wnd.Role); e != nil {
				goBackground(func() { s.executaExpert(wnd, e) })
				studying = append(studying, wnd.ID)
			}
		}
	}
	out := map[string]any{"created": created, "studying": studying}
	if len(members) == 1 {
		out["wind"] = members[0]
		writeJSON(w, out)
		return
	}
	id := s.capRotllanaWith(members)
	if id == "" {
		names := []string{}
		for _, m := range members {
			if wnd, ok := s.winds.ByID(m); ok {
				names = append(names, wnd.Name)
			}
		}
		and := " & "
		if ca {
			and = " i "
		}
		name := strings.Join(names[:len(names)-1], ", ") + and + names[len(names)-1]
		s.mu.Lock()
		workspace := s.cwd
		s.mu.Unlock()
		rot, err := s.saveRotllana(Rotllana{Name: name, Members: members, Workspace: workspace})
		if err != nil {
			http.Error(w, "could not open the conversation", http.StatusInternalServerError)
			return
		}
		id = rot.ID
	}
	out["rotllana"] = id
	writeJSON(w, out)
}

// capConfirmed: si el missatge és un «endavant» i la mascota acaba de
// proposar un camí (un vent, una rotllana, una colla), torna aquella decisió
// marcada com a confirmada, amb la petició original.
func capConfirmed(st capState, msg string, byID map[string]companion.Wind, ca bool) (capDecision, bool) {
	n := len(st.History)
	if n < 2 || !capIsConfirm(msg) {
		return capDecision{}, false
	}
	last, asked := st.History[n-1], st.History[n-2]
	if last.From != "cap" || asked.From != "user" {
		return capDecision{}, false
	}
	switch last.Action {
	case "crew":
		task := strings.TrimSpace(last.Task)
		if task == "" {
			task = asked.Text
		}
		return capDecision{Action: "crew", Task: task, Winds: []string{}, Create: []crewNew{}, Novices: []string{},
			Reply:   map[bool]string{true: "Som-hi! Munto la colla i es posen a treballar 🛠️", false: "On it! Putting the crew together 🛠️"}[ca],
			Confirm: true, Message: asked.Text}, true
	case "wind", "rotllana":
		var ids []string
		for _, id := range last.Winds {
			if _, ok := byID[id]; ok {
				ids = append(ids, id)
			}
		}
		if len(ids) == 0 {
			return capDecision{}, false
		}
		action := "rotllana"
		if len(ids) == 1 {
			action = "wind"
		}
		d := capDecision{Action: action, Winds: ids, Create: []crewNew{}, Novices: []string{}, Confirm: true, Message: asked.Text}
		if action == "wind" {
			d.Reply = capHandOver(byID[ids[0]].Name, ca)
		} else {
			d.Reply = map[bool]string{true: "Som-hi! Obro la rotllana 🌀", false: "On it! Opening the group 🌀"}[ca]
		}
		return d, true
	}
	return capDecision{}, false
}

// capConfirmWords: paraules que, soles o juntes, volen dir «fes-ho».
var capConfirmWords = map[string]bool{
	"endavant": true, "si": true, "som-hi": true, "somhi": true, "fes-ho": true, "fesho": true, "dacord": true, "d'acord": true,
	"ok": true, "okay": true, "vale": true, "va": true, "perfecte": true, "genial": true, "crea-la": true, "creala": true,
	"crea-ho": true, "engega-la": true, "engega": true, "comenca": true, "yes": true, "yep": true, "sure": true, "go": true,
	"ahead": true, "proceed": true, "adelante": true, "dale": true, "fet": true, "perfect": true, "great": true,
}

// capConfirmFiller: paraules que poden acompanyar el «sí» sense canviar-lo.
var capConfirmFiller = map[string]bool{
	"crea": true, "fes": true, "la": true, "el": true, "ho": true, "colla": true, "do": true, "it": true, "please": true,
	"sisplau": true, "siusplau": true, "i": true, "and": true, "ja": true, "ara": true, "now": true, "doncs": true, "then": true,
	"create": true, "start": true, "the": true, "crew": true, "us": true, "let's": true, "lets": true, "vinga": true, "molt": true, "be": true,
}

// capIsConfirm diu si el missatge és només un «endavant» (curt i sense res
// nou: «endavant, crea-la» sí; «sí, però fes-ho en anglès» no).
func capIsConfirm(msg string) bool {
	t := strings.ToLower(strings.TrimSpace(msg))
	if t == "" || len([]rune(t)) > 60 {
		return false
	}
	t = strings.NewReplacer("í", "i", "à", "a", "è", "e", "é", "e", "ò", "o", "ó", "o", "ú", "u", "ç", "c", "’", "'").Replace(t)
	words := strings.FieldsFunc(t, func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r == '-' || r == '\'')
	})
	strong := false
	for _, w := range words {
		w = strings.Trim(w, "-'")
		switch {
		case capConfirmWords[w]:
			strong = true
		case capConfirmFiller[w] || w == "":
		default:
			return false
		}
	}
	return strong
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
