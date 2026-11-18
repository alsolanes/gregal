package web

import (
	"hash/fnv"
	"strconv"
	"strings"
	"sync"
	"time"

	"gregal/internal/llm"
)

// Notes apreses congelades per conversa.
//
// Després de cada resposta, el vent aprèn en segon pla (coneixement del tema,
// memòria de la persona, lliçons de feina) i ho afegeix als blocs del system.
// Si el system canviava a cada torn, el proveïdor havia de tornar a processar
// tot el que hi ha darrere: la resta del system, les eines i l'historial
// sencer (mesurat amb un model local, desenes de milers de tokens per torn).
// I el que s'aprèn d'aquesta conversa ja hi és, a la conversa.
//
// Per això els blocs es llegeixen al primer torn d'una conversa i es
// mantenen mentre només creixin. Es tornen a llegir si:
//   - canvia la conversa (el primer missatge de l'usuari és un altre: fil nou,
//     esborrat o podat);
//   - alguna línia de la còpia ja no hi és (la persona ha editat o esborrat
//     una nota: s'ha de notar de seguida);
//   - fa més de windNotesMaxAge que es van llegir.

const windNotesMaxAge = 6 * time.Hour

type windNotesSnap struct {
	conv   string
	at     time.Time
	blocks map[string]string
}

var (
	windNotesMu    sync.Mutex
	windNotesSnaps = map[string]windNotesSnap{}
)

// frozenWindNotes torna els blocs que s'han d'enviar: la còpia de la
// conversa si encara val, o els actuals (i en desa la còpia).
func (s *Server) frozenWindNotes(windID, conv string, fresh map[string]string) map[string]string {
	key := s.windStateDir() + "|" + windID
	windNotesMu.Lock()
	defer windNotesMu.Unlock()
	if snap, ok := windNotesSnaps[key]; ok && snap.conv == conv && time.Since(snap.at) < windNotesMaxAge && notesOnlyGrew(snap.blocks, fresh) {
		return snap.blocks
	}
	windNotesSnaps[key] = windNotesSnap{conv: conv, at: time.Now(), blocks: fresh}
	return fresh
}

// notesOnlyGrew diu si cada línia de la còpia encara és als blocs actuals: el
// que s'ha afegit (aprenentatge de fons) no obliga a refer-la; el que s'ha
// tret o canviat (edició de la persona), sí.
func notesOnlyGrew(snap, fresh map[string]string) bool {
	for name, old := range snap {
		now := map[string]bool{}
		for _, l := range strings.Split(fresh[name], "\n") {
			now[strings.TrimSpace(l)] = true
		}
		for _, l := range strings.Split(old, "\n") {
			if l = strings.TrimSpace(l); l != "" && !now[l] {
				return false
			}
		}
	}
	return true
}

// windConversationKey identifica una conversa pel primer missatge de la
// persona (sense hora: al primer torn encara no és a l'historial i és el
// missatge nou).
func windConversationKey(previous []llm.Message, msg string) string {
	first := msg
	for _, m := range previous {
		if m.Role == "user" && m.Kind == "" && strings.TrimSpace(m.Content) != "" {
			first = m.Content
			break
		}
	}
	h := fnv.New64a()
	h.Write([]byte(strings.TrimSpace(first)))
	return strconv.FormatUint(h.Sum64(), 36)
}
