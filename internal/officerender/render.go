// Package officerender converteix documents d'Office en imatges perquè
// l'agent pugui revisar-ne l'aspecte (desbordaments, solapaments,
// taules trencades) abans de lliurar-los. És el bucle que fan servir
// Claude i ChatGPT: generar → renderitzar → mirar → corregir.
//
// Motors, per ordre:
//   - LibreOffice (soffice) → PDF; PDF → PNG amb pdftoppm si hi és, o
//     amb l'API de Windows (Windows.Data.Pdf) via PowerShell.
//   - Sense rasteritzador de PDF: per pàgina amb els filtres PNG de
//     LibreOffice (Writer admet PageRange; les diapositives es separen
//     en presentacions d'una sola diapositiva).
//   - A Windows sense LibreOffice: PowerPoint/Word per COM → PDF/PNG.
//
// Sense cap motor, l'eina ho diu clarament i només fa QA estructural.
package officerender

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"gregal/internal/ooxml"
)

// Result és el resultat d'un render.
type Result struct {
	Engine string   // "libreoffice+winpdf", "libreoffice+pdftoppm", "libreoffice", "powerpoint", "word+winpdf"
	PDF    string   // ruta del PDF intermedi (si n'hi ha)
	Pages  []string // PNG per pàgina/diapositiva (en ordre; "" si no s'ha fet)
	Total  int      // pàgines/diapositives totals del document
	Notes  []string // avisos (tipus de lletra substituïts, pàgines retallades…)
}

// Options configura el render.
type Options struct {
	OutDir   string // on deixar PNG/PDF (es crea)
	Pages    []int  // pàgines 1-based a renderitzar (buit = les primeres MaxPages)
	MaxPages int    // límit per defecte (12)
	Width    int    // amplada en píxels (per defecte 1280)
	Timeout  time.Duration
}

// ErrNoEngine vol dir que no hi ha cap motor de render.
var ErrNoEngine = errors.New("no hi ha cap motor de render: instal·la LibreOffice (https://www.libreoffice.org) per poder revisar visualment documents d'Office")

var (
	sofficeOnce sync.Once
	sofficePath string
	loMu        sync.Mutex // LibreOffice no admet dues instàncies amb el mateix perfil
)

// Soffice torna la ruta de LibreOffice (o "").
func Soffice() string {
	sofficeOnce.Do(func() { sofficePath = findSoffice() })
	return sofficePath
}

func findSoffice() string {
	if p := os.Getenv("GREGAL_SOFFICE"); p != "" {
		return p
	}
	names := []string{"soffice", "libreoffice"}
	if runtime.GOOS == "windows" {
		names = []string{"soffice.com", "soffice.exe", "soffice"}
	}
	for _, n := range names {
		if p, err := exec.LookPath(n); err == nil {
			return p
		}
	}
	var cands []string
	switch runtime.GOOS {
	case "windows":
		for _, env := range []string{"ProgramFiles", "ProgramFiles(x86)", "ProgramW6432", "LOCALAPPDATA"} {
			if d := os.Getenv(env); d != "" {
				cands = append(cands, filepath.Join(d, "LibreOffice", "program", "soffice.com"))
				if env == "LOCALAPPDATA" {
					cands = append(cands, filepath.Join(d, "Programs", "LibreOffice", "program", "soffice.com"))
				}
			}
		}
	case "darwin":
		home, _ := os.UserHomeDir()
		cands = []string{"/Applications/LibreOffice.app/Contents/MacOS/soffice", filepath.Join(home, "Applications/LibreOffice.app/Contents/MacOS/soffice")}
	default:
		cands = []string{"/usr/bin/soffice", "/usr/lib/libreoffice/program/soffice", "/opt/libreoffice/program/soffice", "/snap/bin/libreoffice", "/usr/local/bin/soffice"}
		if m, _ := filepath.Glob("/opt/libreoffice*/program/soffice"); len(m) > 0 {
			cands = append(cands, m...)
		}
	}
	for _, c := range cands {
		if fi, err := os.Stat(c); err == nil && !fi.IsDir() {
			return c
		}
	}
	return ""
}

// Available diu quin motor hi ha (per a missatges i el doctor).
func Available() string {
	if runtime.GOOS == "windows" && officeCOM() != "" {
		if Soffice() != "" {
			return "Microsoft Office (COM), LibreOffice de reserva"
		}
		return "Microsoft Office (COM)"
	}
	if Soffice() != "" {
		return "LibreOffice (" + Soffice() + ")"
	}
	return ""
}

func profileURL() string {
	dir := filepath.Join(os.TempDir(), "gregal-lo-profile")
	p := filepath.ToSlash(dir)
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return "file://" + p
}

// runSoffice executa LibreOffice amb perfil propi (si no, es penja quan
// l'usuari el té obert) i temps límit.
func runSoffice(ctx context.Context, timeout time.Duration, args ...string) error {
	loMu.Lock()
	defer loMu.Unlock()
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	full := append([]string{"-env:UserInstallation=" + profileURL(), "--headless", "--invisible", "--norestore", "--nolockcheck", "--nodefault", "--nofirststartwizard"}, args...)
	cmd := exec.CommandContext(ctx, Soffice(), full...)
	cmd.Env = append(os.Environ(), "SAL_USE_VCLPLUGIN=svp")
	hideWindow(cmd)
	out, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		killStray()
		return fmt.Errorf("LibreOffice no ha acabat en %s", timeout)
	}
	if err != nil {
		return fmt.Errorf("LibreOffice: %v: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// Render converteix el document en PNG per pàgina.
func Render(ctx context.Context, path string, o Options) (*Result, error) {
	if o.MaxPages <= 0 {
		o.MaxPages = 12
	}
	if o.Width <= 0 {
		o.Width = 1280
	}
	if o.Timeout <= 0 {
		o.Timeout = 150 * time.Second
	}
	if o.OutDir == "" {
		d, err := os.MkdirTemp("", "gregal-render-")
		if err != nil {
			return nil, err
		}
		o.OutDir = d
	}
	if err := os.MkdirAll(o.OutDir, 0o755); err != nil {
		return nil, err
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	ext := strings.ToLower(filepath.Ext(abs))
	// Amb Microsoft Office instal·lat, primer Office: és el que veurà
	// l'usuari i, mesurat el 2026-10-28, 3-6 vegades més ràpid que
	// LibreOffice (Word → PDF 1,3 s contra 6 s; el primer LibreOffice en
	// fred, 23 s). Si falla, LibreOffice continua sent l'alternativa.
	if runtime.GOOS == "windows" && officeCOM() != "" && ext != ".pdf" {
		res, err := renderCOM(ctx, abs, ext, o)
		if err == nil || Soffice() == "" {
			return res, err
		}
	}
	if Soffice() == "" {
		return nil, ErrNoEngine
	}
	res := &Result{}
	// 1. PDF.
	pdf, err := toPDF(ctx, abs, o)
	if err != nil {
		return nil, err
	}
	res.PDF = pdf
	res.Total = pdfPageCount(pdf)
	pages := choosePages(o, res.Total)
	// 2. PDF → PNG.
	pngs, eng, rerr := rasterPDF(ctx, pdf, pages, o)
	if rerr == nil {
		res.Engine = "libreoffice+" + eng
		res.Pages = pngs
		return finish(res, o, pages), nil
	}
	res.Notes = append(res.Notes, "rasteritzador de PDF no disponible ("+rerr.Error()+"); render pàgina a pàgina")
	// 3. Sense rasteritzador: filtres PNG de LibreOffice.
	res.Engine = "libreoffice"
	switch ext {
	case ".pptx", ".potx", ".pptm", ".ppt", ".odp":
		res.Pages, err = slidesViaSplit(ctx, abs, pages, o)
	default:
		res.Pages, err = pagesViaWriter(ctx, abs, ext, pages, o)
	}
	if err != nil {
		return nil, err
	}
	return finish(res, o, pages), nil
}

func finish(res *Result, o Options, pages []int) *Result {
	if res.Total > len(pages) && len(o.Pages) == 0 {
		res.Notes = append(res.Notes, fmt.Sprintf("s'han renderitzat %d de %d pàgines; demana'n d'altres amb pages", len(pages), res.Total))
	}
	return res
}

func choosePages(o Options, total int) []int {
	var out []int
	if len(o.Pages) > 0 {
		for _, p := range o.Pages {
			if p >= 1 && (total == 0 || p <= total) {
				out = append(out, p)
			}
		}
		return out
	}
	n := total
	if n == 0 {
		n = 1
	}
	if n > o.MaxPages {
		n = o.MaxPages
	}
	for i := 1; i <= n; i++ {
		out = append(out, i)
	}
	return out
}

func toPDF(ctx context.Context, abs string, o Options) (string, error) {
	base := strings.TrimSuffix(filepath.Base(abs), filepath.Ext(abs))
	// Copia amb nom simple: LibreOffice té problemes amb alguns noms.
	src := filepath.Join(o.OutDir, "src"+filepath.Ext(abs))
	data, err := os.ReadFile(abs)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(src, data, 0o644); err != nil {
		return "", err
	}
	if err := runSoffice(ctx, o.Timeout, "--convert-to", "pdf", "--outdir", o.OutDir, src); err != nil {
		return "", err
	}
	out := filepath.Join(o.OutDir, "src.pdf")
	if _, err := os.Stat(out); err != nil {
		return "", fmt.Errorf("LibreOffice no ha generat el PDF de %s", base)
	}
	final := filepath.Join(o.OutDir, base+".pdf")
	os.Remove(final)
	if err := os.Rename(out, final); err != nil {
		final = out
	}
	return final, nil
}

var countRe = regexp.MustCompile(`/Count\s+(\d+)`)
var pageRe = regexp.MustCompile(`/Type\s*/Page[^s]`)

func pdfPageCount(pdf string) int {
	data, err := os.ReadFile(pdf)
	if err != nil {
		return 0
	}
	// Objectes /Type /Page (no /Pages). /Count pot venir de l'índex
	// d'esquema (outlines) i no és fiable; només si no hi ha objectes
	// visibles (PDF amb object streams).
	if n := len(pageRe.FindAll(data, -1)); n > 0 {
		return n
	}
	max := 0
	for _, m := range countRe.FindAllSubmatch(data, -1) {
		if n, _ := strconv.Atoi(string(m[1])); n > max {
			max = n
		}
	}
	return max
}

// rasterPDF: pdftoppm o, a Windows, Windows.Data.Pdf.
func rasterPDF(ctx context.Context, pdf string, pages []int, o Options) ([]string, string, error) {
	if len(pages) == 0 {
		return nil, "", fmt.Errorf("cap pàgina")
	}
	if p, err := exec.LookPath("pdftoppm"); err == nil {
		var out []string
		for _, n := range pages {
			prefix := filepath.Join(o.OutDir, fmt.Sprintf("page-%d", n))
			c, cancel := context.WithTimeout(ctx, 60*time.Second)
			cmd := exec.CommandContext(c, p, "-png", "-singlefile", "-f", strconv.Itoa(n), "-l", strconv.Itoa(n), "-scale-to-x", strconv.Itoa(o.Width), "-scale-to-y", "-1", pdf, prefix)
			hideWindow(cmd)
			err := cmd.Run()
			cancel()
			if err != nil {
				return nil, "", err
			}
			out = append(out, prefix+".png")
		}
		return out, "pdftoppm", nil
	}
	if runtime.GOOS == "windows" {
		out, err := winPDF(ctx, pdf, pages, o)
		if err == nil {
			return out, "winpdf", nil
		}
		return nil, "", err
	}
	return nil, "", fmt.Errorf("sense rasteritzador de PDF")
}

// pagesViaWriter fa una crida per pàgina amb el filtre PNG de Writer o
// Calc (admeten PageRange).
func pagesViaWriter(ctx context.Context, abs, ext string, pages []int, o Options) ([]string, error) {
	filter := "writer_png_Export"
	if ext == ".xlsx" || ext == ".xltx" || ext == ".xlsm" || ext == ".ods" || ext == ".csv" {
		filter = "calc_png_Export"
	}
	var out []string
	for _, n := range pages {
		d := filepath.Join(o.OutDir, fmt.Sprintf("p%d", n))
		os.MkdirAll(d, 0o755)
		opt := fmt.Sprintf(`png:%s:{"PageRange":{"type":"string","value":"%d"}}`, filter, n)
		if err := runSoffice(ctx, o.Timeout, "--convert-to", opt, "--outdir", d, abs); err != nil {
			return nil, err
		}
		m, _ := filepath.Glob(filepath.Join(d, "*.png"))
		if len(m) == 0 {
			return nil, fmt.Errorf("LibreOffice no ha generat la pàgina %d", n)
		}
		dst := filepath.Join(o.OutDir, fmt.Sprintf("page-%d.png", n))
		os.Remove(dst)
		os.Rename(m[0], dst)
		os.RemoveAll(d)
		out = append(out, dst)
	}
	return out, nil
}

// slidesViaSplit crea una presentació per diapositiva i les converteix
// totes d'una sola tirada (l'exportació PNG d'Impress només fa la 1a).
func slidesViaSplit(ctx context.Context, abs string, pages []int, o Options) ([]string, error) {
	data, err := os.ReadFile(abs)
	if err != nil {
		return nil, err
	}
	var files []string
	for _, n := range pages {
		pk, err := ooxml.OpenBytes(data)
		if err != nil {
			return nil, err
		}
		p, err := ooxml.OpenPptx(pk)
		if err != nil {
			return nil, err
		}
		if err := p.KeepOnlySlide(n - 1); err != nil {
			return nil, err
		}
		f := filepath.Join(o.OutDir, fmt.Sprintf("page-%d.pptx", n))
		if err := p.Save(f); err != nil {
			return nil, err
		}
		files = append(files, f)
	}
	h := o.Width * 9 / 16
	opt := fmt.Sprintf(`png:impress_png_Export:{"PixelWidth":{"type":"long","value":"%d"},"PixelHeight":{"type":"long","value":"%d"}}`, o.Width, h)
	args := append([]string{"--convert-to", opt, "--outdir", o.OutDir}, files...)
	if err := runSoffice(ctx, o.Timeout+time.Duration(len(files))*10*time.Second, args...); err != nil {
		return nil, err
	}
	var out []string
	for _, f := range files {
		png := strings.TrimSuffix(f, ".pptx") + ".png"
		os.Remove(f)
		if _, err := os.Stat(png); err != nil {
			return nil, fmt.Errorf("LibreOffice no ha generat %s", filepath.Base(png))
		}
		out = append(out, png)
	}
	return out, nil
}

// PageList interpreta "1,3,5-7" com a llista de pàgines.
func PageList(s string) []int {
	var out []int
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if a, b, ok := strings.Cut(part, "-"); ok {
			x, e1 := strconv.Atoi(strings.TrimSpace(a))
			y, e2 := strconv.Atoi(strings.TrimSpace(b))
			if e1 == nil && e2 == nil && x <= y && y-x < 200 {
				for i := x; i <= y; i++ {
					out = append(out, i)
				}
			}
			continue
		}
		if n, err := strconv.Atoi(part); err == nil {
			out = append(out, n)
		}
	}
	sort.Ints(out)
	return out
}
