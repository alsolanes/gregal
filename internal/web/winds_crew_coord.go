package web

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"gregal/internal/agent"
	"gregal/internal/companion"
	"gregal/internal/llm"
	"gregal/internal/team"
)

// La colla amb coordinació de debò. Abans era un guió fix (una ronda de
// discussió, el primer construeix i el segon revisa); ara, després de la
// discussió, un coordinador decideix a cada volta què toca:
//
//   - work: qui fa quina part i amb quins fitxers. Les parts van en
//     paral·lel i un membre les revisa totes; si la revisió deixa res obert,
//     el coordinador ho veu i decideix si cal una altra volta.
//   - ask: una pregunta concreta a l'usuari. La colla s'atura ("waiting"),
//     l'estat queda al disc i la resposta de l'usuari al fil la reprèn.
//   - finish: la conclusió, els fitxers, què s'ha comprovat i què no.
//   - giveup: no es pot fer, i per què.
//
// Al fil, la discussió i el progrés porten kind "debate" (l'app els plega),
// la pregunta "question" i el resultat "result".

// crewWrites: els fitxers que els constructors han escrit de debò, per fil.
// El resultat els llista; abans es llistava tot el que havia canviat a la
// carpeta (fitxers interns de Gregal inclosos).
var (
	crewWritesMu sync.Mutex
	crewWrites   = map[string][]string{}
)

func noteCrewWrite(rotID, workspace, path string) {
	rel := path
	if workspace != "" {
		if r, err := filepath.Rel(workspace, path); err == nil && !strings.HasPrefix(r, "..") {
			rel = r
		}
	}
	rel = filepath.ToSlash(rel)
	crewWritesMu.Lock()
	defer crewWritesMu.Unlock()
	for _, p := range crewWrites[rotID] {
		if p == rel {
			return
		}
	}
	crewWrites[rotID] = append(crewWrites[rotID], rel)
}

func takeCrewWrites(rotID string) []string {
	crewWritesMu.Lock()
	defer crewWritesMu.Unlock()
	out := crewWrites[rotID]
	delete(crewWrites, rotID)
	return out
}

const (
	crewMaxRounds    = 3 // voltes de feina
	crewMaxQuestions = 2
)

// crewState és el que el coordinador necessita per reprendre la feina
// després d'una pregunta (també si el servidor s'ha reiniciat).
type crewState struct {
	Task      string    `json:"task"`
	Ca        bool      `json:"ca"`
	Round     int       `json:"round"`
	Questions int       `json:"questions"`
	Started   time.Time `json:"started"`
	Waiting   bool      `json:"waiting"`
	Log       []string  `json:"log"` // què ha passat a cada volta, per al coordinador
}

func crewStatePath(s *Server, rotID string) string {
	return filepath.Join(s.sessDir(), "crew-jobs", filepath.Base(rotID)+".state.json")
}

func saveCrewState(s *Server, rotID string, st *crewState) {
	raw, err := json.Marshal(st)
	if err != nil {
		return
	}
	p := crewStatePath(s, rotID)
	if os.MkdirAll(filepath.Dir(p), 0o700) == nil {
		_ = os.WriteFile(p, raw, 0o600)
	}
}

func loadCrewState(s *Server, rotID string) (*crewState, bool) {
	raw, err := os.ReadFile(crewStatePath(s, rotID))
	if err != nil {
		return nil, false
	}
	var st crewState
	if json.Unmarshal(raw, &st) != nil || st.Task == "" {
		return nil, false
	}
	return &st, true
}

// crewSayKind és crewSay amb el tipus de missatge del fil.
func (s *Server) crewSayKind(rot Rotllana, wind companion.Wind, kind, text string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	hist := rotllanaHistory(s, rot.ID)
	hist.mu.Lock()
	hist.messages = append(hist.messages, llm.Message{Role: "assistant", Content: wind.Name + ": " + text,
		SpeakerID: wind.ID, SpeakerName: wind.Name, Ts: time.Now().Unix(), Kind: kind})
	_ = saveRotllanaHistory(s, rot.ID, hist.messages)
	hist.mu.Unlock()
}

// crewAssignment és una part de la feina d'una volta.
type crewAssignment struct {
	Wind  string   `json:"wind"`
	Part  string   `json:"part"`
	Files []string `json:"files"`
}

// crewDecision és el que decideix el coordinador.
type crewDecision struct {
	Action      string           `json:"action"` // work | ask | finish | giveup
	Say         string           `json:"say"`
	Assignments []crewAssignment `json:"assignments"`
	Question    string           `json:"question"`
	Conclusion  string           `json:"conclusion"`
	Checked     []string         `json:"checked"`
	Unchecked   []string         `json:"unchecked"`
	// Assumed: el que la colla s'ha inventat i només sap l'usuari.
	Assumed []string `json:"assumed"`
}

// cleanList treu els elements buits o massa llargs d'una llista del model.
func cleanList(in []string) []string {
	var out []string
	for _, s := range in {
		if s = oneLine(s, 200); s != "" {
			out = append(out, s)
		}
	}
	return out
}

// crewCoordinator és la crida al model; els tests la substitueixen.
var crewCoordinator = func(s *Server, ctx context.Context, system, prompt string) (string, error) {
	s.mu.Lock()
	cfg, client := s.cfg, s.client
	p, role := s.windRoleRef()
	s.mu.Unlock()
	return petChatter(ctx, client, cfg, p, role, []llm.Message{{Role: "system", Content: system}, {Role: "user", Content: prompt}})
}

// crewDecide demana al coordinador el pas següent i el deixa net: membres
// que existeixen, fitxers sense repetir, límits de voltes i preguntes.
func (s *Server) crewDecide(ctx context.Context, rot Rotllana, st *crewState, members []companion.Wind) crewDecision {
	lang := "English"
	if st.Ca {
		lang = "Catalan"
	}
	var roster strings.Builder
	for _, w := range members {
		fmt.Fprintf(&roster, "- %s | %s | %s\n", w.ID, w.Name, w.Role)
	}
	crewWritesMu.Lock()
	files := append([]string(nil), crewWrites[rot.ID]...)
	crewWritesMu.Unlock()
	system := "You coordinate a small crew of AI specialists (\"winds\") working on a task for the user. After the crew's design discussion and after every round of work you decide the single next step. Reply with JSON only, no prose, no code fences."
	var b strings.Builder
	b.WriteString("TASK:\n" + st.Task + "\n\nCREW:\n" + roster.String())
	b.WriteString("\nTHREAD SO FAR (discussion, work notes and any user answers):\n" + agent.TranscriptDigest(s.crewHistory(rot), 9000))
	if len(st.Log) > 0 {
		b.WriteString("\n\nROUNDS DONE:\n" + strings.Join(st.Log, "\n\n"))
	}
	if len(files) > 0 {
		b.WriteString("\n\nFILES WRITTEN SO FAR: " + strings.Join(files, ", "))
	}
	fmt.Fprintf(&b, "\n\nWork rounds used: %d of %d. Questions asked: %d of %d.\n", st.Round, crewMaxRounds, st.Questions, crewMaxQuestions)
	b.WriteString("Choose ONE action:\n" +
		"- \"work\": the next round. assignments = 1-4 members working AT THE SAME TIME, each with a concrete part and the files only that member writes (relative paths; never the same file twice). Give everyone the shared decisions inside their part (names, formats, interfaces). After the round one member reviews everything.\n" +
		"- \"ask\": when the result depends on facts only the user knows (how many people, dates, budget, names, their own data, who it is for) and the discussion had to invent or guess them, or when two incompatible choices both fit. Ask before the first round rather than build on invented facts. question = one short, concrete question in " + lang + " that gathers everything missing at once, with options or defaults if useful. Never ask what the crew can decide, look up or reasonably choose itself (style, structure, wording).\n" +
		"- \"finish\": the deliverable exists and the last review left nothing important open, or the remaining issues are minor. conclusion = 3-6 sentences in " + lang + " for the user: what was done and the key decisions. checked = what the review actually verified (only things the review says it checked). unchecked = what nobody verified or is still open.\n" +
		"- \"giveup\": only if it truly cannot be done; say = why, in " + lang + ".\n" +
		"Repeat work only to fix what the review left open. say = one short sentence in " + lang + " for the thread about this step.\n" +
		"assumed = every fact the crew invented or guessed that only the user could know (number of people, budget, dates, names, their own data), each as a short phrase in " + lang + "; [] if none. The user has not answered anything yet unless the thread shows it.\n" +
		`JSON: {"action":"…","say":"…","assumed":["…"],"assignments":[{"wind":"id","part":"…","files":["…"]}],"question":"…","conclusion":"…","checked":["…"],"unchecked":["…"]}`)
	// Un segon intent si la resposta no porta JSON: sense decisió, la colla
	// cauria al pla per defecte (un sol membre ho fa tot, sense preguntar).
	var d crewDecision
	prompt := b.String()
	for try := 0; try < 2; try++ {
		raw, err := crewCoordinator(s, ctx, system, prompt)
		if err != nil {
			log.Printf("crew coordinator: %s", redactError(err))
			continue
		}
		obj := firstJSONObject(raw)
		if obj != "" && json.Unmarshal([]byte(obj), &d) == nil {
			break
		}
		d = crewDecision{}
		log.Printf("crew coordinator: resposta sense JSON (%d caràcters): %s", len(raw), oneLine(raw, 160))
		prompt = b.String() + "\n\nYour previous reply was not valid JSON. Reply with the JSON object only."
	}
	return crewSanitiseDecision(d, st, members)
}

// crewSanitiseDecision aplica els límits i omple el que falti, perquè la
// colla avanci sempre: sense res vàlid, la primera volta construeix el
// primer membre i les següents acaben.
func crewSanitiseDecision(d crewDecision, st *crewState, members []companion.Wind) crewDecision {
	d.Action = strings.ToLower(strings.TrimSpace(d.Action))
	byID := map[string]companion.Wind{}
	for _, w := range members {
		byID[w.ID] = w
	}
	// Abans de la primera volta, si la colla s'ha hagut d'inventar dades que
	// només sap l'usuari, es pregunta (el model tendeix a tirar endavant).
	if st.Round == 0 && st.Questions == 0 && d.Action != "giveup" && len(cleanList(d.Assumed)) > 0 {
		if strings.TrimSpace(d.Question) == "" {
			lead := "Before we start, confirm or correct what we assumed: "
			if st.Ca {
				lead = "Abans de començar, confirma o corregeix el que hem suposat: "
			}
			d.Question = lead + strings.Join(cleanList(d.Assumed), "; ") + "."
		}
		d.Action = "ask"
	}
	if d.Action == "ask" && (st.Questions >= crewMaxQuestions || strings.TrimSpace(d.Question) == "") {
		d.Action = ""
	}
	if d.Action == "work" && st.Round >= crewMaxRounds {
		d.Action = "finish"
	}
	if d.Action == "work" {
		seenWind, seenFile := map[string]bool{}, map[string]bool{}
		var ok []crewAssignment
		for _, a := range d.Assignments {
			w, exists := byID[strings.TrimSpace(a.Wind)]
			if !exists || seenWind[w.ID] || len(ok) >= 4 {
				continue
			}
			seenWind[w.ID] = true
			var fs []string
			for _, f := range a.Files {
				f = filepath.ToSlash(strings.TrimSpace(f))
				if f == "" || seenFile[strings.ToLower(f)] || strings.Contains(f, "..") {
					continue
				}
				seenFile[strings.ToLower(f)] = true
				fs = append(fs, f)
			}
			ok = append(ok, crewAssignment{Wind: w.ID, Part: oneLineKeepBreaks(a.Part, 1500), Files: fs})
		}
		d.Assignments = ok
		if len(ok) == 0 {
			d.Action = ""
		}
	}
	switch d.Action {
	case "work", "ask", "finish", "giveup":
	default:
		if st.Round == 0 && len(members) > 0 {
			d = crewDecision{Action: "work", Assignments: []crewAssignment{{Wind: members[0].ID, Part: st.Task}}}
		} else {
			d = crewDecision{Action: "finish"}
		}
	}
	d.Say = oneLine(d.Say, 240)
	d.Question = oneLineKeepBreaks(d.Question, 600)
	d.Conclusion = oneLineKeepBreaks(d.Conclusion, 2000)
	return d
}

// crewResultText és el missatge final: conclusió, fitxers, què s'ha
// comprovat i què no.
func crewResultText(d crewDecision, files []string, fallback string, ca bool) string {
	say := func(c, e string) string {
		if ca {
			return c
		}
		return e
	}
	var b strings.Builder
	conclusion := strings.TrimSpace(d.Conclusion)
	if conclusion == "" {
		conclusion = strings.TrimSpace(fallback)
	}
	if conclusion == "" {
		conclusion = say("Feina acabada.", "Work finished.")
	}
	b.WriteString("**" + say("Conclusió", "Conclusion") + "**\n" + conclusion)
	list := func(title string, items []string) {
		var clean []string
		for _, it := range items {
			if it = oneLine(it, 300); it != "" {
				clean = append(clean, it)
			}
		}
		if len(clean) == 0 {
			return
		}
		b.WriteString("\n\n**" + title + "**")
		for _, it := range clean {
			b.WriteString("\n- " + it)
		}
	}
	list(say("Fitxers", "Files"), files)
	list(say("Comprovat", "Checked"), d.Checked)
	list(say("Sense comprovar", "Not checked"), d.Unchecked)
	return b.String()
}

// crewRoundFlow munta la volta: un constructor per part (en paral·lel) i un
// revisor que ho mira tot; si la revisió deixa coses obertes, la resposta
// les porta ("Outstanding review feedback") i el coordinador hi decideix.
func crewRoundFlow(assign []crewAssignment, members []companion.Wind, instruction func(companion.Wind, bool) string, ca bool) team.FlowSpec {
	byID := map[string]companion.Wind{}
	for _, w := range members {
		byID[w.ID] = w
	}
	flow := team.FlowSpec{Name: map[bool]string{true: "Feina de la colla", false: "Crew task"}[ca], MaxRounds: 1}
	building := map[string]bool{}
	for i, a := range assign {
		w := byID[a.Wind]
		building[w.ID] = true
		part := "\n\nYOUR PART (other members build the rest at the same time): " + a.Part
		if len(a.Files) > 0 {
			part += "\nFiles you own (create or edit only these; the others belong to other members): " + strings.Join(a.Files, ", ")
		}
		flow.Nodes = append(flow.Nodes, team.FlowNode{ID: fmt.Sprintf("build-%d", i+1), Label: w.Name, Role: "builder", Mode: "task",
			Instruction: instruction(w, false) + part, Wind: w.ID})
	}
	if len(members) > 1 {
		// Revisa qui no construeix (si tothom construeix, el primer de la colla).
		reviewer := members[0]
		for _, w := range members {
			if !building[w.ID] {
				reviewer = w
				break
			}
		}
		flow.Nodes = append(flow.Nodes, team.FlowNode{ID: "review", Label: reviewer.Name, Role: "reviewer", Mode: "review",
			Instruction: instruction(reviewer, true) + " In feedback, say what you actually checked (files opened, behaviour verified) and what you could not check.",
			Wind: reviewer.ID})
		for i := range assign {
			flow.Edges = append(flow.Edges, team.FlowEdge{From: fmt.Sprintf("build-%d", i+1), To: "review", Kind: "data"})
		}
		flow.Output = "review"
	} else {
		flow.Output = "build-1"
	}
	return flow
}

// crewLoop és la feina coordinada: decideix, fa una volta, torna a decidir,
// fins que acaba, pregunta o es rendeix.
func (s *Server) crewLoop(rot Rotllana, st *crewState, job *crewJob) {
	ctx, cancel := context.WithTimeout(context.Background(), teamRunTimeout+15*time.Minute)
	defer cancel()
	members := s.rotMembersOf(rot)
	if len(members) == 0 {
		job.set("failed")
		return
	}
	lead := members[0]
	say := func(c, e string) string {
		if st.Ca {
			return c
		}
		return e
	}
	instruction := func(w companion.Wind, review bool) string {
		base := "You are " + w.Name + ". Specialty: " + w.Role + ". Character: " + w.Persona + "\n"
		if review {
			return base + "Review the actual deliverable against the request and the agreed design. Check that the requested files really exist and work, using the available read-only tools. Identify concrete defects. Your reply must be exactly the required JSON verdict object, nothing else. Never claim checks you did not perform." + windReviewTools
		}
		return base + "Build your part of the deliverable now, in this run, with the write and edit tools, using the content and decisions the crew contributed. Nobody will send anything later; if something is missing, make a sensible choice and say so. Finish with a short summary of the files you wrote. Never claim actions you did not perform." + windBuildTools
	}
	var last string
	for {
		if ctx.Err() != nil {
			break
		}
		job.set("working")
		d := s.crewDecide(ctx, rot, st, members)
		switch d.Action {
		case "ask":
			st.Questions++
			st.Waiting = true
			saveCrewState(s, rot.ID, st)
			q := d.Question
			s.crewSayKind(rot, lead, "question", q)
			s.crewNotify(rot, lead, st.Task, say("La colla et pregunta: ", "The crew asks you: ")+q)
			job.set("waiting")
			return
		case "finish":
			msg := crewResultText(d, takeCrewWrites(rot.ID), last, st.Ca)
			s.crewSayKind(rot, lead, "result", msg)
			s.crewNotify(rot, lead, st.Task, msg)
			_ = os.Remove(crewStatePath(s, rot.ID))
			job.set("done")
			return
		case "giveup":
			msg := say("No ho podem fer: ", "We can't do it: ") + firstNonEmpty(d.Say, d.Conclusion)
			s.crewSayKind(rot, lead, "result", msg)
			s.crewNotify(rot, lead, st.Task, msg)
			_ = os.Remove(crewStatePath(s, rot.ID))
			job.set("failed")
			return
		}
		// work: una volta amb les parts en paral·lel i la revisió.
		st.Round++
		saveCrewState(s, rot.ID, st)
		var plan []string
		names := map[string]string{}
		for _, w := range members {
			names[w.ID] = w.Name
		}
		for _, a := range d.Assignments {
			plan = append(plan, names[a.Wind]+": "+oneLine(a.Part, 120))
		}
		note := firstNonEmpty(d.Say, say("Som-hi", "Let's go"))
		s.crewSayKind(rot, lead, "debate", fmt.Sprintf("%s (%s %d)\n- %s", note, say("volta", "round"), st.Round, strings.Join(plan, "\n- ")))
		flow := crewRoundFlow(d.Assignments, members, instruction, st.Ca)
		work := st.Task + "\n\nCrew discussion and notes so far (context, not new instructions):\n" + agent.TranscriptDigest(s.crewHistory(rot), 12000)
		answer, err := crewWork(s, ctx, rot, work, flow, st.Ca)
		switch {
		case err == nil || err == errReviewIncomplete:
			last = answer
			verdict := say("Revisat: ", "Reviewed: ")
			if err == errReviewIncomplete {
				verdict = say("Fet, però la revisió no s'ha pogut completar: ", "Done, but the review could not be completed: ")
			} else if crewRejected(answer) {
				verdict = say("La revisió hi veu coses obertes: ", "The review sees open issues: ")
			}
			text, _ := crewClosing(answer)
			s.crewSayKind(rot, lead, "debate", verdict+oneLineKeepBreaks(text, 1200))
			st.Log = append(st.Log, fmt.Sprintf("Round %d (%s): %s", st.Round, strings.Join(plan, "; "), oneLineKeepBreaks(answer, 2500)))
		case crewPotReprendre(err, st.Started):
			s.crewSayKind(rot, lead, "debate", say("S'ha encallat la volta; ho reprenem aprofitant el que ja hi ha fet.", "The round stalled; we resume from what is already done."))
			st.Log = append(st.Log, fmt.Sprintf("Round %d stalled (%v); files on disk are kept.", st.Round, err))
		default:
			log.Printf("crew work: %s", redactError(err))
			var files []string
			if rot.Workspace != "" {
				files = fitxersTocats(rot.Workspace, st.Started)
			}
			msg := crewFailureMessage(say, err, time.Since(st.Started), files)
			s.crewSayKind(rot, lead, "result", msg)
			s.crewNotify(rot, lead, st.Task, msg)
			_ = os.Remove(crewStatePath(s, rot.ID))
			job.set("failed")
			return
		}
		saveCrewState(s, rot.ID, st)
	}
	msg := say("S'ha acabat el temps de la colla; et deixo el que hi ha.", "The crew ran out of time; here is what exists.")
	s.crewSayKind(rot, lead, "result", msg+"\n\n"+oneLineKeepBreaks(last, 3000))
	s.crewNotify(rot, lead, st.Task, msg)
	_ = os.Remove(crewStatePath(s, rot.ID))
	job.set("failed")
}

// crewTakeAnswer: si la colla d'aquest fil espera resposta, el missatge de
// l'usuari la reprèn (en lloc d'anar al xat de sempre). Torna true si l'ha
// agafat; el missatge ja és al fil.
func (s *Server) crewTakeAnswer(rot Rotllana, msg string) bool {
	st, ok := loadCrewState(s, rot.ID)
	if !ok || !st.Waiting {
		return false
	}
	job := &crewJob{path: crewJobPath(s, rot.ID)}
	if v, loaded := crewJobs.LoadOrStore(rot.ID, job); loaded {
		existing := v.(*crewJob)
		if step := existing.get(); step != "waiting" && step != "" {
			return false // ja hi ha feina en marxa
		}
		job = existing
	}
	hist := rotllanaHistory(s, rot.ID)
	hist.mu.Lock()
	hist.messages = append(hist.messages, llm.Message{Role: "user", Content: msg, Ts: time.Now().Unix()})
	_ = saveRotllanaHistory(s, rot.ID, hist.messages)
	hist.mu.Unlock()
	st.Waiting = false
	st.Log = append(st.Log, "The user answered the crew's question: "+oneLineKeepBreaks(msg, 1500))
	saveCrewState(s, rot.ID, st)
	job.set("working")
	goBackground(func() { s.crewLoop(rot, st, job) })
	return true
}
