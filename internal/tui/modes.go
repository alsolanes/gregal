package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"gregal/internal/agent"
	"gregal/internal/config"
)

// El TUI mostra els mateixos tres modes que la web i l'Android
// (agent/access.go): Pregunta, Planifica i Actua, i a Actua un permís:
// Demana abans de canviar, Aprova les edicions o Autonomia total. Objectiu
// i Depuració són avançats (/goal, /mode debug) i no entren al cicle.

// modeFromWord tradueix el nom que escriu la persona (/mode ask, /mode
// actua…) a l'id intern: el mateix criteri a totes les interfícies
// (agent.ModeFromWord). Torna "" si no el coneix.
func modeFromWord(w string) string { return agent.ModeFromWord(w) }

// cycleMode passa al mode següent (Pregunta → Planifica → Actua) i el desa.
func (m *Model) cycleMode() string {
	seguent := agent.UIModes[0]
	for i, nom := range agent.UIModes {
		if nom == agent.UIMode(m.mode) {
			seguent = agent.UIModes[(i+1)%len(agent.UIModes)]
			break
		}
	}
	mode := agent.ModeFor(seguent, "")
	if seguent == agent.UIModeAct {
		mode = m.currentActMode()
	}
	m.setMode(mode)
	return mode
}

// currentActMode és el mode al qual torna Actua: l'últim d'Actua (code,
// autonomous o debug), perquè sortir a Pregunta i tornar no perdi
// l'Autonomia total.
func (m *Model) currentActMode() string {
	if agent.UIMode(m.mode) == agent.UIModeAct {
		return m.mode
	}
	if m.actMode != "" {
		return m.actMode
	}
	return agent.ModeCode
}

// access és el permís d'Actua que es mostra.
func (m *Model) access() string {
	ask := m.cfg.Permissions.AskEdits()
	return agent.AccessOf(m.currentActMode(), ask)
}

// setMode canvia el mode i el desa al config. El mode permissiu (aprova
// el que preguntaria) va lligat a l'Autonomia total, com a la web.
func (m *Model) setMode(mode string) {
	if agent.UIMode(m.mode) == agent.UIModeAct {
		m.actMode = m.mode
	}
	if agent.UIMode(mode) == agent.UIModeAct {
		m.actMode = mode
	}
	m.mode = mode
	m.cfg.Mode = mode
	m.permissive = mode == agent.ModeAutonomous
	if m.permissive {
		m.autoReview = false
	}
	if err := m.cfg.Save(m.cfgPath); err != nil {
		m.push(systemLine("mode: " + mode + T("modes.noDesat") + err.Error() + ")"))
	}
}

// setAccess tria el permís d'Actua i hi passa. ask i edits queden al config
// (permissions.access); full és el mode autònom. La depuració és Actua:
// amb ask o edits s'hi queda.
func (m *Model) setAccess(a string) {
	switch a {
	case agent.AccessAsk:
		m.cfg.Permissions.Access = config.AccessAsk
	case agent.AccessEdits:
		m.cfg.Permissions.Access = ""
	}
	if m.policy != nil {
		cp := *m.policy
		cp.AskEdits = m.cfg.Permissions.AskEdits()
		m.policy = &cp
	}
	mode := agent.ModeFor(agent.UIModeAct, a)
	if a != agent.AccessFull && m.currentActMode() == agent.ModeDebug {
		mode = agent.ModeDebug
	}
	m.setMode(mode)
}

// accessKeys són el nom i l'explicació de cada permís, iguals a totes les
// interfícies.
var accessKeys = map[string][2]string{
	agent.AccessAsk:   {"perm.ask", "perm.askHelp"},
	agent.AccessEdits: {"perm.edits", "perm.editsHelp"},
	agent.AccessFull:  {"perm.full", "perm.fullHelp"},
}

// accessLabel és el nom del permís.
func accessLabel(a string) string { return T(accessKeys[a][0]) }

// accessHelp és el que fa el permís.
func accessHelp(a string) string { return T(accessKeys[a][1]) }

// takeDeferred retorna (i buida) la feina asíncrona demanada des d'un menú.
func (m *Model) takeDeferred() tea.Cmd {
	cmd := m.deferred
	m.deferred = nil
	return cmd
}

// modeNom és el nom del mode que es mostra (no l'id intern): Pregunta,
// Planifica o Actua, o el mode avançat si n'hi ha un.
func modeNom(mode string) string {
	switch mode {
	case agent.ModeGoal:
		return T("mode.goal.badge")
	case agent.ModeDebug:
		return T("mode.debug.badge")
	}
	switch agent.UIMode(mode) {
	case agent.UIModeAsk:
		return T("mode.chat.badge")
	case agent.UIModePlan:
		return T("mode.plan.badge")
	}
	return T("mode.code.badge")
}
