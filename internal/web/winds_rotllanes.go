package web

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"gregal/internal/companion"

	"gregal/internal/agent"
	"gregal/internal/llm"
	"gregal/internal/session"
)

// Rotllanes: xats de grup amb els vents que l'usuari triï. Cada vent veu tota
// la conversa del grup i respon al seu torn, amb la seva persona i el seu
// color. La història és compartida i duradora, com la del xat individual.
//
//	GET    /api/winds/rotllanes           → {rotllanes:[{id,name,members:[windID]}]}
//	POST   /api/winds/rotllanes           → crea/actualitza {name,members:[windID]}
//	DELETE /api/winds/rotllanes           → {id}
//	GET    /api/winds/rotllana?rotlla=<id> → {rotllana,history:[{from,text}]}
//	POST   /api/winds/rotllana            → {reply:{wind,text},history} — un vent del grup respon
type Rotllana struct {
	Workspace string   `json:"workspace,omitempty"`
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Members   []string `json:"members"` // ids de vent
}

// rotllanaEntry és la història en memòria + persistència, com windHistoryEntry.
type rotllanaEntry struct {
	mu       sync.RWMutex
	loadOnce sync.Once
	messages []llm.Message // Role user/assistant; les del vent porten el nom al from
	gen      uint64        // torns publicats: commit només si cap altre ha publicat entremig
	// torn serialitza els torns d'escriptura (com a windHistoryEntry).
	torn sync.Mutex
}

var rotllanaHistories sync.Map // conversationHistoryKey -> *rotllanaEntry

// esRespostaMecanica diu si el text és el resum automàtic d'un torn aturat
// («Resposta parcial: …» / «Partial answer: …») i no una resposta del vent.
func esRespostaMecanica(text string) bool {
	t := strings.TrimSpace(text)
	return strings.HasPrefix(t, "Resposta parcial:") || strings.HasPrefix(t, "Partial answer:")
}

// fraseTornAturat és l'última línia de defensa: una sola frase curta, en la
// llengua de la conversa, que diu què ha passat i convida a tornar-ho a provar.
// Cap URL, cap text de Go, cap «Resposta parcial».
func fraseTornAturat(motiu, lang string) string {
	amable := agent.MotiuAmablePub(motiu, lang)
	if lang == "ca" {
		if amable != "" {
			return "Ui, se m'ha tallat el fil a mitges (" + amable + "). Ho tornem a provar?"
		}
		return "Ui, se m'ha tallat el fil a mitges. Ho tornem a provar?"
	}
	if amable != "" {
		return "Oops, I lost my train of thought mid-answer (" + amable + "). Shall we try again?"
	}
	return "Oops, I lost my train of thought mid-answer. Shall we try again?"
}

func rotllanaHistory(s *Server, id string) *rotllanaEntry {
	h, _ := rotllanaHistories.LoadOrStore(historyKey(s, "rotllana", id), &rotllanaEntry{})
	entry := h.(*rotllanaEntry)
	entry.loadOnce.Do(func() {
		if saved, err := session.Load(s.sessDir(), "rotllana-"+id); err == nil {
			entry.messages = append([]llm.Message(nil), saved.Convo...)
		}
	})
	return entry
}

func saveRotllanaHistory(s *Server, id string, history []llm.Message) error {
	_, err := session.SaveSession(s.sessDir(), session.Session{
		Name:  "rotllana-" + id,
		Title: "Rotllana — " + id,
		Role:  "chat",
		Convo: append([]llm.Message(nil), podaHistorialVent(history)...),
	})
	return err
}

// handleRotllanes gestiona el catàleg de rotllanes.
func (s *Server) handleRotllanes(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		var req Rotllana
		if !decodeSmall(w, r, &req) {
			return
		}
		saved, err := s.saveRotllana(req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, map[string]any{"rotllana": saved})
	case http.MethodDelete:
		var req struct {
			ID string `json:"id"`
		}
		if !decodeSmall(w, r, &req) || strings.TrimSpace(req.ID) == "" {
			http.Error(w, "id obligatori", http.StatusBadRequest)
			return
		}
		if err := s.deleteRotllana(req.ID); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		writeJSON(w, map[string]bool{"ok": true})
	case http.MethodGet:
		list, err := s.listRotllanes()
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		writeJSON(w, map[string]any{"rotllanes": list})
	default:
		http.Error(w, "GET, POST or DELETE required", http.StatusMethodNotAllowed)
	}
}

// rotllanaVoiceLoop és la via de cerca del vent que parla a la rotllana; els
// tests la substitueixen. Per defecte, el loop d'inspecció amb eines web
// (web_search, web_fetch), igual que el xat directe amb un vent, amb el
// model per defecte dels vents (windRoleRef).
var rotllanaVoiceLoop = func(s *Server, ctx context.Context, brief string, routed ...*routedRole) (exploracio, error) {
	p, role := s.windRoleRef()
	if len(routed) > 0 && routed[0] != nil {
		p, role = routed[0].p, routed[0].role
	}
	return s.runReadOnlyLoopForRole(ctx, brief, 6, p, role)
}

// handleRotllanaChat és el xat de grup: cada missatge de l'usuari fa parlar
// un vent del grup (el següent en torn), veient tota la conversa.
func (s *Server) handleRotllanaChat(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		id := strings.TrimSpace(r.URL.Query().Get("rotlla"))
		rot, ok := s.rotllanaByID(id)
		if !ok {
			http.Error(w, "unknown rotllana", http.StatusBadRequest)
			return
		}
		hist := rotllanaHistory(s, rot.ID)
		hist.mu.RLock()
		messages := append([]llm.Message(nil), hist.messages...)
		hist.mu.RUnlock()
		writeJSON(w, map[string]any{"rotllana": rot.ID, "history": s.rotllanaHistoryJSON(messages)})
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "GET or POST required", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Rotllana string `json:"rotllana"`
		Message  string `json:"message"`
		Wind     string `json:"wind"`     // opcional: quin vent parla (si buit, el del torn)
		Continue bool   `json:"continue"` // reprèn l'última resposta parcial
	}
	if !decodeSmall(w, r, &req) {
		return
	}
	rot, ok := s.rotllanaByID(strings.TrimSpace(req.Rotllana))
	if !ok {
		http.Error(w, "unknown rotllana", http.StatusBadRequest)
		return
	}
	msg := strings.TrimSpace(req.Message)
	if wanted := strings.TrimSpace(req.Wind); wanted != "" {
		if _, err := s.nextWindInHistory(rot, wanted, nil); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}
	// La colla d'aquest fil espera resposta: el missatge la reprèn.
	if !req.Continue && msg != "" {
		switch s.crewTakeAnswer(rot, msg) {
		case crewAnswerConsumed:
			if v, ok := crewJobs.Load(crewRegistryKey(s, rot.ID)); ok {
				s.followCrewJob(w, r, rot, v.(*crewJob), len(s.crewHistory(rot)), false)
				return
			}
			writeJSON(w, map[string]any{"reply": map[string]string{"wind": "", "text": ""}, "silenci": true, "history": s.rotllanaHistoryJSON(s.crewHistory(rot))})
			return
		case crewAnswerFailed:
			errText := "could not resume the crew; the answer may not have been saved. Please retry."
			if s.cfg.Lang() == "ca" {
				errText = "No s'ha pogut reprendre la colla; potser la resposta no s'ha desat. Torna-ho a provar."
			}
			http.Error(w, errText, http.StatusInternalServerError)
			return
		}
	}
	if crewChatBusyFor(s, rot.ID) {
		http.Error(w, s.crewBusyMessage(), http.StatusConflict)
		return
	}
	if !req.Continue && strings.TrimSpace(req.Wind) == "" && msg != "" && s.crewChatAllowed(msg) {
		s.crewChat(w, r, rot, msg, false)
		return
	}
	hist := rotllanaHistory(s, rot.ID)
	// torn serialitza les escriptures del fil; mu queda lliure per als GET.
	hist.torn.Lock()
	defer hist.torn.Unlock()
	hist.mu.RLock()
	base := append([]llm.Message(nil), hist.messages...)
	gotGen := hist.gen
	hist.mu.RUnlock()
	commitRotllanaClassic := func(next []llm.Message) []llm.Message {
		next = podaHistorialVent(next)
		hist.mu.Lock()
		if hist.gen == gotGen {
			hist.messages = next
			hist.gen++
			gotGen = hist.gen
		}
		out := append([]llm.Message(nil), hist.messages...)
		hist.mu.Unlock()
		return out
	}
	// «Continua» reprèn l'última resposta: sense historial o sense qui
	// parla no té què reprendre; si no, cal text nou.
	isContinue := req.Continue && len(base) > 0
	if msg == "" && !isContinue {
		http.Error(w, "missatge buit", http.StatusBadRequest)
		return
	}
	// Tria local: membres nomenats (màxim dos) o participació equilibrada;
	// «continua» conserva l’última veu, sense una crida extra al model.
	speakers := s.rotllanaTurnSpeakers(context.Background(), rot, strings.TrimSpace(req.Wind), msg, base, isContinue)
	if len(speakers) == 0 && (strings.TrimSpace(req.Wind) != "" || isContinue) {
		speakingWind, err := s.nextWindInHistory(rot, strings.TrimSpace(req.Wind), base)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		speakers = []companion.Wind{speakingWind}
	}

	conversation := append([]llm.Message(nil), base...)
	if !isContinue {
		conversation = append(conversation, llm.Message{Role: "user", Content: msg, Ts: time.Now().Unix()})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	ctx = s.withWindSteerKey(s.interactiveWindCtx(ctx), "rotllana", rot.ID)

	// Silenci: ningú hi té res a dir. Es desa el missatge i prou.
	if len(speakers) == 0 {
		if err := saveRotllanaHistory(s, rot.ID, conversation); err != nil {
			http.Error(w, "no s'ha pogut desar la conversa", http.StatusInternalServerError)
			return
		}
		conversation = commitRotllanaClassic(conversation)
		writeJSON(w, map[string]any{
			"reply":   map[string]string{"wind": "", "text": ""},
			"silenci": true,
			"history": s.rotllanaHistoryJSON(conversation),
		})
		return
	}
	turnStart := time.Now()
	speakingWind := speakers[0]
	parcial := false
	type veu struct {
		wind companion.Wind
		text string
	}
	var veus []veu
	for i, sp := range speakers {
		system, brief := rotllanaChatBrief(s, rot, sp, conversation, msg)
		brief += windDocsBrief
		perspective := rotllanaPerspective(sp, i, len(speakers))
		brief += perspective
		system += perspective
		vctx := ctx
		if i > 0 {
			vctx = s.readOnlyWindCtx(vctx)
		} else {
			vctx = s.windDocsCtx(vctx)
		}
		tVeu := time.Now()
		var res exploracio
		var err error
		if isContinue && i == 0 {
			// «Continua» reprèn l'historial (amb la parcial) amb
			// pressupost nou; la resta arrenca amb la consigna.
			s.mu.Lock()
			p, role := s.windRoleRef()
			s.mu.Unlock()
			res, err = s.runReadOnlyLoopForRole(vctx, "", 6, p, role, conversation)
			if err == nil {
				parcial = res.parcial
			}
		} else {
			var routed *routedRole
			if p, role, routeWhy := s.windRouteRole(msg); routeWhy != "" {
				log.Printf("vents router %s (rotllana %s): %s", sp.ID, rot.ID, routeWhy)
				routed = &routedRole{p: p, role: role}
			}
			res, err = rotllanaVoiceLoop(s, agent.WithSkillsDirs(vctx, s.windSkillsDir(sp.ID)), brief, routed)
			if err == nil {
				parcial = res.parcial || parcial
			}
		}
		out := res.text
		// Un torn que mor a mitges (proveïdor caigut després d'un pas
		// bo) no és una resposta del vent: és el resum mecànic de
		// «Resposta parcial». Es tracta com un error i cau al xat pla,
		// perquè si el proveïdor s'ha recuperat el vent contesta de veritat.
		if err == nil && res.parcial && esRespostaMecanica(out) {
			err = errors.New(res.motiu)
			out = ""
		}
		if err != nil {
			// La cerca web no pot tombar el torn: cau al xat pla
			// (sense eines), que sempre respon.
			p, role, ok2 := s.sideRoleRef()
			if !ok2 {
				http.Error(w, "no side role available", http.StatusServiceUnavailable)
				return
			}
			s.mu.Lock()
			cfg, client := s.cfg, s.client
			s.mu.Unlock()
			msgs := []llm.Message{{Role: "system", Content: system}}
			msgs = append(msgs, conversation...)
			if len(msgs) > 41 {
				msgs = append(msgs[:1], msgs[len(msgs)-40:]...)
			}
			plain, perr := petChatter(ctx, client, cfg, p, role, msgs)
			if perr != nil {
				http.Error(w, redactError(perr), http.StatusBadGateway)
				return
			}
			out = plain
		}
		reply := rotllanaOwnReply(stripQuoteTrim(out), s.rotMembersOf(rot), sp)
		if reply == "" {
			continue
		}
		reply += s.windDocLinks(tVeu, reply)
		conversation = append(conversation, llm.Message{Role: "assistant", Content: reply, SpeakerID: sp.ID, SpeakerName: sp.Name, Ts: time.Now().Unix()})
		veus = append(veus, veu{sp, reply})
	}
	if len(veus) == 0 {
		writeJSON(w, map[string]any{"reply": map[string]string{}, "silenci": true, "history": s.rotllanaHistoryJSON(base)})
		return
	}
	answer := veus[0].text
	if len(veus) >= 2 && !parcial && !isContinue && rotllanaVolDebat(msg) {
		if summary, err := s.rotllanaSummary(ctx, rot, speakingWind, conversation, msg); err == nil && summary != "" {
			conversation = append(conversation, llm.Message{Role: "assistant", Content: summary, SpeakerID: speakingWind.ID, SpeakerName: speakingWind.Name, Ts: time.Now().Unix()})
			answer = summary
		}
	}
	if err := saveRotllanaHistory(s, rot.ID, conversation); err != nil {
		http.Error(w, "no s'ha pogut desar la conversa", http.StatusInternalServerError)
		return
	}
	conversation = commitRotllanaClassic(conversation)
	// Cada veu recorda l'usuari (gustos, fets duradors), en segon pla.
	if strings.TrimSpace(msg) != "" {
		for _, v := range veus {
			s.startWindMemoryLearn(v.wind, msg, v.text)
		}
	}
	writeJSON(w, map[string]any{
		"reply":   map[string]string{"wind": veus[0].wind.ID, "text": answer},
		"parcial": parcial,
		"history": s.rotllanaHistoryJSON(conversation),
		"actions": remindActionsSince(s.remindOwner(), speakingWind.ID, turnStart),
	})
}

// rotllanaVoice és una veu d'una resposta multi-veu: qui parla i el seu text.
type rotllanaVoice struct {
	wind companion.Wind
	text string
}

// splitRotllanaVoices parteix una resposta multi-veu ("Mestral: …\nMarinada:
// …") en un missatge per veu. Les línies que arrenquen amb el prefix "Nom: "
// d'un membre real del grup obren una veu nova; tot el que ve després (fins
// al següent prefix vàlid) pertany a aquella veu. Si cap línia no porta
// prefix vàlid, tota la resposta és una sola veu: la del vent del torn.
func splitRotllanaVoices(reply string, members []companion.Wind, speaking companion.Wind) []rotllanaVoice {
	if len(members) == 0 {
		return []rotllanaVoice{{wind: speaking, text: reply}}
	}
	speakerOf := func(line string) (companion.Wind, string, bool) {
		// El model pot escriure el prefix amb èmfasi («**Nom:**», «**Nom**:»,
		// «*Nom:*») o repetir el seu propi nom dins de la veu («Nom: **Nom:**
		// text»): s'arrenca l'etiqueta i prou. L'èmfasi del text («**Pla** és…»)
		// es queda, i un altre nom després de l'etiqueta («Anna: Bob, …») és
		// algú a qui la veu parla, no qui parla.
		w, rest, ok := rotllanaLabel(members, line)
		if !ok {
			return companion.Wind{}, "", false
		}
		for i := 0; i < 3; i++ {
			again, more, ok := rotllanaLabel(members, rest)
			if !ok || again.ID != w.ID {
				break
			}
			rest = more
		}
		return w, rest, true
	}
	var voices []rotllanaVoice
	current := companion.Wind{}
	var buf strings.Builder
	var preamble strings.Builder
	flush := func() {
		if current.ID == "" {
			return
		}
		text := strings.TrimSpace(buf.String())
		if text == "" {
			return
		}
		voices = append(voices, rotllanaVoice{wind: current, text: text})
		buf.Reset()
	}
	for _, line := range strings.Split(reply, "\n") {
		if w, rest, ok := speakerOf(line); ok {
			flush()
			// El que va abans del primer prefix és del vent del torn.
			if len(voices) == 0 && current.ID == "" {
				if pre := strings.TrimSpace(preamble.String()); pre != "" {
					voices = append(voices, rotllanaVoice{wind: speaking, text: pre})
					preamble.Reset()
				}
			}
			current = w
			buf.WriteString(rest)
			continue
		}
		if current.ID != "" {
			buf.WriteString("\n" + line)
		} else {
			preamble.WriteString(line + "\n")
		}
	}
	flush()
	if len(voices) == 0 {
		return []rotllanaVoice{{wind: speaking, text: reply}}
	}
	return voices
}

// rotMembersOf llista els vents reals d'una rotllana.
func (s *Server) rotMembersOf(rot Rotllana) []companion.Wind {
	members := make([]companion.Wind, 0, len(rot.Members))
	for _, id := range rot.Members {
		if w, ok := s.winds.ByID(id); ok {
			members = append(members, w)
		}
	}
	return members
}

// memberByPrefix busca un membre pel nom exacte o per un prefix únic a la
// frontera de paraula ("Marinada" → "Marinada de prova"). Només coincideix si
// un sol membre encaixa: ambigu o desconegut no és ningú.
func memberByPrefix(members []companion.Wind, name string) (companion.Wind, bool) {
	name = strings.TrimSpace(name)
	if name == "" {
		return companion.Wind{}, false
	}
	needle := strings.ToLower(name) + " "
	var found companion.Wind
	matches := 0
	for _, w := range members {
		wn := strings.TrimSpace(w.Name)
		if strings.EqualFold(wn, name) || strings.HasPrefix(strings.ToLower(wn), needle) {
			found = w
			matches++
		}
	}
	if matches != 1 {
		return companion.Wind{}, false
	}
	return found, true
}

// Keep from/text byte-compatible. New replies persist identity independently
// of their display name; old chats resolve a unique name prefix when possible
// and the resolved prefix is stripped from the text so it does not show twice.
func (s *Server) rotllanaHistoryJSON(messages []llm.Message) []map[string]any {
	out := petHistoryJSON(messages)
	winds, _ := s.winds.List()
	for i, m := range messages {
		id, name := m.SpeakerID, m.SpeakerName
		if m.Role == "assistant" && id == "" {
			matches := 0
			for _, wind := range winds {
				if strings.HasPrefix(m.Content, wind.Name+": ") {
					id, name = wind.ID, wind.Name
					matches++
				}
			}
			if matches != 1 {
				id, name = "", ""
			}
		}
		out[i]["speaker_id"], out[i]["speaker_name"] = id, name
		// Un missatge antic amb el prefix resolt el mostra net: la identitat
		// ja viatja per speaker_name i la UI no ha de duplicar el nom.
		if name != "" && strings.HasPrefix(m.Content, name+": ") {
			if text, _ := out[i]["text"].(string); strings.HasPrefix(text, name+": ") {
				out[i]["text"] = strings.TrimPrefix(text, name+": ")
			}
		}
		if m.Kind != "" {
			out[i]["kind"] = m.Kind
		}
	}
	return out
}

// rotllanaRoster descriu els membres per al system del vent que parla.
func (s *Server) rotllanaRoster(rot Rotllana) string {
	var b strings.Builder
	for _, id := range rot.Members {
		if w, ok := s.winds.ByID(id); ok {
			b.WriteString("- " + w.Name + " (" + companion.RolePrompt(w.Role) + ")\n")
		}
	}
	return strings.TrimSpace(b.String())
}

// nextWindInTurn tria quin vent respon: el demanat si és membre, o el que toca
// per torn (el que menys ha parlat recentment).
func (s *Server) nextWindInTurn(rot Rotllana, wanted string) (companion.Wind, error) {
	hist := rotllanaHistory(s, rot.ID)
	hist.mu.Lock()
	defer hist.mu.Unlock()
	return s.nextWindInHistory(rot, wanted, hist.messages)
}

// Caller holds the conversation lock, so queued turns select their speaker
// only after the preceding model call has completed and its turn is saved.
func (s *Server) nextWindInHistory(rot Rotllana, wanted string, tail []llm.Message) (companion.Wind, error) {
	members := make([]companion.Wind, 0, len(rot.Members))
	for _, id := range rot.Members {
		if w, ok := s.winds.ByID(id); ok {
			members = append(members, w)
		}
	}
	if len(members) == 0 {
		return companion.Wind{}, errNoMembers
	}
	if wanted != "" {
		for _, w := range members {
			if w.ID == wanted {
				return w, nil
			}
		}
		return companion.Wind{}, errUnknownWind
	}
	// Torn: el que menys cops apareix al final de la història.
	if len(tail) > 6 {
		tail = tail[len(tail)-6:]
	}
	counts := map[string]int{}
	for _, m := range tail {
		if m.Role == "assistant" {
			for _, w := range members {
				if m.SpeakerID == w.ID || (m.SpeakerID == "" && strings.HasPrefix(m.Content, w.Name+":")) {
					counts[w.ID]++
				}
			}
		}
	}
	best := members[0]
	for _, w := range members[1:] {
		if counts[w.ID] < counts[best.ID] {
			best = w
		}
	}
	return best, nil
}

var (
	errNoMembers   = errConst("la rotllana no té vents")
	errUnknownWind = errConst("el vent no és membre de la rotllana")
)

func errConst(msg string) error { return &constErr{msg} }

type constErr struct{ msg string }

func (e *constErr) Error() string { return e.msg }

// --- Persistència de rotllanes (rotllanes.json al costat de winds.json) ---

// RotllanaStore desa les rotllanes d'un usuari (o les compartides).
type RotllanaStore struct {
	dir string
	mu  *sync.Mutex
}

// Sessions of the same user can have separate store handles to the same file.
var rotllanaStoreMu sync.Mutex

func NewRotllanaStore(dir string) *RotllanaStore {
	return &RotllanaStore{dir: dir, mu: &rotllanaStoreMu}
}

func (rs *RotllanaStore) path() string { return filepath.Join(rs.dir, "rotllanes.json") }

type rotllanesDisc struct {
	Rotllanes []Rotllana `json:"rotllanes"`
}

// List torna les rotllanes amb només vents que existeixen al catàleg.
func (rs *RotllanaStore) List(winds *companion.WindStore) ([]Rotllana, error) {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	raw, err := os.ReadFile(rs.path())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var d rotllanesDisc
	if err := json.Unmarshal(raw, &d); err != nil {
		return nil, err
	}
	out := make([]Rotllana, 0, len(d.Rotllanes))
	for _, rot := range d.Rotllanes {
		rot.Members = resolveWindMembers(winds, rot.Members)
		out = append(out, rot)
	}
	return out, nil
}

// Save crea o actualitza una rotllana (id ja resolt).
func (rs *RotllanaStore) Save(rot Rotllana) (Rotllana, error) {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	raw, err := os.ReadFile(rs.path())
	if err != nil && !os.IsNotExist(err) {
		return Rotllana{}, err
	}
	var d rotllanesDisc
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &d); err != nil {
			return Rotllana{}, err
		}
	}
	explicit := rot.ID != ""
	if !explicit {
		base := companion.SlugWind(rot.Name)
		if base == "" {
			base = "rotllana"
		}
		for {
			var suffix [8]byte
			if _, err := rand.Read(suffix[:]); err != nil {
				return Rotllana{}, err
			}
			rot.ID = base + "-" + hex.EncodeToString(suffix[:])
			collision := false
			for _, prev := range d.Rotllanes {
				if prev.ID == rot.ID {
					collision = true
					break
				}
			}
			if !collision {
				break
			}
		}
	}
	found := false
	for i := range d.Rotllanes {
		if d.Rotllanes[i].ID == rot.ID {
			d.Rotllanes[i] = rot
			found = true
			break
		}
	}
	if !found {
		if explicit {
			return Rotllana{}, errConst("unknown rotllana")
		}
		d.Rotllanes = append(d.Rotllanes, rot)
	}
	if err := os.MkdirAll(rs.dir, 0o755); err != nil {
		return Rotllana{}, err
	}
	b, _ := json.MarshalIndent(d, "", "  ")
	return rot, os.WriteFile(rs.path(), b, 0o644)
}

// Delete treu una rotllana del disc.
func (rs *RotllanaStore) Delete(id string) error {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	raw, err := os.ReadFile(rs.path())
	if err != nil {
		return err
	}
	var d rotllanesDisc
	if err := json.Unmarshal(raw, &d); err != nil {
		return err
	}
	out := make([]Rotllana, 0, len(d.Rotllanes))
	for _, rot := range d.Rotllanes {
		if rot.ID != id {
			out = append(out, rot)
		}
	}
	b, _ := json.MarshalIndent(rotllanesDisc{Rotllanes: out}, "", "  ")
	return os.WriteFile(rs.path(), b, 0o644)
}

// resolveWindMembers manté només vents que encara existeixen.
func resolveWindMembers(winds *companion.WindStore, ids []string) []string {
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if _, ok := winds.ByID(id); ok {
			out = append(out, id)
		}
	}
	return out
}

func (s *Server) listRotllanes() ([]Rotllana, error) { return s.rotllanes.List(s.winds) }

func (s *Server) rotllanaByID(id string) (Rotllana, bool) {
	list, err := s.listRotllanes()
	if err != nil {
		return Rotllana{}, false
	}
	for _, rot := range list {
		if rot.ID == id {
			return rot, true
		}
	}
	return Rotllana{}, false
}

func (s *Server) saveRotllana(rot Rotllana) (Rotllana, error) {
	if rot.Workspace != "" {
		resolved, err := resolveWorkspace(rot.Workspace)
		if err != nil {
			return Rotllana{}, err
		}
		if err := s.user.Allow(resolved); err != nil {
			return Rotllana{}, err
		}
		rot.Workspace = resolved
	}
	name := strings.TrimSpace(rot.Name)
	if name == "" {
		return Rotllana{}, errNoName
	}
	rot.Name = name
	rot.Members = resolveWindMembers(s.winds, rot.Members)
	if len(rot.Members) == 0 {
		return Rotllana{}, errNoMembers
	}
	return s.rotllanes.Save(rot)
}

var errNoName = errConst("la rotllana necessita un nom")

func (s *Server) deleteRotllana(id string) error {
	if err := s.rotllanes.Delete(id); err != nil {
		return err
	}
	// La conversa no és el catàleg: sense netejar-la, l'historial orfe
	// quedaria al disc i tornaria si es recrea una rotllana amb el mateix id.
	rotllanaHistories.Delete(historyKey(s, "rotllana", id))
	if err := session.Delete(s.sessDir(), "rotllana-"+id); err != nil {
		if !strings.Contains(err.Error(), "no trobada") {
			return err
		}
	}
	return nil
}

func rotllanaPerspective(sp companion.Wind, index, count int) string {
	if count < 2 {
		return ""
	}
	voice := "\n\nNow only YOU, " + sp.Name + ", speak. "
	if index == 0 {
		return voice + "Give the group a concrete proposal from your own specialty. Include the key assumption or trade-off."
	}
	return voice + "Read the preceding proposals. Add one concrete risk, correction, missing fact, or alternative that changes the outcome. Reply with exactly - if you have nothing to add. Speak only for yourself."
}

// rotllanaOwnReply enforces the identity assigned to a model call. A name
// written by the model is never sufficient to create a second speaker.
func rotllanaOwnReply(reply string, members []companion.Wind, speaking companion.Wind) string {
	voices := ownVoiceOnly(splitRotllanaVoices(reply, members, speaking), speaking)
	if len(voices) == 0 {
		return ""
	}
	text := strings.TrimSpace(voices[0].text)
	switch text {
	case "-", "—", "–", ".":
		return ""
	}
	return text
}

// ownVoiceOnly es queda només amb el que diu el vent que parla: el que el
// model hagi escrit en nom d'altres membres es descarta (com fa SillyTavern
// tallant a l'etiqueta d'un altre personatge).
func ownVoiceOnly(voices []rotllanaVoice, sp companion.Wind) []rotllanaVoice {
	var parts []string
	for _, v := range voices {
		if v.wind.ID == sp.ID {
			parts = append(parts, v.text)
		}
	}
	if len(parts) == 0 {
		return nil
	}
	return []rotllanaVoice{{wind: sp, text: strings.Join(parts, "\n\n")}}
}

// rotllanaLabel llegeix una etiqueta «Nom:» al principi d'una línia, amb
// l'èmfasi que l'envolta («**Nom:**», «**Nom**:», «__Nom:__», «*Nom:*»).
// Torna el membre i el text que segueix, sense tocar-ne l'èmfasi propi.
func rotllanaLabel(members []companion.Wind, line string) (companion.Wind, string, bool) {
	t := strings.TrimSpace(line)
	open := ""
	for _, m := range []string{"**", "__", "*", "_"} {
		if strings.HasPrefix(t, m) {
			open, t = m, t[len(m):]
			break
		}
	}
	cut := strings.Index(t, ":")
	if cut <= 0 {
		return companion.Wind{}, "", false
	}
	name, rest := t[:cut], t[cut+1:]
	if open != "" {
		switch {
		case strings.HasSuffix(name, open): // «**Nom**:»
			name = strings.TrimSuffix(name, open)
		case strings.HasPrefix(rest, open): // «**Nom:**»
			rest = rest[len(open):]
		default:
			return companion.Wind{}, "", false
		}
	}
	w, ok := memberByPrefix(members, strings.TrimSpace(name))
	if !ok {
		return companion.Wind{}, "", false
	}
	return w, strings.TrimSpace(rest), true
}
