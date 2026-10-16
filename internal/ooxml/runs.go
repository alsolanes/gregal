package ooxml

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Text dins de paràgrafs de Word (w:) i DrawingML (a:, pptx). Els
// documents reals parteixen el text en moltes «runs» (w:r) amb el
// mateix format; per substituir sense perdre format:
//  1. es normalitza: cada run té com a molt un fill de contingut;
//  2. es fusionen runs veïnes amb el mateix format;
//  3. es busca sobre el text concatenat i s'edita run a run.

type dialect struct {
	p, r, rPr, t string
}

var (
	wordD = dialect{"w:p", "w:r", "w:rPr", "w:t"}
	drawD = dialect{"a:p", "a:r", "a:rPr", "a:t"}
)

func dialectFor(p *Node) dialect {
	if strings.HasPrefix(p.Name, "a:") {
		return drawD
	}
	return wordD
}

// splitRuns fa que cada w:r tingui com a molt un fill de contingut
// (w:t, w:tab, w:br, w:drawing…) a més del rPr.
func splitRuns(p *Node, dl dialect) {
	for _, r := range runsOf(p, dl) {
		var rpr *Node
		var content []*Node
		for _, c := range r.Children {
			if c.Kind != ElemNode {
				continue
			}
			if c.Name == dl.rPr {
				rpr = c
			} else {
				content = append(content, c)
			}
		}
		if len(content) <= 1 {
			continue
		}
		var nrs []*Node
		for _, c := range content {
			nr := El(r.Name)
			nr.Attrs = append([]Attr(nil), r.Attrs...)
			if rpr != nil {
				nr.Add(rpr.Clone())
			}
			nr.Add(c.Clone())
			nrs = append(nrs, nr)
		}
		r.Replace(nrs...)
	}
}

// runsOf torna les runs de text visibles d'un paràgraf (també dins
// d'hipervincles, smartTags, w:ins o camps simples; no dins de w:del).
func runsOf(p *Node, dl dialect) []*Node {
	var out []*Node
	var walk func(n *Node)
	walk = func(n *Node) {
		for _, c := range n.Children {
			if c.Kind != ElemNode {
				continue
			}
			switch c.Name {
			case dl.r:
				out = append(out, c)
			case "w:del", "w:moveFrom", dl.p, "w:txbxContent", "mc:AlternateContent", "w:pPr", "a:pPr":
				// no: text esborrat, paràgrafs niats o propietats
			default:
				walk(c)
			}
		}
	}
	walk(p)
	return out
}

func runText(r *Node, dl dialect) (*Node, bool) {
	t := r.Child(dl.t)
	return t, t != nil
}

// onlyText diu si la run només té rPr + un text.
func onlyText(r *Node, dl dialect) bool {
	n := 0
	for _, c := range r.Children {
		if c.Kind != ElemNode || c.Name == dl.rPr {
			continue
		}
		if c.Name != dl.t {
			return false
		}
		n++
	}
	return n == 1
}

// rprKey és el format d'una run, sense atributs de revisió (rsid).
func rprKey(r *Node, dl dialect) string {
	rpr := r.Child(dl.rPr)
	if rpr == nil {
		return ""
	}
	c := rpr.Clone()
	c.Walk(func(x *Node) bool {
		for _, a := range append([]Attr(nil), x.Attrs...) {
			if strings.HasPrefix(a.Name, "w:rsid") || a.Name == "dirty" || a.Name == "err" || a.Name == "lang" {
				x.Del(a.Name)
			}
		}
		return true
	})
	// w:noProof / w:lang / spell marks no canvien l'aspecte.
	for _, k := range c.Elems() {
		if k.Name == "w:noProof" || k.Name == "w:lang" {
			k.Remove()
		}
	}
	return c.String()
}

// MergeRuns fusiona runs veïnes de text amb el mateix format i esborra
// marques de correcció ortogràfica (w:proofErr) que les separen.
func MergeRuns(p *Node) {
	dl := dialectFor(p)
	for _, pe := range p.Find("w:proofErr") {
		pe.Remove()
	}
	splitRuns(p, dl)
	var prev *Node
	for _, c := range append([]*Node(nil), p.Children...) {
		if c.Kind != ElemNode {
			continue
		}
		if c.Name != dl.r || !onlyText(c, dl) {
			prev = nil
			continue
		}
		if prev != nil && prev.Parent == c.Parent && rprKey(prev, dl) == rprKey(c, dl) {
			pt, _ := runText(prev, dl)
			ct, _ := runText(c, dl)
			pt.SetText(pt.InnerText() + ct.InnerText())
			preserve(pt)
			c.Remove()
			continue
		}
		prev = c
	}
}

func preserve(t *Node) {
	if t.Name == "w:t" || t.Name == "w:delText" || t.Name == "w:instrText" {
		s := t.InnerText()
		if s != strings.TrimSpace(s) || strings.Contains(s, "  ") {
			t.Set("xml:space", "preserve")
		}
	}
}

// ParaText torna el text visible d'un paràgraf (tabulacions i salts com
// a \t i \n).
func ParaText(p *Node) string {
	dl := dialectFor(p)
	var b strings.Builder
	for _, r := range runsOf(p, dl) {
		for _, c := range r.Children {
			switch c.Name {
			case dl.t:
				b.WriteString(c.InnerText())
			case "w:tab", "a:tab":
				b.WriteString("\t")
			case "w:br", "w:cr", "a:br":
				b.WriteString("\n")
			}
		}
	}
	return b.String()
}

// ---------- cerca ----------

// Match és una coincidència dins d'un paràgraf.
type Match struct{ Start, End int }

// Finder busca text (literal o regex, opcionalment sense majúscules).
type Finder struct {
	re *regexp.Regexp
}

// NewFinder prepara la cerca.
func NewFinder(find string, isRegex, ignoreCase bool) (*Finder, error) {
	if find == "" {
		return nil, fmt.Errorf("find buit")
	}
	pat := find
	if !isRegex {
		pat = regexp.QuoteMeta(find)
	}
	if ignoreCase {
		pat = "(?i)" + pat
	}
	re, err := regexp.Compile(pat)
	if err != nil {
		return nil, fmt.Errorf("regex invàlida: %v", err)
	}
	return &Finder{re: re}, nil
}

func (f *Finder) all(s string) []Match {
	var out []Match
	for _, ix := range f.re.FindAllStringIndex(s, -1) {
		if ix[1] > ix[0] {
			out = append(out, Match{ix[0], ix[1]})
		}
	}
	return out
}

func (f *Finder) expand(s string, m Match, repl string) string {
	sub := f.re.FindStringSubmatchIndex(s[m.Start:])
	if sub == nil {
		return repl
	}
	return string(f.re.ExpandString(nil, repl, s[m.Start:], sub))
}

// seg és un tros de text d'una run amb la seva posició al paràgraf.
type seg struct {
	run        *Node
	t          *Node
	start, end int
}

func segments(p *Node, dl dialect) []seg {
	var out []seg
	pos := 0
	for _, r := range runsOf(p, dl) {
		t, ok := runText(r, dl)
		if !ok {
			// tab/br compten com un caràcter per no fusionar paraules
			for _, c := range r.Children {
				switch c.Name {
				case "w:tab", "a:tab", "w:br", "w:cr", "a:br":
					pos++
				}
			}
			continue
		}
		s := t.InnerText()
		out = append(out, seg{r, t, pos, pos + len(s)})
		pos += len(s)
	}
	return out
}

// splitRunAt parteix una run de text a l'offset (bytes del seu text) i
// torna les dues meitats (la segona és nova, inserida just després).
func splitRunAt(r *Node, dl dialect, off int) (*Node, *Node) {
	t, _ := runText(r, dl)
	s := t.InnerText()
	if off <= 0 || off >= len(s) {
		return r, nil
	}
	r2 := r.Clone()
	t2, _ := runText(r2, dl)
	t.SetText(s[:off])
	preserve(t)
	t2.SetText(s[off:])
	preserve(t2)
	r.InsertAfter(r2)
	return r, r2
}

// isolate parteix les runs perquè [start,end) del paràgraf caigui en
// runs senceres, i les torna en ordre.
func isolate(p *Node, dl dialect, start, end int) []*Node {
	// Primer el final (no mou offsets anteriors), després l'inici.
	for _, sg := range segments(p, dl) {
		if end > sg.start && end < sg.end {
			splitRunAt(sg.run, dl, end-sg.start)
			break
		}
	}
	for _, sg := range segments(p, dl) {
		if start > sg.start && start < sg.end {
			splitRunAt(sg.run, dl, start-sg.start)
			break
		}
	}
	var out []*Node
	for _, sg := range segments(p, dl) {
		if sg.start >= start && sg.end <= end && sg.end > sg.start {
			out = append(out, sg.run)
		}
	}
	return out
}

// ---------- substitució ----------

// Revision són les dades d'una revisió (control de canvis).
type Revision struct {
	Author string
	Date   string
	nextID *int
}

// NewRevision prepara ids únics per a canvis registrats al document.
func NewRevision(doc *Node, author string) *Revision {
	if author == "" {
		author = "Gregal"
	}
	max := 0
	doc.Walk(func(x *Node) bool {
		if x.Kind == ElemNode {
			if v, err := strconv.Atoi(x.Get("w:id")); err == nil && v > max {
				max = v
			}
		}
		return true
	})
	n := max + 1
	return &Revision{Author: author, Date: time.Now().UTC().Format("2006-01-02T15:04:05Z"), nextID: &n}
}

func (rv *Revision) id() string {
	v := *rv.nextID
	*rv.nextID = v + 1
	return strconv.Itoa(v)
}

func (rv *Revision) mark(name string) *Node {
	return El(name, "w:id", rv.id(), "w:author", rv.Author, "w:date", rv.Date)
}

// toDeleted converteix una run en text esborrat (w:t → w:delText).
func toDeleted(r *Node) {
	r.Walk(func(x *Node) bool {
		if x.Name == "w:t" {
			x.Name = "w:delText"
			x.Set("xml:space", "preserve")
		}
		if x.Name == "w:instrText" {
			x.Name = "w:delInstrText"
		}
		return true
	})
}

// ReplaceInPara substitueix totes les coincidències d'un paràgraf.
// Sense revisió: el text nou hereta el format de la primera run de la
// coincidència i la resta de runs queden intactes. Amb revisió (només
// Word): el text vell va a w:del i el nou a w:ins.
func ReplaceInPara(p *Node, f *Finder, repl string, rv *Revision) int {
	dl := dialectFor(p)
	MergeRuns(p)
	// Text amb les mateixes posicions que segments() (0 per tab/salt).
	var b strings.Builder
	pos := 0
	for _, sg := range segments(p, dl) {
		for pos < sg.start {
			b.WriteByte(0)
			pos++
		}
		b.WriteString(sg.t.InnerText())
		pos = sg.end
	}
	text := b.String()
	ms := f.all(text)
	// De darrere cap endavant: els offsets anteriors no canvien.
	for i := len(ms) - 1; i >= 0; i-- {
		m := ms[i]
		if strings.ContainsRune(text[m.Start:m.End], 0) {
			continue // creuaria un tab/salt: no es toca
		}
		newText := f.expand(text, m, repl)
		if rv != nil && dl == wordD {
			// Com un revisor humà: només es marca el que canvia («30»→«45»),
			// no la frase sencera que el model ha passat com a find.
			old := text[m.Start:m.End]
			cp, cs := commonEdges(old, newText)
			if cp > 0 || cs > 0 {
				m.Start += cp
				m.End -= cs
				newText = newText[cp : len(newText)-cs]
			}
			if m.Start == m.End {
				if newText != "" {
					insertTracked(p, dl, m.Start, newText, rv)
				}
				continue
			}
		}
		runs := isolate(p, dl, m.Start, m.End)
		if len(runs) == 0 {
			continue
		}
		if rv == nil || dl != wordD {
			t, _ := runText(runs[0], dl)
			t.SetText(newText)
			preserve(t)
			for _, r := range runs[1:] {
				r.Remove()
			}
			if newText == "" {
				runs[0].Remove()
			}
			continue
		}
		// Control de canvis.
		var ins *Node
		if newText != "" {
			nr := runs[0].Clone()
			t, _ := runText(nr, dl)
			t.SetText(newText)
			preserve(t)
			ins = rv.mark("w:ins").Add(nr)
		}
		del := rv.mark("w:del")
		runs[0].InsertBefore(del)
		for _, r := range runs {
			r.Remove()
			toDeleted(r)
			del.Append(r)
		}
		if ins != nil {
			del.InsertAfter(ins)
		}
	}
	return len(ms)
}

// ---------- paràgrafs del document ----------

// Paragraphs torna tots els paràgrafs d'un part (cos, taules, quadres
// de text), en ordre de document.
func Paragraphs(root *Node) []*Node {
	var out []*Node
	root.Walk(func(x *Node) bool {
		if x.Name == "w:p" || x.Name == "a:p" {
			out = append(out, x)
		}
		return true
	})
	return out
}

// ParaTextAll torna el text de tots els paràgrafs (un per línia).
func ParaTextAll(root *Node) string {
	var b strings.Builder
	for _, p := range Paragraphs(root) {
		b.WriteString(ParaText(p))
		b.WriteByte('\n')
	}
	return b.String()
}

// FindParagraph torna el primer paràgraf el text del qual conté
// anchor (o n'és igual si exact). Les coincidències múltiples es
// tornen totes perquè qui crida pugui avisar.
func FindParagraph(root *Node, anchor string) []*Node {
	var out []*Node
	a := strings.TrimSpace(anchor)
	for _, p := range Paragraphs(root) {
		if strings.Contains(ParaText(p), a) {
			out = append(out, p)
		}
	}
	return out
}

// commonEdges torna el prefix i el sufix comuns (en bytes, tallats a
// límit de paraula) entre el text vell i el nou.
func commonEdges(a, b string) (int, int) {
	p := 0
	for p < len(a) && p < len(b) && a[p] == b[p] {
		p++
	}
	// enrere fins a un espai (no partir paraules) o fins al principi
	for p > 0 && p < len(a) && a[p-1] != ' ' {
		p--
	}
	s := 0
	for s < len(a)-p && s < len(b)-p && a[len(a)-1-s] == b[len(b)-1-s] {
		s++
	}
	for s > 0 && s < len(a)-p && a[len(a)-s] != ' ' && a[len(a)-s] != '.' && a[len(a)-s] != ',' {
		s--
	}
	return p, s
}

// insertTracked insereix text registrat (w:ins) a la posició pos del
// paràgraf, amb el format de la run on cau.
func insertTracked(p *Node, dl dialect, pos int, text string, rv *Revision) {
	segs := segments(p, dl)
	var host *Node
	for _, sg := range segs {
		if pos > sg.start && pos < sg.end {
			splitRunAt(sg.run, dl, pos-sg.start)
			break
		}
	}
	segs = segments(p, dl)
	before := true
	for _, sg := range segs {
		if sg.start == pos {
			host, before = sg.run, true
			break
		}
		if sg.end == pos {
			host, before = sg.run, false
		}
	}
	if host == nil {
		return
	}
	nr := host.Clone()
	t, _ := runText(nr, dl)
	t.SetText(text)
	preserve(t)
	ins := rv.mark("w:ins").Add(nr)
	if before {
		host.InsertBefore(ins)
	} else {
		host.InsertAfter(ins)
	}
}
