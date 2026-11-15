package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"gregal/internal/llm"
)

func localTryGet(t *testing.T, s *Server, id string) localTry {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		rec := httptest.NewRecorder()
		s.handleWindsLocalTry(rec, httptest.NewRequest(http.MethodGet, "/api/winds/local/try?id="+id, nil))
		if rec.Code != 200 {
			t.Fatalf("try status: %d %s", rec.Code, rec.Body.String())
		}
		var out localTry
		_ = json.Unmarshal(rec.Body.Bytes(), &out)
		if out.State != "running" || time.Now().After(deadline) {
			return out
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// «Prova-ho ara»: amb un lloc qualsevol del formulari, sense activar res, es
// veu allà mateix què ha trobat (amb fonts), què ha descartat i què ha
// buscat. No desa res: ni el lloc, ni «Avui», ni el que ja t'ha dit.
func TestLocalTryShowsWhatItFoundWithoutSavingAnything(t *testing.T) {
	s := localTestServer(t)
	old := localInfoLoop
	var brief string
	localInfoLoop = func(_ *Server, _ context.Context, b string) (exploracio, error) {
		brief = b
		call := llm.ToolCall{}
		call.Function.Name, call.Function.Arguments = "web_search", `{"query":"festa major Gràcia 2026 programa"}`
		return exploracio{text: localReply, hist: []llm.Message{{Role: "assistant", ToolCalls: []llm.ToolCall{call, call}}}}, nil
	}
	t.Cleanup(func() { localInfoLoop = old })
	cardsBefore := len(loadAvui(s))

	rec := localPost(t, s, `{"action":"try","place":{"label":"Gràcia, Barcelona"},"topics":["concerts","festes","news"]}`)
	if rec.Code != 200 {
		t.Fatalf("try: %d %s", rec.Code, rec.Body.String())
	}
	var started struct{ Try string }
	_ = json.Unmarshal(rec.Body.Bytes(), &started)
	got := localTryGet(t, s, started.Try)
	if got.State != "done" || got.Place != "Gràcia, Barcelona" {
		t.Fatalf("try = %+v", got)
	}
	// Concert i notícia; la llibreria (tema no demanat) i la línia sense font, fora.
	if len(got.Items) != 2 || got.Items[0].Host != "ajuntament.barcelona.cat" || got.Dropped != 1 {
		t.Fatalf("items = %+v dropped = %d", got.Items, got.Dropped)
	}
	if len(got.Searches) != 1 || !strings.Contains(got.Searches[0], "festa major") {
		t.Fatalf("searches = %v", got.Searches)
	}
	if !strings.Contains(brief, "FESTA MAJOR") || !strings.Contains(brief, "45 days") {
		t.Fatal("with festes on, the look must ask when the next festa major is")
	}
	if len(loadAvui(s)) != cardsBefore {
		t.Fatal("a try must not leave cards in Today")
	}
	if raw, err := os.ReadFile(localPath(s)); err == nil && strings.Contains(string(raw), "Gràcia") {
		t.Fatalf("a try must not save the place: %s", raw)
	}

	// Sense lloc no es pot provar; sense festes, la consigna no en parla.
	if rec := localPost(t, s, `{"action":"try","topics":["news"]}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("try without a place: %d", rec.Code)
	}
	st := defaultLocalSettings()
	st.Place.Label, st.Topics = "Vic", []string{"news"}
	w, _ := s.localWind("")
	if strings.Contains(s.localInfoBrief(w, st, time.Now()), "FESTA MAJOR") {
		t.Fatal("festa major asked without the festes topic")
	}
}

// El que ja t'havia dit també surt a la prova, marcat; si no, la segona prova
// quedaria buida. I «(res)» es diu clarament.
func TestLocalTryMarksRepeatedAndSaysWhenNothing(t *testing.T) {
	s := localTestServer(t)
	stubLocalLoop(t, localReply, "(res)")
	localMu.Lock()
	st := s.localSettings()
	st.Seen = []string{"Concert de Txarango a la Plaça del Sol, dissabte 22 h"}
	_ = s.saveLocalSettings(st)
	localMu.Unlock()

	var started struct{ Try string }
	_ = json.Unmarshal(localPost(t, s, `{"action":"try","place":{"label":"Gràcia"},"topics":["concerts","news"]}`).Body.Bytes(), &started)
	got := localTryGet(t, s, started.Try)
	if len(got.Items) != 2 || !got.Items[0].Repeated || got.Items[1].Repeated {
		t.Fatalf("items = %+v", got.Items)
	}
	_ = json.Unmarshal(localPost(t, s, `{"action":"try","place":{"label":"Gràcia"},"topics":["concerts"]}`).Body.Bytes(), &started)
	if got := localTryGet(t, s, started.Try); got.State != "done" || !got.Nothing || len(got.Items) != 0 {
		t.Fatalf("nothing found = %+v", got)
	}
}

// Un segon toc mentre la primera prova va (o el mòbil ha perdut el fil i
// ho torna a provar) no és un error: torna la que ja va, i s'hi enganxa.
func TestLocalTryTwiceFollowsTheRunningOne(t *testing.T) {
	s := localTestServer(t)
	old := localInfoLoop
	release := make(chan struct{})
	localInfoLoop = func(_ *Server, _ context.Context, _ string) (exploracio, error) {
		<-release
		return exploracio{text: localReply}, nil
	}
	t.Cleanup(func() { localInfoLoop = old })

	body := `{"action":"try","place":{"label":"Gràcia"},"topics":["concerts","news"]}`
	var first, second struct{ Try string }
	_ = json.Unmarshal(localPost(t, s, body).Body.Bytes(), &first)
	rec := localPost(t, s, body)
	if rec.Code != 200 {
		t.Fatalf("second tap: %d %s", rec.Code, rec.Body.String())
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &second)
	if first.Try == "" || second.Try != first.Try {
		t.Fatalf("second tap started another try: %q vs %q", first.Try, second.Try)
	}
	close(release)
	if got := localTryGet(t, s, second.Try); got.State != "done" || len(got.Items) != 2 {
		t.Fatalf("followed try = %+v", got)
	}
}
