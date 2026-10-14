package knowledge

import (
	"path"
	"regexp"
	"strings"
	"sync"
	"unicode"
)

// Paraules: minúscules, sense accents, trossos de lletres i xifres. Els
// identificadors de codi es parteixen (snake_case, camelCase) però també
// queden sencers, perquè «OfficeRead» trobi tant «office» com
// «officeread».

var plegaAccents = map[rune]rune{
	'à': 'a', 'á': 'a', 'â': 'a', 'ä': 'a', 'ã': 'a', 'å': 'a',
	'è': 'e', 'é': 'e', 'ê': 'e', 'ë': 'e',
	'ì': 'i', 'í': 'i', 'î': 'i', 'ï': 'i',
	'ò': 'o', 'ó': 'o', 'ô': 'o', 'ö': 'o', 'õ': 'o', 'ø': 'o',
	'ù': 'u', 'ú': 'u', 'û': 'u', 'ü': 'u',
	'ç': 'c', 'ñ': 'n', 'ý': 'y', 'ÿ': 'y', 'ß': 's', 'œ': 'o', 'æ': 'a',
	'·': 0,
}

// stopwords: paraules buides en anglès, català i castellà (les tres
// llengües de la gent que fa servir Gregal). Curtes i comunes: no aporten
// res al rànquing i engreixen l'índex.
var stopwords = func() map[string]bool {
	m := map[string]bool{}
	for _, w := range strings.Fields(`a an and are as at be been but by can do does for from had has have he her his how i if in into is it its me my no not of on or our she so than that the their them then there these they this to was we were what when where which who why will with would you your
	al als amb aquest aquesta aquests aquestes aixo cap com de del dels el els em en entre era es esta et fa ha han hi i la les li lo los per perque pero que qui se ser si sobre son tambe te un una uns unes va van
	con cual cuando donde ella ellos en esta este esto fue han hay las le les mas mi muy nos o para pero por se sin su sus tambien un una unos y ya`) {
		m[w] = true
	}
	return m
}()

func foldRune(r rune) rune {
	r = unicode.ToLower(r)
	if f, ok := plegaAccents[r]; ok {
		return f
	}
	return r
}

// Tokens torna les paraules indexables d'un text (amb repeticions).
func Tokens(s string) []string {
	var out []string
	var raw []rune
	flush := func() {
		if len(raw) == 0 {
			return
		}
		addWord(raw, &out)
		raw = raw[:0]
	}
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			raw = append(raw, r)
			continue
		}
		flush()
	}
	flush()
	return out
}

// addWord afegeix la paraula plegada i, si és un identificador camelCase,
// també les parts.
func addWord(raw []rune, out *[]string) {
	whole := make([]rune, 0, len(raw))
	for _, r := range raw {
		if f := foldRune(r); f != 0 {
			whole = append(whole, f)
		}
	}
	w := string(whole)
	if keepToken(w) {
		*out = append(*out, w)
	}
	// camelCase: OfficeReadFull → office, read, full.
	start := 0
	var parts []string
	for i := 1; i < len(raw); i++ {
		if unicode.IsUpper(raw[i]) && unicode.IsLower(raw[i-1]) {
			parts = append(parts, string(raw[start:i]))
			start = i
		}
	}
	if len(parts) > 0 {
		parts = append(parts, string(raw[start:]))
		for _, p := range parts {
			pr := make([]rune, 0, len(p))
			for _, r := range p {
				if f := foldRune(r); f != 0 {
					pr = append(pr, f)
				}
			}
			if pw := string(pr); keepToken(pw) && pw != w {
				*out = append(*out, pw)
			}
		}
	}
}

func keepToken(w string) bool {
	n := len([]rune(w))
	if n == 0 || n > 64 {
		return false
	}
	if n == 1 && !(w[0] >= '0' && w[0] <= '9') {
		return false
	}
	return !stopwords[w]
}

// QueryTerms són les paraules d'una consulta, sense repetir.
func QueryTerms(q string) []string {
	seen := map[string]bool{}
	var out []string
	for _, t := range Tokens(q) {
		if !seen[t] {
			seen[t] = true
			out = append(out, t)
		}
	}
	return out
}

// ---- globs ----

var globCache sync.Map // patró → *regexp.Regexp

// MatchGlob diu si una ruta relativa (amb /) compleix el patró. Sense /
// el patró es compara amb el nom del fitxer (`*.md`); amb /, amb la ruta
// sencera (`docs/**`, `**/*.pdf`, `src/*.go`). Un patró acabat en / és un
// prefix de carpeta.
func MatchGlob(pattern, rel string) bool {
	pattern = strings.TrimSpace(strings.ReplaceAll(pattern, `\`, "/"))
	rel = strings.TrimPrefix(strings.ReplaceAll(rel, `\`, "/"), "./")
	if pattern == "" {
		return false
	}
	if strings.HasSuffix(pattern, "/") {
		pattern += "**"
	}
	pattern = strings.TrimPrefix(pattern, "./")
	if !strings.Contains(pattern, "/") {
		ok, _ := path.Match(strings.ToLower(pattern), strings.ToLower(path.Base(rel)))
		return ok
	}
	var re *regexp.Regexp
	if v, ok := globCache.Load(pattern); ok {
		re = v.(*regexp.Regexp)
	} else {
		var b strings.Builder
		b.WriteString("(?i)^")
		for i := 0; i < len(pattern); i++ {
			c := pattern[i]
			switch {
			case c == '*' && i+1 < len(pattern) && pattern[i+1] == '*':
				i++
				if i+1 < len(pattern) && pattern[i+1] == '/' {
					i++
					b.WriteString("(?:.*/)?")
				} else {
					b.WriteString(".*")
				}
			case c == '*':
				b.WriteString("[^/]*")
			case c == '?':
				b.WriteString("[^/]")
			default:
				b.WriteString(regexp.QuoteMeta(string(c)))
			}
		}
		b.WriteString("$")
		var err error
		re, err = regexp.Compile(b.String())
		if err != nil {
			return false
		}
		globCache.Store(pattern, re)
	}
	return re.MatchString(rel)
}

// ---- fragments per al resultat ----

// Snippet retalla el tros d'un fragment on surten més paraules de la
// consulta, amb uns max caràcters, sense tallar paraules.
func Snippet(text string, terms []string, max int) string {
	text = strings.Join(strings.Fields(text), " ")
	r := []rune(text)
	if len(r) <= max {
		return text
	}
	if len(terms) == 0 {
		return string(r[:max]) + "…"
	}
	// Posició plegada per rune: el text plegat té la mateixa llargada en
	// runes (els accents es pleguen 1 a 1; els que desapareixen, a espai).
	folded := make([]rune, len(r))
	for i, c := range r {
		f := foldRune(c)
		if f == 0 {
			f = ' '
		}
		folded[i] = f
	}
	fs := string(folded)
	// Posicions (en runes) de cada aparició.
	var hits []int
	for _, t := range terms {
		off := 0
		for {
			k := strings.Index(fs[off:], t)
			if k < 0 {
				break
			}
			byteAt := off + k
			hits = append(hits, len([]rune(fs[:byteAt])))
			off = byteAt + len(t)
			if len(hits) > 200 {
				break
			}
		}
	}
	if len(hits) == 0 {
		return string(r[:max]) + "…"
	}
	// Finestra amb més aparicions.
	best, bestN := hits[0], 0
	for _, h := range hits {
		n := 0
		for _, o := range hits {
			if o >= h && o < h+max-40 {
				n++
			}
		}
		if n > bestN {
			best, bestN = h, n
		}
	}
	start := best - max/5
	if start < 0 {
		start = 0
	}
	for start > 0 && start < len(r) && r[start-1] != ' ' {
		start--
	}
	end := start + max
	if end > len(r) {
		end = len(r)
		start = end - max
		if start < 0 {
			start = 0
		}
	}
	for end < len(r) && end > start && r[end-1] != ' ' && end-start > max-20 {
		end--
	}
	out := strings.TrimSpace(string(r[start:end]))
	if start > 0 {
		out = "…" + out
	}
	if end < len(r) {
		out += "…"
	}
	return out
}
