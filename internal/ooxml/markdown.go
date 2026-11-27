package ooxml

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	east "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/text"
)

// Model neutre de contingut, comú al docx i al pptx: el model escriu
// markdown i aquí es converteix en blocs amb format de debò (estils,
// llistes numerades, taules), no en text amb asteriscs.

// Inline és un tros de text amb format.
type Inline struct {
	Text   string
	Bold   bool
	Italic bool
	Code   bool
	Strike bool
	Link   string
	// Format ampliat: ++subratllat++ i [text]{color=#rrggbb bg=#rrggbb
	// font="Nom" size=N}. Colors en hex sense «#», mida en punts.
	Underline bool
	Color     string
	Bg        string
	Font      string
	Size      float64
}

// BlockKind és el tipus de bloc.
type BlockKind int

const (
	BPara BlockKind = iota
	BHeading
	BBullet
	BNumbered
	BQuote
	BCode
	BTable
	BRule
	BPageBreak
	BImage
	BTOC
)

// Block és un bloc de contingut.
type Block struct {
	Kind    BlockKind
	Level   int // títol 1–6; nivell de llista 0–8
	Inlines []Inline
	Text    string       // codi
	Rows    [][][]Inline // taula: files → cel·les → inlines (la primera és capçalera)
	Align   []string     // alineació per columna: "", "left", "center", "right"
	Image   string       // ruta de la imatge
	Alt     string
	ListID  int // llistes numerades: cada llista nova reinicia
	// ParaAlign ve d'una línia «{align=…}» just després del paràgraf o
	// títol: "left", "center", "right" o "justify".
	ParaAlign string
}

var (
	pageBreakRe = regexp.MustCompile(`(?i)^\s*(\\pagebreak|\[pagebreak\]|<!--\s*pagebreak\s*-->)\s*$`)
	tocRe       = regexp.MustCompile(`(?i)^\s*(\[toc\]|\[\[toc\]\]|\{\{toc\}\})\s*$`)
	alignRe     = regexp.MustCompile(`^\{align=(left|center|right|justify)\}$`)
	// La línia d'alineació passa a goldmark com a marcador perquè quedi
	// enganxada al paràgraf (o sola, després d'un títol) i no es confongui
	// amb el mateix text escrit dins d'una línia.
	alignMarkRe = regexp.MustCompile("\x00ALIGN=([a-z]+)\x00")
)

// ParseMarkdown converteix markdown (CommonMark + taules GFM +
// ratllat) en blocs. Admet també «\pagebreak» i «[TOC]» en una línia.
func ParseMarkdown(src string) []Block {
	src = strings.ReplaceAll(src, "\r\n", "\n")
	// Línies especials: es converteixen en marcadors que goldmark
	// tracta com a paràgrafs propis.
	var lines []string
	for _, l := range strings.Split(src, "\n") {
		switch {
		case pageBreakRe.MatchString(l):
			lines = append(lines, "", "\x00PAGEBREAK\x00", "")
		case tocRe.MatchString(l):
			lines = append(lines, "", "\x00TOC\x00", "")
		case alignRe.MatchString(l) && len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) != "":
			lines = append(lines, "\x00ALIGN="+alignRe.FindStringSubmatch(l)[1]+"\x00")
		default:
			lines = append(lines, l)
		}
	}
	srcB := []byte(strings.Join(lines, "\n"))
	md := goldmark.New(goldmark.WithExtensions(extension.Table, extension.Strikethrough))
	doc := md.Parser().Parse(text.NewReader(srcB))
	c := &mdConv{src: srcB}
	c.blocks(doc, 0)
	return c.out
}

type mdConv struct {
	src    []byte
	out    []Block
	listID int
}

func (c *mdConv) blocks(n ast.Node, listLevel int) {
	for ch := n.FirstChild(); ch != nil; ch = ch.NextSibling() {
		c.block(ch, listLevel)
	}
}

func (c *mdConv) block(ch ast.Node, listLevel int) {
	{
		switch b := ch.(type) {
		case *ast.Heading:
			c.out = append(c.out, Block{Kind: BHeading, Level: b.Level, Inlines: c.styled(b, Inline{})})
		case *ast.Paragraph, *ast.TextBlock:
			ins, align := splitAlign(c.styled(b, Inline{}))
			plain := inlineText(ins)
			switch {
			case len(ins) == 0 && align != "" && len(c.out) > 0 && c.out[len(c.out)-1].Kind == BHeading:
				// Després d'un títol, la línia d'alineació és un paràgraf a part.
				c.out[len(c.out)-1].ParaAlign = align
			case len(ins) == 0 && align != "":
				c.out = append(c.out, Block{Kind: BPara, Inlines: PlainInlines("{align=" + align + "}")})
			case plain == "\x00PAGEBREAK\x00":
				c.out = append(c.out, Block{Kind: BPageBreak})
			case plain == "\x00TOC\x00":
				c.out = append(c.out, Block{Kind: BTOC})
			default:
				// Una imatge sola en un paràgraf → bloc d'imatge.
				if img, ok := soleImage(b, c.src); ok {
					c.out = append(c.out, Block{Kind: BImage, Image: string(img.Destination), Alt: inlineText(c.inlines(img, Inline{}))})
					return
				}
				if len(ins) > 0 {
					c.out = append(c.out, Block{Kind: BPara, Inlines: ins, ParaAlign: align})
				}
			}
		case *ast.List:
			c.list(b, listLevel)
		case *ast.Blockquote:
			start := len(c.out)
			c.blocks(b, listLevel)
			for i := start; i < len(c.out); i++ {
				if c.out[i].Kind == BPara {
					c.out[i].Kind = BQuote
				}
			}
		case *ast.FencedCodeBlock, *ast.CodeBlock:
			var sb strings.Builder
			lines := ch.Lines()
			for i := 0; i < lines.Len(); i++ {
				seg := lines.At(i)
				sb.Write(seg.Value(c.src))
			}
			text := alignMarkRe.ReplaceAllString(strings.TrimRight(sb.String(), "\n"), "{align=$1}")
			c.out = append(c.out, Block{Kind: BCode, Text: text})
		case *ast.ThematicBreak:
			c.out = append(c.out, Block{Kind: BRule})
		case *east.Table:
			c.table(b)
		case *ast.HTMLBlock:
			// HTML cru: s'ignora (no es pot representar amb fidelitat).
		default:
			c.blocks(ch, listLevel)
		}
	}
}

func (c *mdConv) list(l *ast.List, level int) {
	kind := BBullet
	if l.IsOrdered() {
		kind = BNumbered
		c.listID++
	}
	id := c.listID
	for it := l.FirstChild(); it != nil; it = it.NextSibling() {
		first := true
		for ch := it.FirstChild(); ch != nil; ch = ch.NextSibling() {
			switch b := ch.(type) {
			case *ast.List:
				c.list(b, level+1)
			case *ast.Paragraph, *ast.TextBlock:
				ins, align := splitAlign(c.styled(b, Inline{}))
				if first {
					c.out = append(c.out, Block{Kind: kind, Level: min(level, 8), Inlines: ins, ListID: id, ParaAlign: align})
					first = false
				} else {
					// Paràgraf de continuació dins l'element: va com un
					// element més del mateix nivell (prou fidel).
					c.out = append(c.out, Block{Kind: kind, Level: min(level, 8), Inlines: ins, ListID: id, ParaAlign: align})
				}
			default:
				c.block(ch, level+1)
			}
		}
	}
}

func (c *mdConv) table(t *east.Table) {
	b := Block{Kind: BTable}
	for _, a := range t.Alignments {
		switch a {
		case east.AlignLeft:
			b.Align = append(b.Align, "left")
		case east.AlignRight:
			b.Align = append(b.Align, "right")
		case east.AlignCenter:
			b.Align = append(b.Align, "center")
		default:
			b.Align = append(b.Align, "")
		}
	}
	for r := t.FirstChild(); r != nil; r = r.NextSibling() {
		var row [][]Inline
		for cell := r.FirstChild(); cell != nil; cell = cell.NextSibling() {
			ins := c.styled(cell, Inline{})
			// Una línia «{align=…}» just després de la taula hi queda com a fila.
			for i := range ins {
				ins[i].Text = alignMarkRe.ReplaceAllString(ins[i].Text, "{align=$1}")
			}
			row = append(row, ins)
		}
		b.Rows = append(b.Rows, row)
	}
	c.out = append(c.out, b)
}

// styled són els trossos amb format d'un node, amb el format ampliat ja
// llegit: goldmark el deixa com a text.
func (c *mdConv) styled(n ast.Node, st Inline) []Inline {
	return extendedFormat(c.inlines(n, st))
}

func (c *mdConv) inlines(n ast.Node, st Inline) []Inline {
	var out []Inline
	for ch := n.FirstChild(); ch != nil; ch = ch.NextSibling() {
		switch x := ch.(type) {
		case *ast.Text:
			s := string(x.Segment.Value(c.src))
			if x.SoftLineBreak() {
				s += " "
			}
			if x.HardLineBreak() {
				s += "\n"
			}
			out = appendInline(out, st, s)
		case *ast.String:
			out = appendInline(out, st, string(x.Value))
		case *ast.CodeSpan:
			s2 := st
			s2.Code = true
			var sb strings.Builder
			for g := x.FirstChild(); g != nil; g = g.NextSibling() {
				if t, ok := g.(*ast.Text); ok {
					sb.Write(t.Segment.Value(c.src))
				}
			}
			out = appendInline(out, s2, sb.String())
		case *ast.Emphasis:
			s2 := st
			if x.Level >= 2 {
				s2.Bold = true
			} else {
				s2.Italic = true
			}
			out = append(out, c.inlines(x, s2)...)
		case *east.Strikethrough:
			s2 := st
			s2.Strike = true
			out = append(out, c.inlines(x, s2)...)
		case *ast.Link:
			s2 := st
			s2.Link = string(x.Destination)
			out = append(out, c.inlines(x, s2)...)
		case *ast.AutoLink:
			s2 := st
			s2.Link = string(x.URL(c.src))
			out = appendInline(out, s2, string(x.Label(c.src)))
		case *ast.Image:
			// Imatge dins de text: queda el text alternatiu.
			out = append(out, c.inlines(x, st)...)
		case *ast.RawHTML:
			// <br> → salt de línia; la resta s'ignora.
			var sb strings.Builder
			for i := 0; i < x.Segments.Len(); i++ {
				seg := x.Segments.At(i)
				sb.Write(seg.Value(c.src))
			}
			if strings.HasPrefix(strings.ToLower(sb.String()), "<br") {
				out = appendInline(out, st, "\n")
			}
		default:
			out = append(out, c.inlines(ch, st)...)
		}
	}
	return out
}

func appendInline(out []Inline, st Inline, s string) []Inline {
	if s == "" {
		return out
	}
	if n := len(out); n > 0 {
		p := out[n-1]
		p.Text = st.Text
		if p == st {
			out[n-1].Text += s
			return out
		}
	}
	st.Text = s
	return append(out, st)
}

var (
	spanEndRe = regexp.MustCompile(`^\]\{([^{}\n]*)\}`)
	attrRe    = regexp.MustCompile(`^([a-z]+)=("[^"\n]*"|[^\s"]*)(\s+|$)`)
	hexRe     = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)
	sizeRe    = regexp.MustCompile(`^\d+(\.\d+)?$`)
)

// extendedFormat llegeix [text]{atributs} i ++subratllat++ sobre el text de
// tots els inlines alhora, perquè poden contenir negretes i cursives o
// ser-hi dins. El codi no es toca, i el que no és vàlid queda com a text.
func extendedFormat(ins []Inline) []Inline {
	var f fmtText
	for _, in := range ins {
		st := in
		st.Text = ""
		for i := 0; i < len(in.Text); i++ {
			f.text = append(f.text, in.Text[i])
			f.style = append(f.style, st)
		}
	}
	f.drop = make([]bool, len(f.text))
	f.spans()
	f.underlines()
	var out []Inline
	for i := 0; i < len(f.text); {
		if f.drop[i] {
			i++
			continue
		}
		j := i
		for j < len(f.text) && !f.drop[j] && f.style[j] == f.style[i] {
			j++
		}
		out = appendInline(out, f.style[i], string(f.text[i:j]))
		i = j
	}
	return out
}

// fmtText és el text d'un paràgraf byte a byte, amb el format de cada byte
// i si és una marca que s'ha de treure.
type fmtText struct {
	text  []byte
	style []Inline
	drop  []bool
}

// plain diu si [from, to) és text normal: ni codi ni una marca ja llegida.
func (f *fmtText) plain(from, to int) bool {
	for j := from; j < to; j++ {
		if f.style[j].Code || f.drop[j] {
			return false
		}
	}
	return true
}

// spans aplica els [text]{…}. Els interiors es tanquen abans, i els
// exteriors no en trepitgen les claus.
func (f *fmtText) spans() {
	var opens []int
	for i := 0; i < len(f.text); i++ {
		if f.style[i].Code || (f.text[i] != '[' && f.text[i] != ']') {
			continue
		}
		if f.text[i] == '[' {
			opens = append(opens, i)
			continue
		}
		if len(opens) == 0 {
			continue
		}
		o := opens[len(opens)-1]
		opens = opens[:len(opens)-1]
		m := spanEndRe.FindSubmatch(f.text[i:])
		if m == nil || !f.plain(i, i+len(m[0])) {
			continue
		}
		a, ok := spanAttrs(string(m[1]))
		if !ok {
			continue
		}
		for j := o + 1; j < i; j++ {
			fillAttrs(&f.style[j], a)
		}
		f.drop[o] = true
		for j := i; j < i+len(m[0]); j++ {
			f.drop[j] = true
		}
		i += len(m[0]) - 1
	}
}

// fillAttrs posa a s les claus de a que s no té encara.
func fillAttrs(s *Inline, a Inline) {
	if s.Color == "" {
		s.Color = a.Color
	}
	if s.Bg == "" {
		s.Bg = a.Bg
	}
	if s.Font == "" {
		s.Font = a.Font
	}
	if s.Size == 0 {
		s.Size = a.Size
	}
}

// underlines aplica els ++…++ no buits.
func (f *fmtText) underlines() {
	open := -1
	for i := 0; i+1 < len(f.text); i++ {
		if !f.plain(i, i+2) || f.text[i] != '+' || f.text[i+1] != '+' {
			continue
		}
		if open < 0 || i == open+2 {
			open = i
		} else {
			for j := open + 2; j < i; j++ {
				f.style[j].Underline = true
			}
			f.drop[open], f.drop[open+1], f.drop[i], f.drop[i+1] = true, true, true, true
			open = -1
		}
		i++
	}
}

// spanAttrs llegeix els atributs d'un [text]{…}: color, bg, font i size,
// cada un com a màxim un cop. Qualsevol altra cosa el fa invàlid.
func spanAttrs(s string) (Inline, bool) {
	var a Inline
	seen := map[string]bool{}
	s = strings.TrimSpace(s)
	if s == "" {
		return a, false
	}
	for s != "" {
		m := attrRe.FindStringSubmatch(s)
		if m == nil || seen[m[1]] || !setAttr(&a, m[1], m[2]) {
			return a, false
		}
		seen[m[1]] = true
		s = s[len(m[0]):]
	}
	return a, true
}

// setAttr posa a a la clau k amb el valor v, si tots dos són vàlids.
func setAttr(a *Inline, k, v string) bool {
	switch {
	case k == "color" && hexRe.MatchString(v):
		a.Color = v[1:]
	case k == "bg" && hexRe.MatchString(v):
		a.Bg = v[1:]
	case k == "font" && len(v) > 2 && v[0] == '"':
		a.Font = v[1 : len(v)-1]
	case k == "size" && sizeRe.MatchString(v):
		n, err := strconv.ParseFloat(v, 64)
		if err != nil || n <= 0 || n > 1638 { // 1638 pt és el màxim del Word
			return false
		}
		a.Size = n
	default:
		return false
	}
	return true
}

// splitAlign treu el marcador d'alineació del final d'un paràgraf.
func splitAlign(ins []Inline) ([]Inline, string) {
	n := len(ins)
	if n == 0 {
		return ins, ""
	}
	loc := alignMarkRe.FindStringSubmatchIndex(ins[n-1].Text)
	if loc == nil || loc[1] != len(ins[n-1].Text) {
		return ins, ""
	}
	align := ins[n-1].Text[loc[2]:loc[3]]
	ins[n-1].Text = ins[n-1].Text[:loc[0]]
	// El salt de línia abans del marcador ha deixat un espai al final.
	for len(ins) > 0 {
		last := &ins[len(ins)-1]
		last.Text = strings.TrimRight(last.Text, " ")
		if last.Text != "" {
			break
		}
		ins = ins[:len(ins)-1]
	}
	return ins, align
}

func soleImage(p ast.Node, src []byte) (*ast.Image, bool) {
	var img *ast.Image
	for ch := p.FirstChild(); ch != nil; ch = ch.NextSibling() {
		switch x := ch.(type) {
		case *ast.Image:
			if img != nil {
				return nil, false
			}
			img = x
		case *ast.Text:
			if strings.TrimSpace(string(x.Segment.Value(src))) != "" {
				return nil, false
			}
		default:
			return nil, false
		}
	}
	return img, img != nil
}

func inlineText(ins []Inline) string {
	var sb strings.Builder
	for _, i := range ins {
		sb.WriteString(i.Text)
	}
	return sb.String()
}

// PlainInlines torna inlines sense format.
func PlainInlines(s string) []Inline {
	if s == "" {
		return nil
	}
	return []Inline{{Text: s}}
}

// InlinesText torna el text pla d'uns inlines.
func InlinesText(ins []Inline) string { return inlineText(ins) }
