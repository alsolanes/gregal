package web

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gregal/internal/companion"
	"gregal/internal/session"
)

// «Posa ordre» als vents: amb el temps s'hi acumulen duplicats (dues
// Marinades, dos Gregals), vents de prova («fdsaf») i vents que no has fet
// servir mai, i la llista es fa inmanejable. Aquí:
//
//	GET  /api/winds/tidy  → què hi ha: grups de duplicats (mateix nom o mateix
//	                        ofici), vents de prova i vents sense ús, amb el que
//	                        té cadascun (missatges, memòria, rutines…).
//	POST /api/winds/tidy  {action:"merge", keep, from} → ajunta «from» a «keep»:
//	                        memòria, coneixement i habilitats, TOTES les converses,
//	                        rutines i recordatoris, grups, targetes, preferits i
//	                        on el fan servir (A prop teu, resum del matí). El vent
//	                        absorbit s'arxiva (amb còpia), no es perd res.
//	POST /api/winds/tidy  {action:"archive", id} → arxiva un vent (amb còpia).

type tidyWind struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Role      string `json:"role,omitempty"`
	Squad     string `json:"squad,omitempty"`
	Msgs      int    `json:"msgs"`
	Threads   int    `json:"threads"`
	Memory    int    `json:"memory"`
	Knowledge int    `json:"knowledge"`
	Skills    int    `json:"skills"`
	Jobs      int    `json:"jobs"`
	LastAt    string `json:"last_at,omitempty"`
}

type tidyGroup struct {
	Reason string     `json:"reason"` // same_name | same_role
	Keep   string     `json:"keep"`
	Winds  []tidyWind `json:"winds"`
}

type tidyReport struct {
	Total  int         `json:"total"`
	Groups []tidyGroup `json:"groups"`
	Test   []tidyWind  `json:"test"`
	Unused []tidyWind  `json:"unused"`
}

// tidyNorm: el nom o l'ofici per comparar (minúscules, sense accents ni espais de més).
func tidyNorm(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	r := strings.NewReplacer("à", "a", "á", "a", "è", "e", "é", "e", "í", "i", "ï", "i", "ò", "o", "ó", "o", "ú", "u", "ü", "u", "ç", "c", "·", "", "ñ", "n")
	return strings.Join(strings.Fields(r.Replace(s)), " ")
}

// tidyLooksTest: un nom de prova («fdsaf», «asdf», «test»), no un nom de debò.
func tidyLooksTest(name string) bool {
	n := tidyNorm(name)
	if n == "" || len([]rune(n)) <= 2 {
		return true
	}
	switch n {
	case "test", "testing", "prova", "proves", "nou vent", "new wind", "vent", "wind", "asdf", "qwerty":
		return true
	}
	if strings.Contains(n, " ") {
		return false
	}
	// Totes les lletres de la mateixa fila del teclat: «fdsaf», «asdfg», «qwer».
	for _, row := range []string{"asdfghjkl", "qwertyuiop", "zxcvbnm"} {
		all := true
		for _, c := range n {
			if !strings.ContainsRune(row, c) {
				all = false
				break
			}
		}
		if all && len(n) >= 3 {
			return true
		}
	}
	vowels := 0
	for _, c := range n {
		if strings.ContainsRune("aeiouy", c) {
			vowels++
		}
	}
	return len(n) >= 4 && vowels == 0
}

func (s *Server) tidyStats(w companion.Wind, jobsByWind map[string]int) tidyWind {
	tw := tidyWind{ID: w.ID, Name: w.Name, Role: w.Role, Squad: w.Squad, Jobs: jobsByWind[w.ID]}
	for _, th := range s.windSessionList(w.ID) {
		if th.Msgs == 0 {
			continue
		}
		tw.Threads++
		tw.Msgs += th.Msgs
		if th.At > tw.LastAt {
			tw.LastAt = th.At
		}
	}
	tw.Memory = len(strings.TrimSpace(companion.LoadWindMemory(s.windStateDir(), w.ID)))
	tw.Knowledge = len(strings.TrimSpace(s.windBrain(w.ID)))
	tw.Skills = len(s.windSkills(w.ID))
	return tw
}

// tidyWeight: quant té un vent (per triar quin es queda).
func tidyWeight(t tidyWind) int {
	return t.Msgs*10 + t.Memory/50 + t.Knowledge/50 + t.Skills*10 + t.Jobs*5
}

func (s *Server) windTidyReport() (tidyReport, error) {
	winds, err := s.winds.List()
	if err != nil {
		return tidyReport{}, err
	}
	jobsByWind := map[string]int{}
	if s.jobs != nil {
		if all, err := s.jobs.List(); err == nil {
			for _, j := range all {
				if j.Wind != "" {
					jobsByWind[j.Wind]++
				}
			}
		}
	}
	stats := map[string]tidyWind{}
	for _, w := range winds {
		stats[w.ID] = s.tidyStats(w, jobsByWind)
	}
	rep := tidyReport{Total: len(winds), Groups: []tidyGroup{}, Test: []tidyWind{}, Unused: []tidyWind{}}
	grouped := map[string]bool{}
	group := func(reason string, key func(companion.Wind) string) {
		buckets := map[string][]companion.Wind{}
		var order []string
		for _, w := range winds {
			k := key(w)
			if k == "" || grouped[w.ID] {
				continue
			}
			if _, ok := buckets[k]; !ok {
				order = append(order, k)
			}
			buckets[k] = append(buckets[k], w)
		}
		for _, k := range order {
			ws := buckets[k]
			if len(ws) < 2 {
				continue
			}
			g := tidyGroup{Reason: reason}
			for _, w := range ws {
				g.Winds = append(g.Winds, stats[w.ID])
				grouped[w.ID] = true
			}
			// Es queda Gregal si hi és; si no, el que té més coses.
			sort.SliceStable(g.Winds, func(i, j int) bool {
				if (g.Winds[i].ID == "gregal") != (g.Winds[j].ID == "gregal") {
					return g.Winds[i].ID == "gregal"
				}
				return tidyWeight(g.Winds[i]) > tidyWeight(g.Winds[j])
			})
			g.Keep = g.Winds[0].ID
			rep.Groups = append(rep.Groups, g)
		}
	}
	group("same_name", func(w companion.Wind) string { return tidyNorm(w.Name) })
	group("same_role", func(w companion.Wind) string {
		r := tidyNorm(w.Role)
		if len([]rune(r)) < 4 {
			return ""
		}
		return r
	})
	for _, w := range winds {
		if w.ID == "gregal" || grouped[w.ID] {
			continue
		}
		t := stats[w.ID]
		switch {
		case tidyLooksTest(w.Name) && t.Msgs < 6:
			rep.Test = append(rep.Test, t)
		case t.Msgs == 0 && t.Memory == 0 && t.Knowledge == 0 && t.Jobs == 0 && t.Skills == 0:
			rep.Unused = append(rep.Unused, t)
		}
	}
	return rep, nil
}

// tidyMergeResult: què s'ha mogut, per dir-ho a la persona.
type tidyMergeResult struct {
	Keep     string `json:"keep"`
	From     string `json:"from"`
	Threads  int    `json:"threads"`
	Jobs     int    `json:"jobs"`
	Groups   int    `json:"groups"`
	Cards    int    `json:"cards"`
	Memory   bool   `json:"memory"`
	Archived bool   `json:"archived"`
}

func (s *Server) windMerge(keepID, fromID string) (tidyMergeResult, error) {
	res := tidyMergeResult{Keep: keepID, From: fromID}
	if keepID == fromID || keepID == "" || fromID == "" {
		return res, errors.New(s.tr("tria dos vents diferents", "choose two different winds"))
	}
	if fromID == "gregal" {
		return res, errors.New(s.tr("Gregal no es pot ajuntar a un altre vent: fes-ho al revés", "Gregal cannot be merged into another wind: do it the other way round"))
	}
	keep, ok := s.winds.ByID(keepID)
	from, ok2 := s.winds.ByID(fromID)
	if !ok || !ok2 {
		return res, errors.New(s.tr("vent desconegut", "unknown wind"))
	}
	// Cap torn d'aquests dos vents mentre es mou tot (sempre en el mateix ordre).
	a, b := keepID, fromID
	if b < a {
		a, b = b, a
	}
	ta, tb := windDirectTurn(s, a), windDirectTurn(s, b)
	ta.Lock()
	defer ta.Unlock()
	tb.Lock()
	defer tb.Unlock()

	// 1. Memòria, coneixement i habilitats (amb còpia del que es queda abans).
	src := s.windSnapshot(from, "version")
	have := map[string]bool{}
	for _, sk := range s.windSkills(keepID) {
		have[sk.Nom] = true
	}
	for i, sk := range src.Skills {
		// Una habilitat amb el mateix nom però diferent: es queda les dues.
		if have[sk.Nom] {
			for n := 2; ; n++ {
				alt := fmt.Sprintf("%s-%d", sk.Nom, n)
				if !have[alt] && validWindSkillName(alt) {
					src.Skills[i].Nom = alt
					have[alt] = true
					break
				}
				if n > 50 {
					break
				}
			}
		}
	}
	if err := s.applyWindInformation(src, keep, true, true); err != nil {
		return res, err
	}
	res.Memory = strings.TrimSpace(src.Memory) != "" || strings.TrimSpace(src.Knowledge) != "" || len(src.Skills) > 0

	// 2. Converses: passen a ser converses del vent que es queda (amb la seva data).
	dir := s.sessDir()
	for i, th := range s.windSessionList(fromID) {
		sess, err := session.Load(dir, th.Name)
		if err != nil {
			continue
		}
		newName := fmt.Sprintf("company-%s-s%d", keepID, time.Now().UnixNano()+int64(i))
		old := sess.Name
		sess.Name = newName
		if strings.HasPrefix(sess.Title, "Company — ") || sess.Title == "" {
			sess.Title = from.Name + " · " + session.TitleFrom(sess.Convo)
		}
		if _, err := session.SaveSession(dir, sess); err != nil {
			return res, err
		}
		_ = session.Delete(dir, old)
		windHistories.Delete(historyKey(s, "wind", old))
		res.Threads++
	}
	windSessionsMu.Lock()
	delete(s.windActiveSession, s.windSessionKey(fromID))
	windSessionsMu.Unlock()

	// 3. Rutines, recordatoris i vigilàncies.
	if s.jobs != nil {
		if all, err := s.jobs.List(); err == nil {
			for _, j := range all {
				if j.Wind != fromID {
					continue
				}
				j.Wind = keepID
				if _, err := s.jobs.Save(j); err == nil {
					res.Jobs++
				}
			}
		}
	}

	// 4. Grups: el vent que es queda hi entra en lloc de l'absorbit.
	if rots, err := s.listRotllanes(); err == nil {
		for _, rot := range rots {
			if !containsStr(rot.Members, fromID) {
				continue
			}
			var members []string
			for _, m := range rot.Members {
				if m == fromID {
					m = keepID
				}
				if !containsStr(members, m) {
					members = append(members, m)
				}
			}
			rot.Members = members
			if _, err := s.saveRotllana(rot); err == nil {
				res.Groups++
			}
		}
	}

	// 5. Targetes desades.
	fromCards := filepath.Join(s.windStateDir(), "wind-cards", fromID)
	if entries, err := os.ReadDir(fromCards); err == nil {
		keepCards := filepath.Join(s.windStateDir(), "wind-cards", keepID)
		_ = os.MkdirAll(keepCards, 0o700)
		for _, e := range entries {
			dst := filepath.Join(keepCards, e.Name())
			if _, err := os.Stat(dst); err == nil {
				continue
			}
			if os.Rename(filepath.Join(fromCards, e.Name()), dst) == nil {
				res.Cards++
			}
		}
	}

	// 6. Preferits i seccions de la barra.
	if meta, err := s.winds.Meta(); err == nil {
		swap := func(keys []string) []string {
			var out []string
			for _, k := range keys {
				if k == "wind:"+fromID {
					k = "wind:" + keepID
				}
				if !containsStr(out, k) {
					out = append(out, k)
				}
			}
			return out
		}
		meta.Favorites = swap(meta.Favorites)
		for i := range meta.Sections {
			meta.Sections[i].Items = swap(meta.Sections[i].Items)
		}
		_ = s.winds.SaveMeta(meta)
	}

	// 7. On el feien servir: «A prop teu».
	localMu.Lock()
	if st := s.localSettings(); st.Wind == fromID {
		st.Wind = keepID
		_ = s.saveLocalSettings(st)
	}
	localMu.Unlock()

	// 8. El vent absorbit s'arxiva (amb una còpia de tot el que tenia).
	if err := s.archiveWind(fromID); err != nil {
		return res, err
	}
	res.Archived = true
	return res, nil
}

func (s *Server) handleWindTidy(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		rep, err := s.windTidyReport()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, rep)
	case http.MethodPost:
		var req struct {
			Action string `json:"action"`
			Keep   string `json:"keep"`
			From   string `json:"from"`
			ID     string `json:"id"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		switch req.Action {
		case "merge":
			res, err := s.windMerge(strings.TrimSpace(req.Keep), strings.TrimSpace(req.From))
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			writeJSON(w, res)
		case "archive":
			id := strings.TrimSpace(req.ID)
			if id == "gregal" {
				http.Error(w, s.tr("Gregal no es pot arxivar", "Gregal cannot be archived"), http.StatusBadRequest)
				return
			}
			if err := s.archiveWind(id); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			writeJSON(w, map[string]any{"archived": id})
		default:
			http.Error(w, "unknown action", http.StatusBadRequest)
		}
	default:
		http.Error(w, "GET or POST required", http.StatusMethodNotAllowed)
	}
}
