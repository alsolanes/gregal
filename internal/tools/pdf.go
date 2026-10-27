package tools

// PDF: llegir (text per pàgina, metadades, formularis, anotacions i, quan
// una pàgina no té capa de text, la pàgina com a imatge) i editar
// (ajuntar, dividir, extreure, reordenar, esborrar, girar, text i
// segells, formularis, anotacions, metadades).
//
//   - internal/pdfdoc: text (extractor propi) i edició (pdfcpu, Go pur).
//   - imatges: internal/officerender.RasterPDF (pdftoppm o Windows.Data.Pdf;
//     opcional: sense cap dels dos, pdf_read ho diu i torna només text).
//
// Com office_edit: tota escriptura passa pel journal (SnapOp) perquè
// /rewind la desfaci, i pdfdoc valida el resultat abans de substituir
// el fitxer.

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"gregal/internal/officerender"
	"gregal/internal/pdfdoc"
)

// IsPDF diu si la ruta és un .pdf.
func IsPDF(path string) bool { return pdfdoc.IsPDF(path) }

// PdfReadOpts configura pdf_read.
type PdfReadOpts struct {
	Path   string
	Pages  string // "1,3-5" (buit = totes)
	Detail string // text (per defecte), info, images
}

// pdfMaxAutoImages: pàgines sense text que pdf_read renderitza soles.
const pdfMaxAutoImages = 4

// PdfRead llegeix un PDF. Torna text per al model i, si cal, imatges.
func PdfRead(ctx context.Context, o PdfReadOpts) (string, []string, error) {
	if !IsPDF(o.Path) {
		return "", nil, fmt.Errorf("no és un PDF: %s", o.Path)
	}
	switch strings.ToLower(strings.TrimSpace(o.Detail)) {
	case "", "text", "full":
		return pdfReadText(ctx, o)
	case "info", "forms", "fields", "annotations":
		out, err := PdfInfoText(ctx, o.Path)
		return out, nil, err
	case "images", "image", "render":
		return PdfRender(ctx, o.Path, o.Pages)
	}
	return "", nil, fmt.Errorf("detail %q: text, info o images", o.Detail)
}

// PdfText és el text del PDF, sense imatges (mencions @fitxer, API).
func PdfText(path string) (string, error) {
	out, _, err := pdfReadTextOpts(context.Background(), PdfReadOpts{Path: path}, false)
	return out, err
}

func pdfReadText(ctx context.Context, o PdfReadOpts) (string, []string, error) {
	return pdfReadTextOpts(ctx, o, true)
}

func pdfReadTextOpts(ctx context.Context, o PdfReadOpts, render bool) (string, []string, error) {
	pages, err := pdfdoc.ReadPages(ctx, o.Path)
	if err != nil {
		return "", nil, pdfErr(err)
	}
	want := map[int]bool{}
	for _, n := range officerender.PageList(o.Pages) {
		want[n] = true
	}
	var b strings.Builder
	var empty []int
	shown := 0
	for _, pg := range pages {
		if len(want) > 0 && !want[pg.N] {
			continue
		}
		shown++
		fmt.Fprintf(&b, "— Pàgina %d —\n", pg.N)
		if strings.TrimSpace(pg.Text) == "" {
			empty = append(empty, pg.N)
			b.WriteString("(sense capa de text: escanejada o només imatges)\n\n")
			continue
		}
		b.WriteString(pg.Text)
		b.WriteString("\n\n")
	}
	head := fmt.Sprintf("PDF de %d pàgines", len(pages))
	if len(want) > 0 {
		head += fmt.Sprintf(" (se'n mostren %d: %s)", shown, o.Pages)
	}
	head += ".\n"
	if shown == 0 {
		return head + "Cap de les pàgines demanades existeix.\n", nil, nil
	}
	var imgs []string
	if len(empty) > 0 {
		tail := fmt.Sprintf("Pàgines sense text: %s.", joinInts(empty))
		if render && len(empty) <= pdfMaxAutoImages && officerender.PDFRasterAvailable() != "" {
			note, ims, rerr := PdfRender(ctx, o.Path, joinInts(empty))
			if rerr == nil && len(ims) > 0 {
				imgs = ims
				tail += " Les adjunto com a imatge: llegeix-les tu mateix.\n" + note
			} else if rerr != nil {
				tail += " No s'han pogut renderitzar: " + rerr.Error()
			}
		} else if render && officerender.PDFRasterAvailable() != "" {
			tail += fmt.Sprintf(" Demana-les com a imatge amb detail=images i pages (com a molt %d de cop).", pdfMaxAutoImages)
		} else if officerender.PDFRasterAvailable() == "" {
			tail += " Per veure-les com a imatge cal pdftoppm (Poppler) al PATH o Windows 10/11."
		}
		b.WriteString(tail + "\n")
	}
	return head + strings.TrimRight(b.String(), "\n") + "\n", imgs, nil
}

func joinInts(xs []int) string {
	s := make([]string, len(xs))
	for i, x := range xs {
		s[i] = strconv.Itoa(x)
	}
	return strings.Join(s, ",")
}

func pdfErr(err error) error {
	if err == pdfdoc.ErrEncrypted {
		return fmt.Errorf("PDF xifrat amb contrasenya d'obertura: demana a l'usuari una còpia sense contrasenya")
	}
	return err
}

// PdfInfoText descriu metadades, mides, formulari i anotacions.
func PdfInfoText(ctx context.Context, path string) (string, error) {
	inf, err := pdfdoc.ReadInfo(ctx, path)
	if err != nil {
		return "", pdfErr(err)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Pàgines: %d · mida: %s · PDF %s\n", inf.Pages, strings.Join(inf.Sizes, "; "), inf.Version)
	for _, kv := range [][2]string{{"Títol", inf.Title}, {"Autor", inf.Author}, {"Assumpte", inf.Subject}, {"Creador", inf.Creator}, {"Productor", inf.Producer}} {
		if strings.TrimSpace(kv[1]) != "" {
			fmt.Fprintf(&b, "%s: %s\n", kv[0], kv[1])
		}
	}
	if inf.Encrypted {
		b.WriteString("Xifrat (sense contrasenya d'obertura): es pot llegir i editar; el resultat es desa sense xifrar.\n")
	}
	if inf.Signed {
		b.WriteString("Signat digitalment: qualsevol edició invalida la signatura.\n")
	}
	if len(inf.Dims) > 0 {
		b.WriteString("Coordenades: punts (1/72 de polzada) des de baix a l'esquerra.\n")
	}
	if len(inf.Fields) > 0 {
		fmt.Fprintf(&b, "Formulari (%d camps; omple'ls amb pdf_edit op=fill_form fields={nom: valor}):\n", len(inf.Fields))
		for _, f := range inf.Fields {
			fmt.Fprintf(&b, "  - %s [%s] pàg. %s = %q", f.Name, f.Type, joinInts(f.Pages), f.Value)
			if len(f.Options) > 0 {
				fmt.Fprintf(&b, " opcions: %s", strings.Join(f.Options, " | "))
			}
			if f.Locked {
				b.WriteString(" (bloquejat)")
			}
			b.WriteString("\n")
		}
	} else if inf.Form {
		b.WriteString("Té formulari, però no se n'han pogut llegir els camps.\n")
	}
	if len(inf.Annots) > 0 {
		fmt.Fprintf(&b, "Anotacions (%d):\n", len(inf.Annots))
		for _, a := range limitAnnots(inf.Annots, 40) {
			fmt.Fprintf(&b, "  - pàg. %d %s %s", a.Page, a.Type, a.Rect)
			if a.Content != "" {
				fmt.Fprintf(&b, ": %q", a.Content)
			}
			b.WriteString("\n")
		}
	}
	return b.String(), nil
}

func limitAnnots(xs []pdfdoc.Annot, n int) []pdfdoc.Annot {
	if len(xs) > n {
		return xs[:n]
	}
	return xs
}

// PdfRender converteix pàgines en imatges: fins a 4 a mida gran; més,
// en una graella numerada (com office_render).
func PdfRender(ctx context.Context, path, pages string) (string, []string, error) {
	if eng := officerender.PDFRasterAvailable(); eng == "" {
		return "Sense rasteritzador de PDF: instal·la Poppler (pdftoppm) o fes servir Windows 10/11 per veure les pàgines com a imatge. El text es llegeix igualment amb pdf_read.\n", nil, nil
	}
	n, err := pdfdoc.PageCount(ctx, path)
	if err != nil {
		return "", nil, pdfErr(err)
	}
	list := officerender.PageList(pages)
	if len(list) == 0 {
		for i := 1; i <= n && i <= 12; i++ {
			list = append(list, i)
		}
	}
	var valid []int
	for _, p := range list {
		if p >= 1 && p <= n {
			valid = append(valid, p)
		}
	}
	if len(valid) == 0 {
		return "", nil, fmt.Errorf("pàgines %q fora de 1..%d", pages, n)
	}
	if len(valid) > 24 {
		valid = valid[:24]
	}
	sum := sha1.Sum([]byte(path + time.Now().String()))
	out := filepath.Join(os.TempDir(), "gregal-render", "pdf-"+hex.EncodeToString(sum[:6]))
	big := len(valid) <= 4
	width := 1280
	if !big {
		width = 800
	}
	pngs, eng, err := officerender.RasterPDF(ctx, path, valid, out, width)
	if err != nil {
		return "", nil, fmt.Errorf("no s'ha pogut renderitzar: %v", err)
	}
	var imgs []string
	var b strings.Builder
	fmt.Fprintf(&b, "Renderitzat amb %s: pàgines %s de %d.\n", eng, joinInts(valid), n)
	if big {
		for _, p := range pngs {
			if u, err := imageURLScaled(p, 1280); err == nil {
				imgs = append(imgs, u)
			}
		}
	} else {
		sheet := filepath.Join(out, "contact.png")
		if err := officerender.ContactSheet(pngs, valid, 4, 400, sheet); err == nil {
			if u, err := ReadImageDataURL(sheet); err == nil {
				imgs = append(imgs, u)
			}
		}
		b.WriteString("Imatge: graella numerada. Per veure detalls, demana pages=\"N\" (fins a 4).\n")
	}
	return b.String(), imgs, nil
}

// ---------- edició ----------

// PdfEditOpts configura pdf_edit.
type PdfEditOpts struct {
	Path      string
	Op        string
	Out       string
	Paths     []string // merge: els PDF a ajuntar, en ordre
	Pages     string
	Degrees   int
	Span      int
	At        string
	Text      string
	Position  string
	X, Y      float64
	W, H      float64
	Size      int
	Color     string
	Opacity   float64
	Rotation  float64
	Kind      string
	URL       string
	Author    string
	Fields    map[string]string
	Overwrite bool
}

// PdfOps són les operacions de pdf_edit (per a l'esquema i els errors).
var PdfOps = []string{"merge", "split", "extract", "reorder", "delete_pages", "rotate", "add_text", "stamp", "watermark", "fill_form", "annotate", "set_info"}

// PdfEditTargets diu quins fitxers escriurà l'operació (permisos).
func PdfEditTargets(o PdfEditOpts) []string {
	switch o.Op {
	case "split":
		dir := o.Out
		if dir == "" {
			dir = filepath.Dir(o.Path)
		}
		return []string{dir}
	case "merge":
		return []string{o.Out}
	}
	return []string{pdfOut(o)}
}

func pdfOut(o PdfEditOpts) string {
	if strings.TrimSpace(o.Out) != "" {
		return o.Out
	}
	if o.Op == "extract" {
		base := strings.TrimSuffix(o.Path, filepath.Ext(o.Path))
		sel := strings.NewReplacer(",", "_", " ", "", "-", "a").Replace(o.Pages)
		return base + "_p" + sel + ".pdf"
	}
	return o.Path
}

// PdfEdit aplica una operació. Torna un resum per al model.
func PdfEdit(ctx context.Context, o PdfEditOpts) (string, error) {
	op := strings.ToLower(strings.TrimSpace(o.Op))
	o.Op = op
	if op == "" {
		return "", fmt.Errorf("falta op (%s)", strings.Join(PdfOps, ", "))
	}
	if op == "merge" {
		return pdfMerge(ctx, o)
	}
	if !IsPDF(o.Path) {
		return "", fmt.Errorf("no és un PDF: %s", o.Path)
	}
	if _, err := os.Stat(o.Path); err != nil {
		return "", err
	}
	if op == "split" {
		return pdfSplit(ctx, o)
	}
	out := pdfOut(o)
	if !IsPDF(out) {
		return "", fmt.Errorf("la sortida ha de ser un .pdf: %s", out)
	}
	if err := pdfCanWrite(o.Path, out, o.Overwrite); err != nil {
		return "", err
	}
	Active.SnapOp(out, "edit")
	var msg string
	var err error
	switch op {
	case "extract":
		err = pdfdoc.Select(ctx, o.Path, out, o.Pages)
		msg = "extretes les pàgines " + o.Pages
	case "reorder":
		err = pdfdoc.Select(ctx, o.Path, out, o.Pages)
		msg = "pàgines en l'ordre " + o.Pages
	case "delete_pages":
		err = pdfdoc.Delete(ctx, o.Path, out, o.Pages)
		msg = "esborrades les pàgines " + o.Pages
	case "rotate":
		err = pdfdoc.Rotate(ctx, o.Path, out, o.Degrees, o.Pages)
		msg = fmt.Sprintf("girades %d° les pàgines %s", firstNonZero(o.Degrees, 90), pagesOrAll(o.Pages))
	case "add_text", "stamp", "watermark":
		so := pdfdoc.StampOpts{Text: o.Text, Pages: o.Pages, Position: o.Position, X: o.X, Y: o.Y,
			Size: o.Size, Color: o.Color, Opacity: o.Opacity, Rotation: o.Rotation}
		switch op {
		case "stamp":
			// Segell: gran, vermell i una mica transparent si no es diu res.
			so.Size = firstNonZero(so.Size, 48)
			if so.Color == "" {
				so.Color = "#C00000"
			}
			if so.Opacity == 0 {
				so.Opacity = 0.6
			}
		case "watermark":
			// Marca d'aigua: sota el contingut, en diagonal, gris clar.
			so.Under = true
			so.Size = firstNonZero(so.Size, 60)
			if so.Color == "" {
				so.Color = "#808080"
			}
			if so.Opacity == 0 {
				so.Opacity = 0.3
			}
			if so.Rotation == 0 {
				so.Rotation = 45
			}
		}
		err = pdfdoc.AddText(ctx, o.Path, out, so)
		msg = fmt.Sprintf("%s %q a les pàgines %s", map[string]string{"add_text": "afegit el text", "stamp": "segell", "watermark": "marca d'aigua"}[op], o.Text, pagesOrAll(o.Pages))
	case "fill_form":
		var filled, missing []string
		filled, missing, err = pdfdoc.FillForm(ctx, o.Path, out, o.Fields)
		msg = fmt.Sprintf("omplerts %d camps: %s", len(filled), strings.Join(filled, ", "))
		if len(missing) > 0 {
			msg += "; no existeixen: " + strings.Join(missing, ", ")
		}
	case "annotate":
		err = pdfdoc.Annotate(ctx, o.Path, out, pdfdoc.AnnotOpts{Kind: o.Kind, Pages: o.Pages, X: o.X, Y: o.Y, W: o.W, H: o.H,
			Text: o.Text, Author: o.Author, Color: o.Color, URL: o.URL})
		msg = fmt.Sprintf("anotació %s a la pàgina %s", firstNonEmpty(o.Kind, "note"), firstNonEmpty(o.Pages, "1"))
	case "set_info":
		err = pdfdoc.SetInfo(ctx, o.Path, out, o.Fields)
		keys := make([]string, 0, len(o.Fields))
		for k := range o.Fields {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		msg = "metadades: " + strings.Join(keys, ", ")
	default:
		return "", fmt.Errorf("op %q desconeguda: %s", o.Op, strings.Join(PdfOps, ", "))
	}
	if err != nil {
		return "", pdfErr(err)
	}
	return pdfSummary(ctx, out, msg), nil
}

func pdfMerge(ctx context.Context, o PdfEditOpts) (string, error) {
	ins := o.Paths
	if len(ins) == 0 && o.Path != "" {
		return "", fmt.Errorf("merge: passa els PDF a ajuntar a paths (en ordre) i el destí a out")
	}
	if strings.TrimSpace(o.Out) == "" {
		return "", fmt.Errorf("merge: falta out (el PDF resultant)")
	}
	for _, p := range ins {
		if !IsPDF(p) {
			return "", fmt.Errorf("merge: %s no és un PDF", p)
		}
	}
	if !IsPDF(o.Out) {
		return "", fmt.Errorf("la sortida ha de ser un .pdf: %s", o.Out)
	}
	if err := pdfCanWrite("", o.Out, o.Overwrite); err != nil {
		return "", err
	}
	Active.SnapOp(o.Out, "edit")
	if err := pdfdoc.Merge(ctx, ins, o.Out); err != nil {
		return "", pdfErr(err)
	}
	return pdfSummary(ctx, o.Out, fmt.Sprintf("ajuntats %d PDF", len(ins))), nil
}

func pdfSplit(ctx context.Context, o PdfEditOpts) (string, error) {
	dir := o.Out
	if strings.TrimSpace(dir) == "" {
		dir = filepath.Dir(o.Path)
	}
	if IsPDF(dir) {
		return "", fmt.Errorf("split: out és un directori, no un .pdf")
	}
	var conflicts []string
	outs, err := pdfdoc.Split(ctx, o.Path, dir, o.Span, o.At, func(p string) {
		if _, err := os.Stat(p); err == nil && !o.Overwrite {
			conflicts = append(conflicts, p)
		}
		Active.SnapOp(p, "edit")
	})
	if err != nil {
		return "", pdfErr(err)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "dividit en %d fitxers:\n", len(outs))
	for _, p := range outs {
		n, _ := pdfdoc.PageCount(ctx, p)
		fmt.Fprintf(&b, "  %s (%d pàg.)\n", p, n)
	}
	if len(conflicts) > 0 {
		b.WriteString("Atenció: s'han sobreescrit " + strings.Join(conflicts, ", ") + " (/rewind els recupera).\n")
	}
	return b.String(), nil
}

// pdfCanWrite: el destí s'ha de poder escriure i, si és un altre fitxer
// que ja existeix, només amb overwrite (mai trepitjar un PDF de l'usuari).
func pdfCanWrite(in, out string, overwrite bool) error {
	if _, err := os.Stat(out); err == nil {
		if !overwrite && (in == "" || !samePath(in, out)) {
			return fmt.Errorf("%s ja existeix: tria un altre out o passa overwrite=true", out)
		}
		if err := OfficeWritable(out); err != nil {
			return fmt.Errorf("no es pot escriure %s (obert en un visor de PDF?): %v", out, err)
		}
	}
	return nil
}

func pdfSummary(ctx context.Context, out, msg string) string {
	n, _ := pdfdoc.PageCount(ctx, out)
	return fmt.Sprintf("%s → %s (%d pàgines). Comprova-ho amb pdf_read (detail=images per veure'l).", msg, out, n)
}

func pagesOrAll(p string) string {
	if strings.TrimSpace(p) == "" {
		return "totes"
	}
	return p
}

func firstNonZero(a, b int) int {
	if a != 0 {
		return a
	}
	return b
}


// PdfFieldsJSON és el formulari en JSON (API web).
func PdfFieldsJSON(ctx context.Context, path string) (json.RawMessage, error) {
	fs, err := pdfdoc.Fields(ctx, path)
	if err != nil {
		return nil, err
	}
	return json.Marshal(fs)
}

// PdfRenderAvailable descriu el rasteritzador de PDF ("" = cap).
func PdfRenderAvailable() string { return officerender.PDFRasterAvailable() }

// PdfCreateOpts configura pdf_create.
type PdfCreateOpts struct {
	Path, Title, Author, Content, Page string
	Landscape, NoPageNumbers, Overwrite bool
}

// PdfCreate crea un PDF nou a partir de markdown (journal: /rewind l'esborra).
func PdfCreate(ctx context.Context, o PdfCreateOpts) (string, error) {
	if !IsPDF(o.Path) {
		return "", fmt.Errorf("la ruta ha d'acabar en .pdf: %s", o.Path)
	}
	if _, err := os.Stat(o.Path); err == nil && !o.Overwrite {
		return "", fmt.Errorf("%s ja existeix: tria un altre nom o passa overwrite=true (només per refer un esborrany teu)", o.Path)
	}
	if err := pdfCanWrite(o.Path, o.Path, true); err != nil {
		return "", err
	}
	Active.SnapOp(o.Path, "write")
	if err := pdfdoc.Create(ctx, o.Path, pdfdoc.CreateOpts{Title: o.Title, Author: o.Author, Content: o.Content,
		Page: o.Page, Landscape: o.Landscape, NoNumbers: o.NoPageNumbers}); err != nil {
		return "", err
	}
	return pdfSummary(ctx, o.Path, "creat"), nil
}
