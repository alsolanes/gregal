package ooxml

import (
	"regexp"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	east "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/extension"
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
	Text    string     // codi
	Rows    [][][]Inline // taula: files → cel·les → inlines (la primera és capçalera)
	Align   []string   // alineació per columna: "", "left", "center", "right"
	Image   string     // ruta de la imatge
	Alt     string
	ListID  int // llistes numerades: cada llista nova reinicia
}

var (
	pageBreakRe = regexp.MustCompile(`(?i)^\s*(\\pagebreak|\[pagebreak\]|<!--\s*pagebreak\s*-->)\s*$`)
	tocRe       = regexp.MustCompile(`(?i)^\s*(\[toc\]|\[\[toc\]\]|\{\{toc\}\})\s*$`)
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
			c.out = append(c.out, Block{Kind: BHeading, Level: b.Level, Inlines: c.inlines(b, Inline{})})
		case *ast.Paragraph, *ast.TextBlock:
			ins := c.inlines(b, Inline{})
			plain := inlineText(ins)
			switch {
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
					c.out = append(c.out, Block{Kind: BPara, Inlines: ins})
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
			c.out = append(c.out, Block{Kind: BCode, Text: strings.TrimRight(sb.String(), "\n")})
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
				ins := c.inlines(b, Inline{})
				if first {
					c.out = append(c.out, Block{Kind: kind, Level: min(level, 8), Inlines: ins, ListID: id})
					first = false
				} else {
					// Paràgraf de continuació dins l'element: va com un
					// element més del mateix nivell (prou fidel).
					c.out = append(c.out, Block{Kind: kind, Level: min(level, 8), Inlines: ins, ListID: id})
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
			row = append(row, c.inlines(cell, Inline{}))
		}
		b.Rows = append(b.Rows, row)
	}
	c.out = append(c.out, b)
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
		if p.Bold == st.Bold && p.Italic == st.Italic && p.Code == st.Code && p.Strike == st.Strike && p.Link == st.Link {
			out[n-1].Text += s
			return out
		}
	}
	st.Text = s
	return append(out, st)
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
