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
