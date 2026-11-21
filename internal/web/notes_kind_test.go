package web

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

// Desat: una resposta desada és una nota amb tipus; el cos és el markdown
// de la resposta amb la targeta tal qual.
func TestNotesKindAutoFromCard(t *testing.T) {
	s := rutinaTestServer(t)
	body := "Aquí la tens.\n\n```gregal-recipe\n{\"title\":\"Llenties estofades\",\"servings\":2}\n```\n"
	n := notesSave(t, s, map[string]any{"body": body, "kind": "auto"})
	if n.Kind != "recipe" || n.Title != "Llenties estofades" || n.Body != body {
		t.Fatalf("kind=%q title=%q body=%q", n.Kind, n.Title, n.Body)
	}
	if c := notesSave(t, s, map[string]any{"body": "```chart\n{\"title\":\"Pluja\"}\n```", "kind": "auto"}); c.Kind != "chart" || c.Title != "Pluja" {
		t.Fatalf("chart: %q %q", c.Kind, c.Title)
	}
	if w := notesSave(t, s, map[string]any{"body": "```gregal-web {\"title\":\"Tauler\"}\n```", "kind": "auto"}); w.Kind != "web" || w.Title != "Tauler" {
		t.Fatalf("same-line card: %q %q", w.Kind, w.Title)
	}
	if c := notesSave(t, s, map[string]any{"body": "```chartjs\nnew Chart()\n```", "kind": "auto"}); c.Kind != "text" {
		t.Fatalf("chartjs is code, not a card: %q", c.Kind)
	}
	if tip := notesSave(t, s, map[string]any{"body": "Posa sal al final.", "kind": "auto"}); tip.Kind != "text" || tip.Title != "Posa sal al final." {
		t.Fatalf("text: %q %q", tip.Kind, tip.Title)
	}
	// Una edició sense kind el conserva, i el fitxer el guarda.
	edited := notesSave(t, s, map[string]any{"id": n.ID, "title": "Llenties de l'àvia"})
	if edited.Kind != "recipe" {
		t.Fatalf("kind lost on edit: %q", edited.Kind)
	}
	code, raw := notesReq(t, s, http.MethodGet, "/api/notes?id="+n.ID, nil)
	var got struct{ Note Note }
	if code != 200 || json.Unmarshal(raw, &got) != nil || got.Note.Kind != "recipe" {
		t.Fatalf("read back: %d %s", code, raw)
	}
}

func TestNotesKindExplicitAndInvalid(t *testing.T) {
	s := rutinaTestServer(t)
	if n := notesSave(t, s, map[string]any{"body": "Talla la ceba en fred.", "kind": "tip", "title": "Ceba"}); n.Kind != "tip" || n.Title != "Ceba" {
		t.Fatalf("tip: %q %q", n.Kind, n.Title)
	}
	if code, _ := notesReq(t, s, http.MethodPost, "/api/notes", map[string]any{"action": "save", "body": "x", "kind": "poem"}); code != 400 {
		t.Fatalf("unknown kind: %d", code)
	}
}

func TestNotesKindFilterAndCounts(t *testing.T) {
	s := rutinaTestServer(t)
	notesSave(t, s, map[string]any{"body": "Una nota normal"})
	notesSave(t, s, map[string]any{"body": "```gregal-recipe\n{\"title\":\"Crema\"}\n```", "kind": "auto"})
	notesSave(t, s, map[string]any{"body": "```gregal-recipe\n{\"title\":\"Coca\"}\n```", "kind": "auto"})
	notesSave(t, s, map[string]any{"body": "Un consell", "kind": "tip"})
	list := func(q string) (titles []string, kinds []noteCount) {
		code, raw := notesReq(t, s, http.MethodGet, "/api/notes"+q, nil)
		var out struct {
			Notes []Note      `json:"notes"`
			Kinds []noteCount `json:"kinds"`
		}
		if code != 200 || json.Unmarshal(raw, &out) != nil {
			t.Fatalf("list %s: %d %s", q, code, raw)
		}
		for _, n := range out.Notes {
			titles = append(titles, n.Title)
		}
		return titles, out.Kinds
	}
	if got, kinds := list("?kind=recipe"); len(got) != 2 || len(kinds) != 2 || kinds[0] != (noteCount{"recipe", 2}) || kinds[1] != (noteCount{"tip", 1}) {
		t.Fatalf("recipes: %v kinds %v", got, kinds)
	}
	if got, _ := list("?kind=*"); len(got) != 3 {
		t.Fatalf("all saved: %v", got)
	}
	if got, _ := list(""); len(got) != 4 {
		t.Fatalf("all notes: %v", got)
	}
}

// Un desat es busca i es llegeix pel que diu la targeta, no pel seu JSON.
func TestNotesCardSearchAndExcerptReadText(t *testing.T) {
	s := rutinaTestServer(t)
	body := "Aquí la tens.\n\n```gregal-recipe\n{\"title\":\"Llenties estofades\",\"image_query\":\"lentil stew\",\"ingredients\":[{\"name\":\"llenties\",\"qty\":\"400 g\"}],\"steps\":[{\"text\":\"Sofregeix la ceba\",\"minutes\":8}]}\n```\n"
	notesSave(t, s, map[string]any{"body": body, "kind": "auto", "title": "Sopar"})
	list := notesGet(t, s, "?q=llenties")
	if len(list.Notes) != 1 {
		t.Fatalf("card text not searchable: %+v", list.Notes)
	}
	n := list.Notes[0]
	for _, raw := range []string{n.Excerpt, n.Snippet} {
		if strings.ContainsAny(raw, "{}\"") || strings.Contains(raw, "image_query") || !strings.Contains(raw, "Llenties estofades") {
			t.Fatalf("excerpt/snippet shows JSON or misses the card: %q", raw)
		}
	}
	// Els textos d'una llista després d'un objecte no hereten el camp d'aquell objecte.
	if got := noteReadable("```gregal-result\n{\"rows\":[{\"lead\":\"A\",\"url\":\"https://x.test\"},\"Visible\"],\"type\":\"bar\"}\n```"); !strings.Contains(got, "A · Visible") || strings.Contains(got, "x.test") || strings.Contains(got, "bar") {
		t.Fatalf("mixed list: %q", got)
	}
	for _, q := range []string{"rega", "title", "image_query", "lentil"} {
		if got := notesGet(t, s, "?q="+q); len(got.Notes) != 0 {
			t.Fatalf("q=%s matched the card syntax: %+v", q, got.Notes)
		}
	}
}
