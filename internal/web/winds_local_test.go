package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"gregal/internal/runs"
)

func runsZero() runs.Run { return runs.Run{} }

func localPost(t *testing.T, s *Server, body string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	s.handleWindsLocal(rec, httptest.NewRequest(http.MethodPost, "/api/winds/local", strings.NewReader(body)))
	return rec
}

func stubLocalLoop(t *testing.T, replies ...string) *[]string {
	t.Helper()
	var briefs []string
	old := localInfoLoop
	i := 0
	localInfoLoop = func(_ *Server, _ context.Context, brief string) (exploracio, error) {
		briefs = append(briefs, brief)
		r := replies[min(i, len(replies)-1)]
		i++
		return exploracio{text: r}, nil
	}
	t.Cleanup(func() { waitBackground(); localInfoLoop = old })
	return &briefs
}

const localReply = "He mirat l'agenda.\n```local\n" +
	"ahead :: concerts :: Concert de Txarango a la Plaça del Sol, dissabte 22 h :: Gratuït · Plaça del Sol :: https://www.ajuntament.barcelona.cat/gracia/agenda/txarango\n" +
	"fyi :: news :: Tallen el carrer Gran de Gràcia per obres :: Fins al 30 de novembre :: https://www.elnacional.cat/gracia-obres.\n" +
	"ahead :: festes :: Festa Major sense font :: demà :: sense url\n" +
	"fyi :: openings :: Obre una llibreria al carrer Verdi :: Llibres i cafè :: https://example.org/llibreria\n" +
	"```"

func TestLocalBlockNeedsASourceForEveryItem(t *testing.T) {
	items := parseLocalBlock(localReply)
	if len(items) != 3 {
		t.Fatalf("an item without a URL must be dropped: %+v", items)
	}
	if items[0].Section != avuiAhead || items[0].Topic != "concerts" || items[0].Detail != "Gratuït · Plaça del Sol" {
		t.Fatalf("first item: %+v", items[0])
	}
	if items[1].Section != avuiFYI || items[1].URL != "https://www.elnacional.cat/gracia-obres" {
		t.Fatalf("trailing punctuation must not be part of the URL: %+v", items[1])
	}
	if parseLocalBlock("(res)") != nil {
		t.Fatal("silence has no items")
	}
}

func TestLocalPickKeepsTopicsAndNeverRepeats(t *testing.T) {
	st := defaultLocalSettings()
	st.Topics = []string{"concerts", "news"}
	st.Seen = []string{"Tallen el carrer Gran de Gràcia per obres!"}
	got := localPick(parseLocalBlock(localReply), st)
	if len(got) != 1 || got[0].Topic != "concerts" {
		t.Fatalf("only unseen items of the chosen topics: %+v", got)
	}
}

func TestLocalPlaceNameIsTownOrNeighbourhoodNeverTheStreet(t *testing.T) {
	p := photonPlace(map[string]any{"name": "Carrer de Verdi", "type": "street", "street": "Carrer de Verdi", "housenumber": "12",
		"district": "Gràcia", "city": "Barcelona", "state": "Catalunya", "country": "Espanya"})
	if p.Label != "Gràcia, Barcelona" || p.Region != "Catalunya" || strings.Contains(p.Label, "Verdi") {
		t.Fatalf("place: %+v", p)
	}
	if p := photonPlace(map[string]any{"name": "Puigcerdà", "type": "city", "state": "Catalunya"}); p.Label != "Puigcerdà" {
		t.Fatalf("town: %+v", p)
	}
	if roundCoord(41.403712) != 41.4 || roundCoord(2.156789) != 2.16 {
		t.Fatal("coordinates must be rounded to ~1 km")
	}
}

func TestLocalLocateRoundsAndStoresNothing(t *testing.T) {
	s := localTestServer(t)
	var gotLat, gotLon float64
	old := localReverseGeocode
	localReverseGeocode = func(lat, lon float64, _ string) (localPlace, error) {
		gotLat, gotLon = lat, lon
		return localPlace{Label: "Gràcia, Barcelona", Region: "Catalunya"}, nil
	}
	t.Cleanup(func() { localReverseGeocode = old })
	rec := httptest.NewRecorder()
	s.handleWindsLocalLocate(rec, httptest.NewRequest(http.MethodPost, "/api/winds/local/locate", strings.NewReader(`{"lat":41.403712,"lon":2.156789,"source":"browser"}`)))
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"label":"Gràcia, Barcelona"`) || !strings.Contains(rec.Body.String(), `"source":"browser"`) {
		t.Fatalf("locate: %d %s", rec.Code, rec.Body.String())
	}
	if gotLat != 41.4 || gotLon != 2.16 {
		t.Fatalf("exact coordinates left the server: %v,%v", gotLat, gotLon)
	}
	if _, err := os.Stat(localPath(s)); err == nil {
		t.Fatal("locating must not store anything")
	}
	rec = httptest.NewRecorder()
	s.handleWindsLocalLocate(rec, httptest.NewRequest(http.MethodPost, "/api/winds/local/locate", strings.NewReader(`{"lat":141}`)))
	if rec.Code != 400 {
		t.Fatalf("bad coordinates: %d", rec.Code)
	}
}

func TestLocalSettingsNeedConsentAndForgetClearsEverything(t *testing.T) {
	s := localTestServer(t)
	if rec := localPost(t, s, `{"enabled":true,"place":{"label":"Gràcia, Barcelona"}}`); rec.Code != 400 || !strings.Contains(rec.Body.String(), s.tr("consentiment", "consent")) {
		t.Fatalf("enabling without consent must fail: %d %s", rec.Code, rec.Body.String())
	}
	if rec := localPost(t, s, `{"enabled":true,"consent":true}`); rec.Code != 400 {
		t.Fatalf("enabling without a place must fail: %d", rec.Code)
	}
	if rec := localPost(t, s, `{"time":"9h"}`); rec.Code != 400 {
		t.Fatalf("bad time must fail: %d", rec.Code)
	}
	rec := localPost(t, s, `{"enabled":true,"consent":true,"place":{"label":"Gràcia, Barcelona","source":"device"},"topics":["concerts","news","bogus"],"radius_km":5,"frequency":"weekly","time":"18:30","voice":true}`)
	if rec.Code != 200 {
		t.Fatalf("enable: %d %s", rec.Code, rec.Body.String())
	}
	var view struct {
		Settings localSettings `json:"settings"`
		Topics   []struct{ ID, Label string }
		Job      map[string]any
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &view)
	st := view.Settings
	if !st.Enabled || st.Consent == 0 || st.RadiusKm != 5 || st.Frequency != "weekly" || len(st.Topics) != 2 || !st.Voice || st.Place.Source != "device" {
		t.Fatalf("settings: %+v", st)
	}
	if len(view.Topics) != len(localTopics) || view.Job["next_due_at"] == nil {
		t.Fatalf("view: %+v", view)
	}
	id := localJobID("")
	list, _ := s.jobs.List()
	if len(list) != 1 || list[0].ID != id || list[0].Kind != "daily" || list[0].Wind == "" {
		t.Fatalf("job: %+v", list)
	}
	// Lo que ja s'ha dit no surt a l'API.
	st.Seen = []string{"Una cosa"}
	_ = s.saveLocalSettings(st)
	if raw, _ := json.Marshal(s.localView()); strings.Contains(string(raw), "Una cosa") {
		t.Fatal("seen titles must not leave the server")
	}
	// Retirar el consentiment l'apaga.
	if rec := localPost(t, s, `{"consent":false}`); rec.Code != 200 || s.localSettings().Enabled || s.jobs.Has(id) {
		t.Fatalf("withdrawing consent must turn it off: %d", rec.Code)
	}
	if rec := localPost(t, s, `{"action":"forget"}`); rec.Code != 200 {
		t.Fatalf("forget: %d", rec.Code)
	}
	if st := s.localSettings(); st.Place.Label != "" || st.Consent != 0 || len(st.Seen) != 0 || st.Enabled {
		t.Fatalf("forget must clear the place, consent and history: %+v", st)
	}
	raw, _ := os.ReadFile(localPath(s))
	if strings.Contains(string(raw), "lat") || strings.Contains(string(raw), "Gràcia") {
		t.Fatalf("no place left on disk: %s", raw)
	}
}

func localTestServer(t *testing.T) *Server {
	t.Helper()
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	return remindTestServer(t)
}

func TestLocalRunBringsSourcedCardsToTodayAndVoiceRespectsQuiet(t *testing.T) {
	s := localTestServer(t)
	briefs := stubLocalLoop(t, localReply)
	if rec := localPost(t, s, `{"enabled":true,"consent":true,"place":{"label":"Gràcia, Barcelona","region":"Catalunya"},"topics":["concerts","news","openings"],"interests":"jazz","avoid":"futbol","voice":true}`); rec.Code != 200 {
		t.Fatalf("enable: %d %s", rec.Code, rec.Body.String())
	}
	// Mode tranquil: a la safata i al fil, però no sona.
	rec := httptest.NewRecorder()
	s.handleWindsAvuiSettings(rec, httptest.NewRequest(http.MethodPost, "/api/winds/avui/settings", strings.NewReader(`{"level":"calm"}`)))
	job, _ := s.ownWindJob(localJobID(""))
	run, err := s.executeLocalInfo(context.Background(), job, runsZero())
	if err != nil || run.Status != "ok" || !strings.Contains(run.Output, "Txarango") {
		t.Fatalf("run: %+v %v", run, err)
	}
	b := (*briefs)[0]
	for _, want := range []string{"Gràcia, Barcelona (Catalunya)", "10 km", "concerts", "jazz", "futbol", "URL"} {
		if !strings.Contains(b, want) {
			t.Fatalf("brief lacks %q:\n%s", want, b)
		}
	}
	var local []avuiCard
	for _, c := range loadAvui(s) {
		if c.Source == "local" {
			local = append(local, c)
		}
	}
	if len(local) != 3 {
		t.Fatalf("one card per sourced item, and no extra card for the voice: %+v", loadAvui(s))
	}
	for _, c := range local {
		if !strings.Contains(c.Body, s.tr("Font: [", "Source: [")) || !strings.Contains(c.Body, "](https://") || !c.Held {
			t.Fatalf("card must cite its source and wait quietly: %+v", c)
		}
	}
	if local[0].Section != avuiAhead || local[1].Section != avuiFYI {
		t.Fatalf("sections: %+v", local)
	}
	var nov []windNovetat
	for _, n := range loadNovetats(s) {
		if n.Source == "local" {
			nov = append(nov, n)
		}
	}
	if len(nov) != 1 || nov[0].Source != "local" || !nov[0].Quiet || !strings.Contains(nov[0].Text, "[elnacional.cat](https://www.elnacional.cat/gracia-obres)") {
		t.Fatalf("the voice must go to the thread without ringing: %+v", nov)
	}
	// Una segona mirada el mateix dia no toca (freqüència), i forçada no repeteix res.
	if run, _ := s.executeLocalInfo(context.Background(), job, runsZero()); run.Output != "" || len(*briefs) != 1 {
		t.Fatalf("not due yet: %+v", run)
	}
	localForce.Store(job.ID, true)
	if run, _ := s.executeLocalInfo(context.Background(), job, runsZero()); run.Output != "" || len(*briefs) != 2 {
		t.Fatalf("a forced look must not repeat what was told: %+v", run)
	}
	if !strings.Contains((*briefs)[1], "ALREADY TOLD") {
		t.Fatal("the second brief must list what was already told")
	}
}

func TestLocalDueFollowsFrequency(t *testing.T) {
	now := time.Now()
	st := defaultLocalSettings()
	if !localDue(st, now) {
		t.Fatal("never run: due")
	}
	st.LastRun = now.Add(-22 * time.Hour).Unix()
	if !localDue(st, now) {
		t.Fatal("daily: due after ~a day")
	}
	st.Frequency = "2days"
	if localDue(st, now) {
		t.Fatal("2days: not after one day")
	}
	st.Frequency = "weekly"
	st.LastRun = now.Add(-6 * 24 * time.Hour).Unix()
	if localDue(st, now) {
		t.Fatal("weekly: not after six days")
	}
}

// Si no troba res a prop (o res que sigui nou), calla del tot: cap targeta,
// cap missatge al fil i cap avís.
func TestLocalNothingFoundStaysSilent(t *testing.T) {
	for _, reply := range []string{"(res)", "No he trobat res d'interessant a la zona.", "```local\nfyi :: news :: Sense font :: res :: sense url\n```"} {
		s := localTestServer(t)
		stubLocalLoop(t, reply)
		if rec := localPost(t, s, `{"enabled":true,"consent":true,"place":{"label":"Vic"},"topics":["news"],"voice":true}`); rec.Code != 200 {
			t.Fatalf("enable: %d %s", rec.Code, rec.Body.String())
		}
		job, _ := s.ownWindJob(localJobID(""))
		run, err := s.executeLocalInfo(context.Background(), job, runsZero())
		if err != nil || run.Status != "ok" || run.Output != "" {
			t.Fatalf("%q: run must be a silent ok: %+v %v", reply, run, err)
		}
		for _, c := range loadAvui(s) {
			if c.Source == "local" {
				t.Fatalf("%q: no card when nothing is found: %+v", reply, c)
			}
		}
		for _, n := range loadNovetats(s) {
			if n.Source == "local" {
				t.Fatalf("%q: no voice or notification when nothing is found: %+v", reply, n)
			}
		}
	}
}
