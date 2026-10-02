package telegram

import (
	"testing"

	"gregal/internal/jobs"
)

func TestExcerptINeteja(t *testing.T) {
	md := "# Butlletí\n\n> resum **important**\n\n- punt u\n- punt dos\n\n---\n\ntext final"
	got := markdownToText(md)
	if got == "" || contains(got, "#") || contains(got, "---") {
		t.Fatalf("neteja pobra: %q", got)
	}
	if e := excerptOf("abcdef", 3); e != "abc…" {
		t.Fatalf("tall: %q", e)
	}
	r := jobs.Run{JobName: "Notícies", Status: "ok", Output: md}
	if title := runTitleFor(r, "ca"); title != "Notícies llest" {
		t.Fatalf("títol: %q", title)
	}
	if title := runTitleFor(r, "en"); title != "Notícies completed" {
		t.Fatalf("English title: %q", title)
	}
	r.Status = "error"
	r.Error = "boom"
	if title := runTitleFor(r, "ca"); title != "Notícies ha fallat" {
		t.Fatalf("títol error: %q", title)
	}
	if title := runTitleFor(r, "en"); title != "Notícies failed" {
		t.Fatalf("English error title: %q", title)
	}
	if e := runExcerpt(r); e != "boom" {
		t.Fatalf("extracte error: %q", e)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	})()
}
