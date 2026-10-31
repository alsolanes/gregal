package web

// La veu dels narradors de l'Arena, gratuïta i al propi ordinador.
//
//   - piper (per defecte): Piper, un motor de veu neuronal lliure que corre a
//     la CPU sense internet ni claus. L'organitzador prem «Instal·la les veus»
//     i Gregal baixa el motor i les veus de la llengua (català: Ona i Pau;
//     anglès: Norman i Cori, de domini públic). El narrador parla més ràpid i
//     amb més variació que l'analista. Cada veu té un procés de Piper que
//     queda obert (--json-input): el model es carrega un cop, no a cada frase.
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
	"bufio"
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
	Pause  float64 // silenci després de cada frase, en segons (0 = el de Piper, 0,2)
	// Per a la pantalla de preferències: qui és i amb quina llicència es pot fer servir.
	Label   string // «Norman (US)»
	License string // la del conjunt de dades, segons el MODEL_CARD
	MB      int    // mida del model (.onnx)
}

// Les veus de cada llengua: el narrador (caster), més viu; l'analista, més pausat.
//
// Llicències (MODEL_CARD de cada veu a huggingface.co/rhasspy/piper-voices):
//   - Anglès: Norman i Cori, de Bryce Beattie, entrenades de zero amb
//     enregistraments de LibriVox: domini públic. Es poden fer servir en una
//     competició d'empresa amb patrocinadors. Abans hi havia Ryan (CC BY-NC-SA
//     4.0: no comercial) i Alan (fine-tune de lessac, llicència de recerca, i
//     dades «All Rights Reserved»): retirades.
//   - Català: Ona i Pau, corpus FestCat de la UPC: CC BY-SA 3.0 ES (cal
//     atribució i compartir igual). Ona «medium» és, a més, un fine-tune de la
//     veu lessac (llicència de recerca). L'organitzador ho ha de decidir: la
//     pantalla de veus ho diu.
var piperVoices = map[string]map[string]piperVoice{
	"ca": {
		"caster":  {Name: "ca_ES-upc_ona-x_low", Path: "ca/ca_ES/upc_ona/x_low", Length: .84, Noise: .9, NoiseW: .95, Label: "Ona", License: "CC BY-SA 3.0 ES", MB: 21},
		"analyst": {Name: "ca_ES-upc_pau-x_low", Path: "ca/ca_ES/upc_pau/x_low", Length: 1, Noise: .667, NoiseW: .8, Label: "Pau", License: "CC BY-SA 3.0 ES", MB: 28},
	},
	"en": {
		// Un narrador d'audiollibre fet comentarista: més de pressa i amb més
		// variació d'entonació i de ritme que la veu de sèrie.
		"caster":  {Name: "en_US-norman-medium", Path: "en/en_US/norman/medium", Length: .82, Noise: .85, NoiseW: .95, Pause: .08, Label: "Norman (US)", License: "Public domain", MB: 63},
		"analyst": {Name: "en_GB-cori-medium", Path: "en/en_GB/cori/medium", Length: .98, Noise: .667, NoiseW: .8, Label: "Cori (UK)", License: "Public domain", MB: 63},
	},
}

// piperRetired: veus que ja no es fan servir (llicència no comercial o de
// recerca). Quan les noves ja hi són, s'esborren del directori de veus.
// Ona medium es va afinar a partir de lessac (llicència només de recerca):
// ara és Ona x_low, entrenada de zero.
var piperRetired = []string{"en_US-ryan-high", "en_GB-alan-medium", "ca_ES-upc_ona-medium"}

// piperEngineMB és el que pesa el motor Piper (el zip de Windows; els altres, semblant).
const piperEngineMB = 22

// voiceSizes: el que baixa «Instal·la» per a cada llengua (motor inclòs).
func voiceSizes() map[string]string {
	out := map[string]string{}
	for l, vs := range piperVoices {
		mb := piperEngineMB
		for _, v := range vs {
			mb += v.MB
		}
		out[l] = fmt.Sprintf("~%d MB", mb)
	}
	return out
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
	s.pruneRetiredVoices()
	return nil
}

// pruneRetiredVoices esborra els models retirats (Ryan, Alan) quan ja no
// fan falta: les veus noves d'aquella llengua ja hi són.
func (s *Server) pruneRetiredVoices() {
	for _, name := range piperRetired {
		inUse := false
		for _, vs := range piperVoices {
			for _, v := range vs {
				inUse = inUse || v.Name == name
			}
		}
		if inUse || !s.voiceReady(name[:2]) { // «en_US-…» → en
			continue
		}
		for _, ext := range []string{".onnx", ".onnx.json"} {
			p := filepath.Join(s.voiceDir(), "voices", name+ext)
			if err := os.Remove(p); err == nil {
				log.Printf("arena: veu retirada esborrada: %s", filepath.Base(p))
			}
		}
	}
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
	voice := c.Model
	if c.Engine == "piper" {
		// La veu (i com s'afina) forma part de la clau: en canviar-la
		// (Ryan → Norman) no es torna l'àudio vell de la memòria cau.
		v := piperVoiceFor(lang, who)
		voice = v.Name + strings.Join(piperArgs(v), " ")
	}
	h := sha256.Sum256([]byte(c.Engine + "|" + c.URL + "|" + voice + "|" + who + "|" + lang + "|" + speechText(text)))
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
		v := piperVoiceFor(lang, who)
		if v.Name == "" || !s.voiceReady(lang) {
			return "", errConst("voices not installed")
		}
		out := filepath.Join(dir, key+".wav")
		cctx, cancel := context.WithTimeout(ctx, 90*time.Second)
		defer cancel()
		// Primer, el procés que ja té el model carregat; si falla, un de sol.
		if err := s.piperSay(cctx, v, text, out+".tmp"); err != nil {
			log.Printf("arena: veu: el procés de Piper (%s) ha fallat, se'n fa un de sol: %v", v.Name, err)
			cmd := exec.CommandContext(cctx, s.piperExe(), append([]string{"--model", s.piperModel(v), "--output_file", out + ".tmp"}, piperArgs(v)...)...)
			cmd.Dir = filepath.Dir(s.piperExe())
			cmd.Env = append(os.Environ(), "LD_LIBRARY_PATH="+cmd.Dir, "DYLD_LIBRARY_PATH="+cmd.Dir)
			cmd.Stdin = strings.NewReader(text + "\n")
			if b, err := cmd.CombinedOutput(); err != nil {
				return "", fmt.Errorf("piper: %v %s", err, oneLine(string(b), 200))
			}
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

// piperVoiceFor: la veu d'un narrador (si no n'hi ha, la del narrador principal).
func piperVoiceFor(lang, who string) piperVoice {
	v, ok := piperVoices[lang][who]
	if !ok {
		v = piperVoices[lang]["caster"]
	}
	return v
}

func piperArgs(v piperVoice) []string {
	args := []string{"--length_scale", fmt.Sprint(v.Length), "--noise_scale", fmt.Sprint(v.Noise), "--noise_w", fmt.Sprint(v.NoiseW)}
	if v.Pause > 0 {
		args = append(args, "--sentence_silence", fmt.Sprint(v.Pause))
	}
	return args
}

// ---- un procés de Piper per veu, obert ----
//
// Arrencar Piper a cada frase vol dir carregar el model cada cop (~0,5-1 s
// de cada frase). Amb --json-input, Piper llegeix una línia JSON per frase
// ({"text", "output_file"}) i escriu a la sortida el camí del fitxer quan
// l'ha acabat: el mateix procés serveix totes les frases d'una veu. Si
// queda uns minuts sense feina, es tanca (cada model ocupa memòria).

type piperProc struct {
	mu    sync.Mutex
	cmd   *exec.Cmd
	in    io.WriteCloser
	out   *bufio.Reader
	timer *time.Timer
	last  time.Time
}

var (
	piperProcsMu sync.Mutex
	piperProcs   = map[string]*piperProc{} // per model (camí del .onnx)
)

const piperIdle = 10 * time.Minute

// piperSay diu una frase amb el procés obert de la veu (l'engega si cal).
func (s *Server) piperSay(ctx context.Context, v piperVoice, text, out string) error {
	model := s.piperModel(v)
	piperProcsMu.Lock()
	p := piperProcs[model]
	if p == nil {
		p = &piperProc{}
		piperProcs[model] = p
	}
	piperProcsMu.Unlock()
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.cmd == nil {
		if err := p.start(s.piperExe(), model, v); err != nil {
			return err
		}
	}
	line, _ := json.Marshal(map[string]string{"text": text, "output_file": out})
	type res struct {
		path string
		err  error
	}
	done := make(chan res, 1)
	go func() {
		if _, err := p.in.Write(append(line, '\n')); err != nil {
			done <- res{"", err}
			return
		}
		got, err := p.out.ReadString('\n')
		done <- res{strings.TrimSpace(got), err}
	}()
	select {
	case r := <-done:
		if r.err != nil {
			p.stop()
			return r.err
		}
		if _, err := os.Stat(out); err != nil {
			p.stop()
			return fmt.Errorf("piper did not write %s (%q)", filepath.Base(out), oneLine(r.path, 120))
		}
		p.last = time.Now()
		p.timer.Reset(piperIdle)
		return nil
	case <-ctx.Done():
		// Un procés penjat no pot quedar a mitja frase: fora, i el pròxim és nou.
		p.stop()
		return ctx.Err()
	}
}

func (p *piperProc) start(exe, model string, v piperVoice) error {
	cmd := exec.Command(exe, append([]string{"--model", model, "--json-input", "--quiet"}, piperArgs(v)...)...)
	cmd.Dir = filepath.Dir(exe)
	cmd.Env = append(os.Environ(), "LD_LIBRARY_PATH="+cmd.Dir, "DYLD_LIBRARY_PATH="+cmd.Dir)
	in, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	p.cmd, p.in, p.out, p.last = cmd, in, bufio.NewReader(stdout), time.Now()
	if p.timer == nil {
		p.timer = time.AfterFunc(piperIdle, func() {
			p.mu.Lock()
			defer p.mu.Unlock()
			if time.Since(p.last) >= piperIdle-time.Second {
				p.stop()
			}
		})
	} else {
		p.timer.Reset(piperIdle)
	}
	return nil
}

// stop tanca el procés (amb p.mu agafat). Sense entrada, Piper acaba sol;
// si no, es mata.
func (p *piperProc) stop() {
	if p.cmd == nil {
		return
	}
	cmd := p.cmd
	p.in.Close()
	p.cmd, p.in, p.out = nil, nil, nil
	go func() {
		t := time.AfterFunc(3*time.Second, func() { _ = cmd.Process.Kill() })
		_ = cmd.Wait()
		t.Stop()
	}()
}

// stopPiperProcs tanca tots els processos de Piper (en reinstal·lar les veus,
// que els fitxers dels models no quedin oberts, i als tests).
func stopPiperProcs() {
	piperProcsMu.Lock()
	defer piperProcsMu.Unlock()
	for k, p := range piperProcs {
		p.mu.Lock()
		p.stop()
		p.mu.Unlock()
		delete(piperProcs, k)
	}
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
				bgWork.Done()
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
	// Cada frase a la cua compta com a feina de fons: qui espera que el fons
	// s'assequi (un test, una aturada) també espera les veus pendents.
	bgWork.Add(1)
	select {
	case voiceJobs <- voiceJob{s, text, who, lang, key}:
	default:
		bgWork.Done()
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

// startVoiceInstall baixa Piper i les veus d'una llengua en segon pla.
func (s *Server) startVoiceInstall(lang string) bool {
	voiceInstMu.Lock()
	if voiceInst.snapshot().State == "installing" {
		voiceInstMu.Unlock()
		return false
	}
	voiceInst.set(func(v *voiceInstall) { v.State, v.Error, v.Step = "installing", "", "" })
	voiceInstMu.Unlock()
	goBackground(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 40*time.Minute)
		defer cancel()
		if err := s.installVoices(ctx, lang); err != nil {
			voiceInst.set(func(v *voiceInstall) { v.State, v.Error = "error", err.Error() })
			return
		}
		voiceInst.set(func(v *voiceInstall) { v.State, v.Step = "ready", "" })
	})
	return true
}

var (
	voiceInstMu   sync.Mutex
	voiceAutoDone = map[string]bool{}
)

// voiceAutoWanted: cal baixar soles les veus d'una llengua? Sí quan Piper
// ja s'havia instal·lat (l'organitzador va triar les veus lliures) però a la
// llengua li falta alguna veu: la d'una altra llengua, o una que ha canviat
// (Ryan → Norman, per la llicència). Sense Piper, no: ho decideix l'organitzador.
func (s *Server) voiceAutoWanted(lang string) bool {
	if _, ok := piperVoices[lang]; !ok {
		return false
	}
	if s.voiceCfg().Engine != "piper" || piperArchive() == "" || s.voiceReady(lang) {
		return false
	}
	_, err := os.Stat(s.piperExe())
	return err == nil
}

// voiceAutoInstall: qui ja té Piper no ha de buscar cap botó quan els jocs
// necessiten unes veus que no té (van en anglès, o les d'abans s'han
// retirat): es baixen soles, de la mateixa font, un cop per arrencada.
func (s *Server) voiceAutoInstall(lang string) {
	if !s.voiceAutoWanted(lang) {
		return
	}
	key := s.arenaDir() + "|" + lang
	voiceInstMu.Lock()
	if voiceAutoDone[key] {
		voiceInstMu.Unlock()
		return
	}
	voiceInstMu.Unlock()
	if !s.startVoiceInstall(lang) {
		return // ja se n'instal·lava una altra: es torna a provar a la pròxima petició
	}
	voiceInstMu.Lock()
	voiceAutoDone[key] = true
	voiceInstMu.Unlock()
	log.Printf("arena: baixant les veus de %s", lang)
}

// voiceAutoUpgrade: qui tenia una veu retirada d'una llengua (p. ex. Ona
// medium) rep la nova d'aquella llengua sola, sense buscar cap botó.
func (s *Server) voiceAutoUpgrade() {
	for _, name := range piperRetired {
		if _, err := os.Stat(filepath.Join(s.voiceDir(), "voices", name+".onnx")); err == nil {
			s.voiceAutoInstall(name[:2])
		}
	}
}

// voicesView: les veus de cada llengua per a la pantalla de preferències.
func (s *Server) voicesView() map[string][]map[string]any {
	out := map[string][]map[string]any{}
	for l, vs := range piperVoices {
		for _, who := range []string{"caster", "analyst"} {
			v, ok := vs[who]
			if !ok {
				continue
			}
			_, err := os.Stat(s.piperModel(v))
			out[l] = append(out[l], map[string]any{"who": who, "name": v.Name, "label": v.Label, "license": v.License, "mb": v.MB,
				"card": strings.TrimSuffix(piperVoicesURL, "resolve/main/") + "blob/main/" + v.Path + "/MODEL_CARD", "installed": err == nil})
		}
	}
	return out
}

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
		s.voiceAutoInstall("en")
		s.voiceAutoUpgrade()
		c := s.voiceCfg()
		c.Key = "" // la clau no surt mai
		inst := voiceInst.snapshot()
		writeJSON(w, map[string]any{"config": c, "install": inst, "ready": map[string]bool{"ca": s.voiceReady("ca"), "en": s.voiceReady("en")},
			"platform": piperArchive() != "", "sizes": voiceSizes(), "voices": s.voicesView()})
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
		if !s.startVoiceInstall(lang) {
			http.Error(w, "already installing", http.StatusConflict)
			return
		}
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
