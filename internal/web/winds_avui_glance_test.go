package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"gregal/internal/companion"
)

// fakeOpenMeteo fa de geocodificació i de previsió: avui plou a les 15 i a
// les 16, i demà al matí (que no és d'avui).
func fakeOpenMeteo(t *testing.T, today string, block chan struct{}) *int32 {
	t.Helper()
	var geo int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if block != nil {
			<-block
		}
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/v1/search" {
			atomic.AddInt32(&geo, 1)
			_, _ = w.Write([]byte(`{"results":[{"name":"Vilafals","latitude":42.1771,"longitude":2.9133,"country":"Espanya"}]}`))
			return
		}
		tomorrow := ""
		if d, err := time.Parse("2006-01-02", today); err == nil {
			tomorrow = d.AddDate(0, 0, 1).Format("2006-01-02")
		}
		_, _ = w.Write([]byte(`{"current":{"temperature_2m":11.4},` +
			`"hourly":{"time":["` + today + `T09:00","` + today + `T15:00","` + today + `T16:00","` + tomorrow + `T10:00"],"precipitation":[0,1.2,0.4,2.0]},` +
			`"daily":{"time":["` + today + `","` + tomorrow + `"],"weather_code":[61,3],"temperature_2m_max":[14.2,15],"temperature_2m_min":[6.1,7],"precipitation_sum":[1.6,2],"sunrise":["` + today + `T07:51","x"],"sunset":["` + today + `T17:24","x"]}}`))
	}))
	t.Cleanup(srv.Close)
	oldBase, oldGeo := openMeteoBase, openMeteoGeoURL
	openMeteoBase, openMeteoGeoURL = srv.URL, srv.URL+"/v1/search"
	t.Cleanup(func() { openMeteoBase, openMeteoGeoURL = oldBase, oldGeo })
	return &geo
}

// fakeICSServer serveix un calendari de debò per HTTP; el calendari connectat
// (https) hi arriba per fetchICS.
func fakeICSServer(t *testing.T, body string) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	old := icsFetch
	icsFetch = func(ctx context.Context, _ string) (string, error) { return fetchICS(ctx, srv.URL+"/basic.ics") }
	t.Cleanup(func() { icsFetch = old })
}

func avuiSettingsPost(t *testing.T, s *Server, body string) avuiSettings {
	t.Helper()
	rec := httptest.NewRecorder()
	s.handleWindsAvuiSettings(rec, httptest.NewRequest(http.MethodPost, "/api/winds/avui/settings", strings.NewReader(body)))
	if rec.Code != 200 {
		t.Fatalf("settings %s: %d %s", body, rec.Code, rec.Body.String())
	}
	var st avuiSettings
	if err := json.Unmarshal(rec.Body.Bytes(), &st); err != nil {
		t.Fatal(err)
	}
	return st
}

type glanceOut struct {
	Weather *struct {
		Place     string   `json:"place"`
		NowC      float64  `json:"now_c"`
		MinC      float64  `json:"min_c"`
		MaxC      float64  `json:"max_c"`
		RainMM    float64  `json:"rain_mm"`
		RainHours []string `json:"rain_hours"`
		Code      int      `json:"code"`
		Sunrise   string   `json:"sunrise"`
		Sunset    string   `json:"sunset"`
	} `json:"weather"`
	Events []struct {
		Start    string `json:"start"`
		End      string `json:"end"`
		Title    string `json:"title"`
		Calendar string `json:"calendar"`
	} `json:"events"`
	Follows []struct {
		ID      string `json:"id"`
		Kind    string `json:"kind"`
		Name    string `json:"name"`
		Wind    string `json:"wind"`
		Title   string `json:"title"`
		When    string `json:"when"`
		Detail  string `json:"detail"`
		URL     string `json:"url"`
		Updated int64  `json:"updated"`
	} `json:"follows"`
}

func getGlance(t *testing.T, s *Server) glanceOut {
	t.Helper()
	rec := httptest.NewRecorder()
	s.handleWindsAvui(rec, httptest.NewRequest(http.MethodGet, "/api/winds/avui", nil))
	var body struct {
		Glance *glanceOut `json:"glance"`
	}
	if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &body) != nil || body.Glance == nil {
		t.Fatalf("GET avui: %d %s", rec.Code, rec.Body.String())
	}
	if body.Glance.Events == nil || body.Glance.Follows == nil {
		t.Fatalf("events and follows are always lists: %s", rec.Body.String())
	}
	return *body.Glance
}

func TestAvuiGlanceWeatherAndTodaysCalendar(t *testing.T) {
	s := localTestServer(t)
	today := startOfToday(s)
	fakeICSServer(t, icsWith(
		icsEvent("a@x", "Dentista", today.Add(10*time.Hour)),
		icsEvent("b@x", "Sopar amb la Marta", today.Add(24*time.Hour+20*time.Hour)),
		icsEvent("c@x", "Massa lluny", today.Add(72*time.Hour)),
	))
	if w := connPost(t, s, `{"add_calendar":{"name":"Personal","url":"https://calendar.example.org/private-glance.ics"}}`); w.Code != 200 {
		t.Fatalf("add calendar: %s", w.Body.String())
	}
	geo := fakeOpenMeteo(t, today.Format("2006-01-02"), nil)
	if st := avuiSettingsPost(t, s, `{"place":"Vilafals"}`); st.Place != "Vilafals" || st.Follows == nil {
		t.Fatalf("settings: %+v", st)
	}

	g := getGlance(t, s)
	// Només les hores de pluja que queden avui (la de demà no).
	if g.Weather == nil || fmt.Sprint(*g.Weather) != "{Vilafals 11.4 6.1 14.2 1.6 [15 16] 61 07:51 17:24}" {
		t.Fatalf("weather: %+v", g.Weather)
	}
	if got := fmt.Sprint(g.Events); len(g.Events) != 2 || !strings.Contains(got, "Dentista Personal") || !strings.Contains(got, "Sopar amb la Marta Personal") {
		t.Fatalf("today's and tomorrow's events: %s", got)
	}
	if start, err := time.Parse(time.RFC3339, g.Events[0].Start); err != nil || !start.Equal(today.Add(10*time.Hour)) {
		t.Fatalf("event times are RFC 3339: %+v", g.Events[0])
	}
	getGlance(t, s)
	if n := atomic.LoadInt32(geo); n != 1 {
		t.Fatalf("the place is geocoded once, then cached: %d", n)
	}
}

func TestAvuiGlanceNeverWaitsForASlowForecast(t *testing.T) {
	s := localTestServer(t)
	release := make(chan struct{})
	fakeOpenMeteo(t, startOfToday(s).Format("2006-01-02"), release)
	old := glanceWait
	glanceWait = 50 * time.Millisecond
	t.Cleanup(func() { glanceWait = old })
	avuiSettingsPost(t, s, `{"place":"Vilafals lent"}`)

	start := time.Now()
	if g := getGlance(t, s); g.Weather != nil {
		t.Fatalf("no forecast yet: %+v", g.Weather)
	}
	if took := time.Since(start); took > 2*time.Second {
		t.Fatalf("GET waited for the forecast: %s", took)
	}
	close(release)
	waitBackground()
	if g := getGlance(t, s); g.Weather == nil || g.Weather.NowC != 11.4 {
		t.Fatalf("the background refresh fills the glance: %+v", g.Weather)
	}
}

func TestAvuiFollowsArePerAccountAndAWindCanAddOne(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	hub := testHubWithUsers(t, "anna", "bert")
	anna := hub.SessionFor(&User{Name: "anna"}, "s1")
	bert := hub.SessionFor(&User{Name: "bert"}, "s1")

	show := func(st avuiSettings) string { return fmt.Sprintf("%s %v", st.Place, st.Follows) }
	if got := show(avuiSettingsPost(t, anna, `{"place":"Girona","follow":{"kind":"team","name":"Barça"}}`)); got != "Girona [{barca team Barça }]" {
		t.Fatalf("anna: %s", got)
	}
	if got := show(avuiSettingsPost(t, anna, `{"follow":{"kind":"concerts","name":"Rosalía"}}`)); got != "Girona [{barca team Barça } {rosalia other Rosalía }]" {
		t.Fatalf("an unknown kind is other, and the place stays: %s", got)
	}
	if got := show(bert.avuiSettings()); got != " []" {
		t.Fatalf("bert must not see anna's place or follows: %s", got)
	}
	avuiSettingsPost(t, bert, `{"follow":{"kind":"team","name":"Girona FC"}}`)
	if got := show(avuiSettingsPost(t, anna, `{"unfollow":"barca"}`)); got != "Girona [{rosalia other Rosalía }]" {
		t.Fatalf("unfollow: %s", got)
	}
	if got := show(bert.avuiSettings()); got != " [{girona-fc team Girona FC }]" {
		t.Fatalf("anna's unfollow must not touch bert: %s", got)
	}

	// «Segueix el Barça» al xat: el vent ho fa amb l'eina routine.
	w, err := anna.winds.Save(companion.Wind{Name: "Garbí", Color: "#29ADFF", Role: "esports"})
	if err != nil {
		t.Fatal(err)
	}
	out, err := anna.routineTool(routineArgs{Wind: w.ID, Action: "follow", Name: "Barça", Kind: "team"}, time.Now())
	if err != nil || !strings.Contains(out, "Barça") {
		t.Fatalf("follow from chat: %v %s", err, out)
	}
	if got := show(anna.avuiSettings()); got != "Girona [{rosalia other Rosalía } {barca team Barça "+w.ID+"}]" {
		t.Fatalf("the wind that follows it keeps it fresh: %s", got)
	}
	if _, err := anna.routineTool(routineArgs{Wind: w.ID, Action: "unfollow", Name: "Barça"}, time.Now()); err != nil || len(anna.avuiSettings().Follows) != 1 {
		t.Fatalf("unfollow from chat: %v %+v", err, anna.avuiSettings().Follows)
	}
}

func stubFollowLoop(t *testing.T, reply *string) *[]string {
	t.Helper()
	var briefs []string
	old := followLoop
	followLoop = func(_ *Server, _ context.Context, brief string) (exploracio, error) {
		briefs = append(briefs, brief)
		return exploracio{text: *reply}, nil
	}
	t.Cleanup(func() { followLoop = old })
	return &briefs
}

func TestAvuiFollowRoutineKeepsTheLastGoodAnswer(t *testing.T) {
	s := localTestServer(t)
	reply := "Ho he mirat.\n```json\n{\"title\":\"Barça – Madrid\",\"when\":\"2026-11-28T21:00:00+01:00\",\"detail\":\"Lliga, jornada 14\",\"url\":\"https://www.fcbarcelona.cat/ca/calendari\"}\n```"
	briefs := stubFollowLoop(t, &reply)
	avuiSettingsPost(t, s, `{"follow":{"kind":"team","name":"Barça"}}`)
	job, ok := s.ownWindJob(followJobID("", "barca"))
	if !ok || job.Kind != "daily" || job.Wind != gregalWindID {
		t.Fatalf("a follow gets a daily routine of its wind: %+v %v", job, ok)
	}

	run, err := s.executeJob(context.Background(), job, runsZero())
	if err != nil || run.Status != "ok" {
		t.Fatalf("run: %+v %v", run, err)
	}
	if b := (*briefs)[0]; !strings.Contains(b, "Barça") || !strings.Contains(b, `"when"`) {
		t.Fatalf("brief: %s", b)
	}
	f := getGlance(t, s).Follows
	if len(f) != 1 || f[0].Updated == 0 {
		t.Fatalf("glance follows: %+v", f)
	}
	f[0].Updated = 0
	if got := fmt.Sprint(f[0]); got != "{barca team Barça  Barça – Madrid 2026-11-28T21:00:00+01:00 Lliga, jornada 14 https://www.fcbarcelona.cat/ca/calendari 0}" {
		t.Fatalf("glance follow: %s", got)
	}

	reply = `{"title":"","when":"dissabte"}`
	if run, _ := s.executeJob(context.Background(), job, runsZero()); run.Status != "error" {
		t.Fatalf("a malformed answer is an error: %+v", run)
	}
	if f := getGlance(t, s).Follows; f[0].Title != "Barça – Madrid" {
		t.Fatalf("a malformed answer keeps the previous one: %+v", f)
	}
	if n := loadNovetats(s); len(n) != 0 {
		t.Fatalf("following sends nothing on its own: %+v", n)
	}

	avuiSettingsPost(t, s, `{"unfollow":"barca"}`)
	if _, ok := s.ownWindJob(followJobID("", "barca")); ok {
		t.Fatal("unfollowing removes its routine")
	}
}

func TestAvuiGlanceSaysWhyThereIsNoWeather(t *testing.T) {
	s := localTestServer(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"results":[]}`))
	}))
	t.Cleanup(srv.Close)
	old := openMeteoGeoURL
	openMeteoGeoURL = srv.URL
	t.Cleanup(func() { openMeteoGeoURL = old })
	avuiSettingsPost(t, s, `{"place":"Llocquenoexisteix"}`)

	rec := httptest.NewRecorder()
	s.handleWindsAvui(rec, httptest.NewRequest(http.MethodGet, "/api/winds/avui", nil))
	var body struct {
		Glance struct {
			Weather      *glanceWeather `json:"weather"`
			WeatherError string         `json:"weather_error"`
		} `json:"glance"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Glance.Weather != nil || !strings.Contains(body.Glance.WeatherError, "Llocquenoexisteix") {
		t.Fatalf("a place with no forecast says why: %s", rec.Body.String())
	}
}

func TestAvuiFollowSaysWhenItFoundNothingOrFailed(t *testing.T) {
	s := localTestServer(t)
	reply := `{"none":true}`
	stubFollowLoop(t, &reply)
	avuiSettingsPost(t, s, `{"follow":{"kind":"team","name":"Barça"}}`)
	job, _ := s.ownWindJob(followJobID("", "barca"))
	follow := func() map[string]any {
		rec := httptest.NewRecorder()
		s.handleWindsAvui(rec, httptest.NewRequest(http.MethodGet, "/api/winds/avui", nil))
		var body struct {
			Glance struct {
				Follows []map[string]any `json:"follows"`
			} `json:"glance"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || len(body.Glance.Follows) != 1 {
			t.Fatalf("GET avui: %v %s", err, rec.Body.String())
		}
		return body.Glance.Follows[0]
	}

	if run, err := s.executeJob(context.Background(), job, runsZero()); err != nil || run.Status != "ok" {
		t.Fatalf("finding nothing is a good answer: %+v %v", run, err)
	}
	if f := follow(); f["none"] != true || f["title"] != nil || f["error"] != nil {
		t.Fatalf("nothing found: %+v", f)
	}

	reply = "no és JSON"
	if run, _ := s.executeJob(context.Background(), job, runsZero()); run.Status != "error" {
		t.Fatalf("a malformed answer is an error: %+v", run)
	}
	if f := follow(); f["error"] == nil || f["error"] == "" {
		t.Fatalf("a failed look says so in the glance: %+v", f)
	}

	reply = `{"title":"Barça – Madrid","when":"2026-11-28T21:00:00+01:00"}`
	if _, err := s.executeJob(context.Background(), job, runsZero()); err != nil {
		t.Fatal(err)
	}
	if f := follow(); f["title"] != "Barça – Madrid" || f["error"] != nil || f["none"] != nil {
		t.Fatalf("a good answer clears the failure: %+v", f)
	}
}
