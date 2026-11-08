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
		fmt.Fprintf(&roster, "- id=%s · %s · %s", w.ID, w.Name, companion.RolePrompt(w.Role))
		if w.Specialty != "" {
			fmt.Fprintf(&roster, " · specialty %s", w.Specialty)
		}
		if brain := strings.TrimSpace(s.windBrain(w.ID)); brain != "" {
			fmt.Fprintf(&roster, " · knows: %s", oneLine(brain, 160))
		}
		roster.WriteString("\n")
	}
	nameLang := "English"
	if ca {
		nameLang = "Catalan"
	}
	system := "You assemble a small crew of AI helpers (\"winds\") for a task. Reply with JSON only, no prose, no code fences."
	prompt := "TASK:\n" + task + "\n\nEXISTING WINDS (they keep their own memory and knowledge):\n" + roster.String() +
		"\nRules:\n- First list in `needs` the areas of expertise the task requires: the subject domain(s) (e.g. gardening, cooking, law, travel) and the delivery skill (e.g. web building, writing, data).\n" +
		"- Reuse an existing wind only for a need its role is specifically about; a reused wind brings what it already knows. Generic roles (organisation, review, data preparation, accessibility) never cover a subject domain.\n" +
		"- If an existing wind already works in the same subject domain, even partly (e.g. a balcony gardener for a question about tomato pests), reuse it instead of creating a similar one: it remembers earlier work.\n" +
		"- Create a new wind for every need that no existing wind specifically covers, especially subject domains (at most 3 new).\n" +
		"- Pick 2 to 4 members in total; prefer the domain experts and one builder.\n" +
		"- New winds: " + s.collectiveNameRule(nameLang) + ", not used above; a role of 2-5 words in " + lang + " that says exactly what it does; " +
		"a warm, slightly funny persona (2 sentences, in English) that states its expertise and how it works; kit = the closest of [" + strings.Join(crewKits, ", ") + "] or \"\".\n" +
		"- Order members so the first one builds the deliverable and the second one reviews it.\n" +
		"- plan: one sentence in " + lang + " on how they will split the work.\n\n" +
		`JSON shape: {"needs":["…"],"plan":"…","reuse":[{"id":"…","why":"…"}],"create":[{"name":"…","role":"…","kit":"…","persona":"…","why":"…"}]}`
	var out crewProposal
	if raw, err := crewPlanner(s, ctx, system, prompt); err == nil {
		if obj := firstJSONObject(raw); obj != "" && json.Unmarshal([]byte(obj), &out) == nil {
			return crewSanitise(out, winds, task, ca, s.crewNamePool(ctx, ca)...)
		}
		log.Printf("crew propose: unparsable plan")
	} else {
		log.Printf("crew propose: %s", redactError(err))
	}
	return crewSanitise(crewFallback(task, winds, ca), winds, task, ca, s.crewNamePool(ctx, ca)...)
}

// crewSanitise keeps only valid members, fills names, colours and faces so
// every new wind looks different from the ones that already exist.
func crewSanitise(p crewProposal, winds []companion.Wind, task string, ca bool, pool ...string) crewProposal {
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
			name = nextCrewName(usedNames, pool...)
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
		name := nextCrewName(usedNames, pool...)
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

func nextCrewName(used map[string]bool, pool ...string) string {
	if len(pool) == 0 {
		pool = crewNames
	}
	for _, n := range pool {
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

var crewJobs sync.Map // owner/session + rotllana id -> *crewJob

// crewRegistryKey scopes in-memory crew state to the session that owns a
// rotllana. Rotllana IDs are unique only inside a session directory.
func crewRegistryKey(s *Server, rotID string) string {
	return filepath.Clean(s.sessDir()) + "\x00" + rotID
}

type crewJob struct {
	mu            sync.Mutex
	step          string                  // discussing | waiting | working | done | failed
	err           error                   // volatile terminal failure when durable status replacement fails
	beforeReplace func(step string) error // optional fault injection for persistence tests
	// path és on es desa el pas: si el servidor es reinicia a mitja feina,
	// l'estat no es perd i el mòbil pot dir «interrompuda» en lloc de
	// quedar-se sense saber res.
	path string
}

func (j *crewJob) set(step string) (retErr error) {
	defer func() {
		if retErr != nil {
			log.Printf("crew job-step persistence step=%s: %s", step, redactError(retErr))
		}
	}()
	j.mu.Lock()
	defer j.mu.Unlock()
	path := j.path
	if path == "" {
		j.step = step
		j.err = nil
		return nil
	}
	raw, err := json.Marshal(map[string]any{"step": step, "at": time.Now().UTC().Format(time.RFC3339)})
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".crew-job-*.tmp")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if err := f.Chmod(0o600); err != nil {
		f.Close()
		return err
	}
	if _, err := f.Write(raw); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if j.beforeReplace != nil {
		if err := j.beforeReplace(step); err != nil {
			return err
		}
	}
	if err := os.Rename(tmp, path); err != nil {
		return err
	}
	if err := syncDir(filepath.Dir(path)); err != nil {
		return err
	}
	// Publish the in-memory status only after the durable replacement succeeds.
	j.step = step
	j.err = nil
	return nil
}
func (j *crewJob) fail(err error) {
	if err == nil {
		return
	}
	j.mu.Lock()
	j.err = err
	j.mu.Unlock()
	log.Printf("crew job terminal failure: %s", redactError(err))
}
func crewJobStep(job *crewJob, step string) bool {
	if err := job.set(step); err != nil {
		job.fail(err)
		return false
	}
	return true
}
func crewJobFailed(job *crewJob) { crewJobStep(job, "failed") }
func (j *crewJob) get() string {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.err != nil {
		return "failed"
	}
	return j.step
}

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
		if !os.IsNotExist(err) {
			log.Printf("crew job status read rotllana=%s: %s", rotID, redactError(err))
			return "interrupted"
		}
		return ""
	}
	var d struct {
		Step string `json:"step"`
	}
	if json.Unmarshal(raw, &d) != nil {
		log.Printf("crew job status decode rotllana=%s: invalid JSON", rotID)
		return "interrupted"
	}
	switch d.Step {
	case "waiting":
		if st, ok := loadCrewState(s, rotID); ok && st.Waiting {
			return "waiting"
		}
		return "interrupted"
	case "discussing", "working":
		return "interrupted"
	}
	return d.Step
}

// crewRecoverInterrupted es crida en arrencar: una colla que va quedar a mig
// fer (el servidor s'ha reiniciat o el procés va morir) ha de dir-ho al fil
// amb un missatge final net, no quedar en silenci amb el debat plegat i cap
// conclusió. Les feines que esperen resposta de l'usuari es conserven.
// Recorre totes les sessions (la default i la de cada usuari), perquè les
// rotllanes viuen a la carpeta de qui les va crear.
func (s *Server) crewRecoverInterrupted() {
	s.crewRecoverInterruptedSession(s)
	if s.users == nil || !s.users.Actiu() {
		return
	}
	for _, name := range s.users.Noms() {
		u := s.users.Usuari(name)
		if u == nil || u.Tothom {
			continue
		}
		sess := s.hub.SessionFor(u, "boot")
		if sess != nil {
			sess.crewRecoverInterruptedSession(sess)
		}
	}
}

func (s *Server) crewRecoverInterruptedSession(sess *Server) {
	states, err := filepath.Glob(filepath.Join(sess.sessDir(), "crew-jobs", "*.state.json"))
	if err != nil {
		log.Printf("crew recovery state scan: %s", redactError(err))
		return
	}
	if len(states) == 0 {
		return
	}
	for _, p := range states {
		rotID := strings.TrimSuffix(filepath.Base(p), ".state.json")
		st, ok, stateErr := readCrewState(sess, rotID)
		if stateErr != nil {
			log.Printf("crew recovery state read rotllana=%s: %s", rotID, redactError(stateErr))
			continue
		}
		if !ok {
			continue
		}
		pendingResult := st.PendingResult
		if pendingResult == "" && crewStepDesat(sess, rotID) != "interrupted" {
			continue
		}
		rot, ok := sess.rotllanaByID(rotID)
		if !ok {
			// El fil ja no existeix: l'estat orfe no serveix de res.
			if err := removeCrewRunState(sess, rotID); err != nil {
				log.Printf("crew orphan cleanup rotllana=%s: %s", rotID, redactError(err))
			}
			continue
		}
		members := sess.rotMembersOf(rot)
		if len(members) == 0 {
			log.Printf("crew recovery has no valid member rotllana=%s; keeping state for retry", rotID)
			continue
		}
		if err := reconcileCrewWriteIntents(sess, rot, st); err != nil {
			log.Printf("crew recovery write-intent reconciliation rotllana=%s: %s", rotID, redactError(err))
			continue
		}
		lead := members[0]
		say := func(c, e string) string {
			if st.Ca {
				return c
			}
			return e
		}
		if pendingResult != "" {
			missing := []string{}
			for _, file := range st.Files {
				if !strings.Contains(pendingResult, file) {
					missing = append(missing, file)
				}
			}
			if len(missing) > 0 {
				pendingResult += "\n\n**" + say("Fitxers", "Files") + "**\n- " + strings.Join(missing, "\n- ")
				st.PendingResult = pendingResult
				if err := saveCrewState(sess, rotID, st); err != nil {
					log.Printf("crew recovery result checkpoint rotllana=%s: %s", rotID, redactError(err))
					continue
				}
			}
			if err := sess.crewSayKindRun(rot, lead, "result", pendingResult, st.RunID); err != nil {
				log.Printf("crew recovery result persistence rotllana=%s: %s", rotID, redactError(err))
				continue
			}
			step := st.TerminalStep
			if step == "" {
				step = "failed"
			}
			job := &crewJob{path: crewJobPath(sess, rotID)}
			if err := job.set(step); err != nil {
				log.Printf("crew recovery status persistence rotllana=%s: %s", rotID, redactError(err))
				continue
			}
		} else {
			files := crewFilesForResult(sess, rot.ID, st)
			msg := say("La colla es va quedar a mig fer (el servidor s'ha reiniciat); el que ja estava fet es conserva.", "The crew was interrupted mid-run (the server restarted); work already done is kept.")
			if len(files) > 0 {
				msg += "\n\n**" + say("Fitxers", "Files") + "**\n- " + strings.Join(files, "\n- ")
			}
			if err := sess.publishCrewStateResult(rot, st, lead, "failed", msg); err != nil {
				log.Printf("crew recovery result persistence rotllana=%s: %s", rotID, redactError(err))
				continue
			}
			job := &crewJob{path: crewJobPath(sess, rotID)}
			if err := job.set("failed"); err != nil {
				log.Printf("crew recovery status persistence rotllana=%s: %s", rotID, redactError(err))
				continue
			}
		}
		if err := removeCrewRunState(sess, rotID); err != nil {
			log.Printf("crew recovery cleanup rotllana=%s: %s", rotID, redactError(err))
		}
	}
}

func (s *Server) handleCrewStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	if !s.crewMayResume() {
		http.Error(w, "crew is unavailable in read-only mode", http.StatusForbidden)
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
	workspace, _ := s.ventWorkspaceLocked()
	s.mu.Unlock()
	rot, err := s.saveRotllana(Rotllana{Name: oneLine(task, 60), Members: members, Workspace: workspace})
	if err != nil {
		// Sense la causa, l'app només pot dir "torna-ho a provar" i ningú sap
		// què falla (directori, permisos, nom...). El detall va al journal i
		// també a la resposta, que és el que l'usuari veu al mòbil.
		qui := ""
		if s.user != nil {
			qui = s.user.Name
		}
		log.Printf("colla: no s'ha pogut obrir la conversa (usuari=%q workspace=%q reuse=%d create=%d membres=%v): %v",
			qui, workspace, len(req.Reuse), len(req.Create), members, err)
		http.Error(w, "could not open the conversation: "+err.Error(), http.StatusInternalServerError)
		return
	}
	hist := rotllanaHistory(s, rot.ID)
	hist.mu.Lock()
	next := append([]llm.Message(nil), hist.messages...)
	next = append(next, llm.Message{Role: "user", Content: task, Ts: time.Now().Unix()})
	if err := saveRotllanaHistory(s, rot.ID, next); err != nil {
		hist.mu.Unlock()
		log.Printf("crew initial history persistence rotllana=%s: %s", rot.ID, redactError(err))
		if cleanupErr := s.deleteRotllana(rot.ID); cleanupErr != nil {
			log.Printf("crew cleanup after history persistence failure rotllana=%s: %s", rot.ID, redactError(cleanupErr))
		}
		http.Error(w, "could not save the crew conversation", http.StatusInternalServerError)
		return
	}
	hist.messages = podaHistorialVent(next)
	hist.mu.Unlock()
	job := &crewJob{path: crewJobPath(s, rot.ID)}
	if err := job.set("discussing"); err != nil {
		if cleanupErr := s.deleteRotllana(rot.ID); cleanupErr != nil {
			log.Printf("crew cleanup after job persistence failure rotllana=%s: %s", rot.ID, redactError(cleanupErr))
		}
		http.Error(w, "could not persist crew job status", http.StatusInternalServerError)
		return
	}
	clearCrewRun(s, rot.ID)
	crewJobs.Store(crewRegistryKey(s, rot.ID), job)
	goBackground(func() { s.runCrew(rot, task, req.Lang == "ca", job) })
	writeJSON(w, map[string]any{"rotllana": rot, "created": created})
}

func (s *Server) handleCrewStatus(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.URL.Query().Get("rotllana"))
	if v, ok := crewJobs.Load(crewRegistryKey(s, id)); ok {
		step := v.(*crewJob).get()
		writeJSON(w, map[string]any{"running": step != "done" && step != "failed" && step != "cancelled", "step": step})
		return
	}
	writeJSON(w, map[string]any{"running": false, "step": crewStepDesat(s, id)})
}

// crewSay appends a message from a wind to the rotllana and saves it.
func (s *Server) crewSay(rot Rotllana, wind companion.Wind, text string) error {
	return s.crewSayKind(rot, wind, "debate", text)
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
	cfg, client := s.cfg, s.client
	// Una colla que fa feina per a la persona treballa a la carpeta de
	// projectes, no a l'arrel del repo del Gregal.
	workspace, _ := s.ventWorkspaceLocked()
	provider, role := s.roleRef()
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		s.flowRunning = false
		s.mu.Unlock()
	}()
	// Una colla creada quan la carpeta era la de l'usuari ara treballa a la de
	// projectes: no és un canvi de projecte.
	if rot.Workspace != "" && rot.Workspace != workspace && !esCarpetaMassaAmpla(rot.Workspace) {
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
	var failure, built, motiu string
	reviewInvalid := false
	var publishErr error
	var publishErrMu sync.Mutex
	var paths []string
	var pathsMu sync.Mutex
	seenPath := map[string]bool{}
	ctx2 = teamWithWriteProgress(ctx2, func(path string) error {
		if err := noteCrewWriteDurable(s, rot.ID, workspace, path); err != nil {
			crewWriteErrors.Store(crewRegistryKey(s, rot.ID), err.Error())
			log.Printf("crew state write-list persistence: %s", redactError(err))
			return err
		}
		pathsMu.Lock()
		defer pathsMu.Unlock()
		if !seenPath[path] {
			seenPath[path] = true
			paths = append(paths, filepath.Base(path))
		}
		return nil
	})
	answer := s.runTeamFlow(ctx2, teamFlowRun{
		task: task, lang: lang, flow: &flow, withTools: true, winds: true, thread: rot.ID, workspace: workspace,
		subagents: subagents, cfg: cfg, client: client, provider: provider, role: role, started: time.Now(),
		// Steer: missatges escrits mentre la colla treballa (bústia del torn
		// "rotllana" que crewLoop ha obert amb beginWindTurn).
		steer: func() []string { return windSteerDrain(turnKey{scope: s.sessDir(), kind: "rotllana", id: rot.ID}) },
	}, func(kind string, value any) {
		e, ok := value.(team.Event)
		if !ok {
			return
		}
		if e.Type == "failed" || e.Type == "cancelled" {
			failure = e.Type
			if e.Message != "" {
				motiu = e.Message
			}
		}
		if e.Type == "completed" && e.ReviewInvalid {
			// The reviewer never produced a usable verdict: the deliverable
			// is the builder's fallback, not a review-verified approval.
			reviewInvalid = true
		}
		if e.Type == "completed" && strings.TrimSpace(e.Output) != "" {
			if err := s.crewPublishNode(rot, flow, e); err != nil {
				publishErrMu.Lock()
				if publishErr == nil {
					publishErr = err
				}
				publishErrMu.Unlock()
				cancel()
			}
		}
		if e.Type == "completed" && (e.Agent == "builder" || strings.HasPrefix(e.Agent, "build-")) && strings.TrimSpace(e.Output) != "" {
			if built != "" {
				built += "\n\n"
			}
			built += e.Output
		}
	})
	publishErrMu.Lock()
	persistErr := publishErr
	publishErrMu.Unlock()
	if persistErr != nil {
		return "", fmt.Errorf("could not persist crew progress history: %w", persistErr)
	}
	if v, ok := crewWriteErrors.Load(crewRegistryKey(s, rot.ID)); ok {
		return "", fmt.Errorf("could not persist crew write list: %s", v)
	}
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
		if motiu != "" {
			return "", errConst(motiu)
		}
		return "", errConst("failed")
	}
	if reviewInvalid {
		// Honest reporting: the work exists but no review verified it.
		return built + answer + files, errReviewIncomplete
	}
	return answer + files, nil
}

// crewNotify deixa la frase final d'una feina d'equip a les novetats: el
// mòbil la mostra com a notificació (també amb l'app tancada) i, en tocar-la,
// obre el fil de la feina.
func (s *Server) crewNotify(rot Rotllana, wind companion.Wind, task, text string) {
	recordNovetat(s, wind, "feina:"+rot.ID, oneLine(task, 80)+"\n"+oneLineKeepBreaks(text, 500))
}

// crewMaxIntents és quants cops s'intenta la construcció abans de donar la
// feina per no acabada. Dos: el primer intent i una represa aprofitant el que
// ja hi ha al disc.
const crewMaxIntents = 2

// crewPotReprendre diu si val la pena tornar-ho a intentar: només quan la
// feina s'ha aturat per temps o per un model encallat (no per un error del
// projecte ni per un stop), i només si encara queda marge dins el run.
func crewPotReprendre(err error, inici time.Time) bool {
	if err == nil {
		return false
	}
	switch err.Error() {
	case "timeout", "stalled":
	default:
		return false
	}
	return time.Since(inici) < teamRunTimeout
}

// runCrew posts the coordinated work and review to the shared thread.
func (s *Server) runCrew(rot Rotllana, task string, ca bool, job *crewJob) {
	ctx, cancel := context.WithTimeout(context.Background(), teamRunTimeout+15*time.Minute)
	defer cancel()
	ctx, doneTurn := s.beginWindTurn("rotllana", rot.ID, ctx)
	defer doneTurn()
	s.runCrewContext(ctx, rot, task, ca, job)
}

func (s *Server) runCrewContext(ctx context.Context, rot Rotllana, task string, ca bool, job *crewJob) {
	inici := time.Now()
	members := s.rotMembersOf(rot)
	if len(members) == 0 {
		crewJobFailed(job)
		return
	}
	st := &crewState{Task: task, Ca: ca, Started: inici, RunID: fmt.Sprintf("%d", inici.UnixNano())}
	if len(members) < 2 {
		st.ReviewIncomplete = true
	}
	if err := saveCrewState(s, rot.ID, st); err != nil {
		log.Printf("crew initial state persistence rotllana=%s: %s", rot.ID, redactError(err))
		crewJobFailed(job)
		return
	}
	s.crewLoopContext(ctx, rot, st, job)
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

// Les tasques dels vents perdien minuts en passos que no podien sortir bé:
// el revisor provava ordres de shell que el mode de revisió bloqueja, i el
// constructor obria navegadors per fer captures que l'app ja ensenya.
const windReviewTools = " Inspect the files with read, grep and glob: shell commands are not available while reviewing. Do not open browsers or take screenshots; read the code instead."
const windBuildTools = " Write files directly with write and edit. Do not launch browsers or take screenshots to check a page (the app previews HTML itself); one quick check with read is enough."

// crewFailureMessage explica una feina que no s'ha acabat: què ha passat, què
// queda fet i què pot fer l'usuari. Un «no s'ha pogut» a seques no serveix de
// res quan la feina ja portava mitja hora.
func crewFailureMessage(say func(caText, enText string) string, err error, duracio time.Duration, fitxers []string) string {
	minuts := int(duracio.Round(time.Minute).Minutes())
	if minuts < 1 {
		minuts = 1
	}
	var msg string
	switch err.Error() {
	case "timeout":
		msg = say(
			"Ens hem quedat sense temps: la feina ha passat de "+fmt.Sprint(minuts)+" min i l'he aturada perquè no es mengi el servidor. El que ja està fet queda desat.",
			"We ran out of time: the work passed "+fmt.Sprint(minuts)+" min, so I stopped it. Whatever was already done is saved.")
	case "stalled":
		msg = say(
			"El model ha deixat de respondre (més de "+fmt.Sprint(int(teamIdleTimeout.Minutes()))+" min sense cap senyal) i he aturat la feina. El que ja està fet queda desat.",
			"The model stopped responding (over "+fmt.Sprint(int(teamIdleTimeout.Minutes()))+" min with no signal) and I stopped the work. Whatever was already done is saved.")
	default:
		msg = say(
			"No hem pogut acabar la feina ("+err.Error()+").",
			"We couldn't finish the work ("+err.Error()+").")
	}
	if len(fitxers) > 0 {
		msg += say("\n\nFitxers tocats: ", "\n\nFiles touched: ") + strings.Join(fitxers, ", ")
	}
	// No prometem represes que no existeixen: el que sí que funciona és
	// tornar a engegar la colla (aprofita el que ja hi ha al disc) o demanar
	// un encàrrec més petit.
	msg += say("\n\nJa he intentat reprendre-la un cop. Pots tornar a engegar la colla amb el mateix encàrrec (repren des del que ja hi ha fet) o partir-ho en encàrrecs més petits.",
		"\n\nI already retried once. You can start the crew again with the same task (it resumes from what is already done) or split it into smaller tasks.")
	return msg
}

// fitxersTocats llista els fitxers del projecte modificats des que va començar
// la feina: és la prova tangible que hi ha feina feta encara que el run caigui.
func fitxersTocats(workspace string, desDe time.Time) []string {
	if strings.TrimSpace(workspace) == "" {
		return nil
	}
	saltar := map[string]bool{".git": true, "node_modules": true, "build": true, ".gradle": true, "vendor": true, "dist": true, ".idea": true}
	var out []string
	_ = filepath.WalkDir(workspace, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if saltar[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		info, err := d.Info()
		if err != nil || info.ModTime().Before(desDe) {
			return nil
		}
		rel, err := filepath.Rel(workspace, p)
		if err != nil {
			rel = filepath.Base(p)
		}
		out = append(out, rel)
		if len(out) >= 8 {
			return filepath.SkipAll
		}
		return nil
	})
	sort.Strings(out)
	return out
}
