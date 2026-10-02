package tui

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"gregal/internal/config"
	"gregal/internal/llm"
)

// Un proveïdor que accepta la connexió i no respon mai (el que fa un
// gateway amb el motor d'inferència caigut: 524 al cap de molt, o res).
func servidorPenjat(t *testing.T) *httptest.Server {
	t.Helper()
	fi := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-fi:
		case <-r.Context().Done():
		}
	}))
	t.Cleanup(func() { close(fi); srv.Close() })
	return srv
}

func modelContraPenjat(t *testing.T) Model {
	t.Helper()
	srv := servidorPenjat(t)
	cfg := &config.Config{Language: "ca",
		Mode: "code",
		Roles: map[string]config.Role{
			"chat": {Provider: "p", Model: "m", ContextWindow: 32768},
			"code": {Provider: "p", Model: "m", ContextWindow: 32768},
		},
		Providers:   map[string]config.Provider{"p": {BaseURL: srv.URL}},
		Permissions: config.PermissionsCfg{Tools: map[string]string{}},
		Verify:      config.VerifyCfg{Mode: "off"},
		Agent:       config.AgentCfg{MaxSteps: 3},
	}
	m := New(cfg, "", llm.New(), "vtest")
	mod, _ := m.Update(tea.WindowSizeMsg{Width: 110, Height: 32})
	return mod.(Model)
}

// Esc mentre el proveïdor no respon ha d'aturar el torn de seguida, i la
// capçalera ha de quedar quieta. Si això falla, «ho he parat i la mar
// encara es mou» és exactament el que es veu.
func TestEscAturaProveidorPenjat(t *testing.T) {
	m := modelContraPenjat(t)
	mod, cmd := m.startAgent("hola, hi ha algú?")
	m = mod.(Model)
	if !m.busy || m.cancel == nil {
		t.Fatalf("el torn hauria d'estar en marxa amb cancel: busy=%v cancel=%v", m.busy, m.cancel != nil)
	}
	// El Batch porta el pas del model, el tick i la detecció de finestres;
	// s'executa tot en segon pla i es recull el missatge del pas.
	msgs := make(chan tea.Msg, 8)
	go func() {
		for _, c := range []tea.Cmd{cmd} {
			if c == nil {
				continue
			}
			recull(c, msgs)
		}
	}()
	// L'usuari s'espera un moment i prem Esc.
	time.Sleep(300 * time.Millisecond)
	mod, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = mod.(Model)
	if !m.cancelRequested {
		t.Fatal("Esc ha de demanar la cancel·lació")
	}
	// El pas ha de tornar de pressa (la petició s'ha d'haver tallat).
	var pas agentStepMsg
	esperant := time.After(5 * time.Second)
	for trobat := false; !trobat; {
		select {
		case msg := <-msgs:
			if p, ok := msg.(agentStepMsg); ok {
				pas, trobat = p, true
			}
		case <-esperant:
			t.Fatal("la petició al proveïdor penjat no s'ha tallat en 5 s: la cancel·lació no arriba a la xarxa")
		}
	}
	mod, _ = m.Update(pas)
	m = mod.(Model)
	if m.busy || m.agentActive || m.cancel != nil {
		t.Fatalf("després de cancel·lar no pot quedar res en marxa: busy=%v agent=%v cancel=%v", m.busy, m.agentActive, m.cancel != nil)
	}
	// Dos ticks més: la capçalera no pot canviar (res que es mogui).
	mod, _ = m.Update(streamTickMsg{})
	m = mod.(Model)
	cap1 := strings.SplitN(m.View(), "\n", 2)[0]
	mod, _ = m.Update(streamTickMsg{})
	m = mod.(Model)
	cap2 := strings.SplitN(m.View(), "\n", 2)[0]
	if cap1 != cap2 {
		t.Fatalf("la capçalera es mou després d'aturar:\n%s\n%s", nomesText(cap1), nomesText(cap2))
	}
	if tot := nomesText(strings.Join(m.lines, "\n")); !strings.Contains(tot, T("est.agentCancel")) {
		t.Fatal("ha de dir que s'ha cancel·lat")
	}
}

// recull executa una Cmd (i els Batch que porti a dins) i envia els
// missatges pel canal.
func recull(c tea.Cmd, out chan<- tea.Msg) {
	if c == nil {
		return
	}
	msg := c()
	switch v := msg.(type) {
	case tea.BatchMsg:
		for _, sub := range v {
			go recull(sub, out)
		}
	default:
		if msg != nil {
			out <- msg
		}
	}
}

// Mentre no arriba ni un byte del proveïdor, el bloc de feina ho ha de
// dir, amb el temps que fa: «treballant…» durant minuts sense cap pista
// és el que fa pensar que s'ha penjat.
func TestAvisProveidorMut(t *testing.T) {
	m := modelContraPenjat(t)
	mod, _ := m.startAgent("hola")
	m = mod.(Model)
	// Simulem que ja fa una estona sense cap byte: el tick pinta l'espera.
	m.stream.ultim = time.Now().Add(-15 * time.Second)
	mod, _ = m.Update(streamTickMsg{})
	m = mod.(Model)
	viu := nomesText(m.lines[m.streamLine])
	if !strings.Contains(viu, "15 s") && !strings.Contains(viu, "14 s") && !strings.Contains(viu, "16 s") {
		t.Fatalf("el bloc de feina ha de dir quant fa que espera: %q", viu)
	}
	if !strings.Contains(strings.ToLower(viu), "esc") {
		t.Fatalf("i com aturar-ho: %q", viu)
	}
	if m.cancel != nil {
		m.cancel()
	}
}

// L'espera es compta des de l'últim byte del pas, no des de l'inici del
// torn: en un torn llarg, cada pas nou deia «no ha respost en 4 minuts»
// mentre el model escrivia.
func TestAvisProveidorMutNomesSiCallaDeDebo(t *testing.T) {
	m := modelContraPenjat(t)
	mod, _ := m.startAgent("hola")
	m = mod.(Model)
	m.turnStart = time.Now().Add(-4 * time.Minute) // el torn fa estona que dura
	mod, _ = m.Update(streamTickMsg{})
	m = mod.(Model)
	if viu := nomesText(m.lines[m.streamLine]); strings.Contains(viu, "no ha respost") {
		t.Fatalf("acabat d'engegar el pas, no hi ha d'haver avís: %q", viu)
	}
	// I mentre pensa, tampoc: el pensament es veu com a files.
	m.stream.addThink("Primer miro el config. " + strings.Repeat("Després penso una mica més. ", 30) + "I acabo aquí.")
	mod, _ = m.Update(streamTickMsg{})
	m = mod.(Model)
	viu := nomesText(m.lines[m.streamLine])
	if strings.Contains(viu, "no ha respost") || !strings.Contains(viu, "I acabo aquí.") {
		t.Fatalf("mentre pensa s'ha de veure el final del pensament: %q", viu)
	}
	if n := strings.Count(viu, "\n") + 1; n != filesPensament {
		t.Fatalf("el pensament es veu en %d files, no %d: %q", filesPensament, n, viu)
	}
	if m.cancel != nil {
		m.cancel()
	}
}
