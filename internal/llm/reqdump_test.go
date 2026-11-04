package llm

import (
	"path/filepath"
	"testing"
)

// Sense HOME (Windows) els dumps no poden acabar en un camí relatiu.
func TestReqDumpDirIsNeverRelative(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", "")
	t.Setenv("HOME", "")
	t.Setenv("USERPROFILE", "")
	if d := reqDumpDir(); d != "" && !filepath.IsAbs(d) {
		t.Fatalf("camí relatiu: %q", d)
	}
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	if d := reqDumpDir(); !filepath.IsAbs(d) || filepath.Base(d) != "req-dumps" {
		t.Fatalf("amb GREGAL_DATA_DIR: %q", d)
	}
}
