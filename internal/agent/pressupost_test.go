package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"gregal/internal/llm"
	"gregal/internal/tools"
)

// Un model que no para mai de cridar eines no pot deixar el torn buit: la
// tirada de 2026-10-16 («Plan · error: límit de 10 passos exhaurit»).

// tossut és un Stepper que sempre demana una lectura nova (arguments
// diferents a cada pas, perquè el detector de repeticions no hi entri).
func tossut(n *int) Stepper {
	return func(ctx context.Context, h []llm.Message) (string, []llm.ToolCall, error) {
		*n++
		return "", []llm.ToolCall{mkCall(fmt.Sprintf("c%d", *n), "read", fmt.Sprintf(`{"path":"f%d.go"}`, *n))}, nil
	}
}

func TestLoopTossutAcabaAmbRespostaParcial(t *testing.T) {
	passos, execs := 0, 0
	var finalHist []llm.Message
	l := &Loop{
		MaxSteps: 4,
		Lang:     "ca",
		Step:     tossut(&passos),
		RunTool: func(ctx context.Context, n, a string) (string, []string, error) {
			execs++
			return "contingut", nil, nil
		},
		Decide: allowAll,
		Finalize: func(ctx context.Context, h []llm.Message) (string, error) {
			finalHist = h
			return "1. Pas parcial\nPENDENT: revisar web/", nil
		},
	}
	final, hist, err := l.Run(context.Background(), "revisa el projecte")
	if err != nil {
		t.Fatalf("quedar-se sense passos no és un error: %v", err)
	}
	if !strings.Contains(final, "Pas parcial") || !l.Parcial || l.Motiu != MotiuPassos {
		t.Fatalf("final=%q parcial=%v motiu=%q", final, l.Parcial, l.Motiu)
	}
	if execs != 4 {
		t.Fatalf("execs=%d: només s'executen els 4 passos, cap a la síntesi", execs)
	}
	// Avís dos passos abans del final, una sola vegada, a l'historial.
	avisos := 0
	for _, m := range hist {
		if m.Role == "user" && strings.Contains(m.Content, "STEP BUDGET") {
			avisos++
		}
	}
	if avisos != 1 {
		t.Fatalf("avisos de pressupost=%d, volia 1", avisos)
	}
	if last := finalHist[len(finalHist)-1]; last.Role != "user" || !strings.Contains(last.Content, "tools are now disabled") {
		t.Fatalf("la síntesi ha de demanar la resposta sense eines: %+v", last)
	}
	if hist[len(hist)-1].Content != final {
		t.Fatalf("la resposta final ha de quedar a l'historial per continuar")
	}

	// Continuar: pressupost nou, conversa intacta.
	final2, hist2, err := l.Continua(context.Background(), nil)
	if err != nil || final2 == "" {
		t.Fatalf("continuar: final=%q err=%v", final2, err)
	}
	if execs != 8 {
		t.Fatalf("execs=%d: continuar dona 4 passos més", execs)
	}
	if hist2[0].Content != "revisa el projecte" {
		t.Fatalf("la conversa ha de continuar sencera: %q", hist2[0].Content)
	}
	trobat := false
	for _, m := range hist2 {
		if m.Role == "user" && m.Content == PromptContinuaExploracio {
			trobat = true
		}
	}
	if !trobat {
		t.Fatal("falta la consigna de continuar")
	}
}

func TestLoopSenseFinalizeNoExecutaLesCridesDeLaSintesi(t *testing.T) {
	passos, execs := 0, 0
	l := &Loop{
		MaxSteps: 3,
		Step:     tossut(&passos),
		RunTool: func(ctx context.Context, n, a string) (string, []string, error) {
			execs++
			return "x", nil, nil
		},
		Decide: allowAll,
	}
	final, _, err := l.Run(context.Background(), "bucle")
	if err != nil {
		t.Fatalf("sense Finalize tampoc és un error: %v", err)
	}
	if execs != 3 {
		t.Fatalf("execs=%d: la crida de la síntesi no s'ha d'executar", execs)
	}
	// El model ha tornat només una crida: el resum mecànic, mai un buit.
	if !strings.Contains(final, "Partial answer") || !strings.Contains(final, "read") {
		t.Fatalf("resum mecànic esperat: %q", final)
	}
}

func TestLoopErrorAMigCamiResumeix(t *testing.T) {
	n := 0
	l := &Loop{
		MaxSteps: 10,
		Step: func(ctx context.Context, h []llm.Message) (string, []llm.ToolCall, error) {
			n++
			if n == 3 {
				return "", nil, errors.New("timeout del proveïdor")
			}
			return "", []llm.ToolCall{mkCall(fmt.Sprintf("c%d", n), "read", fmt.Sprintf(`{"path":"%d"}`, n))}, nil
		},
		RunTool:  func(ctx context.Context, n, a string) (string, []string, error) { return "x", nil, nil },
		Decide:   allowAll,
		Finalize: func(ctx context.Context, h []llm.Message) (string, error) { return "el que he vist", nil },
	}
	final, _, err := l.Run(context.Background(), "t")
	if err != nil || final != "el que he vist" || !l.Parcial || !strings.Contains(l.Motiu, "timeout") {
		t.Fatalf("final=%q err=%v parcial=%v motiu=%q", final, err, l.Parcial, l.Motiu)
	}
	// Sense res fet, l'error sí que és la resposta.
	l2 := &Loop{MaxSteps: 3, Decide: allowAll,
		Step: func(ctx context.Context, h []llm.Message) (string, []llm.ToolCall, error) {
			return "", nil, errors.New("401")
		}}
	if _, _, err := l2.Run(context.Background(), "t"); err == nil {
		t.Fatal("un error al primer pas no té feina a salvar")
	}
}

func TestAmbAvisPassosNoEncadenaDosUsuaris(t *testing.T) {
	h := []llm.Message{{Role: "user", Content: "tasca"}, {Role: "assistant", Content: "a"}, {Role: "user", Content: "guia"}}
	out := AmbAvisPassos(h, 4, 5)
	if len(out) != 3 || !strings.Contains(out[2].Content, "guia") || !strings.Contains(out[2].Content, "STEP BUDGET") {
		t.Fatalf("l'avís s'ha d'enganxar a l'últim missatge d'usuari: %+v", out)
	}
	if len(h[2].Content) != len("guia") {
		t.Fatal("no s'ha de modificar l'historial original")
	}
	if got := AmbAvisPassos(h, 3, 5); len(got) != 3 || got[2].Content != "guia" {
		t.Fatal("fora del penúltim pas no s'avisa")
	}
}

func TestPassosPlaTeUnMinim(t *testing.T) {
	if PassosPla(10) != PassosMinimsPla || PassosPla(40) != 40 {
		t.Fatalf("PassosPla(10)=%d PassosPla(40)=%d", PassosPla(10), PassosPla(40))
	}
}

// servidorTossut és un proveïdor fals que amb eines sempre torna una
// lectura nova, i sense eines (síntesi, pregunta d'ampliació) respon FINAL
// o el resum, però hi enganxa una crida write: no s'ha d'executar mai.
func servidorTossut(t *testing.T, dir string, sentinella string, ambEines *atomic.Int32, senseEines *atomic.Int32) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		var req struct {
			Messages []llm.Message    `json:"messages"`
			Tools    []json.RawMessage `json:"tools"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Error(err)
			http.Error(w, "bad", 400)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		write, _ := json.Marshal(map[string]string{"path": sentinella, "content": "NO"})
		writeArg, _ := json.Marshal(string(write))
		if len(req.Tools) == 0 {
			senseEines.Add(1)
			last := req.Messages[len(req.Messages)-1].Content
			text := "Pla parcial: 1. revisar loop.go\\nPENDENT: web/"
			if strings.Contains(last, "CONTINUE <n>") {
				text = "FINAL"
			}
			fmt.Fprintf(w, `{"choices":[{"message":{"role":"assistant","content":"%s","tool_calls":[{"id":"w1","type":"function","function":{"name":"write","arguments":%s}}]},"finish_reason":"stop"}]}`, text, writeArg)
			return
		}
		n := ambEines.Add(1)
		args, _ := json.Marshal(map[string]string{"path": filepath.Join(dir, fmt.Sprintf("f%d.txt", n))})
		wire, _ := json.Marshal(string(args))
		fmt.Fprintf(w, `{"choices":[{"message":{"role":"assistant","content":"","tool_calls":[{"id":"c%d","type":"function","function":{"name":"read","arguments":%s}}]},"finish_reason":"tool_calls"}]}`, n, wire)
	}))
}

// El motor (Ask/Act, Plan/inspect, Goal, Debug, Autonomous; -p, web i
// Telegram el fan servir) amb un model que no para mai: el torn acaba amb
// resposta de text, no amb error, i la síntesi no executa res.
func TestMotorTossutAcabaAmbRespostaEnTotsElsModes(t *testing.T) {
	tools.TodoClear()
	for _, mode := range []string{ModeCode, ModeInspect, ModeGoal, ModeDebug, ModeAutonomous} {
		for _, dur := range []bool{true, false} {
			t.Run(fmt.Sprintf("%s/dur=%v", mode, dur), func(t *testing.T) {
				dir := t.TempDir()
				for i := 1; i <= 120; i++ {
					_ = os.WriteFile(filepath.Join(dir, fmt.Sprintf("f%d.txt", i)), []byte(fmt.Sprintf("dada %d", i)), 0o600)
				}
				sentinella := filepath.Join(dir, "NO_ESCRIT.txt")
				var amb, sense atomic.Int32
				srv := servidorTossut(t, dir, sentinella, &amb, &sense)
				defer srv.Close()
				cfg := testCfg(srv.URL)
				res, err := RunNonInteractiveOpts(context.Background(), llm.New(), cfg, RunOpts{
					Task: "revisa a fons el projecte", Mode: mode, MaxSteps: 5, HardStepLimit: dur,
					AutoApprove: false, Dir: dir,
				})
				if err != nil {
					t.Fatalf("el torn no pot acabar en error: %v", err)
				}
				if !strings.Contains(res.Answer, "Pla parcial") {
					t.Fatalf("cal una resposta final de text: %q", res.Answer)
				}
				if !res.Exhausted {
					t.Fatal("la resposta s'ha de marcar com a parcial (exhausted)")
				}
				if _, err := os.Stat(sentinella); err == nil {
					t.Fatal("la síntesi ha executat una eina")
				}
				for _, tr := range res.Trace {
					if tr.Tool != "read" {
						t.Fatalf("eina inesperada al trace: %+v", tr)
					}
				}
				if sense.Load() == 0 {
					t.Fatal("no s'ha fet la crida final sense eines")
				}
			})
		}
	}
}

func TestMotorAvisaDosPassosAbansDelFinal(t *testing.T) {
	tools.TodoClear()
	tn := tornProva(ModeCode, 4)
	tn.o.SenseAmpliacions = true
	for i := 1; i <= 4; i++ {
		p := tn.Seguent()
		if p.Ordre != OrdrePasModel {
			t.Fatalf("pas %d: %v", i, p.Ordre)
		}
		avisat := strings.Contains(p.Hist[len(p.Hist)-1].Content, "STEP BUDGET")
		if avisat != (i == 3) {
			t.Fatalf("pas %d: avís=%v (només al penúltim)", i, avisat)
		}
		tn.RepPas("", []llm.ToolCall{cridaProva(fmt.Sprint(i), "read", fmt.Sprintf(`{"path":"%d.go"}`, i))}, nil)
		p = tn.Seguent()
		tn.RepExecucions([]Execucio{{Call: p.Calls[0], Sortida: "ok"}})
	}
	if p := tn.Seguent(); p.Ordre != OrdreSintesi {
		t.Fatalf("topall dur: síntesi, no %v", p.Ordre)
	}
}

func TestMotorErrorAMigTornFaSintesiIResumMecanic(t *testing.T) {
	tools.TodoClear()
	tn := tornProva(ModeCode, 10)
	tn.Seguent()
	tn.RepPas("", []llm.ToolCall{cridaProva("1", "read", `{"path":"x.go"}`)}, nil)
	p := tn.Seguent()
	tn.RepExecucions([]Execucio{{Call: p.Calls[0], Sortida: "ok"}})
	tn.Seguent()
	tn.RepPas("", nil, errors.New("model inactiu 240s"))
	if tn.Acabat() {
		t.Fatal("amb feina feta, un error no mata el torn: primer la síntesi")
	}
	if p := tn.Seguent(); p.Ordre != OrdreSintesi {
		t.Fatalf("esperava síntesi: %v", p.Ordre)
	}
	// La síntesi també falla: resum mecànic, mai un torn buit.
	evs := tn.RepSintesi("", errors.New("proveïdor caigut"))
	if !tn.Acabat() || tn.Fallit() || !strings.Contains(tn.Resposta(), "read") {
		t.Fatalf("acabat=%v fallit=%v resposta=%q", tn.Acabat(), tn.Fallit(), tn.Resposta())
	}
	if tn.Interromput() == "" {
		t.Fatal("cal saber que el torn s'ha interromput")
	}
	hiHaResposta := false
	for _, e := range evs {
		if e.Tipus == EvResposta {
			hiHaResposta = true
		}
	}
	if !hiHaResposta {
		t.Fatal("el resum mecànic ha d'arribar com a resposta")
	}
	// Sense feina feta, l'error continua sent un error.
	tn2 := tornProva(ModeCode, 10)
	tn2.Seguent()
	tn2.RepPas("", nil, errors.New("401 unauthorized"))
	if !tn2.Fallit() {
		t.Fatal("sense feina no hi ha res a resumir")
	}
}

func TestSubagentTossutTornaRespostaParcial(t *testing.T) {
	dir := t.TempDir()
	for i := 1; i <= 30; i++ {
		_ = os.WriteFile(filepath.Join(dir, fmt.Sprintf("f%d.txt", i)), []byte("x"), 0o600)
	}
	sentinella := filepath.Join(dir, "NO_ESCRIT.txt")
	var amb, sense atomic.Int32
	srv := servidorTossut(t, dir, sentinella, &amb, &sense)
	defer srv.Close()
	cfg := testCfg(srv.URL)
	runner := buildDelegateRunner(cfg, llm.New())
	ctx := withDelegateScope(context.Background(), "", dir)
	out, err := runner(ctx, "investiga")
	if err != nil {
		t.Fatalf("el subagent no pot tornar error per passos: %v", err)
	}
	if !strings.HasPrefix(out, "[PARTIAL") || !strings.Contains(out, "Pla parcial") {
		t.Fatalf("resposta parcial marcada esperada: %q", out)
	}
	if got := amb.Load(); got != 12 {
		t.Fatalf("passos amb eines=%d, volia 12", got)
	}
	if _, err := os.Stat(sentinella); err == nil {
		t.Fatal("la síntesi del subagent ha executat una eina")
	}
}
