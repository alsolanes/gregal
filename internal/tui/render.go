package tui

import (
	"regexp"
	"strings"

	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/glamour/ansi"

	"gregal/internal/tema"
)

// renderMD pinta markdown per terminal (codi amb colors, capçaleres, llistes).
// Estil dark explícit (no AutoStyle: aquest pregunta al terminal amb DSR i
// la resposta pot colar-se a l'input com a "[n;mR"). width<=0 = sense wrap.
func renderMD(text string, width int) string {
	if strings.TrimSpace(text) == "" {
		return text
	}
	opts := []glamour.TermRendererOption{
		glamour.WithStyles(estilMD()),
	}
	if width > 20 {
		opts = append(opts, glamour.WithWordWrap(width))
	} else {
		opts = append(opts, glamour.WithWordWrap(0))
	}
	r, err := glamour.NewTermRenderer(opts...)
	if err != nil {
		return text
	}
	out, err := r.Render(text)
	if err != nil {
		return text
	}
	return trimTrailing(strings.TrimSuffix(out, "\n"))
}

// estilMD és l'estil de markdown del tema actiu (internal/tema): títols
// en arena, codi amb la paleta de Gregal i, sobretot, un estil per a
// cada tema. Abans era styles.DarkStyleConfig tal qual, amb els colors
// d'una altra marca, i en tema clar no canviava.
func estilMD() ansi.StyleConfig {
	return tema.EstilGlamour(TemaActiu())
}

// trailPad casa la cua d'emplenat de glamour: espais i codis de color
// barrejats fins al final de línia. No n'hi ha prou amb TrimRight(" "),
// perquè cada espai va embolcallat en el seu propi parell d'escapades i la
// línia acaba en "\x1b[0m", no en espai.
var trailPad = regexp.MustCompile(`(?:\x1b\[[0-9;]*m|[ \t])+$`)

// trimTrailing treu aquesta cua. Glamour omple cada línia fins a l'amplada
// amb espais pintats: en un terminal fosc són un bloc de fons que fa que la
// resposta sembli una taula, i qualsevol còpia del text se'ls endú.
func trimTrailing(s string) string {
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		t := trailPad.ReplaceAllString(l, "")
		if t != l && t != "" {
			t += "\x1b[0m" // tanquem l'estil que hem retallat
		}
		lines[i] = t
	}
	return strings.Join(lines, "\n")
}
