package ooxml

import (
	"bytes"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Pptx edita o construeix una presentació (pròpia o d'una plantilla).
type Pptx struct {
	Pkg     *Package
	Main    string
	Pres    *Node
	Theme   Theme
	BaseDir string
}

// OpenPptx obre una presentació existent.
func OpenPptx(pk *Package) (*Pptx, error) {
	main := pk.MainPart()
	if !strings.HasPrefix(main, "ppt/") {
		return nil, fmt.Errorf("no és una presentació de PowerPoint")
	}
	doc, err := pk.XML(main)
	if err != nil {
		return nil, err
	}
	return &Pptx{Pkg: pk, Main: main, Pres: doc.Root(), Theme: GetTheme("")}, nil
}

// NewPptx crea una presentació buida amb el tema propi.
func NewPptx(t Theme, title string) *Pptx {
	if t.Name == "" {
		t = GetTheme("")
	}
	p, err := OpenPptx(NewPptxPackage(t, title))
	if err != nil {
		panic(err)
	}
	p.Theme = t
	return p
}

// PptxFromTemplate parteix d'una plantilla (.potx/.pptx).
func PptxFromTemplate(tpl string) (*Pptx, error) {
	pk, err := Open(tpl)
	if err != nil {
		return nil, err
	}
	pk.ToDocument()
	p, err := OpenPptx(pk)
	if err != nil {
		return nil, err
	}
	p.Theme = p.LoadTheme()
	return p, nil
}

// LoadTheme llegeix colors i tipus de lletra del tema de la
// plantilla (per a taules i gràfics que dibuixem nosaltres).
func (p *Pptx) LoadTheme() Theme {
	t := GetTheme("")
	masters := p.Pkg.ByType(p.Main, RelSlideMaster)
	if len(masters) == 0 {
		return t
	}
	th := p.Pkg.ByType(masters[0], RelTheme)
	if len(th) == 0 {
		return t
	}
	doc, err := p.Pkg.XML(th[0])
	if err != nil {
		return t
	}
	cs := doc.First("a:clrScheme")
	get := func(name string) string {
		if n := cs.Child(name); n != nil {
			if c := n.Child("a:srgbClr"); c != nil {
				return c.Get("val")
			}
			if c := n.Child("a:sysClr"); c != nil {
				return c.Get("lastClr")
			}
		}
		return ""
	}
	if cs != nil {
		set := func(dst *string, v string) {
			if v != "" {
				*dst = v
			}
		}
		set(&t.Text, get("a:dk1"))
		set(&t.Dark, get("a:dk2"))
		set(&t.Light, get("a:lt2"))
		set(&t.Primary, get("a:accent1"))
		set(&t.Accent, get("a:accent2"))
		set(&t.Accent2, get("a:accent3"))
		set(&t.Muted, get("a:accent4"))
	}
	if f := doc.First("a:majorFont"); f != nil {
		if l := f.Child("a:latin"); l != nil && l.Get("typeface") != "" {
			t.HeadingFont = l.Get("typeface")
		}
	}
	if f := doc.First("a:minorFont"); f != nil {
		if l := f.Child("a:latin"); l != nil && l.Get("typeface") != "" {
			t.BodyFont = l.Get("typeface")
		}
	}
	t.Name = "template"
	return t
}

// ---------- layouts i placeholders ----------

// Box és un rectangle en EMU.
type Box struct{ X, Y, CX, CY int }

// In torna el rectangle en polzades (per mostrar).
func (b Box) In() string {
	f := func(v int) string { return strconv.FormatFloat(float64(v)/emuIn, 'f', 2, 64) }
	return fmt.Sprintf("x=%s y=%s w=%s h=%s", f(b.X), f(b.Y), f(b.CX), f(b.CY))
}

// PhInfo és un marcador de posició d'un layout.
type PhInfo struct {
	Type string // title, ctrTitle, subTitle, body, obj, pic, …
	Idx  string
	Box  Box
	Name string
}

// LayoutInfo descriu un layout.
type LayoutInfo struct {
	Part   string
	Name   string
	Type   string
	Phs    []PhInfo
	Master string
}

func phOf(sp *Node) (typ, idx string, ok bool) {
	var nv *Node
	for _, k := range []string{"p:nvSpPr", "p:nvPicPr", "p:nvGraphicFramePr", "p:nvGrpSpPr", "p:nvCxnSpPr"} {
		if c := sp.Child(k); c != nil {
			nv = c
			break
		}
	}
	if nv == nil {
		return "", "", false
	}
	ph := nv.Path("p:nvPr", "p:ph")
	if ph == nil {
		return "", "", false
	}
	typ = ph.Get("type")
	if typ == "" {
		typ = "obj"
	}
	return typ, ph.Get("idx"), true
}

func xfrmOf(sp *Node) (Box, bool) {
	var x *Node
	if pr := sp.Child("p:spPr"); pr != nil {
		x = pr.Child("a:xfrm")
	} else if gx := sp.Child("p:xfrm"); gx != nil {
		x = gx
	}
	if x == nil {
		return Box{}, false
	}
	off, ext := x.Child("a:off"), x.Child("a:ext")
	if off == nil || ext == nil {
		return Box{}, false
	}
	a, _ := strconv.Atoi(off.Get("x"))
	b, _ := strconv.Atoi(off.Get("y"))
	c, _ := strconv.Atoi(ext.Get("cx"))
	d, _ := strconv.Atoi(ext.Get("cy"))
	return Box{a, b, c, d}, c > 0 && d > 0
}

func phMatch(t1, i1, t2, i2 string) bool {
	if i1 != "" && i1 != "0" && i1 == i2 {
		return true
	}
	norm := func(t string) string {
		switch t {
		case "ctrTitle":
			return "title"
		case "subTitle", "obj":
			return "body"
		}
		return t
	}
	return (i1 == "" || i1 == "0") && (i2 == "" || i2 == "0") && norm(t1) == norm(t2)
}

func (p *Pptx) masterPhBox(master, typ string) (Box, bool) {
	doc, err := p.Pkg.XML(master)
	if err != nil {
		return Box{}, false
	}
	want := "body"
	if typ == "title" || typ == "ctrTitle" {
		want = "title"
	}
	for _, sp := range doc.Root().Path("p:cSld", "p:spTree").Elems() {
		t, _, ok := phOf(sp)
		if ok && t == want {
			return xfrmOf(sp)
		}
	}
	return Box{}, false
}

// Layouts torna els layouts disponibles (del primer màster i següents).
func (p *Pptx) Layouts() []LayoutInfo {
	var out []LayoutInfo
	for _, m := range p.Pkg.ByType(p.Main, RelSlideMaster) {
		for _, lp := range p.Pkg.ByType(m, RelSlideLayout) {
			doc, err := p.Pkg.XML(lp)
			if err != nil {
				continue
			}
			root := doc.Root()
			li := LayoutInfo{Part: lp, Name: root.Path("p:cSld").Get("name"), Type: root.Get("type"), Master: m}
			for _, sp := range root.Path("p:cSld", "p:spTree").Elems() {
				t, idx, ok := phOf(sp)
				if !ok || t == "dt" || t == "ftr" || t == "sldNum" {
					continue
				}
				b, ok := xfrmOf(sp)
				if !ok {
					b, _ = p.masterPhBox(m, t)
				}
				name := ""
				if c := sp.Path("p:nvSpPr", "p:cNvPr"); c != nil {
					name = c.Get("name")
				}
				li.Phs = append(li.Phs, PhInfo{Type: t, Idx: idx, Box: b, Name: name})
			}
			out = append(out, li)
		}
	}
	return out
}

// countPh compta marcadors per tipus normalitzat.
func (l LayoutInfo) count(kind string) int {
	n := 0
	for _, ph := range l.Phs {
		switch kind {
		case "title":
			if ph.Type == "title" || ph.Type == "ctrTitle" {
				n++
			}
		case "body":
			if ph.Type == "body" || ph.Type == "obj" || ph.Type == "subTitle" {
				n++
			}
		default:
			if ph.Type == kind {
				n++
			}
		}
	}
	return n
}

// layoutKinds: tipus semàntic → (type OOXML preferit, paraules del nom).
var layoutKinds = map[string]struct {
	types []string
	words []string
}{
	"title":       {[]string{"title"}, []string{"title slide", "portada", "títol"}},
	"closing":     {[]string{"title"}, []string{"title slide", "closing", "cierre"}},
	"content":     {[]string{"obj", "tx"}, []string{"title and content", "títol i contingut", "content"}},
	"section":     {[]string{"secHead"}, []string{"section", "secció"}},
	"two_columns": {[]string{"twoObj", "twoColTx"}, []string{"two content", "two column", "dues"}},
	"comparison":  {[]string{"twoTxTwoObj"}, []string{"comparison", "comparació"}},
	"big_number":  {nil, []string{"big number", "number", "xifra"}},
	"image":       {[]string{"picTx"}, []string{"picture", "imatge", "image"}},
	"title_only":  {[]string{"titleOnly"}, []string{"title only", "només"}},
	"blank":       {[]string{"blank"}, []string{"blank", "en blanc"}},
}

// PickLayout tria el layout que millor encaixa amb un tipus semàntic.
// Admet també el nom exacte o l'índex (1-based) d'un layout.
func (p *Pptx) PickLayout(kind string) (LayoutInfo, error) {
	ls := p.Layouts()
	if len(ls) == 0 {
		return LayoutInfo{}, fmt.Errorf("la presentació no té layouts")
	}
	k := strings.ToLower(strings.TrimSpace(kind))
	if k == "" {
		k = "content"
	}
	if n, err := strconv.Atoi(k); err == nil && n >= 1 && n <= len(ls) {
		return ls[n-1], nil
	}
	for _, l := range ls {
		if strings.EqualFold(l.Name, kind) {
			return l, nil
		}
	}
	def, known := layoutKinds[k]
	if known {
		for _, l := range ls {
			for _, w := range def.words {
				if strings.Contains(strings.ToLower(l.Name), w) && (k != "content" || l.count("body") == 1) {
					return l, nil
				}
			}
		}
		for _, t := range def.types {
			for _, l := range ls {
				if l.Type == t {
					return l, nil
				}
			}
		}
	}
	// Per estructura.
	score := func(l LayoutInfo) int {
		t, b, pic := l.count("title"), l.count("body"), l.count("pic")
		switch k {
		case "two_columns":
			if t >= 1 && b == 2 {
				return 3
			}
		case "comparison":
			if t >= 1 && b == 4 {
				return 3
			}
			if t >= 1 && b == 2 {
				return 1
			}
		case "image":
			if pic >= 1 {
				return 3
			}
		case "title_only", "big_number":
			if t == 1 && b == 0 {
				return 2
			}
		case "blank":
			if t == 0 && b == 0 {
				return 3
			}
		}
		if t >= 1 && b == 1 {
			return 1
		}
		return 0
	}
	best, bs := ls[0], -1
	for _, l := range ls {
		if s := score(l); s > bs {
			best, bs = l, s
		}
	}
	return best, nil
}

// ---------- diapositives ----------

// SlideParts torna els parts de diapositiva en l'ordre de la presentació.
func (p *Pptx) SlideParts() []string {
	var out []string
	for _, s := range p.Pres.Path("p:sldIdLst").Elems("p:sldId") {
		if t, _, ok := p.Pkg.RelByID(p.Main, s.Get("r:id")); ok {
			out = append(out, t)
		}
	}
	return out
}

func (p *Pptx) sldIdLst() *Node {
	return p.Pres.EnsureChild("p:sldIdLst", []string{"p:sldMasterIdLst", "p:notesMasterIdLst", "p:handoutMasterIdLst", "p:sldIdLst", "p:sldSz", "p:notesSz", "p:smartTags", "p:embeddedFontLst", "p:custShowLst", "p:photoAlbum", "p:custDataLst", "p:kinsoku", "p:defaultTextStyle", "p:modifyVerifier", "p:extLst"})
}

func (p *Pptx) nextSldID() int {
	max := 255
	for _, s := range p.Pres.Path("p:sldIdLst").Elems("p:sldId") {
		if v, err := strconv.Atoi(s.Get("id")); err == nil && v > max {
			max = v
		}
	}
	return max + 1
}

// insertSlidePart registra un part de diapositiva a la posició pos
// (0-based; -1 = al final).
func (p *Pptx) insertSlidePart(part string, pos int) {
	p.Pkg.SetOverride(part, ctSlide)
	rid := p.Pkg.AddRel(p.Main, RelSlide, part)
	lst := p.sldIdLst()
	el := El("p:sldId", "id", strconv.Itoa(p.nextSldID()), "r:id", rid)
	ids := lst.Elems("p:sldId")
	if pos < 0 || pos >= len(ids) {
		lst.Append(el)
	} else {
		ids[pos].InsertBefore(el)
	}
}

// AddSlide crea una diapositiva amb el layout donat a la posició pos
// (-1 = al final) i torna el seu part. La diapositiva neix buida (només
// el número de diapositiva si el layout en té).
func (p *Pptx) AddSlide(l LayoutInfo, pos int) string {
	part := p.Pkg.NextPartName("ppt/slides", "slide", ".xml")
	p.Pkg.Put(part, []byte(xmlHead+`<p:sld xmlns:a="`+nsA+`" xmlns:r="`+nsR+`" xmlns:p="`+nsP+`"><p:cSld><p:spTree>`+spTreeHead+`</p:spTree></p:cSld><p:clrMapOvr><a:masterClrMapping/></p:clrMapOvr></p:sld>`))
	p.Pkg.AddRel(part, RelSlideLayout, l.Part)
	p.insertSlidePart(part, pos)
	// Número de diapositiva si el layout el té.
	if ld, err := p.Pkg.XML(l.Part); err == nil {
		for _, sp := range ld.Root().Path("p:cSld", "p:spTree").Elems() {
			if t, idx, ok := phOf(sp); ok && t == "sldNum" {
				doc, _ := p.Pkg.XML(part)
				tree := doc.Root().Path("p:cSld", "p:spTree")
				n, _ := ParseFragment(phSp(p.nextShapeID(tree), "Slide Number", "sldNum", idx, 0, 0, 0, 0,
					`<a:bodyPr/><a:lstStyle/><a:p><a:fld id="{B6F15528-21DE-4FAA-801E-634DDDAF4B2B}" type="slidenum"><a:rPr lang="ca-ES"/><a:t>‹#›</a:t></a:fld><a:endParaRPr lang="ca-ES"/></a:p>`))
				tree.Append(n...)
			}
		}
	}
	return part
}

// DuplicateSlide copia la diapositiva n (0-based) i la posa a pos.
func (p *Pptx) DuplicateSlide(n, pos int) (string, error) {
	sl := p.SlideParts()
	if n < 0 || n >= len(sl) {
		return "", fmt.Errorf("diapositiva %d no existeix (n'hi ha %d)", n+1, len(sl))
	}
	src := sl[n]
	raw, _ := p.Pkg.Raw(src)
	part := p.Pkg.NextPartName("ppt/slides", "slide", ".xml")
	p.Pkg.Put(part, append([]byte(nil), raw...))
	// Mateixes relacions, menys les notes (cada diapositiva té les seves).
	// Els r:id es conserven perquè es copien en el mateix ordre i amb el
	// mateix Id.
	if rd, err := p.Pkg.XML(RelsName(src)); err == nil {
		cl := rd.Clone()
		for _, r := range cl.Root().Elems("Relationship") {
			if r.Get("Type") == RelNotesSlide {
				r.Remove()
			}
			if r.Get("Type") == RelChart {
				// Un gràfic no es pot compartir: se'n fa una còpia.
				t := RelTarget(src, r.Get("Target"))
				craw, _ := p.Pkg.Raw(t)
				np := p.Pkg.NextPartName("ppt/charts", "chart", ".xml")
				p.Pkg.Put(np, append([]byte(nil), craw...))
				p.Pkg.SetOverride(np, ctChart)
				r.Set("Target", RelPath(part, np))
			}
		}
		p.Pkg.PutXML(RelsName(part), cl)
	}
	p.insertSlidePart(part, pos)
	return part, nil
}

// DeleteSlide esborra la diapositiva n (0-based) i les seves notes.
func (p *Pptx) DeleteSlide(n int) error {
	ids := p.Pres.Path("p:sldIdLst").Elems("p:sldId")
	if n < 0 || n >= len(ids) {
		return fmt.Errorf("diapositiva %d no existeix (n'hi ha %d)", n+1, len(ids))
	}
	rid := ids[n].Get("r:id")
	part, _, _ := p.Pkg.RelByID(p.Main, rid)
	ids[n].Remove()
	p.Pkg.RemoveRel(p.Main, rid)
	for _, nt := range p.Pkg.ByType(part, RelNotesSlide) {
		p.Pkg.Delete(nt)
		p.Pkg.Delete(RelsName(nt))
		p.Pkg.RemoveOverride(nt)
	}
	for _, ch := range p.Pkg.ByType(part, RelChart) {
		p.Pkg.Delete(ch)
		p.Pkg.Delete(RelsName(ch))
		p.Pkg.RemoveOverride(ch)
	}
	p.Pkg.Delete(part)
	p.Pkg.Delete(RelsName(part))
	p.Pkg.RemoveOverride(part)
	return nil
}

// MoveSlide mou la diapositiva from a la posició to (0-based).
func (p *Pptx) MoveSlide(from, to int) error {
	lst := p.Pres.Path("p:sldIdLst")
	ids := lst.Elems("p:sldId")
	if from < 0 || from >= len(ids) || to < 0 || to >= len(ids) {
		return fmt.Errorf("posició fora de rang (1–%d)", len(ids))
	}
	el := ids[from]
	el.Remove()
	ids = lst.Elems("p:sldId")
	if to >= len(ids) {
		lst.Append(el)
	} else {
		ids[to].InsertBefore(el)
	}
	return nil
}

// ---------- formes ----------

func (p *Pptx) nextShapeID(tree *Node) int {
	max := 1
	tree.Walk(func(x *Node) bool {
		if x.Name == "p:cNvPr" {
			if v, err := strconv.Atoi(x.Get("id")); err == nil && v > max {
				max = v
			}
		}
		return true
	})
	return max + 1
}

func (p *Pptx) slideTree(part string) (*Node, error) {
	doc, err := p.Pkg.XML(part)
	if err != nil {
		return nil, err
	}
	tree := doc.Root().Path("p:cSld", "p:spTree")
	if tree == nil {
		return nil, fmt.Errorf("%s sense spTree", part)
	}
	return tree, nil
}

// layoutOf torna el layout d'una diapositiva.
func (p *Pptx) layoutOf(slide string) (LayoutInfo, bool) {
	lp := p.Pkg.ByType(slide, RelSlideLayout)
	if len(lp) == 0 {
		return LayoutInfo{}, false
	}
	for _, l := range p.Layouts() {
		if l.Part == lp[0] {
			return l, true
		}
	}
	return LayoutInfo{}, false
}

// PlaceholderShape torna (o crea) la forma de la diapositiva lligada al
// marcador (tipus/idx) del layout.
func (p *Pptx) PlaceholderShape(slide string, ph PhInfo) (*Node, error) {
	tree, err := p.slideTree(slide)
	if err != nil {
		return nil, err
	}
	for _, sp := range tree.Elems("p:sp") {
		if t, idx, ok := phOf(sp); ok && phMatch(t, idx, ph.Type, ph.Idx) {
			return sp, nil
		}
	}
	typ := ph.Type
	if typ == "obj" {
		typ = ""
	}
	frag, err := ParseFragment(`<p:sp xmlns:p="` + nsP + `" xmlns:a="` + nsA + `">` +
		fmt.Sprintf(`<p:nvSpPr><p:cNvPr id="%d" name="%s"/><p:cNvSpPr><a:spLocks noGrp="1"/></p:cNvSpPr><p:nvPr>`, p.nextShapeID(tree), Esc(firstNonEmpty(ph.Name, "Placeholder"))) +
		phTag(typ, ph.Idx) + `</p:nvPr></p:nvSpPr><p:spPr/><p:txBody><a:bodyPr/><a:lstStyle/><a:p><a:endParaRPr lang="ca-ES"/></a:p></p:txBody></p:sp>`)
	if err != nil {
		return nil, err
	}
	sp := frag[0]
	sp.Del("xmlns:p")
	sp.Del("xmlns:a")
	// Abans del número de diapositiva, si n'hi ha (ordre de lectura).
	for _, x := range tree.Elems("p:sp") {
		if t, _, ok := phOf(x); ok && t == "sldNum" {
			x.InsertBefore(sp)
			return sp, nil
		}
	}
	tree.Append(sp)
	return sp, nil
}

func phTag(typ, idx string) string {
	s := `<p:ph`
	if typ != "" {
		s += ` type="` + typ + `"`
	}
	if idx != "" {
		s += ` idx="` + idx + `"`
	}
	return s + `/>`
}

func firstNonEmpty(xs ...string) string {
	for _, x := range xs {
		if x != "" {
			return x
		}
	}
	return ""
}

// TextOpts és el format extra d'un quadre de text.
type TextOpts struct {
	Size    int // centèsimes de punt (2000 = 20 pt); 0 = el del layout
	Bold    bool
	Color   string // hex
	Align   string // l, ctr, r
	NoBullet bool
}

// SetText posa contingut markdown dins d'una forma amb txBody,
// conservant bodyPr/lstStyle. Les llistes van amb nivells (lvl) i
// hereten les vinyetes del layout; el text normal va sense vinyeta.
func SetText(sp *Node, md string, o TextOpts) {
	tb := sp.Child("p:txBody")
	if tb == nil {
		tb = El("p:txBody").Add(El("a:bodyPr"), El("a:lstStyle"))
		sp.Append(tb)
	}
	for _, c := range tb.Elems("a:p") {
		c.Remove()
	}
	blocks := ParseMarkdown(md)
	if len(blocks) == 0 {
		tb.Append(El("a:p").Add(El("a:endParaRPr", "lang", "ca-ES")))
		return
	}
	for _, b := range blocks {
		for _, para := range slideParas(b, o) {
			tb.Append(para)
		}
	}
}

func slideParas(b Block, o TextOpts) []*Node {
	mk := func(lvl int, bullet string, ins []Inline, extra func(*Node)) *Node {
		ap := El("a:p")
		ppr := El("a:pPr")
		if lvl > 0 {
			ppr.Set("lvl", strconv.Itoa(lvl))
		}
		if o.Align != "" {
			ppr.Set("algn", o.Align)
		}
		switch bullet {
		case "none":
			ppr.Set("marL", "0")
			ppr.Set("indent", "0")
			ppr.Add(El("a:buNone"))
		case "num":
			ppr.Add(El("a:buFont", "typeface", "+mj-lt"), El("a:buAutoNum", "type", "arabicPeriod"))
		}
		if len(ppr.Attrs) > 0 || len(ppr.Children) > 0 {
			ap.Add(ppr)
		}
		for _, in := range ins {
			for i, part := range strings.Split(in.Text, "\n") {
				if i > 0 {
					ap.Add(El("a:br"))
				}
				if part == "" {
					continue
				}
				r := El("a:r")
				rpr := El("a:rPr", "lang", "ca-ES", "dirty", "0")
				if o.Size > 0 {
					rpr.Set("sz", strconv.Itoa(o.Size))
				}
				if in.Bold || o.Bold {
					rpr.Set("b", "1")
				}
				if in.Italic {
					rpr.Set("i", "1")
				}
				if in.Strike {
					rpr.Set("strike", "sngStrike")
				}
				if o.Color != "" {
					rpr.Add(El("a:solidFill").Add(El("a:srgbClr", "val", o.Color)))
				}
				if in.Code {
					rpr.Add(El("a:latin", "typeface", "Courier New"))
				}
				if extra != nil {
					extra(rpr)
				}
				r.Add(rpr, El("a:t").Add(Txt(part)))
				ap.Add(r)
			}
		}
		if len(ap.Elems("a:r")) == 0 {
			ap.Add(El("a:endParaRPr", "lang", "ca-ES"))
		}
		return ap
	}
	plain := "none"
	if !o.NoBullet {
		plain = "none"
	}
	switch b.Kind {
	case BBullet:
		if o.NoBullet {
			return []*Node{mk(b.Level, "none", b.Inlines, nil)}
		}
		return []*Node{mk(b.Level, "", b.Inlines, nil)}
	case BNumbered:
		return []*Node{mk(b.Level, "num", b.Inlines, nil)}
	case BHeading:
		ins := append([]Inline(nil), b.Inlines...)
		for i := range ins {
			ins[i].Bold = true
		}
		return []*Node{mk(0, plain, ins, nil)}
	case BQuote:
		ins := append([]Inline(nil), b.Inlines...)
		for i := range ins {
			ins[i].Italic = true
		}
		return []*Node{mk(0, plain, ins, nil)}
	case BCode:
		var out []*Node
		for _, l := range strings.Split(b.Text, "\n") {
			out = append(out, mk(0, "none", []Inline{{Text: l, Code: true}}, nil))
		}
		return out
	case BTable:
		// Una taula dins de text: files com a línies.
		var out []*Node
		for _, r := range b.Rows {
			var cells []string
			for _, c := range r {
				cells = append(cells, InlinesText(c))
			}
			out = append(out, mk(0, "none", PlainInlines(strings.Join(cells, " · ")), nil))
		}
		return out
	case BPara:
		return []*Node{mk(0, plain, b.Inlines, nil)}
	}
	return nil
}

// AddTextBox afegeix un quadre de text lliure.
func (p *Pptx) AddTextBox(slide string, b Box, md string, o TextOpts) (*Node, error) {
	tree, err := p.slideTree(slide)
	if err != nil {
		return nil, err
	}
	id := p.nextShapeID(tree)
	frag, err := ParseFragment(`<p:sp xmlns:p="` + nsP + `" xmlns:a="` + nsA + `">` +
		fmt.Sprintf(`<p:nvSpPr><p:cNvPr id="%d" name="TextBox %d"/><p:cNvSpPr txBox="1"/><p:nvPr/></p:nvSpPr><p:spPr><a:xfrm><a:off x="%d" y="%d"/><a:ext cx="%d" cy="%d"/></a:xfrm><a:prstGeom prst="rect"><a:avLst/></a:prstGeom><a:noFill/></p:spPr>`, id, id, b.X, b.Y, b.CX, b.CY) +
		`<p:txBody><a:bodyPr wrap="square" lIns="0" tIns="0" rIns="0" bIns="0" anchor="t"><a:normAutofit/></a:bodyPr><a:lstStyle/><a:p/></p:txBody></p:sp>`)
	if err != nil {
		return nil, err
	}
	sp := frag[0]
	sp.Del("xmlns:p")
	sp.Del("xmlns:a")
	o.NoBullet = true
	SetText(sp, md, o)
	tree.Append(sp)
	return sp, nil
}

// AddPicture posa una imatge encaixada (sense deformar) dins del box.
func (p *Pptx) AddPicture(slide, src string, b Box) error {
	path := src
	if !filepath.IsAbs(path) && p.BaseDir != "" {
		if _, err := os.Stat(filepath.Join(p.BaseDir, path)); err == nil {
			path = filepath.Join(p.BaseDir, path)
		}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("imatge no reconeguda (png/jpeg/gif): %v", err)
	}
	ext := map[string]string{"png": ".png", "jpeg": ".jpeg", "gif": ".gif"}[format]
	media := p.Pkg.NextPartName("ppt/media", "image", ext)
	p.Pkg.Put(media, data)
	p.Pkg.EnsureDefault(strings.TrimPrefix(ext, "."), "image/"+format)
	rid := p.Pkg.AddRel(slide, RelImage, media)
	// Encaixa conservant proporcions, centrada.
	cx, cy := b.CX, b.CY
	if cfg.Width > 0 && cfg.Height > 0 {
		ar := float64(cfg.Width) / float64(cfg.Height)
		if float64(b.CX)/float64(b.CY) > ar {
			cx = int(float64(b.CY) * ar)
		} else {
			cy = int(float64(b.CX) / ar)
		}
	}
	x, y := b.X+(b.CX-cx)/2, b.Y+(b.CY-cy)/2
	tree, err := p.slideTree(slide)
	if err != nil {
		return err
	}
	id := p.nextShapeID(tree)
	frag, err := ParseFragment(`<p:pic xmlns:p="` + nsP + `" xmlns:a="` + nsA + `" xmlns:r="` + nsR + `">` +
		fmt.Sprintf(`<p:nvPicPr><p:cNvPr id="%d" name="Picture %d" descr="%s"/><p:cNvPicPr><a:picLocks noChangeAspect="1"/></p:cNvPicPr><p:nvPr/></p:nvPicPr><p:blipFill><a:blip r:embed="%s"/><a:stretch><a:fillRect/></a:stretch></p:blipFill><p:spPr><a:xfrm><a:off x="%d" y="%d"/><a:ext cx="%d" cy="%d"/></a:xfrm><a:prstGeom prst="rect"><a:avLst/></a:prstGeom></p:spPr></p:pic>`,
			id, id, Esc(filepath.Base(path)), rid, x, y, cx, cy))
	if err != nil {
		return err
	}
	pic := frag[0]
	for _, a := range []string{"xmlns:p", "xmlns:a", "xmlns:r"} {
		pic.Del(a)
	}
	tree.Append(pic)
	return nil
}

// AddTable afegeix una taula amb format (capçalera amb color primari,
// files alternes suaus, números a la dreta).
func (p *Pptx) AddTable(slide string, rows [][]string, b Box) error {
	if len(rows) == 0 {
		return fmt.Errorf("taula buida")
	}
	ncol := 0
	for _, r := range rows {
		if len(r) > ncol {
			ncol = len(r)
		}
	}
	sz := 1700
	switch {
	case len(rows) > 12:
		sz = 1100
	case len(rows) > 8:
		sz = 1300
	case len(rows) > 6:
		sz = 1400
	}
	rowH := sz * 12700 * 21 / 10 / 100 // ~2,1 línies
	if total := rowH * len(rows); total > b.CY {
		rowH = b.CY / len(rows)
	}
	// Amplades segons contingut.
	w := make([]float64, ncol)
	tot := 0.0
	for _, r := range rows {
		for i, c := range r {
			n := float64(utf8.RuneCountInString(c))
			if n > 40 {
				n = 40
			}
			if n > w[i] {
				w[i] = n
			}
		}
	}
	for i := range w {
		if w[i] < 4 {
			w[i] = 4
		}
		tot += w[i]
	}
	widths := make([]int, ncol)
	sum := 0
	for i := range w {
		widths[i] = int(float64(b.CX) * w[i] / tot)
		sum += widths[i]
	}
	widths[ncol-1] += b.CX - sum
	numeric := make([]bool, ncol)
	for i := 0; i < ncol; i++ {
		all, any := true, false
		for _, r := range rows[1:] {
			if i < len(r) && strings.TrimSpace(r[i]) != "" {
				any = true
				if !isNumeric(r[i]) {
					all = false
				}
			}
		}
		numeric[i] = all && any
	}
	var tb strings.Builder
	tb.WriteString(`<a:tbl><a:tblPr firstRow="1" bandRow="1"/><a:tblGrid>`)
	for _, wd := range widths {
		tb.WriteString(fmt.Sprintf(`<a:gridCol w="%d"/>`, wd))
	}
	tb.WriteString(`</a:tblGrid>`)
	border := `<a:lnB w="6350"><a:solidFill><a:srgbClr val="D0D0D0"/></a:solidFill></a:lnB>`
	for ri, r := range rows {
		tb.WriteString(fmt.Sprintf(`<a:tr h="%d">`, rowH))
		for ci := 0; ci < ncol; ci++ {
			cell := ""
			if ci < len(r) {
				cell = r[ci]
			}
			fill, color, bold := "FFFFFF", p.Theme.Text, ""
			if ri == 0 {
				fill, color, bold = p.Theme.Primary, "FFFFFF", ` b="1"`
			} else if ri%2 == 0 {
				fill = p.Theme.Light
			}
			algn := "l"
			if numeric[ci] && ri > 0 {
				algn = "r"
			}
			tb.WriteString(fmt.Sprintf(`<a:tc><a:txBody><a:bodyPr/><a:lstStyle/><a:p><a:pPr algn="%s"/><a:r><a:rPr lang="ca-ES" sz="%d"%s dirty="0"><a:solidFill><a:srgbClr val="%s"/></a:solidFill></a:rPr><a:t>%s</a:t></a:r></a:p></a:txBody><a:tcPr marL="91440" marR="91440" marT="45720" marB="45720" anchor="ctr">%s<a:solidFill><a:srgbClr val="%s"/></a:solidFill></a:tcPr></a:tc>`,
				algn, sz, bold, color, Esc(trimMD(cell)), border, fill))
		}
		tb.WriteString(`</a:tr>`)
	}
	tb.WriteString(`</a:tbl>`)
	tree, err := p.slideTree(slide)
	if err != nil {
		return err
	}
	id := p.nextShapeID(tree)
	h := rowH * len(rows)
	frag, err := ParseFragment(`<p:graphicFrame xmlns:p="` + nsP + `" xmlns:a="` + nsA + `">` +
		fmt.Sprintf(`<p:nvGraphicFramePr><p:cNvPr id="%d" name="Table %d"/><p:cNvGraphicFramePr><a:graphicFrameLocks noGrp="1"/></p:cNvGraphicFramePr><p:nvPr/></p:nvGraphicFramePr><p:xfrm><a:off x="%d" y="%d"/><a:ext cx="%d" cy="%d"/></p:xfrm>`, id, id, b.X, b.Y, b.CX, h) +
		`<a:graphic><a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/table">` + tb.String() + `</a:graphicData></a:graphic></p:graphicFrame>`)
	if err != nil {
		return err
	}
	gf := frag[0]
	gf.Del("xmlns:p")
	gf.Del("xmlns:a")
	tree.Append(gf)
	return nil
}

func trimMD(s string) string {
	return strings.NewReplacer("**", "", "__", "", "`", "").Replace(strings.TrimSpace(s))
}

// SetNotes posa les notes de l'orador d'una diapositiva.
func (p *Pptx) SetNotes(slide, text string) error {
	nm := p.Pkg.ByType(p.Main, RelNotesMaster)
	if len(nm) == 0 {
		return fmt.Errorf("la presentació no té patró de notes; no s'hi poden afegir notes")
	}
	var part string
	if ns := p.Pkg.ByType(slide, RelNotesSlide); len(ns) > 0 {
		part = ns[0]
	} else {
		part = p.Pkg.NextPartName("ppt/notesSlides", "notesSlide", ".xml")
		p.Pkg.Put(part, []byte(xmlHead+`<p:notes xmlns:a="`+nsA+`" xmlns:r="`+nsR+`" xmlns:p="`+nsP+`"><p:cSld><p:spTree>`+spTreeHead+
			phSp(2, "Slide Image Placeholder 1", "sldImg", "", 0, 0, 0, 0, `<a:bodyPr/><a:lstStyle/><a:p><a:endParaRPr lang="ca-ES"/></a:p>`)+
			phSp(3, "Notes Placeholder 2", "body", "1", 0, 0, 0, 0, "")+
			`</p:spTree></p:cSld><p:clrMapOvr><a:masterClrMapping/></p:clrMapOvr></p:notes>`))
		// sldImg no porta txBody: el treiem.
		if d, err := p.Pkg.XML(part); err == nil {
			for _, sp := range d.Root().Path("p:cSld", "p:spTree").Elems("p:sp") {
				if t, _, ok := phOf(sp); ok && t == "sldImg" {
					if tb := sp.Child("p:txBody"); tb != nil {
						tb.Remove()
					}
				}
			}
		}
		p.Pkg.SetOverride(part, ctNotesSlide)
		p.Pkg.AddRel(part, RelNotesMaster, nm[0])
		p.Pkg.AddRel(part, RelSlide, slide)
		p.Pkg.AddRel(slide, RelNotesSlide, part)
	}
	d, err := p.Pkg.XML(part)
	if err != nil {
		return err
	}
	for _, sp := range d.Root().Path("p:cSld", "p:spTree").Elems("p:sp") {
		if t, _, ok := phOf(sp); ok && t == "body" {
			SetText(sp, text, TextOpts{NoBullet: true})
			return nil
		}
	}
	return fmt.Errorf("notes sense marcador de text")
}

// Save desa la presentació.
func (p *Pptx) Save(path string) error { return p.Pkg.Save(path) }

// KeepOnlySlide deixa només la diapositiva n (0-based): per renderitzar
// diapositives una a una (l'exportació a PNG de LibreOffice només fa la
// primera).
func (p *Pptx) KeepOnlySlide(n int) error {
	total := len(p.SlideParts())
	if n < 0 || n >= total {
		return fmt.Errorf("diapositiva %d no existeix (n'hi ha %d)", n+1, total)
	}
	for i := total - 1; i >= 0; i-- {
		if i != n {
			if err := p.DeleteSlide(i); err != nil {
				return err
			}
		}
	}
	return nil
}
