package tools

import (
	"path/filepath"
	"strings"
	"testing"
)

// Un sol objecte de diapositiva (com diu la descripció de l'eina) ha
// d'afegir una diapositiva, no «0 diapositives» amb èxit.
func TestAddSlideAcceptsSingleSlideObject(t *testing.T) {
	p := filepath.Join(t.TempDir(), "deck.pptx")
	if _, err := OfficeCreate(p, "Prova", "# Primera\n\n- un punt"); err != nil {
		t.Fatal(err)
	}
	out, err := OfficeEdit(EditOpts{Path: p, Op: "add_slide", Spec: `{"title":"Segona","bullets":["a","b"]}`})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "afegides 1 diapositives") {
		t.Fatalf("single slide object not added: %s", out)
	}
	if _, err := OfficeEdit(EditOpts{Path: p, Op: "add_slide", Spec: `[]`}); err == nil {
		t.Fatal("an empty slide list must fail, not report success")
	}
}
