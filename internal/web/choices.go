package web

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Les respostes ràpides d'un vent són un bloc
//
//	```choices
//	Primera opció
//	Segona opció
//	```
//
// i l'app en fa botons. En proves repetides, prop d'un 10 % de les respostes
// el model l'escrivia malament i els botons es perdien:
//   - la primera opció a la mateixa línia que «```choices» (la resta bé);
//   - totes les opcions en una sola línia, separades només per espais.
// repairChoicesBlock ho torna a la forma bona quan és segur fer-ho; si no
// s'hi veu clar, ho deixa com està (l'app amaga el bloc igualment).

var choicesInline = regexp.MustCompile("(?s)```choices[ \t]+([^\n]*?)[ \t]*(```[ \t]*)?(\n|$)")

func repairChoicesBlock(text string) string {
	loc := choicesInline.FindStringSubmatchIndex(text)
	if loc == nil {
		return text
	}
	first := strings.TrimSpace(text[loc[2]:loc[3]])
	closedOnLine := loc[4] >= 0
	if first == "" {
		return text
	}
	var opts []string
	if closedOnLine {
		// Tot en una línia: es parteix on comença una opció nova (majúscula,
		// «¿» o «¡» després d'una paraula).
		opts = splitInlineChoices(first)
		if opts == nil {
			return text
		}
	} else {
		opts = []string{first}
	}
	block := "```choices\n" + strings.Join(opts, "\n")
	rest := text[loc[1]:]
	if closedOnLine {
		block += "\n```"
		if strings.HasSuffix(text[loc[0]:loc[1]], "\n") {
			block += "\n"
		}
	} else {
		block += "\n"
	}
	return text[:loc[0]] + block + rest
}

// splitInlineChoices parteix «Opció u Opció dos Opció tres»; nil si el
// resultat no sembla un conjunt de 2-4 opcions curtes.
func splitInlineChoices(line string) []string {
	words := strings.Fields(line)
	var opts []string
	var cur []string
	for i, w := range words {
		r, _ := utf8.DecodeRuneInString(w)
		starts := i > 0 && (unicode.IsUpper(r) || r == '¿' || r == '¡') && len(cur) >= 3
		if starts {
			opts = append(opts, strings.Join(cur, " "))
			cur = nil
		}
		cur = append(cur, w)
	}
	if len(cur) > 0 {
		opts = append(opts, strings.Join(cur, " "))
	}
	if len(opts) < 2 || len(opts) > 4 {
		return nil
	}
	for _, o := range opts {
		if utf8.RuneCountInString(o) > 60 || len(strings.Fields(o)) < 3 {
			return nil
		}
	}
	return opts
}
