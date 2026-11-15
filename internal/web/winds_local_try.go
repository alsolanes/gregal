package web

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"
)

// «Prova-ho ara» d'«A prop teu»: abans, per veure si funcionava calia
// activar-ho amb el teu lloc i esperar; «Mira-ho ara» deixava el resultat a
// «Avui» en silenci, i si no trobava res no ho sabies. La prova fa una mirada
// amb el que hi ha al formulari (qualsevol lloc, sense activar ni desar res),
// i ensenya allà mateix què ha trobat, amb les fonts, què ha descartat i què
// ha buscat. No toca «Avui», ni el que ja t'ha dit, ni la feina programada.
//
//	POST /api/winds/local {action:"try", place?, topics?, interests?, avoid?, radius_km?, wind?} → {try}
//	GET  /api/winds/local/try?id=…  → {state: running|done|error, …}

type localTryItem struct {
	Section  string `json:"section"`
	Topic    string `json:"topic"`
	Title    string `json:"title"`
	Detail   string `json:"detail,omitempty"`
	URL      string `json:"url"`
	Host     string `json:"host"`
	Repeated bool   `json:"repeated,omitempty"`
}

type localTry struct {
	ID       string         `json:"id"`
	State    string         `json:"state"`
	Place    string         `json:"place"`
	Wind     string         `json:"wind,omitempty"`
	Items    []localTryItem `json:"items"`
	Dropped  int            `json:"dropped"`
	Searches []string       `json:"searches"`
	Nothing  bool           `json:"nothing,omitempty"`
	Error    string         `json:"error,omitempty"`
	Started  int64          `json:"started"`
	Seconds  int            `json:"seconds,omitempty"`
	owner    string
}

var localTries = struct {
	sync.Mutex
	m map[string]*localTry
}{m: map[string]*localTry{}}

const localTryKeep = time.Hour

// localTrySettings: el formulari per sobre del que hi ha desat. Res no es desa.
func (s *Server) localTrySettings(req localSettingsReq) localSettings {
	localMu.Lock()
	st := s.localSettings()
	localMu.Unlock()
	if req.Place != nil {
		st.Place = *req.Place
	}
	if req.Topics != nil {
		st.Topics = *req.Topics
	}
	if req.Interests != nil {
		st.Interests = *req.Interests
	}
	if req.Avoid != nil {
		st.Avoid = *req.Avoid
	}
	if req.RadiusKm != nil {
		st.RadiusKm = *req.RadiusKm
	}
	if req.Wind != nil {
		st.Wind = strings.TrimSpace(*req.Wind)
	}
	return normalizeLocal(st)
}

func (s *Server) startLocalTry(req localSettingsReq) (*localTry, error) {
	st := s.localTrySettings(req)
	if st.Place.Label == "" {
		return nil, errors.New(s.tr("tria un lloc per provar-ho (una ciutat o un barri)", "choose a place to try it (a town or a neighbourhood)"))
	}
	if len(st.Topics) == 0 {
		return nil, errors.New(s.tr("tria almenys un tema", "choose at least one topic"))
	}
	w, ok := s.localWind(st.Wind)
	if !ok {
		return nil, errors.New(s.tr("no hi ha cap vent que ho pugui mirar", "there is no wind to look for it"))
	}
	owner := s.localOwner()
	localTries.Lock()
	now := time.Now()
	for id, t := range localTries.m {
		if now.Sub(time.Unix(t.Started, 0)) > localTryKeep {
			delete(localTries.m, id)
			continue
		}
		// Ja n'hi ha una en marxa (un segon toc, o el mòbil ha perdut el
		// fil): es torna aquella, i el client la segueix fins al final.
		if t.owner == owner && t.State == "running" {
			localTries.Unlock()
			return t, nil
		}
	}
	t := &localTry{ID: avuiID(), State: "running", Place: st.Place.Label, Wind: w.Name, Items: []localTryItem{}, Searches: []string{}, Started: now.Unix(), owner: owner}
	localTries.m[t.ID] = t
	localTries.Unlock()

	go func() {
		defer func() {
			if r := recover(); r != nil {
				s.finishLocalTry(t.ID, func(t *localTry) { t.State, t.Error = "error", "internal error" })
			}
		}()
		ctx, cancel := context.WithTimeout(context.Background(), 6*time.Minute)
		defer cancel()
		loc := s.loadConnections().zone()
		res, err := localInfoLoop(s, ctx, s.localInfoBrief(w, st, time.Now().In(loc)))
		if err != nil {
			s.finishLocalTry(t.ID, func(t *localTry) { t.State, t.Error = "error", friendlyRunError(err.Error()) })
			return
		}
		parsed := parseLocalBlock(res.text)
		fresh := map[string]bool{}
		for _, it := range localPick(parsed, st) {
			fresh[localKey(it.Title)] = true
		}
		// Amb el mateix filtre que la mirada de debò, però el que ja t'havia
		// dit també surt (marcat): si no, una segona prova sortiria buida.
		again := st
		again.Seen = nil
		items := []localTryItem{}
		for _, it := range localPick(parsed, again) {
			items = append(items, localTryItem{Section: it.Section, Topic: it.Topic, Title: it.Title, Detail: it.Detail,
				URL: it.URL, Host: localHost(it.URL), Repeated: !fresh[localKey(it.Title)]})
		}
		searches := localTrySearches(res)
		s.finishLocalTry(t.ID, func(t *localTry) {
			t.State, t.Items, t.Searches = "done", items, searches
			t.Dropped = len(parsed) - len(items)
			if t.Dropped < 0 {
				t.Dropped = 0
			}
			t.Nothing = len(items) == 0
		})
	}()
	return t, nil
}

func (s *Server) finishLocalTry(id string, f func(*localTry)) {
	localTries.Lock()
	defer localTries.Unlock()
	if t, ok := localTries.m[id]; ok {
		f(t)
		t.Seconds = int(time.Since(time.Unix(t.Started, 0)).Seconds())
	}
}

// localTrySearches: què ha buscat el vent (les cerques web, sense repetir).
func localTrySearches(res exploracio) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, m := range res.hist {
		for _, c := range m.ToolCalls {
			if c.Function.Name != "web_search" {
				continue
			}
			var a struct {
				Query string `json:"query"`
			}
			_ = json.Unmarshal([]byte(c.Function.Arguments), &a)
			q := oneLine(a.Query, 120)
			if q != "" && !seen[q] {
				seen[q] = true
				out = append(out, q)
			}
		}
	}
	if len(out) > 8 {
		out = out[:8]
	}
	return out
}

// handleWindsLocalTry: GET ?id= → l'estat de la prova.
func (s *Server) handleWindsLocalTry(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "GET required", http.StatusMethodNotAllowed)
		return
	}
	id := r.URL.Query().Get("id")
	localTries.Lock()
	t, ok := localTries.m[id]
	var out localTry
	if ok && t.owner == s.localOwner() {
		out = *t
		if out.State == "running" {
			out.Seconds = int(time.Since(time.Unix(out.Started, 0)).Seconds())
		}
	} else {
		ok = false
	}
	localTries.Unlock()
	if !ok {
		http.Error(w, s.tr("aquesta prova ja no hi és", "this try is gone"), http.StatusNotFound)
		return
	}
	writeJSON(w, out)
}

func localWants(st localSettings, topic string) bool {
	for _, t := range st.Topics {
		if t == topic {
			return true
		}
	}
	return false
}
