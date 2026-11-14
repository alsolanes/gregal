package web

import (
	"context"
	"strings"
	"testing"
)

func fakeCardImages(t *testing.T, found map[string]string) *[]string {
	t.Helper()
	var asked []string
	old := cardImageSearch
	cardImageSearch = func(_ context.Context, q string, _ int) (string, error) {
		asked = append(asked, q)
		if u, ok := found[q]; ok {
			return "1. Foto\n   image: " + u + "\n   page: https://commons.wikimedia.org/x\n", nil
		}
		return "No images found.", nil
	}
	t.Cleanup(func() { cardImageSearch = old })
	return &asked
}

func TestCardWithoutPhotoGetsOneFromImageQuery(t *testing.T) {
	asked := fakeCardImages(t, map[string]string{"Montserrat monastery": "https://upload.wikimedia.org/m.jpg"})
	s := &Server{}
	in := "Aquí el tens.\n\n```gregal-plan {\"title\":\"Un matí a Montserrat\",\"image_query\":\"Montserrat monastery\",\"items\":[{\"time\":\"08:36\",\"title\":\"R5 des d'Espanya\"}]}\n```\nBon viatge."
	out := s.windCardImages(in)
	if !strings.Contains(out, `"image":"https://upload.wikimedia.org/m.jpg"`) || strings.Contains(out, "image_query") {
		t.Fatalf("photo not set or query left:\n%s", out)
	}
	if !strings.HasPrefix(out, "Aquí el tens.") || !strings.HasSuffix(out, "Bon viatge.") || !strings.Contains(out, "```gregal-plan\n{") {
		t.Fatalf("text around the card changed:\n%s", out)
	}
	if strings.Join(*asked, "|") != "Montserrat monastery" {
		t.Fatalf("asked %v", *asked)
	}
}

func TestCardPhotoFallsBackToTitleAndInventedURLsGo(t *testing.T) {
	fakeCardImages(t, map[string]string{"Cigrons amb espinacs": "https://upload.wikimedia.org/c.jpg"})
	s := &Server{}
	out := s.windCardImages("```gregal-recipe\n{\"title\":\"Cigrons amb espinacs\",\"image\":\"/api/winds/image?id=placeholder\",\"steps\":[\"Bull\"]}\n```")
	if !strings.Contains(out, "https://upload.wikimedia.org/c.jpg") || strings.Contains(out, "placeholder") {
		t.Fatalf("%s", out)
	}
}

func TestCardKeepsItsPhotoAndOptionsOnlyAskWhenTold(t *testing.T) {
	asked := fakeCardImages(t, map[string]string{})
	s := &Server{}
	keep := "```gregal-plan\n{\"title\":\"Girona\",\"image\":\"https://e.com/g.jpg\",\"items\":[{\"title\":\"Tren\"}]}\n```"
	if out := s.windCardImages(keep); !strings.Contains(out, "https://e.com/g.jpg") {
		t.Fatalf("%s", out)
	}
	s.windCardImages("```gregal-result\n{\"title\":\"Portàtils\",\"rows\":[{\"lead\":\"A\"}]}\n```")
	if len(*asked) != 0 {
		t.Fatalf("searched for %v", *asked)
	}
	// Sense foto enlloc: la targeta queda igual (sense camps nous) i el text no es perd.
	out := s.windCardImages("Res:\n```gregal-plan\n{\"title\":\"Lloc rar\",\"items\":[{\"title\":\"X\"}]}\n```")
	if strings.Contains(out, `"image"`) || !strings.HasPrefix(out, "Res:") {
		t.Fatalf("%s", out)
	}
	if bad := "```gregal-plan\nno és json\n```"; s.windCardImages(bad) != bad {
		t.Fatal("broken JSON must stay as written")
	}
}
