package tools

import (
	"testing"
)

func TestParseQuestionObjectOptions(t *testing.T) {
	input := `{"query": "Vols procedir?", "options": [{"label": "Sí", "description": "Continua"}, {"label": "No"}]}`
	q, opts, err := ParseQuestion(input)
	if err != nil {
		t.Fatalf("ParseQuestion error: %v", err)
	}
	if q != "Vols procedir?" {
		t.Fatalf("query: volia 'Vols procedir?', obtingut %q", q)
	}
	if len(opts) != 2 || opts[0].Label != "Sí" || opts[0].Description != "Continua" || opts[1].Label != "No" {
		t.Fatalf("options: %+v", opts)
	}
}

func TestParseQuestionStringOptions(t *testing.T) {
	input := `{"question": "Quina opció tries?", "options": ["Opció A", "Opció B", "Opció C"]}`
	q, opts, err := ParseQuestion(input)
	if err != nil {
		t.Fatalf("ParseQuestion error: %v", err)
	}
	if q != "Quina opció tries?" {
		t.Fatalf("query: %q", q)
	}
	if len(opts) != 3 || opts[0].Label != "Opció A" || opts[1].Label != "Opció B" || opts[2].Label != "Opció C" {
		t.Fatalf("options: %+v", opts)
	}
}

func TestParseQuestionMapOptions(t *testing.T) {
	input := `{"query": "Tria", "options": [{"text": "Primer"}, {"value": "Segon"}]}`
	_, opts, err := ParseQuestion(input)
	if err != nil {
		t.Fatalf("ParseQuestion error: %v", err)
	}
	if len(opts) != 2 || opts[0].Label != "Primer" || opts[1].Label != "Segon" {
		t.Fatalf("options: %+v", opts)
	}
}

func TestParseQuestionValidation(t *testing.T) {
	if _, _, err := ParseQuestion(`{"options": ["A"]}`); err == nil {
		t.Fatal("esperava error amb query buida")
	}
	if _, _, err := ParseQuestion(`{"query": "Hola", "options": []}`); err == nil {
		t.Fatal("esperava error amb options buit")
	}
	if _, _, err := ParseQuestion(`{"query": "Hola", "options": ["1", "2", "3", "4", "5"]}`); err == nil {
		t.Fatal("esperava error amb més de 4 opcions")
	}
}
