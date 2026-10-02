package agent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gregal/internal/llm"
)

func histDeProva(passos int) []llm.Message {
	h := []llm.Message{{Role: "system", Content: "sys"}, {Role: "user", Content: "Tasca: arregla el bug"}}
	for i := 0; i < passos; i++ {
		c := crida("read", `{"path":"f.go"}`)
		h = append(h, llm.Message{Role: "assistant", Content: "", ToolCalls: []llm.ToolCall{c}})
		h = append(h, llm.Message{Role: "tool", ToolCallID: c.ID, Name: "read", Content: strings.Repeat("línia de codi\n", 200)})
	}
	return h
}

func TestPruneToolOutputsNomesLesVelles(t *testing.T) {
	h := histDeProva(10)
	out, n := PruneToolOutputs(h)
	if n != 10-KeepRecentToolOutputs {
		t.Fatalf("retallades %d", n)
	}
	// Les últimes sis sencers; les altres curtes amb nota; l'original intacte.
	tools := 0
	for i := len(out) - 1; i >= 0; i-- {
		if out[i].Role != "tool" {
			continue
		}
		tools++
		llarg := len(out[i].Content) > 2000
		if tools <= KeepRecentToolOutputs && !llarg {
			t.Fatalf("resultat recent %d retallat", tools)
		}
		if tools > KeepRecentToolOutputs && (llarg || !strings.Contains(out[i].Content, "retallada")) {
			t.Fatalf("resultat vell %d no retallat: %d", tools, len(out[i].Content))
		}
	}
	if len(h[2+1].Content) < 2000 {
		t.Fatal("l'historial original s'ha modificat")
	}
	// Idempotent: una segona passada no torna a comptar.
	if _, n2 := PruneToolOutputs(out); n2 != 0 {
		t.Fatalf("segona passada retalla %d", n2)
	}
}

func TestSafeCutNoComencaPerTool(t *testing.T) {
	h := histDeProva(5)  // system, user, (assistant, tool) x5 = 12
	cut := SafeCut(h, 3) // 12-3 = 9 → tool; ha de recular a l'assistant (8)
	if h[cut].Role == "tool" || cut != 8 {
		t.Fatalf("cut=%d role=%s", cut, h[cut].Role)
	}
	// El tall es menja la consigna: torna davant de la cua com a user.
	tk := TrimKeepSafe(h, 3)
	if tk[0].Role != "system" || tk[1].Role != "user" || !strings.Contains(tk[1].Content, "arregla el bug") || tk[2].Role != "assistant" || len(tk) != 6 {
		t.Fatalf("trim: %d msgs, %s %s", len(tk), tk[0].Role, tk[1].Role)
	}
	if SafeCut([]llm.Message{{Role: "user", Content: "x"}}, 4) != 0 {
		t.Fatal("historial curt: tall a 0")
	}
}

func servidorResum(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"role": "assistant", "content": "RESUM: s'ha llegit f.go deu vegades."}}}})
	}))
}

func TestMakeRoomEnEtapes(t *testing.T) {
	srv := servidorResum(t)
	defer srv.Close()
	client := llm.New()
	prim := llm.Target{BaseURL: srv.URL, Model: "m"}
	h := histDeProva(12)
	// Finestra gran: no fa res.
	if r := MakeRoom(context.Background(), client, prim, nil, "", h, 1_000_000); r.Changed() {
		t.Fatal("amb finestra gran no s'ha de tocar res")
	}
	// Finestra mitjana: el retall de sortides velles ja basta.
	tok := llm.EstimateTokens(h)
	r := MakeRoom(context.Background(), client, prim, nil, "", h, int(float64(tok)*0.9))
	if r.Pruned == 0 || r.Summarized || r.Trimmed || len(r.Hist) != len(h) {
		t.Fatalf("etapa 1: %+v", r.Note())
	}
	// Finestra petita: cal resum; queda system + resum + cua, cap tool orfe.
	r = MakeRoom(context.Background(), client, prim, nil, "", h, 800)
	if !r.Summarized || r.Trimmed {
		t.Fatalf("etapa 2: %s (err %v)", r.Note(), r.Err)
	}
	if r.Hist[0].Role != "system" || !strings.Contains(r.Hist[1].Content, "RESUM: s'ha llegit") || !strings.Contains(r.Hist[1].Content, "Tasca: arregla el bug") {
		t.Fatalf("capçalera: %q / %q", r.Hist[0].Role, r.Hist[1].Content)
	}
	if r.Hist[2].Role == "tool" {
		t.Fatal("la cua comença per un tool orfe")
	}
	if r.After >= r.Before || !strings.Contains(r.Note(), "resumits") {
		t.Fatalf("no ha reduït: %s", r.Note())
	}
	// Sense client (o resum fallit): retall segur, amb la tasca al davant.
	r = MakeRoom(context.Background(), nil, prim, nil, "", h, 800)
	if !r.Trimmed || r.Summarized || r.Hist[0].Role != "system" || r.Hist[1].Role != "user" || !strings.Contains(r.Hist[1].Content, "arregla el bug") {
		t.Fatalf("etapa 3: %s / %s %s", r.Note(), r.Hist[1].Role, r.Hist[1].Content)
	}
}

// Un sol pas amb quatre lectures grosses que no caben: cap és vella, però
// s'han de retallar igualment (les dues últimes queden senceres).
func TestMakeRoomPocsMissatgesRetallaRecents(t *testing.T) {
	h := histDeProva(1)
	c := h[len(h)-2].ToolCalls[0]
	h = h[:len(h)-2]
	h = append(h, llm.Message{Role: "assistant", ToolCalls: []llm.ToolCall{c, c, c, c}})
	for i := 0; i < 4; i++ {
		h = append(h, llm.Message{Role: "tool", ToolCallID: c.ID, Name: "read", Content: strings.Repeat("codi ", 1500)})
	}
	r := MakeRoom(context.Background(), nil, llm.Target{}, nil, "", h, 3000)
	if r.Pruned < 2 || r.Summarized || r.Trimmed || len(r.Hist) != len(h) {
		t.Fatalf("pocs missatges: %s (pruned=%d)", r.Note(), r.Pruned)
	}
	if len(r.Hist[len(r.Hist)-1].Content) < 5000 || len(r.Hist[3].Content) > 2000 {
		t.Fatal("cal retallar les dues primeres lectures i deixar les dues últimes")
	}
}

func TestMakeRoomSenseSystemAlDavant(t *testing.T) {
	h := histDeProva(8)[1:] // com el TUI: el system va a part
	r := MakeRoom(context.Background(), nil, llm.Target{}, nil, "el system", h, 600)
	if !r.Trimmed || r.Hist[0].Role != "user" || r.Hist[1].Role == "tool" {
		t.Fatalf("sense system: %s %s", r.Hist[0].Role, r.Note())
	}
}

func TestPlanCallsOrdreIDoom(t *testing.T) {
	pol := &Policy{}
	doom := &DoomTracker{}
	calls := []llm.ToolCall{crida("read", `{"path":"a"}`), crida("write", `{"path":"b","content":"x"}`), crida("bash", `{"command":"rm -rf /"}`)}
	plans := PlanCalls(pol, ModeCode, doom, calls)
	if plans[0].Fixed != "" || plans[0].Ask {
		t.Fatalf("read: %+v", plans[0])
	}
	if !plans[1].Ask {
		t.Fatalf("write fora de projecte ha de demanar: %+v", plans[1])
	}
	if plans[2].Fixed == "" && !plans[2].Ask {
		t.Fatalf("bash perillós: %+v", plans[2])
	}
	// La mateixa lectura tres cops seguits: la tercera és repetida.
	rep := []llm.ToolCall{crida("read", `{"path":"a"}`), crida("read", `{"path":"a"}`), crida("read", `{"path":"a"}`)}
	p2 := PlanCalls(pol, ModeInspect, &DoomTracker{}, rep)
	if p2[1].Fixed != "" || p2[2].Fixed != DoomGuide {
		t.Fatalf("doom: %+v", p2)
	}
	// En mode consulta, write es bloqueja i no compta per al doom.
	p3 := PlanCalls(pol, ModeInspect, nil, calls[1:2])
	if !strings.HasPrefix(p3[0].Fixed, "EINA BLOQUEJADA") {
		t.Fatalf("inspect: %+v", p3)
	}
}
