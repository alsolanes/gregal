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
