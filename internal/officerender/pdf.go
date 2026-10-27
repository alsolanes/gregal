package officerender

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// RasterPDF converteix pàgines (1-based) d'un PDF existent en PNG dins
// outDir. Fa servir el mateix rasteritzador que el render d'Office:
// pdftoppm (Poppler) si és al PATH o, a Windows 10/11, el Windows.Data.Pdf
// integrat. Torna les rutes en ordre i el motor.
func RasterPDF(ctx context.Context, pdf string, pages []int, outDir string, width int) ([]string, string, error) {
	abs, err := filepath.Abs(pdf)
	if err != nil {
		return nil, "", err
	}
	// Windows.Data.Pdf no accepta rutes amb «/»: totes dues absolutes i netes.
	if outDir, err = filepath.Abs(outDir); err != nil {
		return nil, "", err
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return nil, "", err
	}
	if width <= 0 {
		width = 1280
	}
	return rasterPDF(ctx, abs, pages, Options{OutDir: outDir, Width: width})
}

// PDFRasterAvailable diu quin rasteritzador de PDF hi ha ("" = cap).
func PDFRasterAvailable() string {
	if _, err := exec.LookPath("pdftoppm"); err == nil {
		return "pdftoppm"
	}
	if runtime.GOOS == "windows" && powershell() != "" {
		return "Windows.Data.Pdf"
	}
	return ""
}
