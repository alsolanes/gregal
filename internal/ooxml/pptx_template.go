package ooxml

import (
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

// ShapeInfo descriu una forma d'una diapositiva.
type ShapeInfo struct {
	ID       string
	Name     string
	Kind     string // text, picture, table, chart, group, other
	PhType   string
	PhIdx    string
	Box      Box
	Text     string
	Overflow float64 // alçada estimada / alçada disponible (0 si no és text)
	Node     *Node
}

// Shapes torna les formes d'una diapositiva amb geometria resolta
// (pròpia, del layout o del màster).
func (p *Pptx) Shapes(slide string) []ShapeInfo {
	tree, err := p.slideTree(slide)
	if err != nil {
		return nil
	}
	l, _ := p.layoutOf(slide)
	var out []ShapeInfo
	for _, sp := range tree.Elems() {
		si := ShapeInfo{Node: sp}
		switch sp.Name {
		case "p:sp":
			si.Kind = "text"
		case "p:pic":
			si.Kind = "picture"
		case "p:graphicFrame":
			si.Kind = "frame"
			if sp.First("a:tbl") != nil {
				si.Kind = "table"
			} else if sp.First("c:chart") != nil {
				si.Kind = "chart"
			}
		case "p:grpSp":
			si.Kind = "group"
		case "p:cxnSp":
			si.Kind = "line"
		default:
			continue
		}
		if c := sp.First("p:cNvPr"); c != nil {
			si.ID, si.Name = c.Get("id"), c.Get("name")
		}
		si.PhType, si.PhIdx, _ = phOf(sp)
		b, ok := xfrmOf(sp)
		if !ok && si.PhType != "" {
			for _, ph := range l.Phs {
				if phMatch(si.PhType, si.PhIdx, ph.Type, ph.Idx) {
					b = ph.Box
					break
				}
			}
			if b.CX == 0 {
				b, _ = p.masterPhBox(l.Master, si.PhType)
			}
		}
		si.Box = b
		if tb := sp.Child("p:txBody"); tb != nil {
			var lines []string
			for _, ap := range tb.Elems("a:p") {
				lines = append(lines, ParaText(ap))
			}
			si.Text = strings.TrimSpace(strings.Join(lines, "\n"))
			si.Overflow = estimateOverflow(tb, si.PhType, b)
		}
		out = append(out, si)
	}
	return out
}

// estimateOverflow calcula alçada de text estimada / alçada del quadre.
// Aproximat (amplada mitjana de caràcter 0,5 em), però detecta bé els
// casos clars: llistes massa llargues o títols de tres línies.
func estimateOverflow(tb *Node, phType string, b Box) float64 {
	if b.CX <= 0 || b.CY <= 0 {
		return 0
	}
	defSz := 2000
	switch phType {
	case "title":
		defSz = 3600
	case "ctrTitle":
		defSz = 4400
	case "subTitle":
		defSz = 2000
	case "sldNum", "dt", "ftr":
		return 0
	}
	if ls := tb.Child("a:lstStyle"); ls != nil {
		if d := ls.First("a:defRPr"); d != nil {
			if v, err := strconv.Atoi(d.Get("sz")); err == nil {
				defSz = v
			}
		}
	}
	lIns, rIns, tIns, bIns := 91440, 91440, 45720, 45720
	if bp := tb.Child("a:bodyPr"); bp != nil {
		get := func(k string, def int) int {
			if v, err := strconv.Atoi(bp.Get(k)); err == nil {
				return v
			}
			return def
		}
		lIns, rIns, tIns, bIns = get("lIns", lIns), get("rIns", rIns), get("tIns", tIns), get("bIns", bIns)
	}
	width := float64(b.CX-lIns-rIns) / emuIn
	if width <= 0.2 {
		return 0
	}
	height := 0.0
	for _, ap := range tb.Elems("a:p") {
		sz := defSz
		lvl := 0
		if ppr := ap.Child("a:pPr"); ppr != nil {
			lvl, _ = strconv.Atoi(ppr.Get("lvl"))
		}
		if lvl > 0 && phType != "title" && phType != "ctrTitle" {
			sz = defSz - 200*lvl
		}
		for _, r := range ap.Elems("a:r") {
			if rp := r.Child("a:rPr"); rp != nil {
				if v, err := strconv.Atoi(rp.Get("sz")); err == nil {
					sz = v
					break
				}
			}
		}
		pt := float64(sz) / 100
		txt := ParaText(ap)
		indent := 0.0
		if phType == "body" || phType == "obj" || phType == "" {
			indent = 0.3 + 0.45*float64(lvl)
		}
		avail := width - indent
		if avail < 0.5 {
			avail = 0.5
		}
		lines := 0
		for _, seg := range strings.Split(txt, "\n") {
			chars := float64(utf8.RuneCountInString(seg))
			n := int(math.Ceil(chars * 0.5 * pt / 72 / avail))
			if n < 1 {
				n = 1
			}
			lines += n
		}
		spc := 0.0
		if phType != "title" && phType != "ctrTitle" {
			spc = 9.0 / 72
		}
		height += float64(lines)*pt*1.2/72 + spc
	}
	avail := float64(b.CY-tIns-bIns) / emuIn
	if avail <= 0 {
		return 0
	}
	return height / avail
}

// Inventory descriu layouts i diapositives (per omplir plantilles).
func Inventory(p *Pptx) string {
	var b strings.Builder
	ls := p.Layouts()
	b.WriteString(fmt.Sprintf("Layouts (%d):\n", len(ls)))
	for i, l := range ls {
		var phs []string
		for _, ph := range l.Phs {
			s := ph.Type
			if ph.Idx != "" {
				s += "#" + ph.Idx
			}
			phs = append(phs, s)
		}
		b.WriteString(fmt.Sprintf("  %d. %q [%s] %s\n", i+1, l.Name, l.Type, strings.Join(phs, ", ")))
	}
	for i, s := range p.SlideParts() {
		l, _ := p.layoutOf(s)
		b.WriteString(fmt.Sprintf("\nDiapositiva %d (layout %q):\n", i+1, l.Name))
		for _, sh := range p.Shapes(s) {
			if sh.PhType == "sldNum" || sh.PhType == "dt" || sh.PhType == "ftr" {
				continue
			}
			key := sh.Kind
			if sh.PhType != "" {
				key = "ph:" + sh.PhType
				if sh.PhIdx != "" {
					key += "#" + sh.PhIdx
				}
			}
			line := fmt.Sprintf("  id=%s %s %q [%s]", sh.ID, key, sh.Name, sh.Box.In())
			if sh.Text != "" {
				t := sh.Text
				if len([]rune(t)) > 160 {
					t = string([]rune(t)[:160]) + "…"
				}
				line += " «" + strings.ReplaceAll(t, "\n", " ⏎ ") + "»"
			}
			if sh.Overflow > 1.05 {
				line += fmt.Sprintf(" ⚠ desborda (~%d%%)", int(sh.Overflow*100))
			}
			b.WriteString(line + "\n")
		}
		if ns := p.Pkg.ByType(s, RelNotesSlide); len(ns) > 0 {
			if nd, err := p.Pkg.XML(ns[0]); err == nil {
				for _, sp := range nd.Root().Path("p:cSld", "p:spTree").Elems("p:sp") {
					if t, _, ok := phOf(sp); ok && t == "body" {
						if txt := strings.TrimSpace(sp.InnerText()); txt != "" {
							b.WriteString("  notes: " + txt + "\n")
						}
					}
				}
			}
		}
	}
	return b.String()
}

// ReadPptx torna el text de les diapositives (vista ràpida).
func ReadPptx(pk *Package) (string, error) {
	p, err := OpenPptx(pk)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	for i, s := range p.SlideParts() {
		b.WriteString(fmt.Sprintf("— Diapo %d —\n", i+1))
		for _, sh := range p.Shapes(s) {
			if sh.PhType == "sldNum" || sh.PhType == "dt" || sh.PhType == "ftr" {
				continue
			}
			switch {
			case sh.Text != "":
				if sh.PhType == "title" || sh.PhType == "ctrTitle" {
					b.WriteString("# " + sh.Text + "\n")
				} else {
					b.WriteString(sh.Text + "\n")
				}
			case sh.Kind == "table":
				for _, tr := range sh.Node.Find("a:tr") {
					var cells []string
					for _, tc := range tr.Elems("a:tc") {
						cells = append(cells, strings.TrimSpace(tc.InnerText()))
					}
					b.WriteString("| " + strings.Join(cells, " | ") + " |\n")
				}
			case sh.Kind == "chart":
				b.WriteString("[gràfic]\n")
			case sh.Kind == "picture":
				b.WriteString("[imatge]\n")
			}
		}
		if ns := p.Pkg.ByType(s, RelNotesSlide); len(ns) > 0 {
			if nd, err := p.Pkg.XML(ns[0]); err == nil {
				for _, sp := range nd.Root().Path("p:cSld", "p:spTree").Elems("p:sp") {
					if t, _, ok := phOf(sp); ok && t == "body" {
						if txt := strings.TrimSpace(sp.InnerText()); txt != "" {
							b.WriteString("[notes] " + txt + "\n")
						}
					}
				}
			}
		}
	}
	return b.String(), nil
}

// FillShapes posa text (markdown) a formes d'una diapositiva (0-based).
// Claus: "title", "subtitle", "body" (= body #1), "body2"…, el nom de la
// forma o "id=N". Torna quantes formes s'han omplert.
func FillShapes(p *Pptx, slideIdx int, texts map[string]string) (int, error) {
	sl := p.SlideParts()
	if slideIdx < 0 || slideIdx >= len(sl) {
		return 0, fmt.Errorf("diapositiva %d no existeix (n'hi ha %d)", slideIdx+1, len(sl))
	}
	slide := sl[slideIdx]
	shapes := p.Shapes(slide)
	l, _ := p.layoutOf(slide)
	var bodies []*Node
	var titleSp, subSp *Node
	for _, sh := range shapes {
		switch sh.PhType {
		case "title", "ctrTitle":
			titleSp = sh.Node
		case "subTitle":
			subSp = sh.Node
			bodies = append(bodies, sh.Node)
		case "body", "obj":
			bodies = append(bodies, sh.Node)
		}
	}
	keys := make([]string, 0, len(texts))
	for k := range texts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	n := 0
	for _, k := range keys {
		md := texts[k]
		lk := strings.ToLower(strings.TrimSpace(k))
		var target *Node
		switch {
		case lk == "title":
			target = titleSp
			if target == nil {
				for _, ph := range l.Phs {
					if ph.Type == "title" || ph.Type == "ctrTitle" {
						target, _ = p.PlaceholderShape(slide, ph)
					}
				}
			}
		case lk == "subtitle":
			target = subSp
			if target == nil && len(bodies) > 0 {
				target = bodies[0]
			}
		case strings.HasPrefix(lk, "body"):
			i := 1
			if s := strings.TrimPrefix(lk, "body"); s != "" {
				i, _ = strconv.Atoi(s)
			}
			if i >= 1 && i <= len(bodies) {
				target = bodies[i-1]
			} else {
				// Marcador del layout encara no present a la diapositiva.
				j := 0
				for _, ph := range l.Phs {
					if ph.Type == "body" || ph.Type == "obj" || ph.Type == "subTitle" {
						j++
						if j == i {
							target, _ = p.PlaceholderShape(slide, ph)
						}
					}
				}
			}
		case strings.HasPrefix(lk, "id="):
			for _, sh := range shapes {
				if sh.ID == strings.TrimPrefix(lk, "id=") {
					target = sh.Node
				}
			}
		default:
			for _, sh := range shapes {
				if strings.EqualFold(sh.Name, k) {
					target = sh.Node
				}
			}
		}
		if target == nil || target.Name != "p:sp" {
			var avail []string
			for _, sh := range shapes {
				if sh.Kind == "text" && sh.PhType != "sldNum" {
					avail = append(avail, fmt.Sprintf("id=%s %q", sh.ID, sh.Name))
				}
			}
			return n, fmt.Errorf("forma %q no trobada a la diapositiva %d (n'hi ha: %s; o fes servir title/subtitle/body/body2)", k, slideIdx+1, strings.Join(avail, ", "))
		}
		o := TextOpts{}
		if target == titleSp {
			o.NoBullet = true
		}
		keepRunFormat(target, func() { SetText(target, md, o) })
		n++
	}
	return n, nil
}

// keepRunFormat conserva el format de la primera run (mida, color,
// tipus de lletra) quan es reemplaça el text d'una forma dissenyada.
func keepRunFormat(sp *Node, apply func()) {
	var proto *Node
	if r := sp.First("a:rPr"); r != nil {
		proto = r.Clone()
	}
	var pproto *Node
	if pp := sp.First("a:pPr"); pp != nil {
		pproto = pp.Clone()
	}
	apply()
	if proto == nil && pproto == nil {
		return
	}
	for _, ap := range sp.Child("p:txBody").Elems("a:p") {
		if pproto != nil && ap.Child("a:pPr") == nil {
			c := pproto.Clone()
			c.Del("lvl")
			ap.Prepend(c)
		}
		for _, r := range ap.Elems("a:r") {
			rp := r.Child("a:rPr")
			if proto == nil || rp == nil {
				continue
			}
			merged := proto.Clone()
			for _, a := range rp.Attrs {
				if a.Name == "b" || a.Name == "i" || a.Name == "strike" {
					merged.Set(a.Name, a.Value)
				}
			}
			for _, c := range rp.Elems() {
				if merged.Child(c.Name) == nil {
					merged.Append(c.Clone())
				}
			}
			rp.Replace(merged)
		}
	}
}

// ---------- QA estructural ----------

var leftoverRe = regexp.MustCompile(`(?i)\b(lorem|ipsum|x{3,}|todo|tbd)\b|\[insert|click to (add|edit)|feu clic per|haga clic para`)

// QAPptx fa comprovacions sense render: formes fora de la diapositiva,
// solapaments entre formes amb text, text que desborda i restes de
// text de mostra.
func QAPptx(p *Pptx) []string {
	var out []string
	cx, cy := SlideW, SlideH
	if sz := p.Pres.Child("p:sldSz"); sz != nil {
		cx, _ = strconv.Atoi(sz.Get("cx"))
		cy, _ = strconv.Atoi(sz.Get("cy"))
	}
	for i, s := range p.SlideParts() {
		shapes := p.Shapes(s)
		n := i + 1
		for _, sh := range shapes {
			if sh.PhType == "sldNum" || sh.PhType == "dt" || sh.PhType == "ftr" {
				continue
			}
			b := sh.Box
			if b.CX > 0 && (b.X < 0 || b.Y < 0 || b.X+b.CX > cx+12700 || b.Y+b.CY > cy+12700) {
				out = append(out, fmt.Sprintf("diapositiva %d: %q surt de la diapositiva (%s)", n, sh.Name, b.In()))
			}
			if sh.Overflow > 1.05 {
				out = append(out, fmt.Sprintf("diapositiva %d: el text de %q probablement desborda (~%d%% de l'espai); escurça'l o divideix la diapositiva", n, sh.Name, int(sh.Overflow*100)))
			}
			if m := leftoverRe.FindString(sh.Text); m != "" {
				out = append(out, fmt.Sprintf("diapositiva %d: text de mostra a %q («%s»)", n, sh.Name, m))
			}
			if sh.PhType != "" && sh.Kind == "text" && sh.Text == "" && sh.PhType != "pic" {
				out = append(out, fmt.Sprintf("diapositiva %d: marcador %q buit (omple'l o esborra'l)", n, sh.Name))
			}
		}
		for a := 0; a < len(shapes); a++ {
			for b := a + 1; b < len(shapes); b++ {
				s1, s2 := shapes[a], shapes[b]
				if s1.Box.CX == 0 || s2.Box.CX == 0 || s1.PhType == "sldNum" || s2.PhType == "sldNum" {
					continue
				}
				if s1.Text == "" && s2.Text == "" && s1.Kind == "text" && s2.Kind == "text" {
					continue
				}
				ov := overlap(s1.Box, s2.Box)
				small := math.Min(float64(s1.Box.CX)*float64(s1.Box.CY), float64(s2.Box.CX)*float64(s2.Box.CY))
				if small > 0 && ov/small > 0.08 {
					out = append(out, fmt.Sprintf("diapositiva %d: %q i %q se solapen (%d%%)", n, s1.Name, s2.Name, int(100*ov/small)))
				}
			}
		}
	}
	return out
}

func overlap(a, b Box) float64 {
	x := math.Max(0, math.Min(float64(a.X+a.CX), float64(b.X+b.CX))-math.Max(float64(a.X), float64(b.X)))
	y := math.Max(0, math.Min(float64(a.Y+a.CY), float64(b.Y+b.CY))-math.Max(float64(a.Y), float64(b.Y)))
	return x * y
}

// LeftoverText busca restes de text de mostra en qualsevol document.
func LeftoverText(s string) []string {
	var out []string
	seen := map[string]bool{}
	for _, m := range leftoverRe.FindAllString(s, -1) {
		k := strings.ToLower(m)
		if !seen[k] {
			seen[k] = true
			out = append(out, m)
		}
	}
	return out
}
