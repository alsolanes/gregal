package tui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

// Canal d'estat del mode Kanban del web. Quan el servidor engega el TUI
// dins d'una targeta, li passa dues variables:
//
//   - GREGAL_KANBAN_STATUS: fitxer on el TUI escriu el seu estat (JSON) cada
//     cop que canvia. El tauler en llegeix l'estat per moure la targeta
//     de columna; no ha d'endevinar res mirant la pantalla.
//   - GREGAL_KANBAN_PROMPT: tasca opcional que s'envia sola en arrencar,
//     com si l'usuari l'hagués escrit i premut Enter.
//
// Fora del Kanban cap de les dues hi és i el TUI no canvia gens.
const (
	KanbanStatusEnv = "GREGAL_KANBAN_STATUS"
	KanbanPromptEnv = "GREGAL_KANBAN_PROMPT"
)

// Estats que publica el TUI. El servidor els tradueix a columnes.
const (
	KanbanIdle       = "idle"        // esperant la primera tasca
	KanbanPlanning   = "planning"    // explorant per proposar un pla
	KanbanPlanReview = "plan_review" // pla proposat, pendent d'aprovar
	KanbanWorking    = "working"     // torn en marxa (model o eines)
	KanbanApproval   = "approval"    // una eina espera permís
	KanbanQuestion   = "question"    // el model ha fet una pregunta
	KanbanDone       = "done"        // ha acabat i espera la següent
)

// KanbanStatus és el que s'escriu al fitxer d'estat.
type KanbanStatus struct {
	State  string    `json:"state"`
	Detail string    `json:"detail,omitempty"` // barra d'estat del TUI
	Intent string    `json:"intent,omitempty"` // què diu el model que fa
	Task   string    `json:"task,omitempty"`   // últim missatge enviat
	Turns  int       `json:"turns"`
	Mode   string    `json:"mode,omitempty"`
	PID    int       `json:"pid"`
	At     time.Time `json:"at"`
}

// kanbanState diu en quin punt és la feina. L'ordre importa: una aprovació
// pendent passa per davant de «treballant», perquè és el que necessita
// l'usuari.
func (m Model) kanbanState() string {
	switch {
	case m.pending != nil:
		return KanbanApproval
	case m.pendingQ != nil:
		return KanbanQuestion
	case m.pendingPlan != "":
		return KanbanPlanReview
	case m.planning:
		return KanbanPlanning
	case m.busy || m.agentActive || m.flow != nil || m.serviceTurn:
		return KanbanWorking
	case m.lastPrompt != "" || len(m.convo) > 0:
		return KanbanDone
	}
	return KanbanIdle
}

func (m Model) kanbanStatus() KanbanStatus {
	turns := 0
	for _, msg := range m.convo {
		if msg.Role == "user" {
			turns++
		}
	}
	return KanbanStatus{
		State:  m.kanbanState(),
		Detail: tailRunes(ansi.Strip(m.status), 160),
		Intent: tailRunes(m.intencio, 200),
		Task:   tailRunes(m.lastPrompt, 300),
		Turns:  turns,
		Mode:   m.mode,
		PID:    os.Getpid(),
	}
}

// kanbanModel embolcalla el Model: després de cada Update publica l'estat
// si ha canviat, i envia la tasca inicial quan el TUI ja té mida.
type kanbanModel struct {
	m      Model
	path   string
	prompt string
	sent   bool
	last   KanbanStatus
	wrote  time.Time
}

type kanbanPromptMsg struct{}

// WithKanban torna el model tal qual fora del Kanban, o embolcallat si el
// servidor ha demanat el canal d'estat.
func WithKanban(m Model) tea.Model {
	path := strings.TrimSpace(os.Getenv(KanbanStatusEnv))
	if path == "" {
		return m
	}
	return &kanbanModel{m: m, path: path, prompt: strings.TrimSpace(os.Getenv(KanbanPromptEnv))}
}

func (k *kanbanModel) Init() tea.Cmd {
	k.publish(true)
	return k.m.Init()
}

func (k *kanbanModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	if _, ok := msg.(kanbanPromptMsg); ok {
		// Pel camí normal de l'Enter: historial, ordres amb /, mode pla…
		k.m.input.SetValue(k.prompt)
		msg = tea.KeyMsg{Type: tea.KeyEnter}
	}
	next, cmd := k.m.Update(msg)
	if mm, ok := next.(Model); ok {
		k.m = mm
	}
	cmds = append(cmds, cmd)
	if _, ok := msg.(tea.WindowSizeMsg); ok && !k.sent && k.prompt != "" && k.m.ready {
		k.sent = true
		cmds = append(cmds, func() tea.Msg { return kanbanPromptMsg{} })
	}
	k.publish(false)
	return k, tea.Batch(cmds...)
}

func (k *kanbanModel) View() string { return k.m.View() }

// publish escriu l'estat si ha canviat. Un canvi d'estat s'escriu de
// seguida; si només canvia el text de detall, com a molt un cop per segon.
func (k *kanbanModel) publish(force bool) {
	st := k.m.kanbanStatus()
	cmp := st
	cmp.At, cmp.PID = k.last.At, k.last.PID
	if !force && cmp == k.last {
		return
	}
	if !force && st.State == k.last.State && time.Since(k.wrote) < time.Second {
		return
	}
	st.At = time.Now().UTC()
	b, err := json.Marshal(st)
	if err != nil {
		return
	}
	if writeAtomic(k.path, b) == nil {
		k.last, k.wrote = st, time.Now()
	}
}

func writeAtomic(path string, b []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
