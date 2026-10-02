package tools

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"strings"
)

// MaxImageBytes limita les imatges llegides (8 MB).
const MaxImageBytes = 8 << 20

// ReadImageDataURL llegeix una imatge del disc i la torna com a data URL
// (base64). Només lectura: no toca el journal. Rebutja no-imatges pel
// contingut (sniff), no per l'extensió.
func ReadImageDataURL(path string) (string, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if fi.IsDir() {
		return "", fmt.Errorf("%s és un directori", path)
	}
	if fi.Size() > MaxImageBytes {
		return "", fmt.Errorf("%s massa gran (%d bytes, màxim %d)", path, fi.Size(), MaxImageBytes)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return DataURLForBytes(raw, path)
}

// DataURLForBytes valida bytes d'imatge (sniff) i els torna com a data URL.
// Serveix bytes que no vénen del disc (p. ex. descarregats de Telegram).
func DataURLForBytes(raw []byte, hint string) (string, error) {
	if len(raw) > MaxImageBytes {
		return "", fmt.Errorf("%s massa gran (%d bytes, màxim %d)", hint, len(raw), MaxImageBytes)
	}
	mime := http.DetectContentType(raw)
	if len(mime) < 6 || mime[:6] != "image/" {
		return "", fmt.Errorf("%s no és una imatge (%s)", hint, mime)
	}
	return "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(raw), nil
}

// NormalizeDataURL valida una data URL rebuda de fora (webapp, app) i la
// torna canonitzada. Rebutja no-imatges, base64 malformat i excés de mida.
func NormalizeDataURL(u, hint string) (string, error) {
	if !strings.HasPrefix(u, "data:") {
		return "", fmt.Errorf("%s: cal data URL (data:image/...;base64,...)", hint)
	}
	comma := strings.Index(u, ",")
	if comma < 0 {
		return "", fmt.Errorf("%s: data URL malformada", hint)
	}
	raw, err := base64.StdEncoding.DecodeString(u[comma+1:])
	if err != nil {
		return "", fmt.Errorf("%s: base64 malformat", hint)
	}
	return DataURLForBytes(raw, hint)
}
