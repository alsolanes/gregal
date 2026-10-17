// Package catalog dona finestra, sortida màxima, preus i capacitats dels
// models remots a partir de models.dev, el catàleg obert que fa servir
// opencode (232 proveïdors el 2026-10).
//
// Sense catàleg, Gregal només sabia la finestra d'un model si el proveïdor
// la declarava a /models, o per una taula escrita a mà (agent/window.go):
// deepseek-v4.1-flash a opencode zen quedava a 128K quan en té 1M, i la
// compactació arribava molt abans del que calia. El cost no es mostrava mai
// si l'usuari no havia posat preus al config.
//
// Funcionament:
//   - Es desa una còpia retallada (només el que fem servir) a
//     <data>/models-catalog.json i es refresca en segon pla si té més d'un
//     dia. Si encara no n'hi ha cap, la primera consulta espera la baixada
//     com a molt uns segons.
//   - Sense xarxa, o amb `models_catalog: off`, no hi ha catàleg i tot va
//     com abans (taula fixa i preus del config).
//   - Sota `go test` no es llegeix ni s'escriu res si no hi ha override.
package catalog

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// SourceURL és d'on es baixa el catàleg.
var SourceURL = "https://models.dev/api.json"

// TTL és cada quant es refresca la còpia del disc.
const TTL = 24 * time.Hour

// Model és el que Gregal fa servir d'una entrada del catàleg.
type Model struct {
	Context    int     `json:"context,omitempty"`
	Output     int     `json:"output,omitempty"`
	In         float64 `json:"in,omitempty"`  // USD per milió de tokens d'entrada
	Out        float64 `json:"out,omitempty"` // USD per milió de tokens de sortida
	CacheRead  float64 `json:"cache_read,omitempty"`
	ToolCall   bool    `json:"tool_call,omitempty"`
	Reasoning  bool    `json:"reasoning,omitempty"`
	Attachment bool    `json:"attachment,omitempty"`
	HasCost    bool    `json:"has_cost,omitempty"`
}

// Provider és un proveïdor del catàleg: l'adreça de l'API i els models.
type Provider struct {
	API    string           `json:"api,omitempty"`
	Models map[string]Model `json:"models"`
}

// Data és la còpia retallada que es desa al disc.
type Data struct {
	Fetched   time.Time           `json:"fetched"`
	Providers map[string]Provider `json:"providers"`
}

var (
	mu         sync.Mutex
	enabled    bool
	path       string
	loaded     bool
	data       *Data
	refrescant bool
	// PathOverride redirigeix el fitxer (tests).
	PathOverride string
)

// Configure activa o desactiva el catàleg. Es crida en arrencar (CLI, TUI,
// web, Telegram) amb el config ja carregat. dataDir buit = el de per
// defecte (GREGAL_DATA_DIR o ~/.local/share/gregal).
func Configure(on bool, dataDir string) {
	mu.Lock()
	defer mu.Unlock()
	enabled = on
	path = defaultPath(dataDir)
	loaded, data = false, nil
}

func defaultPath(dataDir string) string {
	if PathOverride != "" {
		return PathOverride
	}
	if testing.Testing() {
		return ""
	}
	if dataDir == "" {
		dataDir = strings.TrimSpace(os.Getenv("GREGAL_DATA_DIR"))
	}
	if dataDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		dataDir = filepath.Join(home, ".local", "share", "gregal")
	}
	return filepath.Join(dataDir, "models-catalog.json")
}

// Set posa un catàleg en memòria (tests i Refresh).
func Set(d *Data) {
	mu.Lock()
	defer mu.Unlock()
	enabled, loaded, data = true, true, d
}

// Reset torna a l'estat inicial: desactivat i sense dades (tests).
func Reset() {
	mu.Lock()
	defer mu.Unlock()
	enabled, loaded, data, path = false, false, nil, ""
}

// get torna el catàleg, carregant-lo del disc la primera vegada. Si no n'hi
// ha o és vell, el refresca: en segon pla si n'hi ha un de vell, i esperant
// com a molt `espera` si no n'hi ha cap.
func get() *Data {
	mu.Lock()
	if !enabled {
		mu.Unlock()
		return nil
	}
	if !loaded {
		loaded = true
		if path != "" {
			if raw, err := os.ReadFile(path); err == nil {
				var d Data
				if json.Unmarshal(raw, &d) == nil && len(d.Providers) > 0 {
					data = &d
				}
			}
		}
	}
	d, p := data, path
	vell := d == nil || time.Since(d.Fetched) > TTL
	if !vell || p == "" || refrescant {
		mu.Unlock()
		return d
	}
	refrescant = true
	mu.Unlock()

	fet := make(chan struct{})
	go func() {
		defer close(fet)
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		nou, err := Fetch(ctx, SourceURL)
		mu.Lock()
		refrescant = false
		if err == nil {
			data = nou
			if raw, err := json.Marshal(nou); err == nil {
				_ = os.MkdirAll(filepath.Dir(p), 0o700)
				tmp := p + ".tmp"
				if os.WriteFile(tmp, raw, 0o600) == nil {
					_ = os.Rename(tmp, p)
				}
			}
		}
		mu.Unlock()
	}()
	if d != nil {
		return d
	}
	// Primera vegada: val la pena esperar una mica, perquè el torn que
	// arrenca ja faci servir la finestra bona.
	select {
	case <-fet:
	case <-time.After(4 * time.Second):
	}
	mu.Lock()
	defer mu.Unlock()
	return data
}

// raw és el format de models.dev (només els camps que es fan servir).
type rawProvider struct {
	API    string              `json:"api"`
	Models map[string]rawModel `json:"models"`
}

type rawModel struct {
	ToolCall   bool `json:"tool_call"`
	Reasoning  bool `json:"reasoning"`
	Attachment bool `json:"attachment"`
	Limit      struct {
		Context int `json:"context"`
		Output  int `json:"output"`
	} `json:"limit"`
	Cost *struct {
		Input     float64 `json:"input"`
		Output    float64 `json:"output"`
		CacheRead float64 `json:"cache_read"`
	} `json:"cost"`
}

// Fetch baixa i retalla el catàleg.
func Fetch(ctx context.Context, src string) (*Data, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, src, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "gregal")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("catàleg: HTTP %d", resp.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
	if err != nil {
		return nil, err
	}
	return Parse(raw)
}

// Parse converteix el JSON de models.dev a la forma retallada.
func Parse(raw []byte) (*Data, error) {
	var in map[string]rawProvider
	if err := json.Unmarshal(raw, &in); err != nil {
		return nil, err
	}
	d := &Data{Fetched: time.Now(), Providers: map[string]Provider{}}
	for id, p := range in {
		out := Provider{API: p.API, Models: map[string]Model{}}
		for mid, m := range p.Models {
			mm := Model{
				Context: m.Limit.Context, Output: m.Limit.Output,
				ToolCall: m.ToolCall, Reasoning: m.Reasoning, Attachment: m.Attachment,
			}
			if m.Cost != nil {
				mm.In, mm.Out, mm.CacheRead, mm.HasCost = m.Cost.Input, m.Cost.Output, m.Cost.CacheRead, true
			}
			out.Models[mid] = mm
		}
		d.Providers[id] = out
	}
	if len(d.Providers) == 0 {
		return nil, fmt.Errorf("catàleg buit")
	}
	return d, nil
}

// Lookup busca un model del proveïdor amb aquesta adreça. Primer el
// proveïdor del catàleg amb la mateixa màquina i el camí més semblant
// (opencode zen: opencode.ai/zen/go/v1 → opencode-go); si no n'hi ha cap
// que tingui el model, el model per nom a tot el catàleg, i en aquest cas
// només si totes les entrades coincideixen en la finestra (la més petita
// si no) i sense preus (no se sap qui cobra). Torna també d'on surt.
func Lookup(baseURL, model string) (Model, string, bool) {
	d := get()
	if d == nil || strings.TrimSpace(model) == "" {
		return Model{}, "", false
	}
	model = strings.TrimSpace(model)
	if pid := millorProveidor(d, baseURL, model); pid != "" {
		return d.Providers[pid].Models[model], pid, true
	}
	return perNom(d, model)
}

func millorProveidor(d *Data, baseURL, model string) string {
	u, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil || u.Host == "" {
		return ""
	}
	host := strings.TrimPrefix(strings.ToLower(u.Hostname()), "api.")
	segs := trossos(u.Path)
	millor, punts := "", -1
	for pid, p := range d.Providers {
		if _, ok := p.Models[model]; !ok || p.API == "" {
			continue
		}
		pu, err := url.Parse(p.API)
		if err != nil || strings.TrimPrefix(strings.ToLower(pu.Hostname()), "api.") != host {
			continue
		}
		n := 0
		ps := map[string]bool{}
		for _, s := range trossos(pu.Path) {
			ps[s] = true
		}
		for _, s := range segs {
			if ps[s] {
				n++
			}
		}
		if n > punts || (n == punts && pid < millor) {
			millor, punts = pid, n
		}
	}
	return millor
}

func trossos(p string) []string {
	var out []string
	for _, s := range strings.Split(strings.ToLower(p), "/") {
		if s != "" && s != "v1" {
			out = append(out, s)
		}
	}
	return out
}

func perNom(d *Data, model string) (Model, string, bool) {
	noms := []string{model}
	if i := strings.LastIndex(model, "/"); i >= 0 {
		noms = append(noms, model[i+1:])
	}
	var trobat Model
	n := 0
	for _, nom := range noms {
		for _, p := range d.Providers {
			m, ok := p.Models[nom]
			if !ok || m.Context <= 0 {
				continue
			}
			if n == 0 || m.Context < trobat.Context {
				trobat.Context = m.Context
			}
			if n == 0 || (m.Output > 0 && m.Output < trobat.Output) {
				trobat.Output = m.Output
			}
			trobat.ToolCall = trobat.ToolCall || m.ToolCall
			trobat.Reasoning = trobat.Reasoning || m.Reasoning
			trobat.Attachment = trobat.Attachment || m.Attachment
			n++
		}
		if n > 0 {
			return trobat, "", true
		}
	}
	return Model{}, "", false
}
