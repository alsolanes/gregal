package web

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"gregal/internal/companion"
	"gregal/internal/llm"
	"gregal/internal/session"
	"gregal/internal/skills"
)

func TestWindTransferExportCommittedVisibleConversation(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	s, _ := companionTestServer(t)
	wind, err := s.winds.Save(companion.Wind{Name: "Breeze", Persona: "Friendly", Color: "#112233", Style: "pso"})
	if err != nil {
		t.Fatal(err)
	}
	if err := saveWindHistory(s, wind, []llm.Message{{Role: "user", Content: "Hello", Ts: 100}, {Role: "assistant", Content: "Hello back", Ts: 101}, {Role: "system", Content: "PRIVATE SYSTEM"}, {Role: "tool", Content: "PRIVATE TOOL"}}); err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	s.Hub().mux().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/winds/transfer?kind=wind&id="+wind.ID, nil))
	var got struct {
		Version int    `json:"version"`
		Kind    string `json:"kind"`
		ID      string `json:"id"`
		History []struct {
			Text string `json:"text"`
			Ts   int64  `json:"ts"`
		} `json:"history"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("transfer must be registered JSON API: status=%d type=%s", rec.Code, rec.Header().Get("Content-Type"))
	}
	if rec.Code != 200 || got.Version != 1 || got.Kind != "wind" || got.ID != wind.ID || len(got.History) != 2 || got.History[1].Text != "Hello back" || got.History[1].Ts != 101 {
		t.Fatalf("wrong visible snapshot: %+v (%s)", got, rec.Body.String())
	}
}

func transferTestSnapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(root, func(path string, ent os.DirEntry, err error) error {
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			return err
		}
		if ent.IsDir() {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		out[rel] = string(raw)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}
func transferTestPost(t *testing.T, s *Server, id string, b transferBundle) *httptest.ResponseRecorder {
	t.Helper()
	raw, err := json.Marshal(map[string]any{"transferId": id, "bundle": b})
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	s.handleWindsTransfer(rec, httptest.NewRequest(http.MethodPost, "/api/winds/transfer", bytes.NewReader(raw)))
	return rec
}

func TestWindTransferCopyNamesakeRetryAndSourceUnchanged(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	hub := testHubWithUsers(t)
	source := hub.SessionFor(&User{Name: "origin"}, "one")
	dest := hub.SessionFor(&User{Name: "destination"}, "one")
	w, err := source.winds.Save(companion.Wind{Name: "Breeze", Color: "#112233", Persona: "Friendly", Style: "pso"})
	if err != nil {
		t.Fatal(err)
	}
	if err = companion.ReplaceWindMemory(source.windStateDir(), w.ID, "- Likes tea.\n"); err != nil {
		t.Fatal(err)
	}
	if err = companion.ReplaceWindBrain(source.windStateDir(), w.ID, "Known fact."); err != nil {
		t.Fatal(err)
	}
	if err = source.saveWindSkill(w.ID, skills.Skill{Nom: "guide", Titol: "Guide", Descripcio: "Reference", Cos: "Read carefully."}); err != nil {
		t.Fatal(err)
	}
	original := []llm.Message{{Role: "user", Content: "Hello", Ts: 100}, {Role: "assistant", Content: "Hello back", Ts: 101}}
	if err = saveWindHistory(source, w, original); err != nil {
		t.Fatal(err)
	}
	namesake, err := dest.winds.Save(companion.Wind{Name: "Breeze", Persona: "Destination persona", Color: "#abcdef"})
	if err != nil {
		t.Fatal(err)
	}
	before := transferTestSnapshot(t, source.windStateDir())
	historyBefore := transferTestSnapshot(t, source.sessDir())
	b, err := source.exportTransfer("wind", w.ID, transferConsent{Memory: true, Knowledge: true, Skills: true})
	if err != nil {
		t.Fatal(err)
	}
	rec := transferTestPost(t, dest, "copy-request-01", b)
	if rec.Code != 200 {
		t.Fatalf("import: %d %s", rec.Code, rec.Body.String())
	}
	var response struct {
		ID         string `json:"id"`
		Kind       string `json:"kind"`
		Replayed   bool   `json:"replayed"`
		VerifyPath string `json:"verifyPath"`
		Counts     struct {
			Winds    int `json:"winds"`
			Messages int `json:"messages"`
			Skills   int `json:"skills"`
		} `json:"counts"`
	}
	if err = json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.ID == w.ID || response.Kind != "wind" || response.Counts.Winds != 1 || response.Counts.Messages != 2 || response.Counts.Skills != 1 || response.Replayed {
		t.Fatalf("bad receipt %s", rec.Body.String())
	}
	clone, ok := dest.winds.ByID(response.ID)
	if !ok || clone.Name != w.Name || clone.Persona != w.Persona {
		t.Fatalf("missing fresh clone: %+v", clone)
	}
	if got, _ := dest.winds.ByID(namesake.ID); got != namesake {
		t.Fatal("namesake overwritten")
	}
	if got := companion.LoadWindMemory(dest.windStateDir(), clone.ID); got != "- Likes tea.\n" {
		t.Fatalf("memory: %q", got)
	}
	get := httptest.NewRecorder()
	dest.handleWindChat(get, httptest.NewRequest(http.MethodGet, response.VerifyPath, nil))
	if get.Code != 200 || !strings.Contains(get.Body.String(), `"ts":101`) {
		t.Fatalf("readback %d %s", get.Code, get.Body.String())
	}
	// A fresh session handle must use the durable receipt, not an in-memory cache.
	resumed := hub.SessionFor(&User{Name: "destination"}, "two")
	rec = transferTestPost(t, resumed, "copy-request-01", b)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"replayed":true`) || !strings.Contains(rec.Body.String(), response.ID) {
		t.Fatalf("retry: %d %s", rec.Code, rec.Body.String())
	}
	list, _ := dest.winds.List()
	if len(list) != 2 {
		t.Fatalf("duplicate clone: %d", len(list))
	}
	saved, err := session.Load(dest.sessDir(), "company-"+clone.ID)
	if err != nil || !reflect.DeepEqual(saved.Convo, original) {
		t.Fatalf("history changed %+v %v", saved.Convo, err)
	}
	if !reflect.DeepEqual(before, transferTestSnapshot(t, source.windStateDir())) || !reflect.DeepEqual(historyBefore, transferTestSnapshot(t, source.sessDir())) {
		t.Fatal("source changed")
	}
	b.History[0].Text = "changed"
	rec = transferTestPost(t, dest, "copy-request-01", b)
	if rec.Code != 409 {
		t.Fatalf("changed retry accepted: %d", rec.Code)
	}
}
