package companion

import (
	"strings"
	"unicode"
)

// DetectLang endevina l'idioma d'un missatge curt de l'usuari: "ca", "es",
// "en" o "" si no n'hi ha prou per dir-ho. És heurístic (paraules funcionals
// que no comparteixen) i prudent: davant del dubte, "". La mateixa lògica que
// scripts/style-rules.cjs fa servir per avaluar les respostes.
func DetectLang(text string) string {
	words := strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return !unicode.IsLetter(r) && r != '\'' && r != '’' && r != '·'
	})
	if len(words) < 3 {
		return ""
	}
	score := map[string]int{}
	for _, w := range words {
		for lang, set := range langWords {
			if set[w] {
				score[lang]++
			}
		}
		switch {
		case strings.ContainsAny(w, "ñ"), strings.HasSuffix(w, "ción"):
			score["es"] += 2
		case strings.Contains(w, "·l"), strings.HasSuffix(w, "-ho"), strings.HasSuffix(w, "-hi"):
			score["ca"] += 2
		case len(w) > 2 && (strings.HasPrefix(w, "l'") || strings.HasPrefix(w, "d'") || strings.HasPrefix(w, "l’") || strings.HasPrefix(w, "d’")):
			score["ca"]++
		}
	}
	if strings.ContainsAny(text, "¿¡") {
		score["es"] += 2
	}
	best, second := "", 0
	bestN := 0
	for _, lang := range []string{"ca", "es", "en"} {
		n := score[lang]
		if n > bestN {
			second, best, bestN = bestN, lang, n
		} else if n > second {
			second = n
		}
	}
	if bestN < 2 || bestN <= second {
		return ""
	}
	return best
}

var langWords = map[string]map[string]bool{
	"ca": set("és amb per els les què però també això molt fer avui tens pots vull aquí perquè doncs ara m'ho digue'm digues com quin quina quins on podem podries ens hem som feu"),
	"es": set("con para los qué pero también esto muy hacer hoy tienes puedes quiero porque pues ahora y cómo cuál dónde podemos podrías nos hemos somos hacéis está"),
	"en": set("the and with for you is are this that what but also very today can here because now it of how which where we could us have do does your my what's it's i'm don't to be was were will would should there their they about some from"),
}

func set(words string) map[string]bool {
	m := map[string]bool{}
	for _, w := range strings.Fields(words) {
		m[w] = true
	}
	return m
}

var langNames = map[string]string{"ca": "Catalan", "es": "Spanish", "en": "English"}

// ReplyLanguageBrief fixa l'idioma de la resposta segons l'últim missatge de
// l'usuari. Sense això, en una conversa llarga en català un missatge en
// anglès rebia la resposta en català (el model segueix l'historial, no l'últim
// missatge). Va al final de la consigna: és l'última cosa que llegeix.
func ReplyLanguageBrief(msg string) string {
	lang := DetectLang(msg)
	if lang == "" {
		return ""
	}
	name := langNames[lang]
	return "\n\nLANGUAGE: the user's last message is in " + name + ". Write your whole reply in " + name + " — every member's lines in a group and the quick-reply options too — even if earlier messages were in another language. Titles of books, songs or places keep their original language."
}
