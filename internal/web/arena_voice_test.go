package web

import (
	"archive/zip"
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// Un Piper fals: el mateix binari dels tests, cridat amb GREGAL_FAKE_PIPER=1.
// Amb --json-input llegeix una frase per línia i escriu un «wav» amb el seu
// pid (per saber si el procés s'ha reaprofitat); sense, una sola frase.
func init() {
	if os.Getenv("GREGAL_FAKE_PIPER") != "1" {
		return
	}
	out, jsonIn := "", false
	for i, a := range os.Args {
		if a == "--json-input" {
			jsonIn = true
		}
		if a == "--output_file" && i+1 < len(os.Args) {
			out = os.Args[i+1]
		}
	}
	write := func(path string) { os.WriteFile(path, []byte(fmt.Sprintf("RIFF%d", os.Getpid())), 0o600) }
	if !jsonIn {
		io.ReadAll(os.Stdin)
		write(out)
		os.Exit(0)
	}
	sc := bufio.NewScanner(os.Stdin)
	for sc.Scan() {
		var line struct {
			Text       string `json:"text"`
			OutputFile string `json:"output_file"`
		}
		if json.Unmarshal(sc.Bytes(), &line) != nil || line.OutputFile == "" {
			os.Exit(2)
		}
		write(line.OutputFile)
		fmt.Println(line.OutputFile)
	}
	os.Exit(0)
}

func TestArenaVoiceHTTPEngineCachesAndServes(t *testing.T) {
	s := cornerTestServer(t)
	var calls int32
	var got map[string]any
	tts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		if r.URL.Path != "/v1/audio/speech" || r.Header.Get("Authorization") != "Bearer k" {
			http.Error(w, "bad", 400)
			return
		}
		raw, _ := io.ReadAll(r.Body)
		json.Unmarshal(raw, &got)
		w.Write([]byte("ID3fake-mp3"))
	}))
	defer tts.Close()
	s.saveVoiceCfg(arenaVoiceCfg{Engine: "http", URL: tts.URL, Key: "k", Model: "chatterbox", Voices: map[string]string{"caster": "Bob"},
		Extra: json.RawMessage(`{"caster":{"exaggeration":0.9}}`)})
	if !s.voiceReady("ca") {
		t.Fatal("el motor http configurat ha d'estar a punt")
	}
	key := s.voiceLater("**Quina jugada** 🏈!", "caster", "ca")
	if key == "" {
		t.Fatal("clau buida")
	}
	for i := 0; i < 100; i++ {
		if p, _ := s.voicePath(key); p != "" {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if p, ext := s.voicePath(key); p == "" || ext != ".mp3" {
		t.Fatalf("l'àudio no s'ha generat: %q %q", p, ext)
	}
	if got["input"] != "Quina jugada !" && got["input"] != "Quina jugada!" {
		t.Errorf("el text ha d'anar net de markdown i emojis: %q", got["input"])
	}
	if got["voice"] != "Bob" || got["exaggeration"] != 0.9 || !strings.Contains(got["instructions"].(string), "sports") {
		t.Errorf("petició: %+v", got)
	}
	// La mateixa frase no es torna a generar.
	if k2 := s.voiceLater("**Quina jugada** 🏈!", "caster", "ca"); k2 != key || atomic.LoadInt32(&calls) != 1 {
		t.Fatalf("la frase havia de sortir de la memòria cau: %d crides", calls)
	}
	// Es serveix, també a un espectador amb l'enllaç.
	s.Token = "secret"
	spect := s.arenaSpectatorKey(false)
	req := httptest.NewRequest("GET", "/api/arena/voice?k="+key, nil)
	req.Header.Set("X-Arena-Spectator", spect)
	rec := httptest.NewRecorder()
	s.Hub().Handler().ServeHTTP(rec, req)
	if rec.Code != 200 || rec.Header().Get("Content-Type") != "audio/mpeg" || !bytes.HasPrefix(rec.Body.Bytes(), []byte("ID3")) {
		t.Fatalf("servir l'àudio: %d %s", rec.Code, rec.Header().Get("Content-Type"))
	}
	// L'estat no ensenya mai la clau del servidor de veu.
	st := httptest.NewRecorder()
	s.handleArenaVoice(st, httptest.NewRequest("GET", "/api/arena/voice", nil))
	if strings.Contains(st.Body.String(), `"key":"k"`) {
		t.Fatalf("la clau no pot sortir: %s", st.Body.String())
	}
	// Una clau de fitxer estranya no surt del directori.
	bad := httptest.NewRecorder()
	s.handleArenaVoice(bad, httptest.NewRequest("GET", "/api/arena/voice?k=../../settings", nil))
	if bad.Code != 404 {
		t.Fatalf("clau dolenta: %d", bad.Code)
	}
}

func TestExtractArchiveStaysInside(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, name := range []string{"piper/piper.exe", "../escape.txt", "piper/espeak-ng-data/x"} {
		w, _ := zw.Create(name)
		w.Write([]byte("x"))
	}
	zw.Close()
	dir := t.TempDir()
	if err := extractArchive(buf.Bytes(), "piper_windows_amd64.zip", dir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "piper", "piper.exe")); err != nil {
		t.Fatal("no s'ha extret")
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(dir), "escape.txt")); err == nil {
		t.Fatal("un fitxer ha sortit del directori")
	}
	_ = context.Background
}

// La veu en anglès ha de ser lliure per a un ús comercial (una competició
// d'empresa amb patrocinadors): res de no comercial (Ryan) ni de recerca (Alan).
func TestArenaVoiceEnglishVoicesAreLicenceClean(t *testing.T) {
	ok := map[string]bool{"Public domain": true, "CC0": true, "CC BY 4.0": true, "MIT": true, "Apache-2.0": true}
	for who, v := range piperVoices["en"] {
		if !ok[v.License] {
			t.Errorf("%s: %s té la llicència %q", who, v.Name, v.License)
		}
		for _, r := range piperRetired {
			if v.Name == r {
				t.Errorf("%s fa servir la veu retirada %s", who, r)
			}
		}
		// en_US-norman-medium ↔ en/en_US/norman/medium
		if parts := strings.SplitN(v.Name, "-", 3); len(parts) != 3 || v.Path != "en/"+strings.Join(parts, "/") {
			t.Errorf("%s: el camí %q no és el de %s", who, v.Path, v.Name)
		}
		if v.Label == "" || v.MB <= 0 {
			t.Errorf("%s: falta el nom o la mida", who)
		}
	}
	c, a := piperVoices["en"]["caster"], piperVoices["en"]["analyst"]
	if c.Name == a.Name {
		t.Fatal("el narrador i l'analista han de sonar diferent")
	}
	// El narrador, viu (més ràpid i amb més variació); l'analista, pausat.
	if !(c.Length < a.Length && c.Length >= .7 && c.Noise >= a.Noise) {
		t.Errorf("afinació: narrador %+v, analista %+v", c, a)
	}
	for _, vs := range piperVoices {
		for _, v := range vs {
			if v.License == "" || v.Label == "" {
				t.Errorf("%s: sense nom o llicència per a la pantalla", v.Name)
			}
		}
	}
	if sz := voiceSizes(); sz["en"] != fmt.Sprintf("~%d MB", piperEngineMB+c.MB+a.MB) || sz["ca"] == "" {
		t.Errorf("mides: %v", sz)
	}
}

// fakePiper posa el binari dels tests com a piper.exe del servidor.
func fakePiper(t *testing.T, s *Server) {
	t.Helper()
	self, err := os.Executable()
	if err != nil {
		t.Skip(err)
	}
	raw, err := os.ReadFile(self)
	if err != nil {
		t.Skip(err)
	}
	os.MkdirAll(filepath.Dir(s.piperExe()), 0o700)
	if err := os.WriteFile(s.piperExe(), raw, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GREGAL_FAKE_PIPER", "1")
}

func touchVoice(t *testing.T, s *Server, name string) {
	t.Helper()
	p := filepath.Join(s.voiceDir(), "voices", name+".onnx")
	os.MkdirAll(filepath.Dir(p), 0o700)
	os.WriteFile(p, []byte("x"), 0o600)
	os.WriteFile(p+".json", []byte("{}"), 0o600)
}

// Qui ja tenia les veus d'abans (Ryan i Alan) no té les noves: es baixen
// soles. Sense Piper (o amb la veu apagada) no es baixa res tot sol.
func TestArenaVoiceAutoInstallSwitchesOldVoices(t *testing.T) {
	s := cornerTestServer(t)
	if piperArchive() == "" {
		t.Skip("Piper no hi és per a aquesta plataforma")
	}
	if s.voiceAutoWanted("en") {
		t.Fatal("sense Piper instal·lat no s'ha de baixar res sol")
	}
	os.MkdirAll(filepath.Dir(s.piperExe()), 0o700)
	os.WriteFile(s.piperExe(), []byte("x"), 0o755)
	for _, n := range piperRetired {
		touchVoice(t, s, n)
	}
	if s.voiceReady("en") {
		t.Fatal("amb les veus retirades, l'anglès no està a punt")
	}
	if !s.voiceAutoWanted("en") {
		t.Fatal("amb Piper i les veus velles, les noves s'han de baixar soles")
	}
	if s.voiceAutoWanted("xx") {
		t.Fatal("una llengua que no existeix no es baixa")
	}
	s.saveVoiceCfg(arenaVoiceCfg{Engine: "off"})
	if s.voiceAutoWanted("en") {
		t.Fatal("amb la veu apagada no es baixa res")
	}
	s.saveVoiceCfg(arenaVoiceCfg{Engine: "piper"})
	for _, v := range piperVoices["en"] {
		touchVoice(t, s, v.Name)
	}
	if !s.voiceReady("en") || s.voiceAutoWanted("en") {
		t.Fatal("amb les veus noves, a punt i res a baixar")
	}
	// Qui tenia Ona medium (retirada) rep Ona x_low sola: el català ja no és
	// a punt fins que hi és la nova.
	if piperVoices["ca"]["caster"].Name != "ca_ES-upc_ona-x_low" {
		t.Fatalf("la narradora en català ha de ser Ona x_low: %s", piperVoices["ca"]["caster"].Name)
	}
	if s.voiceReady("ca") || !s.voiceAutoWanted("ca") {
		t.Fatal("amb Ona medium (retirada), Ona x_low s'ha de baixar sola")
	}
	// En acabar, les retirades se'n van; les noves de cada llengua es queden.
	for _, v := range piperVoices["ca"] {
		touchVoice(t, s, v.Name)
	}
	s.pruneRetiredVoices()
	for _, n := range piperRetired {
		if _, err := os.Stat(filepath.Join(s.voiceDir(), "voices", n+".onnx")); err == nil {
			t.Errorf("%s s'havia d'esborrar", n)
		}
	}
	for _, n := range []string{piperVoices["en"]["caster"].Name, piperVoices["ca"]["caster"].Name} {
		if _, err := os.Stat(filepath.Join(s.voiceDir(), "voices", n+".onnx")); err != nil {
			t.Errorf("%s no s'havia d'esborrar", n)
		}
	}
	// L'estat diu el nom i la llicència de cada veu.
	rec := httptest.NewRecorder()
	s.handleArenaVoice(rec, httptest.NewRequest("GET", "/api/arena/voice", nil))
	var st struct {
		Voices map[string][]struct {
			Who, Name, Label, License, Card string
			Installed                       bool
		} `json:"voices"`
		Sizes map[string]string `json:"sizes"`
	}
	json.Unmarshal(rec.Body.Bytes(), &st)
	if len(st.Voices["en"]) != 2 || st.Voices["en"][0].Who != "caster" || st.Voices["en"][0].License != "Public domain" ||
		!strings.Contains(st.Voices["en"][0].Card, "huggingface.co/rhasspy/piper-voices/blob/main/en/") || !st.Voices["en"][0].Installed {
		t.Fatalf("veus a l'estat: %s", rec.Body.String())
	}
	if !strings.HasPrefix(st.Sizes["en"], "~") {
		t.Fatalf("mides: %v", st.Sizes)
	}
}

// Cada veu té un procés de Piper obert: la segona frase no n'engega un altre.
// Si el procés s'atura, la frase següent n'engega un de nou.
func TestArenaVoicePiperProcessIsReused(t *testing.T) {
	s := cornerTestServer(t)
	if piperArchive() == "" {
		t.Skip("Piper no hi és per a aquesta plataforma")
	}
	fakePiper(t, s)
	defer stopPiperProcs()
	for _, v := range piperVoices["en"] {
		touchVoice(t, s, v.Name)
	}
	if !s.voiceReady("en") {
		t.Fatal("no està a punt")
	}
	ctx := context.Background()
	pid := func(key string) string {
		p, ext := s.voicePath(key)
		if ext != ".wav" {
			t.Fatalf("sense wav: %q", p)
		}
		b, _ := os.ReadFile(p)
		return strings.TrimPrefix(string(b), "RIFF")
	}
	say := func(text, who string) string {
		k, err := s.synth(ctx, text, who, "en")
		if err != nil {
			t.Fatal(err)
		}
		return k
	}
	k1, k2 := say("And here it comes!", "caster"), say("What a play, ladies and gentlemen!", "caster")
	if k1 == k2 || pid(k1) == "" || pid(k1) != pid(k2) {
		t.Fatalf("el mateix procés havia de dir les dues frases: %q %q", pid(k1), pid(k2))
	}
	if k3 := say("Calm analysis.", "analyst"); pid(k3) == pid(k1) {
		t.Fatal("l'analista té el seu propi procés (el seu model)")
	}
	stopPiperProcs()
	if k4 := say("Back again!", "caster"); pid(k4) == "" || pid(k4) == pid(k1) {
		t.Fatalf("després d'aturar-lo, un procés nou: %q", pid(k4))
	}
	// La veu forma part de la clau: una altra veu no torna l'àudio d'aquesta.
	old := piperVoices["en"]["caster"]
	nv := old
	nv.Name = "en_US-other-medium"
	piperVoices["en"]["caster"] = nv
	defer func() { piperVoices["en"]["caster"] = old }()
	if s.voiceKey("And here it comes!", "caster", "en") == k1 {
		t.Fatal("canviar de veu ha de canviar la clau")
	}
}
