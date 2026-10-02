package tools

// Office: lectura i edició bàsica de .docx/.xlsx/.pptx amb stdlib
// (archive/zip + encoding/xml). Sense dependències: el binari segueix
// sent únic. Tot el que escriu passa pel journal (SnapOp) perquè el
// /rewind ho pugui desfer. Límits honestos d'aquesta v1:
//   - .doc/.xls/.ppt antics (binaris): NO suportats (cal convertir
//     amb LibreOffice: soffice --headless --convert-to docx f.doc).
//   - xlsx set_cell: posa valor (número o text); conserva l'estil de
//     la cel·la però NO recalcula fórmules (Excel/LibreOffice ho fan
//     en obrir). No toca gràfics ni taules estructurades.
//   - docx/pptx replace: substitueix text del cos (+capçaleres/peus
//     en docx, +totes les diapositives en pptx). Si el text cau dins
//     d'un sol fragment conserva el format; si està partit en
//     fragments, el paràgraf queda amb format uniforme (s'avisa).
//   - No es llegeixen comentaris, control de canvis ni notes.

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

const officeMaxMB = 50

// OfficeKind diu si el fitxer és d'office modern ("xlsx", "docx",
// "pptx"), antic ("legacy") o res ("").
func OfficeKind(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".xlsx":
		return "xlsx"
	case ".docx":
		return "docx"
	case ".pptx":
		return "pptx"
	case ".xls", ".doc", ".ppt":
		return "legacy"
	}
	return ""
}

func officeOpen(path string) (*zip.ReadCloser, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if fi.Size() > officeMaxMB<<20 {
		return nil, fmt.Errorf("fitxer massa gran (%d MB, límit %d)", fi.Size()>>20, officeMaxMB)
	}
	z, err := zip.OpenReader(path)
	if err != nil {
		return nil, fmt.Errorf("no és un zip office vàlid: %v", err)
	}
	return z, nil
}

func zipPart(z *zip.ReadCloser, name string) ([]byte, error) {
	for _, f := range z.File {
		if f.Name == name {
			rc, err := f.Open()
			if err != nil {
				return nil, err
			}
			defer rc.Close()
			return io.ReadAll(rc)
		}
	}
	return nil, fmt.Errorf("part %s no trobada", name)
}

// zipRewrite torna a escriure el zip canviant les parts de mods
// (nom → contingut nou). La resta d'entrades es copien tal qual.
func zipRewrite(path string, mods map[string][]byte) error {
	z, err := officeOpen(path)
	if err != nil {
		return err
	}
	type entry struct {
		name   string
		method uint16
		data   []byte
	}
	var entries []entry
	for _, f := range z.File {
		rc, err := f.Open()
		if err != nil {
			z.Close()
			return err
		}
		data, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			z.Close()
			return err
		}
		if nb, ok := mods[f.Name]; ok {
			data = nb
		}
		entries = append(entries, entry{f.Name, f.Method, data})
	}
	z.Close()
	tmp := path + ".gregal-tmp"
	w, err := os.Create(tmp)
	if err != nil {
		return err
	}
	zw := zip.NewWriter(w)
	for _, e := range entries {
		h := &zip.FileHeader{Name: e.name, Method: e.method}
		fw, err := zw.CreateHeader(h)
		if err != nil {
			zw.Close()
			w.Close()
			os.Remove(tmp)
			return err
		}
		if _, err := fw.Write(e.data); err != nil {
			zw.Close()
			w.Close()
			os.Remove(tmp)
			return err
		}
	}
	if err := zw.Close(); err != nil {
		w.Close()
		os.Remove(tmp)
		return err
	}
	w.Close()
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		if kind := OfficeKind(path); kind != "" {
			return fmt.Errorf("no s'ha pogut desar (fitxer bloquejat? tanca'l al Word/Excel/PowerPoint): %v", err)
		}
		return err
	}
	return nil
}

// ---------- xlsx: lectura ----------

func xlsxStrings(z *zip.ReadCloser) []string {
	raw, err := zipPart(z, "xl/sharedStrings.xml")
	if err != nil {
		return nil
	}
	dec := xml.NewDecoder(bytes.NewReader(raw))
	var out []string
	var cur strings.Builder
	inSi := false
	for {
		t, err := dec.RawToken()
		if err != nil {
			break
		}
		switch t := t.(type) {
		case xml.StartElement:
			if t.Name.Local == "si" {
				cur.Reset()
				inSi = true
			}
		case xml.EndElement:
			if t.Name.Local == "si" && inSi {
				out = append(out, cur.String())
				inSi = false
			}
		case xml.CharData:
			if inSi {
				cur.Write([]byte(t))
			}
		}
	}
	return out
}

type xlsxSheet struct {
	Name string
	File string // xl/worksheets/sheetN.xml
}

func xlsxSheets(z *zip.ReadCloser) []xlsxSheet {
	wb, err := zipPart(z, "xl/workbook.xml")
	if err != nil {
		return nil
	}
	rels, _ := zipPart(z, "xl/_rels/workbook.xml.rels")
	rid2target := map[string]string{}
	if rels != nil {
		dec := xml.NewDecoder(bytes.NewReader(rels))
		for {
			t, err := dec.RawToken()
			if err != nil {
				break
			}
			if se, ok := t.(xml.StartElement); ok && se.Name.Local == "Relationship" {
				var id, tgt string
				for _, a := range se.Attr {
					if a.Name.Local == "Id" {
						id = a.Value
					}
					if a.Name.Local == "Target" {
						tgt = a.Value
					}
				}
				rid2target[id] = tgt
			}
		}
	}
	var out []xlsxSheet
	dec := xml.NewDecoder(bytes.NewReader(wb))
	for {
		t, err := dec.RawToken()
		if err != nil {
			break
		}
		if se, ok := t.(xml.StartElement); ok && se.Name.Local == "sheet" {
			var name, rid string
			for _, a := range se.Attr {
				if a.Name.Local == "name" {
					name = a.Value
				}
				if a.Name.Local == "id" {
					rid = a.Value
				}
			}
			tgt := rid2target[rid]
			if !strings.HasPrefix(tgt, "xl/") {
				tgt = "xl/" + strings.TrimPrefix(tgt, "/")
			}
			out = append(out, xlsxSheet{name, tgt})
		}
	}
	return out
}

// OfficeReadXlsx torna fulls + files (capats).
func officeReadXlsx(z *zip.ReadCloser) string {
	shared := xlsxStrings(z)
	sheets := xlsxSheets(z)
	if len(sheets) == 0 {
		return "xlsx sense fulls llegibles"
	}
	var b strings.Builder
	ns := len(sheets)
	if ns > 4 {
		sheets = sheets[:4]
	}
	for _, sh := range sheets {
		raw, err := zipPart(z, sh.File)
		if err != nil {
			continue
		}
		fmt.Fprintf(&b, "\n[FULL %s]\n", sh.Name)
		dec := xml.NewDecoder(bytes.NewReader(raw))
		nrows := 0
		var row []string
		inRow := false
		flush := func() {
			if len(row) == 0 {
				return
			}
			for len(row) > 0 && row[len(row)-1] == "" {
				row = row[:len(row)-1]
			}
			if len(row) > 20 {
				row = append(row[:20], "…")
			}
			b.WriteString(strings.Join(row, " | ") + "\n")
			row = nil
		}
		_ = flush
		var inC, inV, inIs, inT bool
		var cType, cVal, cInline, cFormula string
		var hasFormula bool
		for {
			t, err := dec.RawToken()
			if err != nil {
				break
			}
			switch t := t.(type) {
			case xml.StartElement:
				switch t.Name.Local {
				case "row":
					row = nil
					inRow = true
				case "c":
					inC = true
					cType, cVal, cInline, cFormula = "", "", "", ""
					hasFormula = false
					for _, a := range t.Attr {
						if a.Name.Local == "t" {
							cType = a.Value
						}
					}
				case "v":
					inV = true
					cVal = ""
				case "is":
					inIs = true
					cInline = ""
				case "f":
					hasFormula = true
					cFormula = ""
				case "t":
					inT = true
				}
			case xml.EndElement:
				switch t.Name.Local {
				case "row":
					if nrows < 60 {
						for len(row) > 0 && row[len(row)-1] == "" {
							row = row[:len(row)-1]
						}
						if len(row) > 0 {
							r := row
							if len(r) > 20 {
								r = append(append([]string{}, r[:20]...), "…")
							}
							b.WriteString(strings.Join(r, " | ") + "\n")
						}
					}
					nrows++
					inRow = false
				case "c":
					val := cVal
					switch cType {
					case "s":
						if i, err := strconv.Atoi(cVal); err == nil && i >= 0 && i < len(shared) {
							val = shared[i]
						}
					case "inlineStr":
						val = cInline
					case "b":
						if cVal == "1" {
							val = "TRUE"
						} else {
							val = "FALSE"
						}
					}
					if hasFormula {
						val = "=" + cFormula
					}
					row = append(row, val)
					inC = false
				case "v":
					inV = false
				case "is":
					inIs = false
				case "f":
					hasFormula = true
				case "t":
					inT = false
				}
			case xml.CharData:
				s := string(t)
				switch {
				case inV:
					cVal += s
				case inIs && inT:
					cInline += s
				case hasFormula && inC && !inV && !inIs:
					cFormula += s
				}
				_ = inRow
			}
		}
		if nrows >= 60 {
			b.WriteString("… (full retallat a 60 files)\n")
		}
	}
	if ns > 4 {
		fmt.Fprintf(&b, "… (%d fulls més)\n", ns-4)
	}
	return strings.TrimSpace(b.String())
}

// ---------- xlsx: escriure cel·la ----------

var cellRefRe = regexp.MustCompile(`^\$?([A-Z]{1,3})\$?([0-9]{1,7})$`)

func colLetters(n int) string {
	s := ""
	for n > 0 {
		n--
		s = string(rune('A'+n%26)) + s
		n /= 26
	}
	return s
}

func colIndex(letters string) int {
	n := 0
	for _, r := range letters {
		n = n*26 + int(r-'A') + 1
	}
	return n
}

// OfficeXlsxSet posa un valor a una cel·la (crea fila/cel·la si cal).
// Números queden numèrics; la resta text (inlineStr, sense tocar
// sharedStrings). Conserva l'estil (s) de la cel·la. Torna resum.
// El valor passa per neteja markdown: una cel·la no interpreta res.
func OfficeXlsxSet(path, sheet, cell, value string) (string, error) {
	value = trauMarkdownPla(value)
	m := cellRefRe.FindStringSubmatch(strings.ToUpper(strings.TrimSpace(cell)))
	if m == nil {
		return "", fmt.Errorf("cel·la invàlida %q (p. ex. B2)", cell)
	}
	col, rowNum := m[1], 0
	rowNum, _ = strconv.Atoi(m[2])
	ref := col + strconv.Itoa(rowNum)
	if err := OfficeWritable(path); err != nil {
		return "", err
	}
	Active.SnapOp(path, "edit")
	z, err := officeOpen(path)
	if err != nil {
		return "", err
	}
	var sheetFile string
	for _, sh := range xlsxSheets(z) {
		if sh.Name == sheet {
			sheetFile = sh.File
			break
		}
	}
	if sheetFile == "" {
		var names []string
		for _, sh := range xlsxSheets(z) {
			names = append(names, sh.Name)
		}
		z.Close()
		return "", fmt.Errorf("full %q no existeix (hi ha: %s)", sheet, strings.Join(names, ", "))
	}
	raw, err := zipPart(z, sheetFile)
	if err != nil {
		z.Close()
		return "", err
	}
	z.Close()
	if len(raw) > 20<<20 {
		return "", fmt.Errorf("full massa gran per editar (20 MB)")
	}

	isNum := false
	if _, err := strconv.ParseFloat(strings.TrimSpace(value), 64); err == nil && strings.TrimSpace(value) != "" {
		isNum = true
	}
	val := strings.TrimSpace(value)

	// Tokens del full sencer (els fulls editables hi caben de sobra).
	dec := xml.NewDecoder(bytes.NewReader(raw))
	var toks []xml.Token
	for {
		t, err := dec.RawToken()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
		toks = append(toks, xml.CopyToken(t))
	}

	// newCellTokens construeix <c> nou conservant l'estil donat.
	newCellTokens := func(style string) []xml.Token {
		attrs := []xml.Attr{{Name: xml.Name{Local: "r"}, Value: ref}}
		if style != "" {
			attrs = append(attrs, xml.Attr{Name: xml.Name{Local: "s"}, Value: style})
		}
		if !isNum {
			attrs = append(attrs, xml.Attr{Name: xml.Name{Local: "t"}, Value: "inlineStr"})
		}
		out := []xml.Token{xml.StartElement{Name: xml.Name{Local: "c"}, Attr: attrs}}
		if isNum {
			out = append(out,
				xml.StartElement{Name: xml.Name{Local: "v"}},
				xml.CharData(val),
				xml.EndElement{Name: xml.Name{Local: "v"}})
		} else {
			out = append(out,
				xml.StartElement{Name: xml.Name{Local: "is"}},
				xml.StartElement{Name: xml.Name{Local: "t"}, Attr: []xml.Attr{{Name: xml.Name{Space: "xml", Local: "space"}, Value: "preserve"}}},
				xml.CharData(val),
				xml.EndElement{Name: xml.Name{Local: "t"}},
				xml.EndElement{Name: xml.Name{Local: "is"}})
		}
		return append(out, xml.EndElement{Name: xml.Name{Local: "c"}})
	}

	// Localitza sheetData i les files (índexs de tokens).
	sdStart, sdEnd := -1, -1
	depth := 0
	for i, t := range toks {
		switch t := t.(type) {
		case xml.StartElement:
			depth++
			if t.Name.Local == "sheetData" && sdStart < 0 {
				sdStart = i
			}
		case xml.EndElement:
			if t.Name.Local == "sheetData" && sdStart >= 0 && sdEnd < 0 {
				sdEnd = i
			}
			depth--
		}
	}
	if sdStart < 0 || sdEnd < 0 {
		return "", fmt.Errorf("sheetData no trobat al full %q", sheet)
	}
	// Files: [startIdx, endIdx] de cada <row> de primer nivell.
	type span struct{ s, e, r int }
	var rows []span
	d := 0
	rs := -1
	for i := sdStart + 1; i < sdEnd; i++ {
		switch t := toks[i].(type) {
		case xml.StartElement:
			d++
			if t.Name.Local == "row" && d == 1 {
				rs = i
			}
		case xml.EndElement:
			if t.Name.Local == "row" && d == 1 && rs >= 0 {
				r := -1
				if se, ok := toks[rs].(xml.StartElement); ok {
					for _, a := range se.Attr {
						if a.Name.Local == "r" {
							r, _ = strconv.Atoi(a.Value)
						}
					}
				}
				rows = append(rows, span{rs, i, r})
				rs = -1
			}
			d--
		}
	}

	cellStyle := ""
	replaceSpan := func(s, e int) {
		// Substitueix toks[s:e] (cel·la vella) per la nova, llegint l'estil.
		for i := s; i <= e; i++ {
			if se, ok := toks[i].(xml.StartElement); ok && se.Name.Local == "c" {
				for _, a := range se.Attr {
					if a.Name.Local == "s" {
						cellStyle = a.Value
					}
				}
				break
			}
		}
		nt := append([]xml.Token{}, toks[:s]...)
		nt = append(nt, newCellTokens(cellStyle)...)
		toks = append(nt, toks[e+1:]...)
	}

	// 1. Fila existent?
	ri := -1
	for i, r := range rows {
		if r.r == rowNum {
			ri = i
			break
		}
	}
	if ri >= 0 {
		// 2. Cel·la existent dins la fila?
		r := rows[ri]
		d2 := 0
		found := false
		for i := r.s; i <= r.e; i++ {
			switch t := toks[i].(type) {
			case xml.StartElement:
				d2++
				if t.Name.Local == "c" && d2 == 2 {
					for _, a := range t.Attr {
						if a.Name.Local == "r" && strings.ToUpper(a.Value) == ref {
							// Troba el seu EndElement aparellat.
							dd := 1
							j := i + 1
							for j <= r.e && dd > 0 {
								switch toks[j].(type) {
								case xml.StartElement:
									dd++
								case xml.EndElement:
									dd--
								}
								j++
							}
							replaceSpan(i, j-1)
							found = true
							break
						}
					}
				}
				if found {
					break
				}
			case xml.EndElement:
				d2--
			}
			if found {
				break
			}
		}
		if !found {
			// Afegeix al final de la fila.
			nt := append([]xml.Token{}, toks[:r.e]...)
			nt = append(nt, newCellTokens("")...)
			toks = append(nt, toks[r.e:]...)
		}
	} else {
		// 3. Crea la fila (ordenada per número).
		newRow := []xml.Token{
			xml.StartElement{Name: xml.Name{Local: "row"}, Attr: []xml.Attr{{Name: xml.Name{Local: "r"}, Value: strconv.Itoa(rowNum)}}},
		}
		newRow = append(newRow, newCellTokens("")...)
		newRow = append(newRow, xml.EndElement{Name: xml.Name{Local: "row"}})
		at := sdEnd // per defecte, al final de sheetData
		for _, r := range rows {
			if r.r > rowNum {
				at = r.s
				break
			}
		}
		nt := append([]xml.Token{}, toks[:at]...)
		nt = append(nt, newRow...)
		toks = append(nt, toks[at:]...)
	}

	var out bytes.Buffer
	enc := xml.NewEncoder(&out)
	for _, t := range toks {
		if err := enc.EncodeToken(t); err != nil {
			return "", err
		}
	}
	if err := enc.Flush(); err != nil {
		return "", err
	}
	if err := zipRewrite(path, map[string][]byte{sheetFile: out.Bytes()}); err != nil {
		return "", err
	}
	return fmt.Sprintf("posat %s!%s = %q", sheet, ref, value), nil
}

// ---------- docx/pptx: lectura ----------

func officeReadDocx(z *zip.ReadCloser) string {
	raw, err := zipPart(z, "word/document.xml")
	if err != nil {
		return "docx il·legible: " + err.Error()
	}
	dec := xml.NewDecoder(bytes.NewReader(raw))
	var b strings.Builder
	var para strings.Builder
	inP, inT, inTbl := false, false, false
	var style string
	npar := 0
	flush := func() {
		t := strings.TrimSpace(para.String())
		if t != "" {
			if style != "" {
				b.WriteString("[" + style + "] ")
			}
			b.WriteString(t + "\n")
			npar++
		}
		para.Reset()
		style = ""
	}
	for {
		t, err := dec.RawToken()
		if err != nil || npar >= 200 {
			break
		}
		switch t := t.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "p":
				inP = true
			case "t":
				inT = true
			case "tbl":
				inTbl = true
			case "tr":
				flush()
			case "pStyle":
				for _, a := range t.Attr {
					if a.Name.Local == "val" {
						style = a.Value
					}
				}
			case "tab", "br":
				if inP {
					para.WriteString(" ")
				}
			}
		case xml.EndElement:
			switch t.Name.Local {
			case "p":
				flush()
				inP = false
			case "t":
				inT = false
			case "tbl":
				inTbl = false
			}
		case xml.CharData:
			if inT {
				para.WriteString(string(t))
				if inTbl {
					para.WriteString(" | ")
				}
			}
		}
	}
	flush()
	if npar >= 200 {
		b.WriteString("… (retallat a 200 paràgrafs)\n")
	}
	return strings.TrimSpace(b.String())
}

func officeReadPptx(z *zip.ReadCloser) string {
	// Ordre de diapositives via presentation.xml.
	pres, err := zipPart(z, "ppt/presentation.xml")
	if err != nil {
		return "pptx il·legible: " + err.Error()
	}
	var order []string
	dec := xml.NewDecoder(bytes.NewReader(pres))
	for {
		t, err := dec.RawToken()
		if err != nil {
			break
		}
		if se, ok := t.(xml.StartElement); ok && se.Name.Local == "sldId" {
			for _, a := range se.Attr {
				if a.Name.Local == "id" {
					order = append(order, a.Value)
				}
			}
		}
	}
	// r:id → fitxer via rels.
	rels, _ := zipPart(z, "ppt/_rels/presentation.xml.rels")
	rid2t := map[string]string{}
	if rels != nil {
		dec = xml.NewDecoder(bytes.NewReader(rels))
		for {
			t, err := dec.RawToken()
			if err != nil {
				break
			}
			if se, ok := t.(xml.StartElement); ok && se.Name.Local == "Relationship" {
				var id, tgt string
				for _, a := range se.Attr {
					if a.Name.Local == "Id" {
						id = a.Value
					}
					if a.Name.Local == "Target" {
						tgt = a.Value
					}
				}
				rid2t[id] = tgt
			}
		}
	}
	// (presentation.xml usa r:id, no id net — relectura fina)
	slideFiles := map[int]string{}
	dec = xml.NewDecoder(bytes.NewReader(pres))
	idx := 0
	for {
		t, err := dec.RawToken()
		if err != nil {
			break
		}
		if se, ok := t.(xml.StartElement); ok && se.Name.Local == "sldId" {
			idx++
			for _, a := range se.Attr {
				if a.Name.Local == "id" && a.Name.Space == "http://schemas.openxmlformats.org/officeDocument/2006/relationships" || (a.Name.Local == "id" && rid2t[a.Value] != "") {
					tgt := rid2t[a.Value]
					if !strings.HasPrefix(tgt, "ppt/") {
						tgt = "ppt/" + strings.TrimPrefix(tgt, "/")
					}
					slideFiles[idx] = tgt
				}
			}
		}
	}
	var b strings.Builder
	ids := make([]int, 0, len(slideFiles))
	for i := range slideFiles {
		ids = append(ids, i)
	}
	sort.Ints(ids)
	if len(ids) == 0 {
		// Fallback: totes les parts slide*.xml.
		for _, f := range z.File {
			if strings.HasPrefix(f.Name, "ppt/slides/slide") && strings.HasSuffix(f.Name, ".xml") {
				ids = append(ids, -len(ids)-1)
				slideFiles[-len(ids)] = f.Name
			}
		}
		sort.Ints(ids)
	}
	for n, i := range ids {
		raw, err := zipPart(z, slideFiles[i])
		if err != nil {
			continue
		}
		fmt.Fprintf(&b, "\n— Diapo %d —\n", n+1)
		dec := xml.NewDecoder(bytes.NewReader(raw))
		var cur strings.Builder
		inT := false
		flush := func() {
			if t := strings.TrimSpace(cur.String()); t != "" {
				b.WriteString(t + "\n")
			}
			cur.Reset()
		}
		for {
			t, err := dec.RawToken()
			if err != nil {
				break
			}
			switch t := t.(type) {
			case xml.StartElement:
				if t.Name.Local == "t" {
					inT = true
				}
			case xml.EndElement:
				switch t.Name.Local {
				case "t":
					inT = false
				case "p":
					flush()
				}
			case xml.CharData:
				if inT {
					cur.Write([]byte(t))
				}
			}
		}
		flush()
	}
	return strings.TrimSpace(b.String())
}

// OfficeRead llegeix el text/contingut d'un office modern.
func OfficeRead(path string) (string, error) {
	kind := OfficeKind(path)
	if kind == "legacy" {
		return "", fmt.Errorf("format antic (.xls/.doc/.ppt): converteix-lo amb LibreOffice (soffice --headless --convert-to xlsx %s)", path)
	}
	if kind == "" {
		return "", fmt.Errorf("no és un document office: %s", path)
	}
	z, err := officeOpen(path)
	if err != nil {
		return "", err
	}
	defer z.Close()
	var out string
	switch kind {
	case "xlsx":
		out = officeReadXlsx(z)
	case "docx":
		out = officeReadDocx(z)
	case "pptx":
		out = officeReadPptx(z)
	}
	if strings.TrimSpace(out) == "" {
		return "(document sense text llegible)", nil
	}
	return out, nil
}

// ---------- docx/pptx: replace ----------

// paraReplace reescriu parts XML substituint text a nivell de paràgraf:
// docx: <w:p> amb <w:t>; pptx: <a:p> amb <a:t>. Tot el text nou va al
// PRIMER fragment (conserva el seu format); els altres fragments del
// paràgraf es buiden però l'estructura (pPr, runs) queda intacta.
// Torna el nombre de substitucions.
func paraReplace(data []byte, pTag, tTag string, find, replace string) ([]byte, int) {
	dec := xml.NewDecoder(bytes.NewReader(data))
	var out bytes.Buffer
	enc := xml.NewEncoder(&out)
	count := 0
	var buf []xml.Token // tokens del paràgraf actual
	inPara := false
	depth := 0

	flushPara := func(end xml.EndElement) error {
		// Junta el text dels tTag d'aquest paràgraf.
		var texts []string
		var idx []int
		for i, t := range buf {
			if se, ok := t.(xml.StartElement); ok && se.Name.Local == tTag {
				// el CharData ve al següent token (o buit)
				if i+1 < len(buf) {
					if cd, ok := buf[i+1].(xml.CharData); ok {
						texts = append(texts, string(cd))
						idx = append(idx, i+1)
					}
				}
			}
		}
		joined := strings.Join(texts, "")
		if !strings.Contains(joined, find) {
			for _, t := range buf {
				if err := enc.EncodeToken(t); err != nil {
					return err
				}
			}
			return enc.EncodeToken(end)
		}
		n := strings.Count(joined, find)
		count += n
		joined = strings.ReplaceAll(joined, find, replace)
		// Reemet: tot el text nou al primer tTag, la resta buits.
		first := true
		skipData := false
		for _, t := range buf {
			switch t := t.(type) {
			case xml.StartElement:
				if t.Name.Local == tTag {
					skipData = true
				}
				if err := enc.EncodeToken(t); err != nil {
					return err
				}
			case xml.EndElement:
				if t.Name.Local == tTag {
					skipData = false
				}
				if err := enc.EncodeToken(t); err != nil {
					return err
				}
			case xml.CharData:
				if skipData {
					if first {
						first = false
						if err := enc.EncodeToken(xml.CharData(joined)); err != nil {
							return err
						}
					}
					continue // buida els altres fragments
				}
				if err := enc.EncodeToken(t); err != nil {
					return err
				}
			default:
				if err := enc.EncodeToken(t); err != nil {
					return err
				}
			}
		}
		return enc.EncodeToken(end)
	}

	for {
		t, err := dec.RawToken()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, -1
		}
		if se, ok := t.(xml.StartElement); ok && se.Name.Local == pTag && depth == 0 {
			inPara = true
			depth = 1
			// CopyToken: RawToken recicla el buffer intern; sense
			// còpia el CharData es corromp amb tokens posteriors.
			buf = []xml.Token{xml.CopyToken(t)}
			continue
		}
		if inPara {
			if _, ok := t.(xml.StartElement); ok {
				depth++
			}
			if ee, ok := t.(xml.EndElement); ok {
				depth--
				if ee.Name.Local == pTag && depth == 0 {
					inPara = false
					if err := flushPara(ee); err != nil {
						return nil, -1
					}
					buf = nil
					continue
				}
			}
			buf = append(buf, xml.CopyToken(t))
			continue
		}
		if err := enc.EncodeToken(t); err != nil {
			return nil, -1
		}
	}
	if err := enc.Flush(); err != nil {
		return nil, -1
	}
	return out.Bytes(), count
}

// OfficeReplace substitueix text en docx (cos+capçaleres+peus) o pptx
// (totes les diapositives). Torna resum amb el recompte.
func OfficeReplace(path, find, replace string) (string, error) {
	if find == "" {
		return "", fmt.Errorf("find buit")
	}
	kind := OfficeKind(path)
	if kind != "docx" && kind != "pptx" {
		return "", fmt.Errorf("replace només en .docx/.pptx (per a xlsx usa set_cell)")
	}
	if err := OfficeWritable(path); err != nil {
		return "", err
	}
	// El reemplaç hereta el format del paràgraf: ha de ser text pla
	// (res de markdown) o els marcadors queden impresos al document.
	// El find també es neteja (el document hi té text pla).
	find = trauMarkdownPla(find)
	replace = trauMarkdownPla(replace)
	if find == "" {
		return "", fmt.Errorf("find buit (després de netejar markdown)")
	}
	Active.SnapOp(path, "edit")
	z, err := officeOpen(path)
	if err != nil {
		return "", err
	}
	var parts []string
	if kind == "docx" {
		parts = append(parts, "word/document.xml")
		for _, f := range z.File {
			if (strings.HasPrefix(f.Name, "word/header") || strings.HasPrefix(f.Name, "word/footer")) && strings.HasSuffix(f.Name, ".xml") {
				parts = append(parts, f.Name)
			}
		}
	} else {
		for _, f := range z.File {
			if strings.HasPrefix(f.Name, "ppt/slides/slide") && strings.HasSuffix(f.Name, ".xml") {
				parts = append(parts, f.Name)
			}
		}
	}
	pTag, tTag := "p", "t"
	mods := map[string][]byte{}
	total := 0
	split := false
	for _, p := range parts {
		raw, err := zipPart(z, p)
		if err != nil {
			continue
		}
		nb, n := paraReplace(raw, pTag, tTag, find, replace)
		if n < 0 {
			z.Close()
			return "", fmt.Errorf("no s'ha pogut processar %s (XML complex)", p)
		}
		if n > 0 {
			// Detecta si el match estava partit (el paràgraf ha quedat uniforme).
			if !bytes.Contains(raw, []byte(xmlEscape(find))) {
				split = true
			}
			mods[p] = nb
			total += n
		}
	}
	z.Close()
	if total == 0 {
		return "", fmt.Errorf("no s'ha trobat %q al document", find)
	}
	if err := zipRewrite(path, mods); err != nil {
		return "", err
	}
	msg := fmt.Sprintf("substituït %q → %q (%d cop(s)) a %s", find, replace, total, path)
	if split {
		msg += " — nota: algun paràgraf estava partit i ha quedat amb format uniforme"
	}
	return msg, nil
}

func xmlEscape(s string) string {
	var b strings.Builder
	_ = xml.EscapeText(&b, []byte(s))
	return b.String()
}

// ---------- crear documents nous ----------

// OfficeCreate crea un .docx/.xlsx/.pptx mínim i vàlid amb títol i
// contingut inicial (text amb salts de línia; en xlsx cada línia és una
// fila i les cel·les se separen amb |). Falla si el fitxer ja existeix.
func OfficeCreate(path, title, content string) (string, error) {
	kind := OfficeKind(path)
	if kind != "docx" && kind != "xlsx" && kind != "pptx" {
		return "", fmt.Errorf("office_create només .docx/.xlsx/.pptx (tinc %q)", path)
	}
	if _, err := os.Stat(path); err == nil {
		return "", fmt.Errorf("el fitxer ja existeix: %s (fes servir office_edit)", path)
	}
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return "", err
		}
	}
	Active.SnapOp(path, "create")
	var parts map[string]string
	switch kind {
	case "docx":
		parts = newDocx(title, content)
	case "xlsx":
		parts = newXlsx(title, content)
	default:
		parts = newPptx(title, content)
	}
	f, err := os.Create(path)
	if err != nil {
		return "", err
	}
	zw := zip.NewWriter(f)
	for _, name := range sortedKeys(parts) {
		w, err := zw.Create(name)
		if err != nil {
			zw.Close()
			f.Close()
			return "", err
		}
		if _, err := w.Write([]byte(parts[name])); err != nil {
			zw.Close()
			f.Close()
			return "", err
		}
	}
	if err := zw.Close(); err != nil {
		f.Close()
		return "", err
	}
	if err := f.Close(); err != nil {
		return "", err
	}
	return fmt.Sprintf("creat %s (%s)", path, kind), nil
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func docxPara(text string, size string) string {
	if size == "" {
		size = "22"
	}
	return `<w:p><w:pPr><w:spacing w:after="120"/></w:pPr><w:r><w:rPr><w:sz w:val="` + size + `"/></w:rPr><w:t xml:space="preserve">` + xmlEscape(text) + `</w:t></w:r></w:p>`
}

// docxParaRich emet una línia markdown convertida: títol amb mida,
// llista amb • i runs amb negreta/cursiva reals (<w:b/>, <w:i/>).
func docxParaRich(rl richLine, defSize string) string {
	size := rl.size
	if size == "" {
		size = defSize
	}
	if size == "" {
		size = "22"
	}
	if len(rl.runs) == 0 {
		return `<w:p/>`
	}
	var runs strings.Builder
	if rl.bullet {
		runs.WriteString(`<w:r><w:rPr><w:sz w:val="` + size + `"/></w:rPr><w:t xml:space="preserve">• </w:t></w:r>`)
	}
	for _, r := range rl.runs {
		var pr strings.Builder
		if r.bold {
			pr.WriteString(`<w:b/>`)
		}
		if r.italic {
			pr.WriteString(`<w:i/>`)
		}
		pr.WriteString(`<w:sz w:val="` + size + `"/>`)
		runs.WriteString(`<w:r><w:rPr>` + pr.String() + `</w:rPr><w:t xml:space="preserve">` + xmlEscape(r.text) + `</w:t></w:r>`)
	}
	return `<w:p><w:pPr><w:spacing w:after="120"/></w:pPr>` + runs.String() + `</w:p>`
}

func newDocx(title, content string) map[string]string {
	var body strings.Builder
	if strings.TrimSpace(title) != "" {
		body.WriteString(docxPara(trauMarkdownPla(strings.TrimSpace(title)), "32"))
	}
	enCodi := false
	for _, ln := range strings.Split(content, "\n") {
		if reMdFence.MatchString(strings.TrimSpace(ln)) {
			enCodi = !enCodi
			continue
		}
		if enCodi {
			// Codi literal: tal qual, sense interpretar markdown.
			body.WriteString(docxPara(ln, "22"))
			continue
		}
		rl := parseRichLine(ln)
		if rl.skip {
			continue
		}
		if len(rl.runs) == 0 {
			body.WriteString(`<w:p/>`)
			continue
		}
		body.WriteString(docxParaRich(rl, "22"))
	}
	doc := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body>` +
		body.String() + `</w:body></w:document>`
	return map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8"?>` +
			`<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">` +
			`<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>` +
			`<Default Extension="xml" ContentType="application/xml"/>` +
			`<Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/></Types>`,
		"_rels/.rels": `<?xml version="1.0" encoding="UTF-8"?>` +
			`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
			`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/></Relationships>`,
		"word/document.xml": doc,
	}
}

func colName(i int) string {
	s := ""
	i++
	for i > 0 {
		i--
		s = string(rune('A'+i%26)) + s
		i /= 26
	}
	return s
}

func newXlsx(title, content string) map[string]string {
	var rows strings.Builder
	rn := 1
	if strings.TrimSpace(title) != "" {
		rows.WriteString(`<row r="` + strconv.Itoa(rn) + `"><c r="A` + strconv.Itoa(rn) + `" t="inlineStr"><is><t xml:space="preserve">` + xmlEscape(trauMarkdownPla(strings.TrimSpace(title))) + `</t></is></c></row>`)
		rn++
	}
	for _, ln := range strings.Split(content, "\n") {
		if strings.TrimSpace(ln) == "" {
			rn++
			continue
		}
		rows.WriteString(`<row r="` + strconv.Itoa(rn) + `">`)
		for ci, cell := range strings.Split(ln, "|") {
			// Les cel·les no interpreten res: neteja markdown.
			cell = trauMarkdownPla(strings.TrimSpace(cell))
			ref := colName(ci) + strconv.Itoa(rn)
			if cell == "" {
				continue
			}
			if _, err := strconv.ParseFloat(cell, 64); err == nil {
				rows.WriteString(`<c r="` + ref + `"><v>` + xmlEscape(cell) + `</v></c>`)
			} else {
				rows.WriteString(`<c r="` + ref + `" t="inlineStr"><is><t xml:space="preserve">` + xmlEscape(cell) + `</t></is></c>`)
			}
		}
		rows.WriteString(`</row>`)
		rn++
	}
	sheet := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData>` +
		rows.String() + `</sheetData></worksheet>`
	return map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8"?>` +
			`<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">` +
			`<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>` +
			`<Default Extension="xml" ContentType="application/xml"/>` +
			`<Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/>` +
			`<Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/></Types>`,
		"_rels/.rels": `<?xml version="1.0" encoding="UTF-8"?>` +
			`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
			`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/></Relationships>`,
		"xl/workbook.xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
			`<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">` +
			`<sheets><sheet name="Full1" sheetId="1" r:id="rId1"/></sheets></workbook>`,
		"xl/_rels/workbook.xml.rels": `<?xml version="1.0" encoding="UTF-8"?>` +
			`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
			`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/></Relationships>`,
		"xl/worksheets/sheet1.xml": sheet,
	}
}

func pptxPara(text string, size string) string {
	if size == "" {
		size = "1800"
	}
	return `<a:p><a:r><a:rPr sz="` + size + `"/><a:t xml:space="preserve">` + xmlEscape(text) + `</a:t></a:r></a:p>`
}

// pptxParaRich emet una línia markdown convertida (mateix model que
// docxParaRich, amb atributs DrawingML b/i).
func pptxParaRich(rl richLine, defSize string) string {
	size := rl.size
	if size == "" {
		size = defSize
	}
	if size == "" {
		size = "1800"
	}
	if len(rl.runs) == 0 {
		return `<a:p><a:r><a:rPr sz="` + size + `"/><a:t xml:space="preserve"></a:t></a:r></a:p>`
	}
	props := func(r richRun) string {
		p := ""
		if r.bold {
			p += ` b="1"`
		}
		if r.italic {
			p += ` i="1"`
		}
		return `<a:rPr` + p + ` sz="` + size + `"/>`
	}
	var runs strings.Builder
	if rl.bullet {
		runs.WriteString(`<a:r>` + props(richRun{}) + `<a:t xml:space="preserve">• </a:t></a:r>`)
	}
	for _, r := range rl.runs {
		runs.WriteString(`<a:r>` + props(r) + `<a:t xml:space="preserve">` + xmlEscape(r.text) + `</a:t></a:r>`)
	}
	return `<a:p>` + runs.String() + `</a:p>`
}

func newPptx(title, content string) map[string]string {
	var body strings.Builder
	if strings.TrimSpace(title) != "" {
		body.WriteString(pptxPara(trauMarkdownPla(strings.TrimSpace(title)), "3200"))
	}
	enCodi := false
	for _, ln := range strings.Split(content, "\n") {
		if strings.TrimSpace(ln) == "" {
			continue
		}
		if reMdFence.MatchString(strings.TrimSpace(ln)) {
			enCodi = !enCodi
			continue
		}
		if enCodi {
			body.WriteString(pptxPara(ln, "1800"))
			continue
		}
		rl := parseRichLine(ln)
		if rl.skip || len(rl.runs) == 0 {
			continue
		}
		body.WriteString(pptxParaRich(rl, "1800"))
	}
	slide := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">` +
		`<p:cSld><p:spTree><p:sp><p:txBody><a:bodyPr/><a:lstStyle/>` +
		body.String() + `</p:txBody></p:sp></p:spTree></p:cSld></p:sld>`
	return map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8"?>` +
			`<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">` +
			`<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>` +
			`<Default Extension="xml" ContentType="application/xml"/>` +
			`<Override PartName="/ppt/presentation.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.presentation.main+xml"/>` +
			`<Override PartName="/ppt/slides/slide1.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slide+xml"/></Types>`,
		"_rels/.rels": `<?xml version="1.0" encoding="UTF-8"?>` +
			`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
			`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="ppt/presentation.xml"/></Relationships>`,
		"ppt/presentation.xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
			`<p:presentation xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">` +
			`<p:sldIdLst><p:sldId id="256" r:id="rId1"/></p:sldIdLst></p:presentation>`,
		"ppt/_rels/presentation.xml.rels": `<?xml version="1.0" encoding="UTF-8"?>` +
			`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
			`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide1.xml"/></Relationships>`,
		"ppt/slides/slide1.xml": slide,
	}
}
