package web

import (
	"context"
	"encoding/json"
	"log"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"gregal/internal/llm"
)

// Com es diuen els ajudants i quins noms se'ls proposen.
//
// La persona tria un nom col·lectiu en lloc de «vents» (fullets, minairons,
// follets…) i els noms que l'app proposa per a cada ajudant nou —al creador, a
// les colles, a les colles fetes— hi han d'encaixar: un fullet no es diu
// Tramuntana. Per als col·lectius coneguts hi ha llistes pròpies; per a un nom
// inventat, el model en proposa (en memòria cau al disc) i, si no respon, una
// llista neutra.

type windCollective struct {
	Singular string `json:"singular"`
	Plural   string `json:"plural"`
	// Asked: ja s'ha preguntat (en qualsevol dispositiu), també si va dir «ara
	// no». La web i el mòbil no tornen a preguntar.
	Asked bool `json:"asked"`
}

const (
	collectiveFile = "collective.json"
	namesCacheFile = "names-cache.json"
)

var collectiveMu sync.Mutex

func (s *Server) loadCollective() windCollective {
	var c windCollective
	raw, err := os.ReadFile(filepath.Join(s.windStateDir(), collectiveFile))
	if err == nil {
		_ = json.Unmarshal(raw, &c)
	}
	return c
}

// handleWindsCollective: GET el nom col·lectiu desat; POST {singular, plural} el canvia.
func (s *Server) handleWindsCollective(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, s.loadCollective())
	case http.MethodPost:
		var c windCollective
		if !decodeSmall(w, r, &c) {
			return
		}
		c.Singular, c.Plural = cleanCollective(c.Singular), cleanCollective(c.Plural)
		c.Asked = true
		collectiveMu.Lock()
		err := writeWindJSON(filepath.Join(s.windStateDir(), collectiveFile), c)
		collectiveMu.Unlock()
		if err != nil {
			http.Error(w, "could not save", http.StatusInternalServerError)
			return
		}
		// Els noms d'un col·lectiu inventat es preparen ja: el creador no espera.
		if c.Plural != "" && presetNames(c.Plural, "ca") == nil {
			goBackground(func() {
				ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
				defer cancel()
				for _, lang := range []string{"ca", "en"} {
					_ = s.collectiveNames(ctx, c, lang)
				}
			})
		}
		writeJSON(w, c)
	default:
		http.Error(w, "GET or POST required", http.StatusMethodNotAllowed)
	}
}

func cleanCollective(v string) string {
	v = strings.TrimSpace(v)
	if len([]rune(v)) > 24 || strings.ContainsAny(v, "\n\r<>{}") {
		return ""
	}
	return v
}

// handleWindsNames: GET ?n=8&lang=ca → noms per a ajudants nous, al gust del
// col·lectiu triat, sense els que ja es fan servir.
func (s *Server) handleWindsNames(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "GET required", http.StatusMethodNotAllowed)
		return
	}
	n, _ := strconv.Atoi(r.URL.Query().Get("n"))
	if n <= 0 || n > 24 {
		n = 8
	}
	lang := r.URL.Query().Get("lang")
	if lang != "en" {
		lang = "ca"
	}
	c := s.loadCollective()
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	pool := s.freshNames(s.collectiveNames(ctx, c, lang))
	if len(pool) > n {
		pool = pool[:n]
	}
	writeJSON(w, map[string]any{"collective": c, "names": pool})
}

// freshNames treu els noms que ja porta algun ajudant i barreja la resta.
func (s *Server) freshNames(pool []string) []string {
	used := map[string]bool{}
	if s.winds != nil {
		if ws, err := s.winds.List(); err == nil {
			for _, w := range ws {
				used[strings.ToLower(strings.TrimSpace(w.Name))] = true
			}
		}
	}
	out := make([]string, 0, len(pool))
	for _, n := range pool {
		if !used[strings.ToLower(n)] {
			out = append(out, n)
		}
	}
	rand.Shuffle(len(out), func(i, j int) { out[i], out[j] = out[j], out[i] })
	return out
}

// crewNamePool són els noms per a especialistes nous segons el col·lectiu.
func (s *Server) crewNamePool(ctx context.Context, ca bool) []string {
	lang := "en"
	if ca {
		lang = "ca"
	}
	return s.freshNames(s.collectiveNames(ctx, s.loadCollective(), lang))
}

// collectiveNameRule és la consigna de noms per als prompts que en creen.
func (s *Server) collectiveNameRule(lang string) string {
	c := s.loadCollective()
	if isWindCollective(c.Plural) {
		return "a short, original name inspired by winds and breezes (not the obvious compass names Tramuntana, Xaloc, Gregal, Ponent, Llevant, Mestral, Llebeig, Migjorn, Marinada)"
	}
	return "a short, original, playful name that fits helpers collectively called \"" + c.Plural + "\" (in " + lang + "; never the name of a character from a film, book, game or brand)"
}

func isWindCollective(plural string) bool {
	p := normCollective(plural)
	return p == "" || p == "vents" || p == "winds" || p == "vent" || p == "wind"
}

func normCollective(v string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(v)) {
		switch r {
		case 'à', 'á':
			r = 'a'
		case 'è', 'é':
			r = 'e'
		case 'í', 'ï':
			r = 'i'
		case 'ò', 'ó':
			r = 'o'
		case 'ú', 'ü':
			r = 'u'
		}
		if unicode.IsLetter(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// Llistes pròpies. Noms inventats o de la natura, cap personatge conegut.
var namePresets = map[string]map[string][]string{
	"vents": {
		"ca": {"Garbinada", "Terral", "Cerç", "Embat", "Torb", "Ventolina", "Rufaga", "Oratge", "Bufarut", "Xiroc",
			"Fiblada", "Ventijol", "Aurina", "Borrufa", "Ratxa", "Remolí", "Llevantada", "Gregalada", "Mestralet", "Brisa",
			"Ventada", "Ponentí", "Marinet", "Bufada"},
		"en": {"Zephyr", "Gust", "Squall", "Draft", "Bora", "Foehn", "Chinook", "Sirocco", "Levanter", "Breeze",
			"Whirl", "Flurry", "Gale", "Puff", "Drift", "Eddy", "Skirl", "Waft", "Billow", "Scud"},
	},
	"fullets": {
		"ca": {"Bolet", "Rovelló", "Molsa", "Gotim", "Pinyó", "Ganxet", "Fullaraca", "Avellana", "Gla", "Esbarzer",
			"Falguera", "Pinassa", "Bri", "Brot", "Gerd", "Nespra", "Saüc", "Romaní", "Ginesta", "Arç"},
		"en": {"Acorn", "Moss", "Thistle", "Bramble", "Pip", "Fern", "Sorrel", "Hazel", "Burr", "Clover",
			"Sprig", "Nettle", "Briar", "Sloe", "Rowan", "Tansy", "Bracken", "Wren"},
	},
	"minairons": {
		"ca": {"Feinet", "Pessic", "Grapat", "Esquitx", "Picarol", "Botó", "Fideu", "Molla", "Llavor", "Clauet",
			"Gra", "Tatxa", "Didal", "Agulla", "Cabdell", "Guspira", "Engrunat", "Mica", "Pinça", "Rodet"},
		"en": {"Tinker", "Pinch", "Speck", "Button", "Crumb", "Thimble", "Bead", "Rivet", "Spool", "Sprocket",
			"Nib", "Tack", "Widget", "Dab", "Grain", "Spark"},
	},
	"esperits": {
		"ca": {"Boira", "Guspira", "Ombra", "Eco", "Aura", "Rosada", "Fumerol", "Calitja", "Lluerna", "Espurna",
			"Serena", "Albada", "Capvespre", "Rierol", "Celístia", "Gebre"},
		"en": {"Mist", "Ember", "Echo", "Glimmer", "Dew", "Wisp", "Haze", "Frost", "Dusk", "Dawn",
			"Shimmer", "Gloam", "Lumen", "Rime"},
	},
}

// Àlies: les maneres de dir el mateix col·lectiu.
var presetAliases = map[string]string{
	"vents": "vents", "vent": "vents", "winds": "vents", "wind": "vents",
	"fullets": "fullets", "fullet": "fullets", "follets": "fullets", "follet": "fullets",
	"donyets": "fullets", "donyet": "fullets", "sprites": "fullets", "elves": "fullets", "elfs": "fullets",
	"minairons": "minairons", "minairo": "minairons", "helpers": "minairons",
	"esperits": "esperits", "esperit": "esperits", "spirits": "esperits", "spirit": "esperits",
}

func presetNames(plural, lang string) []string {
	key, ok := presetAliases[normCollective(plural)]
	if !ok {
		if isWindCollective(plural) {
			key = "vents"
		} else {
			return nil
		}
	}
	return append([]string(nil), namePresets[key][lang]...)
}

// Noms neutres si el model no respon per a un col·lectiu inventat.
var neutralNames = map[string][]string{
	"ca": {"Nora", "Pau", "Lluc", "Bru", "Ona", "Jana", "Biel", "Mar", "Roc", "Gal·la", "Teo", "Aina"},
	"en": {"Nova", "Milo", "Juno", "Ivy", "Otto", "Remy", "Wren", "Ada", "Felix", "Iris", "Hugo", "Luna"},
}

// namesGenerator demana noms al model; els tests el substitueixen.
var namesGenerator = func(s *Server, ctx context.Context, prompt string) (string, error) {
	s.mu.Lock()
	cfg, client := s.cfg, s.client
	p, role := s.windRoleRef()
	s.mu.Unlock()
	return petChatter(ctx, client, cfg, p, role, []llm.Message{{Role: "user", Content: prompt}})
}

var namesCacheMu sync.Mutex

// collectiveNames: la llista del col·lectiu en aquesta llengua.
func (s *Server) collectiveNames(ctx context.Context, c windCollective, lang string) []string {
	if names := presetNames(c.Plural, lang); names != nil {
		return names
	}
	key := normCollective(c.Plural) + "|" + lang
	cachePath := filepath.Join(s.windStateDir(), namesCacheFile)
	namesCacheMu.Lock()
	cache := map[string][]string{}
	if raw, err := os.ReadFile(cachePath); err == nil {
		_ = json.Unmarshal(raw, &cache)
	}
	namesCacheMu.Unlock()
	if got := cache[key]; len(got) >= 6 {
		return append([]string(nil), got...)
	}
	language := "Catalan"
	if lang == "en" {
		language = "English"
	}
	prompt := "An app lets the user call their AI helpers \"" + c.Plural + "\" (one is a \"" + c.Singular + "\"). " +
		"Invent 16 original first names for individual " + c.Plural + ": short (1-2 words, at most 14 letters), warm or playful, clearly in the spirit of \"" + c.Plural + "\", easy to say in " + language + ". " +
		"Never use the name of an existing character from films, books, classic literature, legends, games, cartoons or brands (no Gulliver, Sinbad, Hook, Jack Sparrow and the like), and no real famous people: the names must be new. " +
		"Reply with JSON only: {\"names\":[\"…\"]}"
	raw, err := namesGenerator(s, ctx, prompt)
	var out struct {
		Names []string `json:"names"`
	}
	if err == nil {
		if obj := firstJSONObject(raw); obj != "" {
			_ = json.Unmarshal([]byte(obj), &out)
		}
	} else {
		log.Printf("collective names: %s", redactError(err))
	}
	var names []string
	seen := map[string]bool{}
	for _, n := range out.Names {
		n = cleanCrewName(n)
		if n == "" || len([]rune(n)) > 14 || seen[strings.ToLower(n)] {
			continue
		}
		seen[strings.ToLower(n)] = true
		names = append(names, n)
	}
	if len(names) < 6 {
		return append([]string(nil), neutralNames[lang]...)
	}
	namesCacheMu.Lock()
	cache = map[string][]string{}
	if raw, err := os.ReadFile(cachePath); err == nil {
		_ = json.Unmarshal(raw, &cache)
	}
	cache[key] = names
	_ = writeWindJSON(cachePath, cache)
	namesCacheMu.Unlock()
	return names
}
