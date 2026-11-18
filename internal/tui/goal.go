package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"gregal/internal/agent"
	"gregal/internal/backlog"
	"gregal/internal/goal"
	"gregal/internal/profile"
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
		return filepath.Join(base, profile.DirName())
	}
	return "."
}

// projectName és el nom del projecte actual.
func (m Model) projectName() string {
	return filepath.Base(strings.TrimRight(m.cwd, string(filepath.Separator)))
}

// El mode objectiu desa tasques al backlog del projecte (goal/backlog.go):
// /goal és la manera de veure-les i posar-s'hi des del terminal.

// migratsTUI recorda si els objectius vells ja s'han passat al backlog.
var migratsTUI = map[string]bool{}

func (m *Model) migraObjectius() {
	dir := m.goalDir()
	if migratsTUI[dir] {
		return
	}
	migratsTUI[dir] = true
	if n, err := goal.MigrateToBacklog(dir); n > 0 {
		m.push(systemLine(fmt.Sprintf("backlog: %d objectius passats a tasques", n)))
	} else if err != nil {
		m.push(systemLine("backlog: " + err.Error()))
	}
}

// recordGoal desa al backlog la tasca del bloc ```goal, n'apunta el número
// a la resposta (perquè el model la pugui revisar amb id:) i pinta la
// targeta.
func (m *Model) recordGoal(reply string) (backlog.Item, bool) {
	if _, ok := goal.ParseItem(reply); !ok {
		return backlog.Item{}, false
	}
	m.migraObjectius()
	it, created, err := goal.Record(reply, m.cwd)
	if err != nil {
		m.push(systemLine(T("goal.noDesat") + err.Error()))
		return backlog.Item{}, false
	}
	goal.AnnotateLast(m.convo, goal.Note(it, created))
	m.lastGoalID = strconv.Itoa(it.ID)
	m.push(goalCard(it, m.vp.Width))
	// Accions immediates (fletxes + Enter): treballar-hi, revisar-la,
	// llistar, descartar. Només si no estaves escrivint res, per no
	// robar-te el teclat a mitja frase.
	if m.menu == nil && strings.TrimSpace(m.input.Value()) == "" {
		m.menu = goalActionMenu(it)
	}
	return it, true
}

// goalCard pinta la tasca amb les accions disponibles.
func goalCard(it backlog.Item, width int) string {
	cos := ""
	for _, linia := range strings.Split(goal.Card(it), "\n")[1:] {
		cos += "  " + linia + "\n"
	}
	titol := goalTitleStyle.Render("◆ "+T("mode.goal.badge")+" · BACKLOG #"+strconv.Itoa(it.ID)) +
		dimStyle.Render("  "+it.Type+" · "+it.Status)
	return goalBoxStyle.Width(max(width-2, 20)).Render(titol + "\n" + cos +
		dimStyle.Render(T("goal.pista")))
}

// executaGoal passa a mode code, posa la tasca en curs i engega l'agent.
func (m *Model) executaGoal(it backlog.Item) (tea.Model, tea.Cmd) {
	it, task, err := goal.Start(m.cwd, it.ID, "")
	if err != nil {
		m.push(systemLine("#" + strconv.Itoa(it.ID) + ": " + err.Error()))
		return m, nil
	}
	if m.mode != agent.ModeCode {
		m.mode = agent.ModeCode
		m.cfg.Mode = m.mode
		if err := m.cfg.Save(m.cfgPath); err != nil {
			m.push(systemLine(T("goal.modeCodeNoDesat") + err.Error() + ")"))
		}
	}
	m.push(okStyle.Render("◆ "+T("goal.executant")+strconv.Itoa(it.ID)) + dimStyle.Render(" — "+it.Title))
	return m.startAgent(task)
}

// goalCommand implementa /goal i les seves variants.
func (m *Model) goalCommand(arg string) (tea.Model, tea.Cmd) {
	m.migraObjectius()
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
	case "esborra", "descarta":
		motiu := ""
		if len(camps) > 2 {
			motiu = strings.Join(camps[2:], " ")
		}
		return m.goalDelete(m.goalArg(camps), motiu)
	default:
		m.push(systemLine(T("goal.us")))
		return m, nil
	}
}

// goalArg retorna el número indicat o l'última tasca usada.
func (m Model) goalArg(camps []string) string {
	if len(camps) > 1 {
		return camps[1]
	}
	return m.lastGoalID
}

// goalResol troba la tasca demanada (per número, l'última o la més recent).
func (m Model) goalResol(id string) (backlog.Item, bool) {
	it, err := goal.Resolve(m.cwd, id)
	if err != nil && id != "" {
		it, err = goal.Resolve(m.cwd, "")
	}
	return it, err == nil
}

func (m *Model) goalList() (tea.Model, tea.Cmd) {
	llista, err := goal.Open(m.cwd)
	if err != nil {
		m.push(systemLine(T("goal.noLlegits") + err.Error()))
		return m, nil
	}
	if len(llista) == 0 {
		m.push(systemLine(T("goal.capEncara")))
		return m, nil
	}
	m.push(systemStyle.Render("◆ "+T("goal.objectiusDe")+m.projectName()) + dimStyle.Render("  (/goal executa <#>)"))
	for _, it := range llista {
		m.push("  " + backlog.Resum(it))
	}
	return m, nil
}

func (m *Model) goalShow(id string) (tea.Model, tea.Cmd) {
	it, ok := m.goalResol(id)
	if !ok {
		m.push(systemLine(T("goal.capDesat")))
		return m, nil
	}
	m.lastGoalID = strconv.Itoa(it.ID)
	m.push(goalCard(it, m.vp.Width))
	return m, nil
}

func (m *Model) goalRun(id string) (tea.Model, tea.Cmd) {
	it, ok := m.goalResol(id)
	if !ok {
		m.push(systemLine(T("goal.capExecutar")))
		return m, nil
	}
	m.lastGoalID = strconv.Itoa(it.ID)
	return m.executaGoal(it)
}

// goalDelete descarta la tasca: al backlog no s'esborra res, i el motiu
// queda al log (si no en dius cap, consta que va ser des d'aquí).
func (m *Model) goalDelete(id, motiu string) (tea.Model, tea.Cmd) {
	it, ok := m.goalResol(id)
	if !ok {
		m.push(systemLine(T("goal.capEsborrar")))
		return m, nil
	}
	if strings.TrimSpace(motiu) == "" {
		motiu = "dropped from the terminal (/goal)"
	}
	st, err := backlog.Open(m.cwd)
	if err == nil {
		_, err = st.Move(it.ID, backlog.StatusDropped, motiu, "")
	}
	if err != nil {
		m.push(systemLine(T("goal.noEsborrat") + err.Error()))
		return m, nil
	}
	if m.lastGoalID == strconv.Itoa(it.ID) {
		m.lastGoalID = ""
	}
	m.push(okStyle.Render("✓ "+T("goal.esborrat")) + dimStyle.Render(" · #"+strconv.Itoa(it.ID)+" "+it.Title))
	return m, nil
}

// goalMenu és la finestra navegable del backlog.
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
	return &menuState{title: T("goal.objectius"), items: items, run: func(m *Model, val string) (string, *menuState) {
		if val == "llista" {
			return "", goalListMenu(m)
		}
		// Les accions poden engegar feina asíncrona (executa → agent):
		// el menú no retorna Cmd, així que va a deferred i el cicle
		// d'Update la recull. Llançar-la aquí moria: torn penjat en busy.
		mm, cmd := m.goalCommand(val)
		// startAgent torna Model per valor: cal copiar l'estat (busy,
		// stream, passos...) al model viu, no només el Cmd. Primer la
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

// goalListMenu és la llista interactiva de tasques obertes: triar-ne una
// n'obre les accions.
func goalListMenu(m *Model) *menuState {
	llista, err := goal.Open(m.cwd)
	if err != nil || len(llista) == 0 {
		return &menuState{
			title: T("goal.objectius") + " (" + m.projectName() + ")",
			items: []menuItem{{text: T("goal.capEncara"), val: "noop"}},
			run:   func(*Model, string) (string, *menuState) { return "", nil },
		}
	}
	items := make([]menuItem, 0, len(llista))
	for _, it := range llista {
		text := fmt.Sprintf("#%-3d %-11s %s", it.ID, it.Status, it.Title)
		items = append(items, menuItem{text: text, val: strconv.Itoa(it.ID)})
	}
	return &menuState{
		title: T("goal.objectius") + " · " + m.projectName(),
		items: items,
		run: func(m *Model, id string) (string, *menuState) {
			if id == "noop" {
				return "", nil
			}
			it, ok := m.goalResol(id)
			if !ok {
				return systemLine(T("goal.capDesat")), nil
			}
			m.lastGoalID = strconv.Itoa(it.ID)
			return "", goalActionMenu(it)
		},
	}
}

// goalActionMenu són les accions sobre la tasca acabada de definir o
// triada (fletxes + Enter). Esc tanca sense fer res.
func goalActionMenu(it backlog.Item) *menuState {
	id := strconv.Itoa(it.ID)
	return &menuState{title: T("goal.objectiu") + " · #" + id, items: []menuItem{
		{text: "▶ " + T("goal.accExecuta"), val: "executa"},
		{text: "👁 " + T("goal.accMostra"), val: "mostra"},
		{text: "✎ " + T("goal.accEdita"), val: "edita"},
		{text: "☰ " + T("goal.accLlista"), val: "llista"},
		{text: "🗑 " + T("goal.accEsborra"), val: "esborra"},
	}, run: func(m *Model, val string) (string, *menuState) {
		switch val {
		case "executa":
			// Com al menú /goal: el Cmd va a deferred i l'estat del
			// Model per valor es copia (si no, torn penjat o panic).
			if mm, cmd := m.goalCommand("executa " + id); cmd != nil {
				if v, ok := mm.(Model); ok {
					*m = v
				}
				m.deferred = cmd
			}
			return "", nil
		case "mostra":
			m.push(goalCard(it, m.vp.Width))
			return "", nil
		case "edita":
			// Es revisa amb l'agent en mode objectiu: la tasca va al
			// composer i la persona hi afegeix què vol canviar; el model
			// torna el bloc amb id: i es reescriu la mateixa tasca.
			m.mode = agent.ModeGoal
			m.input.SetValue("Revise backlog item #" + id + " (keep `id: " + id + "` in the block):\n" + goal.Card(it) + "\n\nChange: ")
			m.fitInput()
			return systemLine(T("goal.editaText")), nil
		case "llista":
			return "", goalListMenu(m)
		case "esborra":
			_, _ = m.goalCommand("esborra " + id)
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
