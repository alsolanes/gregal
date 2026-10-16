// Package xlsxdoc crea, edita i revisa fulls de càlcul amb excelize.
//
// Regles que segueix (les mateixes que fan servir Claude i ChatGPT
// per a fulls de qualitat):
//   - fórmules, no valors calculats a mà (un valor que comença per «=»
//     és una fórmula);
//   - zero errors de fórmula (#REF!, #DIV/0!…): Recalc els detecta;
//   - colors de rol: blau = entrada, negre = fórmula, verd = enllaç a
//     un altre full, groc = hipòtesi clau;
//   - formats de nombre explícits (moneda, %, milers) i capçaleres
//     destacades amb panells fixats.
package xlsxdoc

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"gregal/internal/ooxml"

	"github.com/xuri/excelize/v2"
)

// ---------- especificació ----------

// Spec descriu un llibre nou.
type Spec struct {
	Theme  string      `json:"theme"`
	Sheets []SheetSpec `json:"sheets"`
}

// SheetSpec és un full.
type SheetSpec struct {
	Name          string            `json:"name"`
	Rows          [][]any           `json:"rows"`
	Start         string            `json:"start"`  // cel·la d'inici (A1)
	Header        *bool             `json:"header"` // primera fila = capçalera (per defecte sí)
	Widths        []float64         `json:"widths"`
	Freeze        string            `json:"freeze"` // p. ex. "A2" o "B2"
	Autofilter    bool              `json:"autofilter"`
	Table         bool              `json:"table"`
	NumberFormats map[string]string `json:"number_formats"` // rang → format
	Roles         map[string]string `json:"roles"`          // rang → input|formula|link|key|total|header|note
	Merge         []string          `json:"merge"`
	Conditional   []CondSpec        `json:"conditional"`
	Charts        []ChartSpec       `json:"charts"`
	Comments      map[string]string `json:"comments"`
	Validation    []ValidSpec       `json:"validation"`
}

// CondSpec és un format condicional.
type CondSpec struct {
	Range    string `json:"range"`
	Type     string `json:"type"`     // data_bar, color_scale, cell
	Criteria string `json:"criteria"` // >, <, >=, <=, =, !=, between
	Value    string `json:"value"`
	Value2   string `json:"value2"`
	Format   string `json:"format"` // good, bad, neutral
}

// ValidSpec és una validació de dades (llista desplegable o rang).
type ValidSpec struct {
	Range string   `json:"range"`
	List  []string `json:"list"`
	Min   *float64 `json:"min"`
	Max   *float64 `json:"max"`
}

// ChartSpec és un gràfic natiu.
type ChartSpec struct {
	Type       string        `json:"type"` // col, bar, line, pie, doughnut, area, scatter
	Title      string        `json:"title"`
	Cell       string        `json:"cell"` // on es col·loca
	Categories string        `json:"categories"`
	Series     []ChartSeries `json:"series"`
	Width      uint          `json:"width"`
	Height     uint          `json:"height"`
}

// ChartSeries referencia rangs (Full!$B$2:$B$6).
type ChartSeries struct {
	Name   string `json:"name"`
	Values string `json:"values"`
}

// ParseSpec llegeix l'especificació (o només una llista de files).
func ParseSpec(s string) (Spec, error) {
	var sp Spec
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "[") {
		var rows [][]any
		if err := json.Unmarshal([]byte(s), &rows); err != nil {
			return sp, err
		}
		sp.Sheets = []SheetSpec{{Name: "Full1", Rows: rows}}
		return sp, nil
	}
	err := json.Unmarshal([]byte(s), &sp)
	return sp, err
}

// palette segons el tema (mateixos noms que ooxml).
type palette struct {
	header, headerText, band, input, link, key, total string
	chart                                             []string
}

func paletteFor(theme string) palette {
	p := palette{header: "1F4E79", headerText: "FFFFFF", band: "EAF1F8", input: "0000FF", link: "008000", key: "FFF2CC", total: "1F2A44",
		chart: []string{"1F4E79", "2E86AB", "F18F01", "6B7280", "1F2A44"}}
	switch strings.ToLower(theme) {
	case "modern":
		p.header, p.band = "2B2D42", "F1F1F4"
		p.chart = []string{"2B2D42", "EF476F", "06D6A0", "6E6E7A", "1E1E24"}
	case "nature":
		p.header, p.band = "2D6A4F", "EEF6F0"
		p.chart = []string{"2D6A4F", "52B788", "D4A373", "667A70", "1B3A2F"}
	case "elegant":
		p.header, p.band = "6D213C", "F6F0EE"
		p.chart = []string{"6D213C", "B08D57", "3E5C76", "7A6E70", "2B1B24"}
	case "mono":
		p.header, p.band = "333333", "F2F2F2"
		p.chart = []string{"333333", "0070C0", "7F7F7F", "A5A5A5", "111111"}
	case "default":
	default:
		// Buit o un tema de branding: els colors del tema d'ooxml (el
		// branding actiu, si n'hi ha; si no, el default de sempre).
		if t := ooxml.GetTheme(theme); t.Name != "default" {
			p.header, p.band, p.total = t.Primary, t.Light, t.Dark
			p.chart = []string{t.Primary, t.Accent, t.Accent2, t.Muted, t.Dark}
		}
	}
	return p
}

// Create crea un llibre nou des de l'especificació. Si template no és
// buit, parteix d'aquesta plantilla (.xltx/.xlsx).
func Create(path, template string, sp Spec) ([]string, error) {
	var f *excelize.File
	var err error
	if template != "" {
		f, err = excelize.OpenFile(template)
		if err != nil {
			return nil, fmt.Errorf("plantilla: %v", err)
		}
	} else {
		f = excelize.NewFile()
	}
	defer f.Close()
	pal := paletteFor(sp.Theme)
	var warns []string
	first := true
	for _, sh := range sp.Sheets {
		name := sh.Name
		if name == "" {
			name = "Full" + strconv.Itoa(len(f.GetSheetList())+1)
		}
		if len([]rune(name)) > 31 {
			name = string([]rune(name)[:31])
			warns = append(warns, "nom de full retallat a 31 caràcters: "+name)
		}
		if first && template == "" {
			f.SetSheetName("Sheet1", name)
		} else if idx, _ := f.GetSheetIndex(name); idx < 0 {
			if _, err := f.NewSheet(name); err != nil {
				return nil, err
			}
		}
		first = false
		w, err := fillSheet(f, name, sh, pal)
		if err != nil {
			return nil, fmt.Errorf("full %s: %w", name, err)
		}
		fitPage(f, name)
		warns = append(warns, w...)
	}
	if idx, err := f.GetSheetIndex(f.GetSheetList()[0]); err == nil {
		f.SetActiveSheet(idx)
	}
	f.SetCalcProps(&excelize.CalcPropsOptions{FullCalcOnLoad: boolPtr(true)})
	f.SetDocProps(&excelize.DocProperties{Creator: "Gregal"})
	if err := f.SaveAs(path); err != nil {
		return nil, err
	}
	return warns, nil
}

func boolPtr(b bool) *bool { return &b }

func fillSheet(f *excelize.File, name string, sh SheetSpec, pal palette) ([]string, error) {
	var warns []string
	start := sh.Start
	if start == "" {
		start = "A1"
	}
	c0, r0, err := excelize.CellNameToCoordinates(start)
	if err != nil {
		return nil, err
	}
	ncol := 0
	for ri, row := range sh.Rows {
		if len(row) > ncol {
			ncol = len(row)
		}
		for ci, v := range row {
			cell, _ := excelize.CoordinatesToCellName(c0+ci, r0+ri)
			if err := setAny(f, name, cell, v); err != nil {
				warns = append(warns, fmt.Sprintf("%s: %v", cell, err))
			}
		}
	}
	nrow := len(sh.Rows)
	header := sh.Header == nil || *sh.Header
	lastCell, _ := excelize.CoordinatesToCellName(c0+max(ncol, 1)-1, r0+max(nrow, 1)-1)
	firstRowEnd, _ := excelize.CoordinatesToCellName(c0+max(ncol, 1)-1, r0)
	// Amplades: les donades o automàtiques segons contingut.
	for ci := 0; ci < ncol; ci++ {
		col, _ := excelize.ColumnNumberToName(c0 + ci)
		w := 0.0
		if ci < len(sh.Widths) {
			w = sh.Widths[ci]
		} else {
			for _, row := range sh.Rows {
				if ci < len(row) {
					n := float64(len([]rune(display(row[ci]))))
					if n > w {
						w = n
					}
				}
			}
			w = math.Min(math.Max(w*1.15+2, 9), 60)
		}
		f.SetColWidth(name, col, col, w)
	}
	if header && nrow > 0 && ncol > 0 {
		st, _ := f.NewStyle(&excelize.Style{
			Font:      &excelize.Font{Bold: true, Color: pal.headerText, Family: defaultFont(f), Size: 11},
			Fill:      excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{pal.header}},
			Alignment: &excelize.Alignment{Vertical: "center", WrapText: true},
			Border:    []excelize.Border{{Type: "bottom", Color: pal.header, Style: 1}},
		})
		f.SetCellStyle(name, start, firstRowEnd, st)
		f.SetRowHeight(name, r0, 22)
		if sh.Freeze == "" {
			sh.Freeze, _ = excelize.CoordinatesToCellName(c0, r0+1)
		}
	}
	if sh.Freeze != "" {
		fc, fr, err := excelize.CellNameToCoordinates(sh.Freeze)
		if err == nil {
			f.SetPanes(name, &excelize.Panes{Freeze: true, XSplit: fc - 1, YSplit: fr - 1, TopLeftCell: sh.Freeze, ActivePane: paneFor(fc, fr)})
		}
	}
	// Rols (colors segons convenció financera).
	roleKeys := sortedKeys(sh.Roles)
	for _, rng := range roleKeys {
		role := strings.ToLower(sh.Roles[rng])
		s := &excelize.Style{}
		switch role {
		case "input":
			s.Font = &excelize.Font{Color: pal.input}
		case "formula":
			s.Font = &excelize.Font{Color: "000000"}
		case "link":
			s.Font = &excelize.Font{Color: pal.link}
		case "key", "assumption":
			s.Fill = excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{pal.key}}
		case "total":
			s.Font = &excelize.Font{Bold: true, Color: pal.total}
			s.Border = []excelize.Border{{Type: "top", Color: "000000", Style: 1}, {Type: "bottom", Color: "000000", Style: 6}}
		case "header":
			s.Font = &excelize.Font{Bold: true, Color: pal.headerText}
			s.Fill = excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{pal.header}}
		case "note":
			s.Font = &excelize.Font{Italic: true, Color: "6B7280"}
		default:
			warns = append(warns, "rol desconegut: "+role)
			continue
		}
		if err := mergeStyle(f, name, rng, s); err != nil {
			warns = append(warns, fmt.Sprintf("rol %s a %s: %v", role, rng, err))
		}
	}
	for _, rng := range sortedKeys(sh.NumberFormats) {
		nf := sh.NumberFormats[rng]
		if err := mergeStyle(f, name, rng, &excelize.Style{CustomNumFmt: &nf}); err != nil {
			warns = append(warns, fmt.Sprintf("format %s: %v", rng, err))
		}
	}
	for _, m := range sh.Merge {
		a, b, ok := strings.Cut(m, ":")
		if ok {
			f.MergeCell(name, a, b)
		}
	}
	if sh.Table && nrow > 1 {
		if err := f.AddTable(name, &excelize.Table{Range: start + ":" + lastCell, Name: tableName(name), StyleName: "TableStyleMedium2", ShowRowStripes: boolPtr(true)}); err != nil {
			warns = append(warns, "taula: "+err.Error())
		}
	} else if sh.Autofilter && nrow > 1 {
		f.AutoFilter(name, start+":"+lastCell, nil)
	}
	for _, c := range sh.Conditional {
		if err := addCond(f, name, c); err != nil {
			warns = append(warns, fmt.Sprintf("format condicional %s: %v", c.Range, err))
		}
	}
	for _, v := range sh.Validation {
		dv := excelize.NewDataValidation(true)
		dv.Sqref = v.Range
		if len(v.List) > 0 {
			dv.SetDropList(v.List)
		} else if v.Min != nil && v.Max != nil {
			dv.SetRange(*v.Min, *v.Max, excelize.DataValidationTypeDecimal, excelize.DataValidationOperatorBetween)
		}
		f.AddDataValidation(name, dv)
	}
	for _, cell := range sortedKeys(sh.Comments) {
		f.AddComment(name, excelize.Comment{Cell: cell, Author: "Gregal", Paragraph: []excelize.RichTextRun{{Text: sh.Comments[cell]}}})
	}
	for i, ch := range sh.Charts {
		if err := addChart(f, name, ch, i, start, ncol, nrow, c0, r0, pal.chart); err != nil {
			warns = append(warns, fmt.Sprintf("gràfic %d: %v", i+1, err))
		}
	}
	return warns, nil
}

func paneFor(c, r int) string {
	switch {
	case c > 1 && r > 1:
		return "bottomRight"
	case c > 1:
		return "topRight"
	default:
		return "bottomLeft"
	}
}

func tableName(sheet string) string {
	var b strings.Builder
	for _, r := range sheet {
		if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' {
			b.WriteRune(r)
		}
	}
	s := b.String()
	if s == "" || (s[0] >= '0' && s[0] <= '9') {
		s = "T" + s
	}
	return "Taula_" + s
}

func sortedKeys(m map[string]string) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// expandRange admet «B2:C10», «B2», «C:C» (fins a l'última fila usada)
// i «2:2».
func expandRange(f *excelize.File, sheet, rng string) (string, string, error) {
	a, b, ok := strings.Cut(strings.ToUpper(strings.ReplaceAll(rng, "$", "")), ":")
	if !ok {
		return a, a, nil
	}
	rows, _ := f.GetRows(sheet)
	lastRow := max(len(rows), 1)
	isCol := func(s string) bool { return strings.Trim(s, "ABCDEFGHIJKLMNOPQRSTUVWXYZ") == "" }
	isRow := func(s string) bool { _, err := strconv.Atoi(s); return err == nil }
	switch {
	case isCol(a) && isCol(b):
		return a + "1", b + strconv.Itoa(lastRow), nil
	case isRow(a) && isRow(b):
		cols, _ := f.GetCols(sheet)
		last, _ := excelize.ColumnNumberToName(max(len(cols), 1))
		return "A" + a, last + b, nil
	}
	return a, b, nil
}

// mergeStyle aplica un estil cel·la a cel·la combinant-lo amb el que
// ja hi ha (per no perdre la capçalera en posar un format de nombre).
func mergeStyle(f *excelize.File, sheet, rng string, add *excelize.Style) error {
	a, b, err := expandRange(f, sheet, rng)
	if err != nil {
		return err
	}
	c1, r1, err := excelize.CellNameToCoordinates(a)
	if err != nil {
		return err
	}
	c2, r2, err := excelize.CellNameToCoordinates(b)
	if err != nil {
		return err
	}
	if (c2-c1+1)*(r2-r1+1) > 200000 {
		return fmt.Errorf("rang massa gran")
	}
	cache := map[int]int{}
	for r := r1; r <= r2; r++ {
		for c := c1; c <= c2; c++ {
			cell, _ := excelize.CoordinatesToCellName(c, r)
			cur, _ := f.GetCellStyle(sheet, cell)
			nid, ok := cache[cur]
			if !ok {
				base, err := f.GetStyle(cur)
				if err != nil || base == nil {
					base = &excelize.Style{}
				}
				combined := *base
				if add.Font != nil {
					fn := excelize.Font{}
					if base.Font != nil {
						fn = *base.Font
					}
					// Sense família, LibreOffice hi posa una serif.
					if fn.Family == "" {
						fn.Family = defaultFont(f)
					}
					if fn.Size == 0 {
						fn.Size = 11
					}
					if add.Font.Bold {
						fn.Bold = true
					}
					if add.Font.Italic {
						fn.Italic = true
					}
					if add.Font.Color != "" {
						fn.Color = add.Font.Color
					}
					combined.Font = &fn
				}
				if add.Fill.Type != "" {
					combined.Fill = add.Fill
				}
				if len(add.Border) > 0 {
					combined.Border = add.Border
				}
				if add.CustomNumFmt != nil {
					combined.CustomNumFmt = add.CustomNumFmt
					combined.NumFmt = 0
				}
				if add.Alignment != nil {
					combined.Alignment = add.Alignment
				}
				nid, err = f.NewStyle(&combined)
				if err != nil {
					return err
				}
				cache[cur] = nid
			}
			f.SetCellStyle(sheet, cell, cell, nid)
		}
	}
	return nil
}

func addCond(f *excelize.File, sheet string, c CondSpec) error {
	var fmts []excelize.ConditionalFormatOptions
	switch strings.ToLower(c.Type) {
	case "data_bar", "databar":
		fmts = []excelize.ConditionalFormatOptions{{Type: "data_bar", Criteria: "=", MinType: "min", MaxType: "max", BarColor: "#2E86AB"}}
	case "color_scale", "3_color_scale":
		fmts = []excelize.ConditionalFormatOptions{{Type: "3_color_scale", Criteria: "=", MinType: "min", MidType: "percentile", MaxType: "max", MinColor: "#F8696B", MidColor: "#FFEB84", MaxColor: "#63BE7B"}}
	default:
		fill, font := "#C6EFCE", "#006100"
		switch strings.ToLower(c.Format) {
		case "bad":
			fill, font = "#FFC7CE", "#9C0006"
		case "neutral":
			fill, font = "#FFEB9C", "#9C5700"
		}
		st, err := f.NewConditionalStyle(&excelize.Style{Font: &excelize.Font{Color: font}, Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{fill}}})
		if err != nil {
			return err
		}
		crit := map[string]string{">": "greater than", "<": "less than", ">=": "greater than or equal to", "<=": "less than or equal to", "=": "equal to", "!=": "not equal to", "between": "between"}[c.Criteria]
		if crit == "" {
			crit = "greater than"
		}
		o := excelize.ConditionalFormatOptions{Type: "cell", Criteria: crit, Format: &st, Value: c.Value}
		if crit == "between" {
			o.MinValue, o.MaxValue, o.Value = c.Value, c.Value2, ""
		}
		fmts = []excelize.ConditionalFormatOptions{o}
	}
	return f.SetConditionalFormat(sheet, c.Range, fmts)
}

func addChart(f *excelize.File, sheet string, ch ChartSpec, i int, start string, ncol, nrow, c0, r0 int, colors []string) error {
	types := map[string]excelize.ChartType{"col": excelize.Col, "column": excelize.Col, "bar": excelize.Bar, "line": excelize.Line, "pie": excelize.Pie, "doughnut": excelize.Doughnut, "area": excelize.Area, "scatter": excelize.Scatter}
	t, ok := types[strings.ToLower(ch.Type)]
	if !ok {
		t = excelize.Col
	}
	q := func(s string) string { return "'" + strings.ReplaceAll(sheet, "'", "''") + "'" }
	var series []excelize.ChartSeries
	if len(ch.Series) == 0 && nrow > 1 && ncol > 1 {
		// Per defecte: primera columna = categories, la resta = sèries.
		catCol, _ := excelize.ColumnNumberToName(c0)
		ch.Categories = fmt.Sprintf("%s!$%s$%d:$%s$%d", q(sheet), catCol, r0+1, catCol, r0+nrow-1)
		for ci := 1; ci < ncol; ci++ {
			col, _ := excelize.ColumnNumberToName(c0 + ci)
			series = append(series, excelize.ChartSeries{
				Name:       fmt.Sprintf("%s!$%s$%d", q(sheet), col, r0),
				Categories: ch.Categories,
				Values:     fmt.Sprintf("%s!$%s$%d:$%s$%d", q(sheet), col, r0+1, col, r0+nrow-1),
			})
		}
	}
	for _, s := range ch.Series {
		series = append(series, excelize.ChartSeries{Name: s.Name, Categories: ch.Categories, Values: s.Values})
	}
	if len(series) == 0 {
		return fmt.Errorf("cal almenys una sèrie")
	}
	if len(colors) > 0 && t != excelize.Pie && t != excelize.Doughnut {
		for k := range series {
			series[k].Fill = excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{colors[k%len(colors)]}}
		}
	}
	cell := ch.Cell
	if cell == "" {
		col, _ := excelize.ColumnNumberToName(c0 + ncol + 1)
		cell = fmt.Sprintf("%s%d", col, r0+i*18)
	}
	w, h := ch.Width, ch.Height
	if w == 0 {
		w = 560
	}
	if h == 0 {
		h = 320
	}
	c := &excelize.Chart{
		Type:   t,
		Series: series,
		Format: excelize.GraphicOptions{OffsetX: 10, OffsetY: 10},
		Legend: excelize.ChartLegend{Position: "bottom"},
		Title:  excelize.ChartTitle{Paragraph: []excelize.RichTextRun{{Text: ch.Title}}},
		PlotArea: excelize.ChartPlotArea{
			ShowVal: t != excelize.Line && len(series)*rangeLen(series[0].Values) <= 12,
		},
		Dimension: excelize.ChartDimension{Width: w, Height: h},
	}
	if t == excelize.Pie || t == excelize.Doughnut {
		c.PlotArea.ShowPercent = true
		c.PlotArea.ShowVal = false
	}
	return f.AddChart(sheet, cell, c)
}

// ---------- valors ----------

func display(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	default:
		return fmt.Sprint(x)
	}
}

// setAny escriu un valor JSON a una cel·la: «=…» fórmula, números com a
// números, booleans, text; null deixa la cel·la buida.
func setAny(f *excelize.File, sheet, cell string, v any) error {
	switch x := v.(type) {
	case nil:
		return nil
	case float64:
		return f.SetCellValue(sheet, cell, x)
	case bool:
		return f.SetCellBool(sheet, cell, x)
	case string:
		return SetString(f, sheet, cell, x)
	default:
		return f.SetCellValue(sheet, cell, fmt.Sprint(x))
	}
}

// SetString interpreta un text com el teclejaria un usuari: «=» fórmula,
// números (també «1.234,5» i «12%»), «true/false», la resta text.
func SetString(f *excelize.File, sheet, cell, s string) error {
	t := strings.TrimSpace(s)
	switch {
	case t == "":
		return f.SetCellValue(sheet, cell, nil)
	case strings.HasPrefix(t, "=") && len(t) > 1:
		return f.SetCellFormula(sheet, cell, strings.TrimPrefix(t, "="))
	case strings.HasPrefix(t, "'"):
		return f.SetCellStr(sheet, cell, strings.TrimPrefix(t, "'"))
	}
	if n, ok := parseNumber(t); ok {
		if strings.HasSuffix(t, "%") {
			if err := f.SetCellValue(sheet, cell, n/100); err != nil {
				return err
			}
			nf := "0.0%"
			return mergeStyle(f, sheet, cell, &excelize.Style{CustomNumFmt: &nf})
		}
		return f.SetCellValue(sheet, cell, n)
	}
	switch strings.ToLower(t) {
	case "true", "cert":
		return f.SetCellBool(sheet, cell, true)
	case "false", "fals":
		return f.SetCellBool(sheet, cell, false)
	}
	return f.SetCellStr(sheet, cell, s)
}

func parseNumber(s string) (float64, bool) {
	t := strings.TrimSuffix(strings.TrimSpace(s), "%")
	t = strings.TrimSpace(t)
	if t == "" {
		return 0, false
	}
	low := strings.ToLower(t)
	if low == "nan" || strings.Contains(low, "inf") || strings.HasPrefix(low, "0x") {
		return 0, false
	}
	if v, err := strconv.ParseFloat(t, 64); err == nil {
		return v, true
	}
	// Formats europeus: 1.234,56 → 1234.56
	if strings.Contains(t, ",") {
		e := strings.ReplaceAll(strings.ReplaceAll(t, ".", ""), ",", ".")
		if v, err := strconv.ParseFloat(e, 64); err == nil {
			return v, true
		}
	}
	return 0, false
}

// ---------- edició ----------

// Open obre un llibre (plantilla inclosa).
func Open(path string) (*excelize.File, error) {
	return excelize.OpenFile(path)
}

func sheetOrFirst(f *excelize.File, sheet string) (string, error) {
	list := f.GetSheetList()
	if sheet == "" && len(list) > 0 {
		return list[0], nil
	}
	for _, s := range list {
		if s == sheet {
			return s, nil
		}
	}
	for _, s := range list {
		if strings.EqualFold(s, sheet) {
			return s, nil
		}
	}
	return "", fmt.Errorf("full %q no existeix (hi ha: %s)", sheet, strings.Join(list, ", "))
}

// SetCells escriu diverses cel·les (cel·la → valor) d'un sol cop.
func SetCells(path, sheet string, cells map[string]string) (string, error) {
	f, err := Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	sh, err := sheetOrFirst(f, sheet)
	if err != nil {
		return "", err
	}
	for _, c := range sortedKeys(cells) {
		cell := strings.ToUpper(strings.TrimSpace(c))
		if _, _, err := excelize.CellNameToCoordinates(cell); err != nil {
			return "", fmt.Errorf("cel·la invàlida %q (p. ex. B2)", c)
		}
		if err := SetString(f, sh, cell, cells[c]); err != nil {
			return "", fmt.Errorf("%s: %v", cell, err)
		}
	}
	f.SetCalcProps(&excelize.CalcPropsOptions{FullCalcOnLoad: boolPtr(true)})
	if err := f.Save(); err != nil {
		return "", err
	}
	if len(cells) == 1 {
		for c, v := range cells {
			return fmt.Sprintf("posat %s!%s = %q", sh, strings.ToUpper(strings.TrimSpace(c)), v), nil
		}
	}
	return fmt.Sprintf("escrites %d cel·les a %s", len(cells), sh), nil
}

// SetRange omple un bloc des de la cel·la anchor; files de text amb «|».
func SetRange(path, sheet, anchor, content string) (string, error) {
	f, err := Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	sh, err := sheetOrFirst(f, sheet)
	if err != nil {
		return "", err
	}
	if anchor == "" {
		anchor = "A1"
	}
	c0, r0, err := excelize.CellNameToCoordinates(strings.ToUpper(anchor))
	if err != nil {
		return "", fmt.Errorf("cel·la invàlida %q", anchor)
	}
	n := 0
	rows := 0
	for _, line := range strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n") {
		t := strings.TrimSpace(line)
		if t == "" {
			continue
		}
		if strings.Trim(t, "|-: ") == "" {
			continue // separador de taula markdown
		}
		t = strings.TrimSuffix(strings.TrimPrefix(t, "|"), "|")
		for ci, v := range strings.Split(t, "|") {
			cell, _ := excelize.CoordinatesToCellName(c0+ci, r0+rows)
			if err := SetString(f, sh, cell, strings.TrimSpace(v)); err != nil {
				return "", fmt.Errorf("%s: %v", cell, err)
			}
			n++
		}
		rows++
		if rows > 5000 {
			return "", fmt.Errorf("massa files (màx. 5000)")
		}
	}
	f.SetCalcProps(&excelize.CalcPropsOptions{FullCalcOnLoad: boolPtr(true)})
	if err := f.Save(); err != nil {
		return "", err
	}
	return fmt.Sprintf("omplert %s!%s: %d files, %d cel·les", sh, strings.ToUpper(anchor), rows, n), nil
}

// ApplySheet aplica formats, rols, gràfics… d'una SheetSpec a un full
// existent (sense tocar valors si Rows és buit).
func ApplySheet(path string, sh SheetSpec, theme string) ([]string, error) {
	f, err := Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	name, err := sheetOrFirst(f, sh.Name)
	if err != nil {
		if sh.Name == "" {
			return nil, err
		}
		if _, err := f.NewSheet(sh.Name); err != nil {
			return nil, err
		}
		name = sh.Name
	}
	if len(sh.Rows) == 0 {
		f2 := false
		sh.Header = &f2
	}
	warns, err := fillSheet(f, name, sh, paletteFor(theme))
	if err != nil {
		return nil, err
	}
	f.SetCalcProps(&excelize.CalcPropsOptions{FullCalcOnLoad: boolPtr(true)})
	return warns, f.Save()
}

// ---------- recàlcul i revisió ----------

// RecalcReport és el resultat de recalcular totes les fórmules.
type RecalcReport struct {
	Formulas    int
	Errors      map[string][]string // tipus d'error → cel·les
	Unsupported []string            // fórmules que el motor no sap calcular
}

var errKinds = []string{"#DIV/0!", "#REF!", "#VALUE!", "#NAME?", "#N/A", "#NUM!", "#NULL!", "#SPILL!", "#CALC!"}

// Recalc avalua totes les fórmules i en recull els errors.
func Recalc(path string) (RecalcReport, error) {
	rep := RecalcReport{Errors: map[string][]string{}}
	f, err := Open(path)
	if err != nil {
		return rep, err
	}
	defer f.Close()
	for _, sh := range f.GetSheetList() {
		rows, err := f.GetRows(sh, excelize.Options{RawCellValue: true})
		if err != nil {
			continue
		}
		maxCols := 0
		for _, r := range rows {
			if len(r) > maxCols {
				maxCols = len(r)
			}
		}
		for ri := range rows {
			for ci := 0; ci < maxCols; ci++ {
				cell, _ := excelize.CoordinatesToCellName(ci+1, ri+1)
				fx, err := f.GetCellFormula(sh, cell)
				if err != nil || fx == "" {
					continue
				}
				rep.Formulas++
				ref := sh + "!" + cell
				v, err := f.CalcCellValue(sh, cell)
				if err != nil {
					msg := err.Error()
					kind := ""
					for _, k := range errKinds {
						if strings.Contains(msg, k) || v == k {
							kind = k
							break
						}
					}
					if kind == "" {
						if strings.Contains(strings.ToLower(msg), "not support") {
							rep.Unsupported = append(rep.Unsupported, ref+" ("+fx+")")
							continue
						}
						kind = "#ERROR"
					}
					rep.Errors[kind] = append(rep.Errors[kind], ref)
					continue
				}
				for _, k := range errKinds {
					if v == k {
						rep.Errors[k] = append(rep.Errors[k], ref)
					}
				}
			}
		}
	}
	return rep, nil
}

// String resumeix l'informe.
func (r RecalcReport) String() string {
	var b strings.Builder
	total := 0
	for _, v := range r.Errors {
		total += len(v)
	}
	b.WriteString(fmt.Sprintf("%d fórmules, %d amb error", r.Formulas, total))
	kinds := make([]string, 0, len(r.Errors))
	for k := range r.Errors {
		kinds = append(kinds, k)
	}
	sort.Strings(kinds)
	for _, k := range kinds {
		cells := r.Errors[k]
		if len(cells) > 20 {
			cells = append(cells[:20], fmt.Sprintf("… (+%d)", len(r.Errors[k])-20))
		}
		b.WriteString(fmt.Sprintf("\n  %s: %s", k, strings.Join(cells, ", ")))
	}
	if len(r.Unsupported) > 0 {
		u := r.Unsupported
		if len(u) > 10 {
			u = u[:10]
		}
		b.WriteString("\n  no avaluades (funció no suportada pel motor intern; Excel les calcularà): " + strings.Join(u, ", "))
	}
	return b.String()
}

// ---------- lectura ----------

// Read torna una vista textual: per full, files amb valors formatats i
// fórmules visibles («=B2*2 → 20»). Límit de files i columnes.
func Read(path string, maxSheets, maxRows, maxCols int) (string, error) {
	f, err := Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	var b strings.Builder
	sheets := f.GetSheetList()
	for si, sh := range sheets {
		if si >= maxSheets {
			b.WriteString(fmt.Sprintf("… (%d fulls més)\n", len(sheets)-si))
			break
		}
		rows, err := f.GetRows(sh)
		if err != nil {
			continue
		}
		dim, _ := f.GetSheetDimension(sh)
		b.WriteString(fmt.Sprintf("== %s (%s) ==\n", sh, dim))
		for ri, row := range rows {
			if ri >= maxRows {
				b.WriteString(fmt.Sprintf("… (%d files més)\n", len(rows)-ri))
				break
			}
			var cells []string
			empty := true
			for ci := 0; ci < len(row) && ci < maxCols; ci++ {
				cell, _ := excelize.CoordinatesToCellName(ci+1, ri+1)
				v := row[ci]
				if fx, _ := f.GetCellFormula(sh, cell); fx != "" {
					if v == "" {
						if cv, err := f.CalcCellValue(sh, cell); err == nil {
							v = cv
						}
					}
					v = "=" + fx + " → " + v
				}
				if v != "" {
					empty = false
				}
				cells = append(cells, v)
			}
			if empty {
				continue
			}
			b.WriteString(fmt.Sprintf("%d: %s\n", ri+1, strings.Join(cells, " | ")))
		}
	}
	return b.String(), nil
}

func defaultFont(f *excelize.File) string {
	if n, err := f.GetDefaultFont(); err == nil && n != "" {
		return n
	}
	return "Calibri"
}

// fitPage fa que el full s'imprimeixi (i es renderitzi) apaïsat i
// ajustat a una pàgina d'ample.
func fitPage(f *excelize.File, sheet string) {
	o := "landscape"
	one, zero := 1, 0
	f.SetPageLayout(sheet, &excelize.PageLayoutOptions{Orientation: &o, FitToWidth: &one, FitToHeight: &zero})
	f.SetSheetProps(sheet, &excelize.SheetPropsOptions{FitToPage: boolPtr(true)})
}

// FromTemplate copia una plantilla (.xltx) com a llibre nou (.xlsx).
func FromTemplate(template, path string) error {
	f, err := excelize.OpenFile(template)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return f.SaveAs(path)
}

// rangeLen compta les cel·les d'una referència «Full!$B$2:$B$13».
func rangeLen(ref string) int {
	if i := strings.LastIndex(ref, "!"); i >= 0 {
		ref = ref[i+1:]
	}
	a, b, ok := strings.Cut(strings.ReplaceAll(ref, "$", ""), ":")
	if !ok {
		return 1
	}
	c1, r1, e1 := excelize.CellNameToCoordinates(a)
	c2, r2, e2 := excelize.CellNameToCoordinates(b)
	if e1 != nil || e2 != nil {
		return 0
	}
	return (c2 - c1 + 1) * (r2 - r1 + 1)
}
