package tools

// Markdown lleuger cap a documents office.
//
// El model escriu markdown per defecte i el .docx/.pptx el mostrava tal
// qual (##, **, -). Les descripcions de les eines ja demanen text pla,
// però el que s'escapa no pot embrutar el document: aquí es converteix
// el subset útil (títols #/##/###, llistes -, **negreta**, *cursiva*,
// `codi`, [enllaç](url), taules simples) i la resta es neteja.
//
// Disseny deliberadament curt: sense taules Word reals (el docx mínim no
// té styles.xml), sense negreta niuada, sense HTML. El que no s'entén es
// deixa tal qual en comptes d'inventar.

import (
	"regexp"
	"strings"
)

// richRun és un tram de text amb format per a docx (<w:r>) o pptx (<a:r>).
type richRun struct {
	text         string
	bold, italic bool
}

// richLine és una línia lògica llesta per emetre.
type richLine struct {
	runs   []richRun
	size   string // "" = normal; si no, mida de títol
	bullet bool   // element de llista (-, *)
	skip   bool   // línia que no s'emet (tancaula, separador de taula)
}

var (
	reMdLink   = regexp.MustCompile(`\[([^\]\n]+)\]\(([^)\n]+)\)`)
	reMdHead   = regexp.MustCompile(`^(#{1,6})\s+(.+)$`)
	reMdBul    = regexp.MustCompile(`^[-*]\s+(.+)$`)
	reMdNum    = regexp.MustCompile(`^\d+[.)]\s+(.+)$`)
	reMdQuote  = regexp.MustCompile(`^>\s?(.*)$`)
	reMdHR     = regexp.MustCompile(`^(-{3,}|\*{3,}|_{3,})\s*$`)
	reMdFence  = regexp.MustCompile("^```")
	reMdInline = regexp.MustCompile("`([^`\\n]+)`|\\*\\*([^*\\n]+)\\*\\*|\\*([^*\\n]+)\\*")
)

// parteixRuns separa **negreta**, *cursiva* i `codi` (que queda pla).
// Els marcadors sense parella queden literals: no es menja text.
func parteixRuns(s string) []richRun {
	var out []richRun
	// Alternança ordenada: codi primer perquè el ** d'allà dins no compti.
	rest := s
	for len(rest) > 0 {
		loc := reMdInline.FindStringSubmatchIndex(rest)
		if loc == nil {
			out = append(out, richRun{text: rest})
			break
		}
		if loc[0] > 0 {
			out = append(out, richRun{text: rest[:loc[0]]})
		}
		switch {
		case loc[2] >= 0:
			out = append(out, richRun{text: rest[loc[2]:loc[3]]})
		case loc[4] >= 0:
			out = append(out, richRun{text: rest[loc[4]:loc[5]], bold: true})
		default:
			out = append(out, richRun{text: rest[loc[6]:loc[7]], italic: true})
		}
		rest = rest[loc[1]:]
	}
	var net []richRun
	for _, r := range out {
		if r.text != "" {
			net = append(net, r)
		}
	}
	if net == nil {
		return []richRun{{}}
	}
	return net
}

// senseLinks treu [text](url) → text.
func senseLinks(s string) string {
	return reMdLink.ReplaceAllString(s, "$1")
}

// parseRichLine converteix una línia markdown en línia rica.
func parseRichLine(s string) richLine {
	t := strings.TrimSpace(s)
	if t == "" {
		return richLine{}
	}
	// Taula: | a | b | → "a · b"; separador |---| → skip.
	if strings.HasPrefix(t, "|") && strings.HasSuffix(t, "|") {
		var cells []string
		separador := true
		for _, c := range strings.Split(strings.Trim(t, "|"), "|") {
			c = strings.TrimSpace(c)
			if c == "" {
				continue
			}
			if strings.Trim(c, "-:") == "" {
				continue
			}
			separador = false
			cells = append(cells, c)
		}
		if separador || len(cells) == 0 {
			return richLine{skip: true}
		}
		return richLine{runs: parteixRuns(strings.Join(cells, " · "))}
	}
	if reMdHR.MatchString(t) {
		return richLine{}
	}
	if m := reMdHead.FindStringSubmatch(t); m != nil {
		mida := "26"
		switch len(m[1]) {
		case 1:
			mida = "30"
		case 2:
			mida = "28"
		}
		return richLine{runs: parteixRuns(senseLinks(m[2])), size: mida}
	}
	if m := reMdBul.FindStringSubmatch(t); m != nil {
		return richLine{runs: parteixRuns(senseLinks(m[1])), bullet: true}
	}
	if m := reMdNum.FindStringSubmatch(t); m != nil {
		return richLine{runs: parteixRuns(senseLinks(m[1]))}
	}
	if m := reMdQuote.FindStringSubmatch(t); m != nil {
		return richLine{runs: parteixRuns(senseLinks(m[1]))}
	}
	return richLine{runs: parteixRuns(senseLinks(t))}
}

// trauMarkdownPla neteja markdown a text pla (substitucions, cel·les,
// títols): sense format, perquè allà el format ja el posa el context
// (paràgraf existent, cel·la). El codi entre tanques ``` queda literal.
func trauMarkdownPla(s string) string {
	var lins []string
	enCodi := false
	for _, ln := range strings.Split(s, "\n") {
		t := strings.TrimSpace(ln)
		if reMdFence.MatchString(t) {
			enCodi = !enCodi
			continue
		}
		if enCodi {
			lins = append(lins, ln)
			continue
		}
		if t == "" {
			lins = append(lins, "")
			continue
		}
		rl := parseRichLine(t)
		if rl.skip {
			continue
		}
		var b strings.Builder
		if rl.bullet {
			b.WriteString("• ")
		}
		for _, r := range rl.runs {
			b.WriteString(r.text)
		}
		lins = append(lins, b.String())
	}
	return strings.Join(lins, "\n")
}
