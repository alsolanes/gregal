package tools

// Ampliació de l'edició d'office des de l'escriptori i el TUI:
//
//   - OfficeAppend afegeix paràgrafs al final d'un .docx (abans del
//     sectPr, que és on Word espera el cos) o a l'últim quadre de text de
//     l'última diapositiva d'un .pptx. El contingut s'entén com al
//     office_create: # títol, ## subtítol, - llista, **negreta**,
//     *cursiva*, i surt amb format de debò (mides, runs), no amb els
//     marcadors impresos. Abans, per afegir text a un document existent,
//     només hi havia replace: calia trobar un text que substituir.
//   - OfficeXlsxSetRange omple un bloc de cel·les a partir d'una
//     cantonada (B2): una línia per fila, cel·les separades per |. Abans
//     una taula de 5×4 eren vint crides de set_cell (i vint reescriptures
//     del zip).
//
// Tot passa pel journal (SnapOp): /rewind ho desfà.

import (
	"bytes"
	"fmt"
	"strings"
)

// OfficeAppend afegeix content (markdown lleuger) al final del document.
func OfficeAppend(path, content string) (string, error) {
	if strings.TrimSpace(content) == "" {
		return "", fmt.Errorf("content buit")
	}
	kind := OfficeKind(path)
	if kind != "docx" && kind != "pptx" {
		return "", fmt.Errorf("append només en .docx/.pptx (per a xlsx usa set_range)")
	}
	if err := OfficeWritable(path); err != nil {
		return "", err
	}
	Active.SnapOp(path, "edit")
	z, err := officeOpen(path)
	if err != nil {
		return "", err
	}
	// El zip es tanca abans de reescriure'l: a Windows no es pot
	// substituir un fitxer obert.
	tancat := false
	tanca := func() {
		if !tancat {
			z.Close()
			tancat = true
		}
	}
	defer tanca()
	if kind == "docx" {
		raw, err := zipPart(z, "word/document.xml")
		if err != nil {
			return "", err
		}
		tanca()
		nous, n := paragrafsDocx(content)
		if n == 0 {
			return "", fmt.Errorf("res a afegir")
		}
		nb, ok := inseriuAbans(raw, []string{"<w:sectPr", "</w:body>"}, nous)
		if !ok {
			return "", fmt.Errorf("no trobo el final del cos del document")
		}
		if err := zipRewrite(path, map[string][]byte{"word/document.xml": nb}); err != nil {
			return "", err
		}
		return fmt.Sprintf("afegits %d paràgrafs al final de %s", n, path), nil
	}
	// pptx: última diapositiva, últim quadre de text.
	var ultima string
	for _, f := range z.File {
		if strings.HasPrefix(f.Name, "ppt/slides/slide") && strings.HasSuffix(f.Name, ".xml") {
			if ultima == "" || numSlide(f.Name) > numSlide(ultima) {
				ultima = f.Name
			}
		}
	}
	if ultima == "" {
		return "", fmt.Errorf("la presentació no té diapositives")
	}
	raw, err := zipPart(z, ultima)
	if err != nil {
		return "", err
	}
	tanca()
	nous, n := paragrafsPptx(content)
	if n == 0 {
		return "", fmt.Errorf("res a afegir")
	}
	i := bytes.LastIndex(raw, []byte("</p:txBody>"))
	if i < 0 {
		return "", fmt.Errorf("l'última diapositiva no té cap quadre de text")
	}
	nb := append(append(append([]byte{}, raw[:i]...), nous...), raw[i:]...)
	if err := zipRewrite(path, map[string][]byte{ultima: nb}); err != nil {
		return "", err
	}
	return fmt.Sprintf("afegits %d paràgrafs a l'última diapositiva de %s", n, path), nil
}

// numSlide extreu el número de ppt/slides/slideN.xml (0 si no en té).
func numSlide(name string) int {
	var n int
	fmt.Sscanf(strings.TrimSuffix(strings.TrimPrefix(name, "ppt/slides/slide"), ".xml"), "%d", &n)
	return n
}

// inseriuAbans posa nous davant del primer marcador que trobi (en ordre
// de preferència), buscant sempre l'última aparició.
func inseriuAbans(raw []byte, marcadors []string, nous []byte) ([]byte, bool) {
	for _, m := range marcadors {
		if i := bytes.LastIndex(raw, []byte(m)); i >= 0 {
			out := make([]byte, 0, len(raw)+len(nous))
			out = append(out, raw[:i]...)
			out = append(out, nous...)
			out = append(out, raw[i:]...)
			return out, true
		}
	}
	return nil, false
}

// paragrafsDocx converteix el markdown lleuger en <w:p> (mateix model que
// office_create). Torna també quants paràgrafs amb text.
func paragrafsDocx(content string) ([]byte, int) {
	var b strings.Builder
	n := 0
	enCodi := false
	for _, ln := range strings.Split(content, "\n") {
		if reMdFence.MatchString(strings.TrimSpace(ln)) {
			enCodi = !enCodi
			continue
		}
		if enCodi {
			b.WriteString(docxPara(ln, "20"))
			n++
			continue
		}
		rl := parseRichLine(ln)
		if rl.skip {
			continue
		}
		if len(rl.runs) == 0 {
			b.WriteString(`<w:p/>`)
			continue
		}
		b.WriteString(docxParaRich(rl, "22"))
		n++
	}
	return []byte(b.String()), n
}

// paragrafsPptx és el mateix per a DrawingML (<a:p>).
func paragrafsPptx(content string) ([]byte, int) {
	var b strings.Builder
	n := 0
	for _, ln := range strings.Split(content, "\n") {
		if reMdFence.MatchString(strings.TrimSpace(ln)) {
			continue
		}
		rl := parseRichLine(ln)
		if rl.skip || len(rl.runs) == 0 {
			continue
		}
		b.WriteString(pptxParaRich(rl, "1800"))
		n++
	}
	return []byte(b.String()), n
}

// OfficeXlsxSetRange omple un bloc de cel·les des de la cantonada anchor
// (B2): content té una línia per fila i cel·les separades per |. Les
// cel·les buides es deixen com són. Torna el rang escrit.
func OfficeXlsxSetRange(path, sheet, anchor, content string) (string, error) {
	if OfficeKind(path) != "xlsx" {
		return "", fmt.Errorf("set_range només en .xlsx")
	}
	anchor = strings.ToUpper(strings.TrimSpace(anchor))
	col, fila, ok := parseCellRef(anchor)
	if !ok {
		return "", fmt.Errorf("cell ha de ser una referència com B2 (tinc %q)", anchor)
	}
	files := strings.Split(strings.TrimRight(content, "\n"), "\n")
	if len(files) == 0 || strings.TrimSpace(content) == "" {
		return "", fmt.Errorf("content buit")
	}
	if len(files) > 500 {
		return "", fmt.Errorf("massa files (%d; màxim 500)", len(files))
	}
	escrites := 0
	maxCol := col
	for r, ln := range files {
		cells := strings.Split(ln, "|")
		for c, v := range cells {
			v = strings.TrimSpace(v)
			if v == "" {
				continue
			}
			ref := colLetters(col+c) + fmt.Sprint(fila+r)
			if _, err := OfficeXlsxSet(path, sheet, ref, v); err != nil {
				return "", fmt.Errorf("%s: %w", ref, err)
			}
			escrites++
			if col+c > maxCol {
				maxCol = col + c
			}
		}
	}
	if escrites == 0 {
		return "", fmt.Errorf("cap cel·la amb valor")
	}
	return fmt.Sprintf("escrites %d cel·les a %s:%s%d de %s", escrites, anchor, colLetters(maxCol), fila+len(files)-1, path), nil
}

// parseCellRef separa "B2" en columna (1-based) i fila.
func parseCellRef(ref string) (col, row int, ok bool) {
	i := 0
	for i < len(ref) && ref[i] >= 'A' && ref[i] <= 'Z' {
		i++
	}
	if i == 0 || i == len(ref) {
		return 0, 0, false
	}
	col = colIndex(ref[:i])
	if _, err := fmt.Sscanf(ref[i:], "%d", &row); err != nil || row <= 0 || col <= 0 {
		return 0, 0, false
	}
	return col, row, true
}
