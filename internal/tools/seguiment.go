package tools

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Seguiment recorda, per sessió, què ha vist el model de cada fitxer. En
// surten dues coses:
//
//   - Llegir abans d'editar (P9). Una edició aproximada (qualsevol nivell
//     de la cascada que no sigui l'exacte) només s'aplica sobre un fitxer
//     que el model té al dia: llegit, escrit o editat per ell, i que no ha
//     canviat des de llavors. Si l'ha tocat un altre —l'usuari, un altre
//     agent d'un team, un `sed` del mateix model— la coincidència aproximada
//     podria caure sobre un text que el model no ha vist mai. Un old que hi
//     és exacte i únic sí que s'aplica sempre (la política de Claude Code):
//     el model ha demostrat que sap què hi ha.
//   - No rellegir (P10). Un read d'un rang que no ha canviat i que s'ha
//     llegit fa poques crides torna «sense canvis» en comptes de milers de
//     tokens repetits.
type Seguiment struct {
	mu      sync.Mutex
	pas     int
	fitxers map[string]*estatFitxer
	vist    time.Time
}

type estatFitxer struct {
	hash     [32]byte
	lectures []lecturaRang
}

// lecturaRang és un tros llegit sencer (línies ini..fi, 1-based) al pas pas.
// avisat vol dir que l'última petició d'aquest rang ja s'ha respost amb
// «sense canvis»: si el model hi torna, és que no el té, i se li dona.
type lecturaRang struct {
	ini, fi, pas int
	avisat       bool
}

// finestraRelectura és quantes crides d'eina enrere es considera que una
// lectura encara és sencera a l'historial. La poda de sortides velles
// (agent.KeepRecentToolOutputs, 6) en deixa sempre senceres les últimes sis;
// més enllà, la lectura pot haver quedat retallada i dir «sense canvis»
// deixaria el model sense el text.
const finestraRelectura = 5

// minRelectura: per sota d'aquesta mida no val la pena estalviar-se res.
const minRelectura = 400

var (
	seguimentsMu sync.Mutex
	seguiments   = map[string]*Seguiment{}
)

// SeguimentDe retorna el seguiment de la sessió (el crea si cal). Les
// sessions que fa un dia que no es mouen s'oblidan aquí mateix.
func SeguimentDe(sessio string) *Seguiment {
	seguimentsMu.Lock()
	defer seguimentsMu.Unlock()
	ara := time.Now()
	s := seguiments[sessio]
	if s == nil {
		for k, v := range seguiments {
			v.mu.Lock()
			vell := ara.Sub(v.vist) > 24*time.Hour
			v.mu.Unlock()
			if vell {
				delete(seguiments, k)
			}
		}
		s = &Seguiment{fitxers: map[string]*estatFitxer{}}
		seguiments[sessio] = s
	}
	s.mu.Lock()
	s.vist = ara
	s.mu.Unlock()
	return s
}

// OblidaSeguiment esborra el que se sap d'una sessió. Cal cridar-lo quan
// l'historial es buida o es resumeix: el que el model «havia llegit» ja no
// és al context.
func OblidaSeguiment(sessio string) {
	seguimentsMu.Lock()
	delete(seguiments, sessio)
	seguimentsMu.Unlock()
}

// NouPas compta una crida d'eina. El loop el crida una vegada per crida.
func (s *Seguiment) NouPas() {
	s.mu.Lock()
	s.pas++
	s.mu.Unlock()
}

func clau(path string) string {
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}
	return strings.ToLower(filepath.Clean(path))
}

// conegut desa que el model coneix aquest contingut del fitxer. Sense
// lectures: un fitxer que acaba de canviar no es pot donar per llegit.
func (s *Seguiment) conegut(path string, raw []byte) {
	if len(s.fitxers) > 500 {
		s.fitxers = map[string]*estatFitxer{}
	}
	s.fitxers[clau(path)] = &estatFitxer{hash: sha256.Sum256(raw)}
}

// Coneix torna a mirar el fitxer després d'escriure'l el model (write) o
// d'haver-hi passat el hook post-edició (un formatador): el que hi ha ara és
// el que el model ha volgut, no un canvi d'un altre.
func (s *Seguiment) Coneix(path string) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.conegut(path, raw)
}

// alDia diu si el model té al dia el contingut raw, i si no, per què.
func (s *Seguiment) alDia(path string, raw []byte) (bool, string) {
	st := s.fitxers[clau(path)]
	if st == nil {
		return false, "no l'has llegit en aquesta conversa"
	}
	if st.hash != sha256.Sum256(raw) {
		return false, "ha canviat des que el vas llegir"
	}
	return true, ""
}

// Read és tools.Read amb memòria: si el rang demanat ja s'ha llegit fa poc i
// el fitxer no ha canviat, ho diu en comptes de repetir-lo.
func (s *Seguiment) Read(path string, offset, limit int) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	l, err := llegeix(path, raw, offset, limit)
	if err != nil {
		return "", err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	h := sha256.Sum256(raw)
	k := clau(path)
	st := s.fitxers[k]
	if st == nil || st.hash != h {
		s.conegut(path, raw)
		st = s.fitxers[k]
	}
	if len(l.text) >= minRelectura {
		// De la més nova a la més vella: la que mana és l'última.
		for i := len(st.lectures) - 1; i >= 0; i-- {
			lr := &st.lectures[i]
			if lr.ini > l.ini || lr.fi < l.fi || s.pas-lr.pas > finestraRelectura {
				continue
			}
			if lr.avisat {
				// Hi ha tornat després del «sense canvis»: no el té.
				break
			}
			lr.avisat = true
			fa := s.pas - lr.pas
			quan := fmt.Sprintf("fa %d crides d'eina", fa)
			if fa <= 0 {
				quan = "en aquest mateix pas"
			}
			return fmt.Sprintf("sense canvis: les línies %d–%d de %s ja les has llegit %s i el fitxer no ha canviat; fes servir aquella lectura. Si ja no la tens al context, torna a cridar read igual i te la tornaré sencera.", l.ini, l.fi, path, quan), nil
		}
	}
	st.lectures = append(st.lectures, lecturaRang{ini: l.ini, fi: l.fi, pas: s.pas})
	if len(st.lectures) > 20 {
		st.lectures = st.lectures[len(st.lectures)-20:]
	}
	return l.text, nil
}

// Edit és tools.Edit amb la guarda de lectura i el resum del canvi.
func (s *Seguiment) Edit(path, old, new string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return edita(path, old, new, false, s)
}

// EditAll és Edit amb replace_all: substitueix totes les aparicions
// exactes d'old (reanomenar un identificador, un literal repetit).
func (s *Seguiment) EditAll(path, old, new string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return edita(path, old, new, true, s)
}

// Patch és tools.Patch amb la guarda de lectura i el resum del canvi.
func (s *Seguiment) Patch(path string, ops []PatchOp) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return pedaça(path, ops, s)
}

// errNoAlDia és el rebuig de P9.
func errNoAlDia(path, perque, nivell string) error {
	return fmt.Errorf("%s %s i old no hi és tal qual (només casaria amb coincidència aproximada, nivell «%s»): una edició aproximada sobre un fitxer que no tens al dia podria tocar un text que no has vist. Llegeix-lo amb read i torna-ho a provar", path, perque, nivell)
}

// ---- Diff del canvi ----

// resumCanvi és el que torna una edició: el diff amb context i números de
// línia del fitxer nou. Amb això el model no ha de rellegir el fitxer per
// saber com ha quedat (P10).
func resumCanvi(abans, despres string) string {
	a := strings.Split(senseCR(abans), "\n")
	b := strings.Split(senseCR(despres), "\n")
	ops := difLinies(a, b)
	const ctx, maxLinies = 3, 80
	// Marca les operacions a ensenyar: els canvis i ctx línies al voltant.
	mostra := make([]bool, len(ops))
	for i, o := range ops {
		if o.tipus != ' ' {
			for j := max(0, i-ctx); j <= min(len(ops)-1, i+ctx); j++ {
				mostra[j] = true
			}
		}
	}
	var out []string
	for i, o := range ops {
		if !mostra[i] {
			if i > 0 && mostra[i-1] {
				out = append(out, "…")
			}
			continue
		}
		switch o.tipus {
		case '-':
			out = append(out, fmt.Sprintf("-%d|%s", o.na, escurça(o.text, 300)))
		case '+':
			out = append(out, fmt.Sprintf("+%d|%s", o.nb, escurça(o.text, 300)))
		default:
			out = append(out, fmt.Sprintf(" %d|%s", o.nb, escurça(o.text, 300)))
		}
	}
	if len(out) == 0 {
		return ""
	}
	if len(out) > maxLinies {
		resta := len(out) - maxLinies
		out = append(out[:maxLinies], fmt.Sprintf("…(%d línies més del diff)", resta))
	}
	return "\n```diff\n" + strings.Join(out, "\n") + "\n```"
}

type opLinia struct {
	tipus  byte // ' ', '-', '+'
	text   string
	na, nb int // número de línia (1-based) a abans i a després
}

// difLinies fa un diff de línies: prefix i sufix comuns i LCS al mig, que en
// una edició és petit. Si el mig fos enorme, tot esborrat i tot afegit.
func difLinies(a, b []string) []opLinia {
	p := 0
	for p < len(a) && p < len(b) && a[p] == b[p] {
		p++
	}
	sa, sb := len(a), len(b)
	for sa > p && sb > p && a[sa-1] == b[sb-1] {
		sa--
		sb--
	}
	var ops []opLinia
	for i := 0; i < p; i++ {
		ops = append(ops, opLinia{' ', a[i], i + 1, i + 1})
	}
	ma, mb := a[p:sa], b[p:sb]
	if len(ma)*len(mb) > 1_000_000 {
		for i, l := range ma {
			ops = append(ops, opLinia{'-', l, p + i + 1, 0})
		}
		for j, l := range mb {
			ops = append(ops, opLinia{'+', l, 0, p + j + 1})
		}
	} else {
		lcs := make([][]int, len(ma)+1)
		for i := range lcs {
			lcs[i] = make([]int, len(mb)+1)
		}
		for i := len(ma) - 1; i >= 0; i-- {
			for j := len(mb) - 1; j >= 0; j-- {
				if ma[i] == mb[j] {
					lcs[i][j] = lcs[i+1][j+1] + 1
				} else {
					lcs[i][j] = max(lcs[i+1][j], lcs[i][j+1])
				}
			}
		}
		i, j := 0, 0
		for i < len(ma) || j < len(mb) {
			switch {
			case i < len(ma) && j < len(mb) && ma[i] == mb[j]:
				ops = append(ops, opLinia{' ', ma[i], p + i + 1, p + j + 1})
				i++
				j++
			case j < len(mb) && (i == len(ma) || lcs[i][j+1] >= lcs[i+1][j]):
				ops = append(ops, opLinia{'+', mb[j], 0, p + j + 1})
				j++
			default:
				ops = append(ops, opLinia{'-', ma[i], p + i + 1, 0})
				i++
			}
		}
	}
	for k := 0; k < len(a)-sa; k++ {
		ops = append(ops, opLinia{' ', a[sa+k], sa + k + 1, sb + k + 1})
	}
	return ops
}

// ---- Sortides llargues: cap i cua ----

// partCap és la part de la sortida que es queda del principi; la resta, del
// final. Els errors de compilació i de test solen ser al final, i el cap diu
// què s'ha executat.
const partCap = 0.6

// retallaCapICua deixa s dins de n caràcters guardant el cap i la cua, tallant
// per línies. Si s'ha de retallar i desa no és nil, la sortida sencera es
// desa i la marca del mig diu on és i com llegir-ne un tros.
func retallaCapICua(s string, n int, desa func(string) string) string {
	if len(s) <= n {
		return s
	}
	cap := tallaLinia(s[:int(float64(n)*partCap)], false)
	cua := tallaLinia(s[len(s)-(n-len(cap)):], true)
	omesos := s[len(cap) : len(s)-len(cua)]
	marca := fmt.Sprintf("\n…[%d línies (%d caràcters) omeses del mig", strings.Count(omesos, "\n"), len(omesos))
	if desa != nil {
		if p := desa(s); p != "" {
			marca += fmt.Sprintf("; la sortida sencera és a %s — llegeix-ne el rang que et calgui amb read (offset/limit) o grep", p)
		}
	}
	return cap + marca + "]…\n" + cua
}

// tallaLinia retalla per una vora de línia: el cap fins a l'últim salt, la
// cua des del primer. Si no n'hi ha cap a prop, talla per una vora de runa.
func tallaLinia(s string, cua bool) string {
	if cua {
		if i := strings.IndexByte(s, '\n'); i >= 0 && i < len(s)/2 {
			return s[i+1:]
		}
		for len(s) > 0 && s[0]&0xC0 == 0x80 {
			s = s[1:]
		}
		return s
	}
	if i := strings.LastIndexByte(s, '\n'); i >= len(s)/2 {
		return s[:i+1]
	}
	for len(s) > 0 && s[len(s)-1]&0xC0 == 0x80 {
		s = s[:len(s)-1]
	}
	if len(s) > 0 && s[len(s)-1] >= 0xC0 {
		s = s[:len(s)-1]
	}
	return s
}

// dirSortides és on es desen les sortides senceres, dins del workspace.
const dirSortides = ".gregal/out"

var numSortida atomic.Int64

// desaSortida desa la sortida sencera a <dir>/.gregal/out/<id>.txt i torna el
// camí relatiu al workspace (el que read resol), o "" si no s'ha pogut. Hi
// deixa un .gitignore perquè no surti a git status, i s'emporta les velles.
func desaSortida(dir, prefix, s string) string {
	base := filepath.Join(dir, filepath.FromSlash(dirSortides))
	if err := os.MkdirAll(base, 0o755); err != nil {
		return ""
	}
	gi := filepath.Join(base, ".gitignore")
	if _, err := os.Stat(gi); err != nil {
		_ = os.WriteFile(gi, []byte("*\n"), 0o644)
	}
	netejaSortides(base)
	nom := fmt.Sprintf("%s-%s-%d.txt", prefix, time.Now().Format("20060102-150405"), numSortida.Add(1))
	if err := os.WriteFile(filepath.Join(base, nom), []byte(s), 0o644); err != nil {
		return ""
	}
	return dirSortides + "/" + nom
}

// netejaSortides esborra les de més d'un dia i deixa com a molt les 100
// més noves.
func netejaSortides(base string) {
	ents, err := os.ReadDir(base)
	if err != nil {
		return
	}
	type f struct {
		nom string
		t   time.Time
	}
	var fs []f
	for _, e := range ents {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".txt") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if time.Since(info.ModTime()) > 24*time.Hour {
			_ = os.Remove(filepath.Join(base, e.Name()))
			continue
		}
		fs = append(fs, f{e.Name(), info.ModTime()})
	}
	sort.Slice(fs, func(i, j int) bool { return fs[i].t.After(fs[j].t) })
	for _, x := range fs[min(len(fs), 100):] {
		_ = os.Remove(filepath.Join(base, x.nom))
	}
}

// OblidaTotsSeguiments esborra el seguiment de totes les sessions. El crida
// la compactació quan resumeix o retalla l'historial: les lectures que el
// model «tenia» ja no hi són, i un «sense canvis» el deixaria sense el text.
// Afecta totes les sessions perquè la compactació no sap de quina és; el
// pitjor cas és una relectura de més.
func OblidaTotsSeguiments() {
	seguimentsMu.Lock()
	seguiments = map[string]*Seguiment{}
	seguimentsMu.Unlock()
}
