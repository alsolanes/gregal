package llm

import "testing"

func TestSenseCridesText(t *testing.T) {
	dsml := "<｜｜DSML｜｜ calls>\n<｜｜DSML｜｜ invoke name=\"bash\">\n<｜｜DSML｜｜ parameter name=\"command\" string=\"true\">npm test</｜｜DSML｜｜ parameter>\n</｜｜DSML｜｜ invoke>\n</｜｜DSML｜｜ calls>"
	if net, ok := SenseCridesText(dsml); !ok || net != "" {
		t.Fatalf("pure DSML: %q %v", net, ok)
	}
	if net, ok := SenseCridesText("Done: 12 tests pass.\n" + dsml); !ok || net != "Done: 12 tests pass." {
		t.Fatalf("text before DSML must stay: %q %v", net, ok)
	}
	if net, ok := SenseCridesText("Done.\n<｜DSML｜calls><｜DSML｜invoke name=\"x\">"); !ok || net != "Done." {
		t.Fatalf("truncated DSML: %q %v", net, ok)
	}
	if net, ok := SenseCridesText("plain answer"); ok || net != "plain answer" {
		t.Fatalf("plain text: %q %v", net, ok)
	}
}
