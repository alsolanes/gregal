package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"gregal/internal/tools"
)

func TestTimelineCasaCridaIResultat(t *testing.T) {
	m := planTestModel(t)
	m.pushToolCall("read", `{"path":"main.go"}`)
	m.pushToolResult("read", "contingut complet", false)
	if len(m.timeline) != 1 || !m.timeline[0].done || m.timeline[0].output != "contingut complet" {
		t.Fatalf("timeline no ha casat crida/resultat: %+v", m.timeline)
	}
	if !strings.Contains(m.timelineBox(90), "read") {
		t.Fatal("l'inspector no mostra l'eina")
	}
}

func TestTimelineNavegaIDesplega(t *testing.T) {
	m := planTestModel(t)
	m.pushToolCall("read", `{"path":"a.go"}`)
	m.pushToolResult("read", "sortida llarga visible", false)
	m.pushToolCall("bash", `{"command":"go test ./..."}`)
	m.pushToolResult("bash", "ok", false)
	m.timelineOpen = true
	m.timelineSel = 1
	mod, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mod.(Model)
	if !m.timeline[1].expanded || !strings.Contains(m.timelineBox(90), "ok") {
		t.Fatal("Enter no desplega el resultat seleccionat")
	}
	mod, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	if mod.(Model).timelineSel != 0 {
		t.Fatal("↑ no navega la timeline")
	}
}

func TestTimelineICuaNoDesborden(t *testing.T) {
	for _, width := range []int{40, 60, 90, 140} {
		m := modelDeBarra(t, width)
		m.pushToolCall("write", `{"path":"un/directori/molt/llarg/fitxer.go"}`)
		m.pushToolResult("write", strings.Repeat("resultat ", 80), false)
		m.timelineOpen = true
		m.timeline[0].expanded = true
		m.queued = []string{strings.Repeat("orientació ", 40)}
		for i, line := range strings.Split(m.View(), "\n") {
			if got := lipgloss.Width(line); got > width {
				t.Fatalf("ample %d, línia %d desborda amb %d: %q", width, i, got, line)
			}
		}
	}
}

func TestQueueEsPotEditar(t *testing.T) {
	m := planTestModel(t)
	m.queued = []string{"primera", "segona", "tercera"}
	mod, _ := m.runCommand("/queue rm 2")
	m = mod.(Model)
	if strings.Join(m.queued, ",") != "primera,tercera" {
		t.Fatalf("/queue rm no ha tret la fila: %q", m.queued)
	}
	mod, _ = m.runCommand("/queue clear")
	if len(mod.(Model).queued) != 0 {
		t.Fatal("/queue clear no ha buidat la cua")
	}
}

func TestCockpitResumeixTasquesCanvisIValidacio(t *testing.T) {
	t.Cleanup(tools.TodoClear)
	tools.TodoSet([]tools.TodoItem{{Title: "editar", Status: "working"}, {Title: "provar", Status: "pending"}})
	m := planTestModel(t)
	m.pushToolCall("edit", `{"path":"internal/tui/app.go"}`)
	m.pushToolResult("edit", "editat", false)
	m.pushToolCall("bash", `{"command":"go test ./..."}`)
	m.pushToolResult("bash", "ok", false)
	m.queued = []string{"afegeix un test"}
	view := stripANSI(m.cockpitBox(100))
	for _, want := range []string{"0/2", "internal/tui/app.go", "validació", "bash", "1 instrucció"} {
		if !strings.Contains(view, want) {
			t.Fatalf("cockpit no conté %q:\n%s", want, view)
		}
	}
}

func TestResumDelTornEsConcret(t *testing.T) {
	m := planTestModel(t)
	m.pushToolCall("edit", `{"path":"a.go"}`)
	m.pushToolResult("edit", "ok", false)
	m.pushToolCall("bash", `{"command":"go test ./..."}`)
	m.pushToolResult("bash", "ERROR: tests", true)
	s := m.runSummaryLine()
	// Plurals bons i sense la pista de Ctrl+L: és un peu de torn, no un
	// avís. «1 fitxers · 1 errors» era el detall que feia semblar el
	// resum escrit per una màquina.
	for _, want := range []string{"2 eines", "1 fitxer ·", "1 error"} {
		if !strings.Contains(s, want) {
			t.Fatalf("resum no conté %q: %s", want, s)
		}
	}
}

func TestReducedMotionCongelaElFrameVisual(t *testing.T) {
	m := planTestModel(t)
	m.reducedMotion = true
	m.spin = 27
	if m.visualFrame() != 0 {
		t.Fatalf("frame reduït=%d", m.visualFrame())
	}
	m.reducedMotion = false
	if m.visualFrame() != 27 {
		t.Fatalf("frame normal=%d", m.visualFrame())
	}
}

func TestTimelineRoundTripDeSessio(t *testing.T) {
	in := []timelineEvent{{name: "read", args: `{"path":"a.go"}`, output: "ok", done: true}, {name: "bash", failed: true, done: true}}
	out := timelineFromSession(timelineToSession(in))
	if len(out) != 2 || out[0].output != "ok" || !out[1].failed {
		t.Fatalf("round trip=%+v", out)
	}
}
