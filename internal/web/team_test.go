package web

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"gregal/internal/agent"
	"gregal/internal/config"
)

func TestTeamAPIRealHandoffs(t *testing.T) {
	var calls atomic.Int32
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Messages []struct {
				Content string `json:"content"`
			} `json:"messages"`
			Tools []any `json:"tools"`
		}
		if json.NewDecoder(r.Body).Decode(&body) != nil {
			t.Error("invalid model request")
		}
		if len(body.Tools) != 0 {
			t.Error("team must not expose tools")
		}
		call := calls.Add(1)
		if call > 1 && !strings.Contains(body.Messages[len(body.Messages)-1].Content, "deliverable:\nresult") {
			t.Error("missing previous result")
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"choices":[{"message":{"content":"result"}}]}`))
	}))
	defer provider.Close()
	s := goalTestServer(t)
	s.cfg.Providers = map[string]config.Provider{"local": {BaseURL: provider.URL + "/v1"}}
	s.cfg.Roles = map[string]config.Role{s.role: {Provider: "local", Model: "test-model"}}
	w := httptest.NewRecorder()
	s.handleTeamRun(w, httptest.NewRequest("POST", "/api/v2/team/run", strings.NewReader(`{"task":"report","lang":"ca"}`)))
	if calls.Load() != 4 || !strings.Contains(w.Body.String(), `"type":"done"`) || strings.Count(w.Body.String(), `"type":"handoff"`) != 4 {
		t.Fatalf("calls=%d body=%s", calls.Load(), w.Body.String())
	}
	if s.flowRunning {
		t.Fatal("busy guard leaked")
	}
}

func TestTeamAPIValidation(t *testing.T) {
	s := goalTestServer(t)
	for _, body := range []string{`{}`, `{"task":"x","lang":"es"}`, `{"task":"x","token":"private"}`} {
		w := httptest.NewRecorder()
		s.handleTeamRun(w, httptest.NewRequest("POST", "/api/v2/team/run", strings.NewReader(body)))
		if w.Code != 400 {
			t.Fatalf("status=%d", w.Code)
		}
	}
	w := httptest.NewRecorder()
	s.handleTeamRun(w, httptest.NewRequest("GET", "/api/v2/team/run", nil))
	if w.Code != 405 {
		t.Fatal("GET should not run")
	}
	s.agentBusy = true
	w = httptest.NewRecorder()
	s.handleTeamRun(w, httptest.NewRequest("POST", "/api/v2/team/run", strings.NewReader(`{"task":"x"}`)))
	if w.Code != 409 {
		t.Fatal("busy guard")
	}
}

func TestTeamRequiresAuthentication(t *testing.T) {
	s := goalTestServer(t)
	s.SetToken("synthetic-team-auth")
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, httptest.NewRequest("POST", "/api/v2/team/run", strings.NewReader(`{"task":"x"}`)))
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated status=%d", w.Code)
	}
}

func TestTeamErrorKindSeparatesTimeoutFromCancel(t *testing.T) {
	live, stopLive := context.WithCancel(context.Background())
	defer stopLive()
	timedOut, stopTimeout := context.WithTimeout(context.Background(), time.Nanosecond)
	defer stopTimeout()
	<-timedOut.Done()
	canceled, stopCanceled := context.WithCancel(context.Background())
	stopCanceled()
	for _, tc := range []struct {
		name string
		err  error
		ctx  context.Context
		want string
	}{
		{"provider error stays failed", errors.New("HTTP 500"), live, "failed"},
		{"user stop stays cancelled", context.Canceled, canceled, "cancelled"},
		{"timeout is a failure, not a stop", context.DeadlineExceeded, timedOut, "failed"},
		{"inner cancel without dead ctx stays cancelled", context.Canceled, live, "cancelled"},
	} {
		if got := teamErrorKind(tc.err, tc.ctx); got != tc.want {
			t.Errorf("%s: got %q, want %q", tc.name, got, tc.want)
		}
	}
}

// Reproducció de l'incident: el pla entra, les branques es pengen (servidor
// local saturat) i el timeout ha de reportar failed, mai cancelled/Stopped.
func TestTeamHungBranchesReportFailedNotStopped(t *testing.T) {
	oldTimeout := teamRunTimeout
	teamRunTimeout = 300 * time.Millisecond
	defer func() { teamRunTimeout = oldTimeout }()
	prevLog := log.Default().Writer()
	var logBuf bytes.Buffer
	log.SetOutput(&logBuf)
	defer log.SetOutput(prevLog)

	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Messages []struct {
				Content string `json:"content"`
			} `json:"messages"`
		}
		if json.NewDecoder(r.Body).Decode(&body) != nil {
			t.Error("invalid model request")
			return
		}
		text := ""
		for _, m := range body.Messages {
			text += "\n" + m.Content
		}
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(text, "concise plan") {
			w.Write([]byte(`{"choices":[{"message":{"content":"plan ok"}}]}`))
			return
		}
		<-r.Context().Done()
		w.WriteHeader(http.StatusGatewayTimeout)
	}))
	defer provider.Close()
	s := goalTestServer(t)
	s.cfg.Providers = map[string]config.Provider{"local": {BaseURL: provider.URL + "/v1"}}
	s.cfg.Roles = map[string]config.Role{s.role: {Provider: "local", Model: "test-model"}}
	w := httptest.NewRecorder()
	s.handleTeamRun(w, httptest.NewRequest("POST", "/api/v2/team/run", strings.NewReader(`{"task":"report","lang":"ca"}`)))
	body := w.Body.String()
	if !strings.Contains(body, `"type":"failed"`) {
		t.Fatalf("hung branches must report failed: %s", body)
	}
	if strings.Contains(body, `"type":"cancelled"`) {
		t.Fatalf("timeout must never look like a user stop: %s", body)
	}
	if !strings.Contains(body, `"agent":"coordinator"`) {
		t.Fatalf("plan stage never ran: %s", body)
	}
	if !strings.Contains(logBuf.String(), "team run failed after") {
		t.Fatalf("terminal line missing: %q", logBuf.String())
	}
	if s.flowRunning {
		t.Fatal("busy guard leaked")
	}
}

// L'error real d'un rol ha de sortir al terminal sanejat: és l'únic lloc
// on es pot diagnosticar una fallada que la UI amaga per seguretat.
func TestTeamRoleErrorLoggedSanitized(t *testing.T) {
	prevLog := log.Default().Writer()
	var logBuf bytes.Buffer
	log.SetOutput(&logBuf)
	defer log.SetOutput(prevLog)

	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		w.Write([]byte(`{"error":"upstream https://models.example.net/v1?key=S3CR3T boom"}`))
	}))
	defer provider.Close()
	s := goalTestServer(t)
	s.cfg.Providers = map[string]config.Provider{"local": {BaseURL: provider.URL + "/v1"}}
	s.cfg.Roles = map[string]config.Role{s.role: {Provider: "local", Model: "test-model"}}
	w := httptest.NewRecorder()
	s.handleTeamRun(w, httptest.NewRequest("POST", "/api/v2/team/run", strings.NewReader(`{"task":"report"}`)))
	body := w.Body.String()
	if !strings.Contains(body, `"type":"failed"`) {
		t.Fatalf("provider error must report failed: %s", body)
	}
	logged := logBuf.String()
	if !strings.Contains(logged, "team role coordinator error:") {
		t.Fatalf("role error missing from terminal log: %q", logged)
	}
	for _, secret := range []string{"S3CR3T", "models.example.net"} {
		if strings.Contains(logged, secret) {
			t.Fatalf("log leaks provider details: %q", logged)
		}
	}
}

// Réplica de l'incident: el reviewer treballa una estona i el proveïdor
// falla tard (502 amb cos sensible). Ha de dir failed, mai cancelled, i
// el terminal ho ha de dir sanejat.
func TestTeamReviewerLateErrorReportsFailed(t *testing.T) {
	prevLog := log.Default().Writer()
	var logBuf bytes.Buffer
	log.SetOutput(&logBuf)
	defer log.SetOutput(prevLog)

	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Messages []struct {
				Content string `json:"content"`
			} `json:"messages"`
		}
		if json.NewDecoder(r.Body).Decode(&body) != nil {
			t.Error("invalid model request")
			return
		}
		text := ""
		for _, m := range body.Messages {
			text += "\n" + m.Content
		}
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(text, "Return exactly one JSON object"):
			time.Sleep(2 * time.Second)
			w.WriteHeader(http.StatusBadGateway)
			w.Write([]byte(`{"error":"upstream https://models.example.net/v1?key=S3CR3T"}`))
		case strings.Contains(text, "concise plan"):
			w.Write([]byte(`{"choices":[{"message":{"content":"plan ok"}}]}`))
		default:
			w.Write([]byte(`{"choices":[{"message":{"content":"branch ok"}}]}`))
		}
	}))
	defer provider.Close()
	s := goalTestServer(t)
	s.cfg.Providers = map[string]config.Provider{"local": {BaseURL: provider.URL + "/v1"}}
	s.cfg.Roles = map[string]config.Role{s.role: {Provider: "local", Model: "test-model"}}
	w := httptest.NewRecorder()
	s.handleTeamRun(w, httptest.NewRequest("POST", "/api/v2/team/run", strings.NewReader(`{"task":"report"}`)))
	body := w.Body.String()
	if !strings.Contains(body, `"type":"failed"`) {
		t.Fatalf("late reviewer error must report failed: %s", body)
	}
	if strings.Contains(body, `"type":"cancelled"`) {
		t.Fatalf("reviewer error must never look like a stop: %s", body)
	}
	logged := logBuf.String()
	if !strings.Contains(logged, "team role reviewer error:") {
		t.Fatalf("reviewer error missing from terminal log: %q", logged)
	}
	for _, secret := range []string{"S3CR3T", "models.example.net"} {
		if strings.Contains(logged, secret) {
			t.Fatalf("log leaks provider details: %q", logged)
		}
	}
	if s.flowRunning {
		t.Fatal("busy guard leaked")
	}
}

// Un reviewer que torna JSON malformat (model petit, esquema estricte)
// també és failed, no stopped.
func TestTeamReviewerGarbageReportsFailed(t *testing.T) {
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Messages []struct {
				Content string `json:"content"`
			} `json:"messages"`
		}
		if json.NewDecoder(r.Body).Decode(&body) != nil {
			t.Error("invalid model request")
			return
		}
		text := ""
		for _, m := range body.Messages {
			text += "\n" + m.Content
		}
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(text, "Return exactly one JSON object") {
			w.Write([]byte(`{"choices":[{"message":{"content":"{\"approved\": true, \"feedback\": \"oops\""}}]}`))
			return
		}
		w.Write([]byte(`{"choices":[{"message":{"content":"fine"}}]}`))
	}))
	defer provider.Close()
	s := goalTestServer(t)
	s.cfg.Providers = map[string]config.Provider{"local": {BaseURL: provider.URL + "/v1"}}
	s.cfg.Roles = map[string]config.Role{s.role: {Provider: "local", Model: "test-model"}}
	w := httptest.NewRecorder()
	s.handleTeamRun(w, httptest.NewRequest("POST", "/api/v2/team/run", strings.NewReader(`{"task":"report"}`)))
	body := w.Body.String()
	if !strings.Contains(body, `"type":"failed"`) {
		t.Fatalf("malformed review must report failed: %s", body)
	}
	if strings.Contains(body, `"type":"cancelled"`) || strings.Contains(body, `"type":"done"`) {
		t.Fatalf("malformed review must not pass or look stopped: %s", body)
	}
}

// Un flow invàlid és un 400 abans de gastar cap crida al model.
func TestTeamFlowValidation400(t *testing.T) {
	s := goalTestServer(t)
	s.cfg.Providers = map[string]config.Provider{"local": {BaseURL: "http://127.0.0.1:1/v1"}}
	s.cfg.Roles = map[string]config.Role{s.role: {Provider: "local", Model: "test-model"}}
	w := httptest.NewRecorder()
	s.handleTeamRun(w, httptest.NewRequest("POST", "/api/v2/team/run", strings.NewReader(`{"task":"x","flow":{"nodes":[{"id":"a","role":"builder"}],"edges":[{"from":"a","to":"a","kind":"data"}],"output":"a"}}`)))
	if w.Code != 400 {
		t.Fatalf("invalid flow status=%d, want 400", w.Code)
	}
}

// Un flow vàlid i propi s'executa de cap a cap pel mateix SSE.
func TestTeamCustomFlowRunsEndToEnd(t *testing.T) {
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Messages []struct {
				Content string `json:"content"`
			} `json:"messages"`
		}
		if json.NewDecoder(r.Body).Decode(&body) != nil {
			t.Error("invalid model request")
			return
		}
		text := ""
		for _, m := range body.Messages {
			text += "\n" + m.Content
		}
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(text, "Return exactly one JSON object") {
			w.Write([]byte(`{"choices":[{"message":{"content":"{\"approved\": true, \"feedback\": \"\", \"deliverable\": \"custom done\"}"}}]}`))
			return
		}
		w.Write([]byte(`{"choices":[{"message":{"content":"custom draft"}}]}`))
	}))
	defer provider.Close()
	s := goalTestServer(t)
	s.cfg.Providers = map[string]config.Provider{"local": {BaseURL: provider.URL + "/v1"}}
	s.cfg.Roles = map[string]config.Role{s.role: {Provider: "local", Model: "test-model"}}
	flow := `{"nodes":[
		{"id":"boss","role":"coordinator","mode":"task","instruction":"Plan briefly."},
		{"id":"maker","role":"builder","mode":"task","instruction":"Build it."},
		{"id":"judge","role":"reviewer","mode":"review"}],
		"edges":[
		{"from":"boss","to":"maker","kind":"data"},
		{"from":"maker","to":"judge","kind":"data"}],
		"output":"judge","max_rounds":0}`
	w := httptest.NewRecorder()
	s.handleTeamRun(w, httptest.NewRequest("POST", "/api/v2/team/run", strings.NewReader(`{"task":"x","flow":`+flow+`}`)))
	body := w.Body.String()
	if !strings.Contains(body, `"type":"done"`) || !strings.Contains(body, "custom done") {
		t.Fatalf("custom flow did not finish: %s", body)
	}
	if strings.Contains(body, `"type":"failed"`) || strings.Contains(body, `"type":"cancelled"`) {
		t.Fatalf("custom flow reported failure: %s", body)
	}
	if s.flowRunning {
		t.Fatal("busy guard leaked")
	}
}

// Un equip real publica la sala compartida: la mateixa sessió exposa
// l'execució com a badia (kind team, flow, agents) sense cap sessió nova.
func TestTeamRunPublishesRoomBay(t *testing.T) {
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"choices":[{"message":{"content":"result"}}]}`))
	}))
	defer provider.Close()
	s := goalTestServer(t)
	s.cfg.Providers = map[string]config.Provider{"local": {BaseURL: provider.URL + "/v1"}}
	s.cfg.Roles = map[string]config.Role{s.role: {Provider: "local", Model: "test-model"}}
	w := httptest.NewRecorder()
	s.handleTeamRun(w, httptest.NewRequest("POST", "/api/v2/team/run", strings.NewReader(`{"task":"report"}`)))
	if !strings.Contains(w.Body.String(), `"type":"done"`) {
		t.Fatalf("team run did not finish: %s", w.Body.String())
	}
	snap := workshopSnapshotForTest(s)
	if snap == nil || snap.Kind != "team" || snap.State != "completed" {
		t.Fatalf("room must show the team run: %+v", snap)
	}
	if snap.Flow == nil || len(snap.Flow.Nodes) != 4 {
		t.Fatalf("room must carry the flow for the graph: %+v", snap.Flow)
	}
	got := map[string]string{}
	for _, a := range snap.Agents {
		got[a.ID] = a.Status
	}
	for _, id := range []string{"coordinator", "researcher", "builder", "reviewer"} {
		if got[id] != agent.DelegateCompleted {
			t.Fatalf("room agent %s = %q, want completed (all %v)", id, got[id], got)
		}
	}
	if snap.Agents[0].Output == "" {
		t.Fatal("room agents must retain deliverables")
	}
}
