package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// El calaix: el vent hi desa el que tornaràs a preguntar i ho troba a l'instant.
func TestCalaixKeepsWhatEachWindFollows(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	s, _ := companionTestServer(t)
	postJSON(t, s.handleWinds, `{"name":"Llevantada","color":"#F6C7A1","role":"Cuina i receptes"}`)
	postJSON(t, s.handleWinds, `{"name":"Garbí","color":"#9ED3C5","role":"Esports"}`)
	cook, ok := s.winds.ByID("llevantada")
	if !ok {
		t.Fatal("no cook")
	}
	// Cada ofici té els seus prestatges des del primer dia.
	if d := s.loadCalaix(cook); shelfIndex(d, "receptari") < 0 {
		t.Fatalf("the cook needs a recipe book: %+v", d)
	}
	recipe := `{"wind":"llevantada","action":"add","shelf":"receptari","item":{"title":"Escalivada","servings":4,"ingredients":["2 albergínies","2 pebrots"],"steps":["Escalivar","Pelar"]}}`
	var a calaixArgs
	_ = json.Unmarshal([]byte(recipe), &a)
	if _, err := s.calaixDo(a); err != nil {
		t.Fatal(err)
	}
	// La mateixa recepta (mateix títol) s'actualitza, no es duplica.
	a.Item["servings"] = 6
	if msg, err := s.calaixDo(a); err != nil || !strings.Contains(msg, "Updated") {
		t.Fatalf("update: %q %v", msg, err)
	}
	d := s.loadCalaix(cook)
	rec := d.Shelves[shelfIndex(d, "receptari")]
	if len(rec.Items) != 1 || fmt.Sprint(rec.Items[0]["servings"]) != "6" {
		t.Fatalf("recipe book: %+v", rec.Items)
	}
	if b := s.windCalaixBrief(cook); !strings.Contains(b, "receptari") || !strings.Contains(b, "1 rows") {
		t.Fatalf("the wind must know what it keeps: %s", b)
	}
	// L'esportiu desa la classificació sencera d'un cop.
	if _, err := s.calaixDo(calaixArgs{Wind: "garbi", Action: "put", Shelf: "classificacio", Items: []map[string]any{{"position": 1, "team": "FC Barcelona", "points": 33}, {"position": 2, "team": "Real Madrid", "points": 30}}}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.calaixDo(calaixArgs{Wind: "garbi", Action: "remove", Shelf: "classificacio", Key: "real madrid"}); err != nil {
		t.Fatal(err)
	}
	if out, _ := s.calaixDo(calaixArgs{Wind: "garbi", Action: "get", Shelf: "classificacio"}); !strings.Contains(out, "FC Barcelona") || strings.Contains(out, "Real Madrid") {
		t.Fatalf("standings: %s", out)
	}
	if _, err := s.calaixDo(calaixArgs{Wind: "ningu", Action: "list"}); err == nil {
		t.Fatal("an unknown wind has no drawer")
	}
	// El calendari va per data.
	_, _ = s.calaixDo(calaixArgs{Wind: "garbi", Action: "add", Shelf: "cal", Kind: shelfReleases, Item: map[string]any{"title": "B", "date": "2026-12-01"}})
	_, _ = s.calaixDo(calaixArgs{Wind: "garbi", Action: "add", Shelf: "cal", Item: map[string]any{"title": "A", "date": "2026-11-20"}})
	garbi, _ := s.winds.ByID("garbi")
	d = s.loadCalaix(garbi)
	if cal := d.Shelves[shelfIndex(d, "cal")]; cal.Items[0]["title"] != "A" {
		t.Fatalf("releases must be sorted by date: %+v", cal.Items)
	}
}

func TestWeatherForecastForThePrimaryPlace(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	s, _ := companionTestServer(t)
	var asked string
	meteo := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asked = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"hourly":{"time":["2026-11-10T10:00"],"temperature_2m":[17.5]},"daily":{"time":["2026-11-10"]}}`))
	}))
	defer meteo.Close()
	saved := openMeteoBase
	openMeteoBase = meteo.URL
	t.Cleanup(func() { openMeteoBase = saved })
	postJSON(t, s.handleWinds, `{"name":"Oreig","color":"#A9D3F0","role":"El temps"}`)
	for _, p := range []string{`{"name":"Girona","lat":41.98,"lon":2.82}`, `{"name":"Barcelona","lat":41.39,"lon":2.17}`} {
		var item map[string]any
		_ = json.Unmarshal([]byte(p), &item)
		if _, err := s.calaixDo(calaixArgs{Wind: "oreig", Action: "add", Shelf: "llocs", Item: item}); err != nil {
			t.Fatal(err)
		}
	}
	rec := postJSON(t, s.handleWindsCalaix, `{"wind":"oreig","shelf":"llocs","action":"primary","key":"barcelona"}`)
	if rec.Code != 200 {
		t.Fatalf("primary: %d %s", rec.Code, rec.Body.String())
	}
	get := httptest.NewRecorder()
	s.handleWindsWeather(get, httptest.NewRequest(http.MethodGet, "/api/winds/weather?wind=oreig", nil))
	if get.Code != 200 || !strings.Contains(get.Body.String(), `"name":"Barcelona"`) || !strings.Contains(get.Body.String(), "17.5") {
		t.Fatalf("forecast: %d %s", get.Code, get.Body.String())
	}
	if !strings.Contains(asked, "latitude=41.39") || !strings.Contains(asked, "forecast_days=7") {
		t.Fatalf("asked open-meteo: %s", asked)
	}
}
