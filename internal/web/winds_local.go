package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"gregal/internal/companion"
	"gregal/internal/config"
	"gregal/internal/jobs"
	"gregal/internal/runs"
)

// «A prop teu»: novetats locals del lloc on és la persona (concerts, festes
// majors, esdeveniments, obertures de botigues i locals, notícies de la zona).
// Un vent ho mira amb cerques web a l'hora que la persona tria i ho deixa a
// la safata «Avui», cada targeta amb la seva font. Si la persona vol, a més
// ho diu com a veu proactiva al seu fil (respectant les hores tranquil·les i
// el mode tranquil d'«Avui»).
//
// Privadesa: res no s'activa sense consentiment explícit, i només es desa el
// nom del lloc (ciutat o barri). La ubicació del mòbil o del navegador arriba
// arrodonida a ~1 km, es tradueix a un nom (Photon, OpenStreetMap) i no es
// desa mai. «Oblida» treu el lloc, el que ja s'ha dit i la feina programada.
//
//	GET  /api/winds/local                     → {settings, topics, job}
//	POST /api/winds/local {…}                 → desa (o action: run|forget)
//	POST /api/winds/local/locate {lat,lon,source} → {place} (sense desar res)

const (
	localFile       = "winds-local.json"
	localJobPrefix  = "local-"
	localMaxItems   = 6
	localMaxSeen    = 200
	localSourceName = "local"
)

// localTopics: el catàleg de temes, en l'ordre en què surten a l'app.
var localTopics = []struct{ ID, CA, EN, Brief string }{
	{"concerts", "Concerts i música", "Concerts and music", "concerts, gigs and live music"},
	{"festes", "Festes majors i populars", "Local festivals", "festes majors, neighbourhood and town festivals, popular celebrations"},
	{"events", "Esdeveniments, fires i mercats", "Events, fairs and markets", "events, fairs, markets, exhibitions, talks, family activities"},
	{"openings", "Obertures de botigues i locals", "New shops and venues", "openings of new shops, restaurants, bars and venues (and notable closures)"},
	{"news", "Notícies locals", "Local news", "local news from the town or neighbourhood (council decisions, services, community)"},
	{"other", "Altres coses de la zona", "Other local things", "traffic cuts, public works, service notices and anything else useful nearby"},
}

func localTopicKnown(id string) bool {
	for _, t := range localTopics {
		if t.ID == id {
			return true
		}
	}
	return false
}

type localPlace struct {
	// Label: el que es mostra i el que es cerca («Gràcia, Barcelona»).
	Label   string `json:"label"`
	Region  string `json:"region,omitempty"`
	Country string `json:"country,omitempty"`
	// Source: d'on ha sortit (manual, device, browser). Mai coordenades.
	Source string `json:"source,omitempty"`
}

type localSettings struct {
	Enabled bool `json:"enabled"`
	// Consent: quan la persona va acceptar que es fes servir el seu lloc (unix).
	Consent   int64      `json:"consent,omitempty"`
	Place     localPlace `json:"place"`
	Topics    []string   `json:"topics"`
	Interests string     `json:"interests,omitempty"`
	Avoid     string     `json:"avoid,omitempty"`
	RadiusKm  int        `json:"radius_km"`
	// Frequency: daily | 2days | weekly (sempre a l'hora Time, hora local).
	Frequency string `json:"frequency"`
	Time      string `json:"time"`
	Wind      string `json:"wind,omitempty"`
	// Voice: a més de la safata, el vent ho diu al seu fil (i sona si no és
	// hora tranquil·la).
	Voice   bool  `json:"voice"`
	LastRun int64 `json:"last_run,omitempty"`
	// Seen: títols ja portats (per no repetir-los). No surt a l'API.
	Seen []string `json:"seen,omitempty"`
}

var localMu sync.Mutex

func localPath(s *Server) string { return filepath.Join(s.windStateDir(), localFile) }

func defaultLocalSettings() localSettings {
	return localSettings{Topics: []string{"concerts", "festes", "events", "openings", "news"}, RadiusKm: 10, Frequency: "daily", Time: "09:00"}
}

func (s *Server) localSettings() localSettings {
	st := defaultLocalSettings()
	if raw, err := os.ReadFile(localPath(s)); err == nil {
		_ = json.Unmarshal(raw, &st)
	}
	return normalizeLocal(st)
}

func normalizeLocal(st localSettings) localSettings {
	switch st.Frequency {
	case "daily", "2days", "weekly":
	default:
		st.Frequency = "daily"
	}
	if minutesOf(st.Time) < 0 {
		st.Time = "09:00"
	}
	if st.RadiusKm <= 0 {
		st.RadiusKm = 10
	}
	if st.RadiusKm > 100 {
		st.RadiusKm = 100
	}
	topics := []string{}
	seen := map[string]bool{}
	for _, t := range st.Topics {
		t = strings.TrimSpace(strings.ToLower(t))
		if localTopicKnown(t) && !seen[t] {
			seen[t] = true
			topics = append(topics, t)
		}
	}
	st.Topics = topics
	st.Place.Label = oneLine(st.Place.Label, 120)
	st.Place.Region = oneLine(st.Place.Region, 120)
	st.Place.Country = oneLine(st.Place.Country, 80)
	st.Interests = oneLine(st.Interests, 400)
	st.Avoid = oneLine(st.Avoid, 400)
	if len(st.Seen) > localMaxSeen {
		st.Seen = append([]string(nil), st.Seen[len(st.Seen)-localMaxSeen:]...)
	}
	return st
}

func (s *Server) saveLocalSettings(st localSettings) error {
	raw, err := json.MarshalIndent(normalizeLocal(st), "", "  ")
	if err != nil {
		return err
	}
	_ = os.MkdirAll(filepath.Dir(localPath(s)), 0o700)
	return writeFileAtomic(localPath(s), raw)
}

func (s *Server) localOwner() string {
	if s.user != nil && !s.user.Tothom {
		return s.user.Name
	}
	return ""
}

func localJobID(owner string) string {
	if owner == "" {
		owner = "me"
	}
	return localJobPrefix + strings.NewReplacer("/", "-", " ", "-").Replace(owner)
}

func isLocalJob(id string) bool { return strings.HasPrefix(id, localJobPrefix) }

// localWind: el vent que ho porta (el triat o el primer que hi hagi).
func (s *Server) localWind(id string) (companion.Wind, bool) {
	if s.winds == nil {
		return companion.Wind{}, false
	}
	if id != "" {
		if w, ok := windByRef(s, id); ok {
			return w, true
		}
	}
	if list, err := s.winds.List(); err == nil && len(list) > 0 {
		return list[0], true
	}
	return companion.Wind{}, false
}

// syncLocalJob crea, actualitza o treu la feina programada.
func (s *Server) syncLocalJob(st localSettings) error {
	id := localJobID(s.localOwner())
	if !st.Enabled {
		if s.jobs != nil && s.jobs.Has(id) {
			return s.jobs.Delete(id)
		}
		return nil
	}
	if s.jobs == nil {
		return errors.New("scheduled jobs are not available")
	}
	w, ok := s.localWind(st.Wind)
	if !ok {
		return errors.New(s.tr("primer crea un vent", "create a wind first"))
	}
	// L'hora és la de la persona; el planificador va amb el rellotge del servidor.
	hm, _ := time.Parse("15:04", st.Time)
	loc := s.loadConnections().zone()
	nowU := time.Now().In(loc)
	serverTime := time.Date(nowU.Year(), nowU.Month(), nowU.Day(), hm.Hour(), hm.Minute(), 0, 0, loc).In(time.Local).Format("15:04")
	now := time.Now()
	_, err := s.jobs.Save(jobs.Job{
		ID: id, Name: s.tr("A prop teu", "Near you") + " · " + st.Place.Label,
		Prompt: "Local news and events around " + st.Place.Label,
		Kind:   "daily", Time: serverTime, Enabled: true, Notify: false,
		Wind: w.ID, WindOwner: s.localOwner(), LastAt: &now,
	})
	return err
}

// localDue: amb freqüència de cada dos dies o setmanal, la feina diària
// només mira quan toca (amb tres hores de marge per l'hora d'estiu).
func localDue(st localSettings, now time.Time) bool {
	if st.LastRun == 0 {
		return true
	}
	gap := 24 * time.Hour
	switch st.Frequency {
	case "2days":
		gap = 48 * time.Hour
	case "weekly":
		gap = 7 * 24 * time.Hour
	}
	return now.Sub(time.Unix(st.LastRun, 0)) >= gap-3*time.Hour
}

// localForce: «Mira-ho ara» passa per davant de la freqüència.
var localForce sync.Map

// localItem és una novetat trobada pel vent.
type localItem struct {
	Section string
	Topic   string
	Title   string
	Detail  string
	URL     string
}

var localBlock = regexp.MustCompile("(?s)```local\\s*\\n(.*?)```")
var localURL = regexp.MustCompile(`https?://[^\s<>()\]]+`)

// parseLocalBlock llegeix el bloc «local» de la resposta del vent:
//
//	```local
//	ahead :: concerts :: Txarango a la Plaça del Sol, dissabte 22 h :: Gratuït · Plaça del Sol :: https://…
//	fyi :: news :: Tallen el carrer Gran de Gràcia per obres :: Fins al 30 :: https://…
//	```
//
// Una línia sense URL no és una novetat: sense font, no hi entra.
func parseLocalBlock(out string) []localItem {
	m := localBlock.FindStringSubmatch(out)
	if m == nil {
		return nil
	}
	var items []localItem
	for _, line := range strings.Split(m[1], "\n") {
		parts := strings.Split(line, "::")
		if len(parts) < 4 {
			continue
		}
		for i := range parts {
			parts[i] = strings.TrimSpace(parts[i])
		}
		u := localURL.FindString(parts[len(parts)-1])
		u = strings.TrimRight(u, ".,;")
		if u == "" {
			continue
		}
		if p, err := url.Parse(u); err != nil || p.Host == "" {
			continue
		}
		sec := strings.ToLower(strings.TrimLeft(parts[0], "-* "))
		if sec != avuiAhead {
			sec = avuiFYI
		}
		it := localItem{Section: sec, Topic: strings.ToLower(parts[1]), Title: oneLine(plainLine(parts[2]), 110), URL: u}
		if len(parts) > 4 {
			it.Detail = oneLine(plainLine(strings.Join(parts[3:len(parts)-1], " · ")), 200)
		}
		if it.Title != "" {
			items = append(items, it)
		}
	}
	return items
}

// localKey normalitza un títol per no tornar a dir el mateix.
func localKey(title string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(title) {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r > 127 {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// localPick filtra: temes demanats, no repetits, com a molt localMaxItems.
func localPick(items []localItem, st localSettings) []localItem {
	want := map[string]bool{}
	for _, t := range st.Topics {
		want[t] = true
	}
	seen := map[string]bool{}
	for _, t := range st.Seen {
		seen[localKey(t)] = true
	}
	var out []localItem
	for _, it := range items {
		if !localTopicKnown(it.Topic) {
			it.Topic = "other"
		}
		k := localKey(it.Title)
		if k == "" || seen[k] || (len(want) > 0 && !want[it.Topic]) {
			continue
		}
		seen[k] = true
		out = append(out, it)
		if len(out) == localMaxItems {
			break
		}
	}
	return out
}

func localHost(u string) string {
	p, err := url.Parse(u)
	if err != nil {
		return u
	}
	return strings.TrimPrefix(p.Hostname(), "www.")
}

// localInfoBrief és la consigna de la mirada.
func (s *Server) localInfoBrief(w companion.Wind, st localSettings, now time.Time) string {
	var b strings.Builder
	b.WriteString(companion.WindIdentity(w))
	where := st.Place.Label
	if st.Place.Region != "" && !strings.Contains(where, st.Place.Region) {
		where += " (" + st.Place.Region + ")"
	}
	if st.Place.Country != "" && !strings.Contains(where, st.Place.Country) {
		where += ", " + st.Place.Country
	}
	fmt.Fprintf(&b, "\n\nYOU ARE LOOKING FOR LOCAL INFORMATION for the user, who is around %s, within about %d km.\nIt is now %s (%s).\n", where, st.RadiusKm, now.Format("Monday 2 January 2006, 15:04"), now.Location().String())
	b.WriteString("\nTOPICS THE USER WANTS:\n")
	for _, t := range localTopics {
		for _, id := range st.Topics {
			if id == t.ID {
				b.WriteString("- " + t.ID + ": " + t.Brief + "\n")
			}
		}
	}
	if st.Interests != "" {
		b.WriteString("\nTHEY ARE ESPECIALLY INTERESTED IN: " + st.Interests + "\n")
	}
	if st.Avoid != "" {
		b.WriteString("\nNEVER BRING THEM: " + st.Avoid + "\n")
	}
	b.WriteString(`
HOW:
- Use web_search and web_fetch. Look at the town council's agenda and news, local and regional press, venue and festival pages, neighbourhood associations. Search in the local language too.
- Things that happen in the next 14 days, or news from the last 3 days. Nothing older.
- ONLY real items you read on a web page. Every item MUST carry the URL of the page where you read it. Never invent an item, a date or a URL.
- Best and most useful first, at most 6. Dates and times in the user's local time.
- If there is nothing new worth telling, reply exactly: (res) — never write a message saying you found nothing: the user hears nothing at all.

ANSWER with only this fenced block, one item per line (section :: topic :: title :: detail :: source URL):
` + "```local\nahead :: concerts :: <what, with the day> :: <when · where · price, one short line> :: https://…\nfyi :: news :: <headline> :: <why it matters to them, one line> :: https://…\n```" + `
section is "ahead" for something on a coming date and "fyi" for news or something already open. topic is one of: concerts, festes, events, openings, news, other. Write title and detail in the user's language.`)
	if len(st.Seen) > 0 {
		last := st.Seen
		if len(last) > 25 {
			last = last[len(last)-25:]
		}
		b.WriteString("\n\nALREADY TOLD (do not repeat these):\n- " + strings.Join(last, "\n- "))
	}
	b.WriteString(s.avuiFeedbackBrief(w.ID))
	b.WriteString("\n\n" + config.LanguageDirective(s.cfg.Lang()))
	return b.String()
}

// localInfoLoop és la via de cerca; els tests la substitueixen.
var localInfoLoop = func(s *Server, ctx context.Context, brief string) (exploracio, error) {
	s.mu.Lock()
	p, role := s.windRoleRef()
	s.mu.Unlock()
	return s.runReadOnlyLoopForRole(ctx, brief, 10, p, role)
}

// localVoiceText és el que diu el vent al fil quan també ho ha de dir.
func (s *Server) localVoiceText(st localSettings, items []localItem) string {
	var b strings.Builder
	b.WriteString(s.tr("A prop teu, a ", "Near you, in ") + st.Place.Label + ":\n")
	for _, it := range items {
		line := "- " + it.Title
		if it.Detail != "" {
			line += " — " + it.Detail
		}
		line += " ([" + localHost(it.URL) + "](" + it.URL + "))"
		b.WriteString(line + "\n")
	}
	return strings.TrimSpace(b.String())
}

// localCards desa les novetats a «Avui», cadascuna amb la font citada.
func (s *Server) localCards(w companion.Wind, job string, items []localItem, held bool) []string {
	now := time.Now().Unix()
	var cards []avuiCard
	var ids []string
	for _, it := range items {
		body := ""
		if it.Detail != "" {
			body = it.Detail + "\n\n"
		}
		body += s.tr("Font: ", "Source: ") + "[" + localHost(it.URL) + "](" + it.URL + ")"
		t := oneLine(it.Title, 90)
		c := avuiCard{
			ID: avuiID(), Wind: w.ID, WindName: w.Name, Source: localSourceName, Job: job,
			Section: it.Section, Title: it.Title, Why: it.Detail, Body: body,
			Actions: []avuiAction{
				{Label: s.tr("Més detalls", "Tell me more"), Prompt: s.tr("Explica'm més sobre això (font: ", "Tell me more about this (source: ") + it.URL + "): " + t},
				{Label: s.tr("Recorda-m'ho", "Remind me"), Prompt: s.tr("Recorda-m'ho el dia abans a les 18: ", "Remind me the day before at 18:00: ") + t + " (" + it.URL + ")"},
			},
			State: avuiOpen, Held: held, Created: now, Updated: now,
		}
		cards = append(cards, c)
		ids = append(ids, c.ID)
	}
	avuiMu.Lock()
	_ = saveAvui(s, append(loadAvui(s), cards...))
	avuiMu.Unlock()
	return ids
}

// executeLocalInfo és el pas de la feina «A prop teu».
func (s *Server) executeLocalInfo(ctx context.Context, job jobs.Job, _ runs.Run) (jobs.Run, error) {
	start := time.Now()
	run := jobs.Run{Wind: job.Wind, WindOwner: job.WindOwner, JobID: job.ID, JobName: job.Name, StartedAt: start}
	finish := func(status, output, errMsg string) (jobs.Run, error) {
		run.FinishedAt, run.Status, run.Output, run.Error = time.Now(), status, output, errMsg
		if s.jobs != nil {
			_ = s.jobs.AddRun(run)
			_ = s.jobs.MarkRun(job.ID, run.FinishedAt, status)
		}
		if errMsg != "" {
			return run, errors.New(errMsg)
		}
		return run, nil
	}
	localMu.Lock()
	st := s.localSettings()
	localMu.Unlock()
	_, forced := localForce.LoadAndDelete(job.ID)
	if !st.Enabled || st.Place.Label == "" || st.Consent == 0 || len(st.Topics) == 0 {
		return finish("ok", "", "")
	}
	if !forced && !localDue(st, start) {
		return finish("ok", "", "")
	}
	w, ok := s.localWind(job.Wind)
	if !ok {
		return finish("error", "", "the wind for local news is not available")
	}
	loc := s.loadConnections().zone()
	res, err := localInfoLoop(s, ctx, s.localInfoBrief(w, st, start.In(loc)))
	if err != nil {
		return finish("error", "", friendlyRunError(err.Error()))
	}
	items := localPick(parseLocalBlock(res.text), st)
	localMu.Lock()
	st = s.localSettings()
	st.LastRun = start.Unix()
	for _, it := range items {
		st.Seen = append(st.Seen, it.Title)
	}
	_ = s.saveLocalSettings(st)
	localMu.Unlock()
	if len(items) == 0 {
		return finish("ok", "", "")
	}
	// Hores tranquil·les o mode tranquil: a la safata, sense fer sonar res.
	av := s.avuiSettings()
	now := time.Now().In(loc)
	quiet := inQuietHours(now, av.QuietFrom, av.QuietTo) || av.Level == "calm"
	s.localCards(w, job.ID, items, quiet)
	text := s.localVoiceText(st, items)
	if st.Voice {
		postWindMessageLinked(s, w, localSourceName, job.ID, text, windMessageLinks{Hold: quiet})
	}
	return finish("ok", text, "")
}

// --- ubicació aproximada ---

const photonReverseURL = "https://photon.komoot.io/reverse"

// roundCoord: dues xifres decimals (~1 km). És tot el que surt del servidor.
func roundCoord(v float64) float64 { return math.Round(v*100) / 100 }

// localReverseGeocode tradueix unes coordenades arrodonides a un nom de lloc.
// Els tests la substitueixen.
var localReverseGeocode = func(lat, lon float64, lang string) (localPlace, error) {
	q := url.Values{}
	q.Set("lat", strconv.FormatFloat(lat, 'f', 2, 64))
	q.Set("lon", strconv.FormatFloat(lon, 'f', 2, 64))
	switch lang {
	case "en", "de", "fr":
		q.Set("lang", lang)
	}
	var data struct {
		Features []struct {
			Properties map[string]any `json:"properties"`
		} `json:"features"`
	}
	if err := meteoGetJSON(photonReverseURL+"?"+q.Encode(), &data); err != nil {
		return localPlace{}, err
	}
	if len(data.Features) == 0 {
		return localPlace{}, errors.New("no place found here")
	}
	return photonPlace(data.Features[0].Properties), nil
}

// photonPlace fa el nom: barri o poble + ciutat, mai el carrer ni el número.
func photonPlace(p map[string]any) localPlace {
	str := func(k string) string { v, _ := p[k].(string); return strings.TrimSpace(v) }
	city := str("city")
	area := str("district")
	if area == "" {
		area = str("locality")
	}
	if city == "" {
		city = str("county")
	}
	if city == "" && str("type") != "street" && str("type") != "house" {
		city = str("name")
	}
	label := city
	if area != "" && area != city {
		label = area + ", " + city
	}
	label = strings.Trim(label, ", ")
	return localPlace{Label: label, Region: str("state"), Country: str("country")}
}

func (s *Server) handleWindsLocalLocate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Lat    *float64 `json:"lat"`
		Lon    *float64 `json:"lon"`
		Source string   `json:"source"`
	}
	if !decodeSmall(w, r, &req) {
		return
	}
	if req.Lat == nil || req.Lon == nil || math.Abs(*req.Lat) > 90 || math.Abs(*req.Lon) > 180 {
		http.Error(w, "lat and lon are required", http.StatusBadRequest)
		return
	}
	p, err := localReverseGeocode(roundCoord(*req.Lat), roundCoord(*req.Lon), s.cfg.Lang())
	if err != nil || p.Label == "" {
		msg := s.tr("No s'ha pogut saber el nom d'aquest lloc", "Could not find the name of this place")
		if err != nil {
			msg += ": " + err.Error()
		}
		http.Error(w, msg, http.StatusBadGateway)
		return
	}
	p.Source = "device"
	if req.Source == "browser" {
		p.Source = "browser"
	}
	writeJSON(w, map[string]any{"place": p})
}

// --- API ---

type localSettingsReq struct {
	Action    string      `json:"action"`
	Enabled   *bool       `json:"enabled"`
	Consent   *bool       `json:"consent"`
	Place     *localPlace `json:"place"`
	Topics    *[]string   `json:"topics"`
	Interests *string     `json:"interests"`
	Avoid     *string     `json:"avoid"`
	RadiusKm  *int        `json:"radius_km"`
	Frequency *string     `json:"frequency"`
	Time      *string     `json:"time"`
	Wind      *string     `json:"wind"`
	Voice     *bool       `json:"voice"`
}

func (s *Server) localView() map[string]any {
	localMu.Lock()
	st := s.localSettings()
	localMu.Unlock()
	seen := len(st.Seen)
	st.Seen = nil
	lang := "ca"
	if s.cfg != nil {
		lang = s.cfg.Lang()
	}
	topics := make([]map[string]string, 0, len(localTopics))
	for _, t := range localTopics {
		label := t.CA
		if lang == "en" {
			label = t.EN
		}
		topics = append(topics, map[string]string{"id": t.ID, "label": label})
	}
	job := map[string]any{"id": localJobID(s.localOwner())}
	if s.jobs != nil {
		if list, err := s.jobs.List(); err == nil {
			for _, j := range list {
				if j.ID != job["id"] {
					continue
				}
				job["enabled"] = j.Enabled
				if j.LastAt != nil {
					job["last_at"] = j.LastAt.Format(time.RFC3339)
				}
				job["last_status"] = j.LastStatus
				if nd := jobs.NextDue(j, time.Now()); !nd.IsZero() {
					job["next_due_at"] = nd.Format(time.RFC3339)
				}
			}
		}
	}
	return map[string]any{"settings": st, "seen": seen, "topics": topics, "job": job}
}

func (s *Server) handleWindsLocal(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, s.localView())
	case http.MethodPost:
		var req localSettingsReq
		if !decodeSmall(w, r, &req) {
			return
		}
		switch req.Action {
		case "forget":
			// Fora el lloc, el consentiment, el que ja s'ha dit i la feina.
			localMu.Lock()
			st := s.localSettings()
			st.Enabled, st.Consent, st.Place, st.Seen, st.LastRun = false, 0, localPlace{}, nil, 0
			err := s.saveLocalSettings(st)
			localMu.Unlock()
			if err == nil {
				err = s.syncLocalJob(st)
			}
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			writeJSON(w, s.localView())
			return
		case "run":
			localMu.Lock()
			st := s.localSettings()
			localMu.Unlock()
			id := localJobID(s.localOwner())
			if !st.Enabled || s.jobs == nil || !s.jobs.Has(id) {
				http.Error(w, s.tr("primer activa «A prop teu»", "turn on «Near you» first"), http.StatusConflict)
				return
			}
			s.mu.Lock()
			busy := s.jobRunning
			s.mu.Unlock()
			if busy {
				http.Error(w, s.tr("hi ha una altra feina en marxa: torna-hi d'aquí a un moment", "another routine is running: try again in a moment"), http.StatusConflict)
				return
			}
			localForce.Store(id, true)
			go func() {
				defer func() { _ = recover() }()
				_, _ = s.runJob(id, true)
			}()
			writeJSON(w, map[string]any{"accepted": true})
			return
		case "":
		default:
			http.Error(w, "unknown action", http.StatusBadRequest)
			return
		}
		localMu.Lock()
		st := s.localSettings()
		if req.Place != nil {
			p := *req.Place
			p.Label = oneLine(p.Label, 120)
			switch p.Source {
			case "device", "browser":
			default:
				p.Source = "manual"
			}
			st.Place = p
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
		if req.Frequency != nil {
			st.Frequency = *req.Frequency
		}
		if req.Time != nil {
			if minutesOf(*req.Time) < 0 {
				localMu.Unlock()
				http.Error(w, s.tr("l'hora ha de ser HH:MM", "time must be HH:MM"), http.StatusBadRequest)
				return
			}
			st.Time = strings.TrimSpace(*req.Time)
		}
		if req.Wind != nil {
			st.Wind = strings.TrimSpace(*req.Wind)
		}
		if req.Voice != nil {
			st.Voice = *req.Voice
		}
		if req.Consent != nil {
			if *req.Consent {
				if st.Consent == 0 {
					st.Consent = time.Now().Unix()
				}
			} else {
				st.Consent = 0
				st.Enabled = false
			}
		}
		if req.Enabled != nil {
			st.Enabled = *req.Enabled
		}
		st = normalizeLocal(st)
		if st.Enabled {
			var problem string
			switch {
			case st.Consent == 0:
				problem = s.tr("cal el teu consentiment per fer servir el teu lloc", "your consent to use your place is required")
			case st.Place.Label == "":
				problem = s.tr("tria on ets (una ciutat o un barri)", "choose where you are (a town or a neighbourhood)")
			case len(st.Topics) == 0:
				problem = s.tr("tria almenys un tema", "choose at least one topic")
			}
			if problem != "" {
				localMu.Unlock()
				http.Error(w, problem, http.StatusBadRequest)
				return
			}
		}
		err := s.saveLocalSettings(st)
		localMu.Unlock()
		if err == nil {
			err = s.syncLocalJob(st)
		}
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, s.localView())
	default:
		http.Error(w, "GET or POST", http.StatusMethodNotAllowed)
	}
}
