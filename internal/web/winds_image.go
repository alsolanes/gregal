package web

import (
	"bytes"
	"context"
	cryptorand "crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"gregal/internal/agent"
	"gregal/internal/config"
	"gregal/internal/llm"
)

// Imatges dels vents: la Llevantada et mostra com quedaria el plat, el Ponent
// et fa el cartell de la festa major. L'eina image genera la imatge, la desa
// al servidor (no caduca, com els missatges) i el vent la posa a la resposta
// com ![títol](/api/winds/image?id=…). La web i l'Android la baixen amb la
// clau, igual que els documents.
//
// D'on surt la imatge:
//   - roles.image (provider + model) del config: un endpoint compatible amb
//     OpenAI (/images/generations: OpenAI, LiteLLM, LocalAI, sd-server) o, si
//     no el té, un xat amb modalities ["image"] (OpenRouter, Gemini).
//   - Sense rol: pollinations.ai, gratuït i sense clau, perquè funcioni el
//     primer dia. roles.image.model: off ho apaga del tot.

const imageToolTimeout = 3 * time.Minute

// maxImageBytes: una imatge generada de debò no passa de pocs MB.
const maxImageBytes = 12 << 20

var imageToolSpec = llm.ToolSpec{
	Name: "image",
	Description: "Create a picture for the user: what a dish would look like, a poster, an illustration, a sketch of an idea, a birthday card. " +
		"Write prompt in English as a detailed visual description (subject, style, composition, colours, mood); never put real people's faces, logos or text you cannot spell exactly. " +
		"shape: square (default), portrait or landscape. Pass your wind id and a short title in the user's language. " +
		"The result gives a markdown line: put it in your reply exactly as given, on its own line, and add one short sentence. " +
		"Only when the user asks for a picture or when a picture clearly helps; never to decorate.",
	Parameters: map[string]any{"type": "object", "properties": map[string]any{
		"wind":   map[string]any{"type": "string"},
		"prompt": map[string]any{"type": "string"},
		"title":  map[string]any{"type": "string"},
		"shape":  map[string]any{"type": "string", "enum": []string{"square", "portrait", "landscape"}},
	}, "required": []string{"prompt"}},
}

type imageArgs struct {
	Wind   string `json:"wind"`
	Prompt string `json:"prompt"`
	Title  string `json:"title"`
	Shape  string `json:"shape"`
}

// pollinationsBase es pot canviar als tests.
var pollinationsBase = "https://image.pollinations.ai/prompt/"

var imageHTTP = &http.Client{Timeout: imageToolTimeout}

func (s *Server) registerImageTool() {
	agent.RegisterExtraCtx(imageToolSpec, func(ctx context.Context, argsJSON string) (string, error) {
		var a imageArgs
		if err := json.Unmarshal([]byte(argsJSON), &a); err != nil {
			return "", errors.New("unreadable arguments")
		}
		target := s
		if cur, ok := ctx.Value(remindServerKey{}).(*Server); ok && cur != nil {
			target = cur
		}
		return target.imageDo(ctx, a)
	})
}

func (s *Server) imageDo(ctx context.Context, a imageArgs) (string, error) {
	prompt := strings.TrimSpace(a.Prompt)
	if prompt == "" {
		return "", errors.New("empty prompt: describe the picture")
	}
	if r := []rune(prompt); len(r) > 1500 {
		prompt = string(r[:1500])
	}
	ctx, cancel := context.WithTimeout(ctx, imageToolTimeout)
	defer cancel()
	raw, ext, via, err := s.generateImage(ctx, prompt, a.Shape)
	if err != nil {
		return "", err
	}
	id, err := s.saveWindImage(raw, ext)
	if err != nil {
		return "", fmt.Errorf("could not save the picture: %w", err)
	}
	title := strings.TrimSpace(a.Title)
	if title == "" {
		title = s.tr("imatge", "picture")
	}
	title = strings.NewReplacer("[", "(", "]", ")", "\n", " ").Replace(title)
	return fmt.Sprintf("Picture ready (%s). Put this line in your reply exactly as it is:\n![%s](%s)", via, title, windImageLink(id)), nil
}

func windImageLink(id string) string { return "/api/winds/image?id=" + id }

// imageBackend: el rol image del config, o "" si no n'hi ha (pollinations)
// i off si està apagat.
func (s *Server) imageBackend() (config.Role, config.Provider, string) {
	if s.cfg == nil {
		return config.Role{}, config.Provider{}, ""
	}
	role, ok := s.cfg.Roles["image"]
	if !ok || strings.TrimSpace(role.Model) == "" {
		return role, config.Provider{}, ""
	}
	if strings.EqualFold(strings.TrimSpace(role.Model), "off") {
		return role, config.Provider{}, "off"
	}
	p, ok := s.cfg.Providers[role.Provider]
	if !ok || strings.TrimSpace(p.BaseURL) == "" {
		return role, config.Provider{}, ""
	}
	return role, p, "role"
}

func (s *Server) generateImage(ctx context.Context, prompt, shape string) ([]byte, string, string, error) {
	role, p, kind := s.imageBackend()
	switch kind {
	case "off":
		return nil, "", "", errors.New("pictures are switched off on this server (roles.image.model: off); tell the user you cannot draw here")
	case "role":
		raw, err := openAIImage(ctx, p, role.Model, prompt, shape)
		if err != nil {
			// Molts proveïdors (OpenRouter, Gemini) fan imatges pel xat.
			raw2, err2 := chatImage(ctx, p, role.Model, prompt)
			if err2 != nil {
				return nil, "", "", fmt.Errorf("image model %s failed: %v", role.Model, err)
			}
			raw = raw2
		}
		return raw, imageExt(raw), role.Model, nil
	}
	raw, err := pollinationsImage(ctx, prompt, shape)
	if err != nil {
		return nil, "", "", err
	}
	return raw, imageExt(raw), "pollinations.ai", nil
}

func shapeSize(shape string) (w, h int) {
	switch shape {
	case "portrait":
		return 1024, 1536
	case "landscape":
		return 1536, 1024
	}
	return 1024, 1024
}

func openAIImage(ctx context.Context, p config.Provider, model, prompt, shape string) ([]byte, error) {
	w, h := shapeSize(shape)
	body, _ := json.Marshal(map[string]any{"model": model, "prompt": prompt, "n": 1, "size": fmt.Sprintf("%dx%d", w, h)})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(p.BaseURL, "/")+"/images/generations", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if p.HasAPIKey() {
		req.Header.Set("Authorization", "Bearer "+p.APIKey)
	}
	resp, err := imageHTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 3*maxImageBytes))
	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, imgClip(string(data)))
	}
	var out struct {
		Data []struct {
			B64 string `json:"b64_json"`
			URL string `json:"url"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &out); err != nil || len(out.Data) == 0 {
		return nil, errors.New("the image model returned no picture")
	}
	if out.Data[0].B64 != "" {
		return decodeImageB64(out.Data[0].B64)
	}
	return fetchImage(ctx, out.Data[0].URL)
}

// chatImage: proveïdors que tornen la imatge dins del missatge del xat
// (message.images[].image_url.url, una data URL).
func chatImage(ctx context.Context, p config.Provider, model, prompt string) ([]byte, error) {
	body, _ := json.Marshal(map[string]any{"model": model, "modalities": []string{"image", "text"},
		"messages": []map[string]any{{"role": "user", "content": prompt}}})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(p.BaseURL, "/")+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if p.HasAPIKey() {
		req.Header.Set("Authorization", "Bearer "+p.APIKey)
	}
	resp, err := imageHTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 3*maxImageBytes))
	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, imgClip(string(data)))
	}
	var out struct {
		Choices []struct {
			Message struct {
				Images []struct {
					ImageURL struct {
						URL string `json:"url"`
					} `json:"image_url"`
				} `json:"images"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(data, &out); err != nil || len(out.Choices) == 0 || len(out.Choices[0].Message.Images) == 0 {
		return nil, errors.New("the model returned no picture")
	}
	u := out.Choices[0].Message.Images[0].ImageURL.URL
	if i := strings.Index(u, ";base64,"); strings.HasPrefix(u, "data:") && i > 0 {
		return decodeImageB64(u[i+len(";base64,"):])
	}
	return fetchImage(ctx, u)
}

// El servei gratuït fa una imatge cada 45 s per IP: les crides van una
// darrere l'altra, i si diu que encara no toca (402 FREE_LIMIT, amb «Retry
// after Ns») o falla de passada (5xx), s'espera i es torna a provar.
var (
	pollinationsMu   sync.Mutex
	pollinationsLast time.Time
	pollinationsGap  = 46 * time.Second
	pollinationsWait = func(d time.Duration) time.Duration { return d }
)

var retryAfterRe = regexp.MustCompile(`(?i)retry after (\d+)\s*s`)

func pollinationsImage(ctx context.Context, prompt, shape string) ([]byte, error) {
	w, h := shapeSize(shape)
	// Una mica més petites: pollinations va més de pressa i el mòbil no en vol més.
	w, h = w*3/4, h*3/4
	pollinationsMu.Lock()
	defer pollinationsMu.Unlock()
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if wait := time.Until(pollinationsLast.Add(pollinationsGap)); wait > 0 {
			if err := sleepCtx(ctx, pollinationsWait(wait)); err != nil {
				return nil, err
			}
		}
		seed := time.Now().UnixNano() % 1000000
		u := pollinationsBase + url.PathEscape(prompt) + fmt.Sprintf("?width=%d&height=%d&nologo=true&seed=%d", w, h, seed)
		raw, err := fetchImage(ctx, u)
		pollinationsLast = time.Now()
		if err == nil {
			return raw, nil
		}
		lastErr = err
		var he *imageHTTPError
		if !errors.As(err, &he) {
			break
		}
		switch {
		case he.Status == http.StatusPaymentRequired || he.Status == http.StatusTooManyRequests:
			secs := 45
			if m := retryAfterRe.FindStringSubmatch(he.Body); m != nil {
				fmt.Sscan(m[1], &secs)
			}
			if secs > 60 {
				return nil, fmt.Errorf("the free picture service (pollinations.ai) is busy; try again in a minute")
			}
			pollinationsLast = time.Now().Add(time.Duration(secs+1)*time.Second - pollinationsGap)
		case he.Status >= 500:
			pollinationsLast = time.Now().Add(3*time.Second - pollinationsGap)
		default:
			attempt = 3
		}
	}
	return nil, fmt.Errorf("the free picture service (pollinations.ai) failed: %v", lastErr)
}

func sleepCtx(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

type imageHTTPError struct {
	Status int
	Body   string
}

func (e *imageHTTPError) Error() string { return fmt.Sprintf("HTTP %d: %s", e.Status, imgClip(e.Body)) }

func fetchImage(ctx context.Context, u string) ([]byte, error) {
	if !strings.HasPrefix(u, "http://") && !strings.HasPrefix(u, "https://") {
		return nil, errors.New("no picture URL")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Gregal")
	resp, err := imageHTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, &imageHTTPError{Status: resp.StatusCode, Body: string(body)}
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxImageBytes+1))
	if err != nil {
		return nil, err
	}
	if len(raw) > maxImageBytes {
		return nil, errors.New("picture too large")
	}
	if imageExt(raw) == "" {
		return nil, errors.New("the answer was not a picture")
	}
	return raw, nil
}

func decodeImageB64(s string) ([]byte, error) {
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(s))
	if err != nil {
		return nil, errors.New("unreadable picture")
	}
	if len(raw) > maxImageBytes || imageExt(raw) == "" {
		return nil, errors.New("the answer was not a picture")
	}
	return raw, nil
}

// imageExt mira la signatura: només desem el que és de debò una imatge.
func imageExt(raw []byte) string {
	switch {
	case bytes.HasPrefix(raw, []byte{0xFF, 0xD8, 0xFF}):
		return "jpg"
	case bytes.HasPrefix(raw, []byte("\x89PNG\r\n\x1a\n")):
		return "png"
	case len(raw) > 12 && string(raw[:4]) == "RIFF" && string(raw[8:12]) == "WEBP":
		return "webp"
	}
	return ""
}

func (s *Server) windImagesDir() string { return filepath.Join(s.windStateDir(), "wind-images") }

func (s *Server) saveWindImage(raw []byte, ext string) (string, error) {
	if ext == "" {
		return "", errors.New("not a picture")
	}
	b := make([]byte, 12)
	if _, err := cryptorand.Read(b); err != nil {
		return "", err
	}
	id := hex.EncodeToString(b) + "." + ext
	dir := s.windImagesDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return id, os.WriteFile(filepath.Join(dir, id), raw, 0o644)
}

var windImageID = regexp.MustCompile(`^[a-f0-9]{24}\.(jpg|png|webp)$`)

// handleWindImage: GET /api/winds/image?id=<id> serveix una imatge d'un vent.
func (s *Server) handleWindImage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "GET required", http.StatusMethodNotAllowed)
		return
	}
	id := r.URL.Query().Get("id")
	if !windImageID.MatchString(id) {
		http.Error(w, "imatge invàlida", http.StatusBadRequest)
		return
	}
	p := filepath.Join(s.windImagesDir(), id)
	if _, err := os.Stat(p); err != nil {
		http.Error(w, "imatge inexistent", http.StatusNotFound)
		return
	}
	ct := map[string]string{"jpg": "image/jpeg", "png": "image/png", "webp": "image/webp"}[strings.TrimPrefix(filepath.Ext(id), ".")]
	w.Header().Set("Content-Type", ct)
	w.Header().Set("Cache-Control", "private, max-age=31536000, immutable")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	http.ServeFile(w, r, p)
}

// windImageLinks: les imatges fetes des de since que el vent no ha posat a
// la resposta (se n'ha oblidat): l'app les afegeix al final.
func (s *Server) windImageLinks(since time.Time, reply string) string {
	entries, err := os.ReadDir(s.windImagesDir())
	if err != nil {
		return ""
	}
	since = since.Add(-time.Second)
	var b strings.Builder
	for _, e := range entries {
		if e.IsDir() || !windImageID.MatchString(e.Name()) || strings.Contains(reply, e.Name()) {
			continue
		}
		if fi, err := e.Info(); err == nil && !fi.ModTime().Before(since) {
			fmt.Fprintf(&b, "\n\n![%s](%s)", s.tr("imatge", "picture"), windImageLink(e.Name()))
		}
	}
	return b.String()
}

func imgClip(s string) string {
	if r := []rune(strings.TrimSpace(s)); len(r) > 300 {
		return string(r[:300]) + "…"
	}
	return strings.TrimSpace(s)
}
