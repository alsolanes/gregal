package llm

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestSpeakerMetadataPersistsWithoutProviderWireFields(t *testing.T) {
	var m Message
	if err := json.Unmarshal([]byte(`{"role":"assistant","content":"Xaloc: hola","speaker_id":"xaloc","speaker_name":"Xaloc"}`), &m); err != nil {
		t.Fatal(err)
	}
	saved, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(saved), `"speaker_id":"xaloc"`) || !strings.Contains(string(saved), `"speaker_name":"Xaloc"`) {
		t.Fatalf("speaker lost in persistence: %s", saved)
	}
	wire, err := marshalMessage(m, true)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(wire), "speaker_") {
		t.Fatalf("UI metadata leaked to strict provider: %s", wire)
	}
}
