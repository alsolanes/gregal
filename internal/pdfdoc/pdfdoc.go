// Package pdfdoc llegeix i edita PDF sense binaris externs.
//
//   - Text: l'extractor propi de text.go (el de la indexació de
//     coneixement), amb un pas previ de desxifrat quan el PDF només té
//     contrasenya de propietari (el cas habitual de «protegit contra
//     còpia», que qualsevol visor obre).
//   - Edició: pdfcpu (Apache-2.0, Go pur): ajuntar, dividir, extreure i
//     reordenar pàgines, girar, esborrar, segells i text, formularis i
//     anotacions.
//
// Imatges de pàgina: no hi ha cap rasteritzador de PDF en Go pur amb
// llicència compatible; internal/officerender en fa servir un d'opcional
// (pdftoppm o el Windows.Data.Pdf integrat a Windows 10/11).
//
// Cada escriptura va a un temporal al mateix directori, es valida amb
// pdfcpu i només llavors substitueix el destí: un PDF a mig escriure o
// que pdfcpu no sap tornar a llegir no trepitja mai el bo.
package pdfdoc

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/color"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/types"
)

func init() {
	// pdfcpu, amb conf nil, crea ~/.config/pdfcpu (config.yml i fonts).
	// Gregal no en vol res: sempre configuració sense estat.
	model.ConfigPath = "disable"
}

// conf és una configuració nova per crida (pdfcpu en modifica camps).
func conf() *model.Configuration {
	c := model.NewStatelessConfiguration()
	c.ValidationMode = model.ValidationRelaxed
	return c
}

// IsPDF diu si la ruta té extensió .pdf.
func IsPDF(path string) bool { return strings.EqualFold(filepath.Ext(path), ".pdf") }

// MaxBytes és el topall de mida per llegir o editar (com les pujades).
const MaxBytes = 100 << 20

func load(path string) ([]byte, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if fi.IsDir() {
		return nil, fmt.Errorf("%s és un directori", path)
	}
	if fi.Size() > MaxBytes {
		return nil, fmt.Errorf("PDF massa gran (%d MB, màxim %d MB)", fi.Size()>>20, MaxBytes>>20)
	}
	return os.ReadFile(path)
}

// decrypted torna el PDF desxifrat si estava xifrat només amb
// contrasenya de propietari. Si demana contrasenya d'usuari: ErrEncrypted.
func decrypted(ctx context.Context, data []byte) ([]byte, bool, error) {
	if !looksEncrypted(data) {
		return data, false, nil
	}
	var out bytes.Buffer
	if err := api.Decrypt(ctx, bytes.NewReader(data), &out, conf()); err != nil {
		return nil, true, ErrEncrypted
	}
	return out.Bytes(), true, nil
}

func looksEncrypted(data []byte) bool {
	return bytes.Contains(data, []byte("/Encrypt"))
}

// ReadPages llegeix el text de totes les pàgines del fitxer.
func ReadPages(ctx context.Context, path string) ([]Page, error) {
	data, err := load(path)
	if err != nil {
		return nil, err
	}
	return ReadPagesBytes(ctx, data)
}

// ReadPagesBytes és ReadPages sobre bytes (la indexació de coneixement).
func ReadPagesBytes(ctx context.Context, data []byte) ([]Page, error) {
	plain, _, err := decrypted(ctx, data)
	if err != nil {
		return nil, err
	}
	pages, err := PagesText(plain)
	if err != nil && plain != nil && !errors.Is(err, ErrEncrypted) {
		// L'extractor propi no troba les pàgines (xref estranya,
		// actualitzacions incrementals trencades): pdfcpu el reescriu net.
		if fixed, ferr := rewrite(ctx, plain); ferr == nil {
			if p2, err2 := PagesText(fixed); err2 == nil {
				return p2, nil
			}
		}
	}
	return pages, err
}

// rewrite torna el PDF tal com el desa pdfcpu (sense object streams, que
// l'extractor llegeix però costen més).
func rewrite(ctx context.Context, data []byte) ([]byte, error) {
	c := conf()
	c.WriteObjectStream = false
	c.WriteXRefStream = false
	pctx, err := api.ReadValidateAndOptimize(ctx, bytes.NewReader(data), c, nil)
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	if err := api.WriteContext(ctx, pctx, &out); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// ---------- informació ----------

// Field és un camp de formulari.
type Field struct {
	Name    string   `json:"name"`
	ID      string   `json:"id"`
	Type    string   `json:"type"`
	Value   string   `json:"value"`
	Options []string `json:"options,omitempty"`
	Pages   []int    `json:"pages"`
	Locked  bool     `json:"locked,omitempty"`
}

// Annot és una anotació existent.
type Annot struct {
	Page    int    `json:"page"`
	Type    string `json:"type"`
	ID      string `json:"id,omitempty"`
	Rect    string `json:"rect"`
	Content string `json:"content,omitempty"`
}

// Info resumeix el document.
type Info struct {
	Pages     int          `json:"pages"`
	Sizes     []string     `json:"sizes"` // «595x842 pt (A4)» per pàgina diferent
	Title     string       `json:"title,omitempty"`
	Author    string       `json:"author,omitempty"`
	Subject   string       `json:"subject,omitempty"`
	Producer  string       `json:"producer,omitempty"`
	Creator   string       `json:"creator,omitempty"`
	Version   string       `json:"version,omitempty"`
	Encrypted bool         `json:"encrypted,omitempty"`
	Form      bool         `json:"form,omitempty"`
	Signed    bool         `json:"signed,omitempty"`
	Fields    []Field      `json:"fields,omitempty"`
	Annots    []Annot      `json:"annotations,omitempty"`
	Dims      [][2]float64 `json:"-"`
}

// ReadInfo llegeix metadades, mides, camps i anotacions.
func ReadInfo(ctx context.Context, path string) (*Info, error) {
	data, err := load(path)
	if err != nil {
		return nil, err
	}
	plain, enc, err := decrypted(ctx, data)
	if err != nil {
		return nil, err
	}
	pi, err := api.PDFInfo(ctx, bytes.NewReader(plain), filepath.Base(path), nil, false, conf())
	if err != nil {
		return nil, fmt.Errorf("PDF il·legible: %w", err)
	}
	inf := &Info{Pages: pi.PageCount, Title: pi.Title, Author: pi.Author, Subject: pi.Subject,
		Producer: pi.Producer, Creator: pi.Creator, Version: pi.Version, Encrypted: enc || pi.Encrypted,
		Form: pi.Form, Signed: pi.Signatures}
	if dims, err := api.PageDims(ctx, bytes.NewReader(plain), conf()); err == nil {
		seen := map[string]bool{}
		for _, d := range dims {
			inf.Dims = append(inf.Dims, [2]float64{d.Width, d.Height})
			s := sizeName(d.Width, d.Height)
			if !seen[s] {
				seen[s] = true
				inf.Sizes = append(inf.Sizes, s)
			}
		}
	}
	if pi.Form {
		inf.Fields, _ = fields(ctx, plain)
	}
	inf.Annots, _ = annots(ctx, plain)
	return inf, nil
}

func sizeName(w, h float64) string {
	s := fmt.Sprintf("%.0fx%.0f pt", w, h)
	near := func(a, b float64) bool { return a-b < 3 && b-a < 3 }
	for _, p := range []struct {
		n    string
		w, h float64
	}{{"A4", 595, 842}, {"Letter", 612, 792}, {"A3", 842, 1191}, {"A5", 420, 595}, {"Legal", 612, 1008}} {
		if near(w, p.w) && near(h, p.h) {
			return s + " (" + p.n + ")"
		}
		if near(w, p.h) && near(h, p.w) {
			return s + " (" + p.n + " apaïsat)"
		}
	}
	return s
}

// Fields llegeix els camps de formulari.
func Fields(ctx context.Context, path string) ([]Field, error) {
	data, err := load(path)
	if err != nil {
		return nil, err
	}
	plain, _, err := decrypted(ctx, data)
	if err != nil {
		return nil, err
	}
	return fields(ctx, plain)
}

func fields(ctx context.Context, data []byte) ([]Field, error) {
	fg, err := api.ExportForm(ctx, bytes.NewReader(data), "", conf())
	if err != nil {
		return nil, err
	}
	var out []Field
	for _, f := range fg.Forms {
		for _, x := range f.TextFields {
			out = append(out, Field{Name: x.Name, ID: x.ID, Type: "text", Value: x.Value, Pages: x.Pages, Locked: x.Locked})
		}
		for _, x := range f.DateFields {
			out = append(out, Field{Name: x.Name, ID: x.ID, Type: "date (" + x.Format + ")", Value: x.Value, Pages: x.Pages, Locked: x.Locked})
		}
		for _, x := range f.CheckBoxes {
			out = append(out, Field{Name: x.Name, ID: x.ID, Type: "checkbox", Value: strconv.FormatBool(x.Value), Pages: x.Pages, Locked: x.Locked})
		}
		for _, x := range f.RadioButtonGroups {
			out = append(out, Field{Name: x.Name, ID: x.ID, Type: "radio", Value: x.Value, Options: x.Options, Pages: x.Pages, Locked: x.Locked})
		}
		for _, x := range f.ComboBoxes {
			out = append(out, Field{Name: x.Name, ID: x.ID, Type: "combo", Value: x.Value, Options: x.Options, Pages: x.Pages, Locked: x.Locked})
		}
		for _, x := range f.ListBoxes {
			out = append(out, Field{Name: x.Name, ID: x.ID, Type: "list", Value: strings.Join(x.Values, ", "), Options: x.Options, Pages: x.Pages, Locked: x.Locked})
		}
	}
	return out, nil
}

func annots(ctx context.Context, data []byte) ([]Annot, error) {
	m, err := api.Annotations(ctx, bytes.NewReader(data), nil, conf())
	if err != nil {
		return nil, err
	}
	var out []Annot
	for pg, pa := range m {
		for typ, a := range pa {
			for _, r := range a.Map {
				out = append(out, Annot{Page: pg, Type: model.AnnotTypeStrings[typ], ID: r.ID(), Rect: r.RectString(), Content: r.Content()})
			}
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Page != out[j].Page {
			return out[i].Page < out[j].Page
		}
		return out[i].Rect < out[j].Rect
	})
	return out, nil
}

// ---------- escriptura ----------

// writeVia escriu el resultat de fn a out passant per un temporal validat.
// in pot ser igual que out (edició in situ).
func writeVia(ctx context.Context, in, out string, fn func(rs io.ReadSeeker, w io.Writer) error) error {
	var src []byte
	if in != "" {
		data, err := load(in)
		if err != nil {
			return err
		}
		plain, _, err := decrypted(ctx, data)
		if err != nil {
			return err
		}
		src = plain
	}
	var buf bytes.Buffer
	if err := fn(bytes.NewReader(src), &buf); err != nil {
		return cleanErr(err)
	}
	return commit(ctx, out, buf.Bytes())
}

// commit valida els bytes i els desa a out de manera atòmica.
func commit(ctx context.Context, out string, data []byte) error {
	if _, err := api.ReadAndValidate(ctx, bytes.NewReader(data), conf()); err != nil {
		return fmt.Errorf("el resultat no seria un PDF vàlid; no s'ha desat: %v", cleanErr(err))
	}
	if dir := filepath.Dir(out); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	tmp, err := os.CreateTemp(filepath.Dir(out), ".gregal-pdf-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	if err := os.Rename(tmpName, out); err != nil {
		os.Remove(tmpName)
		return err
	}
	return nil
}

// cleanErr treu el prefix de paquet dels errors de pdfcpu.
func cleanErr(err error) error {
	if err == nil {
		return nil
	}
	s := strings.TrimPrefix(err.Error(), "pdfcpu: ")
	return errors.New(s)
}

// Selection converteix «1-3,5,last» a la sintaxi de pdfcpu. Buit = totes.
func Selection(pages string) ([]string, error) {
	s := strings.TrimSpace(pages)
	if s == "" || strings.EqualFold(s, "all") || s == "*" {
		return nil, nil
	}
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(strings.ToLower(s), "last", "l")
	sel, err := api.ParsePageSelection(s)
	if err != nil {
		return nil, fmt.Errorf("pàgines %q: %v (exemples: \"1-3,5\", \"2-l\", \"odd\")", pages, cleanErr(err))
	}
	return sel, nil
}

// PageCount compta les pàgines.
func PageCount(ctx context.Context, path string) (int, error) {
	data, err := load(path)
	if err != nil {
		return 0, err
	}
	plain, _, err := decrypted(ctx, data)
	if err != nil {
		return 0, err
	}
	n, err := api.PageCount(ctx, bytes.NewReader(plain), conf())
	return n, cleanErr(err)
}

// Merge ajunta els PDF d'ins, en ordre, a out.
func Merge(ctx context.Context, ins []string, out string) error {
	if len(ins) < 2 {
		return errors.New("merge necessita almenys dos PDF")
	}
	// pdfcpu.Merge llegeix de disc: els xifrats es desxifren a temporals.
	dir, err := os.MkdirTemp("", "gregal-pdf-merge-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	var files []string
	for i, in := range ins {
		data, err := load(in)
		if err != nil {
			return err
		}
		plain, _, err := decrypted(ctx, data)
		if err != nil {
			return fmt.Errorf("%s: %w", in, err)
		}
		p := filepath.Join(dir, fmt.Sprintf("%03d.pdf", i))
		if err := os.WriteFile(p, plain, 0o600); err != nil {
			return err
		}
		files = append(files, p)
	}
	var buf bytes.Buffer
	if err := api.Merge(ctx, "", files, &buf, conf(), false); err != nil {
		return cleanErr(err)
	}
	return commit(ctx, out, buf.Bytes())
}

// Select escriu a out les pàgines triades, en l'ordre donat (es poden
// repetir): serveix per extreure i per reordenar.
func Select(ctx context.Context, in, out, pages string) error {
	sel, err := Selection(pages)
	if err != nil {
		return err
	}
	if len(sel) == 0 {
		return errors.New("falten les pàgines (pages)")
	}
	return writeVia(ctx, in, out, func(rs io.ReadSeeker, w io.Writer) error {
		return api.Collect(ctx, rs, w, sel, conf())
	})
}

// Delete esborra les pàgines triades.
func Delete(ctx context.Context, in, out, pages string) error {
	sel, err := Selection(pages)
	if err != nil {
		return err
	}
	if len(sel) == 0 {
		return errors.New("falten les pàgines a esborrar (pages)")
	}
	return writeVia(ctx, in, out, func(rs io.ReadSeeker, w io.Writer) error {
		return api.RemovePages(ctx, rs, w, sel, conf())
	})
}

// Rotate gira les pàgines (graus múltiples de 90, en sentit horari).
func Rotate(ctx context.Context, in, out string, degrees int, pages string) error {
	if degrees == 0 {
		degrees = 90
	}
	if degrees%90 != 0 {
		return fmt.Errorf("rotate: %d no és múltiple de 90", degrees)
	}
	sel, err := Selection(pages)
	if err != nil {
		return err
	}
	return writeVia(ctx, in, out, func(rs io.ReadSeeker, w io.Writer) error {
		return api.Rotate(ctx, rs, w, degrees, sel, conf())
	})
}

// Split divideix el PDF. Amb at (p. ex. «3,7»), talla abans de cada
// pàgina indicada; sense, en trossos de span pàgines (1 per defecte).
// Torna els fitxers creats, en ordre. before (si no és nil) es crida amb
// cada ruta just abans d'escriure-la: el journal de l'agent la hi apunta.
func Split(ctx context.Context, in, outDir string, span int, at string, before func(string)) ([]string, error) {
	n, err := PageCount(ctx, in)
	if err != nil {
		return nil, err
	}
	var cuts []int // primera pàgina de cada tros
	if strings.TrimSpace(at) != "" {
		cuts = []int{1}
		for _, s := range strings.Split(at, ",") {
			k, err := strconv.Atoi(strings.TrimSpace(s))
			if err != nil || k < 2 || k > n {
				return nil, fmt.Errorf("split: tall %q fora de 2..%d", s, n)
			}
			cuts = append(cuts, k)
		}
		sort.Ints(cuts)
	} else {
		if span <= 0 {
			span = 1
		}
		for p := 1; p <= n; p += span {
			cuts = append(cuts, p)
		}
	}
	base := strings.TrimSuffix(filepath.Base(in), filepath.Ext(in))
	var outs []string
	for i, from := range cuts {
		to := n
		if i+1 < len(cuts) {
			to = cuts[i+1] - 1
		}
		if to < from {
			continue
		}
		rng := strconv.Itoa(from)
		if to > from {
			rng += "-" + strconv.Itoa(to)
		}
		out := filepath.Join(outDir, fmt.Sprintf("%s_%s.pdf", base, rng))
		if before != nil {
			before(out)
		}
		if err := Select(ctx, in, out, rng); err != nil {
			return outs, err
		}
		outs = append(outs, out)
	}
	return outs, nil
}

// StampOpts descriu un text sobre (o sota) les pàgines.
type StampOpts struct {
	Text     string
	Pages    string
	Position string  // tl, tc, tr, l, c, r, bl, bc, br (per defecte c)
	X, Y     float64 // desplaçament en punts des de la posició
	Size     int     // punts (per defecte 12; segell 48)
	Color    string  // #RRGGBB o nom (red, blue…)
	Opacity  float64 // 0..1 (per defecte 1)
	Rotation float64 // graus
	Under    bool    // marca d'aigua sota el contingut
	Font     string  // Helvetica (per defecte), Times-Roman, Courier…
}

// AddText afegeix text a les pàgines. És la mateixa operació per a un
// segell («APROVAT» gran al mig), un peu o un text en una posició exacta
// (position=bl i x/y en punts des de baix a l'esquerra).
func AddText(ctx context.Context, in, out string, o StampOpts) error {
	if strings.TrimSpace(o.Text) == "" {
		return errors.New("falta el text")
	}
	sel, err := Selection(o.Pages)
	if err != nil {
		return err
	}
	desc, err := o.desc()
	if err != nil {
		return err
	}
	wm, err := api.TextWatermark(ctx, o.Text, desc, !o.Under, false, types.POINTS, conf())
	if err != nil {
		return fmt.Errorf("text: %v", cleanErr(err))
	}
	return writeVia(ctx, in, out, func(rs io.ReadSeeker, w io.Writer) error {
		return api.AddWatermarks(ctx, rs, w, sel, wm, conf())
	})
}

func (o StampOpts) desc() (string, error) {
	pos := strings.ToLower(strings.TrimSpace(o.Position))
	if pos == "" {
		pos = "c"
	}
	switch pos {
	case "tl", "tc", "tr", "l", "c", "r", "bl", "bc", "br", "full":
	default:
		return "", fmt.Errorf("position %q: fes servir tl, tc, tr, l, c, r, bl, bc o br", o.Position)
	}
	size := o.Size
	if size <= 0 {
		size = 12
	}
	op := o.Opacity
	if op <= 0 || op > 1 {
		op = 1
	}
	font := o.Font
	if font == "" {
		font = "Helvetica"
	}
	parts := []string{
		"fontname:" + font,
		"points:" + strconv.Itoa(size),
		"scalefactor:1 abs",
		"position:" + pos,
		fmt.Sprintf("offset:%g %g", o.X, o.Y),
		fmt.Sprintf("rotation:%g", o.Rotation),
		fmt.Sprintf("opacity:%g", op),
	}
	if c := strings.TrimSpace(o.Color); c != "" {
		if _, err := color.ParseColor(c); err != nil {
			return "", fmt.Errorf("color %q: %v", c, err)
		}
		parts = append(parts, "fillcolor:"+c)
	}
	return strings.Join(parts, ", "), nil
}

// FillForm omple camps pel nom (o id). Torna els que s'han omplert i els
// noms que no existeixen. Les caselles accepten true/false, sí/no, x.
func FillForm(ctx context.Context, in, out string, values map[string]string) ([]string, []string, error) {
	if len(values) == 0 {
		return nil, nil, errors.New("falten els valors (fields)")
	}
	data, err := load(in)
	if err != nil {
		return nil, nil, err
	}
	plain, _, err := decrypted(ctx, data)
	if err != nil {
		return nil, nil, err
	}
	fg, err := api.ExportForm(ctx, bytes.NewReader(plain), filepath.Base(in), conf())
	if err != nil {
		return nil, nil, fmt.Errorf("el PDF no té formulari: %v", cleanErr(err))
	}
	used := map[string]bool{}
	pick := func(name, id string) (string, bool) {
		for _, k := range []string{name, id} {
			if k == "" {
				continue
			}
			if v, ok := values[k]; ok {
				used[k] = true
				return v, true
			}
		}
		return "", false
	}
	var filled []string
	for _, f := range fg.Forms {
		for _, x := range f.TextFields {
			if v, ok := pick(x.Name, x.ID); ok {
				x.Value = v
				filled = append(filled, x.Name)
			}
		}
		for _, x := range f.DateFields {
			if v, ok := pick(x.Name, x.ID); ok {
				x.Value = v
				filled = append(filled, x.Name)
			}
		}
		for _, x := range f.CheckBoxes {
			if v, ok := pick(x.Name, x.ID); ok {
				x.Value = truthy(v)
				filled = append(filled, x.Name)
			}
		}
		for _, x := range f.RadioButtonGroups {
			if v, ok := pick(x.Name, x.ID); ok {
				if !contains(x.Options, v) {
					return nil, nil, fmt.Errorf("camp %s: %q no és cap de les opcions %v", x.Name, v, x.Options)
				}
				x.Value = v
				filled = append(filled, x.Name)
			}
		}
		for _, x := range f.ComboBoxes {
			if v, ok := pick(x.Name, x.ID); ok {
				if !x.Editable && !contains(x.Options, v) {
					return nil, nil, fmt.Errorf("camp %s: %q no és cap de les opcions %v", x.Name, v, x.Options)
				}
				x.Value = v
				filled = append(filled, x.Name)
			}
		}
		for _, x := range f.ListBoxes {
			if v, ok := pick(x.Name, x.ID); ok {
				var vals []string
				for _, s := range strings.Split(v, ",") {
					if s = strings.TrimSpace(s); s != "" {
						vals = append(vals, s)
					}
				}
				x.Values = vals
				filled = append(filled, x.Name)
			}
		}
	}
	var missing []string
	for k := range values {
		if !used[k] {
			missing = append(missing, k)
		}
	}
	sort.Strings(missing)
	if len(filled) == 0 {
		return nil, missing, fmt.Errorf("cap camp coincideix amb %v (llegeix-los amb pdf_read detail=info)", missing)
	}
	js, err := json.Marshal(fg)
	if err != nil {
		return nil, nil, err
	}
	var buf bytes.Buffer
	if err := api.FillForm(ctx, bytes.NewReader(plain), bytes.NewReader(js), &buf, conf()); err != nil {
		return nil, nil, cleanErr(err)
	}
	return filled, missing, commit(ctx, out, buf.Bytes())
}

func truthy(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "1", "true", "yes", "y", "x", "on", "sí", "si", "s", "checked":
		return true
	}
	return false
}

func contains(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}

// AnnotOpts és una anotació nova. Coordenades en punts des de baix a
// l'esquerra de la pàgina (pdf_read detail=info en dona la mida).
type AnnotOpts struct {
	Kind       string // note (per defecte), highlight, box, link
	Pages      string // per defecte 1
	X, Y, W, H float64
	Text       string // contingut (nota, comentari del requadre)
	Author     string
	Color      string
	URL        string // per a link
}

// Annotate afegeix una anotació a cada pàgina triada.
func Annotate(ctx context.Context, in, out string, o AnnotOpts) error {
	pages := o.Pages
	if strings.TrimSpace(pages) == "" {
		pages = "1"
	}
	sel, err := Selection(pages)
	if err != nil {
		return err
	}
	col := color.SimpleColor{R: 1, G: 0.85, B: 0}
	if c := strings.TrimSpace(o.Color); c != "" {
		if col, err = color.ParseColor(c); err != nil {
			return fmt.Errorf("color %q: %v", c, err)
		}
	}
	w, h := o.W, o.H
	if w <= 0 {
		w = 20
	}
	if h <= 0 {
		h = 20
	}
	rect := *types.RectForWidthAndHeight(o.X, o.Y, w, h)
	var ar model.AnnotationRenderer
	switch strings.ToLower(strings.TrimSpace(o.Kind)) {
	case "", "note", "comment", "text":
		if strings.TrimSpace(o.Text) == "" {
			return errors.New("una nota necessita text")
		}
		ar = model.NewTextAnnotation(rect, 0, o.Text, "", "", 0, &col, o.Author, nil, nil, "", "", 0, 0, 0, false, "Comment")
	case "highlight":
		ql := types.NewQuadLiteralForRect(&rect)
		ar = model.NewHighlightAnnotation(rect, 0, o.Text, "", "", 0, &col, 0, 0, 0, o.Author, nil, nil, "", "", types.QuadPoints{*ql})
	case "box", "square", "rect":
		ar = model.NewSquareAnnotation(rect, 0, o.Text, "", "", 0, &col, o.Author, nil, nil, "", "", nil, 0, 0, 0, 0, 2, model.BSSolid, false, 0)
	case "link":
		if strings.TrimSpace(o.URL) == "" {
			return errors.New("un link necessita url")
		}
		ar = model.NewLinkAnnotation(rect, 0, o.Text, "", "", 0, &col, nil, o.URL, nil, false, 0, model.BSSolid)
	default:
		return fmt.Errorf("kind %q: fes servir note, highlight, box o link", o.Kind)
	}
	return writeVia(ctx, in, out, func(rs io.ReadSeeker, w io.Writer) error {
		return api.AddAnnotations(ctx, rs, w, sel, ar, conf())
	})
}

// SetInfo canvia títol, autor, assumpte i paraules clau.
func SetInfo(ctx context.Context, in, out string, props map[string]string) error {
	if len(props) == 0 {
		return errors.New("falten les propietats (fields: title, author, subject…)")
	}
	return writeVia(ctx, in, out, func(rs io.ReadSeeker, w io.Writer) error {
		pctx, err := api.ReadValidateAndOptimize(ctx, rs, conf(), nil)
		if err != nil {
			return err
		}
		// El diccionari Info és el que es desa (els camps Title… del
		// context només es llegeixen). Text en UTF-16 perquè els accents
		// surtin bé a qualsevol visor.
		if pctx.Info == nil {
			ir, err := pctx.IndRefForNewObject(types.NewDict())
			if err != nil {
				return err
			}
			pctx.Info = ir
		}
		d, err := pctx.DereferenceDict(*pctx.Info)
		if err != nil || d == nil {
			return fmt.Errorf("diccionari Info il·legible: %v", err)
		}
		keys := map[string]string{"title": "Title", "author": "Author", "subject": "Subject", "keywords": "Keywords", "creator": "Creator"}
		for k, v := range props {
			key, ok := keys[strings.ToLower(k)]
			if !ok {
				return fmt.Errorf("propietat %q: title, author, subject, keywords o creator", k)
			}
			d[key] = types.NewHexLiteral([]byte(types.EncodeUTF16String(v)))
		}
		return api.WriteContext(ctx, pctx, w)
	})
}

// Validate comprova que pdfcpu el pot llegir i validar.
func Validate(ctx context.Context, path string) error {
	data, err := load(path)
	if err != nil {
		return err
	}
	plain, _, err := decrypted(ctx, data)
	if err != nil {
		return err
	}
	_, err = api.ReadAndValidate(ctx, bytes.NewReader(plain), conf())
	return cleanErr(err)
}
