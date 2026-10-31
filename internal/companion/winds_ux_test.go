package companion

import (
	"reflect"
	"testing"
)

func TestWindDeletePreservesUnrelatedOrganization(t *testing.T) {
	store := NewWindStore(t.TempDir())
	removed, err := store.Save(Wind{Name: "North"})
	if err != nil {
		t.Fatal(err)
	}
	kept, err := store.Save(Wind{Name: "South"})
	if err != nil {
		t.Fatal(err)
	}
	meta := WindMeta{
		Favorites: []string{"wind:" + removed.ID, "wind:" + kept.ID, "rot:weekend"},
		Sections: []WindSection{
			{ID: "first", Name: "Shared", Items: []string{"wind:" + removed.ID, "wind:" + kept.ID, "rot:weekend"}},
			{ID: "empty", Name: "Keep this section", Items: []string{"wind:" + removed.ID}},
		},
	}
	if err := store.SaveMeta(meta); err != nil {
		t.Fatal(err)
	}
	if err := store.Delete(removed.ID); err != nil {
		t.Fatal(err)
	}
	want := WindMeta{
		Favorites: []string{"wind:" + kept.ID, "rot:weekend"},
		Sections: []WindSection{
			{ID: "first", Name: "Shared", Items: []string{"wind:" + kept.ID, "rot:weekend"}},
			{ID: "empty", Name: "Keep this section", Items: []string{}},
		},
	}
	got, err := store.Meta()
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("delete lost unrelated organization: got %+v, want %+v (error %v)", got, want, err)
	}
	if err := store.Restore(removed); err != nil {
		t.Fatal(err)
	}
	afterRestore, err := store.Meta()
	if err != nil || !reflect.DeepEqual(afterRestore, want) {
		t.Fatalf("restore changed organization: %+v (%v)", afterRestore, err)
	}
}

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
