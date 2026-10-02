package flow

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// fals és un Runner sense model: torna el que li diguis i apunta què li han
// demanat. Tot el motor es prova sense cap proveïdor.
type fals struct {
	agent  map[string]string
	tool   map[string]string
	errors map[string]error
	vist   []string
}

func (f *fals) Agent(_ context.Context, task, mode string, maxSteps int) (string, error) {
	f.vist = append(f.vist, "agent:"+task)
	for k, v := range f.agent {
		if strings.Contains(task, k) {
			if e, hi := f.errors[k]; hi {
				return "", e
			}
			return v, nil
		}
	}
	return "resposta per: " + task, nil
}

func (f *fals) Tool(_ context.Context, name, args string) (string, error) {
	f.vist = append(f.vist, "tool:"+name+" "+args)
	if e, hi := f.errors[name]; hi {
		return "", e
	}
	if v, hi := f.tool[name]; hi {
		return v, nil
	}
	return "sortida de " + name, nil
}

func lineal() *Flow {
	return &Flow{
		Name: "revisió",
		Nodes: []Node{
			{ID: "llegir", Kind: KindTool, Tool: "read", Args: `{"path":"calc.go"}`},
			{ID: "revisar", Kind: KindAgent, Task: "Revisa això:\n{{llegir}}", Mode: "chat"},
		},
		Edges: []Edge{{From: "llegir", To: "revisar"}},
	}
}

// El cas base: dos passos, i el segon fa servir el que ha produït el primer.
func TestRunLinealPassaEstat(t *testing.T) {
	r := &fals{tool: map[string]string{"read": "func Suma(a, b int) int"}}
	res, err := Run(context.Background(), lineal(), r, Opcions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Steps) != 2 {
		t.Fatalf("volia 2 passos, en tinc %d", len(res.Steps))
	}
	if !strings.Contains(r.vist[1], "func Suma(a, b int) int") {
		t.Fatalf("el segon pas no ha rebut el que va produir el primer: %q", r.vist[1])
	}
	if res.State["llegir"] != "func Suma(a, b int) int" {
		t.Fatalf("estat=%v", res.State)
	}
	if res.State["last"] == "" {
		t.Fatal("«last» ha de portar l'última sortida")
	}
}

// La gràcia del graf: una fletxa amb condició tria camí. Aquí, «si els tests
// fallen, arregla-ho».
func TestCondicioTriaCami(t *testing.T) {
	f := &Flow{
		Name: "tests",
		Nodes: []Node{
			{ID: "test", Kind: KindTool, Tool: "bash", Args: `{"command":"go test ./..."}`},
			{ID: "arreglar", Kind: KindAgent, Task: "Arregla: {{test}}"},
			{ID: "avisar", Kind: KindAgent, Task: "Digues que tot passa"},
		},
		Edges: []Edge{
			{From: "test", To: "arreglar", When: `test conté "FAIL"`},
			{From: "test", To: "avisar", When: `test != "FAIL"`},
		},
	}
	verd := &fals{tool: map[string]string{"bash": "ok  gregal/internal/flow"}}
	res, err := Run(context.Background(), f, verd, Opcions{})
	if err != nil {
		t.Fatal(err)
	}
	if ultim, _ := res.Last(); ultim.Node != "avisar" {
		t.Fatalf("amb els tests verds havia d'anar a «avisar», ha anat a %q", ultim.Node)
	}
	roig := &fals{tool: map[string]string{"bash": "--- FAIL: TestX\nFAIL"}}
	res, err = Run(context.Background(), f, roig, Opcions{})
	if err != nil {
		t.Fatal(err)
	}
	if ultim, _ := res.Last(); ultim.Node != "arreglar" {
		t.Fatalf("amb els tests vermells havia d'anar a «arreglar», ha anat a %q", ultim.Node)
	}
}

// Un cicle a posta (torna-hi fins que passi) s'ha de poder fer, i el sostre
// ha d'evitar que doni voltes per sempre.
func TestCicleAmbSostre(t *testing.T) {
	f := &Flow{
		Name: "insistent",
		Nodes: []Node{
			{ID: "inici", Kind: KindNote, Text: "va"},
			{ID: "prova", Kind: KindTool, Tool: "bash", Args: `{"command":"go test"}`},
			{ID: "arregla", Kind: KindAgent, Task: "arregla"},
		},
		Edges: []Edge{
			{From: "inici", To: "prova"},
			{From: "prova", To: "arregla", When: `prova conté "FAIL"`},
			{From: "arregla", To: "prova"},
		},
	}
	r := &fals{tool: map[string]string{"bash": "FAIL"}}
	res, err := Run(context.Background(), f, r, Opcions{MaxPassos: 9})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Steps) != 9 {
		t.Fatalf("volia parar als 9 passos, n'ha fet %d", len(res.Steps))
	}
	if !strings.Contains(res.Stopped, "sostre") {
		t.Fatalf("hauria de dir que ha topat amb el sostre: %q", res.Stopped)
	}
}

// Si un pas falla i no hi ha cap fletxa prevista per a l'error, s'atura i ho
// diu; no continua com si res.
func TestErrorSenseSortidaAtura(t *testing.T) {
	r := &fals{errors: map[string]error{"read": errors.New("no existeix")}}
	res, err := Run(context.Background(), lineal(), r, Opcions{})
	if err == nil {
		t.Fatal("un pas fallat sense sortida ha de tornar error")
	}
	if len(res.Steps) != 1 || res.Steps[0].Err == "" {
		t.Fatalf("el pas fallat ha de quedar registrat: %+v", res.Steps)
	}
	if !strings.Contains(res.Stopped, "no existeix") {
		t.Fatalf("stopped=%q", res.Stopped)
	}
}

// Amb una fletxa que mira l'error, el graf se'n recupera tot sol.
func TestErrorAmbSortidaContinua(t *testing.T) {
	f := &Flow{
		Name: "tolerant",
		Nodes: []Node{
			{ID: "llegir", Kind: KindTool, Tool: "read", Args: `{"path":"no.go"}`},
			{ID: "pla-b", Kind: KindAgent, Task: "El fitxer no hi és: {{llegir.error}}"},
		},
		Edges: []Edge{{From: "llegir", To: "pla-b", When: "llegir.error"}},
	}
	r := &fals{errors: map[string]error{"read": errors.New("no existeix")}}
	res, err := Run(context.Background(), f, r, Opcions{})
	if err != nil {
		t.Fatal(err)
	}
	if ultim, _ := res.Last(); ultim.Node != "pla-b" {
		t.Fatalf("havia de continuar pel pla B: %+v", res.Steps)
	}
	if !strings.Contains(r.vist[1], "no existeix") {
		t.Fatalf("el pla B ha de rebre l'error: %q", r.vist[1])
	}
}

func TestValidate(t *testing.T) {
	casos := []struct {
		nom string
		f   Flow
		vol string
	}{
		{"sense nom", Flow{Nodes: []Node{{ID: "a", Kind: KindNote}}}, "nom"},
		{"sense passos", Flow{Name: "x"}, "cap pas"},
		{"id repetit", Flow{Name: "x", Nodes: []Node{{ID: "a", Kind: KindNote}, {ID: "a", Kind: KindNote}}}, "mateix id"},
		{"id rar", Flow{Name: "x", Nodes: []Node{{ID: "a b", Kind: KindNote}}}, "invàlid"},
		{"agent sense tasca", Flow{Name: "x", Nodes: []Node{{ID: "a", Kind: KindAgent}}}, "no diu què"},
		{"eina sense nom", Flow{Name: "x", Nodes: []Node{{ID: "a", Kind: KindTool}}}, "no diu quina"},
		{"mena desconeguda", Flow{Name: "x", Nodes: []Node{{ID: "a", Kind: "mag"}}}, "desconeguda"},
		{"fletxa al buit", Flow{Name: "x", Nodes: []Node{{ID: "a", Kind: KindNote}},
			Edges: []Edge{{From: "a", To: "b"}}}, "no existeix"},
		{"auto-fletxa", Flow{Name: "x", Nodes: []Node{{ID: "a", Kind: KindNote}},
			Edges: []Edge{{From: "a", To: "a"}}}, "ell mateix"},
		{"dos principis", Flow{Name: "x", Nodes: []Node{{ID: "a", Kind: KindNote}, {ID: "b", Kind: KindNote}}}, "un principi"},
		{"condició rara", Flow{Name: "x", Nodes: []Node{{ID: "a", Kind: KindNote}, {ID: "b", Kind: KindNote}},
			Edges: []Edge{{From: "a", To: "b", When: "a b c d"}}}, "no entenc"},
	}
	for _, c := range casos {
		err := c.f.Validate()
		if err == nil {
			t.Errorf("%s: havia de fallar", c.nom)
			continue
		}
		if !strings.Contains(err.Error(), c.vol) {
			t.Errorf("%s: error=%q, esperava que digués %q", c.nom, err, c.vol)
		}
	}
	if err := lineal().Validate(); err != nil {
		t.Fatalf("un graf bo no pot fallar: %v", err)
	}
}

func TestCond(t *testing.T) {
	st := State{"a": "hola món", "buit": "", "zero": "0", "no": "false"}
	casos := []struct {
		expr string
		vol  bool
	}{
		{"", true},
		{"a", true},
		{"buit", false},
		{"zero", false},
		{"no", false},
		{"!buit", true},
		{"!a", false},
		{`a == "hola món"`, true},
		{`a == "adéu"`, false},
		{`a != "adéu"`, true},
		{`a conté "món"`, true},
		{`a conté "MÓN"`, true},
		{`a conté "cel"`, false},
		{`desconeguda conté "x"`, false},
	}
	for _, c := range casos {
		got, err := ParseCond(c.expr)
		if err != nil {
			t.Errorf("%q: %v", c.expr, err)
			continue
		}
		if got.Eval(st) != c.vol {
			t.Errorf("%q = %v, volia %v", c.expr, !c.vol, c.vol)
		}
	}
}

func TestExpand(t *testing.T) {
	st := State{"nom": "Gregal", "buit": ""}
	if got := Expand("Hola {{nom}}, {{ nom }} i {{gens}}!", st); got != "Hola Gregal, Gregal i !" {
		t.Fatalf("got=%q", got)
	}
}
