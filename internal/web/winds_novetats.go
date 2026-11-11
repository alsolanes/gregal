package web

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gregal/internal/companion"
	"gregal/internal/events"
)

// Novetats dels vents per a l'app: quan un vent parla sol (cor proactiu,
// recordatori), el text entra al seu xat i a més queda apuntat aquí amb
// l'hora. L'app en mode servidor ho consulta de tant en tant
// (GET /api/winds/novetats?des_de=<unix>) i avisa amb notificació local
// si hi ha veus noves. Sense Telegram ni canals externs: tot dins l'app.
//
// El registre viu a winds-novetats.json del dir de sessions (tallat a 100).

const (
	novetatsFile   = "winds-novetats.json"
	maxNovetats    = 100
	maxNovetatsOut = 50
)

// windNovetat és una veu proactiva d'un vent: qui, d'on ve i què ha dit.
type windNovetat struct {
	Ts     int64  `json:"ts"`
	Wind   string `json:"wind"`
	Name   string `json:"name"`
	Source string `json:"source"`
	Text   string `json:"text"`
	// Job: l'id del recordatori que l'ha fet sonar. L'app el fa servir per
	// no avisar dues vegades si ja l'havia mostrat ella amb el servidor caigut.
	Job string `json:"job,omitempty"`
	// Card: la targeta de la safata «Avui» que en va sortir (l'app hi posa
	// els botons Fet / Ara no). Quiet: arriba en hores tranquil·les o amb el
	// nivell «calm»: queda a la safata però el mòbil no sona.
	Card  string `json:"card,omitempty"`
	Quiet bool   `json:"quiet,omitempty"`
	// MessageID: el missatge del fil que és aquesta novetat. La novetat és
	// una vista del missatge, no una còpia: la notificació i «Avui» hi
	// apunten per obrir-lo al lloc exacte i no tornar-lo a anunciar.
	MessageID string `json:"message_id,omitempty"`
	// ID: identificador opac de la novetat (B7). És l'únic que viatja en un
	// push; l'app el fa servir per llegir-la amb la seva sessió autenticada i
	// per no avisar dues vegades del mateix (push + sondeig).
	ID string `json:"id,omitempty"`
	// Owner: el compte que la va provocar (s.usuari()). La lectura filtra
	// per ell: mai no surt una novetat d'un altre compte.
	Owner string `json:"owner,omitempty"`
	// Read: l'usuari ja l'ha vista (fil obert, notificació tocada, «Avui»).
	// Coherent entre dispositius i vistes: tots llegeixen aquest estat.
	Read bool `json:"read,omitempty"`
}

// Tipus d'esdeveniment del registre durable (internal/events) per a les
// novetats. El payload porta només identificadors: el text es llegeix amb
// GET /api/winds/novetats?id=… (el mateix camí que fa servir el push).
const (
	eventNovetat     = "novetat"
	eventNovetatRead = "novetat_read"
)

// novetatID torna l'id de la novetat: el desat o, per a les antigues, un de
// derivat i estable (mateixa idea que messageID).
func novetatID(n windNovetat) string {
	if n.ID != "" {
		return n.ID
	}
	h := sha256.Sum256([]byte(strconv.FormatInt(n.Ts, 10) + "\x00" + n.Wind + "\x00" + n.Source + "\x00" + n.Text))
	return "n-" + hex.EncodeToString(h[:8])
}

func newNovetatID() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return "n-" + strconv.FormatInt(time.Now().Unix(), 36) + hex.EncodeToString(b[:])
}

// visibleNovetats són les del compte que pregunta, amb id omplert.
func visibleNovetats(s *Server, all []windNovetat) []windNovetat {
	me := s.usuari()
	out := make([]windNovetat, 0, len(all))
	for _, n := range all {
		// Les antigues no porten owner: el fitxer ja és del compte (sessDir).
		if n.Owner != "" && n.Owner != me {
			continue
		}
		n.ID = novetatID(n)
		out = append(out, n)
	}
	return out
}

// novetatsMu protegeix les veus proactives també entre processos (crosslock.go).
var novetatsMu = &crossLock{name: ".novetats.lock"}

func novetatsPath(s *Server) string { return filepath.Join(s.sessDir(), novetatsFile) }

func loadNovetats(s *Server) []windNovetat {
	raw, err := os.ReadFile(novetatsPath(s))
	if err != nil {
		return nil
	}
	var list []windNovetat
	if err := json.Unmarshal(raw, &list); err != nil {
		return nil
	}
	return list
}

func saveNovetats(s *Server, list []windNovetat) {
	if len(list) > maxNovetats {
		list = append([]windNovetat(nil), list[len(list)-maxNovetats:]...)
	}
	raw, err := json.Marshal(list)
	if err != nil {
		return
	}
	// El dir pot no existir (tests, primera veu): best-effort igualment.
	_ = os.MkdirAll(filepath.Dir(novetatsPath(s)), 0o700)
	// Temporal + rename: l'altre servidor del perfil mai no llegeix mig fitxer.
	_ = writeFileAtomic(novetatsPath(s), raw)
}

// recordNovetat apunta una veu proactiva. Best-effort: mai no falla el torn.
func recordNovetat(s *Server, w companion.Wind, source, text string) {
	recordNovetatJob(s, w, source, "", text)
}

// recordNovetatJob és recordNovetat amb l'id del job que l'ha provocat.
func recordNovetatJob(s *Server, w companion.Wind, source, job, text string) {
	recordNovetatMsg(s, w, source, job, "", text)
}

// recordNovetatMsg és recordNovetatJob amb el missatge del fil que la novetat representa.
func recordNovetatMsg(s *Server, w companion.Wind, source, job, messageID, text string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	// Cada veu és també una targeta de la safata «Avui» (winds_avui.go).
	card, ring := s.avuiFromVoice(w, source, job, text)
	n := windNovetat{
		Ts:        time.Now().Unix(),
		Wind:      w.ID,
		Name:      w.Name,
		Source:    source,
		Text:      text,
		Job:       job,
		Card:      card,
		Quiet:     !ring,
		MessageID: messageID,
		ID:        newNovetatID(),
		Owner:     s.usuari(),
	}
	novetatsMu.Lock()
	saveNovetats(s, append(loadNovetats(s), n))
	novetatsMu.Unlock()
	// Al registre durable: l'app oberta (stream amb cursor) la insereix al
	// fil sense esperar cap sondeig. Només ids, cap text privat.
	s.appendNovetatEvent(eventNovetat, novetatEventPayload{
		ID: n.ID, MessageID: n.MessageID, Wind: n.Wind, Source: n.Source, Quiet: n.Quiet, Ts: n.Ts,
	})
	// Fora de l'app: push amb l'id opac, si el servidor en té (winds_push.go).
	if ring {
		s.pushNovetat(n)
	}
}

// novetatEventPayload és el que porta un event «novetat» o «novetat_read».
type novetatEventPayload struct {
	ID        string   `json:"id,omitempty"`
	IDs       []string `json:"ids,omitempty"`
	Winds     []string `json:"winds,omitempty"`
	MessageID string   `json:"message_id,omitempty"`
	Wind      string   `json:"wind,omitempty"`
	Source    string   `json:"source,omitempty"`
	Quiet     bool     `json:"quiet,omitempty"`
	Ts        int64    `json:"ts,omitempty"`
}

func (s *Server) appendNovetatEvent(kind string, p novetatEventPayload) {
	if s == nil || s.hub == nil {
		return
	}
	raw, err := json.Marshal(p)
	if err != nil {
		return
	}
	// Sense sessió: és de l'usuari sencer (stream amb ?scope=user).
	_ = s.hub.appendEvent(events.Event{User: s.usuari(), Kind: kind, Payload: raw})
}

// handleWindNovetats torna les veus proactives posteriors a des_de (unix,
// 0 = totes les guardades), de vella a nova. Lleuger: l'app ho crida en
// segon pla sense descarregar historials.
func (s *Server) handleWindNovetats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "només GET", http.StatusMethodNotAllowed)
		return
	}
	novetatsMu.Lock()
	all := visibleNovetats(s, loadNovetats(s))
	novetatsMu.Unlock()
	// Una de sola per id: és la lectura autenticada que fa l'app quan rep un
	// push (que només porta l'id). D'un altre compte o inexistent: 404.
	if id := strings.TrimSpace(r.URL.Query().Get("id")); id != "" {
		for _, n := range all {
			if n.ID == id {
				writeJSON(w, map[string]any{"novetat": n})
				return
			}
		}
		http.Error(w, "novetat desconeguda", http.StatusNotFound)
		return
	}
	var desDe int64
	if raw := strings.TrimSpace(r.URL.Query().Get("des_de")); raw != "" {
		desDe, _ = strconv.ParseInt(raw, 10, 64)
	}
	out := make([]windNovetat, 0, len(all))
	for _, n := range all {
		if n.Ts > desDe {
			out = append(out, n)
		}
	}
	if len(out) > maxNovetatsOut {
		out = append([]windNovetat(nil), out[len(out)-maxNovetatsOut:]...)
	}
	writeJSON(w, map[string]any{"novetats": out})
}

// handleWindNovetatsRead marca novetats com a llegides (POST
// /api/winds/novetats/read). Accepta ids, message_ids o un vent sencer
// (wind: obrir el seu fil les llegeix totes). És idempotent i torna quantes
// han canviat; l'event «novetat_read» avisa els altres dispositius perquè
// treguin la notificació i la insígnia.
func (s *Server) handleWindNovetatsRead(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "només POST", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		IDs        []string `json:"ids"`
		MessageIDs []string `json:"message_ids"`
		Wind       string   `json:"wind"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&req); err != nil {
		http.Error(w, "json invàlid", http.StatusBadRequest)
		return
	}
	ids := map[string]bool{}
	for _, id := range req.IDs {
		ids[strings.TrimSpace(id)] = true
	}
	msgs := map[string]bool{}
	for _, id := range req.MessageIDs {
		msgs[strings.TrimSpace(id)] = true
	}
	delete(ids, "")
	delete(msgs, "")
	wind := strings.TrimSpace(req.Wind)
	if len(ids) == 0 && len(msgs) == 0 && wind == "" {
		http.Error(w, "cal ids, message_ids o wind", http.StatusBadRequest)
		return
	}
	marked := s.readNovetats(func(n windNovetat) bool {
		return ids[novetatID(n)] || (n.MessageID != "" && msgs[n.MessageID]) || (wind != "" && n.Wind == wind)
	})
	writeJSON(w, map[string]any{"ok": true, "marked": marked})
}

// readNovetats marca com a llegides les del compte que compleixen match,
// avisa els altres dispositius (event novetat_read, amb els vents afectats
// perquè treguin l'avís) i torna els ids canviats.
func (s *Server) readNovetats(match func(windNovetat) bool) []string {
	me := s.usuari()
	novetatsMu.Lock()
	list := loadNovetats(s)
	marked := []string{}
	winds := []string{}
	seen := map[string]bool{}
	for i, n := range list {
		if n.Read || (n.Owner != "" && n.Owner != me) || !match(n) {
			continue
		}
		list[i].Read = true
		list[i].ID = novetatID(n)
		marked = append(marked, list[i].ID)
		if n.Wind != "" && !seen[n.Wind] {
			seen[n.Wind] = true
			winds = append(winds, n.Wind)
		}
	}
	if len(marked) > 0 {
		saveNovetats(s, list)
	}
	novetatsMu.Unlock()
	if len(marked) > 0 {
		s.appendNovetatEvent(eventNovetatRead, novetatEventPayload{IDs: marked, Winds: winds})
	}
	return marked
}
