package ooxml

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"
)

// MaxBytes és la mida màxima d'un paquet que s'accepta (comprimit).
const MaxBytes = 50 << 20

// maxPart limita cada part descomprimit (anti zip-bomb).
const maxPart = 200 << 20

// Tipus de relació habituals.
const (
	RelOfficeDoc   = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument"
	RelStyles      = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles"
	RelNumbering   = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/numbering"
	RelSettings    = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/settings"
	RelTheme       = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/theme"
	RelFontTable   = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/fontTable"
	RelHeader      = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/header"
	RelFooter      = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/footer"
	RelImage       = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/image"
	RelHyperlink   = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/hyperlink"
	RelComments    = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/comments"
	RelFootnotes   = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/footnotes"
	RelSlide       = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide"
	RelSlideLayout = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideLayout"
	RelSlideMaster = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideMaster"
	RelNotesSlide  = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/notesSlide"
	RelNotesMaster = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/notesMaster"
	RelChart       = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/chart"
	RelCoreProps   = "http://schemas.openxmlformats.org/package/2006/relationships/metadata/core-properties"
	RelExtProps    = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/extended-properties"
	RelPresProps   = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/presProps"
	RelViewProps   = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/viewProps"
	RelTableStyles = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/tableStyles"
)

// Package és un paquet OOXML en memòria. Els parts XML que es
// demanen amb XML() queden en memòria cau i es tornen a serialitzar
// en desar; la resta es copia byte a byte.
type Package struct {
	order   []string
	parts   map[string][]byte
	methods map[string]uint16
	trees   map[string]*Node
}

// New crea un paquet buit.
func New() *Package {
	return &Package{parts: map[string][]byte{}, methods: map[string]uint16{}, trees: map[string]*Node{}}
}

// Open llegeix un paquet del disc.
func Open(p string) (*Package, error) {
	fi, err := os.Stat(p)
	if err != nil {
		return nil, err
	}
	if fi.Size() > MaxBytes {
		return nil, fmt.Errorf("fitxer massa gran (%d MB, límit %d)", fi.Size()>>20, MaxBytes>>20)
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return nil, err
	}
	return OpenBytes(data)
}

// OpenBytes llegeix un paquet de memòria.
func OpenBytes(data []byte) (*Package, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("no és un zip office vàlid: %v", err)
	}
	pk := New()
	for _, f := range zr.File {
		if strings.HasSuffix(f.Name, "/") {
			continue
		}
		name := strings.TrimPrefix(f.Name, "/")
		if strings.Contains(name, "..") || strings.HasPrefix(name, "\\") || f.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("entrada sospitosa al zip: %q", f.Name)
		}
		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		b, err := io.ReadAll(io.LimitReader(rc, maxPart+1))
		rc.Close()
		if err != nil {
			return nil, err
		}
		if len(b) > maxPart {
			return nil, fmt.Errorf("part %s massa gran", name)
		}
		pk.order = append(pk.order, name)
		pk.parts[name] = b
		pk.methods[name] = f.Method
	}
	if _, ok := pk.parts["[Content_Types].xml"]; !ok {
		// Alguns generadors mínims ometen [Content_Types].xml: es llegeix
		// igual si hi ha el part principal (Validate ho farà notar).
		if pk.MainPart() == "" {
			return nil, fmt.Errorf("no és un paquet office: falta [Content_Types].xml")
		}
	}
	return pk, nil
}

// Names torna els noms de part en l'ordre del zip.
func (pk *Package) Names() []string { return append([]string(nil), pk.order...) }

// Has diu si el part existeix.
func (pk *Package) Has(name string) bool {
	_, ok := pk.parts[name]
	if !ok {
		_, ok = pk.trees[name]
	}
	return ok
}

// Raw torna els bytes d'un part (serialitzant l'arbre si s'ha tocat).
func (pk *Package) Raw(name string) ([]byte, bool) {
	if t, ok := pk.trees[name]; ok {
		return t.Bytes(), true
	}
	b, ok := pk.parts[name]
	return b, ok
}

// XML torna l'arbre d'un part (el parseja una sola vegada). Els canvis
// a l'arbre es desen amb Save.
func (pk *Package) XML(name string) (*Node, error) {
	if t, ok := pk.trees[name]; ok {
		return t, nil
	}
	b, ok := pk.parts[name]
	if !ok {
		return nil, fmt.Errorf("part %s no trobat", name)
	}
	t, err := Parse(b)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	pk.trees[name] = t
	return t, nil
}

// Put escriu un part (bytes). Si era nou, va al final.
func (pk *Package) Put(name string, data []byte) {
	if !pk.Has(name) {
		pk.order = append(pk.order, name)
	}
	delete(pk.trees, name)
	pk.parts[name] = data
}

// PutXML escriu un part a partir d'un arbre.
func (pk *Package) PutXML(name string, doc *Node) {
	if !pk.Has(name) {
		pk.order = append(pk.order, name)
	}
	pk.trees[name] = doc
	delete(pk.parts, name)
}

// Delete esborra un part (i el seu .rels, si en té).
func (pk *Package) Delete(name string) {
	delete(pk.parts, name)
	delete(pk.trees, name)
	delete(pk.methods, name)
	out := pk.order[:0]
	for _, n := range pk.order {
		if n != name {
			out = append(out, n)
		}
	}
	pk.order = out
}

// Bytes empaqueta el zip ([Content_Types].xml primer, com fa Office).
func (pk *Package) Bytes() ([]byte, error) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	names := pk.Names()
	sort.SliceStable(names, func(i, j int) bool {
		return names[i] == "[Content_Types].xml" && names[j] != "[Content_Types].xml"
	})
	for _, n := range names {
		data, _ := pk.Raw(n)
		m, ok := pk.methods[n]
		if !ok {
			m = zip.Deflate
		}
		w, err := zw.CreateHeader(&zip.FileHeader{Name: n, Method: m})
		if err != nil {
			return nil, err
		}
		if _, err := w.Write(data); err != nil {
			return nil, err
		}
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Save desa de manera atòmica (fitxer temporal + rename).
func (pk *Package) Save(p string) error {
	data, err := pk.Bytes()
	if err != nil {
		return err
	}
	tmp := p + ".gregal-tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, p); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("no s'ha pogut desar (fitxer bloquejat? tanca'l al Word/Excel/PowerPoint): %v", err)
	}
	return nil
}

// ---------- [Content_Types].xml ----------

const ctPart = "[Content_Types].xml"

// ContentType torna el tipus d'un part (Override o Default per extensió).
func (pk *Package) ContentType(part string) string {
	doc, err := pk.XML(ctPart)
	if err != nil {
		return ""
	}
	for _, o := range doc.Root().Elems("Override") {
		if strings.TrimPrefix(o.Get("PartName"), "/") == part {
			return o.Get("ContentType")
		}
	}
	ext := strings.TrimPrefix(path.Ext(part), ".")
	for _, d := range doc.Root().Elems("Default") {
		if strings.EqualFold(d.Get("Extension"), ext) {
			return d.Get("ContentType")
		}
	}
	return ""
}

// SetOverride declara (o canvia) el tipus d'un part.
func (pk *Package) SetOverride(part, ctype string) {
	doc, err := pk.XML(ctPart)
	if err != nil {
		return
	}
	pn := "/" + strings.TrimPrefix(part, "/")
	for _, o := range doc.Root().Elems("Override") {
		if o.Get("PartName") == pn {
			o.Set("ContentType", ctype)
			return
		}
	}
	doc.Root().Append(El("Override", "PartName", pn, "ContentType", ctype))
}

// RemoveOverride treu la declaració d'un part.
func (pk *Package) RemoveOverride(part string) {
	doc, err := pk.XML(ctPart)
	if err != nil {
		return
	}
	pn := "/" + strings.TrimPrefix(part, "/")
	for _, o := range doc.Root().Elems("Override") {
		if o.Get("PartName") == pn {
			o.Remove()
		}
	}
}

// EnsureDefault declara un tipus per extensió (png, jpeg…).
func (pk *Package) EnsureDefault(ext, ctype string) {
	doc, err := pk.XML(ctPart)
	if err != nil {
		return
	}
	for _, d := range doc.Root().Elems("Default") {
		if strings.EqualFold(d.Get("Extension"), ext) {
			return
		}
	}
	doc.Root().Prepend(El("Default", "Extension", ext, "ContentType", ctype))
}

// ---------- relacions ----------

// Rel és una relació d'un part.
type Rel struct {
	ID, Type, Target, Mode string
}

// RelsName torna el nom del .rels d'un part («» per al paquet).
func RelsName(part string) string {
	if part == "" {
		return "_rels/.rels"
	}
	dir, file := path.Split(part)
	return dir + "_rels/" + file + ".rels"
}

// Rels llegeix les relacions d'un part.
func (pk *Package) Rels(part string) []Rel {
	doc, err := pk.XML(RelsName(part))
	if err != nil {
		return nil
	}
	var out []Rel
	for _, r := range doc.Root().Elems("Relationship") {
		out = append(out, Rel{r.Get("Id"), r.Get("Type"), r.Get("Target"), r.Get("TargetMode")})
	}
	return out
}

// RelTarget resol la destinació d'una relació a un nom de part absolut.
func RelTarget(part, target string) string {
	if strings.HasPrefix(target, "/") {
		return strings.TrimPrefix(path.Clean(target), "/")
	}
	return strings.TrimPrefix(path.Clean(path.Join(path.Dir(part), target)), "/")
}

// RelPath torna la ruta relativa de from a to (per escriure Target).
func RelPath(from, to string) string {
	fd := strings.Split(path.Dir(from), "/")
	td := strings.Split(to, "/")
	if path.Dir(from) == "." {
		fd = nil
	}
	i := 0
	for i < len(fd) && i < len(td)-1 && fd[i] == td[i] {
		i++
	}
	var parts []string
	for j := i; j < len(fd); j++ {
		parts = append(parts, "..")
	}
	parts = append(parts, td[i:]...)
	return strings.Join(parts, "/")
}

// ByType torna els parts destinació de les relacions d'un tipus.
func (pk *Package) ByType(part, typ string) []string {
	var out []string
	for _, r := range pk.Rels(part) {
		if r.Type == typ && r.Mode != "External" {
			out = append(out, RelTarget(part, r.Target))
		}
	}
	return out
}

// RelByID resol un r:id d'un part al nom de part destinació.
func (pk *Package) RelByID(part, id string) (string, Rel, bool) {
	for _, r := range pk.Rels(part) {
		if r.ID == id {
			if r.Mode == "External" {
				return "", r, true
			}
			return RelTarget(part, r.Target), r, true
		}
	}
	return "", Rel{}, false
}

// AddRel afegeix una relació i en torna l'Id (rIdN lliure).
func (pk *Package) AddRel(part, typ, targetPart string) string {
	return pk.addRel(part, typ, RelPath(relBase(part), targetPart), "")
}

// AddExternalRel afegeix una relació externa (hipervincle).
func (pk *Package) AddExternalRel(part, typ, url string) string {
	return pk.addRel(part, typ, url, "External")
}

func relBase(part string) string {
	if part == "" {
		return "x"
	}
	return part
}

func (pk *Package) addRel(part, typ, target, mode string) string {
	rn := RelsName(part)
	doc, err := pk.XML(rn)
	if err != nil {
		doc = MustParse(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\n" +
			`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"></Relationships>`)
		doc.Root().Children = nil
		pk.PutXML(rn, doc)
	}
	used := map[string]bool{}
	for _, r := range doc.Root().Elems("Relationship") {
		used[r.Get("Id")] = true
	}
	id := ""
	for i := 1; ; i++ {
		id = "rId" + strconv.Itoa(i)
		if !used[id] {
			break
		}
	}
	r := El("Relationship", "Id", id, "Type", typ, "Target", target)
	if mode != "" {
		r.Set("TargetMode", mode)
	}
	doc.Root().Append(r)
	return id
}

// RemoveRel treu una relació per Id.
func (pk *Package) RemoveRel(part, id string) {
	doc, err := pk.XML(RelsName(part))
	if err != nil {
		return
	}
	for _, r := range doc.Root().Elems("Relationship") {
		if r.Get("Id") == id {
			r.Remove()
		}
	}
}

// MainPart torna el part principal (word/document.xml,
// ppt/presentation.xml, xl/workbook.xml).
func (pk *Package) MainPart() string {
	if ts := pk.ByType("", RelOfficeDoc); len(ts) > 0 {
		return ts[0]
	}
	for _, c := range []string{"word/document.xml", "ppt/presentation.xml", "xl/workbook.xml"} {
		if pk.Has(c) {
			return c
		}
	}
	return ""
}

// ---------- plantilles ----------

var templateTypes = map[string]string{
	"application/vnd.openxmlformats-officedocument.wordprocessingml.template.main+xml":   "application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml",
	"application/vnd.ms-word.template.macroEnabledTemplate.main+xml":                     "application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml",
	"application/vnd.openxmlformats-officedocument.presentationml.template.main+xml":     "application/vnd.openxmlformats-officedocument.presentationml.presentation.main+xml",
	"application/vnd.ms-powerpoint.template.macroEnabled.main+xml":                       "application/vnd.openxmlformats-officedocument.presentationml.presentation.main+xml",
	"application/vnd.openxmlformats-officedocument.spreadsheetml.template.main+xml":      "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml",
	"application/vnd.ms-excel.template.macroEnabled.main+xml":                            "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml",
	"application/vnd.ms-word.document.macroEnabled.main+xml":                             "application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml",
	"application/vnd.ms-powerpoint.presentation.macroEnabled.main+xml":                   "application/vnd.openxmlformats-officedocument.presentationml.presentation.main+xml",
}

// IsTemplate diu si el part principal és de plantilla (.dotx/.potx/.xltx).
func (pk *Package) IsTemplate() bool {
	ct := pk.ContentType(pk.MainPart())
	return strings.Contains(ct, ".template.") || strings.Contains(ct, "Template")
}

// ToDocument converteix una plantilla (o un fitxer amb macros) en un
// document normal: canvia el tipus del part principal i treu les macros
// (vbaProject), que un .docx/.pptx/.xlsx no pot dur.
func (pk *Package) ToDocument() {
	main := pk.MainPart()
	if nt, ok := templateTypes[pk.ContentType(main)]; ok {
		pk.SetOverride(main, nt)
	}
	for _, r := range pk.Rels(main) {
		if strings.HasSuffix(r.Type, "/vbaProject") {
			t := RelTarget(main, r.Target)
			pk.Delete(t)
			pk.Delete(RelsName(t))
			pk.RemoveOverride(t)
			pk.RemoveRel(main, r.ID)
		}
	}
	doc, err := pk.XML(ctPart)
	if err == nil {
		for _, d := range doc.Root().Elems("Default") {
			if strings.EqualFold(d.Get("Extension"), "bin") && strings.Contains(d.Get("ContentType"), "vbaProject") {
				d.Remove()
			}
		}
	}
}

// NextPartName torna el primer nom lliure amb el patró «dir/baseN.ext».
func (pk *Package) NextPartName(dir, base, ext string) string {
	for i := 1; ; i++ {
		n := fmt.Sprintf("%s/%s%d%s", dir, base, i, ext)
		if !pk.Has(n) {
			return n
		}
	}
}
