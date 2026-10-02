package tui

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// Animacions de la marea. La identitat del TUI és el mar (escuma, onada,
// arena), així que el que es mou són onades i no pas spinners genèrics.
//
// Tot són funcions pures de (frame, amplada): View no hi guarda estat ni fa
// I/O, i es poden provar sense arrencar el programa. El frame és m.spin, que
// avança un tick cada 60 ms amb feina i cada 500 ms en repòs: per això les
// animacions vistoses només s'encenen quan hi ha alguna cosa a dir. En repòs
// el TUI es queda quiet a posta, que és una eina de treball i no una demo.

// tideCrest són els glifs de l'onada segons com de lluny queden del cim:
// cim, vessant, cua i escuma. Omplir tot el buit de patró repetit semblava
// soroll; una sola onada que passa es llegeix com el que és.
var tideCrest = []rune{'≋', '≈', '~', '·'}

// tideMargin són les columnes de calma abans i després del recorregut,
// perquè entre onada i onada hi hagi mar plana i no un desfilar continu.
const tideMargin = 6

// swellLevels són densitats de la mar grossa: de calma a cresta. Un sinus
// continu es llegeix com aigua en moviment. Els blocs (░▒▓█) que hi havia
// abans es veien com una franja tramada, no com aigua: a Windows Terminal
// semblaven caràcters trencats. Ara la cresta és el mateix glif que el
// logo i la calma és espai: la mar passa per darrere del text.
var swellLevels = []rune("  ·~≈≋")

// swellWave és la longitud d'ona en columnes: prou llarga per ser calmada,
// prou curta per veure-la viatjar en capçaleres estretes.
const swellWave = 14

// swellLine torna width columnes amb una mar contínua que viatja d'esquerra
// a dreta. Sempre fa exactament width cel·les (mateix contracte que
// tideLine): la fa servir la capçalera per omplir el buit mentre hi ha
// feina. Sense marges morts: l'aigua no s'atura entre onada i onada.
func swellLine(width, frame int) string {
	if width <= 0 {
		return ""
	}
	out := make([]rune, width)
	n := float64(len(swellLevels))
	for i := range out {
		// Fase entera (0..swellWave-1): el cicle tanca exacte, sense que
		// la coma flotant trenqui el 2π en una frontera d'arrodoniment.
		m := (i - frame) % swellWave
		if m < 0 {
			m += swellWave
		}
		// 0..1: 0 = calma, 1 = cresta.
		v := (math.Sin(2*math.Pi*float64(m)/swellWave) + 1) / 2
		out[i] = swellLevels[int(v*(n-1)+0.5)]
	}
	return string(out)
}

// tideLine torna width columnes amb una onada que viatja d'esquerra a dreta.
// Sempre fa exactament width cel·les. De moment només la fan servir els
// tests: la capçalera omple el buit amb la mar contínua (swellLine).
func tideLine(width, frame int) string {
	if width <= 0 {
		return ""
	}
	period := width + 2*tideMargin
	center := frame%period - tideMargin
	out := make([]rune, width)
	for i := range out {
		d := i - center
		if d < 0 {
			d = -d
		}
		if d < len(tideCrest) {
			out[i] = tideCrest[d]
		} else {
			out[i] = ' '
		}
	}
	return string(out)
}

// shimmer passa una cresta d'escuma per sobre del text, d'esquerra a dreta i
// tornant a començar després d'una pausa. Agrupa les runes per estil perquè
// la sortida no sigui una escapada ANSI per lletra.
func shimmer(text string, frame int, base, hi lipgloss.Style) string {
	r := []rune(text)
	if len(r) == 0 {
		return ""
	}
	// La pausa (les columnes de més) fa que la cresta no sigui un estrobo.
	period := len(r) + 10
	center := frame % period
	var b strings.Builder
	start, cur := 0, crestAt(0, center)
	for i := 1; i <= len(r); i++ {
		at := i < len(r) && crestAt(i, center)
		if i == len(r) || at != cur {
			seg := string(r[start:i])
			if cur {
				b.WriteString(hi.Render(seg))
			} else {
				b.WriteString(base.Render(seg))
			}
			start, cur = i, at
		}
	}
	return b.String()
}

// crestAt diu si la columna i cau sota la cresta centrada a center.
func crestAt(i, center int) bool {
	d := i - center
	return d >= -1 && d <= 1
}

// gaugeCells són els vuitens de bloc: donen resolució sub-cel·la al mesurador
// perquè el context es vegi pujar de mica en mica i no a salts d'una columna.
var gaugeCells = []rune("▏▎▍▌▋▊▉█")

// tideGauge pinta pct (0–100) en width columnes. Torna el text sense estil:
// el color del llindar ja el decideix qui el crida, que és on viuen els
// llindars de context. Per sobre del 80% l'última cel·la plena batega, que
// és l'avís que de seguida caldrà compactar.
func tideGauge(pct, width, frame int) string {
	if width <= 0 {
		return ""
	}
	pct = min(max(pct, 0), 100)
	eighths := pct * width * 8 / 100
	full := eighths / 8
	rest := eighths % 8
	var b strings.Builder
	for range full {
		b.WriteRune('█')
	}
	if full < width {
		if rest > 0 {
			b.WriteRune(gaugeCells[rest-1])
		} else {
			b.WriteRune('·')
		}
		for range width - full - 1 {
			b.WriteRune('·')
		}
	}
	out := []rune(b.String())
	if pct >= 80 && full > 0 && frame%2 == 1 {
		out[full-1] = '▓' // batec: ple → mig ple → ple
	}
	return string(out)
}

// elapsedShort dona la durada del torn en curt: "7s", "2m04s", "1h05m". Que
// es vegi quant fa que l'agent hi és treballant treu l'angoixa de no saber
// si s'ha penjat.
func elapsedShort(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	s := int(d.Seconds())
	switch {
	case s < 60:
		return fmt.Sprintf("%ds", s)
	case s < 3600:
		return fmt.Sprintf("%dm%02ds", s/60, s%60)
	default:
		return fmt.Sprintf("%dh%02dm", s/3600, (s%3600)/60)
	}
}

// shorten retalla s a n runes posant "…" al final. Per a noms de projecte i
// branques que no caben a la capçalera.
func shorten(s string, n int) string {
	r := []rune(s)
	if n <= 1 || len(r) <= n {
		if len(r) <= n {
			return s
		}
		return "…"
	}
	return string(r[:n-1]) + "…"
}
