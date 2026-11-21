package web

import (
	"encoding/json"
	"io"
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

// noteCardSkip: camps d'una targeta que no són text per llegir (fotos,
// enllaços, l'estil del gràfic).
var noteCardSkip = map[string]bool{"image": true, "image_query": true, "url": true, "src": true, "type": true, "mode": true}

// noteReadable és el cos amb cada targeta canviada pel seu text, per cercar i
// fer l'extracte: «Llenties estofades · llenties · 400 g · …», no el JSON.
func noteReadable(body string) string {
	return noteCardFence.ReplaceAllStringFunc(body, func(fence string) string {
		m := noteCardFence.FindStringSubmatch(fence)
		text, ok := noteCardText(m[2])
		if !ok {
			return fence
		}
		return "\n" + text + "\n"
	})
}

// noteCardText recull els textos d'una targeta en l'ordre del JSON.
func noteCardText(raw string) (string, bool) {
	dec := json.NewDecoder(strings.NewReader(strings.TrimSpace(raw)))
	var w cardWalk
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", false
		}
		if d, ok := tok.(json.Delim); ok {
			w.delim(d)
		} else {
			w.value(tok)
		}
	}
	return strings.Join(w.out, " · "), len(w.stack) == 0
}

// cardWalk segueix on és cada token del JSON d'una targeta. key és el camp de
// cada nivell: un text dins d'una llista és del camp que la conté.
type cardWalk struct {
	stack []cardFrame
	out   []string
}

type cardFrame struct {
	obj, wantKey bool
	key          string
}

func (w *cardWalk) delim(d json.Delim) {
	top := len(w.stack) - 1
	if d == '}' || d == ']' {
		if top >= 0 {
			w.stack = w.stack[:top]
		}
		return
	}
	key := ""
	if top >= 0 {
		key = w.stack[top].key
		// L'objecte o la llista és el valor del camp: després ve un altre camp.
		w.stack[top].wantKey = w.stack[top].obj
	}
	w.stack = append(w.stack, cardFrame{obj: d == '{', wantKey: d == '{', key: key})
}

func (w *cardWalk) value(tok json.Token) {
	key := ""
	if top := len(w.stack) - 1; top >= 0 {
		f := &w.stack[top]
		if f.obj && f.wantKey {
			f.key, _ = tok.(string)
			f.wantKey = false
			return
		}
		f.wantKey = f.obj
		key = f.key
	}
	s, ok := tok.(string)
	s = strings.TrimSpace(s)
	if ok && s != "" && !noteCardSkip[key] && !strings.HasPrefix(s, "http://") && !strings.HasPrefix(s, "https://") {
		w.out = append(w.out, s)
	}
}
