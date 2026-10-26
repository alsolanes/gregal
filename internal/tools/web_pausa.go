package tools

// Motors i llocs que bloquegen els agents.
//
// DuckDuckGo demana captcha a la cinquena cerca seguida des d'una mateixa
// IP, i amb tots els vents cercant alhora (vigilàncies, rutines, rotllanes)
// això passa en segons. Insistir-hi només allarga el bloqueig: el motor que
// bloqueja queda en pausa (15 minuts, el doble cada cop seguit fins a 2
// hores, o el que digui el seu Retry-After) i la cerca fa servir els altres.
// El mateix val per a l'arxiu de Wayback quan respon 429.
//
// També hi ha el filtre de rellevància (Bing RSS de vegades torna resultats
// d'una altra cerca) i la detecció de llocs que no deixen entrar agents,
// perquè l'agent rebi un avís clar en lloc de tornar-ho a provar.

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// bloqueig és l'error d'un motor que ens ha tallat (captcha, 429, 403).
type bloqueig struct {
	motiu  string
	espera time.Duration // del Retry-After; 0 = la pausa per defecte
}

func (b *bloqueig) Error() string { return b.motiu }

func esBloqueig(err error) bool {
	var b *bloqueig
	return errors.As(err, &b)
}

const (
	pausaMinima = 15 * time.Minute
	pausaMaxima = 2 * time.Hour
)

type pausaMotor struct {
	fins  time.Time
	cops  int
	motiu string
}

var (
	pausesMu sync.Mutex
	pauses   = map[string]*pausaMotor{}
)

// PausesClear oblida totes les pauses (tests).
func PausesClear() {
	pausesMu.Lock()
	pauses = map[string]*pausaMotor{}
	pausesMu.Unlock()
}

// enPausa diu si el motor està en pausa i, si ho està, per què i fins quan.
func enPausa(nom string) (string, bool) {
	pausesMu.Lock()
	defer pausesMu.Unlock()
	p, ok := pauses[nom]
	if !ok || time.Now().After(p.fins) {
		return "", false
	}
	return fmt.Sprintf("en pausa fins a les %s (%s)", p.fins.Format("15:04"), p.motiu), true
}

// anotaResultat posa el motor en pausa si l'error és un bloqueig, i la
// treu (i en reinicia el comptador) quan respon bé.
func anotaResultat(nom string, err error) {
	pausesMu.Lock()
	defer pausesMu.Unlock()
	var b *bloqueig
	if !errors.As(err, &b) {
		if err == nil {
			delete(pauses, nom)
		}
		return
	}
	p := pauses[nom]
	if p == nil {
		p = &pausaMotor{}
		pauses[nom] = p
	}
	p.cops++
	dur := pausaMinima << (p.cops - 1)
	if p.cops > 4 || dur > pausaMaxima {
		dur = pausaMaxima
	}
	if b.espera > 0 {
		dur = min(b.espera, 24*time.Hour)
	}
	p.fins, p.motiu = time.Now().Add(dur), b.motiu
}

// bloqueigHTTP converteix els codis que volen dir «m'estàs bloquejant» en
// bloqueig; la resta, en error normal.
func bloqueigHTTP(code int, retry string) error {
	switch code {
	case 403, 429:
		return &bloqueig{motiu: fmt.Sprintf("HTTP %d", code), espera: retryAfter(retry)}
	}
	return fmt.Errorf("HTTP %d", code)
}

// ---- rellevància ----

// paraulesBuides no compten per decidir si un resultat té relació amb la
// cerca (català, castellà i anglès).
var paraulesBuides = map[string]bool{}

func init() {
	for _, w := range strings.Fields(`als amb aquest aquesta com del dels els una uns unes per que qui quin quina sobre fins
		les los las con para por como una unos unas del sus que qué cual este esta
		the and for with from what how who why when where which this that are was best`) {
		paraulesBuides[w] = true
	}
}

// pla treu accents i passa a minúscules («Mèxic» → «mexic»).
func pla(s string) string {
	var b strings.Builder
	for _, r := range norm.NFD.String(strings.ToLower(s)) {
		if unicode.Is(unicode.Mn, r) {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func termesCerca(query string) []string {
	var out []string
	for _, w := range strings.FieldsFunc(pla(query), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	}) {
		if len([]rune(w)) >= 3 && !paraulesBuides[w] {
			out = append(out, w)
		}
	}
	return out
}

// rellevants descarta els resultats que no comparteixen cap paraula de la
// consulta (ni al títol, ni al resum, ni a la URL). El canal RSS de Bing de
// vegades torna resultats d'una altra cerca: per «concerts Razzmatazz
// novembre», fòrums de golf de Florida. No s'aplica a Exa, que és semàntic.
func rellevants(query string, hits []SearchHit) []SearchHit {
	termes := termesCerca(query)
	if len(termes) == 0 {
		return hits
	}
	out := hits[:0:0]
	for _, h := range hits {
		text := pla(h.Title + " " + h.Snippet + " " + h.URL)
		for _, t := range termes {
			if strings.Contains(text, t) {
				out = append(out, h)
				break
			}
		}
	}
	return out
}

// ---- llocs que no deixen entrar agents ----

// avisBloqueig és el que rep l'agent quan un lloc no el deixa entrar: sense
// aquest avís, tornava a provar amb web_fetch i amb el navegador, i cada
// intent era un altre bloqueig.
func avisBloqueig(host string) string {
	return fmt.Sprintf("%s no deixa entrar agents automàtics (bloqueig anti-robots). No ho tornis a provar, ni amb web_fetch ni amb el navegador: fes servir el que ja tens de web_search o busca-ho en una altra font.", host)
}

// errorDeBloqueig diu si l'error de la lectura directa és d'un lloc que
// rebutja els agents (i no una pàgina inexistent o una caiguda).
func errorDeBloqueig(err error) bool {
	if err == nil {
		return false
	}
	if esBloqueig(err) {
		return true
	}
	s := err.Error()
	return strings.Contains(s, "HTTP 401") || strings.Contains(s, "HTTP 403") ||
		strings.Contains(s, "HTTP 429") || strings.Contains(s, "intersticial de bot")
}

func hostDe(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return u.Hostname()
}

// NotaAntiRobots torna l'avís per a una pàgina oberta al navegador que és
// una comprovació anti-robots («Just a moment…», «Prove your humanity»), o
// "" si és una pàgina normal.
func NotaAntiRobots(title, host string) string {
	if !isInterstitialTitle(title) {
		return ""
	}
	if host == "" {
		host = "Aquest lloc"
	}
	return "⚠ " + avisBloqueig(host)
}
