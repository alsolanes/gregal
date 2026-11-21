package web

import (
	"encoding/json"
	"regexp"
	"strings"
)

// Els tipus d'una cosa desada d'una resposta. Les targetes (md.js, Md.kt)
// donen el seu; una resposta sense targeta és text, o un consell si el vent
// o la persona ho diuen.
var noteKindByFence = map[string]string{
	"gregal-recipe": "recipe", "gregal-plan": "plan", "gregal-route": "route", "gregal-compare": "compare",
	"gregal-facts": "facts", "gregal-result": "result", "chart": "chart", "gregal-plot": "chart", "gregal-web": "web",
}

var noteKindNames = map[string]bool{"recipe": true, "tip": true, "plan": true, "route": true, "compare": true,
	"facts": true, "result": true, "chart": true, "web": true, "text": true}

// noteCardFence: la primera targeta del cos, amb el JSON a la mateixa línia
// («```gregal-recipe {…}») o a la següent.
var noteCardFence = regexp.MustCompile("(?s)```(gregal-[a-z]+|chart)(?:[ \t]+|\n)(.*?)```")

// noteKind valida el tipus demanat; "auto" el treu de la primera targeta i en
// torna també el títol. "" = tipus desconegut.
func noteKind(want, body string) (kind, title string) {
	want = strings.TrimSpace(want)
	if want != "auto" {
		if noteKindNames[want] {
			return want, ""
		}
		return "", ""
	}
	for _, m := range noteCardFence.FindAllStringSubmatch(body, -1) {
		k, ok := noteKindByFence[m[1]]
		if !ok {
			continue
		}
		var head struct {
			Title string `json:"title"`
			Name  string `json:"name"`
		}
		// Un JSON trencat encara és una targeta d'aquest tipus, sense títol.
		if json.NewDecoder(strings.NewReader(strings.TrimSpace(m[2]))).Decode(&head) != nil {
			return k, ""
		}
		return k, strings.TrimSpace(firstNonEmpty(head.Title, head.Name))
	}
	return "text", ""
}
