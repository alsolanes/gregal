package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gregal/internal/companion"
	"gregal/internal/config"
	"gregal/internal/jobs"
	"gregal/internal/llm"
	"gregal/internal/runs"
)

func TestCompanionMemoryAuthenticatedScope(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	t.Setenv("GREGAL_MEMORY", t.TempDir()+"/global.md")
	if err := config.Remember("GLOBAL-PRIVATE"); err != nil {
		t.Fatal(err)
	}
	hub := testHubWithUsers(t)
	anna := hub.SessionFor(&User{Name: "anna"}, "a")
	bert := hub.SessionFor(&User{Name: "bert"}, "a")
	fact := "ANNA-PRIVATE e-bike"
	companionSuggests.Store(anna, companionSuggestion{State: "ready", Facts: []string{fact}})
	rec := postJSON(t, anna.handleCompanionMemory, `{"action":"accept","fact":"`+fact+`"}`)
	if rec.Code != 200 {
		t.Fatalf("accept: %d %s", rec.Code, rec.Body.String())
	}
	get := func(s *Server) string {
		out := httptest.NewRecorder()
		s.handleCompanionMemory(out, httptest.NewRequest(http.MethodGet, "/api/companion/memory", nil))
		return out.Body.String()
	}
	if got := get(anna); !strings.Contains(got, fact) || strings.Contains(got, "GLOBAL-PRIVATE") {
		t.Errorf("anna scope: %s", got)
	}
	if got := get(bert); strings.Contains(got, "PRIVATE") {
		t.Errorf("bert sees foreign memory: %s", got)
	}
	if got := config.LoadUserMemory(); strings.Contains(got, fact) {
		t.Errorf("accept mutated global memory: %s", got)
	}
	anna2 := hub.SessionFor(&User{Name: "anna"}, "b")
	if !strings.Contains(get(anna2), fact) {
		t.Error("same user new session lost memory")
	}
	if rec := postJSON(t, bert.handleCompanionMemory, `{"action":"forget","term":"e-bike"}`); rec.Code != 200 {
		t.Fatal(rec.Body.String())
	}
	if !strings.Contains(get(anna2), fact) {
		t.Error("bert forgot anna's fact")
	}
	if rec := postJSON(t, anna2.handleCompanionMemory, `{"action":"forget","term":"e-bike"}`); rec.Code != 200 {
		t.Fatal(rec.Body.String())
	}
	if strings.Contains(get(anna), fact) {
		t.Error("forget not scoped to same user")
	}
	for _, s := range []*Server{hub.SessionFor(nil, "local"), hub.SessionFor(&User{Name: "local", Tothom: true}, "mobile")} {
		if !strings.Contains(get(s), "GLOBAL-PRIVATE") {
			t.Error("legacy global memory lost")
		}
	}
}
func TestCompanionPromptsUseAuthenticatedMemory(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	t.Setenv("GREGAL_MEMORY", t.TempDir()+"/global.md")
	if err := config.Remember("GLOBAL-PRIVATE"); err != nil {
		t.Fatal(err)
	}
	hub := testHubWithUsers(t)
	s := hub.SessionFor(&User{Name: "anna"}, "a")
	if err := s.rememberPersonal("ANNA-PRIVATE"); err != nil {
		t.Fatal(err)
	}
	wind, err := s.winds.Save(companion.Wind{Name: "Xaloc"})
	if err != nil {
		t.Fatal(err)
	}
	rot, err := s.saveRotllana(Rotllana{Name: "Vesprada", Members: []string{wind.ID}})
	if err != nil {
		t.Fatal(err)
	}
	check := func(label, prompt string) {
		t.Helper()
		if !strings.Contains(prompt, "ANNA-PRIVATE") || strings.Contains(prompt, "GLOBAL-PRIVATE") {
			t.Errorf("%s leaked/missed memory: %s", label, prompt)
		}
	}
	oldWind, oldPet, oldCheck, oldPetCheck, oldHeart, oldLearn := windLoop, petChatter, companionCheckiner, petCheckiner, windHeartbeatLoop, windLearnLoop
	t.Cleanup(func() { waitBackground()
		windLoop = oldWind
		petChatter = oldPet
		companionCheckiner = oldCheck
		petCheckiner = oldPetCheck
		windHeartbeatLoop = oldHeart
		windLearnLoop = oldLearn
	})
	learned := make(chan struct{})
	windLearnLoop = func(_ *Server, _ context.Context, _ *llm.Client, _ *config.Config, _ config.Provider, _ config.Role, _ []llm.Message) (string, error) {
		close(learned)
		return "", nil
	}
	windLoop = func(_ *Server, _ context.Context, b string) (exploracio, error) { check("wind", b); return exploracio{text: "ok"}, nil }
	petChatter = func(_ context.Context, _ *llm.Client, _ *config.Config, _ config.Provider, _ config.Role, m []llm.Message) (string, error) {
		check("group/pet", m[0].Content)
		return "ok", nil
	}
	companionCheckiner = func(_ context.Context, _ *llm.Client, _ *config.Config, _ config.Provider, _ config.Role, m, d string) (string, error) {
		check("checkin", m)
		return "ok", nil
	}
	petCheckiner = func(_ context.Context, _ *llm.Client, _ *config.Config, _ config.Provider, _ config.Role, sys, u string) (string, error) {
		check("pet checkin", u)
		return "ok", nil
	}
	windHeartbeatLoop = func(_ *Server, _ context.Context, b string) (exploracio, error) {
		check("heartbeat", b)
		return exploracio{text: "(res)"}, nil
	}
	for _, tc := range []struct {
		handler http.HandlerFunc
		body    string
	}{
		{s.handleWindChat, `{"wind":"xaloc","message":"hola"}`},
		{s.handleRotllanaChat, `{"rotllana":"` + rot.ID + `","message":"hola"}`},
		{s.handleCompanionPetChat, `{"pet":"mel","message":"hola"}`},
		{s.handleCompanionCheckin, `{}`},
		{s.handleCompanionCheckinPet, `{"pet":"mel"}`},
	} {
		if rec := postJSON(t, tc.handler, tc.body); rec.Code != 200 {
			t.Fatalf("prompt route: %d %s", rec.Code, rec.Body.String())
		}
	}
	<-learned // the asynchronous seam must finish before cleanup restores it
	if _, err := s.executeWindHeartbeat(context.Background(), jobs.Job{ID: "test"}, runs.Run{}, wind); err != nil {
		t.Fatal(err)
	}
}

func TestWindProfileAndKnowledgeAuthenticatedAPI(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	t.Setenv("GREGAL_MEMORY", t.TempDir()+"/global.md")
	hub := testHubWithUsers(t)
	anna := hub.SessionFor(&User{Name: "anna"}, "default")
	bert := hub.SessionFor(&User{Name: "bert"}, "default")
	for _, s := range []*Server{anna, bert} {
		if _, err := s.winds.Save(companion.Wind{Name: "Xaloc", Persona: s.user.Name + " persona"}); err != nil {
			t.Fatal(err)
		}
	}
	if err := anna.rememberPersonal("ANNA-PERSONAL"); err != nil {
		t.Fatal(err)
	}
	var who *User = &User{Name: "anna"}
	handler := hub.def.HandlerWithIdentity(func(_ *http.Request) (*User, error) { return who, nil })
	call := func(method, path, body string) *httptest.ResponseRecorder {
		t.Helper()
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(method, path, strings.NewReader(body)))
		return rec
	}
	profile := call(http.MethodGet, "/api/winds/profile?wind=xaloc", "")
	if profile.Code != 200 {
		t.Fatalf("profile route: %d %s", profile.Code, profile.Body.String())
	}
	var got struct {
		Wind      companion.Wind `json:"wind"`
		Knowledge string         `json:"knowledge"`
		Personal  string         `json:"personal_memory"`
	}
	if err := json.Unmarshal(profile.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Wind.Persona != "anna persona" || !strings.Contains(got.Personal, "ANNA-PERSONAL") || got.Knowledge != "" {
		t.Fatalf("profile: %+v", got)
	}
	markdown := "  # Notes\n\n- coneixement meu\n"
	data, _ := json.Marshal(map[string]string{"wind": "xaloc", "text": markdown})
	edited := call(http.MethodPost, "/api/winds/memory", string(data))
	if edited.Code != 200 {
		t.Fatalf("memory route: %d %s", edited.Code, edited.Body.String())
	}
	if err := json.Unmarshal(edited.Body.Bytes(), &got); err != nil || got.Knowledge != markdown || got.Wind.Persona != "anna persona" {
		t.Fatalf("replace response: %+v %v", got, err)
	}
	if raw, err := os.ReadFile(filepath.Join(companion.BrainDir(anna.windStateDir()), "xaloc.md")); err != nil || string(raw) != markdown {
		t.Fatalf("durable markdown: %q %v", raw, err)
	}
	who = &User{Name: "bert"}
	other := call(http.MethodGet, "/api/winds/profile?wind=xaloc", "")
	if strings.Contains(other.Body.String(), "ANNA-PERSONAL") || strings.Contains(other.Body.String(), "coneixement meu") {
		t.Fatalf("foreign profile: %s", other.Body.String())
	}
	if res := call(http.MethodPost, "/api/winds/memory", `{"wind":"xaloc","text":"bert notes"}`); res.Code != 200 {
		t.Fatal(res.Body.String())
	}
	who = &User{Name: "anna"}
	for _, body := range []string{`{"wind":"../bert/xaloc","text":"oops"}`, `{"wind":"xaloc"}`, `{"wind":"xaloc","text":null}`, `{"wind":"xaloc","text":3}`, `{"wind":"xaloc","text":"a"} {}`, `{"wind":"xaloc","text":"a","extra":true}`, `{"wind":"xaloc","text":"` + strings.Repeat("x", 24*1024+1) + `"}`} {
		if res := call(http.MethodPost, "/api/winds/memory", body); res.Code != 400 {
			t.Errorf("invalid payload accepted: %d %.100s", res.Code, body)
		}
	}
	if res := call(http.MethodGet, "/api/winds/profile?wind=missing", ""); res.Code != 400 {
		t.Errorf("unknown profile: %d", res.Code)
	}
	if res := call(http.MethodPost, "/api/winds/profile", `{}`); res.Code != 405 {
		t.Errorf("profile method: %d", res.Code)
	}
	if res := call(http.MethodGet, "/api/winds/memory", ""); res.Code != 405 {
		t.Errorf("memory method: %d", res.Code)
	}
	if got := call(http.MethodGet, "/api/winds/profile?wind=xaloc", "").Body.String(); !strings.Contains(got, "coneixement meu") {
		t.Errorf("validation mutated knowledge: %s", got)
	}
	cleared := call(http.MethodPost, "/api/winds/memory", `{"wind":"xaloc","text":""}`)
	if cleared.Code != 200 {
		t.Fatal(cleared.Body.String())
	}
	if err := json.Unmarshal(cleared.Body.Bytes(), &got); err != nil || got.Knowledge != "" {
		t.Fatalf("empty replace: %+v %v", got, err)
	}
	who = &User{Name: "bert"}
	if got := call(http.MethodGet, "/api/winds/profile?wind=xaloc", "").Body.String(); !strings.Contains(got, "bert notes") {
		t.Errorf("clearing anna migrated bert: %s", got)
	}
	who = nil
	if res := call(http.MethodGet, "/api/winds/profile?wind=xaloc", ""); res.Code != 401 {
		t.Errorf("unauthenticated profile: %d", res.Code)
	}
}

func TestNamedSingletonRotllanesAreIndependentAndEditable(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	s, _ := companionTestServer(t)
	if _, err := s.winds.Save(companion.Wind{Name: "Xaloc"}); err != nil {
		t.Fatal(err)
	}
	first, err := s.saveRotllana(Rotllana{Name: "Vesprada", Members: []string{"xaloc"}})
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.saveRotllana(Rotllana{Name: "Vesprada", Members: []string{"xaloc"}})
	if err != nil {
		t.Fatal(err)
	}
	if first.ID == second.ID {
		t.Fatalf("new named chats collided: %s", first.ID)
	}
	if err := saveRotllanaHistory(s, first.ID, []llm.Message{{Role: "user", Content: "FIRST-PRIVATE"}}); err != nil {
		t.Fatal(err)
	}
	edited, err := s.saveRotllana(Rotllana{ID: first.ID, Name: "Nou nom", Members: []string{"xaloc"}})
	if err != nil {
		t.Fatal(err)
	}
	if edited.ID != first.ID {
		t.Fatalf("edit changed ID: %+v", edited)
	}
	list, err := s.listRotllanes()
	if err != nil || len(list) != 2 {
		t.Fatalf("catalog: %+v %v", list, err)
	}
	if _, err := s.saveRotllana(Rotllana{ID: "unknown", Name: "New", Members: []string{"xaloc"}}); err == nil {
		t.Fatal("unknown explicit group ID accepted")
	}
	for _, tc := range []struct {
		id       string
		hasFirst bool
	}{{first.ID, true}, {second.ID, false}} {
		out := httptest.NewRecorder()
		s.handleRotllanaChat(out, httptest.NewRequest(http.MethodGet, "/api/winds/rotllana?rotlla="+tc.id, nil))
		if out.Code != 200 || strings.Contains(out.Body.String(), "FIRST-PRIVATE") != tc.hasFirst {
			t.Fatalf("independent history %s: %d %s", tc.id, out.Code, out.Body.String())
		}
	}
	if len(windHistory(s, "xaloc").messages) != 0 {
		t.Fatal("singleton migrated direct individual history")
	}
	if err := s.deleteRotllana(first.ID); err != nil {
		t.Fatal(err)
	}
	recreated, err := s.saveRotllana(Rotllana{Name: "Vesprada", Members: []string{"xaloc"}})
	if err != nil {
		t.Fatal(err)
	}
	if recreated.ID == first.ID {
		t.Fatal("recreated chat reused deleted thread's durable identity/history")
	}
}

func TestRotllanaHistorySpeakerIdentitySurvivesReloadAndRename(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	s, _ := companionTestServer(t)
	wind, err := s.winds.Save(companion.Wind{Name: "Xaloc"})
	if err != nil {
		t.Fatal(err)
	}
	// Load a preexisting slug-id catalog without migrating its ID or chat.
	if err := os.WriteFile(s.rotllanes.path(), []byte(`{"rotllanes":[{"id":"vesprada","name":"Vesprada","members":["xaloc"]}]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	rot, ok := s.rotllanaByID("vesprada")
	if !ok {
		t.Fatal("legacy slug group lost")
	}
	old := petChatter
	petChatter = func(_ context.Context, _ *llm.Client, _ *config.Config, _ config.Provider, _ config.Role, _ []llm.Message) (string, error) {
		return "resposta", nil
	}
	t.Cleanup(func() { waitBackground(); petChatter = old })
	// Existing chats have only the textual name prefix; keep them readable.
	if err := saveRotllanaHistory(s, rot.ID, []llm.Message{{Role: "assistant", Content: "Xaloc: antic"}}); err != nil {
		t.Fatal(err)
	}
	rec := postJSON(t, s.handleRotllanaChat, `{"rotllana":"`+rot.ID+`","message":"hola"}`)
	if rec.Code != 200 {
		t.Fatalf("chat: %d %s", rec.Code, rec.Body.String())
	}
	assert := func(raw []byte) {
		t.Helper()
		var got struct {
			History []map[string]string `json:"history"`
		}
		if err := json.Unmarshal(raw, &got); err != nil {
			t.Fatal(err)
		}
		if len(got.History) != 3 {
			t.Fatalf("history length: %+v", got)
		}
		last := got.History[2]
		if last["from"] != "pet" || last["text"] != "Xaloc: resposta" || last["speaker_id"] != wind.ID || last["speaker_name"] != "Xaloc" {
			t.Fatalf("speaker identity: %+v", last)
		}
		if got.History[1]["from"] != "user" || got.History[1]["text"] != "hola" {
			t.Fatalf("user compatibility: %+v", got.History[1])
		}
	}
	assert(rec.Body.Bytes())
	var first struct {
		History []map[string]string `json:"history"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &first)
	if first.History[0]["speaker_id"] != wind.ID || first.History[0]["speaker_name"] != "Xaloc" {
		t.Fatalf("legacy history unresolved: %+v", first.History[0])
	}
	if _, err := s.winds.Save(companion.Wind{ID: wind.ID, Name: "Nom nou"}); err != nil {
		t.Fatal(err)
	}
	rotllanaHistories.Delete(s)
	out := httptest.NewRecorder()
	s.handleRotllanaChat(out, httptest.NewRequest(http.MethodGet, "/api/winds/rotllana?rotlla="+rot.ID, nil))
	if out.Code != 200 {
		t.Fatal(out.Body.String())
	}
	assert(out.Body.Bytes())
}

func TestWindChatSendsBoundedPreviousTurns(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	s, _ := companionTestServer(t)
	s.cfg.Roles = map[string]config.Role{} // no asynchronous knowledge extraction
	wind, err := s.winds.Save(companion.Wind{Name: "Xaloc"})
	if err != nil {
		t.Fatal(err)
	}
	hist := windHistory(s, wind.ID)
	hist.messages = []llm.Message{{Role: "user", Content: "OLD-SECRET-TURN"}}
	for i := 0; i < 44; i++ {
		hist.messages = append(hist.messages, llm.Message{Role: "assistant", Content: strings.Repeat("far away ", 900)})
	}
	hist.messages = append(hist.messages, llm.Message{Role: "user", Content: "Em dic Joana"}, llm.Message{Role: "assistant", Content: "Hola Joana, recordo el teu nom"})
	if err := saveWindHistory(s, wind, hist.messages); err != nil {
		t.Fatal(err)
	}
	windHistories.Delete(s) // force the real persisted-history path
	var brief string
	prev := windLoop
	windLoop = func(_ *Server, _ context.Context, b string) (exploracio, error) { brief = b; return exploracio{text: "Joana"}, nil }
	t.Cleanup(func() { waitBackground(); windLoop = prev })
	rec := postJSON(t, s.handleWindChat, `{"wind":"xaloc","message":"Com em dic?"}`)
	if rec.Code != 200 {
		t.Fatalf("chat: %d %s", rec.Code, rec.Body.String())
	}
	for _, want := range []string{"USER: Em dic Joana", "ASSISTANT: Hola Joana", "Com em dic?"} {
		if !strings.Contains(brief, want) {
			t.Errorf("model missing previous context %q", want)
		}
	}
	if strings.Contains(brief, "OLD-SECRET-TURN") || len([]rune(brief)) > 20000 {
		t.Fatalf("history not bounded: %d chars", len([]rune(brief)))
	}
	req := httptest.NewRequest(http.MethodGet, "/api/winds/chat?wind=xaloc", nil)
	out := httptest.NewRecorder()
	s.handleWindChat(out, req)
	if !strings.Contains(out.Body.String(), "OLD-SECRET-TURN") {
		t.Fatal("prompt bound must not trim durable chat")
	}
}
