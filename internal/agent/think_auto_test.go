package agent

import (
	"testing"

	"gregal/internal/llm"
	"gregal/internal/tools"
)

// think: auto raona on aporta i no als passos mecànics. Els altres valors
// del rol passen tal qual.
func TestThinkPerPas(t *testing.T) {
	cas := []struct {
		rol  string
		p    Pas
		vol  string
		desc string
	}{
		{"", Pas{Ordre: OrdrePasModel, Mecanic: true}, "", "sense think al rol, res"},
		{"no", Pas{Ordre: OrdrePasModel}, "no", "no és no"},
		{"auto", Pas{Ordre: OrdrePasModel}, "", "un pas que decideix, raona"},
		{"auto", Pas{Ordre: OrdrePasModel, Mecanic: true}, "no", "un pas mecànic, no"},
		{"auto", Pas{Ordre: OrdreAmplia}, "no", "CONTINUA/FINAL, no"},
		{"auto", Pas{Ordre: OrdreSintesi}, "no", "el resum, no"},
	}
	for _, c := range cas {
		if got := ThinkPerPas(c.rol, c.p); got != c.vol {
			t.Errorf("%s: %q", c.desc, got)
		}
	}
}

// Només és mecànic el que ha anat bé i no porta informació nova: edicions
// aplicades i comprovacions verdes. Una lectura, una ordre qualsevol o un
// error tornen a demanar raonar.
func TestLotMecanic(t *testing.T) {
	ex := func(nom, args, sortida string) Execucio {
		return Execucio{Call: cridaProva("x", nom, args), Sortida: sortida}
	}
	cas := []struct {
		res  []Execucio
		vol  bool
		desc string
	}{
		{[]Execucio{ex("edit", `{}`, "edit aplicat a a.go")}, true, "edició aplicada"},
		{[]Execucio{ex("bash", `{"command":"go test ./..."}`, "ok\tpkg\t0.1s")}, true, "test verd"},
		{[]Execucio{ex("bash", `{"command":"go test ./..."}`, "--- FAIL: TestX\nFAIL\tpkg")}, false, "test vermell"},
		{[]Execucio{ex("read", `{"path":"a.go"}`, "package a")}, false, "lectura"},
		{[]Execucio{ex("bash", `{"command":"cat a.go"}`, "package a")}, false, "ordre que no és comprovació"},
		{[]Execucio{ex("edit", `{}`, "edit aplicat\n[sintaxi] a.go:3: expected }")}, false, "edició amb diagnòstic"},
		{[]Execucio{ex("write", `{}`, "ERROR: permís")}, false, "error"},
		{nil, false, "res"},
	}
	for _, c := range cas {
		if got := lotMecanic(c.res); got != c.vol {
			t.Errorf("%s: %v", c.desc, got)
		}
	}
}

// El motor marca el pas següent: mecànic després d'una edició que ha anat
// bé, no després d'una lectura ni d'una negativa.
func TestMotorMarcaPasMecanic(t *testing.T) {
	tools.TodoClear()
	tn := tornProva(ModeCode, 20)
	if p := tn.Seguent(); p.Mecanic {
		t.Fatal("el primer pas planifica: no és mecànic")
	}
	tn.RepPas("", []llm.ToolCall{cridaProva("r1", "read", `{"path":"a.go"}`)}, nil)
	p := tn.Seguent()
	tn.RepExecucions([]Execucio{{Call: p.Calls[0], Sortida: "package a"}})
	if p := tn.Seguent(); p.Ordre != OrdrePasModel || p.Mecanic {
		t.Fatalf("després de llegir es decideix: %v %v", p.Ordre, p.Mecanic)
	}
	edit := cridaProva("e1", "edit", `{"path":"a.go","old_string":"a","new_string":"b"}`)
	tn.RepPas("", []llm.ToolCall{edit}, nil)
	finsAExecuta(t, tn, []Execucio{{Call: edit, Sortida: "edit aplicat a a.go"}})
	if p := tn.Seguent(); p.Ordre != OrdrePasModel || !p.Mecanic {
		t.Fatalf("després d'una edició aplicada, pas mecànic: %v %v", p.Ordre, p.Mecanic)
	}
}
