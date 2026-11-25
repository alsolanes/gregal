package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/url"
	"strings"
	"sync"
	"time"
)

// «Avui» d'una ullada: el temps d'avui on és la persona (Open-Meteo, sense
// clau), el que té avui i demà als calendaris connectats i el que segueix
// (winds_avui_follows.go). GET /api/winds/avui no espera mai la xarxa més
// d'un moment: serveix el que té desat i, si és vell, ho refresca de fons.
//
//	glance: {weather?, weather_error?, events:[{start, end?, title, calendar}], follows:[…]}

type glanceWeather struct {
	Place     string   `json:"place"`
	NowC      float64  `json:"now_c"`
	MinC      float64  `json:"min_c"`
	MaxC      float64  `json:"max_c"`
	RainMM    float64  `json:"rain_mm"`
	RainHours []string `json:"rain_hours"`
	Code      int      `json:"code"`
	Sunrise   string   `json:"sunrise"`
	Sunset    string   `json:"sunset"`
}

type glanceEvent struct {
	Start    string `json:"start"`
	End      string `json:"end,omitempty"`
	Title    string `json:"title"`
	Calendar string `json:"calendar"`
}

type avuiGlanceOut struct {
	Weather *glanceWeather `json:"weather,omitempty"`
	// WeatherError: per què no hi ha el temps del lloc (lloc no trobat, Open-Meteo no respon).
	WeatherError string         `json:"weather_error,omitempty"`
	Events       []glanceEvent  `json:"events"`
	Follows      []glanceFollow `json:"follows"`
}

// glanceSlot és el que s'ha portat de fora per a un compte i un lloc.
type glanceSlot struct {
	place   string
	at      time.Time
	weather *glanceWeather
	err     string
	events  []glanceEvent
	// busy no és nil mentre es refresca; es tanca en acabar.
	busy chan struct{}
}

var (
	glanceMu    sync.Mutex
	glanceSlots = map[string]*glanceSlot{}
	glanceFresh = 15 * time.Minute
	// glanceWait: el primer cop (res desat encara) el GET espera això com a molt.
	glanceWait = 3 * time.Second
)

func (s *Server) avuiGlance() avuiGlanceOut {
	st := s.avuiSettings()
	weather, werr, events := s.glanceCached(st.Place)
	if events == nil {
		events = []glanceEvent{}
	}
	return avuiGlanceOut{Weather: weather, WeatherError: werr, Events: events, Follows: s.glanceFollows(st)}
}

func (s *Server) glanceCached(place string) (*glanceWeather, string, []glanceEvent) {
	key := avuiSettingsPath(s)
	glanceMu.Lock()
	slot := glanceSlots[key]
	if slot == nil || slot.place != place {
		slot = &glanceSlot{place: place}
		glanceSlots[key] = slot
	}
	if slot.busy == nil && time.Since(slot.at) > glanceFresh {
		slot.busy = make(chan struct{})
		goBackground(func() { s.refreshGlance(slot) })
	}
	busy, cold := slot.busy, slot.at.IsZero()
	glanceMu.Unlock()
	if cold && busy != nil {
		select {
		case <-busy:
		case <-time.After(glanceWait):
		}
	}
	glanceMu.Lock()
	defer glanceMu.Unlock()
	return slot.weather, slot.err, slot.events
}

func (s *Server) refreshGlance(slot *glanceSlot) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	// Amb un calendari connectat, la preparació de les reunions es posa en marxa.
	if len(s.calendarSecrets()) > 0 {
		if err := s.ensurePrepJob(); err != nil {
			log.Printf("avui prep: %v", err)
		}
	}
	events := s.glanceEvents(ctx)
	weather, err := s.glanceWeatherFor(slot.place)
	glanceMu.Lock()
	defer glanceMu.Unlock()
	// Si Open-Meteo falla, es queda la previsió d'abans (millor que res) i es diu per què.
	slot.err = ""
	if err == nil {
		slot.weather = weather
	} else {
		slot.err = err.Error()
	}
	slot.events, slot.at = events, time.Now()
	close(slot.busy)
	slot.busy = nil
}

// glanceEvents: avui i demà, de tots els calendaris connectats.
func (s *Server) glanceEvents(ctx context.Context) []glanceEvent {
	today := startOfToday(s)
	evs, _ := s.calendarEvents(ctx, today, today.AddDate(0, 0, 2))
	out := []glanceEvent{}
	for _, e := range evs {
		g := glanceEvent{Start: e.Start.Format(time.RFC3339), Title: strings.TrimSpace(e.Summary), Calendar: e.Calendar}
		if e.End.After(e.Start) {
			g.End = e.End.Format(time.RFC3339)
		}
		out = append(out, g)
	}
	return out
}

type geoPlace struct {
	name     string
	lat, lon float64
	at       time.Time
}

var geoCached = map[string]geoPlace{}

// geocodePlace tradueix el nom del lloc a coordenades (cache 30 min).
func geocodePlace(name, lang string) (geoPlace, error) {
	key := strings.ToLower(name) + "|" + lang
	weatherMu.Lock()
	if g, ok := geoCached[key]; ok && time.Since(g.at) < 30*time.Minute {
		weatherMu.Unlock()
		return g, nil
	}
	weatherMu.Unlock()
	q := url.Values{}
	q.Set("name", name)
	q.Set("count", "1")
	q.Set("language", lang)
	var data struct {
		Results []struct {
			Name string  `json:"name"`
			Lat  float64 `json:"latitude"`
			Lon  float64 `json:"longitude"`
		} `json:"results"`
	}
	if err := meteoGetJSON(openMeteoGeoURL+"?"+q.Encode(), &data); err != nil {
		return geoPlace{}, err
	}
	if len(data.Results) == 0 {
		return geoPlace{}, errors.New("place not found: " + name)
	}
	r := data.Results[0]
	g := geoPlace{name: r.Name, lat: r.Lat, lon: r.Lon, at: time.Now()}
	weatherMu.Lock()
	geoCached[key] = g
	weatherMu.Unlock()
	return g, nil
}

func (s *Server) glanceWeatherFor(place string) (*glanceWeather, error) {
	if strings.TrimSpace(place) == "" {
		return nil, nil
	}
	g, err := geocodePlace(place, s.cfg.Lang())
	if err != nil {
		return nil, fmt.Errorf(s.tr("no he trobat el temps de «%s» (%w)", "could not find the weather for «%s» (%w)"), place, err)
	}
	data, err := forecast(g.lat, g.lon, s.loadConnections().zone().String())
	if err != nil {
		return nil, fmt.Errorf(s.tr("no he pogut portar el temps de «%s» (%w)", "could not get the weather for «%s» (%w)"), place, err)
	}
	return weatherToday(g.name, data)
}

// weatherToday treu d'una previsió d'Open-Meteo el que toca avui.
func weatherToday(place string, data map[string]any) (*glanceWeather, error) {
	raw, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	var f struct {
		Current struct {
			Temp float64 `json:"temperature_2m"`
		} `json:"current"`
		Hourly struct {
			Time   []string  `json:"time"`
			Precip []float64 `json:"precipitation"`
		} `json:"hourly"`
		Daily struct {
			Time    []string  `json:"time"`
			Code    []float64 `json:"weather_code"`
			Max     []float64 `json:"temperature_2m_max"`
			Min     []float64 `json:"temperature_2m_min"`
			Rain    []float64 `json:"precipitation_sum"`
			Sunrise []string  `json:"sunrise"`
			Sunset  []string  `json:"sunset"`
		} `json:"daily"`
	}
	if err := json.Unmarshal(raw, &f); err != nil {
		return nil, err
	}
	d := f.Daily
	if len(d.Time) == 0 || len(d.Code) == 0 || len(d.Max) == 0 || len(d.Min) == 0 || len(d.Rain) == 0 || len(d.Sunrise) == 0 || len(d.Sunset) == 0 {
		return nil, errors.New("open-meteo: no forecast for today")
	}
	w := &glanceWeather{Place: place, NowC: f.Current.Temp, MinC: d.Min[0], MaxC: d.Max[0], RainMM: d.Rain[0],
		RainHours: []string{}, Code: int(d.Code[0]), Sunrise: clockOf(d.Sunrise[0]), Sunset: clockOf(d.Sunset[0])}
	for i, t := range f.Hourly.Time {
		if strings.HasPrefix(t, d.Time[0]+"T") && len(t) >= 13 && i < len(f.Hourly.Precip) && f.Hourly.Precip[i] >= 0.1 {
			w.RainHours = append(w.RainHours, t[11:13])
		}
	}
	return w, nil
}

// clockOf: «2026-11-24T07:51» → «07:51».
func clockOf(t string) string {
	if _, hm, ok := strings.Cut(t, "T"); ok {
		return hm
	}
	return t
}
