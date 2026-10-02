package tui

import (
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"gregal/internal/llm"
	"gregal/internal/session"
)

func desaSessio(t *testing.T, dir, nom, rol string, convo []llm.Message) {
	t.Helper()
	if _, err := session.SaveSession(dir, session.Session{Name: nom, Role: rol, Convo: convo}); err != nil {
		t.Fatal(err)
	}
}

// Reprendre restaura conversa i rol (si encara existeix al config).
func TestResumeRestauraRol(t *testing.T) {
	dir := t.TempDir()
	desaSessio(t, dir, "feta", "code", []llm.Message{{Role: "user", Content: "hola"}})
	m := modelDeBarra(t, 100)
	if m.role != "chat" {
		// modelDeBarra no fixa rol: sigui quin sigui, el resume el canvia.
	}
	m.role = "chat"
	m.resumeSessionDir(dir, "feta")
	if m.role != "code" {
		t.Fatalf("rol=%q, volia code", m.role)
	}
	if len(m.convo) != 1 || m.convo[0].Content != "hola" {
		t.Fatalf("conversa mal restaurada: %+v", m.convo)
	}
}

// Rol desaparegut del config: s'avisa i es queda l'actual.
func TestResumeRolPerdutAvisa(t *testing.T) {
	dir := t.TempDir()
	desaSessio(t, dir, "vella", "fantasma", []llm.Message{{Role: "user", Content: "hola"}})
	m := modelDeBarra(t, 100)
	m.role = "chat"
	m.resumeSessionDir(dir, "vella")
	if m.role != "chat" {
		t.Fatalf("rol=%q, havia de quedar chat", m.role)
	}
	if tot := strings.Join(m.lines, "\n"); !strings.Contains(tot, "ja no existeix") {
		t.Fatalf("cal avís de rol perdut:\n%s", tot)
	}
}

// Doble Enter antic desava l'usuari dos cops: en reprendre es veu un.
func TestResumeTreuDuplicats(t *testing.T) {
	dir := t.TempDir()
	desaSessio(t, dir, "dup", "chat", []llm.Message{
		{Role: "user", Content: "fes-ho"},
		{Role: "user", Content: "fes-ho"},
		{Role: "assistant", Content: "fet"},
		{Role: "assistant", Content: "fet"},
	})
	m := modelDeBarra(t, 100)
	m.resumeSessionDir(dir, "dup")
	if len(m.convo) != 2 {
		t.Fatalf("els duplicats seguits sobren: %+v", m.convo)
	}
}

// Sessió d'un altre directori: el TUI restaura el workspace abans de seguir.
func TestResumeAltreDirAvisa(t *testing.T) {
	dir := t.TempDir()
	workspace := t.TempDir()
	_, err := session.SaveSession(dir, session.Session{
		Name: "aliena", Role: "chat", Workspace: workspace,
		Convo: []llm.Message{{Role: "user", Content: "hola"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	m := modelDeBarra(t, 100)
	m.resumeSessionDir(dir, "aliena")
	if m.cwd != workspace {
		t.Fatalf("cal restaurar el workspace: got=%q want=%q", m.cwd, workspace)
	}
	if tot := strings.Join(m.lines, "\n"); !strings.Contains(tot, "workspace restaurat") {
		t.Fatalf("cal informar del workspace restaurat:\n%s", tot)
	}
}

func TestResumeWorkspaceAbsentConservaLaConversa(t *testing.T) {
	dir := t.TempDir()
	_, err := session.SaveSession(dir, session.Session{
		Name: "trencada", Role: "chat", Workspace: filepath.Join(t.TempDir(), "ja-no-hi-es"),
		Convo: []llm.Message{{Role: "user", Content: "antiga"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	m := modelDeBarra(t, 100)
	m.convo = []llm.Message{{Role: "user", Content: "actual"}}
	m.sessionFile = "actual"
	m.resumeSessionDir(dir, "trencada")
	if len(m.convo) != 1 || m.convo[0].Content != "actual" || m.sessionFile != "actual" {
		t.Fatalf("una represa amb workspace absent ha canviat l'estat: convo=%+v file=%q", m.convo, m.sessionFile)
	}
}

// /resume N continua la N (1 = recent); nom encara funciona; número
// dolent avisa sense fer res.
func TestResumeNumeroINom(t *testing.T) {
	dir := t.TempDir()
	desaSessio(t, dir, "vella", "chat", []llm.Message{{Role: "user", Content: "primer"}})
	desaSessio(t, dir, "nova", "chat", []llm.Message{{Role: "user", Content: "segon"}})
	m := modelDeBarra(t, 100)
	m.resumeTriada(dir, "1")
	if len(m.convo) != 1 || m.convo[0].Content != "segon" {
		t.Fatalf("/resume 1 ha de ser la recent: %+v", m.convo)
	}
	m.resumeTriada(dir, "2")
	if len(m.convo) != 1 || m.convo[0].Content != "primer" {
		t.Fatalf("/resume 2 ha de ser l'anterior: %+v", m.convo)
	}
	m.resumeTriada(dir, "vella")
	if len(m.convo) != 1 || m.convo[0].Content != "primer" {
		t.Fatalf("per nom encara funciona: %+v", m.convo)
	}
	m.resumeTriada(dir, "9")
	if tot := strings.Join(m.lines, "\n"); !strings.Contains(tot, "cap sessió amb número") {
		t.Fatalf("número dolent ha d'avisar:\n%s", tot)
	}
}

// Clicar una fila "N · ..." la reprèn; fora de files, res.
func TestClicSessioResum(t *testing.T) {
	dir := t.TempDir()
	desaSessio(t, dir, "vella", "chat", []llm.Message{{Role: "user", Content: "primer"}})
	desaSessio(t, dir, "nova", "chat", []llm.Message{{Role: "user", Content: "segon"}})
	m := modelDeBarra(t, 100)
	m.sessDir = dir
	// Neteja la benvinguda i pinta només la llista: files 3.. = viewport.
	m.lines = nil
	m.push(llistaSessions(dir))
	clic := func(y int) Model {
		mod, _ := m.Update(tea.MouseMsg{X: 5, Y: y, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress})
		m = mod.(Model)
		return m
	}
	m.convo = nil
	clic(5) // fila "2 · ..." = "vella"
	if len(m.convo) != 1 || m.convo[0].Content != "primer" {
		t.Fatalf("el clic a la 2 ha de reprendre la vella: %+v", m.convo)
	}
	m.convo = nil
	clic(0) // capçalera: res
	if len(m.convo) != 0 {
		t.Fatalf("clic fora de files no fa res: %+v", m.convo)
	}
	// En feina, els clics s'ignoren (no roben torns).
	m.busy = true
	clic(3)
	m.busy = false
	if len(m.convo) != 0 {
		t.Fatal("en feina el clic no fa res")
	}
}

// La pista inicial diu quantes n'hi ha i com continuar; buida si cap.
func TestSessionsHint(t *testing.T) {
	dir := t.TempDir()
	if h := sessionsHint(dir); h != "" {
		t.Fatalf("sense sessions no hi ha pista: %q", h)
	}
	desaSessio(t, dir, "una", "chat", []llm.Message{{Role: "user", Content: "primera"}})
	desaSessio(t, dir, "dos", "chat", []llm.Message{{Role: "user", Content: "segona"}})
	h := sessionsHint(dir)
	if !strings.Contains(h, "2") || !strings.Contains(h, "/resume") {
		t.Fatalf("pista pobra: %q", h)
	}
}

// /sessions mostra títols llegibles, no només noms de fitxer.
func TestLlistaSessionsTitols(t *testing.T) {
	dir := t.TempDir()
	desaSessio(t, dir, "x", "chat", []llm.Message{{Role: "user", Content: "fes el dinar"}})
	if tot := llistaSessions(dir); !strings.Contains(tot, "fes el dinar") {
		t.Fatalf("cal el títol, no el slug:\n%s", tot)
	}
	if buit := llistaSessions(t.TempDir()); !strings.Contains(buit, T("app.capSessio")) {
		t.Fatalf("sense sessions cal avís:\n%s", buit)
	}
}

// /pin actualitza la mateixa conversa i no crea una còpia nova.
func TestPinTogglePersisteixLaConversaActual(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	m := modelDeBarra(t, 100)
	m.cwd = t.TempDir()
	m.convo = []llm.Message{{Role: "user", Content: "fixa'm"}}

	mod, _ := m.runCommand("/pin")
	m = mod.(Model)
	if !m.pinned || m.sessionFile == "" {
		t.Fatalf("/pin no ha activat l'estat: pinned=%v file=%q", m.pinned, m.sessionFile)
	}
	s, err := session.Load(session.DefaultDir(), m.sessionFile)
	if err != nil {
		t.Fatal(err)
	}
	if !s.Pinned || len(s.Convo) != 1 {
		t.Fatalf("persistència de pin incorrecta: %+v", s)
	}

	mod, _ = m.runCommand("/pin")
	m = mod.(Model)
	s, err = session.Load(session.DefaultDir(), m.sessionFile)
	if err != nil {
		t.Fatal(err)
	}
	if m.pinned || s.Pinned {
		t.Fatalf("/pin no ha desfixat: model=%v sessió=%v", m.pinned, s.Pinned)
	}
}

// La conversa es desa en acabar el torn, no només amb /quit: tancar la
// finestra del terminal feia perdre hores de feina.
func TestLaSessioEsDesaEnAcabarElTorn(t *testing.T) {
	arrel := t.TempDir()
	t.Setenv("GREGAL_DATA_DIR", arrel)
	m := modelDeBarra(t, 100)
	m.convo = []llm.Message{{Role: "user", Content: "fes un joc de plataformes"}}
	m.torn = nil
	mod, _ := m.tancaTorn()
	m = mod.(Model)
	infos, err := session.List(filepath.Join(arrel, "sessions"))
	if err != nil || len(infos) != 1 {
		t.Fatalf("en acabar el torn la sessió ha de ser a disc: %v %+v", err, infos)
	}
	if infos[0].Msgs != 1 {
		t.Fatalf("amb el missatge dins: %+v", infos[0])
	}
	// I el torn següent escriu a la mateixa sessió, no en crea una de nova.
	m.convo = append(m.convo, llm.Message{Role: "assistant", Content: "fet"})
	mod, _ = m.tancaTorn()
	if infos, _ := session.List(filepath.Join(arrel, "sessions")); len(infos) != 1 || infos[0].Msgs != 2 {
		t.Fatalf("el segon torn actualitza la mateixa sessió: %+v", infos)
	}
}
