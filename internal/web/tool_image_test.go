package web

import (
	"strings"
	"testing"
)

func TestToolResultImageOnlyForBrowserScreenshots(t *testing.T) {
	shot := "data:image/jpeg;base64,/9j/4AAQSkZJRg=="
	if got := toolResultImage("browser", []string{shot}); got != shot {
		t.Fatalf("browser screenshot must reach the tool card: %q", got)
	}
	for _, c := range []struct {
		name string
		imgs []string
	}{
		{"read_image", []string{shot}},
		{"browser", nil},
		{"browser", []string{"https://evil.example/x.png"}},
		{"browser", []string{"data:image/jpeg;base64," + strings.Repeat("A", maxToolResultImage)}},
	} {
		if got := toolResultImage(c.name, c.imgs); got != "" {
			t.Errorf("%s: unexpected image (%d bytes)", c.name, len(got))
		}
	}
}
