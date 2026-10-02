package tui

import (
	"strings"
	"testing"

	"gregal/internal/flow"
)

func grafDeProva() *flow.Flow {
	return &flow.Flow{
		Name: "tests verds",
		Desc: "prova-ho i arregla-ho",
		Nodes: []flow.Node{
			{ID: "tests", Kind: flow.KindTool, Title: "Executa els tests", Tool: "bash", Args: `{"command":"go test ./..."}`},
			{ID: "arregla", Kind: flow.KindAgent, Task: "Arregla\nel que falli"},
		},
		Edges: []flow.Edge{{From: "tests", To: "arregla", When: `tests conté "FAIL"`}},
	}
}

// El dibuix ha de dir les tres coses que necessites saber abans d'executar
// un graf: per on comença, què fa cada pas i quan s'agafa cada fletxa.
func TestDibuixaFlow(t *testing.T) {
	out := dibuixaFlow(grafDeProva())
	for _, vol := range []string{"tests verds", "prova-ho i arregla-ho", "▸", "tests", "arregla", `tests conté "FAIL"`, "Executa els tests"} {
		if !strings.Contains(out, vol) {
			t.Errorf("el dibuix no diu %q:\n%s", vol, out)
		}
	}
	// Un pas d'agent sense títol s'identifica per la primera línia de la
	// tasca; ensenyar-la sencera trencaria el dibuix.
	if !strings.Contains(out, "Arregla") || strings.Contains(out, "el que falli") {
		t.Errorf("del pas sense títol només n'ha de sortir la primera línia:\n%s", out)
	}
}

// lipgloss tracta un text amb "\n" com un bloc i el farceix: una línia amb
// el salt a dins feia que la següent sortís escopida cap a la dreta. Cap
// línia del dibuix no ha de començar amb una pila d'espais.
func TestDibuixaFlowNoDesplaçaLinies(t *testing.T) {
	for _, l := range strings.Split(dibuixaFlow(grafDeProva()), "\n") {
		if n := len(l) - len(strings.TrimLeft(l, " ")); n > 6 {
			t.Errorf("línia desplaçada %d espais: %q", n, l)
		}
	}
}

func TestPrimeraLinia(t *testing.T) {
	if got := primeraLinia("  una\ndues  "); got != "una" {
		t.Fatalf("%q", got)
	}
	llarg := strings.Repeat("à", 100)
	got := primeraLinia(llarg)
	// Es compta en runes, no en bytes: amb accents, tallar per bytes parteix
	// un caràcter pel mig i el terminal escup un interrogant.
	if len([]rune(got)) != 61 || !strings.HasSuffix(got, "…") {
		t.Fatalf("retall dolent: %d runes", len([]rune(got)))
	}
}

// El TUI només ha de pintar cada pas una vegada, encara que el batec passi
// deu cops entre pas i pas.
func TestFlowRunNoRepeteixPassos(t *testing.T) {
	fr := &flowRun{nom: "x"}
	if nous, acabat := fr.nous(); len(nous) != 0 || acabat {
		t.Fatalf("de bon principi no hi ha res: %v %v", nous, acabat)
	}
	fr.afegeix(flow.StepResult{Node: "a"})
	fr.afegeix(flow.StepResult{Node: "b"})
	nous, acabat := fr.nous()
	if len(nous) != 2 || acabat {
		t.Fatalf("volia 2 passos sense acabar: %d %v", len(nous), acabat)
	}
	if nous, _ := fr.nous(); len(nous) != 0 {
		t.Fatalf("els mateixos passos no s'han de tornar dos cops: %v", nous)
	}
	fr.afegeix(flow.StepResult{Node: "c"})
	fr.acaba(flow.RunResult{Steps: []flow.StepResult{{Node: "a"}}}, nil)
	nous, acabat = fr.nous()
	if len(nous) != 1 || !acabat {
		t.Fatalf("volia l'últim pas i acabat: %d %v", len(nous), acabat)
	}
}

func TestFlowStepLine(t *testing.T) {
	ok := flowStepLine(flow.StepResult{Node: "a", Title: "Executa els tests"})
	if !strings.Contains(ok, "Executa els tests") || !strings.Contains(ok, "✓") {
		t.Fatalf("pas correcte: %q", ok)
	}
	// Sense títol es cau a l'id: una línia sense nom no diu res.
	if got := flowStepLine(flow.StepResult{Node: "compila"}); !strings.Contains(got, "compila") {
		t.Fatalf("sense títol ha de sortir l'id: %q", got)
	}
	mal := flowStepLine(flow.StepResult{Node: "a", Title: "Compila", Err: "exit 1"})
	if !strings.Contains(mal, "exit 1") || !strings.Contains(mal, "✗") {
		t.Fatalf("un pas fallat ha de dir per què: %q", mal)
	}
}

// El resum és el que li queda al model a la conversa: ha de portar què s'ha
// fet, si s'ha aturat i per què, i l'última sortida.
func TestResumFlow(t *testing.T) {
	res := flow.RunResult{Steps: []flow.StepResult{
		{Node: "a", Title: "Prova", Output: "ok"},
		{Node: "b", Err: "peta", Output: ""},
	}, Stopped: "el pas «b» ha fallat: peta"}
	out := resumFlow(res)
	for _, vol := range []string{"Prova (ok)", "b (error: peta)", "Aturat:"} {
		if !strings.Contains(out, vol) {
			t.Errorf("el resum no diu %q:\n%s", vol, out)
		}
	}
}

// Sense grafs al projecte, /graf ho diu i diu on es fan. No ha de petar ni
// obrir un menú buit.
func TestGrafSenseGrafs(t *testing.T) {
	m := &Model{cwd: t.TempDir()}
	m.flowCommand("")
	if m.menu != nil {
		t.Fatal("sense grafs no s'ha d'obrir cap menú")
	}
	text := strings.Join(m.lines, "\n")
	if !strings.Contains(text, flow.Dir) {
		t.Fatalf("hauria de dir on van els grafs:\n%s", text)
	}
}

func TestGrafObreMenu(t *testing.T) {
	dir := t.TempDir()
	if _, err := flow.Desa(dir, grafDeProva()); err != nil {
		t.Fatal(err)
	}
	m := &Model{cwd: dir}
	m.flowCommand("")
	if m.menu == nil || len(m.menu.items) != 1 {
		t.Fatalf("volia un menú amb un graf: %+v", m.menu)
	}
	if !strings.Contains(m.menu.items[0].text, "tests verds") {
		t.Fatalf("item=%q", m.menu.items[0].text)
	}
	// Triar-lo ensenya el dibuix, no l'executa: executar toca fitxers.
	msg, seg := m.menu.run(m, m.menu.items[0].val)
	if seg != nil {
		t.Fatal("triar un graf no ha d'obrir cap submenú")
	}
	if !strings.Contains(msg, "/graf tests verds") {
		t.Fatalf("ha de dir com executar-lo:\n%s", msg)
	}
}

// Un graf que no existeix no ha de deixar el TUI ocupat per sempre.
func TestGrafInexistentNoDeixaOcupat(t *testing.T) {
	dir := t.TempDir()
	if _, err := flow.Desa(dir, grafDeProva()); err != nil {
		t.Fatal(err)
	}
	m := &Model{cwd: dir}
	m.startFlow("no-hi-es")
	if m.flow != nil || m.busy {
		t.Fatalf("flow=%v busy=%v", m.flow, m.busy)
	}
	if !strings.Contains(strings.Join(m.lines, "\n"), "no hi ha cap flux") {
		t.Fatalf("hauria de dir que no hi és:\n%s", strings.Join(m.lines, "\n"))
	}
}
