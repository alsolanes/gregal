package pdfdoc

// Crear un PDF a partir de markdown, sense binaris ni dependències noves:
// les 14 fonts estàndard (Helvetica, Courier) amb WinAnsiEncoding, que
// qualsevol visor té, i les mètriques de pdfcpu per partir les línies.
//
// Cobreix el que fa servir un informe, una carta o un resum: títol,
// capçaleres, paràgrafs amb **negreta**, *cursiva* i `codi`, llistes
// (també niuades i numerades), cites, blocs de codi, taules, línies
// horitzontals i salts de pàgina (\pagebreak). Peu amb «N / M».
//
// Límit: només el joc de caràcters de Windows-1252 (català, castellà,
// francès, alemany…). La resta (emojis, alfabets no llatins) surt com «?».

import (
	"bytes"
	"compress/zlib"
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"github.com/pdfcpu/pdfcpu/pkg/font"
	"golang.org/x/text/encoding/charmap"
)

// CreateOpts configura Create.
type CreateOpts struct {
	Title     string // títol gran a la primera pàgina i a les metadades
	Author    string
	Content   string // markdown
	Page      string // a4 (per defecte) o letter
	Landscape bool
	NoNumbers bool // sense «N / M» al peu
}

type fontID int

const (
	fReg fontID = iota
	fBold
	fItal
	fMono
	fBoldItal
)

var fontNames = [...]string{"Helvetica", "Helvetica-Bold", "Helvetica-Oblique", "Courier", "Helvetica-BoldOblique"}

// word és un tros de text amb la seva font (un mot, o un espai implícit
// entre mots).
type word struct {
	text []byte // Windows-1252
	f    fontID
	glue bool // enganxat a l'anterior (canvi de font sense espai: «`codi`.»)
}

type layout struct {
	ctx            context.Context
	w, h           float64 // mida de pàgina
	ml, mr, mt, mb float64
	pages          []*bytes.Buffer
	cur            *bytes.Buffer
	y              float64
}

const lineFactor = 1.35

func widthOf(ctx context.Context, b []byte, f fontID, size float64) float64 {
	total := 0
	for _, c := range b {
		w, _ := font.CharWidth(ctx, fontNames[f], rune(c))
		total += w
	}
	return float64(total) / 1000 * size
}

func (l *layout) newPage() {
	l.cur = &bytes.Buffer{}
	l.pages = append(l.pages, l.cur)
	l.y = l.h - l.mt
}

// need comprova que hi cap una alçada; si no, pàgina nova.
func (l *layout) need(h float64) {
	if l.y-h < l.mb {
		l.newPage()
	}
}

func pdfStr(b []byte) string {
	var s strings.Builder
	s.WriteByte('(')
	for _, c := range b {
		switch c {
		case '(', ')', '\\':
			s.WriteByte('\\')
			s.WriteByte(c)
		case '\r', '\n':
			s.WriteByte(' ')
		default:
			s.WriteByte(c)
		}
	}
	s.WriteByte(')')
	return s.String()
}

func (l *layout) text(x, y float64, b []byte, f fontID, size float64) {
	fmt.Fprintf(l.cur, "BT /F%d %.2f Tf %.2f %.2f Td %s Tj ET\n", f+1, size, x, y, pdfStr(b))
}

// words escriu una línia en un sol objecte de text: els espais van dins
// les cadenes i el canvi de font no talla la línia (els lectors de PDF,
// i el nostre extractor, la llegeixen sencera).
func (l *layout) words(x, y float64, ln []word, size float64) {
	fmt.Fprintf(l.cur, "BT %.2f %.2f Td", x, y)
	cur := fontID(-1)
	for j, w := range ln {
		if w.f != cur {
			fmt.Fprintf(l.cur, " /F%d %.2f Tf", w.f+1, size)
			cur = w.f
		}
		t := w.text
		if j > 0 && !w.glue {
			t = append([]byte{' '}, t...)
		}
		fmt.Fprintf(l.cur, " %s Tj", pdfStr(t))
	}
	l.cur.WriteString(" ET\n")
}

func (l *layout) gray(g float64) { fmt.Fprintf(l.cur, "%.2f g\n", g) }

func (l *layout) line(x1, y1, x2, y2, width float64) {
	fmt.Fprintf(l.cur, "%.2f w 0.6 G %.2f %.2f m %.2f %.2f l S 0 G\n", width, x1, y1, x2, y2)
}

func (l *layout) rect(x, y, w, h, fill float64) {
	fmt.Fprintf(l.cur, "%.2f g %.2f %.2f %.2f %.2f re f 0 g\n", fill, x, y, w, h)
}

// wrap parteix els mots en línies d'amplada màxima maxW.
func wrap(ctx context.Context, words []word, size, maxW float64) [][]word {
	var lines [][]word
	var cur []word
	curW := 0.0
	for _, w := range words {
		ww := widthOf(ctx, w.text, w.f, size)
		sp := 0.0
		if len(cur) > 0 && !w.glue {
			sp = widthOf(ctx, []byte{' '}, w.f, size)
		}
		if len(cur) > 0 && curW+sp+ww > maxW {
			lines = append(lines, cur)
			cur, curW, sp = nil, 0, 0
		}
		// Un mot sol més ample que la línia (una URL): es talla a trossos.
		for len(cur) == 0 && ww > maxW && len(w.text) > 1 {
			n := len(w.text)
			for n > 1 && widthOf(ctx, w.text[:n], w.f, size) > maxW {
				n--
			}
			lines = append(lines, []word{{text: w.text[:n], f: w.f}})
			w.text = w.text[n:]
			ww = widthOf(ctx, w.text, w.f, size)
		}
		cur = append(cur, w)
		curW += sp + ww
	}
	if len(cur) > 0 {
		lines = append(lines, cur)
	}
	return lines
}

// paragraph escriu mots partits en línies a partir de x.
func (l *layout) paragraph(words []word, x float64, size float64, firstPrefix *word) {
	maxW := l.w - l.mr - x
	lh := size * lineFactor
	for i, ln := range wrap(l.ctx, words, size, maxW) {
		l.need(lh)
		l.y -= lh
		base := l.y + (lh-size)/2
		if i == 0 && firstPrefix != nil {
			pw := widthOf(l.ctx, firstPrefix.text, firstPrefix.f, size)
			l.text(x-pw-5, base, firstPrefix.text, firstPrefix.f, size)
		}
		l.words(x, base, ln, size)
	}
}

var cp1252 = charmap.Windows1252.NewEncoder()

// enc passa el text a Windows-1252; el que no hi és, «?» (emojis fora).
func enc(s string) []byte {
	var out []byte
	for _, r := range s {
		switch {
		case r == '\t':
			out = append(out, ' ')
			continue
		case r < 0x20:
			continue
		case unicode.Is(unicode.So, r) || (r >= 0x1F000 && r <= 0x1FFFF) || r == 0xFE0F || r == 0x200D:
			continue // emojis i selectors: millor res que «?»
		}
		if b, err := cp1252.Bytes([]byte(string(r))); err == nil && len(b) == 1 {
			out = append(out, b[0])
		} else {
			out = append(out, '?')
		}
	}
	return out
}

// inline converteix una línia de markdown en mots amb font: **negreta**,
// *cursiva* / _cursiva_, `codi`, [text](url) → «text (url)».
func inline(s string, base fontID) []word {
	s = linkRe.ReplaceAllString(s, "$1 ($2)")
	var words []word
	bold, ital, code := base == fBold, base == fItal, false
	var cur strings.Builder
	glue := false // el pròxim mot va enganxat (no hi ha hagut espai)
	curFont := func() fontID {
		switch {
		case code:
			return fMono
		case bold && ital:
			return fBoldItal
		case bold:
			return fBold
		case ital:
			return fItal
		}
		return fReg
	}
	flush := func() {
		if cur.Len() > 0 {
			words = append(words, word{enc(cur.String()), curFont(), glue && len(words) > 0})
			cur.Reset()
			glue = true
		}
	}
	rs := []rune(s)
	for i := 0; i < len(rs); i++ {
		r := rs[i]
		switch {
		case r == '`':
			flush()
			code = !code
		case !code && r == '*' && i+1 < len(rs) && rs[i+1] == '*':
			flush()
			bold = !bold
			i++
		case !code && (r == '*' || r == '_') && (ital || (i+1 < len(rs) && !unicode.IsSpace(rs[i+1]))) && (r == '*' || i == 0 || !unicode.IsLetter(rs[i-1])):
			flush()
			ital = !ital
		case unicode.IsSpace(r):
			flush()
			glue = false
		default:
			cur.WriteRune(r)
		}
	}
	flush()
	return words
}

// Create escriu un PDF nou a out a partir de markdown.
func Create(ctx context.Context, out string, o CreateOpts) error {
	data, err := Render(ctx, o)
	if err != nil {
		return err
	}
	return commit(ctx, out, data)
}

// Render torna els bytes del PDF.
func Render(ctx context.Context, o CreateOpts) ([]byte, error) {
	if strings.TrimSpace(o.Content) == "" && strings.TrimSpace(o.Title) == "" {
		return nil, errors.New("falta el contingut (content, en markdown)")
	}
	l := &layout{ctx: ctx, w: 595.28, h: 841.89, ml: 62, mr: 62, mt: 64, mb: 64}
	if strings.EqualFold(o.Page, "letter") {
		l.w, l.h = 612, 792
	}
	if o.Landscape {
		l.w, l.h = l.h, l.w
	}
	l.newPage()
	if t := strings.TrimSpace(o.Title); t != "" {
		l.paragraph(inline(t, fBold), l.ml, 22, nil)
		l.y -= 6
		l.line(l.ml, l.y, l.w-l.mr, l.y, 1.2)
		l.y -= 14
	}
	blocks(l, strings.ReplaceAll(o.Content, "\r\n", "\n"))
	if !o.NoNumbers && len(l.pages) > 0 {
		for i, p := range l.pages {
			l.cur = p
			s := enc(fmt.Sprintf("%d / %d", i+1, len(l.pages)))
			l.gray(0.45)
			l.text(l.w/2-widthOf(ctx, s, fReg, 9)/2, l.mb/2, s, fReg, 9)
			l.gray(0)
		}
	}
	return writePDF(l, o)
}

func isTableLine(s string) bool {
	t := strings.TrimSpace(s)
	return strings.HasPrefix(t, "|") && strings.Count(t, "|") >= 2
}

func tableCells(s string) []string {
	t := strings.TrimSpace(s)
	t = strings.TrimPrefix(t, "|")
	t = strings.TrimSuffix(t, "|")
	parts := strings.Split(t, "|")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

func isSeparatorRow(cells []string) bool {
	for _, c := range cells {
		if strings.Trim(c, ":- ") != "" || !strings.Contains(c, "-") {
			return false
		}
	}
	return len(cells) > 0
}

// blocks recorre el markdown línia a línia.
func blocks(l *layout, md string) {
	lines := strings.Split(md, "\n")
	var para []string
	flushPara := func() {
		if len(para) == 0 {
			return
		}
		l.paragraph(inline(strings.Join(para, " "), fReg), l.ml, 11, nil)
		l.y -= 6
		para = nil
	}
	for i := 0; i < len(lines); i++ {
		raw := lines[i]
		t := strings.TrimSpace(raw)
		switch {
		case t == "":
			flushPara()
		case t == `\pagebreak` || t == "<!-- pagebreak -->" || t == `\newpage`:
			flushPara()
			l.newPage()
		case strings.HasPrefix(t, "```"):
			flushPara()
			var code []string
			for i++; i < len(lines) && !strings.HasPrefix(strings.TrimSpace(lines[i]), "```"); i++ {
				code = append(code, strings.TrimRight(lines[i], " \t"))
			}
			codeBlock(l, code)
		case strings.HasPrefix(t, "#"):
			flushPara()
			level := len(t) - len(strings.TrimLeft(t, "#"))
			text := strings.TrimSpace(strings.TrimLeft(t, "#"))
			size := map[int]float64{1: 18, 2: 15}[level]
			if size == 0 {
				size = 12.5
			}
			l.need(size*lineFactor*2 + 11*lineFactor) // capçalera mai sola al peu
			l.y -= size * 0.5
			l.paragraph(inline(text, fBold), l.ml, size, nil)
			l.y -= 4
		case t == "---" || t == "***" || t == "___":
			flushPara()
			l.need(14)
			l.y -= 7
			l.line(l.ml, l.y, l.w-l.mr, l.y, 0.6)
			l.y -= 7
		case isTableLine(t):
			flushPara()
			var rows [][]string
			for ; i < len(lines) && isTableLine(lines[i]); i++ {
				c := tableCells(lines[i])
				if !isSeparatorRow(c) {
					rows = append(rows, c)
				}
			}
			i--
			table(l, rows)
		case strings.HasPrefix(t, ">"):
			flushPara()
			text := strings.TrimSpace(strings.TrimPrefix(t, ">"))
			for i+1 < len(lines) && strings.HasPrefix(strings.TrimSpace(lines[i+1]), ">") {
				i++
				text += " " + strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(lines[i]), ">"))
			}
			top := l.y
			l.paragraph(inline(text, fItal), l.ml+16, 11, nil)
			if len(l.pages) > 0 && l.y < top {
				l.line(l.ml+6, top-2, l.ml+6, l.y+2, 2)
			}
			l.y -= 6
		case listItem(raw) != nil:
			flushPara()
			it := listItem(raw)
			text := it.text
			// Línies de continuació (sagnades, sense marcador).
			for i+1 < len(lines) && strings.TrimSpace(lines[i+1]) != "" && listItem(lines[i+1]) == nil &&
				(strings.HasPrefix(lines[i+1], "  ") || strings.HasPrefix(lines[i+1], "\t")) {
				i++
				text += " " + strings.TrimSpace(lines[i])
			}
			x := l.ml + 16 + float64(it.level)*16
			l.paragraph(inline(text, fReg), x, 11, &word{text: enc(it.marker), f: fReg})
			if i+1 >= len(lines) || listItem(lines[i+1]) == nil {
				l.y -= 6
			}
		default:
			para = append(para, t)
		}
	}
	flushPara()
}

type item struct {
	level  int
	marker string
	text   string
}

func listItem(raw string) *item {
	indent := 0
	for _, r := range raw {
		if r == ' ' {
			indent++
		} else if r == '\t' {
			indent += 4
		} else {
			break
		}
	}
	t := strings.TrimSpace(raw)
	level := indent / 2
	if level > 4 {
		level = 4
	}
	for _, b := range []string{"- ", "* ", "+ "} {
		if strings.HasPrefix(t, b) {
			text := strings.TrimSpace(t[2:])
			marker := "•"
			if strings.HasPrefix(text, "[ ] ") {
				marker, text = "[  ]", text[4:]
			} else if strings.HasPrefix(text, "[x] ") || strings.HasPrefix(text, "[X] ") {
				marker, text = "[x]", text[4:]
			}
			if level%2 == 1 {
				marker = strings.Replace(marker, "•", "–", 1)
			}
			return &item{level, marker, text}
		}
	}
	n := 0
	for n < len(t) && n < 3 && t[n] >= '0' && t[n] <= '9' {
		n++
	}
	if n > 0 && n+1 < len(t) && (t[n] == '.' || t[n] == ')') && t[n+1] == ' ' {
		return &item{level, t[:n+1], strings.TrimSpace(t[n+2:])}
	}
	return nil
}

func codeBlock(l *layout, code []string) {
	const size = 9.5
	lh := size * lineFactor
	maxW := l.w - l.ml - l.mr - 16
	var lines [][]byte
	for _, c := range code {
		b := enc(c)
		for {
			n := len(b)
			for n > 0 && widthOf(l.ctx, b[:n], fMono, size) > maxW {
				n--
			}
			if n == 0 {
				n = 1
			}
			if n > len(b) {
				n = len(b)
			}
			lines = append(lines, b[:n])
			b = b[n:]
			if len(b) == 0 {
				break
			}
		}
	}
	l.y -= 2
	for _, ln := range lines {
		l.need(lh)
		l.rect(l.ml, l.y-lh, l.w-l.ml-l.mr, lh, 0.95)
		l.y -= lh
		l.text(l.ml+8, l.y+(lh-size)/2+1, ln, fMono, size)
	}
	l.y -= 8
}

func table(l *layout, rows [][]string) {
	if len(rows) == 0 {
		return
	}
	cols := 0
	for _, r := range rows {
		if len(r) > cols {
			cols = len(r)
		}
	}
	const size = 10
	lh := size * lineFactor
	total := l.w - l.ml - l.mr
	colW := total / float64(cols)
	pad := 4.0
	for ri, r := range rows {
		f := fReg
		if ri == 0 {
			f = fBold
		}
		cells := make([][][]word, cols)
		maxLines := 1
		for c := 0; c < cols; c++ {
			txt := ""
			if c < len(r) {
				txt = r[c]
			}
			cells[c] = wrap(l.ctx, inline(txt, f), size, colW-2*pad)
			if len(cells[c]) > maxLines {
				maxLines = len(cells[c])
			}
		}
		rowH := float64(maxLines)*lh + 2*pad
		l.need(rowH)
		top := l.y
		if ri == 0 {
			l.rect(l.ml, top-rowH, total, rowH, 0.92)
		}
		for c := 0; c < cols; c++ {
			x := l.ml + float64(c)*colW + pad
			y := top - pad
			for _, ln := range cells[c] {
				y -= lh
				l.words(x, y+(lh-size)/2, ln, size)
			}
		}
		l.y = top - rowH
		l.line(l.ml, l.y, l.ml+total, l.y, 0.5)
		if ri == 0 {
			l.line(l.ml, top, l.ml+total, top, 0.5)
		}
	}
	l.y -= 10
}

// writePDF munta el fitxer: catàleg, pàgines, fonts, continguts i Info.
func writePDF(l *layout, o CreateOpts) ([]byte, error) {
	var objs [][]byte
	add := func(s string) int { objs = append(objs, []byte(s)); return len(objs) }
	addStream := func(dict string, data []byte) int {
		var z bytes.Buffer
		zw := zlib.NewWriter(&z)
		zw.Write(data)
		zw.Close()
		objs = append(objs, []byte(fmt.Sprintf("<< %s /Length %d /Filter /FlateDecode >>\nstream\n%s\nendstream", dict, z.Len(), z.Bytes())))
		return len(objs)
	}
	catalog := add("") // es reescriu quan sabem l'id de Pages
	pagesID := add("")
	var fontRefs strings.Builder
	for i, n := range fontNames {
		id := add(fmt.Sprintf("<< /Type /Font /Subtype /Type1 /BaseFont /%s /Encoding /WinAnsiEncoding >>", n))
		fmt.Fprintf(&fontRefs, "/F%d %d 0 R ", i+1, id)
	}
	var kids []string
	for _, p := range l.pages {
		c := addStream("", p.Bytes())
		pg := add(fmt.Sprintf("<< /Type /Page /Parent %d 0 R /MediaBox [0 0 %.2f %.2f] /Resources << /Font << %s>> >> /Contents %d 0 R >>",
			pagesID, l.w, l.h, fontRefs.String(), c))
		kids = append(kids, fmt.Sprintf("%d 0 R", pg))
	}
	objs[catalog-1] = []byte(fmt.Sprintf("<< /Type /Catalog /Pages %d 0 R >>", pagesID))
	objs[pagesID-1] = []byte(fmt.Sprintf("<< /Type /Pages /Kids [%s] /Count %d >>", strings.Join(kids, " "), len(kids)))
	info := "<< /Producer (Gregal)"
	if t := strings.TrimSpace(o.Title); t != "" {
		info += " /Title " + utf16Str(t)
	}
	if a := strings.TrimSpace(o.Author); a != "" {
		info += " /Author " + utf16Str(a)
	}
	infoID := add(info + " >>")

	var b bytes.Buffer
	b.WriteString("%PDF-1.4\n%\xe2\xe3\xcf\xd3\n")
	offs := make([]int, len(objs))
	for i, o := range objs {
		offs[i] = b.Len()
		fmt.Fprintf(&b, "%d 0 obj\n", i+1)
		b.Write(o)
		b.WriteString("\nendobj\n")
	}
	xref := b.Len()
	fmt.Fprintf(&b, "xref\n0 %d\n0000000000 65535 f \n", len(objs)+1)
	for _, o := range offs {
		fmt.Fprintf(&b, "%010d 00000 n \n", o)
	}
	fmt.Fprintf(&b, "trailer\n<< /Size %d /Root %d 0 R /Info %d 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objs)+1, catalog, infoID, xref)
	return b.Bytes(), nil
}

// utf16Str és una cadena hexadecimal UTF-16BE amb BOM (metadades amb accents).
func utf16Str(s string) string {
	var b strings.Builder
	b.WriteString("<FEFF")
	for _, r := range s {
		if r > 0xFFFF {
			r1, r2 := 0xD800+((r-0x10000)>>10), 0xDC00+((r-0x10000)&0x3FF)
			fmt.Fprintf(&b, "%04X%04X", r1, r2)
			continue
		}
		fmt.Fprintf(&b, "%04X", r)
	}
	b.WriteString(">")
	return b.String()
}

var linkRe = regexp.MustCompile(`\[([^\]]+)\]\(([^)\s]+)\)`)
