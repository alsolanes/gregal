package pdfdoc

import (
	"testing"

	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/color"
)

func TestNormalizeColorAcceptsHexWithoutHash(t *testing.T) {
	for in, want := range map[string]string{"FFFFFF": "#FFFFFF", "#ff0000": "#FF0000", "0x00ff00": "#00FF00", "#fff": "#FFFFFF", "abc": "#AABBCC", "red": "red", "": ""} {
		if got := normalizeColor(in); got != want {
			t.Errorf("normalizeColor(%q) = %q, want %q", in, got, want)
		}
		if want != "" {
			if _, err := color.ParseColor(normalizeColor(in)); err != nil {
				t.Errorf("pdfcpu rejects %q: %v", normalizeColor(in), err)
			}
		}
	}
}
