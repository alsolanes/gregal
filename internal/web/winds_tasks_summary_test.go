package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"gregal/internal/companion"
	"gregal/internal/llm"
	"gregal/internal/team"
)

// TestResumDeFeinesPerAlMobil: la llista lleugera diu l'estat (també si va
// quedar a mitges), els fitxers que són el resultat i la resposta final.
func TestResumDeFeinesPerAlMobil(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	s := goalTestServer(t)
	ws := t.TempDir()
	s.cwd = ws
	emit, err := s.recordWindTask("Fes una web", ws, "rot-9", &team.FlowSpec{Nodes: []team.FlowNode{{ID: "builder", Label: "Tramuntana"}}})
	if err != nil {
		t.Fatal(err)
	}
	emit(team.Event{Type: "tool_result", Agent: "builder", Path: "index.html"})
	emit(team.Event{Type: "tool_result", Agent: "builder", Path: "_harness.html"})
	emit(team.Event{Type: "tool_result", Agent: "builder", Path: "index.html"})

	llista := func() []windTaskSummary {
		rec := httptest.NewRecorder()
		s.handleWindTasks(rec, httptest.NewRequest("GET", "/api/winds/tasks?summary=1", nil))
		var out struct {
			Tasks []windTaskSummary `json:"tasks"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil || len(out.Tasks) != 1 {
			t.Fatalf("resposta: %s", rec.Body.String())
		}
		return out.Tasks
	}
	got := llista()[0]
	if got.Status != "running" || got.Thread != "rot-9" || len(got.Files) != 1 || got.Files[0] != "index.html" {
		t.Fatalf("en marxa: %+v", got)
	}

	// El servidor es reinicia a mitja feina: ja no és viva.
	liveWindTasks.Delete(got.ID)
	if st := llista()[0].Status; st != "interrupted" {
		t.Fatalf("una feina a mitges després d'un reinici: %q", st)
	}

	liveWindTasks.Store(got.ID, true)
	emit(team.Event{Type: "done", Output: "He fet index.html amb la portada."})
	got = llista()[0]
	if got.Status != "done" || got.Result == "" {
		t.Fatalf("acabada: %+v", got)
	}
}

// A turn owns mu until its model work and persistence finish. The inbox must
// still return the last durable summary, even on its first request while busy.
func TestWindsSummariesReadCommittedDuringOpenTurn(t *testing.T) {
	for _, kind := range []string{"wind", "rot"} {
		t.Run(kind, func(t *testing.T) {
			t.Setenv("GREGAL_DATA_DIR", t.TempDir())
			s, _ := companionTestServer(t)
			wind, err := s.winds.Save(companion.Wind{Name: "Tramuntana"})
			if err != nil {
				t.Fatal(err)
			}
			other, err := s.winds.Save(companion.Wind{Name: "Xaloc"})
			if err != nil {
				t.Fatal(err)
			}
			rot, err := s.saveRotllana(Rotllana{Name: "Grup", Members: []string{wind.ID, other.ID}})
			if err != nil {
				t.Fatal(err)
			}
			committed := []llm.Message{
				{Role: "user", Content: "Pregunta desada", Ts: 100},
				// La identitat viatja per SpeakerID/SpeakerName; el contingut no
				// porta prefix de nom (com desa el codi de rotllana).
				{Role: "assistant", Content: "Resposta desada", SpeakerID: wind.ID, SpeakerName: wind.Name, Ts: 101},
			}
			if err := saveWindHistory(s, wind, committed); err != nil {
				t.Fatal(err)
			}
			if err := saveWindHistory(s, other, []llm.Message{{Role: "assistant", Content: "Un altre vent", Ts: 102}}); err != nil {
				t.Fatal(err)
			}
			if err := saveRotllanaHistory(s, rot.ID, committed); err != nil {
				t.Fatal(err)
			}

			key := "wind:" + wind.ID
			save := func(history []llm.Message) error { return saveWindHistory(s, wind, history) }
			h := windHistory(s, wind.ID)
			mu, messages := &h.mu, &h.messages
			if kind == "rot" {
				key = "rot:" + rot.ID
				save = func(history []llm.Message) error { return saveRotllanaHistory(s, rot.ID, history) }
				h := rotllanaHistory(s, rot.ID)
				mu, messages = &h.mu, &h.messages
			}
			mu.Lock()
			defer mu.Unlock()
			// In-flight data must not leak, even if a caller has changed the
			// mutable buffer before attempting its durable save.
			*messages = append(append([]llm.Message(nil), committed...), llm.Message{Role: "assistant", Content: "NO DESAT", Ts: 999})
			items := readWindsSummariesPromptly(t, s)
			want := convoSummary{Key: key, From: "pet", Text: "Resposta desada", Ts: 101, Count: 2}
			if kind == "rot" {
				want.Speaker = wind.Name
			}
			if got := items[key]; got != want {
				t.Fatalf("busy summary = %+v, want last commit %+v", got, want)
			}
			if got := items["wind:"+other.ID]; got.Text != "Un altre vent" || got.Ts != 102 || got.Count != 1 {
				t.Fatalf("unrelated wind lost or stalled: %+v", got)
			}
			if _, ok := items["cap"]; !ok {
				t.Fatal("combined response lost the cap summary")
			}
			if mu.TryLock() {
				mu.Unlock()
				t.Fatal("summary reader released the turn serialization lock")
			}

			// A completed voice/save is visible immediately, even before the
			// enclosing SSE handler releases the turn lock (multi-voice groups).
			final := append(append([]llm.Message(nil), committed...), llm.Message{Role: "assistant", Content: "Resposta final", Ts: 103})
			if err := save(final); err != nil {
				t.Fatal(err)
			}
			*messages = final
			if got := readWindsSummariesPromptly(t, s)[key]; got.Text != "Resposta final" || got.Ts != 103 || got.Count != 3 {
				t.Fatalf("new commit not visible: %+v", got)
			}
		})
	}
}

func TestWindsSummariesDuringStream(t *testing.T) {
	for _, kind := range []string{"wind", "rot"} {
		t.Run(kind, func(t *testing.T) {
			t.Setenv("GREGAL_DATA_DIR", t.TempDir())
			s, _ := companionTestServer(t)
			wind, err := s.winds.Save(companion.Wind{Name: "Tramuntana"})
			if err != nil {
				t.Fatal(err)
			}
			committed := []llm.Message{{Role: "assistant", Content: "Resposta anterior", Ts: 100}}
			key := "wind:" + wind.ID
			handler := s.handleWindChatStream
			body := `{"wind":"` + wind.ID + `","message":"Pregunta nova"}`
			if kind == "rot" {
				rot, err := s.saveRotllana(Rotllana{Name: "Grup", Members: []string{wind.ID}})
				if err != nil {
					t.Fatal(err)
				}
				key = "rot:" + rot.ID
				handler = s.handleRotllanaChatStream
				body = `{"rotllana":"` + rot.ID + `","wind":"` + wind.ID + `","message":"Pregunta nova"}`
				if err := saveRotllanaHistory(s, rot.ID, committed); err != nil {
					t.Fatal(err)
				}
			} else if err := saveWindHistory(s, wind, committed); err != nil {
				t.Fatal(err)
			}

			started, release := make(chan struct{}), make(chan struct{})
			var releaseOnce sync.Once
			finish := func() { releaseOnce.Do(func() { close(release) }) }
			savedWind, savedRot := windLoopStream, rotllanaVoiceLoopStream
			loop := func(_ *Server, _ context.Context, _ string, _ func(string, string), _ ...*routedRole) (exploracio, error) {
				close(started)
				<-release
				return exploracio{text: "Resposta nova"}, nil
			}
			windLoopStream, rotllanaVoiceLoopStream = loop, loop
			done := make(chan *httptest.ResponseRecorder, 1)
			finished := make(chan struct{})
			go func() {
				defer close(finished)
				rec := httptest.NewRecorder()
				handler(rec, httptest.NewRequest(http.MethodPost, "/stream", strings.NewReader(body)))
				done <- rec
			}()
			t.Cleanup(func() {
				finish()
				select {
				case <-finished:
				case <-time.After(5 * time.Second):
					t.Error("stream did not stop during cleanup")
				}
				waitBackground()
				windLoopStream, rotllanaVoiceLoopStream = savedWind, savedRot
			})
			defer finish()
			select {
			case <-started:
			case <-time.After(5 * time.Second):
				t.Fatal("stream never reached its model turn")
			}
			if got := readWindsSummariesPromptly(t, s)[key]; got.Text != "Resposta anterior" || got.Ts != 100 || got.Count != 1 {
				t.Fatalf("stream hid or replaced its previous committed summary: %+v", got)
			}
			finish()
			select {
			case rec := <-done:
				if !strings.Contains(rec.Body.String(), "event: done") || strings.Contains(rec.Body.String(), "event: error") {
					t.Fatalf("stream did not commit: %s", rec.Body.String())
				}
			case <-time.After(5 * time.Second):
				t.Fatal("stream did not finish")
			}
			if got := readWindsSummariesPromptly(t, s)[key]; !strings.Contains(got.Text, "Resposta nova") || got.Ts <= 100 || got.Count != 3 {
				t.Fatalf("stream completion did not refresh the committed summary: %+v", got)
			}
		})
	}
}

func readWindsSummariesPromptly(t *testing.T, s *Server) map[string]convoSummary {
	t.Helper()
	done := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		rec := httptest.NewRecorder()
		s.handleWindsSummaries(rec, httptest.NewRequest("GET", "/api/winds/summaries", nil))
		done <- rec
	}()
	select {
	case rec := <-done:
		var out struct {
			Items []convoSummary `json:"items"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil || rec.Code != 200 {
			t.Fatalf("summaries: %d %s (%v)", rec.Code, rec.Body.String(), err)
		}
		items := make(map[string]convoSummary, len(out.Items))
		for _, item := range out.Items {
			if _, exists := items[item.Key]; exists {
				t.Fatalf("duplicate summary: %s", item.Key)
			}
			items[item.Key] = item
		}
		return items
	case <-time.After(time.Second):
		// The caller releases its held turn lock while unwinding. Drain the
		// request before its temporary environment is restored on a red run.
		t.Cleanup(func() {
			select {
			case <-done:
			case <-time.After(5 * time.Second):
				t.Error("blocked summary request did not finish during cleanup")
			}
		})
		t.Fatal("combined summaries blocked on an open turn instead of reading the last commit")
		return nil
	}
}
