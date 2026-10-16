package ooxml

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const sampleDeck = `{
 "title": "Pla de migració 2027",
 "theme": "default",
 "slides": [
  {"layout":"title","title":"Pla de migració 2027","subtitle":"Comitè de direcció · octubre 2026"},
  {"layout":"content","title":"Per què migrem","bullets":["Costos d'infraestructura +38% en dos anys","Plataforma actual sense suport des del 2027","  Riscos de seguretat sense pegats","Temps de desplegament: 3 setmanes"],"notes":"Començar per l'impacte econòmic."},
  {"layout":"section","title":"Opcions","subtitle":"Tres camins avaluats"},
  {"layout":"comparison","title":"Núvol públic vs. privat","left_title":"Públic","left":"- Escala automàtica\n- Pagament per ús","right_title":"Privat","right":"- Control total\n- Inversió inicial alta"},
  {"layout":"big_number","title":"Estalvi anual estimat","number":"1,2 M€","caption":"a partir del segon any, amb l'opció B"},
  {"layout":"content","title":"Cost per opció","chart":{"type":"column","categories":["A","B","C"],"series":[{"name":"Any 1","values":[120,85,200]},{"name":"Any 2","values":[90,60,150]}]}},
  {"layout":"two_columns","title":"Calendari","left":"1. Pilot (T1)\n2. Migració (T2–T3)\n3. Tancament (T4)","table":[["Fase","Inici","Durada"],["Pilot","Gen","2 mesos"],["Migració","Abr","5 mesos"],["Tancament","Oct","2 mesos"]]},
  {"layout":"content","title":"Repartiment del pressupost","chart":{"type":"doughnut","categories":["Infra","Equip","Llicències"],"series":[{"name":"Pressupost","values":[50,35,15]}]}},
  {"layout":"closing","title":"Gràcies","subtitle":"Preguntes?"}
 ]
}`

func TestBuildDeckIsValid(t *testing.T) {
	spec, err := ParseDeckSpec(sampleDeck)
	if err != nil {
		t.Fatal(err)
	}
	p := NewPptx(GetTheme(spec.Theme), spec.Title)
	warns, err := p.BuildSlides(spec)
	if err != nil {
		t.Fatal(err)
	}
	if len(warns) > 0 {
		t.Fatalf("avisos: %v", warns)
	}
	data, err := p.Pkg.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	pk, err := OpenBytes(data)
	if err != nil {
		t.Fatal(err)
	}
	if iss := Validate(pk); len(iss) > 0 {
		t.Fatalf("problemes: %v", iss)
	}
	p2, _ := OpenPptx(pk)
	if n := len(p2.SlideParts()); n != 9 {
		t.Fatalf("diapositives: %d", n)
	}
	inv := Inventory(p2)
	if !strings.Contains(inv, "1,2 M€") || !strings.Contains(inv, "Comitè de direcció") {
		t.Fatalf("inventari:\n%s", inv)
	}
	if out := os.Getenv("GREGAL_OOXML_OUT"); out != "" {
		os.WriteFile(filepath.Join(out, "deck.pptx"), data, 0o644)
	}
}

func TestDeckFromMarkdown(t *testing.T) {
	d := DeckFromMarkdown("Informe", "# Context\n- a\n- b\n## Resultats\nText")
	if len(d.Slides) != 3 || d.Slides[0].Layout != "title" || d.Slides[2].Title != "Resultats" {
		t.Fatalf("%+v", d.Slides)
	}
}

func TestTemplateDeckOps(t *testing.T) {
	p, err := PptxFromTemplate("testdata/lo.pptx")
	if err != nil {
		t.Fatal(err)
	}
	before := len(p.SlideParts())
	if _, err := p.DuplicateSlide(0, -1); err != nil {
		t.Fatal(err)
	}
	if err := p.MoveSlide(before, 0); err != nil {
		t.Fatal(err)
	}
	if err := p.DeleteSlide(1); err != nil {
		t.Fatal(err)
	}
	if _, err := p.BuildSlides(DeckSpec{Slides: []SlideSpec{{Layout: "content", Title: "Nova", Bullets: []string{"u", "dos"}}, {Layout: "big_number", Title: "Xifra", Number: "42%"}}}); err != nil {
		t.Fatal(err)
	}
	n, err := FillShapes(p, 0, map[string]string{"title": "Títol nou"})
	if err != nil || n != 1 {
		t.Fatalf("fill: %d %v", n, err)
	}
	data, _ := p.Pkg.Bytes()
	pk, _ := OpenBytes(data)
	orig, _ := Open("testdata/lo.pptx")
	if iss := NewIssues(pk, orig); len(iss) > 0 {
		t.Fatalf("problemes nous: %v", iss)
	}
	p2, _ := OpenPptx(pk)
	if got := len(p2.SlideParts()); got != before+2 {
		t.Fatalf("diapositives %d, esperades %d", got, before+2)
	}
	inv := Inventory(p2)
	if !strings.Contains(inv, "Títol nou") || !strings.Contains(inv, "42%") {
		t.Fatalf("inventari:\n%s", inv)
	}
	if out := os.Getenv("GREGAL_OOXML_OUT"); out != "" {
		os.WriteFile(filepath.Join(out, "tpl.pptx"), data, 0o644)
	}
}
