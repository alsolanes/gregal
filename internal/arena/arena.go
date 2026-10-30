// Package arena és la competició de prompts: dos equips idèntics, i l'única
// diferència és el fitxer .md d'estratègia de cada jugador. A cada punt de
// decisió del partit, el mateix model (el mateix per a tothom) llegeix el .md
// del seu equip i la situació, i decideix. Guanya qui ha escrit millors
// instruccions.
//
// Cada joc (NFL, Counter-Strike 2, League of Legends) implementa Game i
// Match. El motor és codi determinista amb llavor: el model només hi entra
// pels Prompt / respostes. Els jocs han de tenir contrajoc (llegir el rival,
// gestionar recursos, adaptar-se), perquè un .md clar i adaptatiu guanyi a un
// de vague.
package arena

import (
	"encoding/json"
	"regexp"
	"sort"
	"strings"
	"sync"
)

// Prompt és el que una banda veu en un punt de decisió.
type Prompt struct {
	// Situation: l'estat del partit tal com el veu aquesta banda (marcador,
	// recursos, temps) i l'historial que pot conèixer del rival. En anglès.
	Situation string `json:"situation"`
	// Schema: el JSON exacte que cal respondre, amb els valors permesos i què
	// vol dir cadascun. En anglès.
	Schema string `json:"schema"`
	// Short: una línia per al registre de decisions (p. ex. "3rd & 7 at own 35").
	Short string `json:"short"`
}

// Event és el que passa al partit entre dos punts de decisió. Data porta el
// que el visor del joc necessita per animar-ho.
type Event struct {
	N         int             `json:"n"`
	Kind      string          `json:"kind"`
	Text      string          `json:"text"`
	Highlight bool            `json:"hl,omitempty"`
	Score     [2]int          `json:"score"`
	Decision  int             `json:"decision"` // índex del punt de decisió que l'ha produït (0 = cap)
	Data      json.RawMessage `json:"data,omitempty"`
}

// Game és un tipus de joc de l'Arena.
type Game interface {
	ID() string               // "nfl", "cs2", "lol"
	Title(lang string) string // nom per a la UI
	// Rules és el que tota estratègia rep com a context fix: les regles que
	// importen i com funcionen les decisions (en anglès, concís).
	Rules() string
	// Starter és un .md d'exemple ben fet (en anglès): la plantilla educativa.
	Starter() string
	// NewMatch prepara un partit. names són els noms de les dues estratègies.
	NewMatch(seed int64, lang string, names [2]string) Match
}

// Match és un partit en marxa.
type Match interface {
	// Next porta el partit fins al proper punt de decisió i torna els prompts
	// de les dues bandes (decideixen alhora). done = el partit s'ha acabat.
	Next() (p [2]Prompt, done bool)
	// Apply resol el punt de decisió amb les dues respostes (objectes JSON
	// del model; una resposta buida o invàlida fa servir un valor per
	// defecte segur) i torna els esdeveniments fins al proper punt.
	Apply(ans [2]json.RawMessage) []Event
	Score() [2]int
	// Winner: 0 o 1, o -1 si empat.
	Winner() int
	// Meta és el que el visor necessita un sol cop (equips, mapa, plantilles).
	Meta() any
}

// Answer és el que el model respon en un punt de decisió.
type Answer struct {
	Decision json.RawMessage `json:"decision"`
	Why      string          `json:"why"`
	Rule     string          `json:"rule"` // la línia del .md que ha seguit (cita), o ""
}

// ParseAnswer treu la resposta del model (tolera text al voltant del JSON).
func ParseAnswer(raw string) Answer {
	var a Answer
	obj := firstJSONObject(raw)
	if obj == "" {
		return a
	}
	if !json.Valid([]byte(obj)) {
		obj = repairJSON(obj)
	}
	if json.Unmarshal([]byte(obj), &a) != nil || len(a.Decision) == 0 {
		// Potser ha respost la decisió sola.
		a = Answer{Decision: json.RawMessage(obj)}
	}
	// Una decisió que no és JSON vàlid trencaria tot el partit en desar-lo:
	// fora (el joc hi posa la decisió per defecte).
	if !json.Valid(a.Decision) {
		a.Decision = nil
	}
	return a
}

var trailingComma = regexp.MustCompile(`,\s*([}\]])`)

// repairJSON arregla les relliscades típiques dels models: comes de més al
// final i cometes simples.
func repairJSON(s string) string {
	s = trailingComma.ReplaceAllString(s, "$1")
	if !json.Valid([]byte(s)) && !strings.Contains(s, `"`) {
		s = strings.ReplaceAll(s, "'", `"`)
	}
	return s
}

func firstJSONObject(s string) string {
	start := strings.Index(s, "{")
	if start < 0 {
		return ""
	}
	depth, inStr, esc := 0, false, false
	for i := start; i < len(s); i++ {
		c := s[i]
		switch {
		case esc:
			esc = false
		case c == '\\' && inStr:
			esc = true
		case c == '"':
			inStr = !inStr
		case inStr:
		case c == '{':
			depth++
		case c == '}':
			depth--
			if depth == 0 {
				return s[start : i+1]
			}
		}
	}
	return ""
}

// Decide és el que juga una banda: rep el prompt i torna la resposta.
type Decide func(side int, p Prompt) json.RawMessage

// Play juga un partit sencer amb dues funcions de decisió (proves i bots).
func Play(m Match, d [2]Decide) []Event {
	var all []Event
	for {
		p, done := m.Next()
		if done {
			return all
		}
		all = append(all, m.Apply([2]json.RawMessage{d[0](0, p[0]), d[1](1, p[1])})...)
	}
}

var (
	regMu sync.Mutex
	games = map[string]Game{}
)

// Register afegeix un joc (cada paquet de joc ho fa al seu init).
func Register(g Game) {
	regMu.Lock()
	defer regMu.Unlock()
	games[g.ID()] = g
}

// Get torna un joc pel seu id.
func Get(id string) (Game, bool) {
	regMu.Lock()
	defer regMu.Unlock()
	g, ok := games[id]
	return g, ok
}

// Games torna els jocs registrats, en un ordre fix.
func Games() []Game {
	regMu.Lock()
	defer regMu.Unlock()
	out := make([]Game, 0, len(games))
	for _, g := range games {
		out = append(out, g)
	}
	order := map[string]int{"nfl": 0, "cs2": 1, "lol": 2}
	sort.Slice(out, func(i, j int) bool { return order[out[i].ID()] < order[out[j].ID()] })
	return out
}
