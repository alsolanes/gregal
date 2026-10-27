package tools

// Office: crear, llegir, editar i revisar visualment .docx/.pptx/.xlsx
// (i les plantilles .dotx/.potx/.xltx).
//
//   - docx/pptx: internal/ooxml, un arbre XML que conserva els prefixos
//     de namespace (encoding/xml.Encoder els trencava i el Word deixava
//     d'obrir el fitxer). Estils, numeració, taules, imatges, control de
//     canvis, comentaris; temes i layouts propis per a presentacions.
//   - xlsx: internal/xlsxdoc (excelize): fórmules, formats, gràfics.
//   - render: internal/officerender (LibreOffice/Office → PNG).
//
// Tota escriptura passa pel journal (SnapOp) perquè /rewind la desfaci, i
// cada edició es valida: si deixaria el paquet amb errors estructurals
// nous, no es desa.

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"image"
	"image/png"
	"strings"
	"time"

	"gregal/internal/llm"
	"gregal/internal/officerender"
	"gregal/internal/ooxml"
	"gregal/internal/xlsxdoc"
)

// OfficeKind diu si el fitxer és d'office modern ("xlsx", "docx",
// "pptx", incloses plantilles i fitxers amb macros), antic ("legacy") o
// res ("").
func OfficeKind(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".xlsx", ".xltx", ".xlsm":
		return "xlsx"
	case ".docx", ".dotx", ".docm":
		return "docx"
	case ".pptx", ".potx", ".pptm":
		return "pptx"
	case ".xls", ".doc", ".ppt":
		return "legacy"
	}
	return ""
}

func officeCheck(path string) (string, error) {
	kind := OfficeKind(path)
	switch kind {
	case "legacy":
		return "", fmt.Errorf("format antic (.doc/.xls/.ppt) no suportat: converteix-lo amb LibreOffice (soffice --headless --convert-to %sx %s)", strings.TrimPrefix(strings.ToLower(filepath.Ext(path)), "."), path)
	case "":
		if IsPDF(path) {
			return "", fmt.Errorf("%s és un PDF: fes servir pdf_read (text, info, images) i pdf_edit", path)
		}
		return "", fmt.Errorf("no és un document office: %s", path)
	}
	return kind, nil
}

// ---------- lectura ----------

// OfficeRead llegeix un document (vista de text per al model).
func OfficeRead(path string) (string, error) { return OfficeReadDetail(path, "text") }

// OfficeReadDetail: detail "text" (per defecte), "layout" (pptx:
// layouts, formes amb posició i desbordaments; docx: estils
// disponibles) o "full" (sense límits).
func OfficeReadDetail(path, detail string) (string, error) {
	if IsPDF(path) {
		// Els models demanen office_read per a qualsevol document: un PDF
		// es llegeix igual que amb pdf_read (sense les imatges).
		if detail == "layout" {
			return PdfInfoText(context.Background(), path)
		}
		return PdfText(path)
	}
	kind, err := officeCheck(path)
	if err != nil {
		return "", err
	}
	full := detail == "full"
	switch kind {
	case "xlsx":
		if full {
			return xlsxdoc.Read(path, 200, 200000, 60)
		}
		return xlsxdoc.Read(path, 6, 80, 26)
	case "docx":
		pk, err := ooxml.Open(path)
		if err != nil {
			return "", err
		}
		lim := 300
		if full {
			lim = 0
		}
		out, err := ooxml.ReadDocx(pk, lim)
		if err != nil {
			return "", err
		}
		if detail == "layout" {
			d, _ := ooxml.OpenDocx(pk)
			var roles []string
			for role, id := range d.StyleIDs() {
				roles = append(roles, role+"="+id)
			}
			out = "Estils: " + strings.Join(sortStrings(roles), ", ") + "\n\n" + out
		}
		return out, nil
	case "pptx":
		pk, err := ooxml.Open(path)
		if err != nil {
			return "", err
		}
		if detail == "layout" {
			p, err := ooxml.OpenPptx(pk)
			if err != nil {
				return "", err
			}
			return ooxml.Inventory(p), nil
		}
		return ooxml.ReadPptx(pk)
	}
	return "", nil
}

// OfficeReadFull és la lectura sense topalls que fa servir la indexació
// de coneixement: «[FULL nom]» per full, «— Diapo N —» per diapositiva i
// «[Estil] text» per paràgraf de Word amb estil.
func OfficeReadFull(path string) (string, error) {
	kind, err := officeCheck(path)
	if err != nil {
		return "", err
	}
	switch kind {
	case "xlsx":
		out, err := xlsxdoc.Read(path, 200, 200000, 60)
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(sheetHeadRe.ReplaceAllString(out, "[FULL $1]")), nil
	case "pptx":
		pk, err := ooxml.Open(path)
		if err != nil {
			return "", err
		}
		out, err := ooxml.ReadPptx(pk)
		return strings.TrimSpace(out), err
	default:
		pk, err := ooxml.Open(path)
		if err != nil {
			return "", err
		}
		return docxStyledText(pk)
	}
}

var sheetHeadRe = regexp.MustCompile(`(?m)^== (.*) \([^)]*\) ==$`)

func docxStyledText(pk *ooxml.Package) (string, error) {
	d, err := ooxml.OpenDocx(pk)
	if err != nil {
		return "", err
	}
	names := map[string]string{}
	if sp := pk.ByType(d.Main, ooxml.RelStyles); len(sp) > 0 {
		if sd, err := pk.XML(sp[0]); err == nil {
			for _, s := range sd.Root().Elems("w:style") {
				names[s.Get("w:styleId")] = s.Child("w:name").Get("w:val")
			}
		}
	}
	var b strings.Builder
	for _, p := range ooxml.Paragraphs(d.Body) {
		t := strings.TrimSpace(ooxml.ParaText(p))
		if t == "" {
			continue
		}
		if ps := p.Path("w:pPr", "w:pStyle"); ps != nil {
			id := ps.Get("w:val")
			name := names[id]
			if name == "" {
				name = id
			}
			if !strings.EqualFold(name, "normal") {
				b.WriteString("[" + name + "] ")
			}
		}
		b.WriteString(t + "\n")
	}
	return strings.TrimSpace(b.String()), nil
}

func sortStrings(xs []string) []string {
	out := append([]string(nil), xs...)
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j] < out[j-1]; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}

// ---------- creació ----------

// CreateOpts configura office_create.
type CreateOpts struct {
	Path, Title, Content string
	Spec                 string // JSON (pptx: diapositives; xlsx: fulls)
	Template             string // .dotx/.docx, .potx/.pptx, .xltx/.xlsx
	Theme                string // default, modern, nature, elegant, mono (+ «:primary=…»)
	Subtitle, Author     string
	Page                 string // a4 / letter
	Landscape            bool
	NoPageNumbers        bool
	KeepTemplateSlides   bool
	Overwrite            bool // substitueix un fitxer existent (queda al journal)
	Vars                 map[string]string
	BaseDir              string // per resoldre imatges relatives
}

// OfficeCreate (compatibilitat): títol + contingut markdown.
func OfficeCreate(path, title, content string) (string, error) {
	return OfficeCreateOpts(CreateOpts{Path: path, Title: title, Content: content})
}

// OfficeCreateOpts crea un document nou amb format de debò.
func OfficeCreateOpts(o CreateOpts) (string, error) {
	kind, err := officeCheck(o.Path)
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(o.Path); err == nil {
		if !o.Overwrite {
			return "", fmt.Errorf("%s ja existeix: fes servir office_edit per modificar-lo, o overwrite=true per tornar-lo a generar", o.Path)
		}
		if err := OfficeWritable(o.Path); err != nil {
			return "", err
		}
		Active.SnapOp(o.Path, "edit") // /rewind recupera la versió anterior
		if err := os.Remove(o.Path); err != nil {
			return "", err
		}
	}
	if o.Template != "" {
		if _, err := os.Stat(o.Template); err != nil {
			return "", fmt.Errorf("plantilla: %v", err)
		}
		if OfficeKind(o.Template) != kind {
			return "", fmt.Errorf("la plantilla %s no és del mateix tipus que %s", o.Template, o.Path)
		}
	}
	if o.BaseDir == "" {
		o.BaseDir = filepath.Dir(o.Path)
	}
	if err := os.MkdirAll(filepath.Dir(o.Path), 0o755); err != nil {
		return "", err
	}
	Active.SnapOp(o.Path, "create")
	var notes []string
	switch kind {
	case "docx":
		notes, err = createDocx(o)
	case "pptx":
		notes, err = createPptx(o)
	case "xlsx":
		notes, err = createXlsx(o)
	}
	if err != nil {
		os.Remove(o.Path)
		return "", err
	}
	msg := fmt.Sprintf("creat %s", o.Path)
	if o.Template != "" {
		msg += " a partir de " + filepath.Base(o.Template)
	}
	if len(notes) > 0 {
		msg += "\n" + strings.Join(notes, "\n")
	}
	if officerender.Available() != "" {
		msg += "\nRevisa'l visualment amb office_render abans de donar-lo per acabat."
	}
	return msg, nil
}

func createDocx(o CreateOpts) ([]string, error) {
	var d *ooxml.Docx
	var notes []string
	content := o.Content
	if o.Template != "" {
		var err error
		d, err = ooxml.DocxFromTemplate(o.Template)
		if err != nil {
			return nil, err
		}
		d.BaseDir = o.BaseDir
		if o.Theme != "" {
			d.Theme = ooxml.GetTheme(o.Theme)
		}
		hasVars := strings.Contains(ooxml.ParaTextAll(d.Body), "{{")
		vars := map[string]string{}
		for k, v := range o.Vars {
			vars[k] = v
		}
		if o.Title != "" {
			if _, ok := vars["title"]; !ok {
				vars["title"] = o.Title
			}
		}
		switch {
		case hasVars:
			// Plantilla amb marcadors: es conserva i s'omple.
			if ps := ooxml.FindParagraph(d.Body, "{{content}}"); len(ps) > 0 && content != "" {
				if _, _, err := d.Insert("{{content}}", "after", content, false, ""); err != nil {
					return nil, err
				}
				ps[0].Remove()
			} else if content != "" {
				d.AppendMarkdown(content)
			}
			n, left := d.FillVars(vars)
			notes = append(notes, fmt.Sprintf("plantilla: %d marcadors omplerts", n))
			if len(left) > 0 {
				notes = append(notes, "marcadors sense valor (passa'ls a vars): {{"+strings.Join(left, "}}, {{")+"}}")
			}
		default:
			md := content
			if o.Title != "" && !strings.HasPrefix(strings.TrimSpace(content), "# ") {
				md = "# " + o.Title + "\n\n" + content
			}
			d.ReplaceBody(md)
		}
		ids := d.StyleIDs()
		notes = append(notes, fmt.Sprintf("estils de la plantilla: títols→%s/%s/%s", ids["Heading1"], ids["Heading2"], ids["Heading3"]))
	} else {
		d = ooxml.NewDocx(ooxml.DocxOptions{
			Title: o.Title, Subtitle: o.Subtitle, Author: o.Author, Page: o.Page, Landscape: o.Landscape,
			PageNumbers: !o.NoPageNumbers, Theme: ooxml.GetTheme(o.Theme), BaseDir: o.BaseDir,
		})
		d.AppendMarkdown(content)
		if len(o.Vars) > 0 {
			d.FillVars(o.Vars)
		}
	}
	return notes, saveValidated(d.Pkg, nil, o.Path)
}

func createPptx(o CreateOpts) ([]string, error) {
	var spec ooxml.DeckSpec
	var err error
	if strings.TrimSpace(o.Spec) != "" {
		spec, err = ooxml.ParseDeckSpec(o.Spec)
		if err != nil {
			if fixed, err2 := ooxml.ParseDeckSpec(llm.FixJSON(o.Spec)); err2 == nil {
				spec, err = fixed, nil
			} else {
				return nil, fmt.Errorf("spec JSON invàlid: %v. Si el JSON et costa, passa content en markdown: una línia «# Títol» per diapositiva i punts amb «- »", err)
			}
		}
	} else if strings.TrimSpace(o.Content) != "" || o.Title != "" {
		spec = ooxml.DeckFromMarkdown(o.Title, o.Content)
	}
	if spec.Title == "" {
		spec.Title = o.Title
	}
	theme := firstOfStr(o.Theme, spec.Theme)
	var p *ooxml.Pptx
	if o.Template != "" {
		p, err = ooxml.PptxFromTemplate(o.Template)
		if err != nil {
			return nil, err
		}
		if !o.KeepTemplateSlides {
			for i := len(p.SlideParts()) - 1; i >= 0; i-- {
				p.DeleteSlide(i)
			}
		}
	} else {
		p = ooxml.NewPptx(ooxml.GetTheme(theme), spec.Title)
	}
	p.BaseDir = o.BaseDir
	if len(spec.Slides) == 0 {
		return nil, fmt.Errorf("cap diapositiva: passa spec (JSON) o content (markdown amb un «#» per diapositiva)")
	}
	warns, err := p.BuildSlides(spec)
	if err != nil {
		return nil, err
	}
	if len(o.Vars) > 0 {
		replaceVarsPptx(p, o.Vars)
	}
	notes := append([]string{fmt.Sprintf("%d diapositives", len(p.SlideParts()))}, warns...)
	if qa := ooxml.QAPptx(p); len(qa) > 0 {
		notes = append(notes, "QA: "+strings.Join(limitList(qa, 8), "; "))
	}
	return notes, saveValidated(p.Pkg, nil, o.Path)
}

func createXlsx(o CreateOpts) ([]string, error) {
	var sp xlsxdoc.Spec
	var err error
	switch {
	case strings.TrimSpace(o.Spec) != "":
		sp, err = xlsxdoc.ParseSpec(o.Spec)
		if err != nil {
			if fixed, err2 := xlsxdoc.ParseSpec(llm.FixJSON(o.Spec)); err2 == nil {
				sp, err = fixed, nil
			} else {
				return nil, fmt.Errorf("spec JSON invàlid: %v. Si el JSON et costa, passa content: una fila per línia, cel·les separades per |, fórmules començant per = (p. ex. «Gener | 1000 | 800 | =B2-C2»)", err)
			}
		}
	default:
		// Text: una fila per línia, cel·les separades per |.
		var rows [][]any
		for _, line := range strings.Split(strings.ReplaceAll(o.Content, "\r\n", "\n"), "\n") {
			t := strings.TrimSpace(line)
			if t == "" || strings.Trim(t, "|-: ") == "" {
				continue
			}
			t = strings.TrimSuffix(strings.TrimPrefix(t, "|"), "|")
			var row []any
			for _, c := range strings.Split(t, "|") {
				row = append(row, strings.TrimSpace(c))
			}
			rows = append(rows, row)
		}
		name := o.Title
		if name == "" {
			name = "Full1"
		}
		sp.Sheets = []xlsxdoc.SheetSpec{{Name: name, Rows: rows}}
	}
	if sp.Theme == "" {
		sp.Theme = o.Theme
	}
	warns, err := xlsxdoc.Create(o.Path, o.Template, sp)
	if err != nil {
		return nil, err
	}
	rep, err := xlsxdoc.Recalc(o.Path)
	if err == nil && rep.Formulas > 0 {
		warns = append(warns, "fórmules: "+rep.String())
	}
	return warns, nil
}

func firstOfStr(xs ...string) string {
	for _, x := range xs {
		if strings.TrimSpace(x) != "" {
			return x
		}
	}
	return ""
}

func limitList(xs []string, n int) []string {
	if len(xs) <= n {
		return xs
	}
	return append(append([]string(nil), xs[:n]...), fmt.Sprintf("… (+%d)", len(xs)-n))
}

// saveValidated desa el paquet si no introdueix errors estructurals nous
// respecte de l'original (si n'hi ha).
func saveValidated(pk *ooxml.Package, orig []byte, path string) error {
	data, err := pk.Bytes()
	if err != nil {
		return err
	}
	check, err := ooxml.OpenBytes(data)
	if err != nil {
		return err
	}
	var base *ooxml.Package
	if orig != nil {
		base, _ = ooxml.OpenBytes(orig)
	}
	if iss := ooxml.NewIssues(check, base); len(iss) > 0 {
		return fmt.Errorf("el resultat no seria un document vàlid; no s'ha desat:\n  %s", strings.Join(limitList(iss, 10), "\n  "))
	}
	return pk.Save(path)
}

func replaceVarsPptx(p *ooxml.Pptx, vars map[string]string) int {
	n := 0
	for k, v := range vars {
		f, err := ooxml.NewFinder(`\{\{\s*`+regexp.QuoteMeta(k)+`\s*\}\}`, true, false)
		if err != nil {
			continue
		}
		for _, s := range p.SlideParts() {
			if doc, err := p.Pkg.XML(s); err == nil {
				for _, para := range ooxml.Paragraphs(doc) {
					n += ooxml.ReplaceInPara(para, f, strings.ReplaceAll(v, "$", "$$"), nil)
				}
			}
		}
	}
	return n
}

// ---------- edició ----------

// EditOpts configura office_edit.
type EditOpts struct {
	Path, Op                         string
	Sheet, Cell, Value               string
	Find, Replace                    string
	Regex, IgnoreCase, Track, All    bool
	Content, Anchor, Position        string
	Text, Author                     string
	Slide, To                        int // 1-based
	Layout, Spec                     string
	Shapes, Cells, Vars              map[string]string
	BaseDir                          string
}

// OfficeEdit aplica una operació. Torna un resum per al model.
func OfficeEdit(o EditOpts) (string, error) {
	kind, err := officeCheck(o.Path)
	if err != nil {
		return "", err
	}
	if err := OfficeWritable(o.Path); err != nil {
		return "", err
	}
	if o.BaseDir == "" {
		o.BaseDir = filepath.Dir(o.Path)
	}
	orig, err := os.ReadFile(o.Path)
	if err != nil {
		return "", err
	}
	Active.SnapOp(o.Path, "edit")
	switch kind {
	case "xlsx":
		return editXlsx(o)
	case "docx":
		return editDocx(o, orig)
	default:
		return editPptx(o, orig)
	}
}

func opsHelp(kind string) string {
	switch kind {
	case "docx":
		return "replace, insert, append, delete, comment, fill_vars, accept_changes"
	case "pptx":
		return "replace, add_slide, append, duplicate_slide, delete_slide, move_slide, fill, notes, fill_vars"
	}
	return "set_cell, set_cells, set_range, format, recalc"
}

func editDocx(o EditOpts, orig []byte) (string, error) {
	pk, err := ooxml.OpenBytes(orig)
	if err != nil {
		return "", err
	}
	d, err := ooxml.OpenDocx(pk)
	if err != nil {
		return "", err
	}
	d.BaseDir = o.BaseDir
	var msg string
	switch o.Op {
	case "replace":
		n, err := d.Replace(o.Find, o.Replace, ooxml.ReplaceOpts{Regex: o.Regex, IgnoreCase: o.IgnoreCase, Track: o.Track, Author: o.Author})
		if err != nil {
			return "", err
		}
		if n == 0 {
			return "", fmt.Errorf("%q no apareix al document (ni a capçaleres, peus, notes o comentaris); llegeix-lo amb office_read per copiar el text exacte", o.Find)
		}
		msg = fmt.Sprintf("substituït %d cop(s)%s; el format de cada fragment es conserva", n, trackNote(o.Track))
	case "insert", "append":
		pos := o.Position
		if o.Op == "append" {
			pos = "end"
		}
		n, warn, err := d.Insert(o.Anchor, pos, firstOfStr(o.Content, o.Text, o.Value), o.Track, o.Author)
		if err != nil {
			return "", err
		}
		msg = fmt.Sprintf("afegits %d blocs amb estils del document%s%s", n, trackNote(o.Track), warn)
	case "delete":
		n, err := d.DeleteParagraphs(firstOfStr(o.Anchor, o.Find), o.All, o.Track, o.Author)
		if err != nil {
			return "", err
		}
		msg = fmt.Sprintf("esborrats %d paràgraf(s)%s", n, trackNote(o.Track))
	case "comment":
		if err := d.AddComment(firstOfStr(o.Anchor, o.Find), firstOfStr(o.Text, o.Content), o.Author); err != nil {
			return "", err
		}
		msg = "comentari afegit"
	case "fill_vars":
		n, left := d.FillVars(o.Vars)
		msg = fmt.Sprintf("omplerts %d marcadors", n)
		if len(left) > 0 {
			msg += "; sense valor: {{" + strings.Join(left, "}}, {{") + "}}"
		}
	case "accept_changes":
		msg = fmt.Sprintf("acceptats %d canvis registrats", d.AcceptAll())
	case "track_on", "track_off":
		d.SetTrackRevisions(o.Op == "track_on")
		msg = "control de canvis " + map[bool]string{true: "activat", false: "desactivat"}[o.Op == "track_on"]
	default:
		return "", fmt.Errorf("op %q no vàlida per a docx (%s)", o.Op, opsHelp("docx"))
	}
	if err := saveValidated(pk, orig, o.Path); err != nil {
		return "", err
	}
	return msg + " a " + o.Path, nil
}

func trackNote(track bool) string {
	if track {
		return " (amb control de canvis)"
	}
	return ""
}

func editPptx(o EditOpts, orig []byte) (string, error) {
	pk, err := ooxml.OpenBytes(orig)
	if err != nil {
		return "", err
	}
	p, err := ooxml.OpenPptx(pk)
	if err != nil {
		return "", err
	}
	p.Theme = themeOfPptx(p)
	p.BaseDir = o.BaseDir
	n := len(p.SlideParts())
	slide := o.Slide - 1
	var msg string
	switch o.Op {
	case "replace":
		f, err := ooxml.NewFinder(o.Find, o.Regex, o.IgnoreCase)
		if err != nil {
			return "", err
		}
		cnt := 0
		for _, s := range p.SlideParts() {
			parts := append([]string{s}, pk.ByType(s, ooxml.RelNotesSlide)...)
			for _, part := range parts {
				if doc, err := pk.XML(part); err == nil {
					for _, para := range ooxml.Paragraphs(doc) {
						cnt += ooxml.ReplaceInPara(para, f, o.Replace, nil)
					}
				}
			}
		}
		if cnt == 0 {
			return "", fmt.Errorf("%q no apareix a cap diapositiva ni a les notes", o.Find)
		}
		msg = fmt.Sprintf("substituït %d cop(s), format conservat", cnt)
	case "add_slide", "append":
		var spec ooxml.DeckSpec
		if strings.TrimSpace(o.Spec) != "" {
			spec, err = ooxml.ParseDeckSpec(o.Spec)
			if err != nil {
				// una sola diapositiva com a objecte
				var one ooxml.SlideSpec
				if e2 := json.Unmarshal([]byte(o.Spec), &one); e2 != nil {
					return "", fmt.Errorf("spec JSON invàlid: %v", err)
				}
				spec.Slides = []ooxml.SlideSpec{one}
			}
		} else {
			spec = ooxml.DeckFromMarkdown("", firstOfStr(o.Content, o.Text))
			if len(spec.Slides) > 0 && spec.Slides[0].Layout == "title" && o.Layout == "" {
				spec.Slides[0].Layout = "content"
				spec.Slides[0].Body = spec.Slides[0].Subtitle
				spec.Slides[0].Subtitle = ""
			}
		}
		if o.Layout != "" {
			for i := range spec.Slides {
				if spec.Slides[i].Layout == "" || len(spec.Slides) == 1 {
					spec.Slides[i].Layout = o.Layout
				}
			}
		}
		warns, err := p.BuildSlides(spec)
		if err != nil {
			return "", err
		}
		added := len(spec.Slides)
		if o.Op == "add_slide" && o.To > 0 && o.To <= n {
			for i := 0; i < added; i++ {
				p.MoveSlide(n+i, o.To-1+i)
			}
		}
		msg = fmt.Sprintf("afegides %d diapositives (ara n'hi ha %d)", added, len(p.SlideParts()))
		if len(warns) > 0 {
			msg += "; " + strings.Join(warns, "; ")
		}
	case "duplicate_slide":
		pos := -1
		if o.To > 0 {
			pos = o.To - 1
		}
		if _, err := p.DuplicateSlide(slide, pos); err != nil {
			return "", err
		}
		msg = fmt.Sprintf("duplicada la diapositiva %d", o.Slide)
	case "delete_slide":
		if err := p.DeleteSlide(slide); err != nil {
			return "", err
		}
		msg = fmt.Sprintf("esborrada la diapositiva %d (queden %d)", o.Slide, n-1)
	case "move_slide":
		if err := p.MoveSlide(slide, o.To-1); err != nil {
			return "", err
		}
		msg = fmt.Sprintf("diapositiva %d moguda a la posició %d", o.Slide, o.To)
	case "fill":
		if len(o.Shapes) == 0 {
			return "", fmt.Errorf("fill necessita shapes: {\"title\": \"…\", \"body\": \"- punt\"} (claus: title, subtitle, body, body2, nom de forma o id=N; mira-les amb office_read detail=layout)")
		}
		c, err := ooxml.FillShapes(p, slide, o.Shapes)
		if err != nil {
			return "", err
		}
		msg = fmt.Sprintf("omplertes %d formes de la diapositiva %d", c, o.Slide)
	case "notes":
		sl := p.SlideParts()
		if slide < 0 || slide >= len(sl) {
			return "", fmt.Errorf("diapositiva %d no existeix (n'hi ha %d)", o.Slide, len(sl))
		}
		if err := p.SetNotes(sl[slide], firstOfStr(o.Text, o.Content)); err != nil {
			return "", err
		}
		msg = fmt.Sprintf("notes de la diapositiva %d actualitzades", o.Slide)
	case "fill_vars":
		msg = fmt.Sprintf("omplerts %d marcadors", replaceVarsPptx(p, o.Vars))
	default:
		return "", fmt.Errorf("op %q no vàlida per a pptx (%s)", o.Op, opsHelp("pptx"))
	}
	if qa := ooxml.QAPptx(p); len(qa) > 0 {
		msg += "\nQA: " + strings.Join(limitList(qa, 6), "; ")
	}
	if err := saveValidated(pk, orig, o.Path); err != nil {
		return "", err
	}
	return msg + " a " + o.Path, nil
}

// themeOfPptx llegeix colors/tipus de lletra del tema de la presentació
// (per a taules i gràfics nous coherents amb la resta).
func themeOfPptx(p *ooxml.Pptx) ooxml.Theme { return p.LoadTheme() }

func editXlsx(o EditOpts) (string, error) {
	switch o.Op {
	case "set_cell":
		return xlsxdoc.SetCells(o.Path, o.Sheet, map[string]string{o.Cell: o.Value})
	case "set_cells":
		if len(o.Cells) == 0 {
			return "", fmt.Errorf("set_cells necessita cells: {\"B2\": \"10\", \"C2\": \"=B2*2\"}")
		}
		return xlsxdoc.SetCells(o.Path, o.Sheet, o.Cells)
	case "set_range":
		return xlsxdoc.SetRange(o.Path, o.Sheet, o.Cell, firstOfStr(o.Content, o.Value))
	case "format":
		var sh xlsxdoc.SheetSpec
		if err := json.Unmarshal([]byte(o.Spec), &sh); err != nil {
			return "", fmt.Errorf("spec JSON invàlid (number_formats, roles, widths, freeze, conditional, charts, merge, comments…): %v", err)
		}
		if sh.Name == "" {
			sh.Name = o.Sheet
		}
		warns, err := xlsxdoc.ApplySheet(o.Path, sh, "")
		if err != nil {
			return "", err
		}
		msg := "format aplicat a " + o.Path
		if len(warns) > 0 {
			msg += "; " + strings.Join(warns, "; ")
		}
		return msg, nil
	case "recalc":
		rep, err := xlsxdoc.Recalc(o.Path)
		if err != nil {
			return "", err
		}
		return rep.String(), nil
	}
	return "", fmt.Errorf("op %q no vàlida per a xlsx (%s)", o.Op, opsHelp("xlsx"))
}

// Compatibilitat amb les crides antigues (web, TUI).

// OfficeXlsxSet posa un valor (o fórmula, si comença per «=»).
func OfficeXlsxSet(path, sheet, cell, value string) (string, error) {
	return OfficeEdit(EditOpts{Path: path, Op: "set_cell", Sheet: sheet, Cell: cell, Value: value})
}

// OfficeXlsxSetRange omple un bloc de cel·les.
func OfficeXlsxSetRange(path, sheet, anchor, content string) (string, error) {
	return OfficeEdit(EditOpts{Path: path, Op: "set_range", Sheet: sheet, Cell: anchor, Content: content})
}

// OfficeReplace substitueix text conservant el format.
func OfficeReplace(path, find, replace string) (string, error) {
	return OfficeEdit(EditOpts{Path: path, Op: "replace", Find: find, Replace: replace})
}

// OfficeAppend afegeix contingut al final (docx) o diapositives (pptx).
func OfficeAppend(path, content string) (string, error) {
	return OfficeEdit(EditOpts{Path: path, Op: "append", Content: content})
}

// ---------- render ----------

// RenderOpts configura office_render.
type RenderOpts struct {
	Path  string
	Pages string // "1,3-5" (buit = les primeres 12 en graella)
}

// OfficeRender converteix el document en imatges i fa QA. Torna text
// (resum, problemes trobats) i imatges (data URL): una graella de
// miniatures i, si es demanen pàgines concretes, aquestes a mida gran.
func OfficeRender(ctx context.Context, o RenderOpts) (string, []string, error) {
	kind, err := officeCheck(o.Path)
	if err != nil {
		return "", nil, err
	}
	var qa []string
	// QA estructural (sempre, amb o sense motor de render).
	if raw, err := os.ReadFile(o.Path); err == nil && kind != "xlsx" {
		if pk, err := ooxml.OpenBytes(raw); err == nil {
			for _, s := range ooxml.Validate(pk) {
				qa = append(qa, "estructura: "+s)
			}
			if kind == "pptx" {
				if p, err := ooxml.OpenPptx(pk); err == nil {
					qa = append(qa, ooxml.QAPptx(p)...)
				}
			} else if txt, err := ooxml.ReadDocx(pk, 0); err == nil {
				if left := ooxml.LeftoverText(txt); len(left) > 0 {
					qa = append(qa, "text de mostra: "+strings.Join(left, ", "))
				}
				if strings.Contains(txt, "{{") {
					qa = append(qa, "queden marcadors {{…}} sense omplir")
				}
			}
		}
	}
	if kind == "xlsx" {
		if rep, err := xlsxdoc.Recalc(o.Path); err == nil && rep.Formulas > 0 {
			qa = append(qa, "fórmules: "+rep.String())
		}
	}
	var b strings.Builder
	if officerender.Available() == "" {
		b.WriteString("Sense comprovació visual: " + officerender.ErrNoEngine.Error() + "\n")
		writeQA(&b, qa)
		return b.String(), nil, nil
	}
	sum := sha1.Sum([]byte(o.Path + time.Now().String()))
	out := filepath.Join(os.TempDir(), "gregal-render", hex.EncodeToString(sum[:6]))
	pages := officerender.PageList(o.Pages)
	res, err := officerender.Render(ctx, o.Path, officerender.Options{OutDir: out, Pages: pages, Width: 1280})
	if err != nil {
		b.WriteString("No s'ha pogut renderitzar: " + err.Error() + "\n")
		writeQA(&b, qa)
		return b.String(), nil, nil
	}
	unit, units := "pàgina", "pàgines"
	if kind == "pptx" {
		unit, units = "diapositiva", "diapositives"
	}
	b.WriteString(fmt.Sprintf("Renderitzat amb %s: %d %s en total, %d a la imatge.\n", res.Engine, res.Total, units, len(res.Pages)))
	for _, n := range res.Notes {
		b.WriteString("nota: " + n + "\n")
	}
	var imgs []string
	labels := pages
	if len(labels) == 0 {
		for i := range res.Pages {
			labels = append(labels, i+1)
		}
	}
	if len(pages) > 0 && len(pages) <= 4 {
		for _, pg := range res.Pages {
			if u, err := imageURLScaled(pg, 1280); err == nil {
				imgs = append(imgs, u)
			}
		}
		b.WriteString("Imatges: " + unit + " " + o.Pages + " a mida gran.\n")
	} else {
		sheet := filepath.Join(out, "contact.png")
		cols := 3
		thumb := 520
		if kind != "pptx" {
			cols, thumb = 4, 400
		}
		if err := officerender.ContactSheet(res.Pages, labels, cols, thumb, sheet); err == nil {
			if u, err := ReadImageDataURL(sheet); err == nil {
				imgs = append(imgs, u)
			}
		}
		b.WriteString("Imatge: graella numerada. Per veure detalls, torna a cridar amb pages=\"N\".\n")
	}
	if res.PDF != "" {
		b.WriteString("PDF: " + res.PDF + "\n")
	}
	b.WriteString("Revisa: text que desborda o es talla, solapaments, contrast, alineacions, taules trencades, pàgines buides, restes de text de mostra.\n")
	writeQA(&b, qa)
	return b.String(), imgs, nil
}

func writeQA(b *strings.Builder, qa []string) {
	if len(qa) == 0 {
		b.WriteString("QA automàtica: cap problema detectat.\n")
		return
	}
	b.WriteString("QA automàtica:\n")
	for _, q := range limitList(qa, 15) {
		b.WriteString("  - " + q + "\n")
	}
}

func imageURLScaled(path string, w int) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	if len(raw) < 3<<20 {
		return DataURLForBytes(raw, path)
	}
	img, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, officerender.Scale(img, w)); err != nil {
		return "", err
	}
	return DataURLForBytes(buf.Bytes(), path)
}

// OfficeRenderAvailable descriu el motor de render (per al doctor).
func OfficeRenderAvailable() string { return officerender.Available() }
