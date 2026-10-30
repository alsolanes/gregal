package web

// La veu dels narradors de l'Arena, gratuïta i al propi ordinador.
//
//   - piper (per defecte): Piper, un motor de veu neuronal lliure que corre a
//     la CPU sense internet ni claus. L'organitzador prem «Instal·la les veus»
//     i Gregal baixa el motor i les veus de la llengua (català: Ona i Pau;
//     anglès: Ryan i Alan). El narrador parla més ràpid i amb més variació que
//     l'analista.
//   - http: qualsevol servidor de veu que parli l'API d'OpenAI
//     (/v1/audio/speech), autoallotjat: Chatterbox, Kokoro, Orpheus… Amb
//     «instructions» i camps extra (p. ex. l'«exaggeration» de Chatterbox) per
//     a una veu de comentarista esportiu.
//
// Cada frase es genera un sol cop, quan el narrador l'escriu (abans que la
// jugada surti a la pantalla), i es guarda: tothom que mira la retransmissió
// sent el mateix àudio, sense retard.

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"
)

type arenaVoiceCfg struct {
	Engine string            `json:"engine"` // piper (per defecte), http, off
	URL    string            `json:"url,omitempty"`
	Key    string            `json:"key,omitempty"`
	Model  string            `json:"model,omitempty"`
	Voices map[string]string `json:"voices,omitempty"` // caster, analyst (motor http)
	Extra  json.RawMessage   `json:"extra,omitempty"`  // camps afegits a la petició (http)
}

type piperVoice struct {
	Name   string  // ca_ES-upc_ona-medium
	Path   string  // ca/ca_ES/upc_ona/medium
	Length float64 // < 1 = més ràpid
	Noise  float64 // variació d'entonació
	NoiseW float64 // variació del ritme
}

// Les veus de cada llengua: el narrador (caster), més viu; l'analista, més pausat.
var piperVoices = map[string]map[string]piperVoice{
	"ca": {
		"caster":  {Name: "ca_ES-upc_ona-medium", Path: "ca/ca_ES/upc_ona/medium", Length: .84, Noise: .9, NoiseW: .95},
		"analyst": {Name: "ca_ES-upc_pau-x_low", Path: "ca/ca_ES/upc_pau/x_low", Length: 1, Noise: .667, NoiseW: .8},
	},
	"en": {
		"caster":  {Name: "en_US-ryan-high", Path: "en/en_US/ryan/high", Length: .86, Noise: .85, NoiseW: .95},
		"analyst": {Name: "en_GB-alan-medium", Path: "en/en_GB/alan/medium", Length: 1, Noise: .667, NoiseW: .8},
	},
}

const piperRelease = "https://github.com/rhasspy/piper/releases/download/2023.11.14-2/"
const piperVoicesURL = "https://huggingface.co/rhasspy/piper-voices/resolve/main/"

func piperArchive() string {
	switch runtime.GOOS + "/" + runtime.GOARCH {
	case "windows/amd64":
		return "piper_windows_amd64.zip"
	case "linux/amd64":
		return "piper_linux_x86_64.tar.gz"
	case "linux/arm64":
		return "piper_linux_aarch64.tar.gz"
	case "darwin/arm64":
		return "piper_macos_aarch64.tar.gz"
	case "darwin/amd64":
		return "piper_macos_x64.tar.gz"
	}
	return ""
}

func (s *Server) voiceDir() string { return filepath.Join(s.arenaDir(), "voice") }

func (s *Server) voiceCfg() arenaVoiceCfg {
	var st struct {
		Voice arenaVoiceCfg `json:"voice"`
	}
	arenaRead(filepath.Join(s.arenaDir(), "settings.json"), &st)
	if st.Voice.Engine == "" {
		st.Voice.Engine = "piper"
	}
	return st.Voice
}

func (s *Server) saveVoiceCfg(c arenaVoiceCfg) {
	arenaMu.Lock()
	defer arenaMu.Unlock()
	path := filepath.Join(s.arenaDir(), "settings.json")
	var raw map[string]json.RawMessage
	arenaRead(path, &raw)
	if raw == nil {
		raw = map[string]json.RawMessage{}
	}
	b, _ := json.Marshal(c)
	raw["voice"] = b
	arenaWrite(path, raw)
}

func (s *Server) piperExe() string {
	name := "piper"
	if runtime.GOOS == "windows" {
		name = "piper.exe"
	}
	return filepath.Join(s.voiceDir(), "piper", name)
}

func (s *Server) piperModel(v piperVoice) string {
	return filepath.Join(s.voiceDir(), "voices", v.Name+".onnx")
}

// voiceReady diu si la veu d'una llengua està a punt.
func (s *Server) voiceReady(lang string) bool {
	c := s.voiceCfg()
	switch c.Engine {
	case "http":
		return strings.TrimSpace(c.URL) != ""
	case "piper":
		if _, err := os.Stat(s.piperExe()); err != nil {
			return false
		}
		for _, v := range piperVoices[lang] {
			if _, err := os.Stat(s.piperModel(v)); err != nil {
				return false
			}
		}
		return true
	}
	return false
}

// ---- instal·lació de Piper ----

type voiceInstall struct {
	mu    sync.Mutex
	State string `json:"state"` // idle, installing, ready, error
	Step  string `json:"step"`
	Done  int64  `json:"done"`
	Total int64  `json:"total"`
	Error string `json:"error,omitempty"`
}

var voiceInst = &voiceInstall{State: "idle"}

func (vi *voiceInstall) set(f func(*voiceInstall)) {
	vi.mu.Lock()
	defer vi.mu.Unlock()
	f(vi)
}

type voiceInstallView struct {
	State string `json:"state"`
	Step  string `json:"step"`
	Done  int64  `json:"done"`
	Total int64  `json:"total"`
	Error string `json:"error,omitempty"`
}

func (vi *voiceInstall) snapshot() voiceInstallView {
	vi.mu.Lock()
	defer vi.mu.Unlock()
	return voiceInstallView{State: vi.State, Step: vi.Step, Done: vi.Done, Total: vi.Total, Error: vi.Error}
}

type countingWriter struct{ n *int64 }

func (c countingWriter) Write(p []byte) (int, error) {
	voiceInst.set(func(v *voiceInstall) { v.Done += int64(len(p)) })
	return len(p), nil
}

var voiceHTTP = &http.Client{Timeout: 30 * time.Minute}

func download(ctx context.Context, url string) ([]byte, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	resp, err := voiceHTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("%s: HTTP %d", filepath.Base(url), resp.StatusCode)
	}
	voiceInst.set(func(v *voiceInstall) { v.Step, v.Done, v.Total = filepath.Base(url), 0, resp.ContentLength })
	var buf bytes.Buffer
	if _, err := io.Copy(io.MultiWriter(&buf, countingWriter{}), resp.Body); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// installVoices baixa Piper (si cal) i les veus d'una llengua.
func (s *Server) installVoices(ctx context.Context, lang string) error {
	arch := piperArchive()
	if arch == "" {
		return fmt.Errorf("Piper is not available for %s/%s", runtime.GOOS, runtime.GOARCH)
	}
	dir := s.voiceDir()
	if _, err := os.Stat(s.piperExe()); err != nil {
		data, err := download(ctx, piperRelease+arch)
		if err != nil {
			return err
		}
		if err := extractArchive(data, arch, dir); err != nil {
			return err
		}
		_ = os.Chmod(s.piperExe(), 0o755)
	}
	for _, v := range piperVoices[lang] {
		model := s.piperModel(v)
		if _, err := os.Stat(model); err == nil {
			continue
		}
		_ = os.MkdirAll(filepath.Dir(model), 0o700)
		for _, ext := range []string{".onnx.json", ".onnx"} {
			data, err := download(ctx, piperVoicesURL+v.Path+"/"+v.Name+ext+"?download=true")
			if err != nil {
				return err
			}
			if err := os.WriteFile(filepath.Join(filepath.Dir(model), v.Name+ext), data, 0o600); err != nil {
				return err
			}
		}
	}
	return nil
}

// extractArchive desempaqueta el zip o el tar.gz de Piper (només dins de dir).
func extractArchive(data []byte, name, dir string) error {
	safe := func(p string) (string, bool) {
		out := filepath.Join(dir, filepath.FromSlash(p))
		rel, err := filepath.Rel(dir, out)
		return out, err == nil && !strings.HasPrefix(rel, "..")
	}
	if strings.HasSuffix(name, ".zip") {
		zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
		if err != nil {
			return err
		}
		for _, f := range zr.File {
			out, ok := safe(f.Name)
			if !ok {
				continue
			}
			if f.FileInfo().IsDir() {
				_ = os.MkdirAll(out, 0o755)
				continue
			}
			_ = os.MkdirAll(filepath.Dir(out), 0o755)
			rc, err := f.Open()
			if err != nil {
				return err
			}
			w, err := os.OpenFile(out, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
			if err != nil {
				rc.Close()
				return err
			}
			_, err = io.Copy(w, rc)
			rc.Close()
			w.Close()
			if err != nil {
				return err
			}
		}
		return nil
	}
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return err
	}
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		out, ok := safe(h.Name)
		if !ok {
			continue
		}
		switch h.Typeflag {
		case tar.TypeDir:
			_ = os.MkdirAll(out, 0o755)
		case tar.TypeReg:
			_ = os.MkdirAll(filepath.Dir(out), 0o755)
			w, err := os.OpenFile(out, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.FileMode(h.Mode)|0o600)
			if err != nil {
				return err
			}
			_, err = io.Copy(w, tr)
			w.Close()
			if err != nil {
				return err
			}
		case tar.TypeSymlink:
			if _, ok := safe(filepath.Join(filepath.Dir(h.Name), h.Linkname)); ok {
				_ = os.Symlink(h.Linkname, out)
			}
		}
	}
}

// ---- síntesi ----

var (
	mdStripRe = regexp.MustCompile("[*_`#>|]+")
	emojiRe   = regexp.MustCompile(`[\x{1F300}-\x{1FAFF}\x{2600}-\x{27BF}\x{FE0F}]`)
)

// speechText deixa el text a punt per dir-lo en veu alta.
func speechText(t string) string {
	t = mdStripRe.ReplaceAllString(t, "")
	t = emojiRe.ReplaceAllString(t, "")
	t = strings.ReplaceAll(t, "«", "")
	t = strings.ReplaceAll(t, "»", "")
	return strings.Join(strings.Fields(t), " ")
}

// voiceKey és el nom del fitxer d'una frase (motor + veu + text).
func (s *Server) voiceKey(text, who, lang string) string {
	c := s.voiceCfg()
	h := sha256.Sum256([]byte(c.Engine + "|" + c.URL + "|" + c.Model + "|" + who + "|" + lang + "|" + speechText(text)))
	return hex.EncodeToString(h[:12])
}

func (s *Server) voicePath(key string) (string, string) {
	base := filepath.Join(s.voiceDir(), "cache", key)
	for _, ext := range []string{".wav", ".mp3"} {
		if _, err := os.Stat(base + ext); err == nil {
			return base + ext, ext
		}
	}
	return "", ""
}

// voiceStyle: com ha de sonar cada narrador (motor http amb instruccions).
var voiceStyle = map[string]string{
	"caster":  "You are an excited TV sports play-by-play commentator calling a big moment live: high energy, fast, punchy, rising pitch on the climax, a smile in the voice.",
	"analyst": "You are a warm, witty TV sports analyst explaining why something happened: confident, clear, measured pace, a bit playful.",
}

// synth genera l'àudio d'una frase (si encara no hi és) i torna la clau.
func (s *Server) synth(ctx context.Context, text, who, lang string) (string, error) {
	text = speechText(text)
	if text == "" {
		return "", errConst("empty")
	}
	key := s.voiceKey(text, who, lang)
	if p, _ := s.voicePath(key); p != "" {
		return key, nil
	}
	c := s.voiceCfg()
	dir := filepath.Join(s.voiceDir(), "cache")
	_ = os.MkdirAll(dir, 0o700)
	switch c.Engine {
	case "piper":
		v, ok := piperVoices[lang][who]
		if !ok {
			v = piperVoices[lang]["caster"]
		}
		if v.Name == "" || !s.voiceReady(lang) {
			return "", errConst("voices not installed")
		}
		out := filepath.Join(dir, key+".wav")
		cctx, cancel := context.WithTimeout(ctx, 90*time.Second)
		defer cancel()
		cmd := exec.CommandContext(cctx, s.piperExe(), "--model", s.piperModel(v), "--output_file", out+".tmp",
			"--length_scale", fmt.Sprint(v.Length), "--noise_scale", fmt.Sprint(v.Noise), "--noise_w", fmt.Sprint(v.NoiseW))
		cmd.Dir = filepath.Dir(s.piperExe())
		cmd.Env = append(os.Environ(), "LD_LIBRARY_PATH="+cmd.Dir, "DYLD_LIBRARY_PATH="+cmd.Dir)
		cmd.Stdin = strings.NewReader(text + "\n")
		if b, err := cmd.CombinedOutput(); err != nil {
			return "", fmt.Errorf("piper: %v %s", err, oneLine(string(b), 200))
		}
		if err := os.Rename(out+".tmp", out); err != nil {
			return "", err
		}
		return key, nil
	case "http":
		url := strings.TrimRight(strings.TrimSpace(c.URL), "/")
		if !strings.HasSuffix(url, "/audio/speech") {
			url += "/v1/audio/speech"
		}
		voice := c.Voices[who]
		if voice == "" {
			voice = map[string]string{"caster": "alloy", "analyst": "echo"}[who]
		}
		body := map[string]any{"model": firstNonEmptyStr(c.Model, "tts-1"), "input": text, "voice": voice, "response_format": "mp3", "instructions": voiceStyle[who]}
		if who == "caster" {
			body["speed"] = 1.1
		}
		if len(c.Extra) > 0 {
			var extra map[string]map[string]any // {"caster": {...}, "analyst": {...}}
			if json.Unmarshal(c.Extra, &extra) == nil {
				for k, v := range extra[who] {
					body[k] = v
				}
			}
		}
		raw, _ := json.Marshal(body)
		cctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
		defer cancel()
		req, _ := http.NewRequestWithContext(cctx, http.MethodPost, url, bytes.NewReader(raw))
		req.Header.Set("Content-Type", "application/json")
		if c.Key != "" {
			req.Header.Set("Authorization", "Bearer "+c.Key)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()
		audio, err := io.ReadAll(io.LimitReader(resp.Body, 20<<20))
		if err != nil {
			return "", err
		}
		if resp.StatusCode != 200 {
			return "", fmt.Errorf("voice server: HTTP %d %s", resp.StatusCode, oneLine(string(audio), 200))
		}
		ext := ".mp3"
		if bytes.HasPrefix(audio, []byte("RIFF")) {
			ext = ".wav"
		}
		if err := os.WriteFile(filepath.Join(dir, key+ext), audio, 0o600); err != nil {
			return "", err
		}
		return key, nil
	}
	return "", errConst("voice off")
}

func firstNonEmptyStr(v ...string) string {
	for _, x := range v {
		if strings.TrimSpace(x) != "" {
			return x
		}
	}
	return ""
}

// La cua de veu: una frase rere l'altra (Piper fa servir tota la CPU).
type voiceJob struct {
	s                    *Server
	text, who, lang, key string
}

var (
	voiceOnce sync.Once
	voiceJobs chan voiceJob
)

// voiceLater encua una frase i en torna la clau (l'àudio hi serà en uns
// segons). Sense veus instal·lades, buit: la pantalla en posa els subtítols.
func (s *Server) voiceLater(text, who, lang string) string {
	if strings.TrimSpace(text) == "" || !s.voiceReady(lang) {
		return ""
	}
	voiceOnce.Do(func() {
		voiceJobs = make(chan voiceJob, 512)
		go func() {
			for j := range voiceJobs {
				ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
				if _, err := j.s.synth(ctx, j.text, j.who, j.lang); err != nil {
					log.Printf("arena: veu: %v", err)
				}
				cancel()
				// Si ha fallat, una altra petició la podrà tornar a encuar.
				voiceQueuedMu.Lock()
				delete(voiceQueued, j.key)
				voiceQueuedMu.Unlock()
			}
		}()
	})
	key := s.voiceKey(text, who, lang)
	if p, _ := s.voicePath(key); p != "" {
		return key
	}
	voiceQueuedMu.Lock()
	if voiceQueued[key] {
		voiceQueuedMu.Unlock()
		return key
	}
	voiceQueued[key] = true
	voiceQueuedMu.Unlock()
	select {
	case voiceJobs <- voiceJob{s, text, who, lang, key}:
	default:
		voiceQueuedMu.Lock()
		delete(voiceQueued, key)
		voiceQueuedMu.Unlock()
		return "" // cua plena: millor subtítols que endarrerir-ho tot
	}
	return key
}

var (
	voiceQueuedMu sync.Mutex
	voiceQueued   = map[string]bool{}
)

// handleArenaVoice: GET ?k= (l'àudio d'una frase), GET (estat); POST (admin)
// {action: install|config|test}.
func (s *Server) handleArenaVoice(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		if k := r.URL.Query().Get("k"); k != "" {
			if !regexp.MustCompile(`^[0-9a-f]{24}$`).MatchString(k) {
				http.NotFound(w, r)
				return
			}
			p, ext := s.voicePath(k)
			if p == "" {
				http.Error(w, "not ready", http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Type", map[string]string{".wav": "audio/wav", ".mp3": "audio/mpeg"}[ext])
			w.Header().Set("Cache-Control", "public, max-age=86400")
			http.ServeFile(w, r, p)
			return
		}
		c := s.voiceCfg()
		c.Key = "" // la clau no surt mai
		inst := voiceInst.snapshot()
		writeJSON(w, map[string]any{"config": c, "install": inst, "ready": map[string]bool{"ca": s.voiceReady("ca"), "en": s.voiceReady("en")},
			"platform": piperArchive() != "", "sizes": map[string]string{"ca": "~113 MB", "en": "~206 MB"}})
		return
	}
	if _, admin := s.arenaUser(r); !admin {
		http.Error(w, "admin only", http.StatusForbidden)
		return
	}
	var req struct {
		Action string        `json:"action"`
		Lang   string        `json:"lang"`
		Config arenaVoiceCfg `json:"config"`
		Text   string        `json:"text"`
		Who    string        `json:"who"`
	}
	if !decodeSmall(w, r, &req) {
		return
	}
	switch req.Action {
	case "install":
		lang := req.Lang
		if _, ok := piperVoices[lang]; !ok {
			lang = "ca"
		}
		if voiceInst.snapshot().State == "installing" {
			http.Error(w, "already installing", http.StatusConflict)
			return
		}
		voiceInst.set(func(v *voiceInstall) { v.State, v.Error, v.Step = "installing", "", "" })
		goBackground(func() {
			ctx, cancel := context.WithTimeout(context.Background(), 40*time.Minute)
			defer cancel()
			if err := s.installVoices(ctx, lang); err != nil {
				voiceInst.set(func(v *voiceInstall) { v.State, v.Error = "error", err.Error() })
				return
			}
			voiceInst.set(func(v *voiceInstall) { v.State, v.Step = "ready", "" })
		})
		writeJSON(w, map[string]bool{"ok": true})
	case "config":
		c := req.Config
		switch c.Engine {
		case "piper", "http", "off":
		default:
			http.Error(w, "engine: piper, http or off", http.StatusBadRequest)
			return
		}
		if c.Engine == "http" && c.Key == "" {
			c.Key = s.voiceCfg().Key // no esborrar la clau si no la tornen a enviar
		}
		s.saveVoiceCfg(c)
		writeJSON(w, map[string]bool{"ok": true})
	case "test":
		lang := req.Lang
		if lang == "" {
			lang = s.fbLangCode()
		}
		who := req.Who
		if who != "analyst" {
			who = "caster"
		}
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
		defer cancel()
		k, err := s.synth(ctx, firstNonEmptyStr(req.Text, gameTr("I aquí arriba el moment! Quina jugada, senyores i senyors!", "And here it comes! What a play, ladies and gentlemen!")), who, lang)
		if err != nil {
			writeJSONStatus(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, map[string]string{"key": k})
	default:
		http.Error(w, "unknown action", http.StatusBadRequest)
	}
}
