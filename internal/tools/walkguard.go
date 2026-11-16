package tools

import (
	"io/fs"
	"path/filepath"
	"strings"
	"time"
)

// Guardes de les eines que recorren carpetes (grep).
//
// Per què: el grep obria TOTS els fitxers de l'arbre (fotos i vídeos inclosos,
// només per veure que eren binaris) i no s'aturava mai. Amb la carpeta de
// treball a la carpeta personal, una cerca entrava a Nextcloud/OneDrive i
// cada fitxer «només en línia» que obria es baixava sencer: el 15/11/2026 una
// sola cerca va començar a baixar tot el carret de fotos del mòbil. Ara:
//   - els fitxers només en línia (marcats així pel sistema) no s'obren mai;
//   - fotos, vídeos, àudio, arxius i binaris no s'obren: pel nom ja se sap;
//   - les carpetes de sincronització al núvol no es recorren si no s'hi ha
//     demanat la cerca expressament;
//   - hi ha un sostre de fitxers i de temps: una cerca no corre hores.

const (
	walkMaxFiles = 20000
	walkMaxTime  = 30 * time.Second
)

// skipReadExts no s'obren per buscar-hi text: no en tenen.
var skipReadExts = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".webp": true, ".bmp": true, ".ico": true,
	".tif": true, ".tiff": true, ".heic": true, ".heif": true, ".raw": true, ".dng": true, ".cr2": true, ".nef": true, ".psd": true,
	".mp3": true, ".wav": true, ".ogg": true, ".flac": true, ".m4a": true, ".aac": true, ".opus": true,
	".mp4": true, ".mov": true, ".avi": true, ".mkv": true, ".webm": true, ".m4v": true, ".3gp": true,
	".zip": true, ".gz": true, ".tgz": true, ".7z": true, ".rar": true, ".tar": true, ".xz": true, ".bz2": true, ".iso": true, ".dmg": true,
	".exe": true, ".dll": true, ".so": true, ".dylib": true, ".bin": true, ".o": true, ".a": true, ".class": true, ".jar": true,
	".pyc": true, ".wasm": true, ".db": true, ".sqlite": true, ".pdf": true, ".docx": true, ".xlsx": true, ".pptx": true,
	".ttf": true, ".otf": true, ".woff": true, ".woff2": true, ".apk": true, ".msi": true,
}

// cloudDirs són carpetes de sincronització al núvol: plenes de fitxers que
// només hi són de nom i que obrir-los vol dir baixar-los.
var cloudDirs = map[string]bool{
	"nextcloud": true, "onedrive": true, "dropbox": true, "icloud drive": true, "iclouddrive": true,
	"google drive": true, "googledrive": true, "my drive": true, "box": true, "pcloud drive": true, "megasync": true,
}

// isCloudDir: el nom d'una carpeta de núvol («OneDrive - Empresa» també).
func isCloudDir(name string) bool {
	n := strings.ToLower(strings.TrimSpace(name))
	if cloudDirs[n] {
		return true
	}
	for _, p := range []string{"onedrive - ", "nextcloud", "dropbox ("} {
		if strings.HasPrefix(n, p) {
			return true
		}
	}
	return false
}

// skipRead: aquest fitxer no s'ha d'obrir per llegir-lo.
func skipRead(d fs.DirEntry) bool {
	if skipReadExts[strings.ToLower(filepath.Ext(d.Name()))] {
		return true
	}
	info, err := d.Info()
	if err != nil {
		return true
	}
	return onlineOnly(info)
}

// walkBudget compta fitxers i temps d'un recorregut.
type walkBudget struct {
	files int
	start time.Time
	hit   bool
}

func newWalkBudget() *walkBudget { return &walkBudget{start: time.Now()} }

// spend torna fals quan s'ha acabat el pressupost.
func (b *walkBudget) spend() bool {
	b.files++
	if b.files > walkMaxFiles || time.Since(b.start) > walkMaxTime {
		b.hit = true
		return false
	}
	return true
}
