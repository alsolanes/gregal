package web

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// El catàleg models.dev (el que fa servir OpenCode) dona, per cada model, el
// nom llegible («DeepSeek V4.1 Flash») i què sap fer amb el raonament (quins
// nivells d'esforç admet). L'endpoint /models dels proveïdors només torna
// l'id. Es baixa en segon pla, es guarda una versió reduïda al disc i, sense
// xarxa, tot continua funcionant amb els noms que la UI deriva de l'id.

const modelsDevURL = "https://models.dev/api.json"
const modelsDevTTL = 24 * time.Hour

type modelInfo struct {
	Name      string   `json:"name"`
	Reasoning bool     `json:"reasoning"`
	Efforts   []string `json:"efforts,omitempty"`
}

var modelsDev struct {
	mu      sync.Mutex
	idx     map[string]modelInfo
	fetched time.Time
	loading bool
}

// parseModelsDev redueix el JSON enorme (6 MB) a id → nom i raonament. Si un
// id surt a diversos proveïdors es queda amb el primer que té nom.
func parseModelsDev(raw []byte) (map[string]modelInfo, error) {
	var all map[string]struct {
		Models map[string]struct {
			ID               string `json:"id"`
			Name             string `json:"name"`
			Reasoning        bool   `json:"reasoning"`
			ReasoningOptions []struct {
				Type   string   `json:"type"`
				Values []string `json:"values"`
			} `json:"reasoning_options"`
		} `json:"models"`
	}
	if err := json.Unmarshal(raw, &all); err != nil {
		return nil, err
	}
	idx := map[string]modelInfo{}
	for _, p := range all {
		for key, m := range p.Models {
			id := strings.ToLower(strings.TrimSpace(m.ID))
			if id == "" {
				id = strings.ToLower(key)
			}
			if strings.TrimSpace(m.Name) == "" {
				continue
			}
			if _, dup := idx[id]; dup {
				continue
			}
			info := modelInfo{Name: strings.TrimSpace(m.Name), Reasoning: m.Reasoning}
			for _, o := range m.ReasoningOptions {
				if o.Type == "effort" {
					info.Efforts = append(info.Efforts, o.Values...)
				}
			}
			idx[id] = info
		}
	}
	return idx, nil
}

func modelsDevCachePath() string {
	if d := strings.TrimSpace(os.Getenv("GREGAL_DATA_DIR")); d != "" {
		return filepath.Join(d, "models-dev.json")
	}
	dir, err := os.UserCacheDir()
	if err != nil || dir == "" {
		return ""
	}
	return filepath.Join(dir, "gregal", "models-dev.json")
}

// modelInfoFor busca un model pel seu id (amb o sense prefix de proveïdor).
// Mai bloqueja: si encara no hi ha catàleg, en demana un en segon pla.
func modelInfoFor(id string) (modelInfo, bool) {
	modelsDevEnsure()
	id = strings.ToLower(strings.TrimSpace(id))
	if id == "" {
		return modelInfo{}, false
	}
	modelsDev.mu.Lock()
	defer modelsDev.mu.Unlock()
	if m, ok := modelsDev.idx[id]; ok {
		return m, true
	}
	if i := strings.LastIndex(id, "/"); i >= 0 {
		m, ok := modelsDev.idx[id[i+1:]]
		return m, ok
	}
	return modelInfo{}, false
}

func modelsDevEnsure() {
	modelsDev.mu.Lock()
	need := !modelsDev.loading && (modelsDev.idx == nil || time.Since(modelsDev.fetched) > modelsDevTTL)
	if need {
		modelsDev.loading = true
	}
	modelsDev.mu.Unlock()
	if !need || os.Getenv("GREGAL_NO_MODELS_DEV") != "" {
		if need {
			modelsDev.mu.Lock()
			modelsDev.loading = false
			modelsDev.mu.Unlock()
		}
		return
	}
	goBackground(func() {
		defer func() { modelsDev.mu.Lock(); modelsDev.loading = false; modelsDev.mu.Unlock() }()
		// Disc primer: una versió recent no es torna a baixar.
		path := modelsDevCachePath()
		if st, err := os.Stat(path); err == nil && time.Since(st.ModTime()) < modelsDevTTL {
			if raw, err := os.ReadFile(path); err == nil {
				var idx map[string]modelInfo
				if json.Unmarshal(raw, &idx) == nil && len(idx) > 0 {
					modelsDev.mu.Lock()
					modelsDev.idx, modelsDev.fetched = idx, st.ModTime()
					modelsDev.mu.Unlock()
					return
				}
			}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
		defer cancel()
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, modelsDevURL, nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil || resp.StatusCode != 200 {
			if resp != nil {
				resp.Body.Close()
			}
			return
		}
		defer resp.Body.Close()
		var buf []byte
		buf, err = io.ReadAll(io.LimitReader(resp.Body, 32<<20))
		if err != nil {
			return
		}
		idx, err := parseModelsDev(buf)
		if err != nil || len(idx) == 0 {
			return
		}
		modelsDev.mu.Lock()
		modelsDev.idx, modelsDev.fetched = idx, time.Now()
		modelsDev.mu.Unlock()
		if path != "" {
			if small, err := json.Marshal(idx); err == nil {
				_ = os.MkdirAll(filepath.Dir(path), 0o700)
				_ = os.WriteFile(path, small, 0o600)
			}
		}
	})
}

// thinkOptionsFor dona els nivells de raonament que té sentit oferir per a un
// model: res si el catàleg diu que no raona; si no, els nostres (per defecte,
// auto, no) més els esforços que el model declara (o els habituals).
func thinkOptionsFor(id string) []string {
	info, ok := modelInfoFor(id)
	if ok && !info.Reasoning {
		return nil
	}
	out := []string{"default", "auto", "no"}
	efforts := []string{"low", "medium", "high", "max"}
	if ok && len(info.Efforts) > 0 {
		efforts = nil
		for _, e := range info.Efforts {
			if validThinkLevel(strings.ToLower(e)) {
				efforts = append(efforts, strings.ToLower(e))
			}
		}
	}
	return append(out, efforts...)
}

// modelDisplayName és el nom del catàleg, o buit (la UI llavors el treu de l'id).
func modelDisplayName(id string) string {
	if m, ok := modelInfoFor(id); ok {
		return m.Name
	}
	return ""
}

// modelNames dona id → nom per als models que el catàleg coneix.
func modelNames(models map[string][]string) map[string]string {
	out := map[string]string{}
	for _, ids := range models {
		for _, id := range ids {
			if n := modelDisplayName(id); n != "" {
				out[id] = n
			}
		}
	}
	return out
}
