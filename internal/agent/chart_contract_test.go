package agent

import (
	"reflect"
	"strings"
	"testing"
)

func TestDashboardChartContract(t *testing.T) {
	want := []string{"line", "bar", "scatter", "area", "horizontal-bar", "pie", "doughnut", "radar"}
	for _, tool := range Specs() {
		if tool.Name != "dashboard_save" {
			continue
		}
		props := tool.Parameters["properties"].(map[string]any)
		plot := props["plots"].(map[string]any)["items"].(map[string]any)
		chartProps := plot["properties"].(map[string]any)
		got := chartProps["type"].(map[string]any)["enum"]
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("chart enum = %v; want %v", got, want)
		}
		for _, phrase := range append(want, "one series", "nonnegative", "positive", "3–32", "Android", "2000", "20 series") {
			if !strings.Contains(tool.Description, phrase) {
				t.Errorf("dashboard description missing %q", phrase)
			}
		}
		return
	}
	t.Fatal("dashboard_save missing")
}
