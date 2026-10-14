package knowledge

import (
	"bytes"
	"compress/flate"
	"compress/zlib"
	"encoding/ascii85"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf16"
)

// Extractor de text de PDF mínim i sense dependències. Cobreix el que
// generen Word, LibreOffice, navegadors i la majoria d'eines: objectes
// directes i en object streams, FlateDecode/ASCIIHex/ASCII85, fonts
// simples (WinAnsi, MacRoman, Differences) i fonts compostes amb
// ToUnicode. No cobreix PDFs xifrats, LZW ni escanejats (imatges sense
// capa de text): aquests es marquen com a no suportats o sense text.

// ErrPDFEncrypted: el PDF està xifrat.
var ErrPDFEncrypted = errors.New("encrypted PDF is not supported")

type pdfRef struct{ N, G int }
type pdfName string
type pdfKw string
type pdfDict map[string]any

type pdfObj struct {
	val    any
	stream []byte // cru, sense descodificar; nil si no és un stream
}

type pdfDoc struct {
	objs map[int]*pdfObj
}

const maxPDFDepth = 32

// ExtractPDF torna una secció per pàgina amb text.
func ExtractPDF(data []byte) ([]Section, error) {
	if !bytes.HasPrefix(bytes.TrimLeft(firstN(data, 1024), " \r\n\t"), []byte("%PDF")) && !bytes.Contains(firstN(data, 1024), []byte("%PDF")) {
		return nil, errors.New("not a PDF file")
	}
	d := &pdfDoc{objs: map[int]*pdfObj{}}
	d.scan(data)
	if d.encrypted(data) {
		return nil, ErrPDFEncrypted
	}
	pages := d.pages()
	var out []Section
	for i, pg := range pages {
		text := d.pageText(pg)
		if strings.TrimSpace(text) == "" {
			continue
		}
		out = append(out, Section{Loc: fmt.Sprintf("page %d", i+1), Text: text})
	}
	if len(out) == 0 {
		if len(pages) == 0 {
			return nil, errors.New("no pages found in PDF")
		}
		return nil, errors.New("PDF has no extractable text (scanned images?)")
	}
	return out, nil
}

var objHeader = regexp.MustCompile(`(\d+)\s+(\d+)\s+obj\b`)

func (d *pdfDoc) scan(data []byte) {
	for _, m := range objHeader.FindAllSubmatchIndex(data, -1) {
		n, _ := strconv.Atoi(string(data[m[2]:m[3]]))
		l := &pdfLexer{b: data, p: m[1]}
		v, err := l.parse(0)
		if err != nil {
			continue
		}
		obj := &pdfObj{val: v}
		l.skipWS()
		if bytes.HasPrefix(data[l.p:], []byte("stream")) {
			start := l.p + len("stream")
			if start < len(data) && data[start] == '\r' {
				start++
			}
			if start < len(data) && data[start] == '\n' {
				start++
			}
			end := -1
			if dict, ok := v.(pdfDict); ok {
				if ln, ok := dict["Length"].(float64); ok && ln >= 0 && start+int(ln) <= len(data) {
					rest := bytes.TrimLeft(data[start+int(ln):min(len(data), start+int(ln)+16)], "\r\n \t")
					if bytes.HasPrefix(rest, []byte("endstream")) {
						end = start + int(ln)
					}
				}
			}
			if end < 0 {
				k := bytes.Index(data[start:], []byte("endstream"))
				if k < 0 {
					continue
				}
				end = start + k
				for end > start && (data[end-1] == '\n' || data[end-1] == '\r') {
					end--
				}
			}
			obj.stream = data[start:end]
		}
		d.objs[n] = obj // les actualitzacions incrementals van després: guanyen
	}
	// Object streams: objectes comprimits dins d'un stream.
	for _, o := range d.objs {
		dict, ok := o.val.(pdfDict)
		if !ok || o.stream == nil || dict["Type"] != pdfName("ObjStm") {
			continue
		}
		raw, err := d.decode(dict, o.stream)
		if err != nil {
			continue
		}
		count, first := toInt(d.res(dict["N"])), toInt(d.res(dict["First"]))
		if count <= 0 || first <= 0 || first > len(raw) {
			continue
		}
		hl := &pdfLexer{b: raw[:first]}
		type ent struct{ n, off int }
		var ents []ent
		for i := 0; i < count; i++ {
			a, err1 := hl.parse(0)
			b, err2 := hl.parse(0)
			if err1 != nil || err2 != nil {
				break
			}
			ents = append(ents, ent{toInt(a), toInt(b)})
		}
		for _, e := range ents {
			if _, exists := d.objs[e.n]; exists {
				continue
			}
			if first+e.off >= len(raw) {
				continue
			}
			l := &pdfLexer{b: raw, p: first + e.off}
			v, err := l.parse(0)
			if err != nil {
				continue
			}
			d.objs[e.n] = &pdfObj{val: v}
		}
	}
}

func (d *pdfDoc) encrypted(data []byte) bool {
	for _, o := range d.objs {
		if dict, ok := o.val.(pdfDict); ok && dict["Type"] == pdfName("XRef") {
			if _, ok := dict["Encrypt"]; ok {
				return true
			}
		}
	}
	if k := bytes.LastIndex(data, []byte("trailer")); k >= 0 {
		l := &pdfLexer{b: data, p: k + len("trailer")}
		if v, err := l.parse(0); err == nil {
			if dict, ok := v.(pdfDict); ok {
				if _, ok := dict["Encrypt"]; ok {
					return true
				}
			}
		}
	}
	return false
}

// res resol una referència indirecta (amb un límit contra cicles).
func (d *pdfDoc) res(v any) any {
	for i := 0; i < 8; i++ {
		r, ok := v.(pdfRef)
		if !ok {
			return v
		}
		o := d.objs[r.N]
		if o == nil {
			return nil
		}
		v = o.val
	}
	return v
}

func (d *pdfDoc) dict(v any) pdfDict {
	dd, _ := d.res(v).(pdfDict)
	return dd
}

// streamOf torna el contingut descodificat de l'stream referenciat.
func (d *pdfDoc) streamOf(v any) []byte {
	r, ok := v.(pdfRef)
	if !ok {
		return nil
	}
	o := d.objs[r.N]
	if o == nil || o.stream == nil {
		return nil
	}
	dict, _ := o.val.(pdfDict)
	out, err := d.decode(dict, o.stream)
	if err != nil {
		return nil
	}
	return out
}

func (d *pdfDoc) decode(dict pdfDict, raw []byte) ([]byte, error) {
	var filters []string
	switch f := d.res(dict["Filter"]).(type) {
	case pdfName:
		filters = []string{string(f)}
	case []any:
		for _, x := range f {
			if n, ok := d.res(x).(pdfName); ok {
				filters = append(filters, string(n))
			}
		}
	}
	data := raw
	for _, f := range filters {
		switch f {
		case "FlateDecode", "Fl":
			out, err := inflate(data)
			if err != nil && len(out) == 0 {
				return nil, err
			}
			data = out
			if parms := d.dict(dict["DecodeParms"]); parms != nil && toInt(parms["Predictor"]) >= 10 {
				data = pngUnpredict(data, max(1, toInt(parms["Columns"])))
			}
		case "ASCIIHexDecode", "AHx":
			s := strings.Map(func(r rune) rune {
				if strings.ContainsRune("0123456789abcdefABCDEF", r) {
					return r
				}
				return -1
			}, strings.SplitN(string(data), ">", 2)[0])
			if len(s)%2 == 1 {
				s += "0"
			}
			out, err := hex.DecodeString(s)
			if err != nil {
				return nil, err
			}
			data = out
		case "ASCII85Decode", "A85":
			s := bytes.TrimSpace(data)
			s = bytes.TrimPrefix(s, []byte("<~"))
			if k := bytes.Index(s, []byte("~>")); k >= 0 {
				s = s[:k]
			}
			out := make([]byte, len(s)*4/5+8)
			n, _, err := ascii85.Decode(out, s, true)
			if err != nil {
				return nil, err
			}
			data = out[:n]
		default:
			return nil, fmt.Errorf("unsupported PDF filter %s", f)
		}
	}
	return data, nil
}

func inflate(b []byte) ([]byte, error) {
	if zr, err := zlib.NewReader(bytes.NewReader(b)); err == nil {
		out, err := io.ReadAll(zr)
		if len(out) > 0 || err == nil {
			return out, err
		}
	}
	// Alguns generadors escriuen deflate sense capçalera zlib.
	out, err := io.ReadAll(flate.NewReader(bytes.NewReader(b)))
	return out, err
}

func pngUnpredict(b []byte, cols int) []byte {
	row := cols + 1
	if len(b)%row != 0 {
		return b
	}
	out := make([]byte, 0, len(b)/row*cols)
	prev := make([]byte, cols)
	for i := 0; i+row <= len(b); i += row {
		ft, line := b[i], append([]byte(nil), b[i+1:i+row]...)
		for j := range line {
			var left, up, ul byte
			if j > 0 {
				left, ul = line[j-1], prev[j-1]
			}
			up = prev[j]
			switch ft {
			case 1:
				line[j] += left
			case 2:
				line[j] += up
			case 3:
				line[j] += byte((int(left) + int(up)) / 2)
			case 4:
				p := int(left) + int(up) - int(ul)
				pa, pb, pc := abs(p-int(left)), abs(p-int(up)), abs(p-int(ul))
				switch {
				case pa <= pb && pa <= pc:
					line[j] += left
				case pb <= pc:
					line[j] += up
				default:
					line[j] += ul
				}
			}
		}
		out = append(out, line...)
		prev = line
	}
	return out
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func toInt(v any) int {
	switch x := v.(type) {
	case float64:
		return int(x)
	case int:
		return x
	}
	return 0
}

func toFloat(v any) float64 {
	if x, ok := v.(float64); ok {
		return x
	}
	return 0
}

// ---- pàgines ----

type pdfPage struct {
	dict pdfDict
	res  pdfDict
}

func (d *pdfDoc) pages() []pdfPage {
	var root pdfDict
	for _, o := range d.objs {
		if dd, ok := o.val.(pdfDict); ok && dd["Type"] == pdfName("Catalog") {
			root = dd
			break
		}
	}
	var out []pdfPage
	seen := map[int]bool{}
	var walk func(v any, inherited pdfDict, depth int)
	walk = func(v any, inherited pdfDict, depth int) {
		if depth > maxPDFDepth {
			return
		}
		if r, ok := v.(pdfRef); ok {
			if seen[r.N] {
				return
			}
			seen[r.N] = true
		}
		node := d.dict(v)
		if node == nil {
			return
		}
		res := inherited
		if r := d.dict(node["Resources"]); r != nil {
			res = r
		}
		switch node["Type"] {
		case pdfName("Pages"):
			kids, _ := d.res(node["Kids"]).([]any)
			for _, k := range kids {
				walk(k, res, depth+1)
			}
		default:
			if _, ok := node["Kids"]; ok {
				kids, _ := d.res(node["Kids"]).([]any)
				for _, k := range kids {
					walk(k, res, depth+1)
				}
				return
			}
			out = append(out, pdfPage{dict: node, res: res})
		}
	}
	if root != nil {
		walk(root["Pages"], nil, 0)
	}
	if len(out) == 0 {
		// Sense catàleg llegible: totes les /Type /Page per número d'objecte.
		var nums []int
		for n, o := range d.objs {
			if dd, ok := o.val.(pdfDict); ok && dd["Type"] == pdfName("Page") {
				nums = append(nums, n)
			}
		}
		sort.Ints(nums)
		for _, n := range nums {
			dd := d.objs[n].val.(pdfDict)
			out = append(out, pdfPage{dict: dd, res: d.dict(dd["Resources"])})
		}
	}
	return out
}

func (d *pdfDoc) pageText(pg pdfPage) string {
	var content []byte
	switch c := pg.dict["Contents"].(type) {
	case pdfRef:
		if arr, ok := d.res(c).([]any); ok {
			for _, x := range arr {
				content = append(append(content, d.streamOf(x)...), '\n')
			}
		} else {
			content = d.streamOf(c)
		}
	case []any:
		for _, x := range c {
			content = append(append(content, d.streamOf(x)...), '\n')
		}
	}
	if len(content) == 0 {
		return ""
	}
	tw := &textWriter{}
	d.runContent(content, pg.res, tw, 0)
	return tw.String()
}

// ---- fonts ----

type pdfFont struct {
	cmap    map[uint32]string
	codeLen int // bytes per codi amb cmap (1 o 2)
	twoByte bool
	enc     *[256]rune
}

func (d *pdfDoc) fontsOf(res pdfDict, cache map[string]*pdfFont) map[string]*pdfFont {
	out := map[string]*pdfFont{}
	fonts := d.dict(res["Font"])
	for name, ref := range fonts {
		key := fmt.Sprint(ref)
		if f, ok := cache[key]; ok {
			out[name] = f
			continue
		}
		f := d.loadFont(d.dict(ref))
		cache[key] = f
		out[name] = f
	}
	return out
}

func (d *pdfDoc) loadFont(fd pdfDict) *pdfFont {
	f := &pdfFont{codeLen: 1}
	if fd == nil {
		f.enc = &winAnsi
		return f
	}
	if fd["Subtype"] == pdfName("Type0") {
		f.twoByte, f.codeLen = true, 2
	}
	if tu := fd["ToUnicode"]; tu != nil {
		if raw := d.streamOf(tu); raw != nil {
			f.cmap, f.codeLen = parseCMap(raw, f.codeLen)
		}
	}
	if !f.twoByte {
		enc := winAnsi
		switch e := d.res(fd["Encoding"]).(type) {
		case pdfName:
			if e == "MacRomanEncoding" {
				enc = macRoman
			}
		case pdfDict:
			if b, ok := d.res(e["BaseEncoding"]).(pdfName); ok && b == "MacRomanEncoding" {
				enc = macRoman
			}
			if diffs, ok := d.res(e["Differences"]).([]any); ok {
				code := 0
				for _, x := range diffs {
					switch v := d.res(x).(type) {
					case float64:
						code = int(v)
					case pdfName:
						if code >= 0 && code < 256 {
							if r := glyphRune(string(v)); r != 0 {
								enc[code] = r
							}
						}
						code++
					}
				}
			}
		}
		f.enc = &enc
	}
	return f
}

var (
	cmapBfChar  = regexp.MustCompile(`(?s)beginbfchar(.*?)endbfchar`)
	cmapBfRange = regexp.MustCompile(`(?s)beginbfrange(.*?)endbfrange`)
	cmapSpace   = regexp.MustCompile(`(?s)begincodespacerange\s*<([0-9A-Fa-f]+)>`)
	cmapHex     = regexp.MustCompile(`<([0-9A-Fa-f]*)>|\[|\]`)
)

// parseCMap llegeix un ToUnicode: bfchar i bfrange.
func parseCMap(raw []byte, defLen int) (map[uint32]string, int) {
	m := map[uint32]string{}
	codeLen := defLen
	if s := cmapSpace.FindSubmatch(raw); s != nil {
		codeLen = max(1, len(s[1])/2)
	}
	for _, blk := range cmapBfChar.FindAllSubmatch(raw, -1) {
		toks := cmapHex.FindAllSubmatch(blk[1], -1)
		for i := 0; i+1 < len(toks); i += 2 {
			src, _ := hex.DecodeString(string(toks[i][1]))
			m[beUint(src)] = utf16Hex(string(toks[i+1][1]))
		}
	}
	for _, blk := range cmapBfRange.FindAllSubmatch(raw, -1) {
		toks := cmapHex.FindAllSubmatch(blk[1], -1)
		for i := 0; i+2 < len(toks); {
			lo, _ := hex.DecodeString(string(toks[i][1]))
			hi, _ := hex.DecodeString(string(toks[i+1][1]))
			a, b := beUint(lo), beUint(hi)
			if b < a || b-a > 65535 {
				b = a
			}
			if string(toks[i+2][0]) == "[" {
				j := i + 3
				for c := a; j < len(toks) && string(toks[j][0]) != "]"; j++ {
					m[c] = utf16Hex(string(toks[j][1]))
					c++
				}
				i = j + 1
				continue
			}
			dst := []rune(utf16Hex(string(toks[i+2][1])))
			for c := a; c <= b; c++ {
				if len(dst) > 0 {
					d := append([]rune(nil), dst...)
					d[len(d)-1] += rune(c - a)
					m[c] = string(d)
				}
			}
			i += 3
		}
	}
	return m, codeLen
}

func beUint(b []byte) uint32 {
	var v uint32
	for _, x := range b {
		v = v<<8 | uint32(x)
	}
	return v
}

func utf16Hex(h string) string {
	b, err := hex.DecodeString(h)
	if err != nil || len(b) == 0 {
		return ""
	}
	if len(b)%2 == 1 {
		return string(rune(b[0]))
	}
	u := make([]uint16, len(b)/2)
	for i := range u {
		u[i] = uint16(b[2*i])<<8 | uint16(b[2*i+1])
	}
	return string(utf16.Decode(u))
}

func (f *pdfFont) decode(b []byte) string {
	if f == nil {
		f = &pdfFont{codeLen: 1, enc: &winAnsi}
	}
	var sb strings.Builder
	if f.cmap != nil {
		step := max(1, f.codeLen)
		for i := 0; i+step <= len(b); i += step {
			code := beUint(b[i : i+step])
			if s, ok := f.cmap[code]; ok {
				sb.WriteString(s)
			} else if !f.twoByte && f.enc != nil {
				sb.WriteRune(f.enc[b[i]])
			}
		}
		return sb.String()
	}
	if f.twoByte {
		// Font composta sense ToUnicode: els codis són glifs, no text.
		return ""
	}
	enc := f.enc
	if enc == nil {
		enc = &winAnsi
	}
	for _, c := range b {
		if r := enc[c]; r != 0 {
			sb.WriteRune(r)
		}
	}
	return sb.String()
}

// ---- contingut de pàgina ----

type textWriter struct {
	b     strings.Builder
	lastY float64
	hasY  bool
}

func (t *textWriter) write(s string) {
	if s == "" {
		return
	}
	t.b.WriteString(s)
}

func (t *textWriter) space() {
	s := t.b.String()
	if len(s) > 0 && !strings.HasSuffix(s, " ") && !strings.HasSuffix(s, "\n") {
		t.b.WriteByte(' ')
	}
}

func (t *textWriter) newline() {
	s := t.b.String()
	if len(s) > 0 && !strings.HasSuffix(s, "\n") {
		t.b.WriteByte('\n')
	}
}

func (t *textWriter) String() string {
	var out []string
	blank := 0
	for _, ln := range strings.Split(t.b.String(), "\n") {
		ln = strings.Join(strings.Fields(ln), " ")
		if ln == "" {
			blank++
			if blank > 1 {
				continue
			}
		} else {
			blank = 0
		}
		out = append(out, ln)
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}

func (d *pdfDoc) runContent(content []byte, res pdfDict, tw *textWriter, depth int) {
	if depth > 4 {
		return
	}
	fonts := d.fontsOf(res, map[string]*pdfFont{})
	var font *pdfFont
	l := &pdfLexer{b: content}
	var ops []any
	for {
		v, err := l.parse(0)
		if err != nil {
			break
		}
		kw, ok := v.(pdfKw)
		if !ok {
			ops = append(ops, v)
			if len(ops) > 64 {
				ops = ops[len(ops)-64:]
			}
			continue
		}
		switch kw {
		case "BI":
			// Imatge en línia: salta fins a EI.
			if k := bytes.Index(content[l.p:], []byte("ID")); k >= 0 {
				l.p += k + 2
				for l.p < len(content) {
					k := bytes.Index(content[l.p:], []byte("EI"))
					if k < 0 {
						l.p = len(content)
						break
					}
					at := l.p + k
					l.p = at + 2
					if at > 0 && isWS(content[at-1]) && (at+2 >= len(content) || isWS(content[at+2]) || isDelim(content[at+2])) {
						break
					}
				}
			}
		case "Tf":
			if len(ops) >= 2 {
				if n, ok := ops[len(ops)-2].(pdfName); ok {
					font = fonts[string(n)]
				}
			}
		case "Tj":
			if len(ops) > 0 {
				if s, ok := ops[len(ops)-1].([]byte); ok {
					tw.write(font.decode(s))
				}
			}
		case "'", "\"":
			tw.newline()
			if len(ops) > 0 {
				if s, ok := ops[len(ops)-1].([]byte); ok {
					tw.write(font.decode(s))
				}
			}
		case "TJ":
			if len(ops) > 0 {
				if arr, ok := ops[len(ops)-1].([]any); ok {
					for _, x := range arr {
						switch v := x.(type) {
						case []byte:
							tw.write(font.decode(v))
						case float64:
							if v < -180 {
								tw.space()
							}
						}
					}
				}
			}
		case "Td", "TD":
			if len(ops) >= 2 {
				if toFloat(ops[len(ops)-1]) != 0 {
					tw.newline()
				} else if toFloat(ops[len(ops)-2]) != 0 {
					tw.space()
				}
			}
		case "Tm":
			if len(ops) >= 6 {
				y := toFloat(ops[len(ops)-1])
				if tw.hasY && abs(int(y-tw.lastY)) >= 1 {
					tw.newline()
				} else {
					tw.space()
				}
				tw.lastY, tw.hasY = y, true
			}
		case "T*":
			tw.newline()
		case "ET":
			tw.space()
		case "Do":
			if len(ops) > 0 {
				if n, ok := ops[len(ops)-1].(pdfName); ok {
					xo := d.dict(res["XObject"])
					if ref, ok := xo[string(n)].(pdfRef); ok {
						if o := d.objs[ref.N]; o != nil && o.stream != nil {
							if od, ok := o.val.(pdfDict); ok && od["Subtype"] == pdfName("Form") {
								sub := res
								if r := d.dict(od["Resources"]); r != nil {
									sub = r
								}
								if raw, err := d.decode(od, o.stream); err == nil {
									d.runContent(raw, sub, tw, depth+1)
								}
							}
						}
					}
				}
			}
		}
		ops = ops[:0]
	}
}

// ---- lèxic ----

type pdfLexer struct {
	b []byte
	p int
}

func isWS(c byte) bool {
	return c == ' ' || c == '\n' || c == '\r' || c == '\t' || c == '\f' || c == 0
}

func isDelim(c byte) bool {
	return strings.IndexByte("()<>[]{}/%", c) >= 0
}

func (l *pdfLexer) skipWS() {
	for l.p < len(l.b) {
		c := l.b[l.p]
		if isWS(c) {
			l.p++
			continue
		}
		if c == '%' {
			for l.p < len(l.b) && l.b[l.p] != '\n' && l.b[l.p] != '\r' {
				l.p++
			}
			continue
		}
		break
	}
}

var errPDFEnd = errors.New("end of data")

// parse llegeix un valor: diccionari, array, nom, cadena (com a []byte),
// número (float64), referència, booleà, null o paraula clau (pdfKw).
func (l *pdfLexer) parse(depth int) (any, error) {
	if depth > maxPDFDepth {
		return nil, errors.New("PDF nesting too deep")
	}
	l.skipWS()
	if l.p >= len(l.b) {
		return nil, errPDFEnd
	}
	c := l.b[l.p]
	switch {
	case c == '<' && l.p+1 < len(l.b) && l.b[l.p+1] == '<':
		l.p += 2
		d := pdfDict{}
		for {
			l.skipWS()
			if l.p >= len(l.b) {
				return d, nil
			}
			if l.b[l.p] == '>' && l.p+1 < len(l.b) && l.b[l.p+1] == '>' {
				l.p += 2
				return d, nil
			}
			k, err := l.parse(depth + 1)
			if err != nil {
				return d, nil
			}
			name, ok := k.(pdfName)
			if !ok {
				continue
			}
			v, err := l.parse(depth + 1)
			if err != nil {
				return d, nil
			}
			d[string(name)] = v
		}
	case c == '<':
		l.p++
		start := l.p
		for l.p < len(l.b) && l.b[l.p] != '>' {
			l.p++
		}
		h := strings.Map(func(r rune) rune {
			if strings.ContainsRune("0123456789abcdefABCDEF", r) {
				return r
			}
			return -1
		}, string(l.b[start:l.p]))
		l.p++
		if len(h)%2 == 1 {
			h += "0"
		}
		out, _ := hex.DecodeString(h)
		return out, nil
	case c == '(':
		return l.literal(), nil
	case c == '[':
		l.p++
		var arr []any
		for {
			l.skipWS()
			if l.p >= len(l.b) {
				return arr, nil
			}
			if l.b[l.p] == ']' {
				l.p++
				return arr, nil
			}
			v, err := l.parse(depth + 1)
			if err != nil {
				return arr, nil
			}
			arr = append(arr, v)
		}
	case c == '/':
		l.p++
		start := l.p
		for l.p < len(l.b) && !isWS(l.b[l.p]) && !isDelim(l.b[l.p]) {
			l.p++
		}
		name := string(l.b[start:l.p])
		if strings.Contains(name, "#") {
			var sb strings.Builder
			for i := 0; i < len(name); i++ {
				if name[i] == '#' && i+2 < len(name) {
					if v, err := strconv.ParseUint(name[i+1:i+3], 16, 8); err == nil {
						sb.WriteByte(byte(v))
						i += 2
						continue
					}
				}
				sb.WriteByte(name[i])
			}
			name = sb.String()
		}
		return pdfName(name), nil
	case c == '+' || c == '-' || c == '.' || (c >= '0' && c <= '9'):
		start := l.p
		l.p++
		for l.p < len(l.b) && (l.b[l.p] == '.' || (l.b[l.p] >= '0' && l.b[l.p] <= '9')) {
			l.p++
		}
		tok := string(l.b[start:l.p])
		f, err := strconv.ParseFloat(tok, 64)
		if err != nil {
			f = 0
		}
		// Referència «N G R»?
		if !strings.ContainsAny(tok, ".+-") {
			save := l.p
			l.skipWS()
			gs := l.p
			for l.p < len(l.b) && l.b[l.p] >= '0' && l.b[l.p] <= '9' {
				l.p++
			}
			if l.p > gs {
				g, _ := strconv.Atoi(string(l.b[gs:l.p]))
				l.skipWS()
				if l.p < len(l.b) && l.b[l.p] == 'R' && (l.p+1 >= len(l.b) || isWS(l.b[l.p+1]) || isDelim(l.b[l.p+1])) {
					l.p++
					return pdfRef{N: int(f), G: g}, nil
				}
			}
			l.p = save
		}
		return f, nil
	case c == ')' || c == '>' || c == ']' || c == '{' || c == '}':
		l.p++
		return pdfKw(string(c)), nil
	}
	start := l.p
	for l.p < len(l.b) && !isWS(l.b[l.p]) && !isDelim(l.b[l.p]) {
		l.p++
	}
	if l.p == start {
		l.p++
	}
	kw := string(l.b[start:l.p])
	switch kw {
	case "true":
		return true, nil
	case "false":
		return false, nil
	case "null":
		return nil, nil
	}
	return pdfKw(kw), nil
}

func (l *pdfLexer) literal() []byte {
	l.p++ // (
	var out []byte
	depth := 1
	for l.p < len(l.b) {
		c := l.b[l.p]
		l.p++
		switch c {
		case '(':
			depth++
			out = append(out, c)
		case ')':
			depth--
			if depth == 0 {
				return out
			}
			out = append(out, c)
		case '\\':
			if l.p >= len(l.b) {
				return out
			}
			e := l.b[l.p]
			l.p++
			switch e {
			case 'n':
				out = append(out, '\n')
			case 'r':
				out = append(out, '\r')
			case 't':
				out = append(out, '\t')
			case 'b':
				out = append(out, '\b')
			case 'f':
				out = append(out, '\f')
			case '\r':
				if l.p < len(l.b) && l.b[l.p] == '\n' {
					l.p++
				}
			case '\n':
			default:
				if e >= '0' && e <= '7' {
					v := int(e - '0')
					for k := 0; k < 2 && l.p < len(l.b) && l.b[l.p] >= '0' && l.b[l.p] <= '7'; k++ {
						v = v*8 + int(l.b[l.p]-'0')
						l.p++
					}
					out = append(out, byte(v))
				} else {
					out = append(out, e)
				}
			}
		default:
			out = append(out, c)
		}
	}
	return out
}

// ---- codificacions ----

var winAnsi = func() [256]rune {
	var t [256]rune
	for i := 32; i < 127; i++ {
		t[i] = rune(i)
	}
	for i := 160; i < 256; i++ {
		t[i] = rune(i)
	}
	t['\t'], t['\n'], t['\r'] = ' ', '\n', '\n'
	extra := map[int]rune{0x80: '€', 0x82: '‚', 0x83: 'ƒ', 0x84: '„', 0x85: '…', 0x86: '†', 0x87: '‡', 0x88: 'ˆ', 0x89: '‰', 0x8A: 'Š', 0x8B: '‹', 0x8C: 'Œ', 0x8E: 'Ž',
		0x91: '‘', 0x92: '’', 0x93: '“', 0x94: '”', 0x95: '•', 0x96: '–', 0x97: '—', 0x98: '˜', 0x99: '™', 0x9A: 'š', 0x9B: '›', 0x9C: 'œ', 0x9E: 'ž', 0x9F: 'Ÿ', 0xAD: '-'}
	for k, v := range extra {
		t[k] = v
	}
	return t
}()

var macRoman = func() [256]rune {
	t := winAnsi
	hi := []rune("ÄÅÇÉÑÖÜáàâäãåçéèêëíìîïñóòôöõúùûü†°¢£§•¶ß®©™´¨≠ÆØ∞±≤≥¥µ∂∑∏π∫ªºΩæø¿¡¬√ƒ≈∆«»… ÀÃÕŒœ–—“”‘’÷◊ÿŸ⁄€‹›ﬁﬂ‡·‚„‰ÂÊÁËÈÍÎÏÌÓÔÒÚÛÙıˆ˜¯˘˙˚¸˝˛ˇ")
	for i, r := range hi {
		if 128+i < 256 {
			t[128+i] = r
		}
	}
	return t
}()

var glyphNames = map[string]rune{
	"space": ' ', "exclam": '!', "quotedbl": '"', "numbersign": '#', "dollar": '$', "percent": '%', "ampersand": '&',
	"quotesingle": '\'', "quoteright": '’', "quoteleft": '‘', "parenleft": '(', "parenright": ')', "asterisk": '*', "plus": '+',
	"comma": ',', "hyphen": '-', "minus": '-', "period": '.', "slash": '/', "colon": ':', "semicolon": ';', "less": '<',
	"equal": '=', "greater": '>', "question": '?', "at": '@', "bracketleft": '[', "backslash": '\\', "bracketright": ']',
	"underscore": '_', "braceleft": '{', "bar": '|', "braceright": '}', "asciitilde": '~', "bullet": '•', "endash": '–',
	"emdash": '—', "quotedblleft": '“', "quotedblright": '”', "ellipsis": '…', "periodcentered": '·', "degree": '°',
	"zero": '0', "one": '1', "two": '2', "three": '3', "four": '4', "five": '5', "six": '6', "seven": '7', "eight": '8', "nine": '9',
	"fi": 'ﬁ', "fl": 'ﬂ', "Euro": '€', "copyright": '©', "registered": '®', "trademark": '™', "section": '§', "paragraph": '¶',
	"agrave": 'à', "aacute": 'á', "acircumflex": 'â', "adieresis": 'ä', "atilde": 'ã', "aring": 'å', "ccedilla": 'ç',
	"egrave": 'è', "eacute": 'é', "ecircumflex": 'ê', "edieresis": 'ë', "igrave": 'ì', "iacute": 'í', "icircumflex": 'î',
	"idieresis": 'ï', "ntilde": 'ñ', "ograve": 'ò', "oacute": 'ó', "ocircumflex": 'ô', "odieresis": 'ö', "otilde": 'õ',
	"ugrave": 'ù', "uacute": 'ú', "ucircumflex": 'û', "udieresis": 'ü', "Agrave": 'À', "Aacute": 'Á', "Ccedilla": 'Ç',
	"Egrave": 'È', "Eacute": 'É', "Iacute": 'Í', "Idieresis": 'Ï', "Ntilde": 'Ñ', "Ograve": 'Ò', "Oacute": 'Ó', "Uacute": 'Ú',
	"Udieresis": 'Ü', "germandbls": 'ß', "exclamdown": '¡', "questiondown": '¿', "guillemotleft": '«', "guillemotright": '»',
}

func glyphRune(name string) rune {
	if r, ok := glyphNames[name]; ok {
		return r
	}
	if len(name) == 1 {
		return rune(name[0])
	}
	if strings.HasPrefix(name, "uni") && len(name) >= 7 {
		if v, err := strconv.ParseUint(name[3:7], 16, 32); err == nil {
			return rune(v)
		}
	}
	return 0
}
