package web

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestChartKindsPersistThroughSharedAPIAndDashboardTool(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	s := &Server{cwd: t.TempDir(), id: "charts"}
	kinds := []string{"line", "bar", "scatter", "area", "horizontal-bar", "pie", "doughnut", "radar"}
	for revision, kind := range kinds {
		p := plotSpec{Title: kind, Type: kind, Labels: []string{"1", "2", "3"}, Series: []plotSeries{{Label: "data", Values: []float64{0, 1, 2}}}}
		raw, err := json.Marshal(map[string]any{"revision": revision, "spec": p})
		if err != nil {
			t.Fatal(err)
		}
		w := httptest.NewRecorder()
		s.handlePlots(w, httptest.NewRequest("POST", "/api/plots", strings.NewReader(string(raw))))
		if w.Code != 200 {
			t.Fatalf("%s: %d %s", kind, w.Code, w.Body.String())
		}
		raw, err = json.Marshal(dashboardSaveArgs{Dashboard: "All charts", Plots: []plotSpec{p}})
		if err != nil {
			t.Fatal(err)
		}
		dashboard := &Server{cwd: s.cwd + "-dashboard", id: "charts"}
		if _, err := dashboard.dashboardSave(string(raw)); err != nil {
			t.Fatalf("dashboard %s: %v", kind, err)
		}
	}
	for _, server := range []*Server{s, {cwd: s.cwd + "-dashboard"}} {
		w := httptest.NewRecorder()
		server.handlePlots(w, httptest.NewRequest("GET", "/api/plots", nil))
		var lib plotLibrary
		if err := json.Unmarshal(w.Body.Bytes(), &lib); err != nil {
			t.Fatal(err)
		}
		if len(lib.Plots) != len(kinds) {
			t.Fatalf("persisted %d charts", len(lib.Plots))
		}
		for i, p := range lib.Plots {
			if p.Spec.Type != kinds[i] || p.Spec.Series[0].Values[2] != 2 {
				t.Fatalf("changed data: %+v", p)
			}
		}
	}
}

func TestChartKindsRejectAliasesAndPreserveGenericCaps(t *testing.T) {
	p := plotSpec{Title: "Bounds", Type: "line", Labels: []string{"1"}, Series: []plotSeries{{Label: "v", Values: []float64{1}}}}
	for _, kind := range []string{"", "Line", "horizontalBar", "donut", "polarArea", "bubble", "html"} {
		p.Type = kind
		if validatePlot(p) == nil {
			t.Fatalf("unsupported type %q accepted", kind)
		}
	}
	p.Type = "area"
	p.Labels = make([]string, 2000)
	p.Series[0].Values = make([]float64, 2000)
	if err := validatePlot(p); err != nil {
		t.Fatal(err)
	}
	p.Labels = append(p.Labels, "extra")
	p.Series[0].Values = append(p.Series[0].Values, 0)
	if validatePlot(p) == nil {
		t.Fatal("2001 labels accepted")
	}
	p.Labels = []string{"x"}
	p.Series = make([]plotSeries, 20)
	for i := range p.Series {
		p.Series[i] = plotSeries{Label: "v", Values: []float64{0}}
	}
	if err := validatePlot(p); err != nil {
		t.Fatal(err)
	}
	p.Series = append(p.Series, p.Series[0])
	if validatePlot(p) == nil {
		t.Fatal("21 series accepted")
	}
}
