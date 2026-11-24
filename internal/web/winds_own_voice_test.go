package web

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gregal/internal/companion"
	"gregal/internal/config"
	"gregal/internal/jobs"
	"gregal/internal/runs"
)

// Quan un vent parla sol, escriu en l'idioma de la persona i curt, com una
// nota d'amic: mai un butlletí en markdown en una altra llengua.
func assertOwnVoice(t *testing.T, kind, lang, brief string) {
	t.Helper()
	other := "ca"
	if lang == "ca" {
		other = "en"
	}
	if !strings.HasSuffix(strings.TrimSpace(brief), config.LanguageDirective(lang)) {
		t.Errorf("%s/%s: the prompt does not end with the person's language:\n%s", kind, lang, brief)
	}
	if strings.Contains(brief, config.LanguageDirective(other)) {
		t.Errorf("%s/%s: the prompt also asks for %s", kind, lang, other)
	}
	for _, want := range []string{"one line with the news first", "No markdown headings"} {
		if !strings.Contains(brief, want) {
			t.Errorf("%s/%s: the prompt does not ask for %q:\n%s", kind, lang, want, brief)
		}
	}
	for _, bad := range []string{"well-structured markdown", "language the task above is written in"} {
		if strings.Contains(brief, bad) {
			t.Errorf("%s/%s: the prompt still asks for %q", kind, lang, bad)
		}
	}
}

func TestWindOwnTurnsAskForPersonLanguageAndShortMessage(t *testing.T) {
	for _, lang := range []string{"ca", "en"} {
		t.Run("heartbeat-"+lang, func(t *testing.T) {
			t.Setenv("GREGAL_DATA_DIR", t.TempDir())
			s, _ := companionTestServer(t)
			s.cfg.Language = lang
			postJSON(t, s.handleWinds, `{"name":"Xaloc","color":"#FF9F00","role":"futbol"}`)
			if _, err := s.jobs.Save(jobs.Job{ID: "wind-xaloc", Name: "Cor de Xaloc", Prompt: "vigila", Kind: "event", Enabled: true}); err != nil {
				t.Fatal(err)
			}
			briefs := stubHeartbeat(t, "(res)")
			if _, err := s.executeWindHeartbeat(context.Background(), jobs.Job{ID: "wind-xaloc", Name: "Cor de Xaloc"}, runs.Run{},
				companion.Wind{ID: "xaloc", Name: "Xaloc", Role: "futbol", Persona: "ets en Xaloc"}); err != nil {
				t.Fatal(err)
			}
			assertOwnVoice(t, "heartbeat", lang, (*briefs)[0])
		})
		for _, followup := range []bool{false, true} {
			kind := "reminder"
			if followup {
				kind = "followup"
			}
			t.Run(kind+"-"+lang, func(t *testing.T) {
				t.Setenv("GREGAL_DATA_DIR", t.TempDir())
				s := remindTestServer(t)
				s.cfg.Language = lang
				briefs := stubHeartbeat(t, "Final: 2-1.")
				j, err := saveWindReminderFull(s, "xaloc", "+30m", "Resultat", "Busca el resultat final del partit", 0, followup)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := s.executeWindRemind(context.Background(), j); err != nil {
					t.Fatal(err)
				}
				assertOwnVoice(t, kind, lang, (*briefs)[0])
			})
		}
		t.Run("routine-"+lang, func(t *testing.T) {
			t.Setenv("GREGAL_DATA_DIR", t.TempDir())
			s, _ := companionTestServer(t)
			s.cfg.Language = lang
			store, err := jobs.New(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			s.jobs = store
			wind, err := s.winds.Save(companion.Wind{Name: "Ponent", Persona: "Weather."})
			if err != nil {
				t.Fatal(err)
			}
			system := make(chan string, 4)
			provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				raw, _ := io.ReadAll(r.Body)
				var req struct {
					Messages []struct {
						Content string `json:"content"`
					} `json:"messages"`
				}
				if err := json.Unmarshal(raw, &req); err == nil && len(req.Messages) > 0 {
					system <- req.Messages[0].Content
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"Demà plourà."}}]}`))
			}))
			defer provider.Close()
			s.cfg.Providers["p"] = config.Provider{BaseURL: provider.URL + "/v1"}
			if _, err := s.executeJob(context.Background(), jobs.Job{ID: "routine", Name: "Weather", Prompt: "Daily weather for Barcelona", Wind: wind.ID}, runs.Run{}); err != nil {
				t.Fatal(err)
			}
			waitBackground()
			select {
			case brief := <-system:
				assertOwnVoice(t, "routine", lang, brief)
			default:
				t.Fatal("the routine did not call the model")
			}
		})
	}
}

// La previsualització d'una novetat (text de /api/winds/novetats i el títol
// de la notificació, que l'app hi llegeix) és la primera línia, sense
// marques de markdown.
func TestNovetatPreviewIsFirstPlainLine(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	s, _ := companionTestServer(t)
	w := companion.Wind{ID: "ponent", Name: "Ponent"}
	recordNovetat(s, w, "job", "# Barcelona — **Daily** Weather\n\n- Rain from 16:00\n- 14 °C")
	waitBackground()
	rec := httptest.NewRecorder()
	s.handleWindNovetats(rec, httptest.NewRequest(http.MethodGet, "/api/winds/novetats", nil))
	var resp struct {
		Novetats []windNovetat `json:"novetats"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil || len(resp.Novetats) != 1 {
		t.Fatalf("novetats: %v %s", err, rec.Body.String())
	}
	if got := resp.Novetats[0].Text; got != "Barcelona — Daily Weather" {
		t.Fatalf("preview = %q", got)
	}
}
