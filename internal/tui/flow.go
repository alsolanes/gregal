package tui

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"gregal/internal/flow"
	"gregal/internal/llm"
)

// Els grafs al TUI: aquí s'executen i es miren, però no s'editen. Dibuixar
// vol arrossegar, i arrossegar vol ratolí i llenç; fer-ho a base de menús
// seria pitjor que obrir l'escriptori. Com que el graf viu al repositori,
// el que has dibuixat allà (o un company) es corre des d'aquí sense més.

// flowRun recull el que va passant mentre el graf corre. Mateix patró que
// el streamer de l'agent: la feina va en una goroutine, el tick de 60 ms en
// fa una fotografia i el TUI no toca res de dins.
type flowRun struct {
	mu      sync.Mutex
	nom     string
	fets    []flow.StepResult
	llegits int // quants n'ha pintat ja el TUI
	acabat  bool
	res     flow.RunResult
	err     error
}

func (f *flowRun) afegeix(st flow.StepResult) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.fets = append(f.fets, st)
}

func (f *flowRun) acaba(res flow.RunResult, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.res, f.err, f.acabat = res, err, true
}

// nous torna els passos que el TUI encara no ha pintat.
func (f *flowRun) nous() ([]flow.StepResult, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	nous := f.fets[f.llegits:]
	f.llegits = len(f.fets)
	return nous, f.acabat
}

// flowCommand reparteix /graf.
func (m *Model) flowCommand(arg string) (tea.Model, tea.Cmd) {
	arg = strings.TrimSpace(arg)
	llista, err := flow.Llista(m.cwd)
	if err != nil {
		m.push(badStyle.Render(T("flow.noLlegits") + err.Error()))
		return *m, nil
	}
	if len(llista) == 0 {
		m.push(systemLine(T("flow.capGraf") + flow.Dir +
			T("flow.onEsDibuixen")))
		return *m, nil
	}
	if arg == "" {
		// Sense nom, el menú de sempre: primer et deixa mirar-lo, que
		// executar un graf toca fitxers i val més saber què farà.
		m.menu = flowMenu(llista)
		return *m, nil
	}
	return m.startFlow(arg)
}

func flowMenu(llista []flow.Resum) *menuState {
	items := make([]menuItem, 0, len(llista))
	for _, f := range llista {
		txt := fmt.Sprintf("%s — %d passos", f.Nom, f.Passos)
		if f.Desc != "" {
			txt += " · " + f.Desc
		}
		items = append(items, menuItem{text: txt, val: f.Nom})
	}
	return &menuState{title: T("flow.menuTitol"), items: items, run: func(m *Model, val string) (string, *menuState) {
		f, err := flow.Carrega(m.cwd, val)
		if err != nil {
			return badStyle.Render(err.Error()), nil
		}
		return dibuixaFlow(f) + "\n" + systemLine(T("flow.executaAmb")+val), nil
	}}
}

// dibuixaFlow escriu el graf en text. No intenta ser un diagrama: és la
// llista de passos amb les fletxes que en surten, que és el que necessites
// saber abans d'executar-lo.
func dibuixaFlow(f *flow.Flow) string {
	var b strings.Builder
	b.WriteString(headStyle.Render("≋ " + f.Name))
	if f.Desc != "" {
		b.WriteString(dimStyle.Render(" — " + f.Desc))
	}
	b.WriteString("\n")
	inici, _ := f.Start()
	for _, n := range f.Nodes {
		marca := " "
		if n.ID == inici {
			marca = "▸"
		}
		b.WriteString(fmt.Sprintf("  %s %s %s\n", marca, cmdStyle.Render(n.ID), dimStyle.Render("("+string(n.Kind)+") "+etiqueta(n))))
		for _, e := range f.Edges {
			if e.From != n.ID {
				continue
			}
			cond := "sempre"
			if e.When != "" {
				cond = e.When
			}
			// El salt de línia va fora del Render: lipgloss tracta un text
			// amb "\n" com un bloc i el farceix fins a l'amplada de la línia
			// més llarga, o sigui que la línia següent sortia escopida cap a
			// la dreta.
			b.WriteString(dimStyle.Render(fmt.Sprintf("      └─[ %s ]→ %s", cond, e.To)) + "\n")
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

func etiqueta(n flow.Node) string {
	if strings.TrimSpace(n.Title) != "" {
		return n.Title
	}
	switch n.Kind {
	case flow.KindTool:
		return n.Tool
	case flow.KindAgent:
		return primeraLinia(n.Task)
	}
	return primeraLinia(n.Text)
}

func primeraLinia(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	if len([]rune(s)) > 60 {
		s = string([]rune(s)[:60]) + "…"
	}
	return s
}

// startFlow executa un graf. Les eines que demanen permís passen, perquè un
// graf corre sol i no hi ha ningú per contestar el diàleg; les denegades
// continuen bloquejades. Es diu en clar abans de començar.
func (m *Model) startFlow(nom string) (tea.Model, tea.Cmd) {
	if m.busy || m.agentActive || m.planning || m.flow != nil {
		m.push(systemLine(T("flow.ocupat")))
		return *m, nil
	}
	f, err := flow.Carrega(m.cwd, nom)
	if err != nil {
		m.push(badStyle.Render(err.Error()))
		return *m, nil
	}
	if err := f.Validate(); err != nil {
		m.push(badStyle.Render(T("flow.noExecutable") + err.Error()))
		return *m, nil
	}

	m.push(userLine("⌘ " + f.Name))
	m.push(workRail(dimStyle.Render(fmt.Sprintf(T("flow.avisPermisos"), len(f.Nodes)))))

	fr := &flowRun{nom: f.Name}
	m.flow = fr
	m.busy = true
	m.status = "graf: " + f.Name

	ctx, cancel := context.WithCancel(context.Background())
	m.flowCancel = cancel
	runner := &flow.AgentRunner{
		Cfg: m.cfg, Client: m.client, Policy: m.policy,
		Mode: "code", AutoApprove: true, Workspace: m.cwd,
	}
	go func() {
		defer func() {
			// Un pànic d'una eina no s'ha d'emportar el TUI sencer.
			if r := recover(); r != nil {
				fr.acaba(flow.RunResult{}, fmt.Errorf(T("flow.panic"), r))
			}
		}()
		res, err := flow.Run(ctx, f, runner, flow.Opcions{
			OnStep: func(st flow.StepResult) {
				fr.afegeix(st)
			},
		})
		fr.acaba(res, err)
	}()
	return *m, tea.Batch(tick(), flowPoll())
}

// flowPoll és el batec propi del graf: un pas pot trigar minuts i el tick
// de l'animació no ha de dependre d'això.
func flowPoll() tea.Cmd {
	return tea.Tick(150*time.Millisecond, func(time.Time) tea.Msg { return flowTickMsg{} })
}

type flowTickMsg struct{}

// flowTick pinta els passos acabats i tanca quan el graf s'acaba.
func (m *Model) flowTick() (tea.Model, tea.Cmd) {
	fr := m.flow
	if fr == nil {
		return *m, nil
	}
	nous, acabat := fr.nous()
	for _, st := range nous {
		m.push(flowStepLine(st))
	}
	if !acabat {
		if len(nous) > 0 {
			m.refresh()
		}
		return *m, flowPoll()
	}

	res, err := fr.res, fr.err
	m.flow = nil
	m.flowCancel = nil
	m.busy = false
	m.status = "llest"
	switch {
	case res.Stopped != "":
		m.push(warnStyle.Render(T("flow.aturat") + res.Stopped))
	case err != nil:
		m.push(badStyle.Render("graf: " + err.Error()))
	default:
		m.push(systemLine(fmt.Sprintf(T("flow.fet"), fr.nom, len(res.Steps))))
	}
	// L'última sortida és la resposta: va com a text de l'agent, no al rail
	// de feina, perquè és el resultat i no el procés.
	if last, ok := res.Last(); ok && strings.TrimSpace(last.Output) != "" && last.Err == "" {
		m.push(assistantLine(last.Output))
	}
	// A la conversa perquè hi puguis preguntar tot seguit.
	m.convo = append(m.convo,
		llm.Message{Role: "user", Content: "Executa el graf «" + fr.nom + "»."},
		llm.Message{Role: "assistant", Content: resumFlow(res)})
	m.refresh()
	return *m, nil
}

func flowStepLine(st flow.StepResult) string {
	nom := st.Title
	if strings.TrimSpace(nom) == "" {
		nom = st.Node
	}
	durada := fmt.Sprintf("%.1fs", st.Took.Seconds())
	if st.Err != "" {
		return workRail(badStyle.Render(fmt.Sprintf("✗ %s · %s · %s", nom, durada, st.Err)))
	}
	return workRail(dimStyle.Render(fmt.Sprintf("✓ %s · %s", nom, durada)))
}

func resumFlow(res flow.RunResult) string {
	var b strings.Builder
	for _, st := range res.Steps {
		estat := "ok"
		if st.Err != "" {
			estat = "error: " + st.Err
		}
		nom := st.Title
		if nom == "" {
			nom = st.Node
		}
		b.WriteString("- " + nom + " (" + estat + ")\n")
	}
	if res.Stopped != "" {
		b.WriteString("\nAturat: " + res.Stopped + "\n")
	}
	if last, ok := res.Last(); ok && strings.TrimSpace(last.Output) != "" {
		b.WriteString("\n" + last.Output)
	}
	return b.String()
}
