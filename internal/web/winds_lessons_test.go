package web

import "testing"

// Un vent es va apuntar «web_images està bloquejada» (un dia la cerca va
// fallar o es va acabar el pressupost de recerca) i no va tornar a buscar cap
// foto. Aquestes lliçons no es llegeixen ni es desen; les de debò, sí.
func TestBlockedSearchToolLessonsAreIgnored(t *testing.T) {
	bad := []string{
		"- `web_images` està bloquejada; per a gràfics, genera HTML amb SVG inline i envia'l amb `send_file`.",
		"- web_search is disabled here, use browser instead",
		"- El cercador d'imatges no està disponible; baixa les fotos amb bash.",
	}
	for _, l := range bad {
		if !blockedToolLesson(l) {
			t.Errorf("should be ignored: %q", l)
		}
	}
	good := []string{
		"- Per a partits d'un equip, usa TheSportsDB `eventsnext.php?id=<id>`.",
		"- Si `web_fetch` amb `find` no troba dates, torna a cridar sense `find`.",
		"- `browser` no obre `file://`; per previsualitzar HTML, usa `send_file`.",
		"- `npm install -g` està bloquejat; instal·la-ho dins del projecte.",
	}
	for _, l := range good {
		if blockedToolLesson(l) {
			t.Errorf("a real lesson was dropped: %q", l)
		}
	}
	body := "# Lliçons de feina\n\n" + bad[0] + "\n" + good[0]
	if got := lessonLines(body); len(got) != 1 || got[0] != good[0] {
		t.Fatalf("lessonLines: %v", got)
	}
}
