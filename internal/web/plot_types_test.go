package web

import (
	"math"
	"strings"
	"testing"
)

func TestCircularPlotTypes(t *testing.T) {
	for _, kind := range []string{"pie", "doughnut"} {
		t.Run(kind, func(t *testing.T) {
			for _, tc := range []struct {
				name   string
				values []float64
				valid  bool
			}{
				{"positive", []float64{0, 2}, true}, {"huge", []float64{math.MaxFloat64, math.MaxFloat64}, true},
				{"tiny", []float64{0, math.SmallestNonzeroFloat64}, true}, {"zero", []float64{0, 0}, false},
				{"negative", []float64{-1, 2}, false}, {"nan", []float64{math.NaN(), 2}, false}, {"inf", []float64{math.Inf(1), 2}, false},
			} {
				t.Run(tc.name, func(t *testing.T) {
					p := plotSpec{Title: "Slices", Type: kind, Labels: []string{"A", "B"}, Series: []plotSeries{{Label: "one", Values: tc.values}}}
					if err := validatePlot(p); (err == nil) != tc.valid {
						t.Fatalf("valid=%v: %v", tc.valid, err)
					}
				})
			}
			p := plotSpec{Title: "Slices", Type: kind, Labels: []string{"A"}, Series: []plotSeries{{Label: "one", Values: []float64{1}}, {Label: "two", Values: []float64{1}}}}
			if validatePlot(p) == nil {
				t.Fatal("multiple circular series accepted")
			}
		})
	}
}

func TestRadarPlotType(t *testing.T) {
	for _, n := range []int{2, 3, 32, 33} {
		p := plotSpec{Title: "Radar", Type: "radar", Labels: make([]string, n), Series: []plotSeries{{Label: "zero", Values: make([]float64, n)}}}
		if err := validatePlot(p); (err == nil) != (n >= 3 && n <= 32) {
			t.Fatalf("labels=%d: %v", n, err)
		}
		if n == 3 {
			for _, value := range []float64{-1, math.Inf(1), math.NaN()} {
				p.Series[0].Values[0] = value
				if validatePlot(p) == nil {
					t.Fatalf("radar accepted %v", value)
				}
			}
		}
	}
}

func TestPlotPromptChartContract(t *testing.T) {
	for _, phrase := range []string{"line, bar, scatter, area, horizontal-bar, pie, doughnut, radar", "one series", "nonnegative", "positive", "3–32", "signed", "Android", "2000 labels and 20 series"} {
		if !strings.Contains(plotPrompt, phrase) {
			t.Errorf("plot prompt missing %q", phrase)
		}
	}
}

func TestSignedCategoryPlotTypes(t *testing.T) {
	for _, kind := range []string{"line", "bar", "area", "horizontal-bar"} {
		t.Run(kind, func(t *testing.T) {
			p := plotSpec{Title: "Signed", Type: kind, Labels: []string{"A", "B", "C"}, Series: []plotSeries{{Label: "one", Values: []float64{-2, 0, 3}}, {Label: "two", Values: []float64{1, -3, 0}}}}
			if err := validatePlot(p); err != nil {
				t.Fatal(err)
			}
		})
	}
}
