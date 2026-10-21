package web

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"gregal/internal/companion"

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
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Members []string `json:"members"` // ids de vent
}

// rotllanaEntry és la història en memòria + persistència, com windHistoryEntry.
type rotllanaEntry struct {
	mu       sync.Mutex
	loadOnce sync.Once
	messages []llm.Message // Role user/assistant; les del vent porten el nom al from
}

var rotllanaHistories sync.Map // conversationHistoryKey -> *rotllanaEntry

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
		Convo: append([]llm.Message(nil), history...),
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
var rotllanaVoiceLoop = func(s *Server, ctx context.Context, brief string) (string, error) {
	s.mu.Lock()
	p, role := s.windRoleRef()
	s.mu.Unlock()
	res, err := s.runReadOnlyLoopForRole(ctx, brief, 6, p, role)
	return res.text, err
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
		hist.mu.Lock()
		messages := append([]llm.Message(nil), hist.messages...)
		hist.mu.Unlock()
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
		Wind     string `json:"wind"` // opcional: quin vent parla (si buit, el del torn)
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
	if msg == "" {
		http.Error(w, "missatge buit", http.StatusBadRequest)
		return
	}
	hist := rotllanaHistory(s, rot.ID)
	hist.mu.Lock()
	defer hist.mu.Unlock() // choose the speaker and commit under the same turn lock
	// Qui parla: el triat o el següent del torn, incloent l'últim torn complet.
	speakingWind, err := s.nextWindInHistory(rot, strings.TrimSpace(req.Wind), hist.messages)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	conversation := append(append([]llm.Message(nil), hist.messages...), llm.Message{Role: "user", Content: msg})

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	system, brief := rotllanaChatBrief(s, rot, speakingWind, conversation, msg)
	turnStart := time.Now()
	out, err := rotllanaVoiceLoop(s, ctx, brief)
	if err != nil {
		// La cerca web no pot tombar el torn: cau al xat pla (sense eines),
		// que sempre respon.
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
	reply := stripQuoteTrim(out)
	// La resposta pot ser multi-veu ("Mestral: …\nMarinada: …"): cada veu
	// queda com un missatge propi amb la seva identitat, en ordre. La
	// primera veu és la resposta del torn.
	voices := splitRotllanaVoices(reply, s.rotMembersOf(rot), speakingWind)
	history := conversation
	for _, v := range voices {
		history = append(history, llm.Message{Role: "assistant", Content: v.wind.Name + ": " + v.text, SpeakerID: v.wind.ID, SpeakerName: v.wind.Name})
	}
	if err := saveRotllanaHistory(s, rot.ID, history); err != nil {
		http.Error(w, "no s'ha pogut desar la conversa", http.StatusInternalServerError)
		return
	}
	hist.messages = history
	replyWind, replyText := speakingWind, reply
	if len(voices) > 0 {
		replyWind, replyText = voices[0].wind, voices[0].text
	}
	writeJSON(w, map[string]any{
		"reply":   map[string]string{"wind": replyWind.ID, "text": replyText},
		"history": s.rotllanaHistoryJSON(history),
		"actions": remindActionsSince(speakingWind.ID, turnStart),
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
		trimmed := strings.TrimSpace(line)
		if i := strings.Index(trimmed, ":"); i > 0 {
			if w, ok := memberByPrefix(members, trimmed[:i]); ok {
				return w, trimmed[i+1:], true
			}
		}
		return companion.Wind{}, "", false
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
// of their display name; old chats resolve a unique name prefix when possible.
func (s *Server) rotllanaHistoryJSON(messages []llm.Message) []map[string]string {
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
	}
	return out
}

// rotllanaRoster descriu els membres per al system del vent que parla.
func (s *Server) rotllanaRoster(rot Rotllana) string {
	var b strings.Builder
	for _, id := range rot.Members {
		if w, ok := s.winds.ByID(id); ok {
			b.WriteString("- " + w.Name + " (" + w.Role + ")\n")
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

func (s *Server) deleteRotllana(id string) error { return s.rotllanes.Delete(id) }
