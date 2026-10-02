package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// La marea ha de fer exactament les columnes demanades: la capçalera l'usa
// per omplir un buit calculat, i una de més trencaria la línia.
func TestTideLineAmplada(t *testing.T) {
	for _, w := range []int{1, 2, 7, 11, 12, 40, 137} {
		for _, f := range []int{0, 1, 5, 10, 11, 99} {
			if got := lipgloss.Width(tideLine(w, f)); got != w {
				t.Fatalf("tideLine(%d,%d) fa %d columnes", w, f, got)
			}
		}
	}
	if tideLine(0, 3) != "" || tideLine(-5, 3) != "" {
		t.Fatal("sense amplada no hi ha onada")
	}
}

// I s'ha de moure: al llarg d'un període l'onada ha de recórrer el buit de
// punta a punta. (Frames seguits poden sortir iguals: entre onada i onada la
// mar és plana a posta.)
func TestTideLineEsMou(t *testing.T) {
	const w = 20
	primera, ultima := -1, -1
	for f := range w + 2*tideMargin {
		r := []rune(tideLine(w, f))
		if r[0] != ' ' {
			primera = f
		}
		if r[w-1] != ' ' {
			ultima = f
		}
	}
	if primera < 0 || ultima < 0 {
		t.Fatalf("l'onada no toca les dues puntes (primera=%d última=%d)", primera, ultima)
	}
	if primera >= ultima {
		t.Fatal("l'onada ha d'anar d'esquerra a dreta")
	}
	// Torna al punt de partida en un període: és un cicle, no una deriva.
	if tideLine(20, 0) != tideLine(20, 20+2*tideMargin) {
		t.Fatal("l'onada hauria de tancar el cicle")
	}
	// I hi ha mar plana entre onada i onada: a mig període, cap glif.
	if got := tideLine(20, (20+2*tideMargin)/2+14); strings.TrimSpace(got) != "" {
		t.Fatalf("entre onades el buit ha de ser buit: %q", got)
	}
}

// La mar grossa fa exactament les columnes demanades (mateix contracte).
func TestSwellLineAmplada(t *testing.T) {
	for _, w := range []int{1, 2, 6, 7, 11, 40, 137} {
		for _, f := range []int{0, 1, 5, 13, 99} {
			if got := lipgloss.Width(swellLine(w, f)); got != w {
				t.Fatalf("swellLine(%d,%d) fa %d columnes", w, f, got)
			}
		}
	}
	if swellLine(0, 3) != "" || swellLine(-5, 3) != "" {
		t.Fatal("sense amplada no hi ha mar")
	}
}

// I viatja: tanca el cicle cada swellWave frames i només usa densitats.
func TestSwellLineViatja(t *testing.T) {
	const w = 30
	if swellLine(w, 0) != swellLine(w, swellWave) {
		t.Fatal("la mar hauria de tancar el cicle")
	}
	if swellLine(w, 0) == swellLine(w, 3) {
		t.Fatal("la mar s'ha de moure entre frames")
	}
	allowed := map[rune]bool{}
	for _, r := range swellLevels {
		allowed[r] = true
	}
	seen := map[rune]bool{}
	for f := range swellWave {
		for _, r := range swellLine(w, f) {
			if !allowed[r] {
				t.Fatalf("glif %q fora de swellLevels", r)
			}
			seen[r] = true
		}
	}
	// En un cicle sencer hi ha d'haver calma i cresta: si no, és plana.
	if !seen[swellLevels[0]] || !seen[swellLevels[len(swellLevels)-1]] {
		var keys []string
		for r := range seen {
			keys = append(keys, string(r))
		}
		t.Fatalf("la mar no respira (vistos: %q)", strings.Join(keys, ""))
	}
}

// shimmer no pot alterar el text, només com es pinta.
func TestShimmerConservaElText(t *testing.T) {
	base := lipgloss.NewStyle()
	hi := lipgloss.NewStyle().Bold(true)
	const text = "TREBALLANT"
	for f := range 40 {
		got := shimmer(text, f, base, hi)
		if stripANSI(got) != text {
			t.Fatalf("frame %d: text=%q", f, stripANSI(got))
		}
	}
	if shimmer("", 3, base, hi) != "" {
		t.Fatal("text buit, sortida buida")
	}
}

// La cresta passa: en algun moment il·lumina el principi i en un altre el
// final, i hi ha frames de pausa on no il·lumina res.
func TestShimmerLaCrestaViatja(t *testing.T) {
	base := lipgloss.NewStyle().Foreground(lipgloss.Color("#111111"))
	hi := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF"))
	r := []rune("TREBALLANT")
	vistos := map[int]bool{}
	for f := range len(r) + 10 {
		for i := range r {
			if crestAt(i, f%(len(r)+10)) {
				vistos[i] = true
			}
		}
	}
	if !vistos[0] || !vistos[len(r)-1] {
		t.Fatalf("la cresta no recorre tot el text: %v", vistos)
	}
	// Amb accents el tall és per runes, no per bytes.
	if got := stripANSI(shimmer("GRÀCIES", 3, base, hi)); got != "GRÀCIES" {
		t.Fatalf("multibyte trencat: %q", got)
	}
}

func TestTideGauge(t *testing.T) {
	for _, w := range []int{1, 4, 8, 20} {
		for pct := 0; pct <= 100; pct += 7 {
			if got := lipgloss.Width(tideGauge(pct, w, 0)); got != w {
				t.Fatalf("tideGauge(%d,%d) fa %d columnes", pct, w, got)
			}
		}
	}
	if tideGauge(0, 8, 0) != "········" {
		t.Fatalf("buit=%q", tideGauge(0, 8, 0))
	}
	if tideGauge(100, 8, 0) != "████████" {
		t.Fatalf("ple=%q", tideGauge(100, 8, 0))
	}
	// Fora de rang no peta ni se surt de l'amplada.
	if lipgloss.Width(tideGauge(-20, 8, 0)) != 8 || lipgloss.Width(tideGauge(400, 8, 0)) != 8 {
		t.Fatal("pct fora de rang")
	}
	if tideGauge(50, 0, 0) != "" {
		t.Fatal("sense amplada no hi ha barra")
	}
}

// Per sota del llindar la barra està quieta; per sobre, batega.
func TestTideGaugeBategaNomesAlFinal(t *testing.T) {
	if tideGauge(50, 8, 0) != tideGauge(50, 8, 1) {
		t.Fatal("al 50% la barra no ha de bategar")
	}
	if tideGauge(90, 8, 0) == tideGauge(90, 8, 1) {
		t.Fatal("al 90% la barra ha de bategar")
	}
}

func TestElapsedShort(t *testing.T) {
	casos := map[time.Duration]string{
		0:                            "0s",
		7 * time.Second:              "7s",
		59 * time.Second:             "59s",
		60 * time.Second:             "1m00s",
		124 * time.Second:            "2m04s",
		time.Hour + 5*time.Minute:    "1h05m",
		-3 * time.Second:             "0s",
		3*time.Hour + 59*time.Minute: "3h59m",
	}
	for d, want := range casos {
		if got := elapsedShort(d); got != want {
			t.Errorf("elapsedShort(%v) = %q, volia %q", d, got, want)
		}
	}
}

// La marea de la capçalera omple un buit calculat: si se n'anés una columna,
// la línia trencaria. Es comprova a amplades i frames variats, amb feina i
// sense, que és on el càlcul canvia.
func TestCapçaleraNoCreixAmbLaMarea(t *testing.T) {
	for _, w := range []int{60, 80, 96, 110, 160} {
		m := planTestModel(t)
		mod, _ := m.Update(tea.WindowSizeMsg{Width: w, Height: 32})
		m = mod.(Model)
		for _, busy := range []bool{false, true} {
			m.busy = busy
			for _, f := range []int{0, 1, 3, 7, 12, 41} {
				m.spin = f
				if got := lipgloss.Width(m.headerLine()); got > w {
					t.Fatalf("amplada %d, busy=%v, frame %d: capçalera de %d columnes", w, busy, f, got)
				}
			}
		}
	}
}

// La capçalera es mou només amb feina: en repòs, mar plana (espais);
// treballant, el buit porta la mar que viatja. Abans no s'animava mai.
func TestCapçaleraEsMouNomesAmbFeina(t *testing.T) {
	m := planTestModel(t)
	m.busy = false
	m.agentActive = false
	for _, f := range []int{0, 1, 2, 3, 7} {
		m.spin = f
		plain := stripANSI(m.headerLine())
		if !strings.Contains(plain, "          ") {
			t.Fatalf("en repòs el buit han de ser espais: %q", plain)
		}
		// La marca Ventet queda fixa; cap onada no es mou en repòs.
		if strings.Count(plain, "‹••›") != 1 || strings.ContainsAny(plain, "~≈≋") {
			t.Fatalf("en repòs cap onada a la capçalera: %q", plain)
		}
	}
	// Amb feina però sense animacions demanades, la capçalera segueix
	// quieta: una eina on es passen hores no pot tenir res que es mogui
	// sol. La mar només surt amb `animacions: on`.
	m.busy = true
	m.spin = 7
	if plain := stripANSI(m.headerLine()); strings.ContainsAny(plain, "~≈") {
		t.Fatalf("sense animacions no hi ha d'haver mar ni amb feina: %q", plain)
	}
	m.animacions = true
	vistos := map[string]bool{}
	for _, f := range []int{0, 3, 7, 12, 20} {
		m.spin = f
		plain := stripANSI(m.headerLine())
		if lipgloss.Width(plain) != m.vp.Width {
			t.Fatalf("frame %d: la capçalera fa %d, volia %d", f, lipgloss.Width(plain), m.vp.Width)
		}
		if !strings.ContainsAny(plain, "~≈≋") {
			t.Fatalf("amb feina hi ha d'haver mar: %q", plain)
		}
		vistos[plain] = true
	}
	if len(vistos) < 2 {
		t.Fatal("amb feina la capçalera s'ha de moure entre frames")
	}
}

// I amb feina, el composer sí que es mou i diu què passa.
func TestComposerDiuQuePassa(t *testing.T) {
	m := planTestModel(t)
	m.busy = true
	m.status = "agent pas 2/40 · read internal/tui/app.go"
	m.spin = 0
	a := stripANSI(m.inputBox())
	m.spin = 1
	b := stripANSI(m.inputBox())
	if a == b {
		t.Fatal("amb feina el composer ha de canviar entre frames (el spinner)")
	}
	if !strings.Contains(a, "read internal/tui/app.go") {
		t.Fatalf("el composer ha de dir què fa, no «TREBALLANT»: %q", a)
	}
}

// El rellotge del torn el porta el tick, no els vint llocs que posen busy.
func TestRellotgeDelTorn(t *testing.T) {
	m := planTestModel(t)
	if !m.turnStart.IsZero() {
		t.Fatal("en repòs no hi ha torn")
	}
	m.busy = true
	mod, _ := m.Update(streamTickMsg{})
	m = mod.(Model)
	if m.turnStart.IsZero() {
		t.Fatal("el tick havia d'engegar el rellotge")
	}
	abans := m.turnStart
	mod, _ = m.Update(streamTickMsg{})
	m = mod.(Model)
	if !m.turnStart.Equal(abans) {
		t.Fatal("el rellotge no es pot reiniciar a cada tick")
	}
	m.busy = false
	mod, _ = m.Update(streamTickMsg{})
	if !mod.(Model).turnStart.IsZero() {
		t.Fatal("en acabar el torn el rellotge s'atura")
	}
}

// La benvinguda fa sis files i no porta caixa: marca amb el vent, una
// frase, tres exemples i les tecles. Els modes s'expliquen al selector de
// mode i a ?, no aquí: abans era una targeta de tretze files que ocupava
// mitja pantalla i no marxava mai.
func TestBenvingudaCurtaISenseCaixa(t *testing.T) {
	got := stripANSI(welcome("chat", 110))
	if n := strings.Count(got, "\n") + 1; n > 6 {
		t.Fatalf("la benvinguda fa %d files, màxim 6:\n%s", n, got)
	}
	for _, want := range []string{"GREGAL", "[45° NE]", "? ajuda", "Shift+Tab"} {
		if !strings.Contains(got, want) {
			t.Fatalf("la benvinguda no diu %q:\n%s", want, got)
		}
	}
	for _, no := range []string{"╭", "│", "CONSULTA", "OBJECTIU"} {
		if strings.Contains(got, no) {
			t.Fatalf("la benvinguda no porta %q (ni caixa ni llista de modes):\n%s", no, got)
		}
	}
	// I els modes s'expliquen on es trien.
	var modes []string
	for _, it := range modeMenu().items {
		modes = append(modes, it.text)
	}
	tot := strings.Join(modes, "\n")
	for _, want := range []string{"CODE", "CONSULTA", "OBJECTIU", "llegeix", "explora"} {
		if !strings.Contains(tot, want) {
			t.Fatalf("el selector de mode ha d'explicar %q:\n%s", want, tot)
		}
	}
}

// La línia d'onada fa l'amplada demanada i s'esmorteeix cap als extrems: al
// centre hi ha cim i a les vores, calma.
func TestWaveLineEsmorteida(t *testing.T) {
	for _, w := range []int{4, 10, 33, 110} {
		got := stripANSI(waveLine(w))
		if lipgloss.Width(got) != w {
			t.Fatalf("waveLine(%d) fa %d columnes", w, lipgloss.Width(got))
		}
		r := []rune(got)
		if r[len(r)/2] != '≋' {
			t.Fatalf("waveLine(%d): el centre hauria de ser cim, és %q", w, r[len(r)/2])
		}
	}
	if strings.Contains(stripANSI(waveLine(110)), "≋≋≋≋≋≋≋≋≋≋≋≋≋≋≋≋≋≋≋≋≋≋≋≋≋≋≋≋≋≋≋≋≋≋≋≋≋≋≋≋≋≋≋≋≋≋≋≋≋≋≋≋≋≋≋≋") {
		t.Fatal("a amplada completa la línia no pot ser una barra massissa")
	}
	if waveLine(3) != "" {
		t.Fatal("massa estret, cap línia")
	}
}
