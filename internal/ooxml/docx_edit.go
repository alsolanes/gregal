package ooxml

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// textParts torna els parts d'un docx que tenen text editable: cos,
// capçaleres, peus, notes al peu/final i comentaris.
func (d *Docx) textParts() []string {
	out := []string{d.Main}
	for _, typ := range []string{RelHeader, RelFooter, RelFootnotes, "http://schemas.openxmlformats.org/officeDocument/2006/relationships/endnotes", RelComments} {
		out = append(out, d.Pkg.ByType(d.Main, typ)...)
	}
	return out
}

// ReplaceOpts configura una substitució.
type ReplaceOpts struct {
	Regex      bool
	IgnoreCase bool
	Track      bool
	Author     string
}

// Replace substitueix a tots els parts de text. Torna quantes
// coincidències i a quins parts.
func (d *Docx) Replace(find, repl string, o ReplaceOpts) (int, error) {
	f, err := NewFinder(find, o.Regex, o.IgnoreCase)
	if err != nil {
		return 0, err
	}
	var rv *Revision
	if o.Track {
		rv = NewRevision(d.Doc, o.Author)
	}
	total := 0
	for _, part := range d.textParts() {
		doc, err := d.Pkg.XML(part)
		if err != nil {
			continue
		}
		for _, p := range Paragraphs(doc) {
			total += ReplaceInPara(p, f, repl, rv)
		}
	}
	return total, nil
}

// FillVars substitueix marcadors {{nom}} (també partits en runs) pels
// valors donats. Torna els marcadors que han quedat sense valor.
func (d *Docx) FillVars(vars map[string]string) (int, []string) {
	n := 0
	keys := make([]string, 0, len(vars))
	for k := range vars {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		c, _ := d.Replace(`\{\{\s*`+regexp.QuoteMeta(k)+`\s*\}\}`, strings.ReplaceAll(vars[k], "$", "$$"), ReplaceOpts{Regex: true})
		n += c
	}
	left := map[string]bool{}
	re := regexp.MustCompile(`\{\{\s*([^{}]+?)\s*\}\}`)
	for _, part := range d.textParts() {
		doc, err := d.Pkg.XML(part)
		if err != nil {
			continue
		}
		for _, p := range Paragraphs(doc) {
			for _, m := range re.FindAllStringSubmatch(ParaText(p), -1) {
				left[m[1]] = true
			}
		}
	}
	var out []string
	for k := range left {
		out = append(out, k)
	}
	sort.Strings(out)
	return n, out
}

// anchorPara troba el paràgraf d'ancoratge al cos.
func (d *Docx) anchorPara(anchor string) (*Node, int, error) {
	if strings.TrimSpace(anchor) == "" {
		return nil, 0, fmt.Errorf("cal un text d'ancoratge (anchor)")
	}
	ps := FindParagraph(d.Body, anchor)
	if len(ps) == 0 {
		return nil, 0, fmt.Errorf("cap paràgraf conté %q (llegeix el document amb office_read per copiar el text exacte)", anchor)
	}
	return ps[0], len(ps), nil
}

// blockOf puja del paràgraf fins al fill directe del cos (per inserir
// després d'una taula sencera si el paràgraf és dins d'una cel·la).
func (d *Docx) blockOf(p *Node) *Node {
	cur := p
	for cur.Parent != nil && cur.Parent != d.Body {
		if cur.Parent.Name == "w:tc" {
			// Dins d'una cel·la: insereix dins la cel·la mateixa.
			return cur
		}
		cur = cur.Parent
	}
	return cur
}

// markInserted marca nodes nous com a inserció registrada.
func markInserted(nodes []*Node, rv *Revision) {
	for _, n := range nodes {
		var ps []*Node
		if n.Name == "w:p" {
			ps = []*Node{n}
		} else {
			ps = n.Find("w:p")
		}
		for _, p := range ps {
			ppr := p.Child("w:pPr")
			if ppr == nil {
				ppr = El("w:pPr")
				p.Prepend(ppr)
			}
			ppr.EnsureChild("w:rPr", pPrOrder).Prepend(rv.mark("w:ins"))
			for _, r := range p.Elems() {
				if r.Name == "w:r" || r.Name == "w:hyperlink" {
					ins := rv.mark("w:ins")
					r.Replace(ins)
					ins.Append(r)
				}
			}
		}
		for _, tr := range n.Find("w:tr") {
			trpr := tr.Child("w:trPr")
			if trpr == nil {
				trpr = El("w:trPr")
				tr.Prepend(trpr)
			}
			trpr.Append(rv.mark("w:ins"))
		}
	}
}

// Insert afegeix markdown abans o després del paràgraf que conté
// anchor (o al final si position="end"). Torna nodes afegits i avís.
func (d *Docx) Insert(anchor, position, md string, track bool, author string) (int, string, error) {
	nodes := d.Blocks(ParseMarkdown(md))
	if len(nodes) == 0 {
		return 0, "", fmt.Errorf("contingut buit")
	}
	if track {
		markInserted(nodes, NewRevision(d.Doc, author))
	}
	warn := ""
	switch position {
	case "end", "":
		if anchor == "" {
			d.AppendBlocksRaw(nodes...)
			return len(nodes), "", nil
		}
		position = "after"
		fallthrough
	case "after", "before":
		p, n, err := d.anchorPara(anchor)
		if err != nil {
			return 0, "", err
		}
		if n > 1 {
			warn = fmt.Sprintf(" (avís: %d paràgrafs contenen l'ancoratge; s'ha fet servir el primer)", n)
		}
		b := d.blockOf(p)
		if position == "before" {
			b.InsertBefore(nodes...)
		} else {
			b.InsertAfter(nodes...)
		}
	case "start":
		first := d.Body.Elems()
		if len(first) > 0 && first[0].Name != "w:sectPr" {
			first[0].InsertBefore(nodes...)
		} else {
			d.AppendBlocksRaw(nodes...)
		}
	default:
		return 0, "", fmt.Errorf("position %q desconeguda (before|after|start|end)", position)
	}
	return len(nodes), warn, nil
}

// DeleteParagraphs esborra els paràgrafs que contenen anchor (tots, o
// només el primer si all=false). Amb track, queden marcats com a esborrats.
func (d *Docx) DeleteParagraphs(anchor string, all, track bool, author string) (int, error) {
	ps := FindParagraph(d.Body, anchor)
	if len(ps) == 0 {
		return 0, fmt.Errorf("cap paràgraf conté %q", anchor)
	}
	if !all {
		ps = ps[:1]
	}
	var rv *Revision
	if track {
		rv = NewRevision(d.Doc, author)
	}
	for _, p := range ps {
		if rv == nil {
			// Un paràgraf sol dins d'una cel·la no es pot treure (la
			// cel·la quedaria buida): es buida.
			if p.Parent != nil && p.Parent.Name == "w:tc" && len(p.Parent.Elems("w:p")) == 1 {
				for _, c := range p.Elems() {
					if c.Name != "w:pPr" {
						c.Remove()
					}
				}
				continue
			}
			p.Remove()
			continue
		}
		ppr := p.Child("w:pPr")
		if ppr == nil {
			ppr = El("w:pPr")
			p.Prepend(ppr)
		}
		ppr.EnsureChild("w:rPr", pPrOrder).Prepend(rv.mark("w:del"))
		for _, r := range runsOf(p, wordD) {
			del := rv.mark("w:del")
			r.Replace(del)
			toDeleted(r)
			del.Append(r)
		}
	}
	return len(ps), nil
}

// ---------- comentaris ----------

func (d *Docx) commentsDoc() (*Node, error) {
	if ps := d.Pkg.ByType(d.Main, RelComments); len(ps) > 0 {
		return d.Pkg.XML(ps[0])
	}
	part := "word/comments.xml"
	d.Pkg.Put(part, []byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`+"\n"+`<w:comments xmlns:w="`+nsW+`" xmlns:r="`+nsR+`"/>`))
	d.Pkg.SetOverride(part, ctComments)
	d.Pkg.AddRel(d.Main, RelComments, part)
	return d.Pkg.XML(part)
}

// AddComment afegeix un comentari ancorat al primer fragment de text
// que coincideix amb anchor.
func (d *Docx) AddComment(anchor, text, author string) error {
	if author == "" {
		author = "Gregal"
	}
	p, _, err := d.anchorPara(anchor)
	if err != nil {
		return err
	}
	MergeRuns(p)
	ptext := ParaText(p)
	start := strings.Index(ptext, strings.TrimSpace(anchor))
	runs := isolate(p, wordD, start, start+len(strings.TrimSpace(anchor)))
	cd, err := d.commentsDoc()
	if err != nil {
		return err
	}
	rv := NewRevision(d.Doc, author)
	// Els ids de comentari han de ser únics també dins comments.xml.
	cd.Walk(func(x *Node) bool {
		if v, err := strconv.Atoi(x.Get("w:id")); err == nil && v >= *rv.nextID {
			*rv.nextID = v + 1
		}
		return true
	})
	id := rv.id()
	c := El("w:comment", "w:id", id, "w:author", author, "w:date", rv.Date, "w:initials", initials(author))
	for i, line := range strings.Split(text, "\n") {
		cp := El("w:p")
		if i == 0 {
			cp.Add(El("w:r").Add(El("w:annotationRef")))
		}
		cp.Add(El("w:r").Add(El("w:t", "xml:space", "preserve").Add(Txt(line))))
		c.Add(cp)
	}
	cd.Root().Append(c)
	ref := El("w:r").Add(El("w:commentReference", "w:id", id))
	if len(runs) == 0 {
		p.Append(El("w:commentRangeStart", "w:id", id), El("w:commentRangeEnd", "w:id", id), ref)
		return nil
	}
	runs[0].InsertBefore(El("w:commentRangeStart", "w:id", id))
	last := runs[len(runs)-1]
	last.InsertAfter(El("w:commentRangeEnd", "w:id", id))
	last.Parent.Children[last.Index()+1].InsertAfter(ref)
	return nil
}

func initials(s string) string {
	var b strings.Builder
	for _, w := range strings.Fields(s) {
		b.WriteString(strings.ToUpper(string([]rune(w)[:1])))
	}
	return b.String()
}

// SetTrackRevisions activa/desactiva el control de canvis del document.
func (d *Docx) SetTrackRevisions(on bool) {
	sp := d.Pkg.ByType(d.Main, RelSettings)
	if len(sp) == 0 {
		return
	}
	sd, err := d.Pkg.XML(sp[0])
	if err != nil {
		return
	}
	if on {
		sd.Root().EnsureChild("w:trackRevisions", settingsOrder)
	} else if c := sd.Root().Child("w:trackRevisions"); c != nil {
		c.Remove()
	}
}

// AcceptAll accepta tots els canvis registrats (w:ins es desembolica,
// w:del s'esborra).
func (d *Docx) AcceptAll() int {
	n := 0
	for _, part := range d.textParts() {
		doc, err := d.Pkg.XML(part)
		if err != nil {
			continue
		}
		for _, x := range doc.Find("w:del") {
			if x.Parent != nil && x.Parent.Name == "w:rPr" {
				// marca de paràgraf esborrat: el paràgraf s'uneix al següent
				if p := x.Parent.Parent.Parent; p != nil && p.Name == "w:p" {
					if next := nextSiblingElem(p); next != nil && next.Name == "w:p" {
						for _, c := range p.Elems() {
							if c.Name != "w:pPr" {
								c.Remove()
								next.InsertAt(firstContentIndex(next), c)
							}
						}
					}
					p.Remove()
				}
				n++
				continue
			}
			x.Remove()
			n++
		}
		for _, x := range doc.Find("w:ins") {
			if x.Parent != nil && x.Parent.Name == "w:rPr" || x.Parent != nil && x.Parent.Name == "w:trPr" {
				x.Remove()
			} else {
				x.Replace(x.Children...)
			}
			n++
		}
	}
	return n
}

func nextSiblingElem(n *Node) *Node {
	i := n.Index()
	if i < 0 {
		return nil
	}
	for _, c := range n.Parent.Children[i+1:] {
		if c.Kind == ElemNode {
			return c
		}
	}
	return nil
}

func firstContentIndex(p *Node) int {
	for i, c := range p.Children {
		if c.Kind == ElemNode && c.Name != "w:pPr" {
			return i
		}
	}
	return len(p.Children)
}

// ---------- lectura rica ----------

// ReadDocx torna una vista textual fidel: estils de paràgraf, llistes,
// taules cel·la a cel·la, canvis registrats, comentaris, capçaleres i
// peus. maxPara limita la sortida (0 = sense límit).
func ReadDocx(pk *Package, maxPara int) (string, error) {
	d, err := OpenDocx(pk)
	if err != nil {
		return "", err
	}
	st := map[string]string{}
	if sd, err := pk.XML(d.stylesPart()); err == nil {
		for _, s := range sd.Root().Elems("w:style") {
			st[s.Get("w:styleId")] = s.Child("w:name").Get("w:val")
		}
	}
	var b strings.Builder
	count := 0
	var para func(p *Node, indent string)
	para = func(p *Node, indent string) {
		if maxPara > 0 && count >= maxPara {
			return
		}
		count++
		txt := ParaText(p)
		style := ""
		if ps := p.Path("w:pPr", "w:pStyle"); ps != nil {
			style = st[ps.Get("w:val")]
			if style == "" {
				style = ps.Get("w:val")
			}
		}
		prefix := indent
		lower := strings.ToLower(style)
		switch {
		case strings.HasPrefix(lower, "heading "):
			n, _ := strconv.Atoi(strings.TrimPrefix(lower, "heading "))
			prefix += strings.Repeat("#", max(n, 1)) + " "
		case lower == "title":
			prefix += "[Title] "
		case style != "" && lower != "normal" && lower != "list paragraph":
			prefix += "[" + style + "] "
		}
		if np := p.Path("w:pPr", "w:numPr"); np != nil {
			lvl, _ := strconv.Atoi(np.Child("w:ilvl").Get("w:val"))
			prefix += strings.Repeat("  ", lvl) + "- "
		}
		var extra []string
		if dels := p.Find("w:delText"); len(dels) > 0 {
			var dt strings.Builder
			for _, x := range dels {
				dt.WriteString(x.InnerText())
			}
			extra = append(extra, "esborrat: «"+dt.String()+"»")
		}
		if len(p.Find("w:ins")) > 0 {
			extra = append(extra, "amb insercions registrades")
		}
		for _, cr := range p.Find("w:commentReference") {
			extra = append(extra, "comentari #"+cr.Get("w:id"))
		}
		if txt == "" && len(extra) == 0 {
			b.WriteString("\n")
			return
		}
		b.WriteString(prefix + txt)
		if len(extra) > 0 {
			b.WriteString("  ⟨" + strings.Join(extra, "; ") + "⟩")
		}
		b.WriteString("\n")
	}
	var block func(n *Node, indent string)
	block = func(n *Node, indent string) {
		for _, c := range n.Elems() {
			switch c.Name {
			case "w:p":
				para(c, indent)
			case "w:tbl":
				rows := c.Elems("w:tr")
				b.WriteString(fmt.Sprintf("%s[taula %d files]\n", indent, len(rows)))
				for _, tr := range rows {
					var cells []string
					for _, tc := range tr.Elems("w:tc") {
						var ct []string
						for _, p := range tc.Find("w:p") {
							ct = append(ct, ParaText(p))
						}
						cells = append(cells, strings.Join(ct, " / "))
					}
					b.WriteString(indent + "| " + strings.Join(cells, " | ") + " |\n")
				}
			case "w:sdt":
				block(c.Child("w:sdtContent"), indent)
			case "w:sectPr":
			default:
				if c.Find("w:p") != nil {
					block(c, indent)
				}
			}
		}
	}
	for _, h := range pk.ByType(d.Main, RelHeader) {
		if hd, err := pk.XML(h); err == nil {
			if t := strings.TrimSpace(hd.InnerText()); t != "" {
				b.WriteString("[capçalera] " + t + "\n")
			}
		}
	}
	block(d.Body, "")
	if maxPara > 0 && count >= maxPara {
		b.WriteString(fmt.Sprintf("… (retallat a %d paràgrafs)\n", maxPara))
	}
	for _, f := range pk.ByType(d.Main, RelFooter) {
		if fd, err := pk.XML(f); err == nil {
			if t := strings.TrimSpace(fd.InnerText()); t != "" {
				b.WriteString("[peu] " + t + "\n")
			}
		}
	}
	for _, cpart := range pk.ByType(d.Main, RelComments) {
		if cd, err := pk.XML(cpart); err == nil {
			for _, c := range cd.Root().Elems("w:comment") {
				b.WriteString(fmt.Sprintf("[comentari #%s, %s] %s\n", c.Get("w:id"), c.Get("w:author"), strings.TrimSpace(c.InnerText())))
			}
		}
	}
	return b.String(), nil
}
