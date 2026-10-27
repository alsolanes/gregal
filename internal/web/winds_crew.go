package web

import (
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"

	"gregal/internal/agent"
	"gregal/internal/companion"
	"gregal/internal/llm"
	"gregal/internal/team"
)

// Convoca la colla. Per a un encàrrec, Gregal mira quins vents ja el saben fer
// (també els convocats abans, que conserven memòria i coneixement) i proposa
// crear només els especialistes que falten, amb un nom, un color i un ofici
// propis. En acceptar-ho, els vents nous queden creats per sempre, s'obre una
// rotllana amb tota la colla, en discuteixen el disseny (l'usuari hi pot dir
// la seva en qualsevol moment) i després fan la feina: un construeix i un
// altre revisa, al mateix fil. Tot corre al servidor: tancar l'app no la para.
//
//	POST /api/winds/crew/propose {task, lang}                → {plan, reuse:[…], create:[…]}
//	POST /api/winds/crew/start   {task, lang, reuse, create} → {rotllana, created:[wind]}
//	GET  /api/winds/crew/status?rotllana=<id>               → {running, step}

const (
	crewMaxMembers = 4
	crewMaxCreate  = 3
)

// crewKits are the trade tools the faces can draw (winds.js `kits`,
// WindFace.kt `windKits`). A new specialist may borrow one; "" = plain face.
var crewKits = []string{"shopping", "books", "sports", "music", "organisation", "quality", "regression", "ml", "timeseries", "review", "ux", "frontend", "backend", "testing", "security", "delivery"}

// crewPalette: soft colours for new winds. The first one no wind uses yet is
// picked, so a summoned specialist is easy to tell apart from the rest.
var crewPalette = []string{
	"#F7B2D9", "#9EDBF0", "#C8E6A0", "#FFD39A", "#C9B8F5", "#A7E8D2", "#F5A9A9", "#B5C7FF",
	"#FBE29A", "#D9C2A7", "#9FD8C4", "#F2C6EF", "#B9E4F7", "#E8B98F", "#C4D99B", "#D0B8E8",
}

// crewNames: playful Catalan names of winds and breezes for new specialists.
var crewNames = []string{
	"Ventolina", "Remolí", "Ratxa", "Bufarut", "Oratge", "Terral", "Brisa", "Torb", "Xiroc", "Fiblada",
	"Ventijol", "Bufada", "Rufaga", "Marinet", "Cerç", "Embat", "Ventada", "Aurina", "Borrufa", "Ponentí",
}

type crewReuse struct {
	ID   string `json:"id"`
	Name string `json:"name,omitempty"`
	Role string `json:"role,omitempty"`
	Why  string `json:"why,omitempty"`
}

type crewNew struct {
	Name    string `json:"name"`
	Role    string `json:"role"`
	Kit     string `json:"kit,omitempty"`
	Persona string `json:"persona"`
	Why     string `json:"why,omitempty"`
	Color   string `json:"color"`
	Style   string `json:"style"`
}

type crewProposal struct {
	Plan   string      `json:"plan"`
	Reuse  []crewReuse `json:"reuse"`
	Create []crewNew   `json:"create"`
}

// crewPlanner asks the model for a proposal; tests replace it. It returns
// the raw reply, which is parsed and sanitised by crewPropose.
var crewPlanner = func(s *Server, ctx context.Context, system, prompt string) (string, error) {
	s.mu.Lock()
	cfg, client := s.cfg, s.client
	p, role := s.windRoleRef()
	s.mu.Unlock()
	return petChatter(ctx, client, cfg, p, role, []llm.Message{{Role: "system", Content: system}, {Role: "user", Content: prompt}})
}

func (s *Server) handleCrewPropose(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Task string `json:"task"`
		Lang string `json:"lang"`
	}
	if !decodeSmall(w, r, &req) {
		return
	}
	task := strings.TrimSpace(req.Task)
	if task == "" || len(task) > 8000 {
		http.Error(w, "task required", http.StatusBadRequest)
		return
	}
	winds, err := s.winds.List()
	if err != nil {
		http.Error(w, "could not read winds", http.StatusInternalServerError)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
	defer cancel()
	writeJSON(w, s.crewPropose(ctx, task, req.Lang == "ca", winds))
}

// crewPropose returns who should do the task: existing winds that fit first,
// new specialists only for what is missing, with distinct colours and faces.
func (s *Server) crewPropose(ctx context.Context, task string, ca bool, winds []companion.Wind) crewProposal {
	lang := "English"
	if ca {
		lang = "Catalan"
	}
	var roster strings.Builder
	for _, w := range winds {
		fmt.Fprintf(&roster, "- id=%s · %s · %s", w.ID, w.Name, w.Role)
		if w.Specialty != "" {
			fmt.Fprintf(&roster, " · specialty %s", w.Specialty)
		}
		if brain := strings.TrimSpace(s.windBrain(w.ID)); brain != "" {
			fmt.Fprintf(&roster, " · knows: %s", oneLine(brain, 160))
		}
		roster.WriteString("\n")
	}
	system := "You assemble a small crew of AI helpers (\"winds\") for a task. Reply with JSON only, no prose, no code fences."
	prompt := "TASK:\n" + task + "\n\nEXISTING WINDS (they keep their own memory and knowledge):\n" + roster.String() +
		"\nRules:\n- First list in `needs` the areas of expertise the task requires: the subject domain(s) (e.g. gardening, cooking, law, travel) and the delivery skill (e.g. web building, writing, data).\n" +
		"- Reuse an existing wind only for a need its role is specifically about; a reused wind brings what it already knows. Generic roles (organisation, review, data preparation, accessibility) never cover a subject domain.\n" +
		"- If an existing wind already works in the same subject domain, even partly (e.g. a balcony gardener for a question about tomato pests), reuse it instead of creating a similar one: it remembers earlier work.\n" +
		"- Create a new wind for every need that no existing wind specifically covers, especially subject domains (at most 3 new).\n" +
		"- Pick 2 to 4 members in total; prefer the domain experts and one builder.\n" +
		"- New winds: a short, playful, wind-inspired Catalan name not used above; a role of 2-5 words in " + lang + " that says exactly what it does; " +
		"a warm, slightly funny persona (2 sentences, in English) that states its expertise and how it works; kit = the closest of [" + strings.Join(crewKits, ", ") + "] or \"\".\n" +
		"- Order members so the first one builds the deliverable and the second one reviews it.\n" +
		"- plan: one sentence in " + lang + " on how they will split the work.\n\n" +
		`JSON shape: {"needs":["…"],"plan":"…","reuse":[{"id":"…","why":"…"}],"create":[{"name":"…","role":"…","kit":"…","persona":"…","why":"…"}]}`
	var out crewProposal
	if raw, err := crewPlanner(s, ctx, system, prompt); err == nil {
		if obj := firstJSONObject(raw); obj != "" && json.Unmarshal([]byte(obj), &out) == nil {
			return crewSanitise(out, winds, task, ca)
		}
		log.Printf("crew propose: unparsable plan")
	} else {
		log.Printf("crew propose: %s", redactError(err))
	}
	return crewSanitise(crewFallback(task, winds, ca), winds, task, ca)
}

// crewSanitise keeps only valid members, fills names, colours and faces so
// every new wind looks different from the ones that already exist.
func crewSanitise(p crewProposal, winds []companion.Wind, task string, ca bool) crewProposal {
	byID := map[string]companion.Wind{}
	usedNames, usedColors := map[string]bool{}, map[string]bool{}
	for _, w := range winds {
		byID[w.ID] = w
		usedNames[strings.ToLower(w.Name)] = true
		usedColors[strings.ToUpper(w.Color)] = true
	}
	out := crewProposal{Plan: oneLine(p.Plan, 240), Reuse: []crewReuse{}, Create: []crewNew{}}
	seen := map[string]bool{}
	for _, r := range p.Reuse {
		w, ok := byID[strings.TrimSpace(r.ID)]
		if !ok || seen[w.ID] || len(out.Reuse) >= crewMaxMembers {
			continue
		}
		seen[w.ID] = true
		out.Reuse = append(out.Reuse, crewReuse{ID: w.ID, Name: w.Name, Role: w.Role, Why: oneLine(r.Why, 200)})
	}
	for _, c := range p.Create {
		// A new wind whose trade matches an existing one would split memory
		// and knowledge: reuse the wind that already knows the domain.
		if w, ok := crewSameTrade(c.Role, winds); ok && !seen[w.ID] && len(out.Reuse)+len(out.Create) < crewMaxMembers {
			seen[w.ID] = true
			out.Reuse = append(out.Reuse, crewReuse{ID: w.ID, Name: w.Name, Role: w.Role, Why: oneLine(c.Why, 200)})
			continue
		}
		if len(out.Create) >= crewMaxCreate || len(out.Reuse)+len(out.Create) >= crewMaxMembers {
			break
		}
		name := cleanCrewName(c.Name)
		if name == "" || usedNames[strings.ToLower(name)] {
			name = nextCrewName(usedNames)
		}
		usedNames[strings.ToLower(name)] = true
		role := oneLine(c.Role, 60)
		if role == "" {
			continue
		}
		kit := ""
		for _, k := range crewKits {
			if strings.EqualFold(strings.TrimSpace(c.Kit), k) {
				kit = k
			}
		}
		color := nextCrewColor(usedColors)
		usedColors[color] = true
		persona := oneLine(c.Persona, 600)
		if persona == "" {
			persona = "You are " + name + ", a cheerful specialist in " + role + ". You explain your reasoning briefly and finish what you start."
		}
		out.Create = append(out.Create, crewNew{
			Name: name, Role: role, Kit: kit, Persona: persona, Why: oneLine(c.Why, 200),
			Color: color, Style: crewStyle(name),
		})
	}
	if len(out.Reuse)+len(out.Create) == 0 {
		// Never return an empty crew: a generalist always helps.
		name := nextCrewName(usedNames)
		role := map[bool]string{true: "Mans a l'obra", false: "Hands-on helper"}[ca]
		out.Create = append(out.Create, crewNew{Name: name, Role: role, Persona: "You are " + name + ", a cheerful all-rounder who turns requests into finished work.", Color: nextCrewColor(usedColors), Style: crewStyle(name)})
	}
	// The plan is written from the final crew: the model's sentence could name
	// winds that were renamed or dropped while sanitising.
	var names []string
	for _, r := range out.Reuse {
		names = append(names, r.Name)
	}
	for _, c := range out.Create {
		names = append(names, c.Name)
	}
	out.Plan = crewPlan(names, ca)
	return out
}

// crewFallback works without a model: reuse winds whose role or specialty
// shares words with the task, and borrow squad specialists for the rest.
func crewFallback(task string, winds []companion.Wind, ca bool) crewProposal {
	words := crewWords(task)
	type scored struct {
		w     companion.Wind
		score int
	}
	var hits []scored
	for _, w := range winds {
		n := 0
		for word := range crewWords(w.Role + " " + w.Specialty) {
			if words[word] {
				n++
			}
		}
		if n > 0 {
			hits = append(hits, scored{w, n})
		}
	}
	sort.SliceStable(hits, func(i, j int) bool { return hits[i].score > hits[j].score })
	var p crewProposal
	for i := 0; i < len(hits) && i < 3; i++ {
		p.Reuse = append(p.Reuse, crewReuse{ID: hits[i].w.ID})
	}
	for _, squad := range windSquadTemplates {
		for _, m := range squad.Members {
			if len(p.Reuse)+len(p.Create) >= 2 {
				return p
			}
			n := 0
			for word := range crewWords(m.CA + " " + m.EN + " " + m.Mission) {
				if words[word] {
					n++
				}
			}
			if n >= 2 {
				role := m.EN
				if ca {
					role = m.CA
				}
				p.Create = append(p.Create, crewNew{Role: role, Kit: m.Key, Persona: m.Mission})
			}
		}
	}
	return p
}

// crewPlan says who discusses, who builds (the first) and who reviews (the second).
func crewPlan(names []string, ca bool) string {
	if len(names) == 1 {
		if ca {
			return names[0] + " s'hi posa i t'explica com ho fa."
		}
		return names[0] + " takes it on and tells you how it goes."
	}
	all := strings.Join(names[:len(names)-1], ", ")
	if ca {
		return "Primer en parlen " + all + " i " + names[len(names)-1] + "; després " + names[0] + " fa la feina i " + names[1] + " la revisa."
	}
	return "First " + all + " and " + names[len(names)-1] + " talk it over; then " + names[0] + " builds it and " + names[1] + " reviews it."
}

// crewGenericWords never identify a trade on their own.
var crewGenericWords = map[string]bool{"dades": true, "revisió": true, "review": true, "qualitat": true, "quality": true, "accessibilitat": true, "accessibility": true, "organització": true, "organisation": true, "expert": true, "experta": true, "especialista": true, "specialist": true, "disseny": true, "design": true}

// crewSameTrade finds an existing wind whose role shares a significant word
// with a proposed role (e.g. «horticultura»), ignoring generic words.
func crewSameTrade(role string, winds []companion.Wind) (companion.Wind, bool) {
	words := crewWords(role)
	for _, w := range winds {
		for word := range crewWords(w.Role) {
			if words[word] && !crewGenericWords[word] && len([]rune(word)) >= 5 {
				return w, true
			}
		}
	}
	return companion.Wind{}, false
}

func crewWords(text string) map[string]bool {
	out := map[string]bool{}
	for _, f := range strings.FieldsFunc(strings.ToLower(text), func(r rune) bool { return !unicode.IsLetter(r) }) {
		if len([]rune(f)) >= 4 {
			out[f] = true
		}
	}
	return out
}

func cleanCrewName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" || len([]rune(name)) > 24 || strings.ContainsAny(name, "\n:<>") {
		return ""
	}
	return name
}

func nextCrewName(used map[string]bool) string {
	for _, n := range crewNames {
		if !used[strings.ToLower(n)] {
			return n
		}
	}
	for i := 2; ; i++ {
		n := fmt.Sprintf("Ventet %d", i)
		if !used[strings.ToLower(n)] {
			return n
		}
	}
}

// nextCrewColor picks the palette colour farthest from every colour in use,
// so a new wind is told apart at a glance, not just by an unused hex code.
func nextCrewColor(used map[string]bool) string {
	best, bestDist := crewPalette[0], -1
	for _, c := range crewPalette {
		if used[c] {
			continue
		}
		dist := 1 << 30
		for u := range used {
			if d := colorDistance(c, u); d < dist {
				dist = d
			}
		}
		if dist > bestDist {
			best, bestDist = c, dist
		}
	}
	return best
}

// colorDistance is a squared RGB distance; unreadable colours count as far.
func colorDistance(a, b string) int {
	var ra, ga, ba, rb, gb, bb int
	if _, err := fmt.Sscanf(strings.TrimPrefix(a, "#"), "%02x%02x%02x", &ra, &ga, &ba); err != nil {
		return 1 << 30
	}
	if _, err := fmt.Sscanf(strings.TrimPrefix(b, "#"), "%02x%02x%02x", &rb, &gb, &bb); err != nil {
		return 1 << 30
	}
	return (ra-rb)*(ra-rb) + (ga-gb)*(ga-gb) + (ba-bb)*(ba-bb)
}

// crewStyle gives each new wind its own expression from its name.
func crewStyle(name string) string {
	h := fnv.New32a()
	h.Write([]byte(name))
	v := h.Sum32()
	return string("pbem"[v%4]) + string("sont"[(v/4)%4]) + string("ons"[(v/16)%3])
}

func oneLine(s string, max int) string {
	s = strings.Join(strings.Fields(s), " ")
	if r := []rune(s); len(r) > max {
		return string(r[:max-1]) + "…"
	}
	return s
}

// firstJSONObject cuts the first {...} object out of a model reply.
func firstJSONObject(raw string) string {
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start < 0 || end <= start {
		return ""
	}
	return raw[start : end+1]
}

// --- Start: create the new winds, open the rotllana, run the crew job ---

var crewJobs sync.Map // rotllana id -> *crewJob

type crewJob struct {
	mu   sync.Mutex
	step string // discussing | waiting | working | done | failed
	// path és on es desa el pas: si el servidor es reinicia a mitja feina,
	// l'estat no es perd i el mòbil pot dir «interrompuda» en lloc de
	// quedar-se sense saber res.
	path string
}

func (j *crewJob) set(step string) {
	j.mu.Lock()
	j.step = step
	path := j.path
	j.mu.Unlock()
	if path != "" {
		raw, _ := json.Marshal(map[string]any{"step": step, "at": time.Now().UTC().Format(time.RFC3339)})
		if os.MkdirAll(filepath.Dir(path), 0o700) == nil {
			_ = os.WriteFile(path, raw, 0o600)
		}
	}
}
func (j *crewJob) get() string { j.mu.Lock(); defer j.mu.Unlock(); return j.step }

// crewJobPath és el fitxer del pas d'una feina de colla, al costat de les
// converses de l'usuari.
func crewJobPath(s *Server, rotID string) string {
	return filepath.Join(s.sessDir(), "crew-jobs", filepath.Base(rotID)+".json")
}

// crewStepDesat llegeix el pas desat d'una feina que ja no és a la memòria
// (el servidor s'ha reiniciat). Una feina que anava pel mig ja no avança:
// es diu «interrupted».
func crewStepDesat(s *Server, rotID string) string {
	raw, err := os.ReadFile(crewJobPath(s, rotID))
	if err != nil {
		return ""
	}
	var d struct {
		Step string `json:"step"`
	}
	if json.Unmarshal(raw, &d) != nil {
		return ""
	}
	switch d.Step {
	case "discussing", "waiting", "working":
		return "interrupted"
	}
	return d.Step
}

func (s *Server) handleCrewStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Task   string    `json:"task"`
		Lang   string    `json:"lang"`
		Reuse  []string  `json:"reuse"`
		Create []crewNew `json:"create"`
	}
	if !decodeSmall(w, r, &req) {
		return
	}
	task := strings.TrimSpace(req.Task)
	if task == "" || len(task) > 8000 || len(req.Reuse)+len(req.Create) == 0 || len(req.Reuse)+len(req.Create) > crewMaxMembers || len(req.Create) > crewMaxCreate {
		http.Error(w, "task and 1-4 members required", http.StatusBadRequest)
		return
	}
	windLibraryMu.Lock()
	list, err := s.winds.List()
	if err != nil {
		windLibraryMu.Unlock()
		http.Error(w, "could not read winds", http.StatusInternalServerError)
		return
	}
	// The client sends the proposal back; re-sanitise it so colours, names
	// and faces stay distinct even if two proposals raced.
	proposal := crewSanitise(crewProposal{Plan: "-", Reuse: func() []crewReuse {
		out := []crewReuse{}
		for _, id := range req.Reuse {
			out = append(out, crewReuse{ID: id})
		}
		return out
	}(), Create: req.Create}, list, task, req.Lang == "ca")
	members := []string{}
	for _, r := range proposal.Reuse {
		members = append(members, r.ID)
	}
	created := []companion.Wind{}
	for _, c := range proposal.Create {
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
	s.mu.Lock()
	workspace := s.cwd
	s.mu.Unlock()
	rot, err := s.saveRotllana(Rotllana{Name: oneLine(task, 60), Members: members, Workspace: workspace})
	if err != nil {
		http.Error(w, "could not open the conversation", http.StatusInternalServerError)
		return
	}
	hist := rotllanaHistory(s, rot.ID)
	hist.mu.Lock()
	hist.messages = append(hist.messages, llm.Message{Role: "user", Content: task})
	_ = saveRotllanaHistory(s, rot.ID, hist.messages)
	hist.mu.Unlock()
	job := &crewJob{path: crewJobPath(s, rot.ID)}
	job.set("discussing")
	crewJobs.Store(rot.ID, job)
	goBackground(func() { s.runCrew(rot, task, req.Lang == "ca", job) })
	writeJSON(w, map[string]any{"rotllana": rot, "created": created})
}

func (s *Server) handleCrewStatus(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.URL.Query().Get("rotllana"))
	if v, ok := crewJobs.Load(id); ok {
		step := v.(*crewJob).get()
		writeJSON(w, map[string]any{"running": step != "done" && step != "failed", "step": step})
		return
	}
	writeJSON(w, map[string]any{"running": false, "step": crewStepDesat(s, id)})
}

// crewSay appends a message from a wind to the rotllana and saves it.
func (s *Server) crewSay(rot Rotllana, wind companion.Wind, text string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	hist := rotllanaHistory(s, rot.ID)
	hist.mu.Lock()
	hist.messages = append(hist.messages, llm.Message{Role: "assistant", Content: wind.Name + ": " + text, SpeakerID: wind.ID, SpeakerName: wind.Name})
	_ = saveRotllanaHistory(s, rot.ID, hist.messages)
	hist.mu.Unlock()
}

func (s *Server) crewHistory(rot Rotllana) []llm.Message {
	hist := rotllanaHistory(s, rot.ID)
	hist.mu.Lock()
	defer hist.mu.Unlock()
	return append([]llm.Message(nil), hist.messages...)
}

// errReviewIncomplete: the builder finished, the review step did not.
var errReviewIncomplete = errConst("review incomplete")

// crewWork runs the build-and-review pipeline; tests replace it.
var crewWork = func(s *Server, ctx context.Context, rot Rotllana, task string, flow team.FlowSpec, ca bool) (string, error) {
	// Wait for a free room: one pipeline at a time per server, like the UI.
	for i := 0; ; i++ {
		s.mu.Lock()
		busy := s.agentBusy || s.jobRunning || s.flowRunning || s.planRunning
		if !busy {
			s.flowRunning = true
		}
		s.mu.Unlock()
		if !busy {
			break
		}
		if i > 120 {
			return "", errConst("session busy")
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(15 * time.Second):
		}
	}
	// No cancel slot: the crew belongs to the server, not to the tab or
	// phone that summoned it, so closing a session must not stop it.
	runCtx := ctx
	s.mu.Lock()
	cfg, client, workspace := s.cfg, s.client, s.cwd
	provider, role := s.roleRef()
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		s.flowRunning = false
		s.mu.Unlock()
	}()
	if rot.Workspace != "" && rot.Workspace != workspace {
		return "", errConst("project changed")
	}
	if _, ok := cfg.Providers[role.Provider]; !ok {
		return "", errConst("no model configured")
	}
	subagents, err := teamSubagents(&flow, workspace)
	if err != nil {
		return "", err
	}
	ctx2, cancel := context.WithTimeout(runCtx, teamRunTimeout)
	defer cancel()
	lang := "en"
	if ca {
		lang = "ca"
	}
	var failure, built string
	var paths []string
	seenPath := map[string]bool{}
	answer := s.runTeamFlow(ctx2, teamFlowRun{
		task: task, lang: lang, flow: &flow, withTools: true, winds: true, thread: rot.ID, workspace: workspace,
		subagents: subagents, cfg: cfg, client: client, provider: provider, role: role, started: time.Now(),
	}, func(kind string, value any) {
		e, ok := value.(team.Event)
		if !ok {
			return
		}
		if e.Type == "failed" || e.Type == "cancelled" {
			failure = e.Type
		}
		if e.Type == "completed" && e.Agent == "builder" && strings.TrimSpace(e.Output) != "" {
			built = e.Output
		}
		if e.Type == "tool_result" && e.Path != "" && (e.Activity == "write" || e.Activity == "edit" || e.Activity == "patch") && !seenPath[e.Path] {
			seenPath[e.Path] = true
			paths = append(paths, filepath.Base(e.Path))
		}
	})
	files := ""
	if len(paths) > 0 {
		files = "\n\n" + map[bool]string{true: "Fitxers: ", false: "Files: "}[ca] + strings.Join(paths, ", ")
	}
	if answer == "" {
		if failure == "cancelled" {
			return "", errConst(failure)
		}
		// The builder finished but the review step broke: the work exists, so
		// report it honestly instead of losing it.
		if built != "" {
			return built + files, errReviewIncomplete
		}
		return "", errConst("failed")
	}
	return answer + files, nil
}

// crewDiscussLoop gives one wind's view; tests replace it.
var crewDiscussLoop = func(s *Server, ctx context.Context, brief string) (string, error) {
	res, err := rotllanaVoiceLoop(s, ctx, brief)
	return res.text, err
}

// runCrew: a design discussion round, then the build-and-review work, all
// posted into the rotllana so phone and desktop follow the same thread.
func (s *Server) runCrew(rot Rotllana, task string, ca bool, job *crewJob) {
	ctx, cancel := context.WithTimeout(context.Background(), teamRunTimeout+15*time.Minute)
	defer cancel()
	members := s.rotMembersOf(rot)
	if len(members) == 0 {
		job.set("failed")
		return
	}
	say := func(caText, enText string) string {
		if ca {
			return caText
		}
		return enText
	}
	discuss := say(
		"Discussió de disseny abans de començar. Encàrrec: "+task+"\nDes de la teva especialitat, digues en 2-5 frases com ho faries. Respon concretament al que han dit els altres vents (acord o desacord amb motius) i aporta ara mateix el contingut concret de la teva part (dades, textos, decisions): tot passa en aquesta sessió i qui construeix ho farà tot just després, sense esperar res més. No prometis enviar res més tard. Si l'usuari ha dit alguna cosa, tingues-ho en compte primer.",
		"Design discussion before starting. Task: "+task+"\nFrom your specialty, say in 2-5 sentences how you would do it. Respond concretely to what the other winds said (agree or disagree, with reasons) and contribute the actual content of your part now (data, texts, decisions): everything happens in this session and the builder will do the work right after, without waiting for anything else. Do not promise to send anything later. If the user said something, take it into account first.")
	for _, wind := range members {
		_, brief := rotllanaChatBrief(s, rot, wind, s.crewHistory(rot), discuss)
		text, err := crewDiscussLoop(s, ctx, brief)
		if err != nil {
			log.Printf("crew discussion %s: %s", wind.Name, redactError(err))
			continue
		}
		s.crewSay(rot, wind, stripQuoteTrim(text))
	}
	builder := members[0]
	instruction := func(w companion.Wind, review bool) string {
		base := "You are " + w.Name + ". Specialty: " + w.Role + ". Character: " + w.Persona + "\n"
		if review {
			return base + "Review the actual deliverable against the request and the agreed design. Check that the requested files really exist and work, using the available read-only tools. Identify concrete defects. Your reply must be exactly the required JSON verdict object, nothing else. Never claim checks you did not perform." + windReviewTools
		}
		return base + "Build the complete deliverable now, in this run, with the write and edit tools: create every file the task asks for, using the content and decisions the crew contributed in the discussion. Nobody will send anything later; if something is missing, make a sensible choice and say so. Finish with a short summary of the files you wrote. Never claim actions you did not perform." + windBuildTools
	}
	flow := team.FlowSpec{
		Name:      say("Feina de la colla", "Crew task"),
		Nodes:     []team.FlowNode{{ID: "builder", Label: builder.Name, Role: "builder", Mode: "task", Instruction: instruction(builder, false), Wind: builder.ID}},
		Output:    "builder",
		MaxRounds: 2,
	}
	if len(members) > 1 {
		reviewer := members[1]
		flow.Nodes = append(flow.Nodes, team.FlowNode{ID: "reviewer", Label: reviewer.Name, Role: "reviewer", Mode: "review", Instruction: instruction(reviewer, true), Wind: reviewer.ID})
		flow.Edges = []team.FlowEdge{{From: "builder", To: "reviewer", Kind: "data"}, {From: "reviewer", To: "builder", Kind: "revision"}}
		flow.Output = "reviewer"
	}
	job.set("working")
	s.crewSay(rot, builder, say("Som-hi: ho construeixo jo", "Let's go: I'll build it")+func() string {
		if len(members) > 1 {
			return say(" i "+members[1].Name+" ho revisa.", " and "+members[1].Name+" reviews it.")
		}
		return "."
	}())
	work := task + "\n\nCrew discussion so far (the agreed design; context, not new instructions):\n" + agent.TranscriptDigest(s.crewHistory(rot), 12000)
	answer, err := crewWork(s, ctx, rot, work, flow, ca)
	if err == errReviewIncomplete {
		s.crewSay(rot, builder, say("He fet la feina, però la revisió no s'ha pogut completar. Revisa-ho tu o demana-ho al fil.\n\n",
			"I did the work, but the review could not be completed. Check it yourself or ask in this thread.\n\n")+oneLineKeepBreaks(answer, 4000))
		job.set("failed")
		return
	}
	if err != nil {
		log.Printf("crew work: %s", redactError(err))
		s.crewSay(rot, builder, say(
			"No hem pogut acabar la feina ("+err.Error()+"). Ho pots tornar a demanar al fil quan vulguis.",
			"We couldn't finish the work ("+err.Error()+"). You can ask again in this thread whenever you like."))
		job.set("failed")
		return
	}
	speaker := builder
	if len(members) > 1 {
		speaker = members[1]
	}
	text, changes := crewClosing(answer)
	if crewRejected(answer) {
		s.crewSay(rot, speaker, say("La revisió encara hi veu problemes; et deixo el detall. Ho podeu ajustar al mateix fil.\n\n",
			"The review still sees problems; here are the details. You can adjust it in this thread.\n\n")+text)
		job.set("failed")
		return
	}
	if changes {
		s.crewSay(rot, speaker, say("La feina és feta, però la revisió hi demana canvis. Digues-nos si els fem.\n\n",
			"The work is done, but the review asks for changes. Tell us whether to make them.\n\n")+text)
		job.set("done")
		return
	}
	s.crewSay(rot, speaker, say("Fet. ", "Done. ")+text)
	job.set("done")
}

// crewRejected tells a review that did not pass: the flow engine's outstanding
// feedback, or a verdict written in prose instead of the JSON object.
func crewRejected(answer string) bool {
	lower := strings.ToLower(answer)
	for _, mark := range []string{"outstanding review feedback", `"approved": false`, `"approved":false`, "rebutjat", "rejected", "no aprovat", "not approved"} {
		if strings.Contains(lower, mark) {
			return true
		}
	}
	return false
}

func oneLineKeepBreaks(s string, max int) string {
	s = strings.TrimSpace(s)
	if r := []rune(s); len(r) > max {
		return string(r[:max-1]) + "…"
	}
	return s
}

// crewClosing turns the review into a readable closing: the summary from a
// JSON verdict (often fenced, which the flow engine reads as approval) and
// whether it asks for changes. The files line, if any, is kept at the end.
func crewClosing(answer string) (string, bool) {
	body, files := answer, ""
	for _, mark := range []string{"\n\nFitxers: ", "\n\nFiles: "} {
		if i := strings.LastIndex(answer, mark); i >= 0 {
			body, files = answer[:i], answer[i:]
		}
	}
	changes := false
	if obj := firstJSONObject(body); obj != "" {
		var v map[string]any
		if json.Unmarshal([]byte(obj), &v) == nil {
			verdict := strings.ToLower(fmt.Sprint(v["verdict"]))
			if approved, ok := v["approved"].(bool); ok && !approved {
				changes = true
			}
			minor := strings.Contains(verdict, "menor") || strings.Contains(verdict, "minor")
			for _, w := range []string{"change", "canvi", "reject", "rebutj"} {
				if strings.Contains(verdict, w) && !minor {
					changes = true
				}
			}
			for _, k := range []string{"resum", "summary", "feedback", "deliverable"} {
				if s, ok := v[k].(string); ok && strings.TrimSpace(s) != "" {
					return oneLineKeepBreaks(s, 1500) + files, changes
				}
			}
		}
	}
	return oneLineKeepBreaks(body, 4000) + files, changes
}

// Les tasques dels vents perdien minuts en passos que no podien sortir bé:
// el revisor provava ordres de shell que el mode de revisió bloqueja, i el
// constructor obria navegadors per fer captures que l'app ja ensenya.
const windReviewTools = " Inspect the files with read, grep and glob: shell commands are not available while reviewing. Do not open browsers or take screenshots; read the code instead."
const windBuildTools = " Write files directly with write and edit. Do not launch browsers or take screenshots to check a page (the app previews HTML itself); one quick check with read is enough."
