package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"gregal/internal/agent"
)

// ordreModes és el cicle del Tab.
// El cicle curt manté el recorregut habitual. El mode autònom és explícit
// amb /mode autonomous perquè no aparegui per accident en una sessió normal.
var ordreModes = []string{agent.ModeCode, agent.ModeInspect, agent.ModeChat, agent.ModeGoal}

// cycleMode passa al mode següent del cicle i el desa.
func (m *Model) cycleMode() string {
	seguent := ordreModes[0]
	for i, nom := range ordreModes {
		if nom == m.mode {
			seguent = ordreModes[(i+1)%len(ordreModes)]
			break
		}
	}
	m.setMode(seguent)
	return seguent
}

// setMode canvia el mode i el desa al config.
func (m *Model) setMode(mode string) {
	m.mode = mode
	m.cfg.Mode = mode
	if err := m.cfg.Save(m.cfgPath); err != nil {
		m.push(systemLine("mode: " + mode + T("modes.noDesat") + err.Error() + ")"))
	}
}

// takeDeferred retorna (i buida) la feina asíncrona demanada des d'un menú.
func (m *Model) takeDeferred() tea.Cmd {
	cmd := m.deferred
	m.deferred = nil
	return cmd
}
