package companion

import "testing"

func TestWindEditKeepsExplicitIDAndPersona(t *testing.T) {
	ws := NewWindStore(t.TempDir())
	first, err := ws.Save(Wind{Name: "Tramuntana", Persona: "Veu pròpia"})
	if err != nil {
		t.Fatal(err)
	}
	edited, err := ws.Save(Wind{ID: first.ID, Name: "Nom nou", Role: "ciència"})
	if err != nil {
		t.Fatal(err)
	}
	if edited.ID != first.ID || edited.Persona != first.Persona {
		t.Fatalf("edit lost identity/persona: %+v", edited)
	}
	list, err := ws.List()
	if err != nil || len(list) != 1 {
		t.Fatalf("edit created duplicate: %+v %v", list, err)
	}
	legacy, err := ws.Save(Wind{Name: "Tramuntana"})
	if err != nil || legacy.ID != first.ID || legacy.Persona != first.Persona {
		t.Fatalf("legacy same-name save: %+v %v", legacy, err)
	}
	if _, err := ws.Save(Wind{ID: "unknown", Name: "Other"}); err == nil {
		t.Fatal("unknown explicit ID must not create a wind")
	}
	list, _ = ws.List()
	if len(list) != 1 {
		t.Fatalf("unknown ID mutated catalog: %+v", list)
	}
}
