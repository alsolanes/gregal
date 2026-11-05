package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"gregal/internal/agent"
	"gregal/internal/config"
	"gregal/internal/llm"
)

// Deterministic local provider: request one real tool, then echo its result.
// No inference, remote service, or process-global tool replacement is involved.
func windPermissionProvider(t *testing.T, name, args string) *httptest.Server {
	t.Helper()
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Stream   bool          `json:"stream"`
			Messages []llm.Message `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		message := map[string]any{"role": "assistant", "tool_calls": []any{map[string]any{"id": "permission-call", "type": "function", "function": map[string]string{"name": name, "arguments": args}}}}
		for _, m := range req.Messages {
			if m.Role == "tool" {
				message = map[string]any{"role": "assistant", "content": m.Content}
			}
		}
		if req.Stream {
			w.Header().Set("Content-Type", "text/event-stream")
			if calls, ok := message["tool_calls"].([]any); ok {
				calls[0].(map[string]any)["index"] = 0
			}
			b, _ := json.Marshal(map[string]any{"choices": []any{map[string]any{"delta": message, "finish_reason": "stop"}}})
			fmt.Fprintf(w, "data: %s\n\ndata: [DONE]\n\n", b)
		} else {
			json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": message}}})
		}
	}))
	t.Cleanup(provider.Close)
	return provider
}

// windScriptProvider fa la feina d'un torn de codi de veritat: demana una
// escriptura diferent a cada pas (el comptador és del proveïdor, no del que
// veu, perquè el guard de repeticions del bucle no les confongui) i només
// quan ha servit need passos dona la resposta final.
func windScriptProvider(t *testing.T, need int, final string) *httptest.Server {
	t.Helper()
	var mu sync.Mutex
	n := 0
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Stream   bool          `json:"stream"`
			Messages []llm.Message `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		mu.Lock()
		pas := n
		if pas < need {
			n++
		}
		mu.Unlock()
		var message map[string]any
		if pas >= need {
			message = map[string]any{"role": "assistant", "content": final}
		} else {
			args, _ := json.Marshal(map[string]string{"path": fmt.Sprintf("pas%d.txt", pas), "content": "fet"})
			message = map[string]any{"role": "assistant", "tool_calls": []any{map[string]any{
				"id": fmt.Sprintf("call-%d", pas), "type": "function",
				"function": map[string]string{"name": "write", "arguments": string(args)},
			}}}
		}
		if req.Stream {
			w.Header().Set("Content-Type", "text/event-stream")
			if calls, ok := message["tool_calls"].([]any); ok {
				calls[0].(map[string]any)["index"] = 0
			}
			b, _ := json.Marshal(map[string]any{"choices": []any{map[string]any{"delta": message, "finish_reason": "stop"}}})
			fmt.Fprintf(w, "data: %s\n\ndata: [DONE]\n\n", b)
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": message}}})
	}))
	t.Cleanup(provider.Close)
	return provider
}

// Una tasca de codi que necessita més passos que una sola ronda s'acaba tota
// sola: el torn reprèn el pressupost en comptes de tancar amb un resum a mig
// fer. Amb l'auto-continuació desactivada, la mateixa tasca queda parcial —
// això és el que demostra que el mecanisme és el que la fa acabar.
func TestWindTurnAcabaTascaLLarga(t *testing.T) {
	old, oldSteps := windAutoContinues, windTurnSteps
	t.Cleanup(func() { windAutoContinues, windTurnSteps = old, oldSteps })
	windTurnSteps = 6
	const passos = 9 // més que els 6 d'una ronda
	for _, cas := range []struct {
		nom       string
		continues int
		volAcabar bool
	}{
		{"amb auto-continuació", 2, true},
		{"sense auto-continuació", 0, false},
	} {
		t.Run(cas.nom, func(t *testing.T) {
			windAutoContinues = cas.continues
			t.Setenv("GREGAL_DATA_DIR", t.TempDir())
			s, _ := companionTestServer(t)
			s.cwd = t.TempDir()
			s.mode = "code"
			makeRotllanaTrio(t, s)
			provider := windScriptProvider(t, passos, "tasca acabada")
			s.cfg.Providers["p"] = config.Provider{BaseURL: provider.URL + "/v1"}
			body, _ := json.Marshal(map[string]any{"wind": "tramuntana", "message": "crea els nou fitxers"})
			rec := postJSON(t, s.handleWindChat, string(body))
			if rec.Code != 200 {
				t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
			}
			var out struct {
				Reply   string `json:"reply"`
				Parcial bool   `json:"parcial"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
				t.Fatalf("resposta il·legible: %v", err)
			}
			fets := 0
			for i := 0; i < passos; i++ {
				if _, err := os.Stat(filepath.Join(s.cwd, fmt.Sprintf("pas%d.txt", i))); err == nil {
					fets++
				}
			}
			if cas.volAcabar {
				if out.Parcial || out.Reply != "tasca acabada" || fets != passos {
					t.Fatalf("parcial=%v reply=%q fets=%d/%d (calia acabar la feina)", out.Parcial, out.Reply, fets, passos)
				}
			} else if !out.Parcial || fets >= passos {
				t.Fatalf("sense auto-continuació havia de quedar parcial: parcial=%v fets=%d", out.Parcial, fets)
			}
		})
	}
}

// Aturar un torn: el vent queda treballant, la persona demana "atura" i el
// torn respon de seguida en comptes d'esperar el límit dur (cinc minuts).
func TestWindCancelAturaElTorn(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	s, _ := companionTestServer(t)
	s.cwd = t.TempDir()
	s.mode = "code"
	makeRotllanaTrio(t, s)
	started := make(chan struct{}, 1)
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case started <- struct{}{}:
		default:
		}
		// Penjat: si l'aturada no arriba a la crida, el torn s'hi queda.
		select {
		case <-r.Context().Done():
		case <-time.After(3 * time.Second):
		}
		http.Error(w, "cancel·lat", 499)
	}))
	t.Cleanup(provider.Close)
	s.cfg.Providers["p"] = config.Provider{BaseURL: provider.URL + "/v1"}
	body, _ := json.Marshal(map[string]any{"wind": "tramuntana", "message": "fes una cosa llarga"})

	fet := make(chan *httptest.ResponseRecorder, 1)
	go func() { fet <- postJSON(t, s.handleWindChat, string(body)) }()

	select {
	case <-started:
	case <-time.After(10 * time.Second):
		t.Fatal("el proveïdor no ha rebut cap petició")
	}
	aturada := time.Now()
	if !s.cancelWindTurn("wind", "tramuntana") {
		t.Fatal("el torn no estava registrat: no es podria aturar")
	}
	select {
	case rec := <-fet:
		if dur := time.Since(aturada); dur > 1500*time.Millisecond {
			t.Fatalf("l'aturada ha trigat %.2fs: no arriba a la crida al model", dur.Seconds())
		}
		if !strings.Contains(rec.Body.String(), "aturat") {
			t.Fatalf("el torn no diu que s'ha aturat: %d %s", rec.Code, rec.Body.String())
		}
	case <-time.After(10 * time.Second):
		t.Fatal("el torn no s'ha aturat: continua penjat després de cancel·lar")
	}
	// Cancel·lar dues vegades no és cap error (la UI pot repetir el gest).
	if s.cancelWindTurn("wind", "tramuntana") {
		t.Fatal("després d'acabar, el torn ja no ha d'estar registrat")
	}
}

// L'endpoint és idempotent i diu si hi havia res a aturar.
func TestWindCancelEndpoint(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	s, _ := companionTestServer(t)
	rec := postJSON(t, s.handleWindCancel, `{"wind":"tramuntana"}`)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"aturat":false`) {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	if rec := postJSON(t, s.handleWindCancel, `{}`); rec.Code != 400 {
		t.Fatalf("sense vent ni rotllana ha de ser 400: %d", rec.Code)
	}
}

func TestWindInteractiveBashAutoAllow(t *testing.T) {
	// L'ordre del xat aprova sola la feina segura d'aquí: un mkdir al
	// projecte s'executa; un curl a internet continua bloquejat.
	for _, cas := range []struct {
		nom     string
		command string
		volDir  bool
		volBloc bool
	}{
		{"mkdir dins", "mkdir -p autosub", true, false},
		{"curl fora", "curl https://example.com/x", false, true},
	} {
		t.Run(cas.nom, func(t *testing.T) {
			t.Setenv("GREGAL_DATA_DIR", t.TempDir())
			s, _ := companionTestServer(t)
			s.cwd = t.TempDir()
			s.mode = "code"
			makeRotllanaTrio(t, s)
			args, _ := json.Marshal(map[string]any{"command": cas.command})
			provider := windPermissionProvider(t, "bash", string(args))
			s.cfg.Providers["p"] = config.Provider{BaseURL: provider.URL + "/v1"}
			body, _ := json.Marshal(map[string]any{"wind": "tramuntana", "message": "fes: " + cas.command})
			rec := postJSON(t, s.handleWindChat, string(body))
			if rec.Code != 200 {
				t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
			}
			if cas.volDir {
				if st, err := os.Stat(filepath.Join(s.cwd, "autosub")); err != nil || !st.IsDir() {
					t.Fatalf("el mkdir del vent no s'ha executat: %v; response=%s", err, rec.Body.String())
				}
			}
			teBloc := strings.Contains(rec.Body.String(), "Not run") ||
				strings.Contains(rec.Body.String(), "BLOQUEJADA") ||
				strings.Contains(rec.Body.String(), "bloquejat")
			if teBloc != cas.volBloc {
				t.Fatalf("bloqueig=%v, volia %v; response=%s", teBloc, cas.volBloc, rec.Body.String())
			}
		})
	}
}

func TestWindInteractivePermissionsUseSessionWorkspace(t *testing.T) {
	for _, group := range []bool{false, true} {
		for _, stream := range []bool{false, true} {
			for _, cont := range []bool{false, true} {
				t.Run(fmt.Sprintf("group=%v/stream=%v/continue=%v", group, stream, cont), func(t *testing.T) {
					t.Setenv("GREGAL_DATA_DIR", t.TempDir())
					s, _ := companionTestServer(t)
					s.cwd = t.TempDir() // deliberately not the original policy's ProjectDir
					s.mode = "code"
					args := `{"path":"created.txt","content":"authorized project edit"}`
					provider := windPermissionProvider(t, "write", args)
					s.cfg.Providers["p"] = config.Provider{BaseURL: provider.URL + "/v1"}
					id := makeRotllanaTrio(t, s)
					body := map[string]any{"wind": "tramuntana", "message": "create the project file", "continue": cont}
					handler := s.handleWindChat
					if stream {
						handler = s.handleWindChatStream
					}
					if group {
						body["rotllana"] = id
						handler = s.handleRotllanaChat
						if stream {
							handler = s.handleRotllanaChatStream
						}
					}
					if cont {
						history := []llm.Message{{Role: "user", Content: "create the file"}, {Role: "assistant", Content: "partial", SpeakerID: "tramuntana", SpeakerName: "Tramuntana"}}
						body["message"] = ""
						if group {
							rotllanaHistory(s, id).messages = history
						} else {
							windHistory(s, "tramuntana").messages = history
						}
					}
					b, _ := json.Marshal(body)
					rec := postJSON(t, handler, string(b))
					if rec.Code != 200 {
						t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
					}
					data, err := os.ReadFile(filepath.Join(s.cwd, "created.txt"))
					if err != nil || string(data) != "authorized project edit" {
						t.Fatalf("interactive write did not reach session workspace: %v %q; response=%s", err, data, rec.Body.String())
					}
				})
			}
		}
	}
}

func TestWindReadOnlySessionCannotScheduleReminderThroughTool(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	s, _ := companionTestServer(t)
	s.cwd = t.TempDir()
	s.mode = "chat"
	makeRotllanaTrio(t, s)
	provider := windPermissionProvider(t, "remind", `{"wind":"tramuntana","at":"+30m","text":"read-only test"}`)
	s.cfg.Providers["p"] = config.Provider{BaseURL: provider.URL + "/v1"}
	rec := postJSON(t, s.handleWindChat, `{"wind":"tramuntana","message":"fes una cosa"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	list, err := s.jobs.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Fatalf("read-only wind scheduled a persistent reminder: %+v; response=%s", list, rec.Body.String())
	}
}

func TestReadOnlyWindCannotRunClassifiedSafeShellThatWrites(t *testing.T) {
	for _, mode := range []string{agent.ModeChat, agent.ModePlan, agent.ModeInspect, agent.ModeGoal} {
		t.Run(mode, func(t *testing.T) {
			t.Setenv("GREGAL_DATA_DIR", t.TempDir())
			s, _ := companionTestServer(t)
			s.cwd = t.TempDir()
			s.mode = mode
			makeRotllanaTrio(t, s)
			if err := os.WriteFile(filepath.Join(s.cwd, "go.mod"), []byte("module read-only-test\n\ngo 1.23\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			testSource := `package readonly
import ("os"; "testing")
func TestWriteMarker(t *testing.T) { if err := os.WriteFile("marker.txt", []byte("written"), 0600); err != nil { t.Fatal(err) } }
`
			if err := os.WriteFile(filepath.Join(s.cwd, "marker_test.go"), []byte(testSource), 0o600); err != nil {
				t.Fatal(err)
			}
			args, _ := json.Marshal(map[string]string{"command": "go test -run TestWriteMarker"})
			provider := windPermissionProvider(t, "bash", string(args))
			s.cfg.Providers["p"] = config.Provider{BaseURL: provider.URL + "/v1"}
			rec := postJSON(t, s.handleWindChat, `{"wind":"tramuntana","message":"check the project"}`)
			if rec.Code != http.StatusOK {
				t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
			}
			if _, err := os.Stat(filepath.Join(s.cwd, "marker.txt")); !os.IsNotExist(err) {
				t.Fatalf("read-only mode %q executed safe-classified shell that wrote marker (stat err %v)", mode, err)
			}
		})
	}
}
