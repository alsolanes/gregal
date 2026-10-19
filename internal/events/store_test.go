package events

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStoreCursorIPersistencia(t *testing.T) {
	p := filepath.Join(t.TempDir(), "events.jsonl")
	s, err := Open(p)
	if err != nil {
		t.Fatal(err)
	}
	a, err := s.Append(Event{SessionID: "a", Kind: "assistant", Text: "hola"})
	if err != nil {
		t.Fatal(err)
	}
	b, err := s.Append(Event{SessionID: "a", Kind: "done", Text: "fi"})
	if err != nil {
		t.Fatal(err)
	}
	if a.ID != 1 || b.ID != 2 {
		t.Fatalf("ids: %d %d", a.ID, b.ID)
	}
	got, err := s.After(1, 10)
	if err != nil || len(got) != 1 || got[0].ID != 2 {
		t.Fatalf("after: %+v %v", got, err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = Open(p)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	c, err := s.Append(Event{Kind: "error", Text: "x"})
	if err != nil {
		t.Fatal(err)
	}
	if c.ID != 3 {
		t.Fatalf("cursor no recuperat: %d", c.ID)
	}
}

func TestStoreRecuperaLiniaFinalTrencada(t *testing.T) {
	p := filepath.Join(t.TempDir(), "events.jsonl")
	if err := os.WriteFile(p, []byte(`{"id":3,"kind":"assistant","text":"bé"}`+"\n"+`{"id":4,"kind":"assistant"`), 0o600); err != nil {
		t.Fatal(err)
	}
	s, err := Open(p)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	e, err := s.Append(Event{Kind: "done", Text: "fi"})
	if err != nil || e.ID != 4 {
		t.Fatalf("recuperació: %+v %v", e, err)
	}
}

func TestStorePayloadOpcionalPersisteixIReprodueix(t *testing.T) {
	p := filepath.Join(t.TempDir(), "events.jsonl")
	s, err := Open(p)
	if err != nil {
		t.Fatal(err)
	}
	payload := json.RawMessage(`{"key":"7","call_id":"call-1","name":"shell","args":"ls"}`)
	added, err := s.Append(Event{Kind: "approve_request", Text: "permís", Payload: payload})
	if err != nil {
		t.Fatal(err)
	}
	legacy, err := s.Append(Event{Kind: "text", Text: "antic"})
	if err != nil {
		t.Fatal(err)
	}
	if !json.Valid(added.Payload) || legacy.Payload != nil {
		t.Fatalf("payload inicial inesperat: %+v %+v", added.Payload, legacy.Payload)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(p)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	got, err := reopened.After(0, 10)
	if err != nil || len(got) != 2 {
		t.Fatalf("replay: %+v %v", got, err)
	}
	if string(got[0].Payload) != string(payload) || got[1].Payload != nil {
		t.Fatalf("payload no recuperat: %+v", got)
	}
	raw, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"payload"`) {
		t.Fatalf("el JSON no desa payload: %s", raw)
	}
}

func TestStoreRefusaPayloadInvalid(t *testing.T) {
	p := filepath.Join(t.TempDir(), "events.jsonl")
	s, err := Open(p)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, err := s.Append(Event{Kind: "bad", Payload: json.RawMessage(`{"`)}); err == nil {
		t.Fatal("un payload JSON invàlid s'havia d'rebutjar")
	}
	if got, err := s.Append(Event{Kind: "ok", Text: "després"}); err != nil || got.ID != 1 {
		t.Fatalf("el cursor no s'hauria d'haver consumit: %+v %v", got, err)
	}
}

func TestMaxRunIDSurvivesReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []int{1, 7, 3} {
		if _, err := s.Append(Event{RunID: id, Kind: "run_queued"}); err != nil {
			t.Fatal(err)
		}
	}
	if got := s.MaxRunID(); got != 7 {
		t.Fatalf("MaxRunID = %d, want 7", got)
	}
	s2, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := s2.MaxRunID(); got != 7 {
		t.Fatalf("after reopen MaxRunID = %d, want 7", got)
	}
}

func TestRunEventsStartsAtRunCursorAndPaginates(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	for i := 0; i < 620; i++ {
		if _, err := s.Append(Event{RunID: 44, User: "anna", SessionID: "tab-a", Kind: "token", Text: "x"}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.Append(Event{RunID: 44, User: "anna", SessionID: "tab-a", Kind: "run_completed"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Append(Event{RunID: 45, User: "anna", SessionID: "tab-b", Kind: "run_started"}); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = Open(path) // the run cursor index must be rebuilt on restart
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	got, err := s.RunEvents(44, "anna", "tab-a")
	if err != nil || len(got) != 621 || got[0].ID != 1 || got[len(got)-1].Kind != "run_completed" {
		t.Fatalf("run events: count=%d err=%v", len(got), err)
	}
	if other, err := s.RunEvents(45, "anna", "tab-a"); err != nil || len(other) != 0 {
		t.Fatalf("other session leaked into run: %+v, %v", other, err)
	}
}

func TestUnfinishedSurvivesReopen(t *testing.T) {
	p := filepath.Join(t.TempDir(), "events.jsonl")
	s, err := Open(p)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range []Event{
		{SessionID: "mobil", RunID: 1, Kind: "run_queued"},
		{SessionID: "mobil", RunID: 1, Kind: "run_started"},
		{SessionID: "mobil", RunID: 1, Kind: "run_completed"},
		{SessionID: "mobil", RunID: 1, Kind: "recap"}, // després del final no el reobre
		{SessionID: "mobil", User: "anna", RunID: 2, Kind: "run_queued"},
		{SessionID: "mobil", RunID: 2, Kind: "tool"},
		{SessionID: "default", RunID: 3, Kind: "run_queued"},
		{SessionID: "default", RunID: 3, Kind: "run_cancelled"},
	} {
		if _, err := s.Append(e); err != nil {
			t.Fatal(err)
		}
	}
	got := s.Unfinished()
	if len(got) != 1 || got[0].RunID != 2 || got[0].User != "anna" {
		t.Fatalf("unfinished: %+v", got)
	}
	s, err = Open(p)
	if err != nil {
		t.Fatal(err)
	}
	if got := s.Unfinished(); len(got) != 1 || got[0].RunID != 2 {
		t.Fatalf("after reopen: %+v", got)
	}
	if _, err := s.Append(Event{SessionID: "mobil", RunID: 2, Kind: "run_failed"}); err != nil {
		t.Fatal(err)
	}
	if got := s.Unfinished(); len(got) != 0 {
		t.Fatalf("closed run still open: %+v", got)
	}
}

func TestAfterReadsLongLines(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "events.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	big := strings.Repeat("x", 200<<10)
	if _, err := s.Append(Event{Kind: "tool", Text: big}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Append(Event{Kind: "done", Text: "fi"}); err != nil {
		t.Fatal(err)
	}
	got, err := s.After(0, 10)
	if err != nil || len(got) != 2 || len(got[0].Text) != len(big) {
		t.Fatalf("after: %d events, err %v", len(got), err)
	}
}

func TestAfterRunSeeksAndPaginatesWithoutOtherRuns(t *testing.T) {
	storePath := filepath.Join(t.TempDir(), "events.jsonl")
	s, err := Open(storePath)
	if err != nil {
		t.Fatal(err)
	}
	for i := 1; i <= 620; i++ {
		run := 1
		if i >= 610 {
			run = 2
		}
		if _, err := s.Append(Event{RunID: run, SessionID: "a", Kind: "token", Text: "progress"}); err != nil {
			t.Fatal(err)
		}
	}
	// Rebuild the run index just as after a process restart.
	s, err = Open(storePath)
	if err != nil {
		t.Fatal(err)
	}
	page, err := s.AfterRun(0, 5, 2, "", "a")
	if err != nil || len(page) != 5 || page[0].ID != 610 {
		t.Fatalf("first page: %+v %v", page, err)
	}
	page, err = s.AfterRun(page[4].ID, 5, 2, "", "a")
	if err != nil || len(page) != 5 || page[0].ID != 615 {
		t.Fatalf("next page: %+v %v", page, err)
	}
	for _, scope := range []struct{ user, session string }{{"other", "a"}, {"", "b"}} {
		page, err = s.AfterRun(0, 5, 2, scope.user, scope.session)
		if err != nil || len(page) != 0 {
			t.Fatalf("scope leak: %+v %v", page, err)
		}
	}
}
