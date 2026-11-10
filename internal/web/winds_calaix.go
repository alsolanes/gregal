package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"gregal/internal/agent"
	"gregal/internal/companion"
	"gregal/internal/llm"
)

// El calaix d'un vent: el que guarda per consultar-ho a l'instant, sense
// tornar a cercar al web. L'esportiu hi té la classificació dels equips que
// segueixes, el de música i el de llibres el calendari de llançaments, el
// cuiner el receptari amb les receptes que t'ha donat, el del temps els llocs
// que vols seguir. Cada calaix té prestatges (shelves) amb files; el vent els
// omple i els consulta amb l'eina «calaix», i l'app els ensenya al perfil amb
// la vista de cada mena (taula, calendari, receptari, llocs i el temps).
//
//	GET  /api/winds/calaix?wind=<id>              → {shelves:[shelf]}
//	POST /api/winds/calaix {wind, shelf, action, item?, key?}
//	     action: remove (una fila, per key) | clear (el prestatge) | add (una fila) | primary (lloc principal)
//	GET  /api/winds/weather?wind=<id>             → la previsió del lloc principal (Open-Meteo)
//	GET  /api/winds/weather/places?q=<lloc>       → llocs per triar (geocoding d'Open-Meteo)

const (
	calaixMaxShelves = 12
	calaixMaxItems   = 200
	calaixMaxBytes   = 400_000
)

// Les menes de prestatge que l'app sap pintar; la resta es pinten com a taula.
const (
	shelfTable    = "table"
	shelfReleases = "releases"
	shelfRecipes  = "recipes"
	shelfPlaces   = "places"
	shelfList     = "list"
)

type calaixShelf struct {
	Name    string           `json:"name"`
	Title   string           `json:"title"`
	Kind    string           `json:"kind"`
	Note    string           `json:"note,omitempty"`
	Updated int64            `json:"updated"`
	Items   []map[string]any `json:"items"`
}

type calaixData struct {
	Shelves []calaixShelf `json:"shelves"`
}

// defaultShelves són els prestatges que cada ofici té des del primer dia,
// encara buits: així el vent sap on ha de desar les coses i l'app ja els ensenya.
var defaultShelves = map[string][]calaixShelf{
	"sports":   {{Name: "classificacio", Title: "Classificació", Kind: shelfTable}, {Name: "equips", Title: "Equips que segueixes", Kind: shelfList}},
	"music":    {{Name: "llancaments", Title: "Calendari de discos", Kind: shelfReleases}, {Name: "artistes", Title: "Artistes que segueixes", Kind: shelfList}},
	"books":    {{Name: "llancaments", Title: "Calendari de llibres", Kind: shelfReleases}, {Name: "autors", Title: "Autors que segueixes", Kind: shelfList}},
	"cooking":  {{Name: "receptari", Title: "Receptari", Kind: shelfRecipes}},
	"weather":  {{Name: "llocs", Title: "Llocs que segueixes", Kind: shelfPlaces}},
	"movies":   {{Name: "estrenes", Title: "Calendari d'estrenes", Kind: shelfReleases}, {Name: "series", Title: "Sèries que segueixes", Kind: shelfList}},
	"shopping": {{Name: "desitjos", Title: "Llista de desitjos", Kind: shelfTable}},
}

var calaixMu sync.Mutex

func (s *Server) calaixPath(windID string) string {
	return filepath.Join(s.windStateDir(), "calaix", companion.SlugWind(windID)+".json")
}

// loadCalaix llegeix el calaix d'un vent, amb els prestatges del seu ofici
// encara que no hi hagi res desat.
func (s *Server) loadCalaix(w companion.Wind) calaixData {
	var d calaixData
	if raw, err := os.ReadFile(s.calaixPath(w.ID)); err == nil {
		_ = json.Unmarshal(raw, &d)
	}
	for _, def := range defaultShelves[kitOf(w)] {
		if shelfIndex(d, def.Name) < 0 {
			def.Items = []map[string]any{}
			d.Shelves = append(d.Shelves, def)
		}
	}
	for i := range d.Shelves {
		if d.Shelves[i].Items == nil {
			d.Shelves[i].Items = []map[string]any{}
		}
	}
	return d
}

func (s *Server) saveCalaix(windID string, d calaixData) error {
	raw, err := json.Marshal(d)
	if err != nil {
		return err
	}
	if len(raw) > calaixMaxBytes {
		return errors.New("the drawer is full: remove old rows first")
	}
	_ = os.MkdirAll(filepath.Dir(s.calaixPath(windID)), 0o700)
	return os.WriteFile(s.calaixPath(windID), raw, 0o600)
}

func shelfIndex(d calaixData, name string) int {
	for i, sh := range d.Shelves {
		if sh.Name == name {
			return i
		}
	}
	return -1
}

// rowKey és la clau d'una fila: id, o el primer camp que la identifica.
func rowKey(item map[string]any) string {
	for _, k := range []string{"id", "key", "title", "name", "team", "equip", "artist", "titol", "nom"} {
		if v, ok := item[k]; ok {
			if s := strings.TrimSpace(fmt.Sprint(v)); s != "" {
				return strings.ToLower(s)
			}
		}
	}
	raw, _ := json.Marshal(item)
	return string(raw)
}

func cleanShelfName(name string) string {
	name = companion.SlugWind(strings.TrimSpace(name))
	if len(name) > 40 {
		name = name[:40]
	}
	return name
}

var calaixToolSpec = llm.ToolSpec{
	Name: "calaix",
	Description: "Your drawer: what you keep for the user so you can answer at once without searching again (standings of the teams they follow, the release calendar of their artists or authors, the recipe book, the places whose weather they follow…). " +
		"Always pass your wind id. Actions: list (shelves and sizes) · get {shelf} · put {shelf, items[], title?, kind?, note?} replaces the whole shelf (use it for a table you refreshed, like standings) · add {shelf, item} adds or updates one row (same id/title replaces it) · remove {shelf, key} removes a row by its id or title · note {shelf, note}. " +
		"Kinds: table (rows with the same columns), releases (rows with date YYYY-MM-DD, title, artist or author, type, link), recipes (rows with title, servings, time, ingredients[], steps[], tips, source, added YYYY-MM-DD, reviewed YYYY-MM-DD), places (rows with name, lat, lon, primary), list. " +
		"Check your drawer BEFORE searching the web when the question is about something you keep, and keep it up to date when you learn something new.",
	Parameters: map[string]any{"type": "object", "properties": map[string]any{
		"wind":   map[string]any{"type": "string"},
		"action": map[string]any{"type": "string", "enum": []string{"list", "get", "put", "add", "remove", "note"}},
		"shelf":  map[string]any{"type": "string"},
		"title":  map[string]any{"type": "string"},
		"kind":   map[string]any{"type": "string"},
		"note":   map[string]any{"type": "string"},
		"key":    map[string]any{"type": "string"},
		"item":   map[string]any{"type": "object"},
		"items":  map[string]any{"type": "array", "items": map[string]any{"type": "object"}},
	}, "required": []string{"wind", "action"}},
}

type calaixArgs struct {
	Wind   string           `json:"wind"`
	Action string           `json:"action"`
	Shelf  string           `json:"shelf"`
	Title  string           `json:"title"`
	Kind   string           `json:"kind"`
	Note   string           `json:"note"`
	Key    string           `json:"key"`
	Item   map[string]any   `json:"item"`
	Items  []map[string]any `json:"items"`
}

// registerCalaixTool exposa l'eina als vents (com remind: el servidor de
// l'usuari arriba pel context).
func (s *Server) registerCalaixTool() {
	agent.RegisterExtraCtx(calaixToolSpec, func(ctx context.Context, argsJSON string) (string, error) {
		var a calaixArgs
		if err := json.Unmarshal([]byte(argsJSON), &a); err != nil {
			return "", errors.New("unreadable arguments")
		}
		target := s
		if cur, ok := ctx.Value(remindServerKey{}).(*Server); ok && cur != nil {
			target = cur
		}
		return target.calaixDo(a)
	})
}

func (s *Server) calaixDo(a calaixArgs) (string, error) {
	if s.winds == nil {
		return "", errors.New("winds unavailable")
	}
	w, ok := s.winds.ByID(strings.TrimSpace(a.Wind))
	if !ok {
		return "", errors.New("unknown wind id: use yours, exactly as the brief gives it")
	}
	calaixMu.Lock()
	defer calaixMu.Unlock()
	d := s.loadCalaix(w)
	shelf := cleanShelfName(a.Shelf)
	i := shelfIndex(d, shelf)
	switch a.Action {
	case "list", "":
		var b strings.Builder
		for _, sh := range d.Shelves {
			fmt.Fprintf(&b, "- %s (%s, %s): %d rows", sh.Name, sh.Title, sh.Kind, len(sh.Items))
			if sh.Updated > 0 {
				b.WriteString(", updated " + time.Unix(sh.Updated, 0).Format("2006-01-02 15:04"))
			}
			b.WriteByte('\n')
		}
		if b.Len() == 0 {
			return "Your drawer is empty. Create a shelf with put or add.", nil
		}
		return b.String(), nil
	case "get":
		if i < 0 {
			return "", fmt.Errorf("no shelf %q (use list)", shelf)
		}
		raw, _ := json.Marshal(d.Shelves[i])
		return string(raw), nil
	}
	if shelf == "" {
		return "", errors.New("shelf is required")
	}
	if i < 0 {
		if len(d.Shelves) >= calaixMaxShelves {
			return "", errors.New("too many shelves: reuse one")
		}
		kind := a.Kind
		if kind == "" {
			kind = shelfTable
		}
		title := strings.TrimSpace(a.Title)
		if title == "" {
			title = a.Shelf
		}
		d.Shelves = append(d.Shelves, calaixShelf{Name: shelf, Title: title, Kind: kind, Items: []map[string]any{}})
		i = len(d.Shelves) - 1
	}
	sh := &d.Shelves[i]
	if a.Title != "" {
		sh.Title = a.Title
	}
	if a.Kind != "" {
		sh.Kind = a.Kind
	}
	var msg string
	switch a.Action {
	case "put":
		items := a.Items
		if len(items) > calaixMaxItems {
			items = items[:calaixMaxItems]
		}
		sh.Items = items
		if a.Note != "" {
			sh.Note = oneLine(a.Note, 400)
		}
		msg = fmt.Sprintf("Saved %d rows in %s.", len(items), sh.Name)
	case "add":
		if len(a.Item) == 0 {
			return "", errors.New("item is required")
		}
		key := rowKey(a.Item)
		replaced := false
		for j := range sh.Items {
			if rowKey(sh.Items[j]) == key {
				sh.Items[j] = a.Item
				replaced = true
				break
			}
		}
		if !replaced {
			if len(sh.Items) >= calaixMaxItems {
				return "", errors.New("the shelf is full: remove old rows first")
			}
			sh.Items = append(sh.Items, a.Item)
		}
		msg = "Saved in " + sh.Name + "."
		if replaced {
			msg = "Updated in " + sh.Name + "."
		}
	case "remove":
		key := strings.ToLower(strings.TrimSpace(a.Key))
		kept := sh.Items[:0]
		removed := 0
		for _, it := range sh.Items {
			if rowKey(it) == key {
				removed++
				continue
			}
			kept = append(kept, it)
		}
		sh.Items = kept
		msg = fmt.Sprintf("Removed %d row(s) from %s.", removed, sh.Name)
	case "note":
		sh.Note = oneLine(a.Note, 400)
		msg = "Note saved."
	default:
		return "", fmt.Errorf("unknown action %q", a.Action)
	}
	sh.Updated = time.Now().Unix()
	if sh.Kind == shelfReleases {
		sortReleases(sh.Items)
	}
	if err := s.saveCalaix(w.ID, d); err != nil {
		return "", err
	}
	return msg, nil
}

// sortReleases: el calendari va per data, el que ve primer a dalt.
func sortReleases(items []map[string]any) {
	date := func(m map[string]any) string { return fmt.Sprint(m["date"]) }
	sort.SliceStable(items, func(i, j int) bool { return date(items[i]) < date(items[j]) })
}

// windCalaixBrief diu al vent què té al calaix (i, si és petit, què hi ha),
// perquè ho consulti abans de cercar i el mantingui al dia.
func (s *Server) windCalaixBrief(w companion.Wind) string {
	if s.winds == nil {
		return ""
	}
	calaixMu.Lock()
	d := s.loadCalaix(w)
	calaixMu.Unlock()
	if len(d.Shelves) == 0 {
		return "\n\nYOUR DRAWER (tool calaix): empty. Keep there what the user will ask you again (what they follow, tables, calendars, recipes)."
	}
	var b strings.Builder
	b.WriteString("\n\nYOUR DRAWER (tool calaix) — check it before searching, keep it up to date:")
	for _, sh := range d.Shelves {
		fmt.Fprintf(&b, "\n- %s «%s» (%s): %d rows", sh.Name, sh.Title, sh.Kind, len(sh.Items))
		if sh.Updated > 0 {
			b.WriteString(", updated " + time.Unix(sh.Updated, 0).Format("02/01 15:04"))
		}
		// Les llistes curtes van senceres: el vent ja sap què segueixes.
		if (sh.Kind == shelfList || sh.Kind == shelfPlaces) && len(sh.Items) > 0 && len(sh.Items) <= 12 {
			var names []string
			for _, it := range sh.Items {
				names = append(names, rowKey(it))
			}
			b.WriteString(": " + strings.Join(names, ", "))
		}
	}
	switch kitOf(w) {
	case "cooking":
		b.WriteString("\nEvery full recipe you give that the user wants to keep (or asks you to save) goes into receptari with add: title, servings, time, ingredients[], steps[], tips, source, added.")
	case "music", "books", "movies":
		b.WriteString("\nWhen you learn of an upcoming release from someone the user follows, add it to the calendar with its exact date and a link.")
	case "sports":
		b.WriteString("\nWhen you fetch standings, save them with put (rows: position, team, played, points, form) so the next question is instant.")
	case "weather":
		b.WriteString("\nThe app shows the forecast chart of the primary place. When the user names a place to follow, add it to llocs with name, lat, lon (use Open-Meteo geocoding) and primary=true if it is the main one.")
	}
	return b.String()
}

func (s *Server) handleWindsCalaix(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		wind, ok := s.winds.ByID(strings.TrimSpace(r.URL.Query().Get("wind")))
		if !ok {
			http.Error(w, "unknown wind", http.StatusBadRequest)
			return
		}
		calaixMu.Lock()
		d := s.loadCalaix(wind)
		calaixMu.Unlock()
		writeJSON(w, d)
	case http.MethodPost:
		var req struct {
			Wind   string         `json:"wind"`
			Shelf  string         `json:"shelf"`
			Action string         `json:"action"`
			Key    string         `json:"key"`
			Item   map[string]any `json:"item"`
		}
		if !decodeSmall(w, r, &req) {
			return
		}
		wind, ok := s.winds.ByID(strings.TrimSpace(req.Wind))
		if !ok {
			http.Error(w, "unknown wind", http.StatusBadRequest)
			return
		}
		switch req.Action {
		case "add", "remove":
			if _, err := s.calaixDo(calaixArgs{Wind: wind.ID, Action: req.Action, Shelf: req.Shelf, Key: req.Key, Item: req.Item}); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
		case "clear":
			if _, err := s.calaixDo(calaixArgs{Wind: wind.ID, Action: "put", Shelf: req.Shelf, Items: []map[string]any{}}); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
		case "primary":
			// Un sol lloc principal: el que mostra el gràfic del temps.
			calaixMu.Lock()
			d := s.loadCalaix(wind)
			i := shelfIndex(d, cleanShelfName(req.Shelf))
			if i >= 0 {
				key := strings.ToLower(strings.TrimSpace(req.Key))
				for _, it := range d.Shelves[i].Items {
					it["primary"] = rowKey(it) == key
				}
				d.Shelves[i].Updated = time.Now().Unix()
			}
			err := s.saveCalaix(wind.ID, d)
			calaixMu.Unlock()
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		default:
			http.Error(w, "unknown action", http.StatusBadRequest)
			return
		}
		calaixMu.Lock()
		d := s.loadCalaix(wind)
		calaixMu.Unlock()
		writeJSON(w, d)
	default:
		http.Error(w, "GET or POST", http.StatusMethodNotAllowed)
	}
}

// ---- El temps: la previsió del lloc principal, directament d'Open-Meteo ----

type weatherCache struct {
	at   time.Time
	data map[string]any
}

var (
	weatherMu     sync.Mutex
	weatherCached = map[string]weatherCache{}
	// openMeteoBase: els tests l'apunten a un servidor local.
	openMeteoBase   = "https://api.open-meteo.com"
	openMeteoGeoURL = "https://geocoding-api.open-meteo.com/v1/search"
	weatherClient   = &http.Client{Timeout: 12 * time.Second}
)

func numberOf(v any) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, true
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(x), 64)
		return f, err == nil
	case json.Number:
		f, err := x.Float64()
		return f, err == nil
	}
	return 0, false
}

// primaryPlace és el lloc principal (o el primer) del prestatge de llocs.
func primaryPlace(d calaixData) (map[string]any, bool) {
	for _, sh := range d.Shelves {
		if sh.Kind != shelfPlaces {
			continue
		}
		var first map[string]any
		for _, it := range sh.Items {
			if _, ok := numberOf(it["lat"]); !ok {
				continue
			}
			if first == nil {
				first = it
			}
			if b, _ := it["primary"].(bool); b {
				return it, true
			}
		}
		if first != nil {
			return first, true
		}
	}
	return nil, false
}

func meteoGetJSON(target string, out any) error {
	req, err := http.NewRequest(http.MethodGet, target, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "Gregal/1.0 (+https://github.com/alsolanes/gregal)")
	resp, err := weatherClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("open-meteo: %s", resp.Status)
	}
	return json.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(out)
}

// forecast demana a Open-Meteo les properes 48 hores i 7 dies (cache 20 min).
func forecast(lat, lon float64, tz string) (map[string]any, error) {
	// «Local» (o res) no és una zona que Open-Meteo entengui: «auto» fa servir la del lloc.
	if tz == "" || tz == "Local" || tz == "UTC" {
		tz = "auto"
	}
	key := fmt.Sprintf("%.3f,%.3f,%s", lat, lon, tz)
	weatherMu.Lock()
	if c, ok := weatherCached[key]; ok && time.Since(c.at) < 20*time.Minute {
		weatherMu.Unlock()
		return c.data, nil
	}
	weatherMu.Unlock()
	q := url.Values{}
	q.Set("latitude", strconv.FormatFloat(lat, 'f', 4, 64))
	q.Set("longitude", strconv.FormatFloat(lon, 'f', 4, 64))
	q.Set("current", "temperature_2m,apparent_temperature,weather_code,wind_speed_10m,precipitation,is_day")
	q.Set("hourly", "temperature_2m,precipitation_probability,precipitation,weather_code,wind_speed_10m,wind_gusts_10m")
	q.Set("daily", "weather_code,temperature_2m_max,temperature_2m_min,precipitation_probability_max,precipitation_sum,wind_gusts_10m_max,uv_index_max,sunrise,sunset")
	q.Set("forecast_hours", "48")
	q.Set("forecast_days", "7")
	q.Set("timezone", tz)
	var data map[string]any
	if err := meteoGetJSON(openMeteoBase+"/v1/forecast?"+q.Encode(), &data); err != nil {
		return nil, err
	}
	weatherMu.Lock()
	weatherCached[key] = weatherCache{at: time.Now(), data: data}
	weatherMu.Unlock()
	return data, nil
}

func (s *Server) handleWindsWeather(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "GET only", http.StatusMethodNotAllowed)
		return
	}
	q := r.URL.Query()
	var place map[string]any
	if id := strings.TrimSpace(q.Get("wind")); id != "" {
		wind, ok := s.winds.ByID(id)
		if !ok {
			http.Error(w, "unknown wind", http.StatusBadRequest)
			return
		}
		calaixMu.Lock()
		d := s.loadCalaix(wind)
		calaixMu.Unlock()
		place, _ = primaryPlace(d)
	}
	if place == nil {
		if lat, err := strconv.ParseFloat(q.Get("lat"), 64); err == nil {
			lon, _ := strconv.ParseFloat(q.Get("lon"), 64)
			place = map[string]any{"name": q.Get("name"), "lat": lat, "lon": lon}
		}
	}
	if place == nil {
		writeJSON(w, map[string]any{"place": nil})
		return
	}
	lat, _ := numberOf(place["lat"])
	lon, _ := numberOf(place["lon"])
	data, err := forecast(lat, lon, s.loadConnections().zone().String())
	if err != nil {
		http.Error(w, s.tr("No s'ha pogut obtenir la previsió: ", "Could not get the forecast: ")+err.Error(), http.StatusBadGateway)
		return
	}
	writeJSON(w, map[string]any{"place": place, "forecast": data})
}

func (s *Server) handleWindsWeatherPlaces(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "GET only", http.StatusMethodNotAllowed)
		return
	}
	name := strings.TrimSpace(r.URL.Query().Get("q"))
	if len([]rune(name)) < 2 {
		writeJSON(w, map[string]any{"places": []any{}})
		return
	}
	q := url.Values{}
	q.Set("name", name)
	q.Set("count", "6")
	q.Set("language", s.cfg.Lang())
	var data struct {
		Results []struct {
			Name    string  `json:"name"`
			Lat     float64 `json:"latitude"`
			Lon     float64 `json:"longitude"`
			Admin1  string  `json:"admin1"`
			Country string  `json:"country"`
		} `json:"results"`
	}
	if err := meteoGetJSON(openMeteoGeoURL+"?"+q.Encode(), &data); err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	out := []map[string]any{}
	for _, p := range data.Results {
		where := strings.Trim(strings.Join([]string{p.Admin1, p.Country}, ", "), ", ")
		out = append(out, map[string]any{"name": p.Name, "where": where, "lat": p.Lat, "lon": p.Lon})
	}
	writeJSON(w, map[string]any{"places": out})
}
