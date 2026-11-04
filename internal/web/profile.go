package web

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// El perfil de cada persona: les preferències de la interfície (tema, idioma,
// densitat…) viuen al compte del servidor, no només al navegador. Entrant des
// d'un altre dispositiu amb el mateix usuari, el dispositiu les recupera.
//
// Va per usuari autenticat (la carpeta d'estat del seu compte, com la memòria):
// qualsevol nombre de comptes, i ningú no pot llegir ni escriure el perfil d'un
// altre, tampoc un administrador. Només hi caben les claus d'aquesta llista, i
// mai el token, les sessions obertes ni res que depengui del dispositiu.
//
// Cada clau porta la seva hora: dos dispositius que canvien coses diferents no
// es trepitgen, i en una mateixa clau guanya l'últim canvi.

// profileKeys: les claus del navegador que viatgen amb la persona.
var profileKeys = map[string]bool{
	"gregal_idioma":            true, // idioma
	"gregal_tema":              true, // tema (clar, fosc, marca…)
	"gregal_densitat":          true,
	"gregal_mida":              true, // mida de la lletra
	"gregal_view":              true, // vista normal/compacta
	"gregal_doc_ample":         true,
	"gregal_side_compact":      true,
	"gregal_vw_more":           true,
	"gregal_inspector":         true,
	"gregal_fun_mascot":        true,
	"gregal_fun_voice":         true,
	"gregal_fun_wrapped":       true,
	"gregal_wrapped_dismissed": true,
	"gregal_retro_persona":     true,
	"gregal_exp_kanban":        true,
	"gregal_todo_obert":        true,
	"arenaCC":                  true,
}

const (
	profileMaxValue = 512
	profileMaxBytes = 64 << 10
)

// profileEntry: el valor d'una clau (nil = l'ha esborrada) i quan va canviar (ms).
type profileEntry struct {
	V *string `json:"v"`
	T int64   `json:"t"`
}

type profileDoc struct {
	Prefs map[string]profileEntry `json:"prefs"`
}

var profileMu sync.Mutex

// profilePath: la carpeta d'estat del compte. Sense comptes (un sol usuari
// compartit), un únic perfil al costat de la configuració.
func (s *Server) profilePath() string {
	if s.user != nil && !s.user.Tothom {
		return filepath.Join(userStateDir(s.cfgPath, s.user.Name), "profile.json")
	}
	return filepath.Join(goalDirFor(s.cfgPath), "profile.json")
}

func readProfile(path string) profileDoc {
	d := profileDoc{Prefs: map[string]profileEntry{}}
	if raw, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(raw, &d)
	}
	if d.Prefs == nil {
		d.Prefs = map[string]profileEntry{}
	}
	// El que no és a la llista no s'hi queda (llista més curta en una versió nova).
	for k := range d.Prefs {
		if !profileKeys[k] {
			delete(d.Prefs, k)
		}
	}
	return d
}

// mergeProfile aplica els canvis que arriben: cada clau només si és de la
// llista, el valor és curt i el canvi és més nou que el que hi ha.
func mergeProfile(cur profileDoc, in map[string]profileEntry, now int64) profileDoc {
	for k, e := range in {
		if !profileKeys[k] {
			continue
		}
		if e.V != nil && len(*e.V) > profileMaxValue {
			continue
		}
		// Una hora al futur llunyà no pot guanyar per sempre: es talla a «ara».
		if e.T <= 0 || e.T > now+5*60*1000 {
			e.T = now
		}
		if old, ok := cur.Prefs[k]; ok && old.T >= e.T {
			continue
		}
		cur.Prefs[k] = e
	}
	return cur
}

// handleProfile: GET el perfil; POST {prefs:{clau:{v,t}}} hi fusiona canvis i
// el torna sencer.
func (s *Server) handleProfile(w http.ResponseWriter, r *http.Request) {
	path := s.profilePath()
	switch r.Method {
	case http.MethodGet:
		profileMu.Lock()
		d := readProfile(path)
		profileMu.Unlock()
		writeJSON(w, profileReply(d))
	case http.MethodPost:
		var req profileDoc
		if !decodeSmall(w, r, &req) {
			return
		}
		profileMu.Lock()
		defer profileMu.Unlock()
		d := mergeProfile(readProfile(path), req.Prefs, time.Now().UnixMilli())
		raw, _ := json.Marshal(d)
		if len(raw) > profileMaxBytes {
			http.Error(w, "profile too large", http.StatusRequestEntityTooLarge)
			return
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			http.Error(w, "cannot save profile", http.StatusInternalServerError)
			return
		}
		tmp := path + ".tmp"
		if err := os.WriteFile(tmp, raw, 0o600); err == nil {
			err = os.Rename(tmp, path)
			if err != nil {
				_ = os.Remove(tmp)
				http.Error(w, "cannot save profile", http.StatusInternalServerError)
				return
			}
		} else {
			http.Error(w, "cannot save profile", http.StatusInternalServerError)
			return
		}
		writeJSON(w, profileReply(d))
	default:
		http.Error(w, "GET or POST only", http.StatusMethodNotAllowed)
	}
}

// profileReply: el perfil i la llista de claus que la pàgina ha de sincronitzar
// (així la llista només és en un lloc).
func profileReply(d profileDoc) map[string]any {
	keys := make([]string, 0, len(profileKeys))
	for k := range profileKeys {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return map[string]any{"prefs": d.Prefs, "keys": keys}
}
