package pdfdoc

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu"
)

// Els PDF de testdata/ són petits (pocs kB) i es tornen a generar amb
//
//	GREGAL_PDF_FIXTURES=1 go test ./internal/pdfdoc -run TestMakeFixtures
//
// - text3.pdf: tres pàgines amb text («Pàgina u/dos/tres» i una paraula
//   clau per pàgina: giraffe, walrus, pelican).
// - form.pdf: un formulari amb un camp de text «nom», una casella
//   «accepto» i un desplegable «ciutat».
// - scan.pdf: una sola pàgina que és una imatge (sense capa de text).

const text3JSON = `{
 "paper": "A4P", "origin": "LowerLeft",
 "pages": {
  "1": {"content": {"text": [{"value": "Pàgina u: giraffe", "pos": [72, 760], "font": {"name": "Helvetica", "size": 14}}]}},
  "2": {"content": {"text": [{"value": "Pàgina dos: walrus", "pos": [72, 760], "font": {"name": "Helvetica", "size": 14}}]}},
  "3": {"content": {"text": [{"value": "Pàgina tres: pelican", "pos": [72, 760], "font": {"name": "Helvetica", "size": 14}}]}}
 }
}`

const formJSON = `{
 "paper": "A4P", "origin": "LowerLeft",
 "pages": {
  "1": {"content": {
   "text": [{"value": "Sol·licitud", "pos": [72, 780], "font": {"name": "Helvetica", "size": 16}}],
   "textfield": [{"id": "nom", "value": "", "pos": [160, 700], "width": 200, "font": {"name": "Helvetica", "size": 12},
     "label": {"value": "Nom", "width": 60, "pos": "left", "font": {"name": "Helvetica", "size": 12}}}],
   "checkbox": [{"id": "accepto", "pos": [160, 660], "width": 12, "label": {"value": "Accepto", "width": 60, "pos": "left", "font": {"name": "Helvetica", "size": 12}}}],
   "combobox": [{"id": "ciutat", "pos": [160, 620], "width": 120, "options": ["Girona", "Lleida", "Tarragona"], "font": {"name": "Helvetica", "size": 12},
     "label": {"value": "Ciutat", "width": 60, "pos": "left", "font": {"name": "Helvetica", "size": 12}}}]
  }}
 }
}`

func TestMakeFixtures(t *testing.T) {
	if os.Getenv("GREGAL_PDF_FIXTURES") == "" {
		t.Skip("només amb GREGAL_PDF_FIXTURES=1")
	}
	ctx := context.Background()
	mk := func(name, js string) {
		var out bytes.Buffer
		if err := api.Create(ctx, nil, strings.NewReader(js), &out, conf()); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if err := os.WriteFile(filepath.Join("testdata", name), out.Bytes(), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	os.MkdirAll("testdata", 0o755)
	mk("text3.pdf", text3JSON)
	mk("form.pdf", formJSON)

	// scan.pdf: una imatge en gris amb unes franges, com un full escanejat.
	img := image.NewGray(image.Rect(0, 0, 120, 160))
	for y := 0; y < 160; y++ {
		for x := 0; x < 120; x++ {
			v := uint8(240)
			if y%20 < 3 && x > 10 && x < 110 {
				v = 40
			}
			img.SetGray(x, y, color.Gray{Y: v})
		}
	}
	dir := t.TempDir()
	pngPath := filepath.Join(dir, "scan.png")
	var pb bytes.Buffer
	png.Encode(&pb, img)
	os.WriteFile(pngPath, pb.Bytes(), 0o644)
	imp := pdfcpu.DefaultImportConfig()
	if err := api.ImportImagesFile(ctx, []string{pngPath}, filepath.Join("testdata", "scan.pdf"), imp, conf()); err != nil {
		t.Fatal(err)
	}
}
