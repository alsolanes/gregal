package tui

import (
	"reflect"
	"testing"
)

func TestExtractMentions(t *testing.T) {
	got := extractMentions("mira @main.go i @internal/x.go, no @main.go")
	want := []string{"main.go", "internal/x.go"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got=%v want=%v", got, want)
	}
	if got := extractMentions("res aquí"); len(got) != 0 {
		t.Fatalf("got=%v", got)
	}
}
