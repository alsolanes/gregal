package web

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"gregal/internal/config"
	"gregal/internal/llm"
)

// «Pregunta-li» convida l'altre vent al mateix fil: respon a sota, amb el
// context de la conversa, i el vent amfitrió el llegeix com una aportació
// d'un altre vent. Abans s'obria un fil nou i l'anterior desapareixia.
func TestWindInviteAnswersInTheSameThread(t *testing.T) {
	s, _ := companionTestServer(t)
	makeRotllanaTrio(t, s)
	var mu sync.Mutex
	var bodies []string
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		mu.Lock()
		bodies = append(bodies, string(raw))
		mu.Unlock()
		var req struct {
			Stream bool `json:"stream"`
		}
		_ = json.Unmarshal(raw, &req)
		if req.Stream {
			w.Header().Set("Content-Type", "text/event-stream")
			fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"Amb el peix, millor un blanc sec.\"},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n")
			return
		}
		fmt.Fprint(w, `{"choices":[{"message":{"content":"Amb el peix, millor un blanc sec."}}]}`)
	}))
	defer provider.Close()
	s.cfg.Providers["p"] = config.Provider{BaseURL: provider.URL + "/v1"}

	host, _ := s.winds.ByID("tramuntana")
	h := windHistory(s, host.ID)
	before := []llm.Message{{Role: "user", Content: "Quin vi porto per al sopar de peix?"}, {Role: "assistant", Content: "Un negre lleuger, el Montsant."}}
	h.messages = before
	if err := saveWindHistoryTo(s, host, h.name, before); err != nil {
		t.Fatal(err)
	}

	rec := postJSON(t, s.handleWindInvite, `{"wind":"tramuntana","guest":"llevant"}`)
	if rec.Code != 200 {
		t.Fatalf("invite: %d %s", rec.Code, rec.Body.String())
	}
	var out struct {
		History []map[string]any `json:"history"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	if len(out.History) != 3 || out.History[1]["text"] != "Un negre lleuger, el Montsant." {
		t.Fatalf("the previous answer must stay in the thread: %+v", out.History)
	}
	last := out.History[2]
	if last["kind"] != guestKind || last["speaker_id"] != "llevant" || last["speaker_name"] != "Llevant" || !strings.Contains(last["text"].(string), "blanc sec") {
		t.Fatalf("guest row = %+v", last)
	}
	mu.Lock()
	sent := strings.Join(bodies, "\n")
	mu.Unlock()
	if !strings.Contains(sent, "Montsant") || !strings.Contains(sent, "sopar de peix") {
		t.Fatal("the guest did not get the conversation as context")
	}
	// L'amfitrió ho llegeix com a aportació d'un altre vent, no com a pròpia.
	turns := windChatTurns(h.messages)
	if got := turns[len(turns)-1].Content; !strings.Contains(got, "Llevant, another wind the user invited") {
		t.Fatalf("host sees the guest as itself: %q", got)
	}

	if rec := postJSON(t, s.handleWindInvite, `{"wind":"tramuntana","guest":"tramuntana"}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("a wind cannot invite itself: %d", rec.Code)
	}
}
