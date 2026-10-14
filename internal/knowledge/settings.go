package knowledge

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"sync"
	"time"
)

// EmbedCfg és el servei d'embeddings opcional (OpenAI-compatible:
// POST <base_url>/embeddings). Sense Model, només BM25.
type EmbedCfg struct {
	Provider string
	BaseURL  string
	APIKey   string
	Model    string
}

// Settings són els límits i l'embedding. Zero = valor per defecte.
type Settings struct {
	MaxFiles   int // fitxers per col·lecció (2000)
	MaxTotalMB int // MB llegits per col·lecció (200)
	MaxFileMB  int // MB per fitxer (25)
	ChunkChars int // caràcters per fragment (1500)
	Overlap    int // solapament (200)
	Embed      EmbedCfg
}

var (
	settingsMu sync.RWMutex
	settings   Settings
)

// Configure fixa els límits i l'embedding (el config ho crida a Load).
func Configure(s Settings) {
	settingsMu.Lock()
	settings = s
	settingsMu.Unlock()
}

// Current torna la configuració efectiva, amb els valors per defecte.
func Current() Settings {
	settingsMu.RLock()
	s := settings
	settingsMu.RUnlock()
	if s.MaxFiles <= 0 {
		s.MaxFiles = 2000
	}
	if s.MaxTotalMB <= 0 {
		s.MaxTotalMB = 200
	}
	if s.MaxFileMB <= 0 {
		s.MaxFileMB = 25
	}
	if s.ChunkChars <= 0 {
		s.ChunkChars = 1500
	}
	if s.Overlap <= 0 {
		s.Overlap = 200
	}
	return s
}

// EmbeddingsOn diu si hi ha un model d'embeddings configurat.
func (s Settings) EmbeddingsOn() bool {
	return strings.TrimSpace(s.Embed.Model) != "" && strings.TrimSpace(s.Embed.BaseURL) != ""
}

var embedClient = &http.Client{Timeout: 60 * time.Second}

// embedTexts demana els vectors d'uns textos, en lots.
func embedTexts(ctx context.Context, cfg EmbedCfg, texts []string) ([][]float32, error) {
	const batch = 64
	out := make([][]float32, 0, len(texts))
	for i := 0; i < len(texts); i += batch {
		j := min(len(texts), i+batch)
		vecs, err := embedBatch(ctx, cfg, texts[i:j])
		if err != nil {
			return out, err
		}
		out = append(out, vecs...)
	}
	return out, nil
}

func embedBatch(ctx context.Context, cfg EmbedCfg, texts []string) ([][]float32, error) {
	in := make([]string, len(texts))
	for i, t := range texts {
		if len(t) > 8000 {
			t = t[:8000]
		}
		in[i] = t
	}
	body, _ := json.Marshal(map[string]any{"model": cfg.Model, "input": in})
	url := strings.TrimSuffix(strings.TrimSpace(cfg.BaseURL), "/") + "/embeddings"
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if k := strings.TrimSpace(cfg.APIKey); k != "" {
		req.Header.Set("Authorization", "Bearer "+k)
	}
	resp, err := embedClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
	if resp.StatusCode/100 != 2 {
		msg := strings.TrimSpace(string(raw))
		if len(msg) > 200 {
			msg = msg[:200]
		}
		return nil, fmt.Errorf("embeddings: HTTP %d %s", resp.StatusCode, msg)
	}
	var r struct {
		Data []struct {
			Index     int       `json:"index"`
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &r); err != nil {
		return nil, fmt.Errorf("embeddings: %w", err)
	}
	if len(r.Data) != len(texts) {
		return nil, errors.New("embeddings: the provider returned a different number of vectors")
	}
	out := make([][]float32, len(texts))
	for k, d := range r.Data {
		idx := d.Index
		if idx < 0 || idx >= len(out) {
			idx = k
		}
		out[idx] = normalize(d.Embedding)
	}
	return out, nil
}

func normalize(v []float32) []float32 {
	var s float64
	for _, x := range v {
		s += float64(x) * float64(x)
	}
	if s == 0 {
		return v
	}
	n := float32(1 / math.Sqrt(s))
	out := make([]float32, len(v))
	for i, x := range v {
		out[i] = x * n
	}
	return out
}

func dot(a, b []float32) float64 {
	if len(a) != len(b) {
		return 0
	}
	var s float64
	for i := range a {
		s += float64(a[i]) * float64(b[i])
	}
	return s
}
