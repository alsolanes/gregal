package web

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"gregal/internal/companion"
)

// «Avui»: la safata dels vents, a l'estil de Duet i del Daily Brief de Gemini.
// Tot el que un vent diu pel seu compte (el resum del matí, el que troba
// vigilant, un recordatori, una feina acabada) és una targeta amb una decisió:
// Fet, Més tard, Ara no, o una acció que obre la conversa amb el vent. El
// resum del matí arriba en tres seccions (Ara toca, Per si de cas, Més
// endavant) i la safata s'acaba: «Això és tot per avui».
//
// El que queda obert torna l'endemà a «Ara toca» (el bucle obert que a Duet
// li faltava), el que no t'interessa ho aprèn el vent («Útil» / «No
// m'interessa») i les hores tranquil·les guarden les veus per al matí en lloc
// de fer sonar el mòbil.
//
//	GET  /api/winds/avui                     → {date, now, fyi, ahead, done, settings, held}
//	POST /api/winds/avui {id, action}        → done|later|dismiss|useful|notuseful|reopen|save
//	GET  /api/winds/avui/settings            → {quiet_from, quiet_to, level, tune}
//	POST /api/winds/avui/settings {…}        → desa i torna la configuració

const (
	avuiFile         = "winds-avui.json"
	avuiSettingsFile = "winds-avui-settings.json"
	maxAvuiCards     = 300
	avuiNow          = "now"
	avuiFYI          = "fyi"
	avuiAhead        = "ahead"
	avuiOpen         = "open"
	avuiDone         = "done"
	avuiDismissed    = "dismissed"
)

type avuiAction struct {
	Label  string `json:"label"`
	Prompt string `json:"prompt,omitempty"`
}

type avuiCard struct {
	ID       string       `json:"id"`
	Wind     string       `json:"wind"`
	WindName string       `json:"wind_name"`
	Source   string       `json:"source"`
	Job      string       `json:"job,omitempty"`
	Section  string       `json:"section"`
	Title    string       `json:"title"`
	Why      string       `json:"why,omitempty"`
	Body     string       `json:"body,omitempty"`
	Actions  []avuiAction `json:"actions,omitempty"`
	State    string       `json:"state"`
	Feedback string       `json:"feedback,omitempty"`
	Saved    bool         `json:"saved,omitempty"`
	// Held: va arribar en hores tranquil·les; no ha fet sonar el mòbil.
	Held    bool  `json:"held,omitempty"`
	Created int64 `json:"created"`
	Updated int64 `json:"updated"`
	// Until: «Més tard» l'amaga fins aquest moment (unix).
	Until int64 `json:"until,omitempty"`
	// Days: dies que fa que és oberta (només a la resposta, per dir «des de dimarts»).
	Days int `json:"days,omitempty"`
}

type avuiSettings struct {
	QuietFrom string `json:"quiet_from"`
	QuietTo   string `json:"quiet_to"`
	// Level: «calm» (només el resum, els recordatoris i les feines fan sonar
	// el mòbil; la resta espera a la safata) o «normal».
	Level string `json:"level"`
	// Tune: el que vols que el resum de demà (i els següents) tingui en compte.
	Tune string `json:"tune,omitempty"`
}

// avuiSettingsReq: el que arriba per canviar-la (Tune nil = no el toquis).
type avuiSettingsReq struct {
	QuietFrom string  `json:"quiet_from"`
	QuietTo   string  `json:"quiet_to"`
	Level     string  `json:"level"`
	Tune      *string `json:"tune"`
}

// avuiMu protegeix avui.json també entre processos (crosslock.go).
var avuiMu = &crossLock{name: ".avui.lock"}

func avuiPath(s *Server) string         { return filepath.Join(s.sessDir(), avuiFile) }
func avuiSettingsPath(s *Server) string { return filepath.Join(s.sessDir(), avuiSettingsFile) }

func loadAvui(s *Server) []avuiCard {
	raw, err := os.ReadFile(avuiPath(s))
	if err != nil {
		return nil
	}
	var list []avuiCard
	if json.Unmarshal(raw, &list) != nil {
		return nil
	}
	return list
}

func saveAvui(s *Server, list []avuiCard) error {
	if len(list) > maxAvuiCards {
		// Les obertes no es perden per fer lloc: primer marxen les tancades més velles.
		sort.SliceStable(list, func(i, j int) bool {
			oi, oj := list[i].State == avuiOpen || list[i].Saved, list[j].State == avuiOpen || list[j].Saved
			if oi != oj {
				return oj
			}
			return list[i].Created < list[j].Created
		})
		list = append([]avuiCard(nil), list[len(list)-maxAvuiCards:]...)
	}
	raw, err := json.Marshal(list)
	if err != nil {
		return err
	}
	_ = os.MkdirAll(filepath.Dir(avuiPath(s)), 0o700)
	// Temporal + rename: l'altre servidor del perfil mai no llegeix mig fitxer.
	return writeFileAtomic(avuiPath(s), raw)
}

func (s *Server) avuiSettings() avuiSettings {
	st := avuiSettings{QuietFrom: "22:00", QuietTo: "08:00", Level: "normal"}
	if raw, err := os.ReadFile(avuiSettingsPath(s)); err == nil {
		_ = json.Unmarshal(raw, &st)
	}
	if st.Level != "calm" {
		st.Level = "normal"
	}
	return st
}

func avuiID() string {
	var b [6]byte
	_, _ = rand.Read(b[:])
	return "c" + hex.EncodeToString(b[:])
}

// minutesOf llegeix «HH:MM»; -1 si no ho és.
func minutesOf(hhmm string) int {
	t, err := time.Parse("15:04", strings.TrimSpace(hhmm))
	if err != nil {
		return -1
	}
	return t.Hour()*60 + t.Minute()
}

// inQuietHours: la franja pot passar mitjanit (22:00 → 08:00).
func inQuietHours(now time.Time, from, to string) bool {
	f, t := minutesOf(from), minutesOf(to)
	if f < 0 || t < 0 || f == t {
		return false
	}
	m := now.Hour()*60 + now.Minute()
	if f < t {
		return m >= f && m < t
	}
	return m >= f || m < t
}

var mdNoise = regexp.MustCompile(`[*_` + "`" + `#>]+|!\[[^\]]*\]\([^)]*\)`)
var mdLink = regexp.MustCompile(`\[([^\]]+)\]\([^)]*\)`)

// plainLine treu el markdown d'una línia (enllaços queden com el seu text).
func plainLine(s string) string {
	s = mdLink.ReplaceAllString(s, "$1")
	s = mdNoise.ReplaceAllString(s, "")
	return strings.Join(strings.Fields(s), " ")
}

// cardTitle és la primera frase del que diu el vent, curta.
func cardTitle(text string) string {
	for _, line := range strings.Split(text, "\n") {
		if l := plainLine(line); l != "" {
			if i := strings.IndexAny(l, ".!?"); i > 24 && i < len(l)-1 {
				l = l[:i+1]
			}
			return oneLine(l, 110)
		}
	}
	return ""
}

func (s *Server) avuiDefaultActions(source, title string) []avuiAction {
	t := oneLine(title, 90)
	switch {
	case source == "remind":
		return []avuiAction{{Label: s.tr("Torna-m'ho a dir d'aquí a una hora", "Remind me again in an hour"), Prompt: s.tr("Torna-m'ho a recordar d'aquí a una hora: ", "Remind me again in an hour: ") + t}}
	case strings.HasPrefix(source, "feina:"):
		return []avuiAction{{Label: s.tr("Explica'm el resultat", "Walk me through it"), Prompt: s.tr("Explica'm el resultat de la feina: ", "Walk me through the result of: ") + t}}
	}
	return []avuiAction{
		{Label: s.tr("Més detalls", "Tell me more"), Prompt: s.tr("Explica'm més sobre això: ", "Tell me more about this: ") + t},
		{Label: s.tr("Recorda-m'ho demà", "Remind me tomorrow"), Prompt: s.tr("Recorda-m'ho demà a les 9: ", "Remind me tomorrow at 9: ") + t},
	}
}

func avuiSectionFor(source string) string {
	switch {
	case source == "remind", strings.HasPrefix(source, "feina:"), source == "job":
		return avuiNow
	}
	return avuiFYI
}

// avuiFromVoice converteix una veu proactiva en targeta. Torna si cal fer
// sonar el mòbil (false: hores tranquil·les o nivell «calm»). El resum del matí
// ja ha posat les seves targetes (avuiFromBrief): aquí només decideix el so.
func (s *Server) avuiFromVoice(w companion.Wind, source, job, text string) (string, bool) {
	st := s.avuiSettings()
	now := time.Now().In(s.loadConnections().zone())
	urgent := source == "remind" || strings.HasPrefix(source, "feina:") || isBriefingJob(job)
	quiet := !urgent && (inQuietHours(now, st.QuietFrom, st.QuietTo) || st.Level == "calm")
	if isBriefingJob(job) {
		return "", true
	}
	title := cardTitle(text)
	if title == "" {
		return "", !quiet
	}
	c := avuiCard{
		ID: avuiID(), Wind: w.ID, WindName: w.Name, Source: source, Job: job,
		Section: avuiSectionFor(source), Title: title, Body: strings.TrimSpace(text),
		Actions: s.avuiDefaultActions(source, title), State: avuiOpen, Held: quiet,
		Created: now.Unix(), Updated: now.Unix(),
	}
	if c.Body == c.Title {
		c.Body = ""
	}
	avuiMu.Lock()
	_ = saveAvui(s, append(loadAvui(s), c))
	avuiMu.Unlock()
	return c.ID, !quiet
}

// avuiBlock és el bloc que el resum del matí afegeix al final per a la safata:
//
//	```avui
//	now :: Truca al dentista abans de les 12 :: Ho vas apuntar dilluns
//	fyi :: Plou a la tarda :: Agafa paraigua
//	ahead :: Divendres sopar a casa :: Compra el vi dijous
//	```
var avuiBlock = regexp.MustCompile("(?s)```avui\\s*\\n(.*?)```")

func parseAvuiBlock(out string) (items [][3]string, clean string) {
	m := avuiBlock.FindStringSubmatch(out)
	if m == nil {
		return nil, strings.TrimSpace(out)
	}
	for _, line := range strings.Split(m[1], "\n") {
		parts := strings.Split(line, "::")
		if len(parts) < 2 {
			continue
		}
		sec := strings.ToLower(strings.TrimSpace(strings.TrimLeft(parts[0], "-* ")))
		switch sec {
		case "now", "ara", "ara toca", "top":
			sec = avuiNow
		case "fyi", "per si de cas":
			sec = avuiFYI
		case "ahead", "més endavant", "later":
			sec = avuiAhead
		default:
			continue
		}
		title := plainLine(parts[1])
		why := ""
		if len(parts) > 2 {
			why = plainLine(strings.Join(parts[2:], "::"))
		}
		if title != "" {
			items = append(items, [3]string{sec, oneLine(title, 110), oneLine(why, 160)})
		}
	}
	return items, strings.TrimSpace(avuiBlock.ReplaceAllString(out, ""))
}

// avuiFromBrief desa les targetes del resum del matí. Les del resum d'ahir que
// eren de «Per si de cas» ja no surten: el resum és d'avui.
func (s *Server) avuiFromBrief(w companion.Wind, job string, items [][3]string, message string) {
	now := time.Now().Unix()
	var cards []avuiCard
	for _, it := range items {
		cards = append(cards, avuiCard{
			ID: avuiID(), Wind: w.ID, WindName: w.Name, Source: "brief", Job: job,
			Section: it[0], Title: it[1], Why: it[2], Actions: s.avuiDefaultActions("brief", it[1]),
			State: avuiOpen, Created: now, Updated: now,
		})
	}
	if len(cards) == 0 {
		if title := cardTitle(message); title != "" {
			cards = append(cards, avuiCard{ID: avuiID(), Wind: w.ID, WindName: w.Name, Source: "brief", Job: job,
				Section: avuiNow, Title: title, Body: message, Actions: s.avuiDefaultActions("brief", title),
				State: avuiOpen, Created: now, Updated: now})
		}
	}
	avuiMu.Lock()
	defer avuiMu.Unlock()
	list := loadAvui(s)
	for i := range list {
		if list[i].Source == "brief" && list[i].State == avuiOpen && list[i].Section != avuiNow && !list[i].Saved {
			list[i].State, list[i].Updated = avuiDismissed, now
		}
	}
	_ = saveAvui(s, append(list, cards...))
}

// avuiBriefExtras afegeix a la consigna del resum el format de la safata, el
// que encara és obert d'altres dies, el que l'usuari vol que tingui en compte
// i el que li ha semblat útil o no.
func (s *Server) avuiBriefExtras(windID string) string {
	var b strings.Builder
	b.WriteString("\n\nAFTER the message, add the items for the user's Today tray in a fenced block, one per line, 3 to 7 lines, most important first:\n```avui\nnow :: <what needs them today, as an action> :: <one short line why>\nfyi :: <good to know> :: <why>\nahead :: <coming up in the next days> :: <the next step>\n```\nUse only facts from your tools, memory or reminders. Never put the block's content in the message itself.")
	avuiMu.ReadLock()
	list := loadAvui(s)
	avuiMu.ReadUnlock()
	var open []string
	cut := time.Now().Add(-14 * 24 * time.Hour).Unix()
	for _, c := range list {
		if c.State == avuiOpen && c.Section == avuiNow && c.Created < startOfToday(s).Unix() && c.Created > cut {
			open = append(open, "- "+c.Title)
		}
	}
	if len(open) > 0 {
		if len(open) > 6 {
			open = open[len(open)-6:]
		}
		b.WriteString("\n\nSTILL OPEN from earlier days (the user has not marked them done; bring back the ones that still matter, briefly):\n" + strings.Join(open, "\n"))
	}
	if tune := strings.TrimSpace(s.avuiSettings().Tune); tune != "" {
		b.WriteString("\n\nWHAT THE USER ASKED THE BRIEF TO TAKE INTO ACCOUNT:\n" + oneLine(tune, 600))
	}
	b.WriteString(s.avuiFeedbackBrief(windID))
	return b.String()
}

// avuiFeedbackBrief: el que la persona ha marcat com a útil o no, perquè el
// vent ho tingui en compte la propera vegada que parli sol.
func (s *Server) avuiFeedbackBrief(windID string) string {
	avuiMu.ReadLock()
	list := loadAvui(s)
	avuiMu.ReadUnlock()
	var good, bad []string
	for i := len(list) - 1; i >= 0 && len(good)+len(bad) < 16; i-- {
		c := list[i]
		if windID != "" && c.Wind != windID {
			continue
		}
		switch c.Feedback {
		case "useful":
			good = append(good, "- "+c.Title)
		case "notuseful":
			bad = append(bad, "- "+c.Title)
		}
	}
	var b strings.Builder
	if len(good) > 0 {
		b.WriteString("\n\nThe user found THESE useful (more like this):\n" + strings.Join(good, "\n"))
	}
	if len(bad) > 0 {
		b.WriteString("\n\nThe user marked THESE as not interesting (avoid similar ones):\n" + strings.Join(bad, "\n"))
	}
	return b.String()
}

func startOfToday(s *Server) time.Time {
	now := time.Now().In(s.loadConnections().zone())
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
}

// avuiView: el que surt avui a cada secció.
func (s *Server) avuiView() map[string]any {
	now := time.Now()
	today := startOfToday(s)
	avuiMu.ReadLock()
	list := loadAvui(s)
	avuiMu.ReadUnlock()
	sections := map[string][]avuiCard{avuiNow: {}, avuiFYI: {}, avuiAhead: {}}
	done := []avuiCard{}
	held := 0
	for _, c := range list {
		created := time.Unix(c.Created, 0)
		switch {
		case c.State == avuiDone && c.Updated >= today.Unix():
			done = append(done, c)
		case c.State != avuiOpen:
		case c.Until > now.Unix():
		case c.Section == avuiNow:
			// El que és obert torna: fins a dues setmanes.
			if now.Sub(created) < 14*24*time.Hour {
				if created.Before(today) {
					c.Days = int(today.Sub(created).Hours()/24) + 1
				}
				sections[avuiNow] = append(sections[avuiNow], c)
			}
		case c.Section == avuiFYI:
			// El que és «per si de cas» caduca en dos dies si no el deses.
			if c.Saved || now.Sub(created) < 48*time.Hour {
				sections[avuiFYI] = append(sections[avuiFYI], c)
			}
		case c.Section == avuiAhead:
			if c.Saved || now.Sub(created) < 7*24*time.Hour {
				sections[avuiAhead] = append(sections[avuiAhead], c)
			}
		}
		if c.Held && c.State == avuiOpen && c.Created >= today.Add(-12*time.Hour).Unix() {
			held++
		}
	}
	for k := range sections {
		cs := sections[k]
		sort.SliceStable(cs, func(i, j int) bool { return cs[i].Created > cs[j].Created })
		if len(cs) > 12 {
			cs = cs[:12]
		}
		sections[k] = cs
	}
	sort.SliceStable(done, func(i, j int) bool { return done[i].Updated > done[j].Updated })
	return map[string]any{
		"date": today.Format("2006-01-02"), "now": sections[avuiNow], "fyi": sections[avuiFYI],
		"ahead": sections[avuiAhead], "done": done, "held": held, "settings": s.avuiSettings(),
	}
}

func (s *Server) handleWindsAvui(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, s.avuiView())
	case http.MethodPost:
		var req struct {
			ID     string `json:"id"`
			Action string `json:"action"`
		}
		if !decodeSmall(w, r, &req) {
			return
		}
		avuiMu.Lock()
		defer avuiMu.Unlock()
		list := loadAvui(s)
		now := time.Now()
		found := false
		for i := range list {
			if list[i].ID != req.ID {
				continue
			}
			found = true
			c := &list[i]
			switch req.Action {
			case "done":
				c.State = avuiDone
			case "dismiss":
				c.State = avuiDismissed
			case "later":
				// Més tard: d'aquí a tres hores, o demà al matí si ja és vespre.
				until := now.Add(3 * time.Hour)
				if local := now.In(s.loadConnections().zone()); local.Hour() >= 19 {
					next := local.AddDate(0, 0, 1)
					until = time.Date(next.Year(), next.Month(), next.Day(), 8, 0, 0, 0, local.Location())
				}
				c.Until = until.Unix()
			case "useful", "notuseful":
				if c.Feedback == req.Action {
					c.Feedback = ""
				} else {
					c.Feedback = req.Action
				}
				if req.Action == "notuseful" && c.Feedback != "" {
					c.State = avuiDismissed
				}
			case "save":
				c.Saved = !c.Saved
			case "reopen":
				c.State, c.Until = avuiOpen, 0
			default:
				http.Error(w, "unknown action", http.StatusBadRequest)
				return
			}
			c.Updated = now.Unix()
			break
		}
		if !found {
			http.Error(w, "unknown card", http.StatusNotFound)
			return
		}
		if err := saveAvui(s, list); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]any{"ok": true})
	default:
		http.Error(w, "GET or POST", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleWindsAvuiSettings(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, s.avuiSettings())
	case http.MethodPost:
		var req avuiSettingsReq
		if !decodeSmall(w, r, &req) {
			return
		}
		st := s.avuiSettings()
		if req.QuietFrom != "" || req.QuietTo != "" {
			if minutesOf(req.QuietFrom) < 0 || minutesOf(req.QuietTo) < 0 {
				http.Error(w, s.tr("les hores han de ser HH:MM", "times must be HH:MM"), http.StatusBadRequest)
				return
			}
			st.QuietFrom, st.QuietTo = req.QuietFrom, req.QuietTo
		}
		if req.Level == "calm" || req.Level == "normal" {
			st.Level = req.Level
		}
		if req.Tune != nil {
			st.Tune = oneLine(*req.Tune, 600)
		}
		raw, _ := json.Marshal(st)
		_ = os.MkdirAll(filepath.Dir(avuiSettingsPath(s)), 0o700)
		if err := os.WriteFile(avuiSettingsPath(s), raw, 0o600); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, st)
	default:
		http.Error(w, "GET or POST", http.StatusMethodNotAllowed)
	}
}
