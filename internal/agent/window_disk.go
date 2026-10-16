package agent

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// Memòria dels sondejos de finestra entre processos. Cada `gregal -p` era
// un procés nou que tornava a demanar /models abans del primer pas (0,4-0,7 s
// contra opencode zen), i els proveïdors que no hi declaren la finestra
// (zen, OpenAI) no quedaven mai «coneguts»: el web també hi tornava cada deu
// minuts. Ara un sondeig bo d'un proveïdor remot es recorda un dia, amb les
// finestres que hagi dit. Els servidors locals no hi entren: la seva
// finestra és la que s'ha configurat en engegar-los i pot canviar d'un
// arrencada a l'altra.

// WindowsPathOverride redirigeix el fitxer (tests). Sota `go test`, sense
// override, no es llegeix ni s'escriu res.
var WindowsPathOverride string

const windowDiskTTL = 24 * time.Hour

type windowDisk struct {
	Probed  map[string]time.Time `json:"probed"`  // base → últim sondeig bo
	Windows map[string]int       `json:"windows"` // base|model → tokens
}

var (
	windowDiskMu     sync.Mutex
	windowDiskLoaded bool
	windowDiskProbed = map[string]time.Time{}
)

func windowsPath() string {
	if WindowsPathOverride != "" {
		return WindowsPathOverride
	}
	if testing.Testing() {
		return ""
	}
	if d := strings.TrimSpace(os.Getenv("GREGAL_DATA_DIR")); d != "" {
		return filepath.Join(d, "windows.json")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".local", "share", "gregal", "windows.json")
}

func llegeixWindowDisk(path string) windowDisk {
	var d windowDisk
	if raw, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(raw, &d)
	}
	if d.Probed == nil {
		d.Probed = map[string]time.Time{}
	}
	if d.Windows == nil {
		d.Windows = map[string]int{}
	}
	return d
}

// carregaWindowDisk passa al procés (una sola vegada) els sondejos recents
// del disc: les finestres a windowCache i l'hora a windowDiskProbed.
func carregaWindowDisk() {
	windowDiskMu.Lock()
	defer windowDiskMu.Unlock()
	if windowDiskLoaded {
		return
	}
	windowDiskLoaded = true
	path := windowsPath()
	if path == "" {
		return
	}
	d := llegeixWindowDisk(path)
	windowMu.Lock()
	defer windowMu.Unlock()
	for base, t := range d.Probed {
		if time.Since(t) >= windowDiskTTL {
			continue
		}
		windowDiskProbed[base] = t
		for k, n := range d.Windows {
			if strings.HasPrefix(k, base+"|") && n > 0 {
				if _, ja := windowCache[k]; !ja {
					windowCache[k] = n
				}
			}
		}
	}
}

// sondejatRecent diu si un proveïdor remot ja s'ha sondejat bé fa menys
// d'un dia (en aquest procés o en un altre).
func sondejatRecent(base string) bool {
	carregaWindowDisk()
	windowDiskMu.Lock()
	defer windowDiskMu.Unlock()
	t, ok := windowDiskProbed[base]
	return ok && time.Since(t) < windowDiskTTL
}

// recordaSondeig desa un sondeig bo d'un proveïdor remot (best-effort: si
// el fitxer no es pot escriure, el proper procés tornarà a preguntar).
func recordaSondeig(base string, ws map[string]int) {
	now := time.Now()
	windowDiskMu.Lock()
	defer windowDiskMu.Unlock()
	windowDiskProbed[base] = now
	path := windowsPath()
	if path == "" {
		return
	}
	d := llegeixWindowDisk(path)
	d.Probed[base] = now
	for k := range d.Windows {
		if strings.HasPrefix(k, base+"|") {
			delete(d.Windows, k)
		}
	}
	for id, n := range ws {
		d.Windows[windowKey(base, id)] = n
	}
	for b, t := range d.Probed {
		if time.Since(t) >= windowDiskTTL {
			delete(d.Probed, b)
		}
	}
	raw, err := json.Marshal(d)
	if err != nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	tmp := path + ".tmp"
	if os.WriteFile(tmp, raw, 0o644) == nil {
		_ = os.Rename(tmp, path)
	}
}

// oblidaWindowDisk buida l'estat en memòria del disc (tests).
func oblidaWindowDisk() {
	windowDiskMu.Lock()
	windowDiskLoaded = false
	windowDiskProbed = map[string]time.Time{}
	windowDiskMu.Unlock()
}
