package tools

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

// Cascada de coincidència per a edit i patch.
//
// El model copia el bloc d'una lectura feta fa uns quants passos, i el copia
// malament de maneres previsibles: espais al final de línia, quatre espais on
// el fitxer té un tabulador, cometes tipogràfiques convertides en rectes, una
// línia del mig reescrita de memòria. A Aider, sense aplicació flexible hi ha
// nou vegades més errors d'edició; opencode, Codex, Cline i Roo fan tots una
// cascada semblant a aquesta.
//
// Cada nivell és més permissiu que l'anterior i s'atura al primer que troba
// alguna cosa: si troba un sol bloc l'aplica, si en troba més d'un és ambigu
// i NO baixa al següent (un nivell més fluix encara en trobaria més).
//
// L'ordre és el de l'informe amb una diferència: la indentació relativa va
// abans de les línies retallades, perquè és més estricta (casa línia a línia
// un cop treta la indentació comuna) i, posada després, no s'hi arribaria
// mai. La reindentació del bloc nou, que és el que la fa útil, s'aplica
// igualment a tots els nivells aproximats.

// Noms dels nivells, tal com els veu el model.
const (
	nivExacte      = "exacte"
	nivEspaisFinal = "espais finals"
	nivIndentacio  = "indentació relativa"
	nivRetallat    = "línies retallades"
	nivUnicode     = "Unicode normalitzat"
	nivAncores     = "àncores primera/última línia"
)

// llindarAncores és la semblança mínima del mig del bloc per al nivell
// d'àncores (la mateixa que opencode).
const llindarAncores = 0.65

// errAmbigu i errNoTrobat es distingeixen perquè patch i edit els expliquen
// cadascun a la seva manera.
type errAmbigu struct {
	n      int
	nivell string
}

func (e errAmbigu) Error() string {
	s := fmt.Sprintf("bloc ambigu (%d ocurrències", e.n)
	if e.nivell != nivExacte {
		s += ", amb coincidència «" + e.nivell + "»"
	}
	return s + "): posa-hi més context perquè n'hi hagi una de sola"
}

// substitueix aplica old→new sobre text. text, old i new ja venen sense \r.
// Retorna el text nou i el nivell que ha casat.
func substitueix(text, old, new string) (string, string, error) {
	if old == "" {
		return "", "", fmt.Errorf("old buit: per crear o reescriure un fitxer sencer fes servir write")
	}
	switch n := strings.Count(text, old); {
	case n == 1:
		return strings.Replace(text, old, new, 1), nivExacte, nil
	case n > 1:
		return "", "", errAmbigu{n: n, nivell: nivExacte}
	}
	lines := strings.Split(text, "\n")
	b := preparaBloc(old, new)
	if len(b.old) == 0 {
		return "", "", errNoTrobat
	}
	for _, niv := range []struct {
		nom string
		cas func([]string, blocVell) []int
	}{
		{nivEspaisFinal, casaPerLinia(func(s string) string { return strings.TrimRight(s, " \t") })},
		{nivIndentacio, casaIndentacioRelativa},
		{nivRetallat, casaPerLinia(strings.TrimSpace)},
		{nivUnicode, casaPerLinia(func(s string) string { return normalitzaUnicode(strings.TrimSpace(s)) })},
	} {
		ini := niv.cas(lines, b)
		if len(ini) > 1 {
			return "", "", errAmbigu{n: len(ini), nivell: niv.nom}
		}
		if len(ini) == 1 {
			return b.aplica(lines, ini[0], ini[0]+len(b.old), niv.nom)
		}
	}
	ini, fi, n := casaAncores(lines, b)
	if n > 1 {
		return "", "", errAmbigu{n: n, nivell: nivAncores}
	}
	if n == 1 {
		return b.aplica(lines, ini, fi, nivAncores)
	}
	// El model ha copiat el bloc de la sortida de read amb els «12|» del
	// davant. Es treuen i es torna a provar tota la cascada.
	if o, ok := senseNumeros(old); ok {
		nw, _ := senseNumeros(new)
		if nw == "" && new != "" {
			nw = new
		}
		nou, niv, err := substitueix(text, o, nw)
		if err == nil {
			niv = nivNumeros + " + " + niv
		}
		return nou, niv, err
	}
	return "", "", errNoTrobat
}

const nivNumeros = "números de línia de read"

// reNumero és el prefix que posa read a cada línia.
var reNumero = regexp.MustCompile(`^\s*\d+\|`)

// senseNumeros treu els «N|» del davant si TOTES les línies no buides en
// porten; si no, no toca res (un «1|» pot ser codi de debò).
func senseNumeros(s string) (string, bool) {
	ls := strings.Split(s, "\n")
	hi := false
	for i, l := range ls {
		if strings.TrimSpace(l) == "" {
			continue
		}
		loc := reNumero.FindStringIndex(l)
		if loc == nil {
			return "", false
		}
		ls[i] = l[loc[1]:]
		hi = true
	}
	return strings.Join(ls, "\n"), hi
}

var errNoTrobat = fmt.Errorf("bloc no trobat")

// blocVell és old partit en línies, sense les línies en blanc dels extrems
// (els models n'hi afegeixen o n'hi treuen sovint), amb el bloc nou a joc.
type blocVell struct {
	old, new []string
}

func preparaBloc(old, new string) blocVell {
	finalNL := strings.HasSuffix(old, "\n")
	old = strings.TrimSuffix(old, "\n")
	if finalNL {
		new = strings.TrimSuffix(new, "\n")
	}
	o := strings.Split(old, "\n")
	var n []string
	if new != "" {
		n = strings.Split(new, "\n")
	}
	for len(o) > 0 && strings.TrimSpace(o[0]) == "" {
		o = o[1:]
		if len(n) > 0 && strings.TrimSpace(n[0]) == "" {
			n = n[1:]
		}
	}
	for len(o) > 0 && strings.TrimSpace(o[len(o)-1]) == "" {
		o = o[:len(o)-1]
		if len(n) > 0 && strings.TrimSpace(n[len(n)-1]) == "" {
			n = n[:len(n)-1]
		}
	}
	return blocVell{old: o, new: n}
}

// aplica substitueix les línies [ini, fi) pel bloc nou, reindentat amb la
// indentació que té el fitxer, després de passar la guarda de proporció.
func (b blocVell) aplica(lines []string, ini, fi int, nivell string) (string, string, error) {
	trobat := lines[ini:fi]
	if desproporcionat(strings.Join(trobat, "\n"), strings.Join(b.old, "\n")) {
		return "", "", fmt.Errorf("el bloc que casa (línies %d–%d, nivell «%s») és molt més gran que old: no s'aplica. Torna a llegir el fitxer i copia old sencer i exacte", ini+1, fi, nivell)
	}
	nou := reindenta(b.new, b.old, trobat)
	out := make([]string, 0, len(lines)-len(trobat)+len(nou))
	out = append(append(append(out, lines[:ini]...), nou...), lines[fi:]...)
	return strings.Join(out, "\n"), nivell, nil
}

// desproporcionat és la guarda d'opencode (isDisproportionateMatch): un
// nivell aproximat que casa un tros molt més llarg que el que el model ha
// escrit està endevinant, i el que s'hi esborraria no ho ha vist ningú.
func desproporcionat(trobat, old string) bool {
	lo, lt := strings.Count(old, "\n")+1, strings.Count(trobat, "\n")+1
	if lt >= max(lo+3, lo*2) {
		return true
	}
	if lo == 1 {
		return false
	}
	to, tt := len(strings.TrimSpace(old)), len(strings.TrimSpace(trobat))
	return tt > max(to+500, to*4)
}

// casaPerLinia fa un nivell que compara línia a línia després de passar-les
// per norm. Retorna l'inici de totes les finestres que casen.
func casaPerLinia(norm func(string) string) func([]string, blocVell) []int {
	return func(lines []string, b blocVell) []int {
		o := make([]string, len(b.old))
		for i, l := range b.old {
			o[i] = norm(l)
		}
		f := make([]string, len(lines))
		for i, l := range lines {
			f[i] = norm(l)
		}
		var res []int
		for i := 0; i+len(o) <= len(f); i++ {
			ok := true
			for k := range o {
				if f[i+k] != o[k] {
					ok = false
					break
				}
			}
			if ok {
				res = append(res, i)
			}
		}
		return res
	}
}

// casaIndentacioRelativa casa si, tret el sagnat comú de cada bloc, les
// línies són iguals: el model ha copiat el bloc bé però un o més nivells
// més amunt o més avall, o amb espais on hi havia tabuladors.
func casaIndentacioRelativa(lines []string, b blocVell) []int {
	o := dedent(b.old)
	var res []int
	for i := 0; i+len(o) <= len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "" && strings.TrimSpace(b.old[0]) != "" {
			continue
		}
		f := dedent(lines[i : i+len(o)])
		ok := true
		for k := range o {
			if f[k] != o[k] {
				ok = false
				break
			}
		}
		if ok {
			res = append(res, i)
		}
	}
	return res
}

// dedent treu el sagnat comú i els espais finals, i expressa el sagnat que
// queda en nivells (un tabulador = un nivell, i n espais = un nivell, on n és
// el pas més petit del bloc) perquè quatre espais i un tabulador casin.
func dedent(ls []string) []string {
	ind := make([]int, len(ls))
	min, pas := -1, 0
	for i, l := range ls {
		if strings.TrimSpace(l) == "" {
			continue
		}
		ind[i] = ampladaSagnat(l)
		if min < 0 || ind[i] < min {
			min = ind[i]
		}
	}
	for i, l := range ls {
		if d := ind[i] - min; strings.TrimSpace(l) != "" && d > 0 && (pas == 0 || d < pas) {
			pas = d
		}
	}
	if pas == 0 {
		pas = 1
	}
	out := make([]string, len(ls))
	for i, l := range ls {
		t := strings.TrimSpace(l)
		if t == "" {
			continue
		}
		out[i] = strings.Repeat(">", (ind[i]-min)/pas) + t
	}
	return out
}

// ampladaSagnat compta el sagnat amb el tabulador a 4 columnes.
func ampladaSagnat(l string) int {
	n := 0
	for _, r := range l {
		switch r {
		case ' ':
			n++
		case '\t':
			n += 4
		default:
			return n
		}
	}
	return n
}

func sagnat(l string) string {
	return l[:len(l)-len(strings.TrimLeft(l, " \t"))]
}

// casaAncores és el darrer recurs (BlockAnchorReplacer d'opencode): blocs de
// tres línies o més on la primera i l'última línia hi són tal qual (sense
// comptar espais) i el mig s'assembla prou. Un model que ha reescrit de
// memòria una línia del mig d'una funció la troba igualment; un que s'ha
// inventat el bloc, no. Les línies de més o de menys compten com a zero,
// així que un bloc amb línies inventades no hi passa.
func casaAncores(lines []string, b blocVell) (ini, fi, n int) {
	o := b.old
	if len(o) < 3 {
		return 0, 0, 0
	}
	primera := normalitzaUnicode(strings.TrimSpace(o[0]))
	darrera := normalitzaUnicode(strings.TrimSpace(o[len(o)-1]))
	if primera == "" || darrera == "" {
		return 0, 0, 0
	}
	delta := max(1, len(o)/4)
	for i := range lines {
		if normalitzaUnicode(strings.TrimSpace(lines[i])) != primera {
			continue
		}
		// La darrera àncora es busca només on el bloc tindria una mida
		// semblant a old: un «}» de més amunt no tanca aquest bloc.
		millor, mj := -1.0, -1
		for j := max(i+2, i+len(o)-1-delta); j < len(lines) && j-i+1 <= len(o)+delta; j++ {
			if normalitzaUnicode(strings.TrimSpace(lines[j])) != darrera {
				continue
			}
			if s := semblancaMig(o, lines[i:j+1]); s > millor {
				millor, mj = s, j
			}
		}
		if mj >= 0 && millor >= llindarAncores {
			n++
			ini, fi = i, mj+1
		}
	}
	return ini, fi, n
}

// semblancaMig compara les línies del mig una a una; les que sobren en un
// costat compten com a zero.
func semblancaMig(o, f []string) float64 {
	mo, mf := o[1:len(o)-1], f[1:len(f)-1]
	total := max(len(mo), len(mf))
	if total == 0 {
		return 1
	}
	var s float64
	for k := 0; k < min(len(mo), len(mf)); k++ {
		s += ratio(strings.TrimSpace(mo[k]), strings.TrimSpace(mf[k]))
	}
	return s / float64(total)
}

// reindenta passa el bloc nou al sagnat del fitxer. Les parelles (línia
// d'old, línia del fitxer) diuen quin sagnat del model correspon a quin del
// fitxer; les línies noves amb un sagnat que no surt a old es resolen pel
// prefix més llarg conegut, convertint la resta entre espais i tabuladors si
// cal. Quan el model ja ha copiat bé el sagnat, no canvia res.
func reindenta(nou, old, fitxer []string) []string {
	mapa := map[string]string{}
	identitat := true
	for k := 0; k < min(len(old), len(fitxer)); k++ {
		if strings.TrimSpace(old[k]) == "" || strings.TrimSpace(fitxer[k]) == "" {
			continue
		}
		so, sf := sagnat(old[k]), sagnat(fitxer[k])
		if _, ja := mapa[so]; !ja {
			mapa[so] = sf
			if so != sf {
				identitat = false
			}
		}
	}
	if identitat || len(mapa) == 0 {
		return nou
	}
	tabs, unitat := unitatSagnat(fitxer)
	pasModel := pasEspais(old)
	out := make([]string, len(nou))
	for i, l := range nou {
		if strings.TrimSpace(l) == "" {
			out[i] = l
			continue
		}
		s := sagnat(l)
		if v, ok := mapa[s]; ok {
			out[i] = v + l[len(s):]
			continue
		}
		millor := ""
		trobat := false
		for k := range mapa {
			if strings.HasPrefix(s, k) && len(k) >= len(millor) {
				millor, trobat = k, true
			}
		}
		if !trobat {
			out[i] = l
			continue
		}
		extra := s[len(millor):]
		if tabs {
			extra = strings.ReplaceAll(extra, strings.Repeat(" ", pasModel), "\t")
		} else {
			extra = strings.ReplaceAll(extra, "\t", unitat)
		}
		out[i] = mapa[millor] + extra + l[len(s):]
	}
	return out
}

// unitatSagnat diu si el fitxer sagna amb tabuladors i, si no, amb quants
// espais per nivell (4 si no se sap).
func unitatSagnat(ls []string) (bool, string) {
	for _, l := range ls {
		if strings.HasPrefix(l, "\t") {
			return true, "\t"
		}
	}
	return false, strings.Repeat(" ", pasEspais(ls))
}

func pasEspais(ls []string) int {
	pas := 0
	prev := -1
	for _, l := range ls {
		if strings.TrimSpace(l) == "" || strings.Contains(sagnat(l), "\t") {
			continue
		}
		n := len(sagnat(l))
		if prev >= 0 {
			if d := n - prev; d > 0 && (pas == 0 || d < pas) {
				pas = d
			}
		}
		prev = n
	}
	if pas == 0 {
		return 4
	}
	return pas
}

// normalitzaUnicode porta a ASCII el que els models canvien sense voler
// (com fa Codex a seek_sequence): cometes i guions tipogràfics, espais
// rars, el·lipsi, caràcters d'amplada zero.
func normalitzaUnicode(s string) string {
	if isASCII(s) {
		return s
	}
	var b strings.Builder
	for _, r := range s {
		switch r {
		case '\u2018', '\u2019', '\u201A', '\u201B', '\u2032', '\u00B4':
			b.WriteByte('\'')
		case '\u201C', '\u201D', '\u201E', '\u201F', '\u2033', '\u00AB', '\u00BB':
			b.WriteByte('"')
		case '\u2010', '\u2011', '\u2012', '\u2013', '\u2014', '\u2015', '\u2212':
			b.WriteByte('-')
		case '\u00A0', '\u2000', '\u2001', '\u2002', '\u2003', '\u2004', '\u2005', '\u2006',
			'\u2007', '\u2008', '\u2009', '\u200A', '\u202F', '\u205F', '\u3000':
			b.WriteByte(' ')
		case '\u200B', '\u200C', '\u200D', '\u2060', '\uFEFF':
		case '\u2026':
			b.WriteString("...")
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= utf8.RuneSelf {
			return false
		}
	}
	return true
}

// ratio és 1 − distància de Levenshtein / longitud més gran, en runes.
// Les línies llargues es tallen: per dir si dues línies s'assemblen, els
// primers 200 caràcters ja ho diuen, i el cost és quadràtic.
func ratio(a, b string) float64 {
	if a == b {
		return 1
	}
	ra, rb := []rune(a), []rune(b)
	if len(ra) > 200 {
		ra = ra[:200]
	}
	if len(rb) > 200 {
		rb = rb[:200]
	}
	m := max(len(ra), len(rb))
	if m == 0 {
		return 1
	}
	return 1 - float64(levenshtein(ra, rb))/float64(m)
}

func levenshtein(a, b []rune) int {
	prev := make([]int, len(b)+1)
	cur := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		cur[0] = i
		for j := 1; j <= len(b); j++ {
			c := prev[j-1]
			if a[i-1] != b[j-1] {
				c++
			}
			cur[j] = min(c, prev[j]+1, cur[j-1]+1)
		}
		prev, cur = cur, prev
	}
	return prev[len(b)]
}

// mesSemblant busca la finestra del fitxer que més s'assembla a old, línia a
// línia, com fa Roo quan una cerca falla: ensenyar-la amb números de línia
// estalvia al model una lectura sencera, i el percentatge li diu si és el
// bloc que buscava mal copiat o una altra cosa.
func mesSemblant(text, old string) (ini, fi int, sim float64) {
	lines := strings.Split(text, "\n")
	o := preparaBloc(old, "").old
	if len(o) == 0 || len(lines) == 0 {
		return 0, 0, 0
	}
	n := min(len(o), len(lines))
	to := make([]string, len(o))
	for i, l := range o {
		to[i] = normalitzaUnicode(strings.TrimSpace(l))
	}
	tl := make([]string, len(lines))
	for i, l := range lines {
		tl[i] = normalitzaUnicode(strings.TrimSpace(l))
	}
	// Fitxers grans: la semblança fina només sobre les finestres que tenen
	// alguna línia idèntica; la resta no pot guanyar de cap manera raonable.
	car := len(lines)*n > 300000
	millor := -1.0
	for i := 0; i+n <= len(lines); i++ {
		var s float64
		if car {
			hi := false
			for k := 0; k < n; k++ {
				if tl[i+k] == to[k] && to[k] != "" {
					hi = true
					break
				}
			}
			if !hi {
				continue
			}
		}
		for k := 0; k < n; k++ {
			s += ratio(to[k], tl[i+k])
		}
		s /= float64(len(o))
		if s > millor {
			millor, ini = s, i
		}
	}
	if millor < 0 {
		return 0, 0, 0
	}
	return ini, ini + n, millor
}

// fragmentSemblant és la part de l'error que ensenya el bloc més semblant.
// Buit si no n'hi ha cap que s'hi assembli una mica.
func fragmentSemblant(text, old string) string {
	ini, fi, sim := mesSemblant(text, old)
	if sim < 0.3 || fi <= ini {
		return ""
	}
	lines := strings.Split(text, "\n")
	var b strings.Builder
	fmt.Fprintf(&b, "\nEl fragment més semblant (%.0f%%) és a les línies %d–%d:\n", sim*100, ini+1, fi)
	const maxLinies = 30
	for i := ini; i < fi && i < ini+maxLinies; i++ {
		fmt.Fprintf(&b, "%d|%s\n", i+1, escurça(lines[i], 200))
	}
	if fi-ini > maxLinies {
		fmt.Fprintf(&b, "…(%d línies més)\n", fi-ini-maxLinies)
	}
	return strings.TrimRight(b.String(), "\n")
}
