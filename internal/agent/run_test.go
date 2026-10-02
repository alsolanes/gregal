package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gregal/internal/config"
	"gregal/internal/llm"
	"gregal/internal/tools"
)

// fakeAgentServer torna tool_calls de read la 1a vegada i resposta final després.
func fakeAgentServer(t *testing.T, file string) *httptest.Server {
	t.Helper()
	n := 0
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.NotFound(w, r) // sonda GET /models de DetectWindows: no és un pas
			return
		}
		w.Header().Set("Content-Type", "application/json")
		n++
		if n == 1 {
			// arguments és JSON dins d'un string JSON: dues capes d'escapat.
			// Fer-ho a mà petava amb les rutes de Windows (\U de \Users).
			inner, _ := json.Marshal(map[string]string{"path": file})
			argsField, _ := json.Marshal(string(inner))
			fmt.Fprintf(w, `{"choices":[{"message":{"role":"assistant","content":"","tool_calls":[{"id":"c1","type":"function","function":{"name":"read","arguments":%s}}]},"finish_reason":"tool_calls"}]}`, argsField)
			return
		}
		fmt.Fprint(w, `{"choices":[{"message":{"role":"assistant","content":"fet i llegit"},"finish_reason":"stop"}]}`)
	}))
}

func testCfg(url string) *config.Config {
	return &config.Config{
		Providers: map[string]config.Provider{"t": {BaseURL: url}},
		Roles: map[string]config.Role{
			"code": {Provider: "t", Model: "m", Temperature: 0.4, MaxTokens: 100},
		},
		Verify: config.VerifyCfg{Mode: "off"},
		Agent:  config.AgentCfg{MaxSteps: 10},
		Mode:   "code",
	}
}

func TestRunNonInteractive(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "d.txt")
	if err := os.WriteFile(f, []byte("DADES"), 0o600); err != nil {
		t.Fatal(err)
	}
	srv := fakeAgentServer(t, f)
	defer srv.Close()
	res, err := RunNonInteractive(context.Background(), llm.New(), testCfg(srv.URL), "llegeix", "code", 5, false)
	if err != nil {
		t.Fatal(err)
	}
	if res.Answer != "fet i llegit" || res.Steps != 2 {
		t.Fatalf("res=%+v", res)
	}
	if len(res.Tools) != 1 || res.Tools[0] != "read" {
		t.Fatalf("tools=%v", res.Tools)
	}
	// Sense preus al config: tokens sí, cost no (mai inventat).
	if res.UpTokens < 1 || res.DownTokens < 1 {
		t.Fatalf("tokens buits: %+v", res)
	}
	if res.CostUSD != nil {
		t.Fatalf("sense preu no hi ha cost: %+v", res)
	}
}

func TestRunNonInteractiveCost(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "d.txt")
	if err := os.WriteFile(f, []byte("DADES"), 0o600); err != nil {
		t.Fatal(err)
	}
	srv := fakeAgentServer(t, f)
	defer srv.Close()
	cfg := testCfg(srv.URL)
	cfg.Cost = map[string]config.CostPrice{"m": {In: 1, Out: 4}}
	SetPrices(nil)
	SetupPrices(cfg)
	defer SetPrices(nil)
	res, err := RunNonInteractive(context.Background(), llm.New(), cfg, "llegeix", "code", 5, false)
	if err != nil {
		t.Fatal(err)
	}
	if res.CostUSD == nil {
		t.Fatal("amb preu hi ha d'haver cost")
	}
	want := float64(res.UpTokens)/1e6*1 + float64(res.DownTokens)/1e6*4
	if *res.CostUSD != want {
		t.Fatalf("cost=%v, volia %v (up=%d down=%d)", *res.CostUSD, want, res.UpTokens, res.DownTokens)
	}
}

// El model escriu la crida al text en comptes d'estructurada: el loop no
// pot tornar el markup com a resposta; guia i continua (acotat).
func TestRunNonInteractiveTextCallRetry(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.NotFound(w, r) // sonda GET /models de DetectWindows: no és un pas
			return
		}
		w.Header().Set("Content-Type", "application/json")
		var body strings.Builder
		// El cos sencer: amb el mapa del projecte, el primer missatge passa
		// de 4 KB i una lectura sola no hi arribava.
		raw, _ := io.ReadAll(r.Body)
		body.Write(raw)
		if strings.Contains(body.String(), "NO s'ha executat") {
			fmt.Fprint(w, `{"choices":[{"message":{"role":"assistant","content":"recuperat"},"finish_reason":"stop"}]}`)
			return
		}
		fmt.Fprint(w, `{"choices":[{"message":{"role":"assistant","content":"Miro el disc.\n<tool_call><function=glob><parameter=pattern>*</parameter></function></tool_call>"},"finish_reason":"stop"}]}`)
	}))
	defer srv.Close()
	res, err := RunNonInteractive(context.Background(), llm.New(), testCfg(srv.URL), "mira", "code", 5, false)
	if err != nil {
		t.Fatal(err)
	}
	if res.Answer != "recuperat" || res.Steps != 2 {
		t.Fatalf("havia de reintentar estructurat: %+v", res)
	}
}

// Si el model insisteix escrivint en text, el topall tanca el torn sense
// penjar-se: la resposta és el text netejat, no un bucle infinit.
func TestRunNonInteractiveTextCallCap(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.NotFound(w, r) // sonda GET /models de DetectWindows: no és un pas
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"choices":[{"message":{"role":"assistant","content":"<tool_call><function=glob><parameter=pattern>*</parameter></function></tool_call>"},"finish_reason":"stop"}]}`)
	}))
	defer srv.Close()
	res, err := RunNonInteractive(context.Background(), llm.New(), testCfg(srv.URL), "mira", "code", 5, false)
	if err != nil {
		t.Fatal(err)
	}
	if res.Steps != 3 {
		t.Fatalf("dos reintents i prou: %+v", res)
	}
	if strings.Contains(res.Answer, "<tool_call") {
		t.Fatalf("el markup no pot arribar a la resposta: %q", res.Answer)
	}
}

// El torn no es pot tancar amb passos pendents: guia i continua (acotat).
func TestRunNonInteractiveTodosNudge(t *testing.T) {
	tools.TodoSet([]tools.TodoItem{{Title: "pas u", Status: "pending"}})
	t.Cleanup(tools.TodoClear)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.NotFound(w, r) // sonda GET /models de DetectWindows: no és un pas
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"choices":[{"message":{"role":"assistant","content":"Ara ho faig."},"finish_reason":"stop"}]}`)
	}))
	defer srv.Close()
	res, err := RunNonInteractive(context.Background(), llm.New(), testCfg(srv.URL), "fes", "code", 5, false)
	if err != nil {
		t.Fatal(err)
	}
	if res.Steps != 3 || res.Answer != "Ara ho faig." {
		t.Fatalf("dues guies i prou: %+v", res)
	}
}

func TestRunNonInteractiveAskDenied(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.NotFound(w, r) // sonda GET /models de DetectWindows: no és un pas
			return
		}
		w.Header().Set("Content-Type", "application/json")
		var body strings.Builder
		// El cos sencer: amb el mapa del projecte, el primer missatge passa
		// de 4 KB i una lectura sola no hi arribava.
		raw, _ := io.ReadAll(r.Body)
		body.Write(raw)
		if strings.Contains(body.String(), `"role":"tool"`) {
			fmt.Fprint(w, `{"choices":[{"message":{"role":"assistant","content":"entesos"},"finish_reason":"stop"}]}`)
			return
		}
		fmt.Fprint(w, `{"choices":[{"message":{"role":"assistant","content":"","tool_calls":[{"id":"c1","type":"function","function":{"name":"write","arguments":"{\"path\":\"/tmp/no.txt\",\"content\":\"x\"}"}}]},"finish_reason":"tool_calls"}]}`)
	}))
	defer srv.Close()
	res, err := RunNonInteractive(context.Background(), llm.New(), testCfg(srv.URL), "escriu", "code", 5, false)
	if err != nil {
		t.Fatal(err)
	}
	if res.Answer != "entesos" {
		t.Fatalf("answer=%q", res.Answer)
	}
	if _, err := os.Stat("/tmp/no.txt"); !os.IsNotExist(err) {
		t.Fatal("sense --auto-approve no hauria d'haver escrit")
	}
}

// Un 500 del servidor a mig torn no mata la feina: el client esgota els
// seus 3 intents interns i el loop guia el model perquè regeneri el pas.
// El servidor falla les 3 primeres crides i després funciona.
func TestRunNonInteractiveServerRetry(t *testing.T) {
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.NotFound(w, r) // sonda GET /models de DetectWindows: no és un pas
			return
		}
		hits++
		if hits <= 3 {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprint(w, `{"error":{"code":500,"message":"transitori"}}`)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		var body strings.Builder
		// El cos sencer: amb el mapa del projecte, el primer missatge passa
		// de 4 KB i una lectura sola no hi arribava.
		raw, _ := io.ReadAll(r.Body)
		body.Write(raw)
		if strings.Contains(body.String(), `"role":"tool"`) {
			fmt.Fprint(w, `{"choices":[{"message":{"role":"assistant","content":"recuperat"},"finish_reason":"stop"}]}`)
			return
		}
		fmt.Fprint(w, `{"choices":[{"message":{"role":"assistant","content":"","tool_calls":[{"id":"c1","type":"function","function":{"name":"todoread","arguments":"{}"}}]},"finish_reason":"tool_calls"}]}`)
	}))
	defer srv.Close()
	res, err := RunNonInteractive(context.Background(), llm.New(), testCfg(srv.URL), "fes", "code", 5, false)
	if err != nil {
		t.Fatalf("el 500 transitori no pot matar el torn: %v", err)
	}
	if res.Answer != "recuperat" {
		t.Fatalf("answer=%q", res.Answer)
	}
	if hits < 4 {
		t.Fatalf("cal que el loop hagi reintentat (hits=%d)", hits)
	}
}

func TestParseAmpliacio(t *testing.T) {
	for _, c := range []struct {
		in string
		n  int
		ok bool
	}{
		{"CONTINUA 5", 5, true},
		{"  continua 3  ", 3, true},
		{"CONTINUA", ChunkAmpliacio, true},
		{"CONTINUA 99", ChunkAmpliacio, true},
		{"continua 2, si us plau", 2, true},
		{"FINAL", 0, false},
		{"", 0, false},
		{"continuaré fent això", 0, false},
		{"No, FINAL.", 0, false},
		{"potser sí", 0, false},
		// Models pensadors: la decisió va dins d'una frase, i mana l'última.
		{"La checklist és a 9/11 i estic avançant, així que CONTINUA 10.", 10, true},
		{"**CONTINUA 8**", 8, true},
		{"Podria dir FINAL, però queda feina: CONTINUA 5", 5, true},
		{"CONTINUA no cal, la tasca està feta. FINAL", 0, false},
		{"Resposta: «CONTINUA»\nJustificació: 3 passos pendents", ChunkAmpliacio, true},
	} {
		n, ok := ParseAmpliacio(c.in)
		if n != c.n || ok != c.ok {
			t.Errorf("%q → (%d,%v), volia (%d,%v)", c.in, n, ok, c.n, c.ok)
		}
	}
}

// Esgotat el pressupost, el model decideix: CONTINUA amplia, FINAL tanca.
// Servidor d'estat: amb tools → 2 reads i després writes reals (fan
// créixer els execs: sense execs noves la 2a ampliació es denega); sense
// tools → 1a CONTINUA, 2a FINAL, 3a síntesi.
func TestRunNonInteractiveAmpliacio(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "d.txt")
	if err := os.WriteFile(f, []byte("DADES"), 0o600); err != nil {
		t.Fatal(err)
	}
	wf := filepath.Join(dir, "w.txt")
	inner, _ := json.Marshal(map[string]string{"path": f})
	argsField, _ := json.Marshal(string(inner))
	senseEines := 0
	eines := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.NotFound(w, r) // sonda GET /models de DetectWindows: no és un pas
			return
		}
		w.Header().Set("Content-Type", "application/json")
		var body strings.Builder
		b := make([]byte, 65536)
		for {
			n, err := r.Body.Read(b)
			body.Write(b[:n])
			if err != nil {
				break
			}
		}
		if !strings.Contains(body.String(), `"tools"`) {
			senseEines++
			switch senseEines {
			case 1:
				fmt.Fprint(w, `{"choices":[{"message":{"role":"assistant","content":"CONTINUA 2"},"finish_reason":"stop"}]}`)
			case 2:
				fmt.Fprint(w, `{"choices":[{"message":{"role":"assistant","content":"FINAL"},"finish_reason":"stop"}]}`)
			default:
				fmt.Fprint(w, `{"choices":[{"message":{"role":"assistant","content":"fet i prou"},"finish_reason":"stop"}]}`)
			}
			return
		}
		// Lectures i, per avançar el journal (progrés mesurable), una
		// escriptura de debò: sense avenços la 2a ampliació es denega.
		eines++
		if eines <= 2 {
			fmt.Fprintf(w, `{"choices":[{"message":{"role":"assistant","content":"","tool_calls":[{"id":"c1","type":"function","function":{"name":"read","arguments":%s}}]},"finish_reason":"tool_calls"}]}`, argsField)
			return
		}
		innerW, _ := json.Marshal(map[string]string{"path": wf, "content": fmt.Sprintf("v%d", eines)})
		argsW, _ := json.Marshal(string(innerW))
		fmt.Fprintf(w, `{"choices":[{"message":{"role":"assistant","content":"","tool_calls":[{"id":"c1","type":"function","function":{"name":"write","arguments":%s}}]},"finish_reason":"tool_calls"}]}`, argsW)
	}))
	defer srv.Close()
	res, err := RunNonInteractiveEx(context.Background(), llm.New(), testCfg(srv.URL), "fes", "code", 2, true, nil)
	if err != nil {
		t.Fatal(err)
	}
	// 2 passos + 2 d'ampliació; la 2a pregunta diu FINAL → síntesi.
	if res.Steps != 4 || res.Answer != "fet i prou" {
		t.Fatalf("res=%+v", res)
	}
	if len(res.Tools) != 4 {
		t.Fatalf("tools=%v", res.Tools)
	}
	if raw, err := os.ReadFile(wf); err != nil {
		t.Fatalf("l'escriptura havia de passar: %v", err)
	} else if string(raw) != "v4" {
		t.Fatalf("contingut=%q", raw)
	}
}

// Sense execs noves des de l'última ampliació no es torna a demanar
// (encara que no estigui encallat repetint): 2 reads, CONTINUA 1,
// 1 write denegat, síntesi. Només 2 peticions sense tools.
func TestRunNonInteractiveAmpliacioSenseExecs(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "d.txt")
	if err := os.WriteFile(f, []byte("DADES"), 0o600); err != nil {
		t.Fatal(err)
	}
	inner, _ := json.Marshal(map[string]string{"path": f})
	argsField, _ := json.Marshal(string(inner))
	innerW, _ := json.Marshal(map[string]string{"path": filepath.Join(dir, "w.txt"), "content": "x"})
	argsW, _ := json.Marshal(string(innerW))
	eines := 0
	senseEines := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.NotFound(w, r) // sonda GET /models de DetectWindows: no és un pas
			return
		}
		w.Header().Set("Content-Type", "application/json")
		var body strings.Builder
		b := make([]byte, 65536)
		for {
			n, err := r.Body.Read(b)
			body.Write(b[:n])
			if err != nil {
				break
			}
		}
		if !strings.Contains(body.String(), `"tools"`) {
			senseEines++
			if senseEines == 1 {
				fmt.Fprint(w, `{"choices":[{"message":{"role":"assistant","content":"CONTINUA 1"},"finish_reason":"stop"}]}`)
			} else {
				fmt.Fprint(w, `{"choices":[{"message":{"role":"assistant","content":"fet"},"finish_reason":"stop"}]}`)
			}
			return
		}
		eines++
		args := argsField
		if eines > 2 {
			args = argsW
		}
		name := "read"
		if eines > 2 {
			name = "write"
		}
		fmt.Fprintf(w, `{"choices":[{"message":{"role":"assistant","content":"","tool_calls":[{"id":"c1","type":"function","function":{"name":%q,"arguments":%s}}]},"finish_reason":"tool_calls"}]}`, name, args)
	}))
	defer srv.Close()
	res, err := RunNonInteractive(context.Background(), llm.New(), testCfg(srv.URL), "fes", "code", 2, false)
	if err != nil {
		t.Fatal(err)
	}
	if res.Steps != 3 || res.Answer != "fet" {
		t.Fatalf("res=%+v", res)
	}
	if senseEines != 2 {
		t.Fatalf("només ampliació+síntesi sense tools: %d", senseEines)
	}
	if _, err := os.Stat(filepath.Join(dir, "w.txt")); !os.IsNotExist(err) {
		t.Fatal("sense auto-approve no s'havia d'escriure")
	}
}

// Sense avenços entre extensions no es torna a demanar: segona
// esgotada amb el mateix estat → síntesi directa.
func TestRunNonInteractiveAmpliacioSenseAvenços(t *testing.T) {
	tools.TodoClear()
	t.Cleanup(tools.TodoClear)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.NotFound(w, r) // sonda GET /models de DetectWindows: no és un pas
			return
		}
		w.Header().Set("Content-Type", "application/json")
		// Sempre text final sense eines ni todos: res no avança.
		fmt.Fprint(w, `{"choices":[{"message":{"role":"assistant","content":"prou"},"finish_reason":"stop"}]}`)
	}))
	defer srv.Close()
	res, err := RunNonInteractive(context.Background(), llm.New(), testCfg(srv.URL), "fes", "code", 2, false)
	if err != nil {
		t.Fatal(err)
	}
	// Pas 1: final sense todos → resposta directa, sense ni demanar.
	if res.Steps != 1 || res.Answer != "prou" {
		t.Fatalf("res=%+v", res)
	}
}
