package llm

import (
	"encoding/json"
	"strings"
	"testing"
)

// L'id estable del missatge es desa amb la conversa però no viatja mai al
// provider (els endpoints estrictes rebutgen camps desconeguts).
func TestMessageIDPersistsWithoutProviderWire(t *testing.T) {
	for _, m := range []Message{
		{Role: "assistant", Content: "hola", ID: "m-abc"},
		{Role: "user", Content: "mira", Images: []string{"data:image/png;base64,AA=="}, ID: "m-img"},
	} {
		saved, err := json.Marshal(m)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(saved), `"id":"`+m.ID+`"`) {
			t.Fatalf("id perdut en desar: %s", saved)
		}
		wire, err := marshalMessage(m, true)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(wire), `"id"`) {
			t.Fatalf("id filtrat al provider: %s", wire)
		}
	}
}

// B6: el lligam d'un missatge proactiu amb el seu run i resultats es desa
// amb el fil però no arriba mai al provider.
func TestProactiveLinksNeverReachProvider(t *testing.T) {
	m := Message{Role: "assistant", Content: "canvi", ID: "m-x", RelatedRunID: 412, ResultRefs: []string{"res-1"}}
	saved, _ := json.Marshal(m)
	if !strings.Contains(string(saved), `"relatedRunId":412`) || !strings.Contains(string(saved), `"resultRefs":["res-1"]`) {
		t.Fatalf("lligams perduts en desar: %s", saved)
	}
	wire, err := marshalMessage(m, true)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(wire), "relatedRunId") || strings.Contains(string(wire), "resultRefs") {
		t.Fatalf("lligams filtrats al provider: %s", wire)
	}
}
