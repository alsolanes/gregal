package ooxml

import (
	"bytes"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	nsW   = "http://schemas.openxmlformats.org/wordprocessingml/2006/main"
	nsR   = "http://schemas.openxmlformats.org/officeDocument/2006/relationships"
	nsWP  = "http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing"
	nsA   = "http://schemas.openxmlformats.org/drawingml/2006/main"
	nsPic = "http://schemas.openxmlformats.org/drawingml/2006/picture"

	ctDocxMain  = "application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"
	ctStyles    = "application/vnd.openxmlformats-officedocument.wordprocessingml.styles+xml"
	ctNumbering = "application/vnd.openxmlformats-officedocument.wordprocessingml.numbering+xml"
	ctSettings  = "application/vnd.openxmlformats-officedocument.wordprocessingml.settings+xml"
	ctHeader    = "application/vnd.openxmlformats-officedocument.wordprocessingml.header+xml"
	ctFooter    = "application/vnd.openxmlformats-officedocument.wordprocessingml.footer+xml"
	ctComments  = "application/vnd.openxmlformats-officedocument.wordprocessingml.comments+xml"
	ctCore      = "application/vnd.openxmlformats-package.core-properties+xml"
	ctApp       = "application/vnd.openxmlformats-officedocument.extended-properties+xml"
)

// DocxOptions configura un document nou.
type DocxOptions struct {
	Title       string
	Subtitle    string
	Author      string
	Page        string // "a4" (per defecte) o "letter"
	Landscape   bool
	Header      string // text de la capçalera ("" = el títol)
	NoHeader    bool
	PageNumbers bool
	Theme       Theme
	BaseDir     string // per resoldre rutes d'imatges relatives
}

// Docx edita o construeix un document de Word.
type Docx struct {
	Pkg     *Package
	Main    string
	Doc     *Node
	Body    *Node
	Theme   Theme
	BaseDir string

	styles    map[string]string // rol → styleId
	numAbs    map[string]int    // "bullet"/"decimal" → abstractNumId
	bulletNum int
	listNums  map[int]int
	nextDrawn int
	textWidth int // DXA
}

// ---------- obrir / crear ----------

// OpenDocx obre un document existent per editar-lo.
func OpenDocx(pk *Package) (*Docx, error) {
	main := pk.MainPart()
	if !strings.HasPrefix(main, "word/") {
		return nil, fmt.Errorf("no és un document de Word")
	}
	doc, err := pk.XML(main)
	if err != nil {
		return nil, err
	}
	body := doc.Root().Child("w:body")
	if body == nil {
		return nil, fmt.Errorf("document sense w:body")
	}
	d := &Docx{Pkg: pk, Main: main, Doc: doc, Body: body, Theme: GetTheme(""), listNums: map[int]int{}}
	d.textWidth = d.computeTextWidth()
	d.nextDrawn = d.maxDocPrID() + 1
	return d, nil
}

// NewDocx crea un document nou amb estils, numeració, capçalera/peu i
// pàgina configurats segons el tema.
func NewDocx(o DocxOptions) *Docx {
	if o.Theme.Name == "" {
		o.Theme = GetTheme("")
	}
	pk := New()
	pk.Put(ctPart, []byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`+"\n"+
		`<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">`+
		`<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>`+
		`<Default Extension="xml" ContentType="application/xml"/>`+
		`<Default Extension="png" ContentType="image/png"/>`+
		`<Default Extension="jpeg" ContentType="image/jpeg"/>`+
		`<Default Extension="jpg" ContentType="image/jpeg"/>`+
		`<Default Extension="gif" ContentType="image/gif"/>`+
		`</Types>`))
	pk.SetOverride("word/document.xml", ctDocxMain)
	pk.SetOverride("word/styles.xml", ctStyles)
	pk.SetOverride("word/numbering.xml", ctNumbering)
	pk.SetOverride("word/settings.xml", ctSettings)
	pk.SetOverride("docProps/core.xml", ctCore)
	pk.SetOverride("docProps/app.xml", ctApp)
	pk.AddRel("", RelOfficeDoc, "word/document.xml")
	pk.AddRel("", RelCoreProps, "docProps/core.xml")
	pk.AddRel("", RelExtProps, "docProps/app.xml")
	pk.Put("docProps/core.xml", []byte(coreProps(o.Title, o.Author)))
	pk.Put("docProps/app.xml", []byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`+"\n"+
		`<Properties xmlns="http://schemas.openxmlformats.org/officeDocument/2006/extended-properties"><Application>Gregal</Application></Properties>`))
	pk.Put("word/styles.xml", []byte(defaultStylesXML(o.Theme)))
	pk.Put("word/numbering.xml", []byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`+"\n"+`<w:numbering xmlns:w="`+nsW+`"/>`))
	pk.Put("word/settings.xml", []byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`+"\n"+
		`<w:settings xmlns:w="`+nsW+`"><w:defaultTabStop w:val="708"/><w:characterSpacingControl w:val="doNotCompress"/><w:compat><w:compatSetting w:name="compatibilityMode" w:uri="http://schemas.microsoft.com/office/word" w:val="15"/></w:compat></w:settings>`))
	pk.AddRel("word/document.xml", RelStyles, "word/styles.xml")
	pk.AddRel("word/document.xml", RelNumbering, "word/numbering.xml")
	pk.AddRel("word/document.xml", RelSettings, "word/settings.xml")

	w, h := 11906, 16838
	if strings.EqualFold(o.Page, "letter") {
		w, h = 12240, 15840
	}
	orient := ""
	if o.Landscape {
		w, h = h, w
		orient = ` w:orient="landscape"`
	}
	sect := fmt.Sprintf(`<w:sectPr>%%s<w:pgSz w:w="%d" w:h="%d"%s/><w:pgMar w:top="1418" w:right="1304" w:bottom="1418" w:left="1304" w:header="709" w:footer="709" w:gutter="0"/><w:cols w:space="708"/></w:sectPr>`, w, h, orient)
	refs := ""
	header := o.Header
	if header == "" {
		header = o.Title
	}
	if !o.NoHeader && header != "" {
		pk.SetOverride("word/header1.xml", ctHeader)
		id := pk.AddRel("word/document.xml", RelHeader, "word/header1.xml")
		pk.Put("word/header1.xml", []byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`+"\n"+
			`<w:hdr xmlns:w="`+nsW+`" xmlns:r="`+nsR+`"><w:p><w:pPr><w:pStyle w:val="Header"/><w:jc w:val="right"/></w:pPr><w:r><w:t xml:space="preserve">`+Esc(header)+`</w:t></w:r></w:p></w:hdr>`))
		refs += `<w:headerReference w:type="default" r:id="` + id + `"/>`
	}
	if o.PageNumbers {
		pk.SetOverride("word/footer1.xml", ctFooter)
		id := pk.AddRel("word/document.xml", RelFooter, "word/footer1.xml")
		pk.Put("word/footer1.xml", []byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`+"\n"+
			`<w:ftr xmlns:w="`+nsW+`" xmlns:r="`+nsR+`"><w:p><w:pPr><w:pStyle w:val="Footer"/><w:jc w:val="center"/></w:pPr>`+
			`<w:r><w:fldChar w:fldCharType="begin"/></w:r><w:r><w:instrText xml:space="preserve"> PAGE </w:instrText></w:r><w:r><w:fldChar w:fldCharType="separate"/></w:r><w:r><w:t>1</w:t></w:r><w:r><w:fldChar w:fldCharType="end"/></w:r>`+
			`<w:r><w:t xml:space="preserve"> / </w:t></w:r>`+
			`<w:r><w:fldChar w:fldCharType="begin"/></w:r><w:r><w:instrText xml:space="preserve"> NUMPAGES </w:instrText></w:r><w:r><w:fldChar w:fldCharType="separate"/></w:r><w:r><w:t>1</w:t></w:r><w:r><w:fldChar w:fldCharType="end"/></w:r>`+
			`</w:p></w:ftr>`))
		refs += `<w:footerReference w:type="default" r:id="` + id + `"/>`
	}
	pk.Put("word/document.xml", []byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`+"\n"+
		`<w:document xmlns:w="`+nsW+`" xmlns:r="`+nsR+`" xmlns:wp="`+nsWP+`" xmlns:a="`+nsA+`" xmlns:pic="`+nsPic+`"><w:body>`+
		fmt.Sprintf(sect, refs)+`</w:body></w:document>`))
	d, err := OpenDocx(pk)
	if err != nil {
		panic(err) // esquelet propi: no pot fallar
	}
	d.Theme = o.Theme
	d.BaseDir = o.BaseDir
	if o.Title != "" {
		d.AppendBlocksRaw(d.para("Title", PlainInlines(o.Title)))
	}
	if o.Subtitle != "" {
		d.AppendBlocksRaw(d.para("Subtitle", PlainInlines(o.Subtitle)))
	}
	return d
}

// DocxFromTemplate parteix d'una plantilla (.dotx/.docx): conserva
// estils, tema, capçaleres, peus i configuració de pàgina.
func DocxFromTemplate(tpl string) (*Docx, error) {
	pk, err := Open(tpl)
	if err != nil {
		return nil, err
	}
	pk.ToDocument()
	d, err := OpenDocx(pk)
	if err != nil {
		return nil, err
	}
	d.Theme = d.ThemeFromStyles()
	return d, nil
}

// ThemeFromStyles adapta el tema (colors de taules, vinyetes) als
// colors de títols del document, perquè el que s'hi afegeix encaixi
// amb la plantilla.
func (d *Docx) ThemeFromStyles() Theme {
	t := GetTheme("")
	sd, err := d.Pkg.XML(d.stylesPart())
	if err != nil {
		return t
	}
	color := func(id string) string {
		for _, s := range sd.Root().Elems("w:style") {
			if s.Get("w:styleId") == id {
				if c := s.Path("w:rPr", "w:color"); c != nil {
					if v := c.Get("w:val"); len(v) == 6 && v != "000000" && !strings.EqualFold(v, "auto") {
						return strings.ToUpper(v)
					}
				}
			}
		}
		return ""
	}
	ids := d.StyleIDs()
	if c := color(ids["Heading1"]); c != "" {
		t.Primary, t.Accent = c, c
	}
	if c := color(ids["Heading2"]); c != "" && c != t.Primary {
		t.Accent = c
	}
	if c := color(ids["Title"]); c != "" {
		t.Dark = c
	}
	t.Light = tint(t.Primary)
	t.Name = "template"
	return t
}

// tint aclareix un color (90% cap al blanc) per a fons de files.
func tint(hex string) string {
	if len(hex) != 6 {
		return "F2F2F2"
	}
	v, err := strconv.ParseUint(hex, 16, 32)
	if err != nil {
		return "F2F2F2"
	}
	r, g, b := (v>>16)&0xFF, (v>>8)&0xFF, v&0xFF
	mix := func(c uint64) uint64 { return c + (255-c)*9/10 }
	return fmt.Sprintf("%02X%02X%02X", mix(r), mix(g), mix(b))
}

func coreProps(title, author string) string {
	if author == "" {
		author = "Gregal"
	}
	now := time.Now().UTC().Format(time.RFC3339)
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\n" +
		`<cp:coreProperties xmlns:cp="http://schemas.openxmlformats.org/package/2006/metadata/core-properties" xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:dcterms="http://purl.org/dc/terms/" xmlns:dcmitype="http://purl.org/dc/dcmitype/" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance">` +
		`<dc:title>` + Esc(title) + `</dc:title><dc:creator>` + Esc(author) + `</dc:creator>` +
		`<dcterms:created xsi:type="dcterms:W3CDTF">` + now + `</dcterms:created><dcterms:modified xsi:type="dcterms:W3CDTF">` + now + `</dcterms:modified>` +
		`</cp:coreProperties>`
}

// ---------- estils ----------

func defaultStylesXML(t Theme) string {
	font := func(f string) string {
		return `<w:rFonts w:ascii="` + f + `" w:hAnsi="` + f + `" w:eastAsia="` + f + `" w:cs="` + f + `"/>`
	}
	h := func(id, name string, lvl, sz int, color string, bold, italic bool, before, after int) string {
		b, i := "", ""
		if bold {
			b = "<w:b/><w:bCs/>"
		}
		if italic {
			i = "<w:i/><w:iCs/>"
		}
		return `<w:style w:type="paragraph" w:styleId="` + id + `"><w:name w:val="` + name + `"/><w:basedOn w:val="Normal"/><w:next w:val="Normal"/><w:uiPriority w:val="9"/><w:qFormat/>` +
			`<w:pPr><w:keepNext/><w:keepLines/><w:spacing w:before="` + strconv.Itoa(before) + `" w:after="` + strconv.Itoa(after) + `"/><w:outlineLvl w:val="` + strconv.Itoa(lvl) + `"/></w:pPr>` +
			`<w:rPr>` + font(t.HeadingFont) + b + i + `<w:color w:val="` + color + `"/><w:sz w:val="` + strconv.Itoa(sz) + `"/><w:szCs w:val="` + strconv.Itoa(sz) + `"/></w:rPr></w:style>`
	}
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\n" +
		`<w:styles xmlns:w="` + nsW + `">` +
		`<w:docDefaults><w:rPrDefault><w:rPr>` + font(t.BodyFont) + `<w:color w:val="` + t.Text + `"/><w:sz w:val="22"/><w:szCs w:val="22"/><w:lang w:val="ca-ES" w:eastAsia="en-US" w:bidi="ar-SA"/></w:rPr></w:rPrDefault>` +
		`<w:pPrDefault><w:pPr><w:spacing w:after="140" w:line="276" w:lineRule="auto"/></w:pPr></w:pPrDefault></w:docDefaults>` +
		`<w:style w:type="paragraph" w:default="1" w:styleId="Normal"><w:name w:val="Normal"/><w:qFormat/></w:style>` +
		`<w:style w:type="character" w:default="1" w:styleId="DefaultParagraphFont"><w:name w:val="Default Paragraph Font"/><w:uiPriority w:val="1"/><w:semiHidden/></w:style>` +
		`<w:style w:type="paragraph" w:styleId="Title"><w:name w:val="Title"/><w:basedOn w:val="Normal"/><w:next w:val="Normal"/><w:qFormat/><w:pPr><w:spacing w:after="80"/><w:contextualSpacing/></w:pPr><w:rPr>` + font(t.HeadingFont) + `<w:b/><w:color w:val="` + t.Dark + `"/><w:sz w:val="56"/><w:szCs w:val="56"/></w:rPr></w:style>` +
		`<w:style w:type="paragraph" w:styleId="Subtitle"><w:name w:val="Subtitle"/><w:basedOn w:val="Normal"/><w:next w:val="Normal"/><w:qFormat/><w:pPr><w:spacing w:after="360"/><w:pBdr><w:bottom w:val="single" w:sz="8" w:space="8" w:color="` + t.Accent + `"/></w:pBdr></w:pPr><w:rPr><w:color w:val="` + t.Muted + `"/><w:sz w:val="28"/><w:szCs w:val="28"/></w:rPr></w:style>` +
		h("Heading1", "heading 1", 0, 34, t.Primary, true, false, 400, 140) +
		h("Heading2", "heading 2", 1, 28, t.Primary, true, false, 300, 100) +
		h("Heading3", "heading 3", 2, 24, t.Dark, true, false, 240, 80) +
		h("Heading4", "heading 4", 3, 22, t.Dark, true, true, 200, 60) +
		h("Heading5", "heading 5", 4, 22, t.Muted, true, false, 200, 60) +
		h("Heading6", "heading 6", 5, 22, t.Muted, false, true, 200, 60) +
		`<w:style w:type="paragraph" w:styleId="ListParagraph"><w:name w:val="List Paragraph"/><w:basedOn w:val="Normal"/><w:uiPriority w:val="34"/><w:qFormat/><w:pPr><w:spacing w:after="60"/><w:ind w:left="720"/><w:contextualSpacing/></w:pPr></w:style>` +
		`<w:style w:type="paragraph" w:styleId="Quote"><w:name w:val="Quote"/><w:basedOn w:val="Normal"/><w:next w:val="Normal"/><w:qFormat/><w:pPr><w:spacing w:before="120" w:after="200"/><w:ind w:left="567" w:right="567"/><w:pBdr><w:left w:val="single" w:sz="18" w:space="12" w:color="` + t.Accent + `"/></w:pBdr></w:pPr><w:rPr><w:i/><w:iCs/><w:color w:val="` + t.Muted + `"/></w:rPr></w:style>` +
		`<w:style w:type="paragraph" w:styleId="Code"><w:name w:val="Code"/><w:basedOn w:val="Normal"/><w:qFormat/><w:pPr><w:spacing w:after="0" w:line="240" w:lineRule="auto"/><w:shd w:val="clear" w:color="auto" w:fill="` + t.Light + `"/><w:ind w:left="170" w:right="170"/></w:pPr><w:rPr>` + font("Courier New") + `<w:sz w:val="19"/><w:szCs w:val="19"/></w:rPr></w:style>` +
		`<w:style w:type="paragraph" w:styleId="Caption"><w:name w:val="caption"/><w:basedOn w:val="Normal"/><w:next w:val="Normal"/><w:qFormat/><w:pPr><w:jc w:val="center"/><w:spacing w:after="240"/></w:pPr><w:rPr><w:i/><w:color w:val="` + t.Muted + `"/><w:sz w:val="18"/></w:rPr></w:style>` +
		`<w:style w:type="paragraph" w:styleId="Header"><w:name w:val="header"/><w:basedOn w:val="Normal"/><w:pPr><w:spacing w:after="0"/></w:pPr><w:rPr><w:color w:val="` + t.Muted + `"/><w:sz w:val="18"/></w:rPr></w:style>` +
		`<w:style w:type="paragraph" w:styleId="Footer"><w:name w:val="footer"/><w:basedOn w:val="Normal"/><w:pPr><w:spacing w:after="0"/></w:pPr><w:rPr><w:color w:val="` + t.Muted + `"/><w:sz w:val="18"/></w:rPr></w:style>` +
		`<w:style w:type="paragraph" w:styleId="TOCHeading"><w:name w:val="TOC Heading"/><w:basedOn w:val="Heading1"/><w:next w:val="Normal"/><w:qFormat/><w:pPr><w:outlineLvl w:val="9"/></w:pPr></w:style>` +
		`<w:style w:type="character" w:styleId="Hyperlink"><w:name w:val="Hyperlink"/><w:rPr><w:color w:val="` + t.Accent + `"/><w:u w:val="single"/></w:rPr></w:style>` +
		`<w:style w:type="character" w:styleId="CodeChar"><w:name w:val="Code Char"/><w:rPr>` + font("Courier New") + `<w:sz w:val="20"/><w:shd w:val="clear" w:color="auto" w:fill="` + t.Light + `"/></w:rPr></w:style>` +
		`<w:style w:type="table" w:default="1" w:styleId="TableNormal"><w:name w:val="Normal Table"/><w:semiHidden/><w:tblPr><w:tblInd w:w="0" w:type="dxa"/><w:tblCellMar><w:top w:w="0" w:type="dxa"/><w:left w:w="108" w:type="dxa"/><w:bottom w:w="0" w:type="dxa"/><w:right w:w="108" w:type="dxa"/></w:tblCellMar></w:tblPr></w:style>` +
		`<w:style w:type="table" w:styleId="TableGrid"><w:name w:val="Table Grid"/><w:basedOn w:val="TableNormal"/><w:pPr><w:spacing w:after="0" w:line="240" w:lineRule="auto"/></w:pPr><w:tblPr><w:tblBorders>` +
		`<w:top w:val="single" w:sz="4" w:space="0" w:color="BFBFBF"/><w:left w:val="single" w:sz="4" w:space="0" w:color="BFBFBF"/><w:bottom w:val="single" w:sz="4" w:space="0" w:color="BFBFBF"/><w:right w:val="single" w:sz="4" w:space="0" w:color="BFBFBF"/><w:insideH w:val="single" w:sz="4" w:space="0" w:color="BFBFBF"/><w:insideV w:val="single" w:sz="4" w:space="0" w:color="BFBFBF"/>` +
		`</w:tblBorders><w:tblCellMar><w:top w:w="57" w:type="dxa"/><w:left w:w="108" w:type="dxa"/><w:bottom w:w="57" w:type="dxa"/><w:right w:w="108" w:type="dxa"/></w:tblCellMar></w:tblPr></w:style>` +
		`</w:styles>`
}

// roleNames: noms canònics (w:name, sempre en anglès encara que el
// Word estigui en català) de cada rol que fa servir el constructor.
var roleNames = map[string][]string{
	"Title":         {"title"},
	"Subtitle":      {"subtitle"},
	"Heading1":      {"heading 1"},
	"Heading2":      {"heading 2"},
	"Heading3":      {"heading 3"},
	"Heading4":      {"heading 4"},
	"Heading5":      {"heading 5"},
	"Heading6":      {"heading 6"},
	"ListParagraph": {"list paragraph"},
	"Quote":         {"quote", "intense quote", "block text"},
	"Code":          {"code", "source code", "html preformatted", "plain text"},
	"Caption":       {"caption"},
	"Hyperlink":     {"hyperlink"},
	"CodeChar":      {"code char", "html code", "verbatim char"},
	"TableGrid":     {"table grid"},
	"TOCHeading":    {"toc heading"},
}

// style torna l'id d'estil per a un rol: el de la plantilla si en té un
// (per nom canònic o per nivell d'esquema), si no l'afegeix des dels
// estils propis perquè el document no perdi format.
func (d *Docx) style(role string) string {
	if d.styles == nil {
		d.loadStyles()
	}
	if id, ok := d.styles[role]; ok {
		return id
	}
	// No existeix: copia la definició pròpia.
	stPart := d.stylesPart()
	sd, err := d.Pkg.XML(stPart)
	if err != nil {
		return ""
	}
	own := MustParse(defaultStylesXML(d.Theme)).Root()
	for _, s := range own.Elems("w:style") {
		if s.Get("w:styleId") == role {
			c := s.Clone()
			// En una plantilla, no imposis tipus de lletra: que hereti.
			if role != "Code" && role != "CodeChar" {
				if rp := c.Child("w:rPr"); rp != nil {
					if f := rp.Child("w:rFonts"); f != nil {
						f.Remove()
					}
				}
			}
			sd.Root().Append(c)
			d.styles[role] = role
			return role
		}
	}
	return ""
}

func (d *Docx) stylesPart() string {
	if ps := d.Pkg.ByType(d.Main, RelStyles); len(ps) > 0 {
		return ps[0]
	}
	d.Pkg.Put("word/styles.xml", []byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`+"\n"+`<w:styles xmlns:w="`+nsW+`"/>`))
	d.Pkg.SetOverride("word/styles.xml", ctStyles)
	d.Pkg.AddRel(d.Main, RelStyles, "word/styles.xml")
	return "word/styles.xml"
}

func (d *Docx) loadStyles() {
	d.styles = map[string]string{}
	sd, err := d.Pkg.XML(d.stylesPart())
	if err != nil {
		return
	}
	byName := map[string]string{}
	outline := map[int]string{}
	for _, s := range sd.Root().Elems("w:style") {
		id := s.Get("w:styleId")
		name := strings.ToLower(s.Child("w:name").Get("w:val"))
		if name != "" {
			byName[name] = id
		}
		if s.Get("w:type") == "paragraph" {
			if ol := s.Path("w:pPr", "w:outlineLvl"); ol != nil {
				if n, err := strconv.Atoi(ol.Get("w:val")); err == nil && n < 9 {
					if _, ok := outline[n]; !ok {
						outline[n] = id
					}
				}
			}
		}
	}
	for role, names := range roleNames {
		for _, n := range names {
			if id, ok := byName[n]; ok {
				d.styles[role] = id
				break
			}
		}
	}
	for i := 1; i <= 6; i++ {
		role := "Heading" + strconv.Itoa(i)
		if _, ok := d.styles[role]; !ok {
			if id, ok := outline[i-1]; ok {
				d.styles[role] = id
			}
		}
	}
}

// StyleIDs torna el mapa rol → id d'estil resolt (per informar).
func (d *Docx) StyleIDs() map[string]string {
	if d.styles == nil {
		d.loadStyles()
	}
	return d.styles
}

// ---------- numeració ----------

func (d *Docx) numberingDoc() *Node {
	var part string
	if ps := d.Pkg.ByType(d.Main, RelNumbering); len(ps) > 0 {
		part = ps[0]
	} else {
		part = "word/numbering.xml"
		d.Pkg.Put(part, []byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`+"\n"+`<w:numbering xmlns:w="`+nsW+`"/>`))
		d.Pkg.SetOverride(part, ctNumbering)
		d.Pkg.AddRel(d.Main, RelNumbering, part)
	}
	n, err := d.Pkg.XML(part)
	if err != nil {
		return nil
	}
	return n.Root()
}

func (d *Docx) abstractNum(kind string) int {
	if d.numAbs == nil {
		d.numAbs = map[string]int{}
	}
	if id, ok := d.numAbs[kind]; ok {
		return id
	}
	nb := d.numberingDoc()
	maxID := -1
	for _, a := range nb.Elems("w:abstractNum") {
		if n, err := strconv.Atoi(a.Get("w:abstractNumId")); err == nil && n > maxID {
			maxID = n
		}
	}
	id := maxID + 1
	var lv strings.Builder
	bullets := []string{"•", "◦", "▪"}
	fmts := []string{"decimal", "lowerLetter", "lowerRoman"}
	for i := 0; i < 9; i++ {
		ind := 720 + 360*i
		if kind == "bullet" {
			lv.WriteString(fmt.Sprintf(`<w:lvl w:ilvl="%d"><w:start w:val="1"/><w:numFmt w:val="bullet"/><w:lvlText w:val="%s"/><w:lvlJc w:val="left"/><w:pPr><w:ind w:left="%d" w:hanging="360"/></w:pPr><w:rPr><w:rFonts w:ascii="Arial" w:hAnsi="Arial"/><w:color w:val="%s"/></w:rPr></w:lvl>`, i, bullets[i%3], ind, d.Theme.Accent))
		} else {
			lv.WriteString(fmt.Sprintf(`<w:lvl w:ilvl="%d"><w:start w:val="1"/><w:numFmt w:val="%s"/><w:lvlText w:val="%%%d."/><w:lvlJc w:val="left"/><w:pPr><w:ind w:left="%d" w:hanging="360"/></w:pPr></w:lvl>`, i, fmts[i%3], i+1, ind))
		}
	}
	frag, _ := ParseFragment(fmt.Sprintf(`<w:abstractNum xmlns:w="%s" w:abstractNumId="%d"><w:multiLevelType w:val="hybridMultilevel"/>%s</w:abstractNum>`, nsW, id, lv.String()))
	a := frag[0]
	a.Del("xmlns:w")
	// Els abstractNum van abans de tots els w:num.
	if first := nb.Child("w:num"); first != nil {
		first.InsertBefore(a)
	} else {
		nb.Append(a)
	}
	d.numAbs[kind] = id
	return id
}

func (d *Docx) newNum(kind string) int {
	abs := d.abstractNum(kind)
	nb := d.numberingDoc()
	maxID := 0
	for _, n := range nb.Elems("w:num") {
		if v, err := strconv.Atoi(n.Get("w:numId")); err == nil && v > maxID {
			maxID = v
		}
	}
	id := maxID + 1
	num := El("w:num", "w:numId", strconv.Itoa(id)).Add(El("w:abstractNumId", "w:val", strconv.Itoa(abs)))
	if kind == "decimal" {
		num.Add(El("w:lvlOverride", "w:ilvl", "0").Add(El("w:startOverride", "w:val", "1")))
	}
	// w:num va abans de w:numIdMacAtCleanup si n'hi ha.
	if c := nb.Child("w:numIdMacAtCleanup"); c != nil {
		c.InsertBefore(num)
	} else {
		nb.Append(num)
	}
	return id
}

// ---------- blocs → XML ----------

// Blocks converteix blocs en nodes del cos (w:p, w:tbl).
func (d *Docx) Blocks(bs []Block) []*Node {
	var out []*Node
	var heads []Block
	for _, b := range bs {
		if b.Kind == BHeading {
			heads = append(heads, b)
		}
	}
	for _, b := range bs {
		switch b.Kind {
		case BHeading:
			lvl := b.Level
			if lvl < 1 {
				lvl = 1
			}
			if lvl > 6 {
				lvl = 6
			}
			out = append(out, d.para("Heading"+strconv.Itoa(lvl), b.Inlines))
		case BPara:
			out = append(out, d.para("", b.Inlines))
		case BQuote:
			out = append(out, d.para("Quote", b.Inlines))
		case BBullet, BNumbered:
			var numID int
			if b.Kind == BBullet {
				if d.bulletNum == 0 {
					d.bulletNum = d.newNum("bullet")
				}
				numID = d.bulletNum
			} else {
				if d.listNums == nil {
					d.listNums = map[int]int{}
				}
				if _, ok := d.listNums[b.ListID]; !ok {
					d.listNums[b.ListID] = d.newNum("decimal")
				}
				numID = d.listNums[b.ListID]
			}
			p := d.para("ListParagraph", b.Inlines)
			ppr := p.Child("w:pPr")
			ppr.EnsureChild("w:numPr", pPrOrder).Add(
				El("w:ilvl", "w:val", strconv.Itoa(b.Level)),
				El("w:numId", "w:val", strconv.Itoa(numID)))
			out = append(out, p)
		case BCode:
			lines := strings.Split(b.Text, "\n")
			for i, l := range lines {
				p := d.para("Code", []Inline{{Text: l}})
				if i == len(lines)-1 {
					p.Child("w:pPr").EnsureChild("w:spacing", pPrOrder).Set("w:after", "200")
				}
				out = append(out, p)
			}
		case BRule:
			p := d.para("", nil)
			p.Child("w:pPr").EnsureChild("w:pBdr", pPrOrder).Add(
				El("w:bottom", "w:val", "single", "w:sz", "6", "w:space", "1", "w:color", d.Theme.Muted))
			out = append(out, p)
		case BPageBreak:
			out = append(out, El("w:p").Add(El("w:r").Add(El("w:br", "w:type", "page"))))
		case BTOC:
			out = append(out, d.toc(heads)...)
		case BTable:
			out = append(out, d.table(b))
			out = append(out, El("w:p").Add(El("w:pPr").Add(El("w:spacing", "w:after", "0"))))
		case BImage:
			if p, err := d.imagePara(b.Image, b.Alt); err == nil {
				out = append(out, p)
				if b.Alt != "" {
					out = append(out, d.para("Caption", PlainInlines(b.Alt)))
				}
			} else {
				out = append(out, d.para("", PlainInlines(fmt.Sprintf("[imatge no trobada: %s]", b.Image))))
			}
		}
	}
	return out
}

// Ordre d'esquema dels fills de w:pPr i w:rPr (OOXML el valida).
var pPrOrder = []string{"w:pStyle", "w:keepNext", "w:keepLines", "w:pageBreakBefore", "w:framePr", "w:widowControl", "w:numPr", "w:suppressLineNumbers", "w:pBdr", "w:shd", "w:tabs", "w:suppressAutoHyphens", "w:kinsoku", "w:wordWrap", "w:overflowPunct", "w:topLinePunct", "w:autoSpaceDE", "w:autoSpaceDN", "w:bidi", "w:adjustRightInd", "w:snapToGrid", "w:spacing", "w:ind", "w:contextualSpacing", "w:mirrorIndents", "w:suppressOverlap", "w:jc", "w:textDirection", "w:textAlignment", "w:textboxTightWrap", "w:outlineLvl", "w:divId", "w:cnfStyle", "w:rPr", "w:sectPr", "w:pPrChange"}
var rPrOrder = []string{"w:ins", "w:del", "w:moveFrom", "w:moveTo", "w:rStyle", "w:rFonts", "w:b", "w:bCs", "w:i", "w:iCs", "w:caps", "w:smallCaps", "w:strike", "w:dstrike", "w:outline", "w:shadow", "w:emboss", "w:imprint", "w:noProof", "w:snapToGrid", "w:vanish", "w:webHidden", "w:color", "w:spacing", "w:w", "w:kern", "w:position", "w:sz", "w:szCs", "w:highlight", "w:u", "w:effect", "w:bdr", "w:shd", "w:fitText", "w:vertAlign", "w:rtl", "w:cs", "w:em", "w:lang", "w:eastAsianLayout", "w:specVanish", "w:oMath", "w:rPrChange"}

func (d *Docx) para(role string, ins []Inline) *Node {
	p := El("w:p")
	ppr := El("w:pPr")
	p.Add(ppr)
	if role != "" {
		if id := d.style(role); id != "" {
			ppr.Add(El("w:pStyle", "w:val", id))
		}
	}
	for _, in := range ins {
		p.Append(d.runs(in)...)
	}
	return p
}

// runs crea els w:r d'un inline (amb hipervincle si cal).
func (d *Docx) runs(in Inline) []*Node {
	var rs []*Node
	parts := strings.Split(in.Text, "\n")
	for i, part := range parts {
		r := El("w:r")
		rpr := El("w:rPr")
		if in.Code {
			if id := d.style("CodeChar"); id != "" {
				rpr.Add(El("w:rStyle", "w:val", id))
			}
		} else if in.Link != "" {
			if id := d.style("Hyperlink"); id != "" {
				rpr.Add(El("w:rStyle", "w:val", id))
			}
		}
		if in.Bold {
			rpr.Add(El("w:b"), El("w:bCs"))
		}
		if in.Italic {
			rpr.Add(El("w:i"), El("w:iCs"))
		}
		if in.Strike {
			rpr.Add(El("w:strike"))
		}
		if len(rpr.Children) > 0 {
			r.Add(rpr)
		}
		if i > 0 {
			r.Add(El("w:br"))
		}
		if part != "" {
			r.Add(El("w:t", "xml:space", "preserve").Add(Txt(part)))
		}
		rs = append(rs, r)
	}
	if in.Link != "" && (strings.HasPrefix(in.Link, "http") || strings.HasPrefix(in.Link, "mailto:")) {
		id := d.Pkg.AddExternalRel(d.Main, RelHyperlink, in.Link)
		h := El("w:hyperlink", "r:id", id, "w:history", "1")
		h.Append(rs...)
		return []*Node{h}
	}
	return rs
}

// toc crea un camp d'índex. El resultat en memòria ja porta els títols
// (sagnats per nivell), així es veu bé fins i tot sense actualitzar el
// camp (LibreOffice, vista prèvia); el Word el refà en obrir.
func (d *Docx) toc(heads []Block) []*Node {
	var out []*Node
	var entries []Block
	for _, h := range heads {
		if h.Level <= 3 {
			entries = append(entries, h)
		}
	}
	if len(entries) == 0 {
		entries = []Block{{Level: 1, Inlines: PlainInlines("Actualitza el camp (F9) per veure l'índex.")}}
	}
	for i, h := range entries {
		p := El("w:p")
		ppr := El("w:pPr").Add(
			El("w:tabs").Add(El("w:tab", "w:val", "right", "w:leader", "dot", "w:pos", strconv.Itoa(d.textWidth))),
			El("w:spacing", "w:after", "60"),
			El("w:ind", "w:left", strconv.Itoa(360*(h.Level-1))))
		p.Add(ppr)
		if i == 0 {
			p.Add(El("w:r").Add(El("w:fldChar", "w:fldCharType", "begin", "w:dirty", "true")))
			p.Add(El("w:r").Add(El("w:instrText", "xml:space", "preserve").Add(Txt(` TOC \o "1-3" \h \z \u `))))
			p.Add(El("w:r").Add(El("w:fldChar", "w:fldCharType", "separate")))
		}
		r := El("w:r")
		if h.Level == 1 {
			r.Add(El("w:rPr").Add(El("w:b")))
		}
		r.Add(El("w:t", "xml:space", "preserve").Add(Txt(InlinesText(h.Inlines))))
		p.Add(r)
		if i == len(entries)-1 {
			p.Add(El("w:r").Add(El("w:fldChar", "w:fldCharType", "end")))
		}
		out = append(out, p)
	}
	out = append(out, El("w:p"))
	// Que el Word actualitzi els camps en obrir.
	if sp := d.Pkg.ByType(d.Main, RelSettings); len(sp) > 0 {
		if sd, err := d.Pkg.XML(sp[0]); err == nil && sd.Root().Child("w:updateFields") == nil {
			sd.Root().EnsureChild("w:updateFields", settingsOrder).Set("w:val", "true")
		}
	}
	return out
}

var settingsOrder = []string{"w:writeProtection", "w:view", "w:zoom", "w:removePersonalInformation", "w:removeDateAndTime", "w:doNotDisplayPageBoundaries", "w:displayBackgroundShape", "w:printPostScriptOverText", "w:printFractionalCharacterWidth", "w:printFormsData", "w:embedTrueTypeFonts", "w:embedSystemFonts", "w:saveSubsetFonts", "w:saveFormsData", "w:mirrorMargins", "w:alignBordersAndEdges", "w:bordersDoNotSurroundHeader", "w:bordersDoNotSurroundFooter", "w:gutterAtTop", "w:hideSpellingErrors", "w:hideGrammaticalErrors", "w:activeWritingStyle", "w:proofState", "w:formsDesign", "w:attachedTemplate", "w:linkStyles", "w:stylePaneFormatFilter", "w:stylePaneSortMethod", "w:documentType", "w:mailMerge", "w:revisionView", "w:trackRevisions", "w:doNotTrackMoves", "w:doNotTrackFormatting", "w:documentProtection", "w:autoFormatOverride", "w:styleLockTheme", "w:styleLockQFSet", "w:defaultTabStop", "w:autoHyphenation", "w:consecutiveHyphenLimit", "w:hyphenationZone", "w:doNotHyphenateCaps", "w:showEnvelope", "w:summaryLength", "w:clickAndTypeStyle", "w:defaultTableStyle", "w:evenAndOddHeaders", "w:bookFoldRevPrinting", "w:bookFoldPrinting", "w:bookFoldPrintingSheets", "w:drawingGridHorizontalSpacing", "w:drawingGridVerticalSpacing", "w:displayHorizontalDrawingGridEvery", "w:displayVerticalDrawingGridEvery", "w:doNotUseMarginsForDrawingGridOrigin", "w:drawingGridHorizontalOrigin", "w:drawingGridVerticalOrigin", "w:doNotShadeFormData", "w:noPunctuationKerning", "w:characterSpacingControl", "w:printTwoOnOne", "w:strictFirstAndLastChars", "w:noLineBreaksAfter", "w:noLineBreaksBefore", "w:savePreviewPicture", "w:doNotValidateAgainstSchema", "w:saveInvalidXml", "w:ignoreMixedContent", "w:alwaysShowPlaceholderText", "w:doNotDemarcateInvalidXml", "w:saveXmlDataOnly", "w:useXSLTWhenSaving", "w:saveThroughXslt", "w:showXMLTags", "w:alwaysMergeEmptyNamespace", "w:updateFields", "w:hdrShapeDefaults", "w:footnotePr", "w:endnotePr", "w:compat", "w:docVars", "w:rsids", "m:mathPr", "w:attachedSchema", "w:themeFontLang", "w:clrSchemeMapping", "w:doNotIncludeSubdocsInStats", "w:doNotAutoCompressPictures", "w:forceUpgrade", "w:captions", "w:readModeInkLockDown", "w:smartTagType", "sl:schemaLibrary", "w:shapeDefaults", "w:doNotEmbedSmartTags", "w:decimalSymbol", "w:listSeparator"}

func isNumeric(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	s = strings.NewReplacer("€", "", "$", "", "%", "", " ", "", ".", "", ",", "", "−", "-", "+", "").Replace(s)
	s = strings.TrimPrefix(s, "-")
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func (d *Docx) table(b Block) *Node {
	ncol := 0
	for _, r := range b.Rows {
		if len(r) > ncol {
			ncol = len(r)
		}
	}
	if ncol == 0 {
		return El("w:p")
	}
	// Amplades proporcionals al contingut (mín. 8% per columna).
	weights := make([]float64, ncol)
	for _, r := range b.Rows {
		for i, c := range r {
			n := float64(utf8.RuneCountInString(InlinesText(c)))
			if n > 40 {
				n = 40
			}
			if n > weights[i] {
				weights[i] = n
			}
		}
	}
	total := 0.0
	for i := range weights {
		if weights[i] < 4 {
			weights[i] = 4
		}
		total += weights[i]
	}
	tw := d.textWidth
	widths := make([]int, ncol)
	sum := 0
	for i := range weights {
		widths[i] = int(float64(tw) * weights[i] / total)
		if lo := tw * 8 / 100; widths[i] < lo {
			widths[i] = lo
		}
		sum += widths[i]
	}
	widths[ncol-1] += tw - sum // que sumin exactament l'amplada
	// Columnes numèriques → a la dreta (si no s'ha dit res).
	align := make([]string, ncol)
	copy(align, b.Align)
	for i := 0; i < ncol; i++ {
		if align[i] != "" {
			continue
		}
		num, tot := 0, 0
		for _, r := range b.Rows[1:] {
			if i < len(r) && strings.TrimSpace(InlinesText(r[i])) != "" {
				tot++
				if isNumeric(InlinesText(r[i])) {
					num++
				}
			}
		}
		if tot > 0 && num == tot {
			align[i] = "right"
		}
	}

	tbl := El("w:tbl")
	tblPr := El("w:tblPr")
	if id := d.style("TableGrid"); id != "" {
		tblPr.Add(El("w:tblStyle", "w:val", id))
	}
	tblPr.Add(El("w:tblW", "w:w", strconv.Itoa(tw), "w:type", "dxa"),
		El("w:tblLayout", "w:type", "fixed"),
		El("w:tblLook", "w:val", "04A0", "w:firstRow", "1", "w:lastRow", "0", "w:firstColumn", "0", "w:lastColumn", "0", "w:noHBand", "0", "w:noVBand", "1"))
	tbl.Add(tblPr)
	grid := El("w:tblGrid")
	for _, w := range widths {
		grid.Add(El("w:gridCol", "w:w", strconv.Itoa(w)))
	}
	tbl.Add(grid)
	for ri, r := range b.Rows {
		tr := El("w:tr")
		if ri == 0 {
			tr.Add(El("w:trPr").Add(El("w:tblHeader")))
		}
		for ci := 0; ci < ncol; ci++ {
			var cell []Inline
			if ci < len(r) {
				cell = r[ci]
			}
			tc := El("w:tc")
			tcPr := El("w:tcPr").Add(El("w:tcW", "w:w", strconv.Itoa(widths[ci]), "w:type", "dxa"))
			switch {
			case ri == 0:
				tcPr.Add(El("w:shd", "w:val", "clear", "w:color", "auto", "w:fill", d.Theme.Primary))
			case ri%2 == 0:
				tcPr.Add(El("w:shd", "w:val", "clear", "w:color", "auto", "w:fill", d.Theme.Light))
			}
			tcPr.Add(El("w:vAlign", "w:val", "center"))
			tc.Add(tcPr)
			if ri == 0 {
				for i := range cell {
					cell[i].Bold = true
				}
			}
			p := d.para("", cell)
			ppr := p.Child("w:pPr")
			ppr.EnsureChild("w:spacing", pPrOrder).Set("w:after", "0")
			if align[ci] != "" {
				ppr.EnsureChild("w:jc", pPrOrder).Set("w:val", align[ci])
			}
			if ri == 0 {
				for _, run := range p.Find("w:r") {
					rpr := run.Child("w:rPr")
					if rpr == nil {
						rpr = El("w:rPr")
						run.Prepend(rpr)
					}
					rpr.EnsureChild("w:color", rPrOrder).Set("w:val", "FFFFFF")
				}
			}
			tc.Add(p)
			tr.Add(tc)
		}
		tbl.Add(tr)
	}
	return tbl
}

// ---------- imatges ----------

func (d *Docx) imagePara(src, alt string) (*Node, error) {
	path := src
	if !filepath.IsAbs(path) && d.BaseDir != "" {
		if _, err := os.Stat(filepath.Join(d.BaseDir, path)); err == nil {
			path = filepath.Join(d.BaseDir, path)
		}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("imatge no reconeguda (png/jpeg/gif): %v", err)
	}
	ext := map[string]string{"png": ".png", "jpeg": ".jpeg", "gif": ".gif"}[format]
	part := d.Pkg.NextPartName("word/media", "image", ext)
	d.Pkg.Put(part, data)
	d.Pkg.EnsureDefault(strings.TrimPrefix(ext, "."), "image/"+format)
	rid := d.Pkg.AddRel(d.Main, RelImage, part)
	// Mida: 96 ppp, limitada a l'amplada útil.
	const emuPerPx = 9525
	cx, cy := int64(cfg.Width)*emuPerPx, int64(cfg.Height)*emuPerPx
	maxCx := int64(d.textWidth) * 635 // 1 DXA = 635 EMU
	if cx > maxCx && cx > 0 {
		cy = cy * maxCx / cx
		cx = maxCx
	}
	id := d.nextDrawn
	d.nextDrawn++
	d.ensureNS("wp", nsWP)
	xml := fmt.Sprintf(`<w:p xmlns:w="%s" xmlns:r="%s" xmlns:wp="%s"><w:pPr><w:jc w:val="center"/></w:pPr><w:r><w:drawing><wp:inline distT="0" distB="0" distL="0" distR="0"><wp:extent cx="%d" cy="%d"/><wp:docPr id="%d" name="Imatge %d" descr="%s"/><wp:cNvGraphicFramePr><a:graphicFrameLocks xmlns:a="%s" noChangeAspect="1"/></wp:cNvGraphicFramePr><a:graphic xmlns:a="%s"><a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/picture"><pic:pic xmlns:pic="%s"><pic:nvPicPr><pic:cNvPr id="%d" name="%s"/><pic:cNvPicPr/></pic:nvPicPr><pic:blipFill><a:blip r:embed="%s"/><a:stretch><a:fillRect/></a:stretch></pic:blipFill><pic:spPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="%d" cy="%d"/></a:xfrm><a:prstGeom prst="rect"><a:avLst/></a:prstGeom></pic:spPr></pic:pic></a:graphicData></a:graphic></wp:inline></w:drawing></w:r></w:p>`,
		nsW, nsR, nsWP, cx, cy, id, id, Esc(alt), nsA, nsA, nsPic, id, Esc(filepath.Base(path)), rid, cx, cy)
	nodes, err := ParseFragment(xml)
	if err != nil {
		return nil, err
	}
	p := nodes[0]
	p.Del("xmlns:w")
	p.Del("xmlns:r")
	p.Del("xmlns:wp")
	return p, nil
}

func (d *Docx) ensureNS(prefix, uri string) {
	root := d.Doc.Root()
	if !root.Has("xmlns:" + prefix) {
		root.Set("xmlns:"+prefix, uri)
	}
	if !root.Has("xmlns:r") {
		root.Set("xmlns:r", nsR)
	}
}

func (d *Docx) maxDocPrID() int {
	m := 0
	for _, x := range d.Doc.Find("wp:docPr") {
		if n, err := strconv.Atoi(x.Get("id")); err == nil && n > m {
			m = n
		}
	}
	return m
}

func (d *Docx) computeTextWidth() int {
	sect := d.Body.Child("w:sectPr")
	if sect == nil {
		return 9026
	}
	w, _ := strconv.Atoi(sect.Child("w:pgSz").Get("w:w"))
	l, _ := strconv.Atoi(sect.Child("w:pgMar").Get("w:left"))
	r, _ := strconv.Atoi(sect.Child("w:pgMar").Get("w:right"))
	if tw := w - l - r; tw > 2000 {
		return tw
	}
	return 9026
}

// ---------- inserció ----------

// AppendBlocksRaw afegeix nodes al final del cos (abans del sectPr).
func (d *Docx) AppendBlocksRaw(nodes ...*Node) {
	if s := d.Body.Child("w:sectPr"); s != nil {
		s.InsertBefore(nodes...)
		return
	}
	d.Body.Append(nodes...)
}

// AppendMarkdown afegeix contingut markdown al final del document.
func (d *Docx) AppendMarkdown(md string) int {
	nodes := d.Blocks(ParseMarkdown(md))
	d.AppendBlocksRaw(nodes...)
	return len(nodes)
}

// ReplaceBody buida el cos (conserva el sectPr final) i hi posa el
// contingut. Per a plantilles sense marcadors.
func (d *Docx) ReplaceBody(md string) int {
	for _, c := range d.Body.Elems() {
		if c.Name != "w:sectPr" {
			c.Remove()
		}
	}
	return d.AppendMarkdown(md)
}

// Save desa el paquet.
func (d *Docx) Save(path string) error { return d.Pkg.Save(path) }
