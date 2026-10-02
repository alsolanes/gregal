package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"gregal/internal/agent"
	"gregal/internal/goal"
)

// modeObjectiu és l'etiqueta curta del mode goal a la UI.
const modeObjectiu = "goal"

// goalDir és on es desen els objectius (al costat del config).
func (m Model) goalDir() string {
	if m.cfgPath != "" {
		if dir := filepath.Dir(m.cfgPath); dir != "" && dir != "." {
			return dir
		}
	}
	if base, err := os.UserConfigDir(); err == nil {
		return filepath.Join(base, "gregal")
	}
	return "."
}

// projectName és el nom del projecte actual.
func (m Model) projectName() string {
	return filepath.Base(strings.TrimRight(m.cwd, string(filepath.Separator)))
}

// recordGoal desa l'objectiu d'una resposta de l'agent i pinta la targeta.
func (m *Model) recordGoal(reply string) (goal.Goal, bool) {
	g, ok := goal.Parse(reply, m.cwd)
	if !ok {
		return goal.Goal{}, false
	}
	if err := goal.Save(m.goalDir(), g); err != nil {
		m.push(systemLine(T("goal.noDesat") + err.Error()))
		return goal.Goal{}, false
	}
	m.lastGoalID = g.ID
	m.push(goalCard(g, m.vp.Width))
	// Accions immediates sobre l'objectiu (fletxes + Enter): executar,
	// editar, llistar, esborrar. Només si no estaves escrivint res, per
	// no robar-te el teclat a mitja frase.
	if m.menu == nil && strings.TrimSpace(m.input.Value()) == "" {
		m.menu = goalActionMenu(g)
	}
	return g, true
}

// goalCard pinta l'objectiu desat amb les accions disponibles.
func goalCard(g goal.Goal, width int) string {
	cos := ""
	for _, linia := range strings.Split(g.Body, "\n") {
		cos += "  " + linia + "\n"
	}
	titol := goalTitleStyle.Render("◆ OBJECTIU") +
		dimStyle.Render("  "+g.ID+" · "+g.Project+" · "+g.Status)
	return goalBoxStyle.Width(max(width-2, 20)).Render(titol + "\n" + cos +
		dimStyle.Render(T("goal.pista")))
}

// executaGoal passa a mode code i engega l'agent amb l'objectiu.
func (m *Model) executaGoal(g goal.Goal) (tea.Model, tea.Cmd) {
	if m.mode != agent.ModeCode {
		m.mode = agent.ModeCode
		m.cfg.Mode = m.mode
		if err := m.cfg.Save(m.cfgPath); err != nil {
			m.push(systemLine(T("goal.modeCodeNoDesat") + err.Error() + ")"))
		}
	}
	g.Status = goal.StatusFet
	if err := goal.Save(m.goalDir(), g); err != nil {
		m.push(systemLine(T("goal.noMarcat") + err.Error()))
	}
	m.push(okStyle.Render("◆ executant l'objectiu "+g.ID) + dimStyle.Render(" — "+g.Title))
	return m.startAgent(goal.Task(g))
}

// goalCommand implementa /goal i les seves variants.
func (m *Model) goalCommand(arg string) (tea.Model, tea.Cmd) {
	arg = strings.TrimSpace(arg)
	camps := strings.Fields(arg)
	accio := ""
	if len(camps) > 0 {
		accio = camps[0]
	}
	switch accio {
	case "":
		m.menu = goalMenu(m)
		return m, nil
	case "nou":
		m.mode = agent.ModeGoal
		m.cfg.Mode = m.mode
		if err := m.cfg.Save(m.cfgPath); err != nil {
			m.push(systemLine(T("goal.modeGoalNoDesat") + err.Error() + ")"))
		} else {
			m.push(systemLine(T("goal.modeNota")))
		}
		return m, nil
	case "llista":
		m.menu = goalListMenu(m)
		return m, nil
	case "mostra":
		return m.goalShow(m.goalArg(camps))
	case "executa":
		return m.goalRun(m.goalArg(camps))
	case "esborra":
		return m.goalDelete(m.goalArg(camps))
	default:
		m.push(systemLine(T("goal.us")))
		return m, nil
	}
}

// goalArg retorna l'id indicat o l'últim objectiu usat.
func (m Model) goalArg(camps []string) string {
	if len(camps) > 1 {
		return camps[1]
	}
	return m.lastGoalID
}

// goalResol troba l'objectiu demanat (per id o l'últim) del projecte actual.
func (m Model) goalResol(id string) (goal.Goal, bool) {
	if id != "" {
		if g, err := goal.Get(m.goalDir(), id); err == nil {
			return g, true
		}
	}
	llista, err := goal.List(m.goalDir(), m.projectName())
	if err != nil || len(llista) == 0 {
		return goal.Goal{}, false
	}
	return llista[0], true
}

func (m *Model) goalList() (tea.Model, tea.Cmd) {
	llista, err := goal.List(m.goalDir(), m.projectName())
	if err != nil {
		m.push(systemLine(T("goal.noLlegits") + err.Error()))
		return m, nil
	}
	if len(llista) == 0 {
		m.push(systemLine(T("goal.capEncara")))
		return m, nil
	}
	m.push(systemStyle.Render("◆ objectius de "+m.projectName()) + dimStyle.Render("  (/goal executa <id>)"))
	for _, g := range llista {
		m.push("  " + goal.Resum(g) + dimStyle.Render("  "+g.Created.Local().Format("02/01 15:04")))
	}
	return m, nil
}

func (m *Model) goalShow(id string) (tea.Model, tea.Cmd) {
	g, ok := m.goalResol(id)
	if !ok {
		m.push(systemLine(T("goal.capDesat")))
		return m, nil
	}
	m.lastGoalID = g.ID
	m.push(goalCard(g, m.vp.Width))
	return m, nil
}

func (m *Model) goalRun(id string) (tea.Model, tea.Cmd) {
	g, ok := m.goalResol(id)
	if !ok {
		m.push(systemLine(T("goal.capExecutar")))
		return m, nil
	}
	m.lastGoalID = g.ID
	return m.executaGoal(g)
}

func (m *Model) goalDelete(id string) (tea.Model, tea.Cmd) {
	g, ok := m.goalResol(id)
	if !ok {
		m.push(systemLine(T("goal.capEsborrar")))
		return m, nil
	}
	if err := goal.Delete(m.goalDir(), g.ID); err != nil {
		m.push(systemLine(T("goal.noEsborrat") + err.Error()))
		return m, nil
	}
	if m.lastGoalID == g.ID {
		m.lastGoalID = ""
	}
	m.push(okStyle.Render("✓ objectiu esborrat") + dimStyle.Render(" · "+g.ID+" "+g.Title))
	return m, nil
}

// goalMenu és la finestra navegable d'objectius.
func goalMenu(m *Model) *menuState {
	teUltim := m.lastGoalID != ""
	items := []menuItem{
		{text: T("goal.nou"), val: "nou"},
	}
	if teUltim {
		items = append(items,
			menuItem{text: T("goal.executaUltim"), val: "executa"},
			menuItem{text: T("goal.mostraUltim"), val: "mostra"},
		)
	} else {
		items = append(items, menuItem{text: T("goal.executaRecent"), val: "executa"})
	}
	items = append(items,
		menuItem{text: T("goal.llista"), val: "llista"},
		menuItem{text: T("goal.esborraUltim"), val: "esborra"},
	)
	return &menuState{title: "objectiu", items: items, run: func(m *Model, val string) (string, *menuState) {
		if val == "llista" {
			return "", goalListMenu(m)
		}
		// Les accions poden engegar feina asíncrona (executa → agent):
		// el menú no retorna Cmd, així que va a deferred i el cicle
		// d'Update la recull. Llançar-la aquí moria: torn penjat en busy.
		mm, cmd := m.goalCommand(val)
		// startAgent torna Model per valor: cal copiar l'estat (busy,
		// stream, passos...) al model viu, no només el Cmd. Sense això,
		// a més de penjar-se, el type-assert feia panic. Primer la
		// còpia i després el deferred, que si no la còpia l'esborra.
		if v, ok := mm.(Model); ok {
			*m = v
		}
		if cmd != nil {
			m.deferred = cmd
		}
		if v, ok := mm.(*Model); ok {
			return "", v.menu
		}
		return "", nil
	}}
}

// goalListMenu mostra la llista interactiva d'objectius per poder-los
// seleccionar, obrir, veure detalls, executar o esborrar.
func goalListMenu(m *Model) *menuState {
	llista, err := goal.List(m.goalDir(), m.projectName())
	if err != nil || len(llista) == 0 {
		return &menuState{
			title: "objectius (" + m.projectName() + ")",
			items: []menuItem{{text: T("goal.capEncara"), val: "noop"}},
			run:   func(*Model, string) (string, *menuState) { return "", nil },
		}
	}
	items := make([]menuItem, 0, len(llista))
	for _, g := range llista {
		st := "·"
		if g.Status == goal.StatusFet {
			st = "✓"
		}
		text := fmt.Sprintf("%s %-8s %s", st, g.ID, g.Title)
		items = append(items, menuItem{text: text, val: g.ID})
	}
	return &menuState{
		title: "objectius · " + m.projectName(),
		items: items,
		run: func(m *Model, id string) (string, *menuState) {
			if id == "noop" {
				return "", nil
			}
			g, ok := m.goalResol(id)
			if !ok {
				return systemLine(T("goal.capDesat")), nil
			}
			m.lastGoalID = g.ID
			return "", goalActionMenu(g)
		},
	}
}

// goalActionMenu són les accions sobre l'objectiu acabat de definir o triat
// (fletxes + Enter): executar-lo, veure'l, editar-lo al composer, llistar o
// esborrar. Esc tanca sense fer res.
func goalActionMenu(g goal.Goal) *menuState {
	return &menuState{title: "objectiu · " + g.ID, items: []menuItem{
		{text: "▶ Executa'l (passa a code)", val: "executa"},
		{text: "👁 Mostra el pla complet", val: "mostra"},
		{text: "✎ Edita'l al composer", val: "edita"},
		{text: "☰ Llista els del projecte", val: "llista"},
		{text: "🗑 Esborra'l", val: "esborra"},
	}, run: func(m *Model, val string) (string, *menuState) {
		switch val {
		case "executa":
			// Com al menú /goal: el Cmd va a deferred i l'estat del
			// Model per valor es copia (si no, torn penjat o panic).
			if mm, cmd := m.goalCommand("executa " + g.ID); cmd != nil {
				if v, ok := mm.(Model); ok {
					*m = v
				}
				m.deferred = cmd
			}
			return "", nil
		case "mostra":
			m.push(goalCard(g, m.vp.Width))
			return "", nil
		case "edita":
			// El cos torna al composer per retocar-lo: en enviar-lo es
			// desarà com a objectiu nou (l'antic es pot esborrar).
			m.input.SetValue(g.Body)
			m.fitInput()
			return systemLine("edita el text i prem enter"), nil
		case "llista":
			return "", goalListMenu(m)
		case "esborra":
			_, _ = m.goalCommand("esborra " + g.ID)
			return "", nil
		}
		return "", nil
	}}
}

// goalBoxStyle emmarca la targeta d'objectiu.
var goalBoxStyle = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()).
	BorderForeground(algua).
	Padding(0, 1)
