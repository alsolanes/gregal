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
	crewWritesMu         sync.Mutex
	crewWrites           = map[string][]string{}
	crewWriteEvents      = map[string]uint64{}
	crewWriteErrors      sync.Map // scoped registry key -> error string from a failed durable write-list update
	crewStateLocks       sync.Map // scoped registry key -> *sync.Mutex; never held across another run's disk I/O
	crewReservationLocks sync.Map // scoped key -> *sync.Mutex; serializes start/answer reservations per thread
)

func crewReservationLock(s *Server, rotID string) *sync.Mutex {
	v, _ := crewReservationLocks.LoadOrStore(crewRegistryKey(s, rotID), &sync.Mutex{})
	return v.(*sync.Mutex)
}

func crewStateLock(s *Server, rotID string) *sync.Mutex {
	v, _ := crewStateLocks.LoadOrStore(crewRegistryKey(s, rotID), &sync.Mutex{})
	return v.(*sync.Mutex)
}

func noteCrewWrite(s *Server, rotID, workspace, path string) {
	rel := path
	if workspace != "" {
		if r, err := filepath.Rel(workspace, path); err == nil && !strings.HasPrefix(r, "..") {
			rel = r
		}
	}
	rel = filepath.ToSlash(rel)
	key := crewRegistryKey(s, rotID)
	crewWritesMu.Lock()
	defer crewWritesMu.Unlock()
	crewWriteEvents[key]++
	for _, p := range crewWrites[key] {
		if p == rel {
			return
		}
	}
	crewWrites[key] = append(crewWrites[key], rel)
}

func crewWriteSnapshot(s *Server, rotID string) []string {
	crewWritesMu.Lock()
	defer crewWritesMu.Unlock()
	return append([]string(nil), crewWrites[crewRegistryKey(s, rotID)]...)
}

func clearCrewRun(s *Server, rotID string) {
	key := crewRegistryKey(s, rotID)
	crewWritesMu.Lock()
	delete(crewWrites, key)
	delete(crewWriteEvents, key)
	crewWritesMu.Unlock()
	crewWriteErrors.Delete(key)
}

func crewFilesForResult(s *Server, rotID string, st *crewState) []string {
	files := crewWriteSnapshot(s, rotID)
	if st == nil {
		return files
	}
	seen := make(map[string]bool, len(files)+len(st.Files))
	for _, p := range files {
		seen[p] = true
	}
	for _, p := range st.Files {
		if p != "" && !seen[p] {
			files = append(files, p)
			seen[p] = true
		}
	}
	return files
}

// noteCrewWriteDurable records the actual successful edit both in the fast
// in-memory registry and in the per-user crew state. A restart must not erase
// the list the recovery message needs to report.
func noteCrewWriteDurable(s *Server, rotID, workspace, path string) error {
	rel, err := crewRelativePath(workspace, path)
	if err != nil {
		return err
	}
	root, err := os.OpenRoot(workspace)
	if err != nil {
		return err
	}
	defer root.Close()
	if err := rejectRootSymlinks(root, filepath.FromSlash(rel)); err != nil {
		return err
	}
	digest, exists, err := rootDigest(root, filepath.FromSlash(rel))
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("committed crew file %s is missing", rel)
	}
	return recordCrewWriteCommittedDurable(s, rotID, rel, digest)
}

func crewRelativePath(workspace, path string) (string, error) {
	if filepath.IsAbs(path) {
		if workspace == "" {
			return "", fmt.Errorf("workspace missing for absolute crew path")
		}
		rel, err := filepath.Rel(workspace, path)
		if err != nil || !filepath.IsLocal(rel) {
			return "", fmt.Errorf("crew write path is outside its workspace")
		}
		path = rel
	}
	rel := filepath.Clean(path)
	if rel == "." || !filepath.IsLocal(rel) {
		return "", fmt.Errorf("invalid crew write path %q", path)
	}
	return filepath.ToSlash(rel), nil
}

func noteCrewWriteIntentDurable(s *Server, rotID, workspace, rel, digest string) error {
	clean, err := crewRelativePath(workspace, rel)
	if err != nil {
		return err
	}
	if digest == "" {
		return fmt.Errorf("crew write intent is missing a digest for %s", clean)
	}
	lock := crewStateLock(s, rotID)
	lock.Lock()
	defer lock.Unlock()
	st, ok, err := readCrewState(s, rotID)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("crew state missing while recording intent %s", clean)
	}
	if st.PendingWrites == nil {
		st.PendingWrites = map[string]string{}
	}
	if st.CommittedWrites == nil {
		st.CommittedWrites = map[string]string{}
	}
	st.PendingWrites[clean] = digest
	return writeCrewStateFile(s, rotID, st)
}

func noteCrewWriteCommittedDurable(s *Server, rotID, workspace, rel string) error {
	clean, err := crewRelativePath("", rel)
	if err != nil {
		return err
	}
	root, err := os.OpenRoot(workspace)
	if err != nil {
		return err
	}
	defer root.Close()
	if err := rejectRootSymlinks(root, filepath.FromSlash(clean)); err != nil {
		return err
	}
	digest, exists, err := rootDigest(root, filepath.FromSlash(clean))
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("committed crew file %s is missing", clean)
	}
	return recordCrewWriteCommittedDurable(s, rotID, clean, digest)
}

func recordCrewWriteCommittedDurable(s *Server, rotID, clean, digest string) error {
	lock := crewStateLock(s, rotID)
	lock.Lock()
	defer lock.Unlock()
	st, ok, err := readCrewState(s, rotID)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("crew state missing while recording %s", clean)
	}
	noteCrewWrite(s, rotID, "", clean)
	for _, existing := range st.Files {
		if existing == clean {
			break
		}
	}
	if !containsString(st.Files, clean) {
		st.Files = append(st.Files, clean)
	}
	if st.PendingWrites != nil {
		delete(st.PendingWrites, clean)
	}
	if st.CommittedWrites == nil {
		st.CommittedWrites = map[string]string{}
	}
	st.CommittedWrites[clean] = digest
	return writeCrewStateFile(s, rotID, st)
}

func containsString(xs []string, x string) bool {
	for _, have := range xs {
		if have == x {
			return true
		}
	}
	return false
}

func reconcileCrewWriteIntents(s *Server, rot Rotllana, st *crewState) error {
	if st == nil || len(st.PendingWrites) == 0 {
		return nil
	}
	lock := crewStateLock(s, rot.ID)
	lock.Lock()
	defer lock.Unlock()
	current, ok, err := readCrewState(s, rot.ID)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("crew state missing while reconciling writes")
	}
	st = current
	workspace := rot.Workspace
	if workspace == "" {
		s.mu.Lock()
		workspace, _ = s.ventWorkspaceLocked()
		s.mu.Unlock()
	}
	if workspace == "" {
		return fmt.Errorf("workspace missing while reconciling crew writes")
	}
	if st.CommittedWrites == nil {
		st.CommittedWrites = map[string]string{}
	}
	root, err := os.OpenRoot(workspace)
	if err != nil {
		return err
	}
	defer root.Close()
	for rel, expected := range st.PendingWrites {
		clean, pathErr := crewRelativePath("", rel)
		if pathErr != nil {
			return pathErr
		}
		relPath := filepath.FromSlash(clean)
		if err := rejectRootSymlinks(root, relPath); err != nil {
			return err
		}
		actual, exists, readErr := rootDigest(root, relPath)
		if readErr != nil && !os.IsNotExist(readErr) {
			return readErr
		}
		if exists && actual == expected {
			if !containsString(st.Files, rel) {
				st.Files = append(st.Files, rel)
			}
			st.CommittedWrites[rel] = expected
			noteCrewWrite(s, rot.ID, "", rel)
		}
	}
	return writeCrewStateFile(s, rot.ID, st)
}

const (
	// Una web amb un joc no hi cabia en 3 voltes: la colla s'aturava a mitges.
	crewMaxRounds = 6 // voltes de feina
	// Una sola pregunta: la segona demanava el que la persona ja havia dit que es busqués.
	crewMaxQuestions = 1
)

// crewState és el que el coordinador necessita per reprendre la feina
// després d'una pregunta (també si el servidor s'ha reiniciat).
type crewState struct {
	Task             string            `json:"task"`
	Ca               bool              `json:"ca"`
	Round            int               `json:"round"`
	Questions        int               `json:"questions"`
	Started          time.Time         `json:"started"`
	RunID            string            `json:"run_id"`
	Waiting          bool              `json:"waiting"`
	PendingAnswer    string            `json:"pending_answer,omitempty"`
	PendingResult    string            `json:"pending_result,omitempty"`
	TerminalStep     string            `json:"terminal_step,omitempty"`
	Log              []string          `json:"log"` // què ha passat a cada volta, per al coordinador
	Last             string            `json:"last,omitempty"`
	Files            []string          `json:"files,omitempty"`
	PendingWrites    map[string]string `json:"pending_writes,omitempty"`
	CommittedWrites  map[string]string `json:"committed_writes,omitempty"`
	ReviewIncomplete bool              `json:"review_incomplete,omitempty"`
}

func crewStatePath(s *Server, rotID string) string {
	return filepath.Join(s.sessDir(), "crew-jobs", filepath.Base(rotID)+".state.json")
}

func saveCrewState(s *Server, rotID string, st *crewState) error {
	lock := crewStateLock(s, rotID)
	lock.Lock()
	defer lock.Unlock()
	return saveCrewStateUnlocked(s, rotID, st)
}

func saveCrewStateUnlocked(s *Server, rotID string, st *crewState) error {
	// Files is provenance: every state save merges prior durable and observed
	// paths so stale snapshots from another part of the run cannot erase it.
	old, exists, readErr := readCrewState(s, rotID)
	if readErr != nil {
		return fmt.Errorf("read existing crew state: %w", readErr)
	}
	if exists && (old.RunID == st.RunID || old.RunID == "" || st.RunID == "") {
		seen := map[string]bool{}
		for _, p := range st.Files {
			seen[p] = true
		}
		for _, p := range old.Files {
			if p != "" && !seen[p] {
				st.Files = append(st.Files, p)
				seen[p] = true
			}
		}
		// The disk snapshot is authoritative for WAL metadata. Coordinator state
		// may be stale while an integration callback records a newer digest.
		st.PendingWrites = cloneCrewDigestMap(old.PendingWrites)
		st.CommittedWrites = cloneCrewDigestMap(old.CommittedWrites)
	}
	for _, p := range crewWriteSnapshot(s, rotID) {
		found := false
		for _, have := range st.Files {
			if have == p {
				found = true
				break
			}
		}
		if !found {
			st.Files = append(st.Files, p)
		}
	}
	return writeCrewStateFile(s, rotID, st)
}

func cloneCrewDigestMap(src map[string]string) map[string]string {
	if len(src) == 0 {
		return nil
	}
	out := make(map[string]string, len(src))
	for k, v := range src {
		out[k] = v
	}
	return out
}

// writeCrewStateFile writes an already-current state snapshot. Callers that
// mutate PendingWrites/CommittedWrites use this only while holding the run lock.
func writeCrewStateFile(s *Server, rotID string, st *crewState) error {
	raw, err := json.Marshal(st)
	if err != nil {
		return err
	}
	p := crewStatePath(s, rotID)
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(p), ".crew-state-*.tmp")
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
	if err := os.Rename(tmp, p); err != nil {
		return err
	}
	return syncDir(filepath.Dir(p))
}

func loadCrewState(s *Server, rotID string) (*crewState, bool) {
	st, ok, _ := readCrewState(s, rotID)
	return st, ok
}

func readCrewState(s *Server, rotID string) (*crewState, bool, error) {
	raw, err := os.ReadFile(crewStatePath(s, rotID))
	if os.IsNotExist(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	var st crewState
	if err := json.Unmarshal(raw, &st); err != nil {
		return nil, true, fmt.Errorf("decode crew state: %w", err)
	}
	if st.Task == "" {
		return nil, true, fmt.Errorf("crew state has empty task")
	}
	return &st, true, nil
}

// crewSayKind és crewSay amb el tipus de missatge del fil.
func (s *Server) crewSayKind(rot Rotllana, wind companion.Wind, kind, text string) error {
	return s.crewSayKindRun(rot, wind, kind, text, "")
}

func (s *Server) crewSayKindRun(rot Rotllana, wind companion.Wind, kind, text, runID string) error {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	hist := rotllanaHistory(s, rot.ID)
	hist.mu.Lock()
	defer hist.mu.Unlock()
	next := append([]llm.Message(nil), hist.messages...)
	if kind == "result" {
		if runID != "" {
			for _, existing := range next {
				if existing.Role == "assistant" && existing.Kind == kind && existing.SpeakerID == wind.ID && existing.CrewRunID == runID {
					hist.messages = podaHistorialVent(next)
					return nil
				}
			}
		} else if len(next) > 0 {
			existing := next[len(next)-1]
			if existing.Role == "assistant" && existing.Kind == kind && existing.SpeakerID == wind.ID && existing.Content == wind.Name+": "+text {
				hist.messages = podaHistorialVent(next)
				return nil
			}
		}
	}

	next = append(next, llm.Message{Role: "assistant", Content: wind.Name + ": " + text,
		SpeakerID: wind.ID, SpeakerName: wind.Name, Ts: time.Now().Unix(), Kind: kind, CrewRunID: runID})
	if err := saveRotllanaHistory(s, rot.ID, next); err != nil {
		return err
	}
	hist.messages = podaHistorialVent(next)
	return nil
}

func (s *Server) crewPublishResult(rot Rotllana, lead companion.Wind, msg string) bool {
	if err := s.crewSayKindRun(rot, lead, "result", msg, ""); err != nil {
		log.Printf("crew result persistence rotllana=%s: %s", rot.ID, redactError(err))
		return false
	}
	return true
}

// publishCrewStateResult checkpoints the terminal message before writing it.
// A restart can safely retry because crewSayKind deduplicates exact results.
func (s *Server) publishCrewStateResult(rot Rotllana, st *crewState, lead companion.Wind, step, msg string) error {
	lock := crewStateLock(s, rot.ID)
	lock.Lock()
	defer lock.Unlock()
	if latest, ok, err := readCrewState(s, rot.ID); err == nil && ok {
		if st.PendingWrites == nil {
			st.PendingWrites = map[string]string{}
		}
		for path, digest := range latest.PendingWrites {
			if _, have := st.PendingWrites[path]; !have {
				st.PendingWrites[path] = digest
			}
		}
		if st.CommittedWrites == nil {
			st.CommittedWrites = map[string]string{}
		}
		for path, digest := range latest.CommittedWrites {
			if _, have := st.CommittedWrites[path]; !have {
				st.CommittedWrites[path] = digest
			}
		}
		for path, digest := range st.CommittedWrites {
			if pending, have := st.PendingWrites[path]; have && pending == digest {
				delete(st.PendingWrites, path)
			}
		}
	}
	st.PendingResult = msg
	st.TerminalStep = step
	if err := saveCrewStateUnlocked(s, rot.ID, st); err != nil {
		return err
	}
	if err := s.crewSayKindRun(rot, lead, "result", msg, st.RunID); err != nil {
		return err
	}
	return nil
}

func (s *Server) finishCrewRun(rot Rotllana, st *crewState, lead companion.Wind, job *crewJob, step, msg string) error {
	if err := s.publishCrewStateResult(rot, st, lead, step, msg); err != nil {
		return err
	}
	s.crewNotify(rot, lead, st.Task, msg)
	if err := job.set(step); err != nil {
		job.fail(err)
		return err
	}
	if err := removeCrewRunState(s, rot.ID); err != nil {
		return err
	}
	return nil
}

func removeCrewRunState(s *Server, rotID string) error {
	if err := os.Remove(crewStatePath(s, rotID)); err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := syncDir(filepath.Dir(crewStatePath(s, rotID))); err != nil && !os.IsNotExist(err) {
		return err
	}
	clearCrewRun(s, rotID)
	return nil
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
	// Publish: en acabar, la carpeta (relativa al projecte) amb index.html que
	// el servidor publica; els seients treballen en còpies i no poden publicar.
	Publish *crewPublishReq `json:"publish"`
	// Options: respostes curtes a la pregunta, que l'app mostra com a botons.
	Options    []string `json:"options"`
	Conclusion string   `json:"conclusion"`
	Checked    []string `json:"checked"`
	Unchecked  []string `json:"unchecked"`
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
		fmt.Fprintf(&roster, "- %s | %s | %s\n", w.ID, w.Name, companion.RolePrompt(w.Role))
	}
	files := crewWriteSnapshot(s, rot.ID)
	system := "You coordinate a small crew of AI specialists (\"winds\") working on a task for the user. After the crew's design discussion and after every round of work you decide the single next step. Reply with JSON only, no prose, no code fences."
	var b strings.Builder
	b.WriteString("TASK:\n" + st.Task + "\n\nCREW:\n" + roster.String())
	b.WriteString("\nTHREAD SO FAR (discussion, work notes and any user answers):\n" + agent.TranscriptDigestSpeakers(s.crewHistory(rot), 9000))
	if len(st.Log) > 0 {
		b.WriteString("\n\nROUNDS DONE:\n" + strings.Join(st.Log, "\n\n"))
	}
	if len(files) > 0 {
		b.WriteString("\n\nFILES WRITTEN SO FAR: " + strings.Join(files, ", "))
	}
	fmt.Fprintf(&b, "\n\nWork rounds used: %d of %d. Questions asked: %d of %d.\n", st.Round, crewMaxRounds, st.Questions, crewMaxQuestions)
	b.WriteString("Choose ONE action:\n" +
		"- \"work\": the next round. assignments = 1-4 members working AT THE SAME TIME, each with a concrete part and the files only that member writes (relative paths; never the same file twice). Give everyone the shared decisions inside their part (names, formats, interfaces). If the crew has more than one member, leave one unassigned to review the result independently.\n" +
		"- \"ask\": when the result depends on facts only the user knows (how many people, dates, budget, names, their own data, who it is for) and the discussion had to invent or guess them, or when two incompatible choices both fit. Ask before the first round rather than build on invented facts. question = ONE short, concrete question in " + lang + " about the single missing decision that changes the result most; for every other missing detail the crew picks a sensible default (put them in assumed). options = 2-4 short concrete answers to that question (at most 40 characters each, in " + lang + ") that the user can tap as buttons; include a \"you decide\" or \"use placeholders\" style answer when it fits. Never ask what the crew can decide, look up or reasonably choose itself (style, structure, wording), and never ask about anything the user already told the crew to look up or to decide: do it in the next round.\n" +
		"- \"finish\": the deliverable exists and the last review left nothing important open, or the remaining issues are minor. A website, app or game is finished only when it is built and checked (it loads and its main interaction works); until then choose \"work\" with the next concrete step. Members cannot publish themselves and need no hosting accounts or deploy files (no Netlify, Vercel, GitHub Pages): for a static site set publish = {\"dir\": the project folder with index.html, relative to the working folder, \"title\": a short title} and the server publishes it and adds the link to the conclusion. conclusion = 3-6 sentences in " + lang + " for the user: what was done and the key decisions. checked = what the review actually verified (only things the review says it checked). unchecked = what nobody verified or is still open.\n" +
		"- \"giveup\": only if it truly cannot be done; say = why, in " + lang + ".\n" +
		"Repeat work only to fix what the review left open. say = one short sentence in " + lang + " for the thread about this step.\n" +
		"assumed = every fact the crew invented or guessed that only the user could know (number of people, budget, dates, names, their own data), each as a short phrase in " + lang + "; [] if none. The user has not answered anything yet unless the thread shows it.\n" +
		`JSON: {"action":"…","say":"…","assumed":["…"],"assignments":[{"wind":"id","part":"…","files":["…"]}],"question":"…","options":["…"],"publish":{"dir":"…","title":"…"},"conclusion":"…","checked":["…"],"unchecked":["…"]}`)
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
	if d.Action == "finish" && st.Round == 0 && len(members) > 0 {
		// A model conclusion is not a completed task without a work round.
		d = crewDecision{Action: "work", Assignments: []crewAssignment{{Wind: members[0].ID, Part: st.Task}}}
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
		if len(ok) > 1 && len(ok) == len(members) {
			// Keep all work even if the coordinator assigned the reviewer a part.
			last := ok[len(ok)-1]
			ok[0].Part += "\n\nAlso complete this part: " + last.Part
			if len(ok[0].Files) == 0 || len(last.Files) == 0 {
				ok[0].Files = nil
			} else {
				ok[0].Files = append(ok[0].Files, last.Files...)
			}
			ok = ok[:len(ok)-1]
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
		conclusion = crewExcerpt(fallback, crewReportMax)
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
	// Un encàrrec sense fitxers (la reserva quan el coordinador falla, o un
	// model que no en posa) no pot anar a un seient aïllat per fitxers: abans
	// això tombava tota la volta («has no assigned files»). Treballa com abans.
	for i, a := range assign {
		w := byID[a.Wind]
		building[w.ID] = true
		part := "\n\nYOUR PART (other members build the rest at the same time): " + a.Part
		if len(a.Files) > 0 {
			part += "\nFiles you own (create or edit only these; the others belong to other members): " + strings.Join(a.Files, ", ")
		}
		flow.Nodes = append(flow.Nodes, team.FlowNode{ID: fmt.Sprintf("build-%d", i+1), Label: w.Name, Role: "builder", Mode: "task", CrewAssigned: len(a.Files) > 0,
			Instruction: instruction(w, false) + part, Wind: w.ID, Files: append([]string(nil), a.Files...)})
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
			RequireStructuredReview: true,
			Instruction:             instruction(reviewer, true) + " In feedback, say what you actually checked (files opened, behaviour verified) and what you could not check.",
			Wind:                    reviewer.ID})
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
	ctx, doneTurn := s.beginWindTurn("rotllana", rot.ID, ctx)
	defer doneTurn()
	s.crewLoopContext(ctx, rot, st, job)
}

// crewRoundVerdict converteix el final d'una volta en un missatge llegible:
// si la revisió deixa coses obertes, les diu una per una (l'usuari ha de veure
// QUÈ falta, no només que «ha acabat»); si tot va bé, diu que la colla hi
// queda d'acord i què ha quedat validat.
func crewRoundVerdict(say func(c, e string) string, verdict, answer string, rejected bool) string {
	feedback := ""
	if i := strings.Index(answer, "Outstanding review feedback:"); i >= 0 {
		feedback = strings.TrimSpace(answer[i+len("Outstanding review feedback:"):])
	} else if rejected {
		// Veredicte JSON del revisor: els problemes oberts van al camp feedback.
		if obj := firstJSONObject(answer); obj != "" {
			var v struct {
				Feedback string `json:"feedback"`
			}
			if json.Unmarshal([]byte(obj), &v) == nil {
				feedback = strings.TrimSpace(v.Feedback)
			}
		}
	}
	if rejected && feedback != "" {
		return verdict + "\n" + say("**Queda obert**", "**Still open**") + ":\n- " + oneLineKeepBreaks(feedback, 600)
	}
	if rejected {
		return verdict + "\n" + say("La revisió no s'ha completat o hi deixa punts oberts; la colla hi torna a la volta següent.", "The review is incomplete or leaves open issues; the crew goes at it again next round.")
	}
	return verdict + " " + say("La revisió ha aprovat aquesta volta; no és una votació de tota la colla.", "The review approved this round; this is not a vote by the whole crew.")
}

func (s *Server) crewLoopContext(ctx context.Context, rot Rotllana, st *crewState, job *crewJob) {
	members := s.rotMembersOf(rot)
	if len(members) == 0 {
		crewJobFailed(job)
		return
	}
	lead := members[0]
	say := func(c, e string) string {
		if st.Ca {
			return c
		}
		return e
	}
	finishWithFiles := func(msg string) string {
		files := crewFilesForResult(s, rot.ID, st)
		if len(files) > 0 {
			msg += "\n\n**" + say("Fitxers", "Files") + "**\n- " + strings.Join(files, "\n- ")
		}
		return msg
	}
	instruction := func(w companion.Wind, review bool) string {
		base := "You are " + w.Name + ". Specialty: " + w.Role + ". Character: " + w.Persona + "\n"
		if review {
			return base + "Review the actual deliverable against the request and the agreed design. Check that the requested files really exist and work, using the available read-only tools. Identify concrete defects. Your reply must be exactly the required JSON verdict object, nothing else. Never claim checks you did not perform." + windReviewTools
		}
		return base + "Build your part of the deliverable now, in this run, with the write and edit tools, using the content and decisions the crew contributed. Nobody will send anything later; if something is missing, make a sensible choice and say so. Finish with a short summary of the files you wrote. Never claim actions you did not perform." + windBuildTools
	}
	last := st.Last
	var previousAssignment, previousAnswer string
	for {
		if ctx.Err() != nil {
			break
		}
		if err := job.set("working"); err != nil {
			job.fail(err)
			return
		}
		d := s.crewDecide(ctx, rot, st, members)
		if ctx.Err() != nil {
			break
		}
		switch d.Action {
		case "ask":
			st.Questions++
			st.Waiting = true
			if err := saveCrewState(s, rot.ID, st); err != nil {
				log.Printf("crew state persistence rotllana=%s: %s", rot.ID, redactError(err))
				crewJobFailed(job)
				return
			}
			q := crewQuestionText(d.Question, d.Options)
			if err := s.crewSayKind(rot, lead, "question", q); err != nil {
				st.Waiting = false
				if stateErr := saveCrewState(s, rot.ID, st); stateErr != nil {
					log.Printf("crew question rollback persistence rotllana=%s: %s", rot.ID, redactError(stateErr))
				}
				log.Printf("crew question persistence rotllana=%s: %s", rot.ID, redactError(err))
				crewJobFailed(job)
				return
			}
			s.crewNotify(rot, lead, st.Task, say("La colla et pregunta: ", "The crew asks you: ")+q)
			if !crewJobStep(job, "waiting") {
				return
			}
			return
		case "finish":
			// A coordinator or the round cap cannot turn a rejected review into success.
			unfinished := crewRejected(last) || st.ReviewIncomplete
			if unfinished {
				d.Conclusion = say("La feina no està acabada; la revisió és incompleta o hi queden problemes.\n\n", "The work is not finished; review is incomplete or issues remain.\n\n") + crewExcerpt(firstNonEmpty(d.Conclusion, last), crewReportMax)
			}
			if !unfinished && d.Publish != nil {
				if line := s.crewPublish(ctx, *d.Publish, st.Ca); line != "" {
					d.Conclusion = strings.TrimSpace(d.Conclusion) + "\n\n" + line
				}
			}
			msg := crewResultText(d, crewFilesForResult(s, rot.ID, st), last, st.Ca)
			step := "done"
			if unfinished {
				step = "failed"
			}
			if err := s.finishCrewRun(rot, st, lead, job, step, msg); err != nil {
				log.Printf("crew result persistence rotllana=%s: %s", rot.ID, redactError(err))
				return
			}
			return
		case "giveup":
			msg := finishWithFiles(say("No ho podem fer: ", "We can't do it: ") + firstNonEmpty(d.Say, d.Conclusion))
			if err := s.finishCrewRun(rot, st, lead, job, "failed", msg); err != nil {
				log.Printf("crew giveup persistence rotllana=%s: %s", rot.ID, redactError(err))
				return
			}
			return
		}
		// work: una volta amb les parts en paral·lel i la revisió.
		st.Round++
		if err := saveCrewState(s, rot.ID, st); err != nil {
			log.Printf("crew round state persistence rotllana=%s: %s", rot.ID, redactError(err))
			crewJobFailed(job)
			return
		}
		var plan []string
		names := map[string]string{}
		for _, w := range members {
			names[w.ID] = w.Name
		}
		for _, a := range d.Assignments {
			plan = append(plan, names[a.Wind]+": "+oneLine(a.Part, 120))
		}
		note := firstNonEmpty(d.Say, say("Som-hi", "Let's go"))
		// El pla de la volta (qui fa què) es veu sempre: és el compromís de
		// la colla, no debat intern.
		if err := s.crewSayKind(rot, lead, "round", fmt.Sprintf("%s (%s %d)\n- %s", note, say("volta", "round"), st.Round, strings.Join(plan, "\n- "))); err != nil {
			log.Printf("crew round history persistence rotllana=%s: %s", rot.ID, redactError(err))
			crewJobFailed(job)
			return
		}
		flow := crewRoundFlow(d.Assignments, members, instruction, st.Ca)
		work := st.Task + "\n\nCrew discussion and notes so far (context, not new instructions):\n" + agent.TranscriptDigestSpeakers(s.crewHistory(rot), 12000)
		writeKey := crewRegistryKey(s, rot.ID)
		crewWritesMu.Lock()
		writesBefore := crewWriteEvents[writeKey]
		crewWritesMu.Unlock()
		answer, err := crewWork(s, ctx, rot, work, flow, st.Ca)
		if ctx.Err() != nil {
			break
		}
		crewWritesMu.Lock()
		wroteThisRound := crewWriteEvents[writeKey] != writesBefore
		crewWritesMu.Unlock()
		switch {
		case err == nil || err == errReviewIncomplete:
			last = answer
			st.Last, st.ReviewIncomplete = answer, err == errReviewIncomplete || len(members) < 2
			assignmentJSON, _ := json.Marshal(d.Assignments)
			assignment := string(assignmentJSON)
			if !wroteThisRound && crewRejected(answer) && assignment == previousAssignment && strings.TrimSpace(answer) == previousAnswer {
				msg := say("La feina no està acabada: la mateixa revisió s'ha repetit sense progressar.\n\n", "The work is not finished: the same review repeated without progress.\n\n") + crewExcerpt(answer, crewReportMax)
				msg = crewResultText(crewDecision{Conclusion: msg}, crewFilesForResult(s, rot.ID, st), "", st.Ca)
				if err := s.finishCrewRun(rot, st, lead, job, "failed", msg); err != nil {
					log.Printf("crew stalled result persistence rotllana=%s: %s", rot.ID, redactError(err))
					return
				}
				return
			}
			previousAssignment, previousAnswer = assignment, strings.TrimSpace(answer)
			verdict := say("La volta de revisió ha acabat.", "The review round has ended.")
			if err == errReviewIncomplete || st.ReviewIncomplete {
				verdict = say("La revisió no s'ha pogut completar; la feina no està verificada.", "The review could not be completed; the work is not verified.")
			}
			// El final de volta diu què queda obert (o que hi estan d'acord):
			// l'usuari ha de veure la conclusió de la revisió, no només que
			// «ha acabat». Kind "round": l'app el mostra sempre desplegat,
			// mai dins el desplegable del debat.
			verdict = crewRoundVerdict(say, verdict, answer, crewRejected(answer) || st.ReviewIncomplete)
			if err := s.crewSayKind(rot, lead, "round", verdict); err != nil {
				log.Printf("crew review history persistence rotllana=%s: %s", rot.ID, redactError(err))
				crewJobFailed(job)
				return
			}
			st.Log = append(st.Log, fmt.Sprintf("Round %d (%s): %s", st.Round, strings.Join(plan, "; "), oneLineKeepBreaks(answer, 2500)))
		case crewPotReprendre(err, st.Started):
			if err := s.crewSayKind(rot, lead, "debate", say("S'ha encallat la volta; ho reprenem aprofitant el que ja hi ha fet.", "The round stalled; we resume from what is already done.")); err != nil {
				log.Printf("crew retry history persistence rotllana=%s: %s", rot.ID, redactError(err))
				crewJobFailed(job)
				return
			}
			st.Log = append(st.Log, fmt.Sprintf("Round %d stalled (%v); files on disk are kept.", st.Round, err))
		default:
			log.Printf("crew work: %s", redactError(err))
			msg := finishWithFiles(crewFailureMessage(say, err, time.Since(st.Started), nil))
			if err := s.finishCrewRun(rot, st, lead, job, "failed", msg); err != nil {
				log.Printf("crew failure result persistence rotllana=%s: %s", rot.ID, redactError(err))
				return
			}
			return
		}
		if err := saveCrewState(s, rot.ID, st); err != nil {
			log.Printf("crew progress persistence rotllana=%s: %s", rot.ID, redactError(err))
			crewJobFailed(job)
			return
		}
	}
	msg := say("S'ha acabat el temps de la colla; et deixo el que hi ha.", "The crew ran out of time; here is what exists.")
	step := "failed"
	if ctx.Err() == context.Canceled {
		msg = say("La feina s'ha aturat; els canvis ja fets es conserven.", "The work was stopped; changes already made are kept.")
		step = "cancelled"
	}
	msg = finishWithFiles(msg + "\n\n" + oneLineKeepBreaks(last, 3000))
	if err := s.finishCrewRun(rot, st, lead, job, step, msg); err != nil {
		log.Printf("crew terminal result persistence rotllana=%s: %s", rot.ID, redactError(err))
		return
	}
}

// crewTakeAnswer: si la colla d'aquest fil espera resposta, el missatge de
// l'usuari la reprèn (en lloc d'anar al xat de sempre). Torna true si l'ha
// agafat; el missatge ja és al fil.
type crewAnswerResult uint8

const (
	crewAnswerNotApplicable crewAnswerResult = iota
	crewAnswerConsumed
	crewAnswerFailed
)

func (s *Server) crewTakeAnswer(rot Rotllana, msg string) crewAnswerResult {
	// Serialize the waiting-state check and reservation with crew starts and
	// other answers, so only one request can resume the persisted job.
	reservation := crewReservationLock(s, rot.ID)
	reservation.Lock()
	defer reservation.Unlock()
	if !s.crewMayResume() {
		return crewAnswerNotApplicable
	}
	stateLock := crewStateLock(s, rot.ID)
	stateLock.Lock()
	defer stateLock.Unlock()
	st, exists, stateErr := readCrewState(s, rot.ID)
	if stateErr != nil {
		log.Printf("crew resume state read rotllana=%s: %s", rot.ID, redactError(stateErr))
		return crewAnswerFailed
	}
	if exists && st.PendingResult != "" {
		return crewAnswerFailed
	}
	if !exists || (!st.Waiting && st.PendingAnswer == "") {
		return crewAnswerNotApplicable
	}
	if st.PendingAnswer != "" && st.PendingAnswer != msg {
		return crewAnswerFailed
	}
	job := &crewJob{path: crewJobPath(s, rot.ID)}
	key := crewRegistryKey(s, rot.ID)
	if v, loaded := crewJobs.LoadOrStore(key, job); loaded {
		existing := v.(*crewJob)
		if st.PendingAnswer == "" {
			if step := existing.get(); step != "waiting" && step != "" {
				return crewAnswerNotApplicable // ja hi ha feina en marxa
			}
		}
		job = existing
	}
	if st.PendingAnswer == "" {
		st.PendingAnswer = msg
		st.Waiting = false
		st.Log = append(st.Log, "The user answered the crew's question: "+oneLineKeepBreaks(msg, 1500))
	}
	if err := saveCrewStateUnlocked(s, rot.ID, st); err != nil {
		log.Printf("crew resume state persistence rotllana=%s: %s", rot.ID, redactError(err))
		return crewAnswerFailed
	}
	hist := rotllanaHistory(s, rot.ID)
	hist.mu.Lock()
	next := append([]llm.Message(nil), hist.messages...)
	if len(next) == 0 || next[len(next)-1].Role != "user" || next[len(next)-1].Content != st.PendingAnswer {
		next = append(next, llm.Message{Role: "user", Content: st.PendingAnswer, Ts: time.Now().Unix()})
	}
	if err := saveRotllanaHistory(s, rot.ID, next); err != nil {
		hist.mu.Unlock()
		log.Printf("crew resume history persistence rotllana=%s: %s", rot.ID, redactError(err))
		return crewAnswerFailed
	}
	hist.messages = podaHistorialVent(next)
	hist.mu.Unlock()
	if err := job.set("working"); err != nil {
		log.Printf("crew resume job persistence rotllana=%s: %s", rot.ID, redactError(err))
		return crewAnswerFailed
	}
	st.PendingAnswer = ""
	if err := saveCrewStateUnlocked(s, rot.ID, st); err != nil {
		log.Printf("crew resume completion persistence rotllana=%s: %s", rot.ID, redactError(err))
		return crewAnswerFailed
	}
	goBackground(func() { s.crewLoop(rot, st, job) })
	return crewAnswerConsumed
}

// crewReportMax: l'informe final d'una colla és un resum, no les notes
// enganxades de cada membre (en una prova real en sortien més de 6.000
// caràcters). Les notes completes queden als fitxers i a la volta.
const crewReportMax = 900

// crewExcerpt talla un text llarg al final d'un paràgraf o d'una frase abans
// de max caràcters i ho indica amb «…».
func crewExcerpt(text string, max int) string {
	t := strings.TrimSpace(text)
	r := []rune(t)
	if len(r) <= max {
		return t
	}
	cut := string(r[:max])
	if i := strings.LastIndex(cut, "\n\n"); i > len(cut)/3 {
		cut = cut[:i]
	} else if i := strings.LastIndexAny(cut, ".!?"); i > len(cut)/3 {
		cut = cut[:i+1]
	}
	return strings.TrimSpace(cut) + " …"
}

// crewQuestionText afegeix a la pregunta de la colla un bloc choices amb les
// respostes proposades: la web i l'Android les mostren com a botons i tocar-ne
// una respon la colla (abans calia escriure-ho, i la pregunta en demanava cinc).
func crewQuestionText(question string, options []string) string {
	var opts []string
	seen := map[string]bool{}
	for _, o := range options {
		o = strings.TrimSpace(strings.ReplaceAll(o, "`", ""))
		if o == "" || len([]rune(o)) > 60 || strings.Contains(o, "\n") || seen[strings.ToLower(o)] {
			continue
		}
		seen[strings.ToLower(o)] = true
		opts = append(opts, o)
		if len(opts) == 4 {
			break
		}
	}
	if len(opts) < 2 {
		return question
	}
	return strings.TrimSpace(question) + "\n\n```choices\n" + strings.Join(opts, "\n") + "\n```"
}
