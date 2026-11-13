package tools

import (
	"context"
	"testing"
)

// El col·lector del torn: la mateixa pàgina llegida dues vegades no fa dues
// captures; una acció que la canvia, sí; i com a molt TurnShotsMax, sempre
// amb la primera.
func TestTurnShotsKeepsWhatChanged(t *testing.T) {
	c := &TurnShots{}
	img := []byte{1}
	c.add(TurnShot{Image: img, URL: "https://a.example/", Action: "open"})
	c.add(TurnShot{Image: img, URL: "https://a.example/", Action: "screenshot"})
	if n := len(c.Shots()); n != 1 {
		t.Fatalf("la mateixa pàgina sense canvis: %d", n)
	}
	c.add(TurnShot{Image: img, URL: "https://a.example/", Action: "click"})
	if n := len(c.Shots()); n != 2 {
		t.Fatalf("un clic és una captura nova: %d", n)
	}
	for i := 0; i < 10; i++ {
		c.add(TurnShot{Image: img, URL: "https://b.example/" + string(rune('a'+i)), Action: "open", Title: string(rune('a' + i))})
	}
	s := c.Shots()
	if len(s) != TurnShotsMax || s[0].URL != "https://a.example/" || s[len(s)-1].Title != "j" {
		t.Fatalf("topall: %d %q %q", len(s), s[0].URL, s[len(s)-1].Title)
	}
	c.add(TurnShot{URL: "https://c.example/", Action: "open"})
	if len(c.Shots()) != TurnShotsMax {
		t.Fatal("sense imatge no es guarda")
	}
	if turnShotsFrom(context.Background()) != nil || turnShotsFrom(WithTurnShots(context.Background(), c)) != c {
		t.Fatal("context")
	}
}
