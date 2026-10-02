// Package telegram — format.go: conversió de markdown del model a HTML de
// Telegram + truncat segur (sense tallar etiquetes ni entitats).
package telegram

import (
	"regexp"
	"strings"
)

var (
	reFence   = regexp.MustCompile("(?s)```(\\w*)\\n?(.*?)```")
	reInline  = regexp.MustCompile("`([^`\\n]+)`")
	reImage   = regexp.MustCompile(`!\[([^\]]*)\]\(([^)\s]+)\)`)
	reLink    = regexp.MustCompile(`\[([^\]]+)\]\(([^)\s]+)\)`)
	reBold    = regexp.MustCompile(`(\*\*|__)(.+?)(\*\*|__)`)
	reStrike  = regexp.MustCompile(`~~(.+?)~~`)
	reItalic  = regexp.MustCompile("(^|[\\s\\(\\[\"'])\\*([^*\\n]+)\\*([\\s\\.\\,\\;\\:\\!\\?\\)\\]\"']|$)")
	reItalicU = regexp.MustCompile(`(^|[\s\(\["'])_([^_` + "\n" + `]+)_([\s\.\,\;\:\!\?\)\]"']|$)`)
	reHead    = regexp.MustCompile(`(?m)^(#{1,6})\s+(.+)$`)
)

// md2html converteix el markdown habitual del model a l'HTML que entén
// Telegram (<b> <i> <s> <code> <pre> <a> <blockquote>). Tot el text ja ve
// escapat: les marques (* ` [ ]) no les toca esc(), així que es processen
// després d'escapar.
func md2html(src string) string {
	s := esc(src)

	// 1) blocs de codi: placeholders perquè res d'allà dins es toqui.
	type ph struct{ html string }
	var fences []ph
	s = reFence.ReplaceAllStringFunc(s, func(m string) string {
		sub := reFence.FindStringSubmatch(m)
		lang, code := "", ""
		if len(sub) == 3 {
			lang, code = sub[1], strings.Trim(sub[2], "\n")
		}
		var h string
		if lang != "" {
			h = `<pre><code class="language-` + lang + `">` + code + `</code></pre>`
		} else {
			h = `<pre>` + code + `</pre>`
		}
		fences = append(fences, ph{h})
		return "\x00F" + itoa(len(fences)-1) + "\x00"
	})

	// 2) codi en línia.
	var inlines []ph
	s = reInline.ReplaceAllStringFunc(s, func(m string) string {
		inner := reInline.FindStringSubmatch(m)[1]
		inlines = append(inlines, ph{"<code>" + inner + "</code>"})
		return "\x00I" + itoa(len(inlines)-1) + "\x00"
	})

	// 3) imatges i enllaços.
	s = reImage.ReplaceAllString(s, `<a href="$2">$1</a>`)
	s = reLink.ReplaceAllString(s, `<a href="$2">$1</a>`)

	// 4) negreta, tatxat, cursiva.
	s = reBold.ReplaceAllString(s, `<b>$2</b>`)
	s = reStrike.ReplaceAllString(s, `<s>$1</s>`)
	s = reItalic.ReplaceAllString(s, `$1<i>$2</i>$3`)
	s = reItalicU.ReplaceAllString(s, `$1<i>$2</i>$3`)

	// 5) titulars, cites i llistes, línia a línia. Les cites consecutives van en
	// un sol <blockquote> (una caixa, no una per línia); les numerades
	// (`1.` o `1)`) es normalitzen a `1.` perquè no semblin text trencat.
	var reNum = regexp.MustCompile(`^(\s*)(\d+)[.)](\s+)`)
	lins := strings.Split(s, "\n")
	var junt []string
	enCita := false
	tancaCita := func() {
		if enCita {
			junt[len(junt)-1] += "</blockquote>"
			enCita = false
		}
	}
	for _, l := range lins {
		t := strings.TrimRight(l, " 	")
		if m := reHead.FindStringSubmatch(t); m != nil {
			tancaCita()
			t = `<b>` + m[2] + `</b>`
		} else if idx := citaInici(t); idx >= 0 {
			inner := strings.TrimSpace(t[idx+4:])
			if !enCita {
				t = "<blockquote>" + inner
				enCita = true
			} else {
				t = inner
			}
		} else {
			tancaCita()
			trim := strings.TrimLeft(t, " 	")
			if strings.HasPrefix(trim, "- ") || strings.HasPrefix(trim, "* ") {
				indent := t[:len(t)-len(trim)]
				t = indent + "• " + trim[2:]
			} else if reNum.MatchString(t) {
				// `1)` → `1.`: el número mana, el parèntesi era soroll.
				t = reNum.ReplaceAllString(t, `${1}${2}.${3}`)
			}
		}
		junt = append(junt, t)
	}
	tancaCita()
	s = strings.Join(junt, "\n")

	// 5b) taules en <pre> (monoespaiat): després del format en línia i abans
	// de restaurar el codi.
	s = embolicTaules(s)

	// 6) restaura els placeholders (el codi ja estava escapat).
	for i, p := range inlines {
		s = strings.ReplaceAll(s, "\x00I"+itoa(i)+"\x00", p.html)
	}
	for i, p := range fences {
		s = strings.ReplaceAll(s, "\x00F"+itoa(i)+"\x00", p.html)
	}
	return s
}

// citaInici torna l'índex on comença la marca de cita (`&gt;`, ja escapada)
// o -1 si la línia no és cita. Respecta la sagnia: `  > x` cita igual.
func citaInici(t string) int {
	trim := strings.TrimLeft(t, " \t")
	if !strings.HasPrefix(trim, "&gt;") {
		return -1
	}
	return strings.Index(t, "&gt;")
}

// esFilaTaula diu si la línia és fila de taula markdown: comença amb `|` i
// té com a mínim una altra barra (les línies normals amb un sol `|` no).
func esFilaTaula(t string) bool {
	trim := strings.TrimSpace(t)
	return strings.HasPrefix(trim, "|") && strings.Contains(trim[1:], "|")
}

// embolicTaules posa en <pre> els blocs de 2+ files de taula: el monoespaiat
// conserva les columnes que el model ja ha alineat. Va després del format en
// línia (les cel·les ja porten <b>/<a>/<code>, vàlids dins de <pre>) i abans
// de restaurar el codi (els placeholders viatgen dins del <pre> sense
// problemes).
func embolicTaules(s string) string {
	lins := strings.Split(s, "\n")
	var fora []string
	i := 0
	for i < len(lins) {
		if !esFilaTaula(lins[i]) {
			fora = append(fora, lins[i])
			i++
			continue
		}
		j := i
		for j < len(lins) && esFilaTaula(lins[j]) {
			j++
		}
		if j-i < 2 {
			fora = append(fora, lins[i])
			i++
			continue
		}
		fora = append(fora, "<pre>"+strings.Join(lins[i:j], "\n")+"</pre>")
		i = j
	}
	return strings.Join(fora, "\n")
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [8]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

// closeOpenTags tanca les etiquetes que el retall hagi deixat obertes
// (Telegram rebutja el missatge si l'HTML no està ben format).
func closeOpenTags(s string) string {
	reTag := regexp.MustCompile(`</?(b|i|s|code|pre|a|blockquote)[^>]*>`)
	reClose := regexp.MustCompile(`^</`)
	var stack []string
	for _, m := range reTag.FindAllString(s, -1) {
		if reClose.MatchString(m) {
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
			continue
		}
		name := m[1:strings.IndexAny(m, " >")]
		if name == "br" {
			continue
		}
		stack = append(stack, name)
	}
	for i := len(stack) - 1; i >= 0; i-- {
		s += "</" + stack[i] + ">"
	}
	return s
}

// truncHTML retalla sense partir etiquetes (<...>) ni entitats (&...;),
// i tanca les etiquetes que quedin obertes.
func truncHTML(s string, max int) string {
	if len([]rune(s)) <= max {
		return s
	}
	runes := []rune(s)
	cut := max
	inTag, inEnt := false, false
	for i, r := range runes {
		switch {
		case r == '<':
			inTag = true
		case r == '>':
			inTag = false
		case r == '&' && !inTag:
			inEnt = true
		case r == ';' && inEnt:
			inEnt = false
		}
		if i == max {
			cut = i
			break
		}
	}
	// Si quedem dins d'una etiqueta o entitat, retrocedeix al límit segur.
	if inTag || inEnt {
		for cut > 0 && runes[cut-1] != '<' && runes[cut-1] != '&' && runes[cut-1] != '\n' {
			cut--
		}
		for cut > 0 && (runes[cut-1] == '<' || runes[cut-1] == '&') {
			cut--
		}
	}
	return closeOpenTags(strings.TrimRight(string(runes[:cut]), " 	\n") + "…")
}

// renderAnswer prepara la resposta del model per Telegram: markdown → HTML i
// retall segur a 3900 caràcters (marge sota el límit de 4096). Si hi ha una
// cerca de codi sense tancar (típic a mitja resposta en streaming), la tanca
// perquè es pinti com a codi en comptes de cridar ``` crus.
func renderAnswer(s string) string {
	if strings.Count(s, "```")%2 == 1 {
		s += "\n```"
	}
	return truncHTML(md2html(s), 3900)
}
