package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"gregal/internal/agent"
	"gregal/internal/llm"
)

func TestDebugInputBoxHeight(t *testing.T) {
	ti := textarea.New()
	ti.Prompt = "❯ "
	ti.ShowLineNumbers = false
	ti.SetHeight(1)
	ti.Focus()
	ti.SetValue("d")
	lines := strings.Split(ti.View(), "\n")
	t.Logf("FOCUSED ti.View(): (lines=%d)", len(lines))
	for i, l := range lines {
		t.Logf("ti line %d: %q", i, l)
	}
}

// TestInputBoxHeightStable verifica que l'alçada de l'inputBox no salta quan
// s'escriu: cada tecla ha de mantenir la mateixa alçada visual si no hi ha
// salt de línia (\\n) explícit.
func TestInputBoxHeightStable(t *testing.T) {
	m := planTestModel(t)
	mod, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 30})
	m = mod.(Model)

	// Estat base: composer buit
	baseIB := m.inputBox()
	baseH := lipgloss.Height(baseIB)
	t.Logf("buit: inputBox height = %d", baseH)
	for _, l := range strings.Split(baseIB, "\n") {
		t.Logf("  %q", l)
	}

	// Teclejar un caràcter
	m.input.SetValue("h")
	m.fitInput()
	ib1 := m.inputBox()
	h1 := lipgloss.Height(ib1)
	t.Logf("1 char: inputBox height = %d", h1)
	if h1 != baseH {
		t.Errorf("inputBox height changed from %d to %d after 1 char", baseH, h1)
	}

	// Text llarg que embolcalla (> 80 cols menys marges)
	m.input.SetValue(strings.Repeat("x", 100))
	m.fitInput()
	ib2 := m.inputBox()
	h2 := lipgloss.Height(ib2)
	t.Logf("wrapping text: inputBox height = %d", h2)
	// Ha de ser >= baseH (pot créixer per wrapping, però no ha de saltar)

	// Comprovem que cap línia del textarea dins l'inputBox desbordi
	// l'ample interior del box. Si ho fa, lipgloss embolcalla i salta.
	boxContentW := max(m.vp.Width-2, 10) // Width del lipgloss.Style
	boxInterior := boxContentW - 2       // menys Padding(0,1)
	t.Logf("box content width = %d, interior = %d", boxContentW, boxInterior)
	tiView := m.input.View()
	for i, l := range strings.Split(tiView, "\n") {
		lw := lipgloss.Width(l)
		t.Logf("textarea line %d width = %d: %q", i, lw, l)
		if lw > boxInterior {
			t.Errorf("textarea line %d width %d > box interior %d — causes wrapping jump!", i, lw, boxInterior)
		}
	}

	// Salt de línia explícit: ha de créixer
	m.input.SetValue("línia1\nlínia2")
	m.fitInput()
	ib3 := m.inputBox()
	h3 := lipgloss.Height(ib3)
	t.Logf("2 lines: inputBox height = %d (base was %d)", h3, baseH)
	if h3 <= baseH {
		t.Errorf("inputBox height should grow with newline: got %d, base %d", h3, baseH)
	}

	// Tornar a 1 línia: ha de tornar a baseH
	m.input.SetValue("a")
	m.fitInput()
	ib4 := m.inputBox()
	h4 := lipgloss.Height(ib4)
	t.Logf("back to 1 char: inputBox height = %d", h4)
	if h4 != baseH {
		t.Errorf("inputBox height should return to base: got %d, want %d", h4, baseH)
	}
}

// Cap línia de la vista pot fer més columnes que el terminal: si una en fa,
// el terminal l'embolcalla i tota la pantalla es desplaça una fila —el
// "salta tot" que es veu com un parpelleig i com a línies trencades. Es
// comprova a amplades i estats variats, que és on canvien els càlculs.
func TestVistaNoDesbordaMai(t *testing.T) {
	amplades := []int{60, 80, 96, 110, 140, 180}
	estats := map[string]func(m *Model){
		"repòs":      func(m *Model) {},
		"treballant": func(m *Model) { m.busy = true; m.status = "read internal/tui/app.go · pas 3/6" },
		"agent": func(m *Model) {
			m.busy = true
			m.agentActive = true
			m.torn = m.nouTorn("una tasca")
			m.status = "escrivint internal/web/serve.go"
		},
		"consulta":  func(m *Model) { m.mode = agent.ModeInspect },
		"objectiu":  func(m *Model) { m.mode = agent.ModeGoal },
		"xat":       func(m *Model) { m.mode = "chat" },
		"context80": func(m *Model) { m.usedTokens = 27000 },
		"context99": func(m *Model) { m.usedTokens = 32500; m.busy = true },
		"branca llarga": func(m *Model) {
			m.branch = "feature/una-branca-amb-un-nom-molt-llarg-que-no-hauria-de-trencar-res"
		},
		"projecte llarg": func(m *Model) {
			m.cwd = "C:/Users/algú/Documents/Projectes/un-directori-amb-un-nom-desproporcionadament-llarg"
		},
		"menú": func(m *Model) {
			items := []menuItem{}
			for _, s := range []string{"cloud/deepseek-v4.1-flash-amb-un-nom-molt-llarg-de-debò", "local/Nex-2.5-mini", "cloud/muse-spark-1.3-contributor", "local/qwen3.6-35b"} {
				items = append(items, menuItem{text: s, val: s})
			}
			m.menu = &menuState{title: "tria provider i model per al rol code (només sessió)", items: items, idx: 1}
		},
		"confirma": func(m *Model) {
			m.busy = true
			m.agentActive = true
			m.pending = &pendingOp{desc: "bash: go build ./... && go test ./... -count=1 -timeout 300s | tail -40 — una ordre llarga que no cap en una línia estreta"}
		},
		"suggeriments": func(m *Model) {
			m.input.SetValue("/mo")
			m.suggIdx = 0
		},
		"ajuda": func(m *Model) {
			for _, l := range strings.Split(helpBlock(), "\n") {
				m.push(l)
			}
		},
		"benvinguda": func(m *Model) {
			m.push(welcome(m.role, m.vp.Width))
		},
		"historial": func(m *Model) {
			m.push(userLine("hola, revisa els fitxers del projecte i digue'm què hi ha"))
			m.push(assistantMD("## Títol\n\nUna llista:\n\n- un\n- dos\n\n```go\nfmt.Println(\"codi molt llarg que no s'hauria de tallar malament perquè és un bloc de codi de veritat\")\n```", 80))
			m.convo = append(m.convo, llm.Message{Role: "user", Content: "hola"})
		},
	}
	for _, w := range amplades {
		for nom, prep := range estats {
			m := planTestModel(t)
			mod, _ := m.Update(tea.WindowSizeMsg{Width: w, Height: 30})
			m = mod.(Model)
			prep(&m)
			m.refresh()
			for _, f := range []int{0, 3, 7} {
				m.spin = f
				for i, l := range strings.Split(m.View(), "\n") {
					if got := lipgloss.Width(l); got > w {
						t.Fatalf("amplada %d, estat %q, frame %d: la línia %d fa %d columnes:\n%s", w, nom, f, i, got, l)
					}
				}
			}
		}
	}
}
