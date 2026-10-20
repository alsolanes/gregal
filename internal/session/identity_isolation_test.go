package session

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gregal/internal/llm"
)

func TestQ1SessionDirectoriesPreserveExactIdentity(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	if got := DirFor(""); got != DefaultDir() {
		t.Fatalf("local directory changed: %s", got)
	}
	seen := map[string]string{}
	for _, name := range []string{"anna_b", "anna-b", "Anna", "anna", "anna!", "../anna", " anna", "anna ", "à", "a", strings.Repeat("A", 512)} {
		dir := DirFor(name)
		if other, exists := seen[dir]; exists {
			t.Errorf("%q and %q share history directory", other, name)
		}
		seen[dir] = name
		if rel, err := filepath.Rel(DefaultDir(), dir); err != nil || rel == "." || strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
			t.Errorf("unsafe directory: %s", dir)
		}
		if len(filepath.Base(dir)) > 255 {
			t.Error("overlong identity component")
		}
	}
}

func TestQ1SessionLegacyDirectoriesRemainUnclaimed(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	legacy := filepath.Join(DefaultDir(), "anna-b")
	_, err := Save(legacy, "private", "chat", []llm.Message{{Role: "user", Content: "unverified-owner"}})
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"anna_b", "anna-b"} {
		if entries, err := List(DirFor(name)); err != nil || len(entries) != 0 {
			t.Errorf("%q claimed legacy history: %+v %v", name, entries, err)
		}
	}
	if _, err := os.Stat(filepath.Join(legacy, "private.json")); err != nil {
		t.Fatalf("legacy data not preserved: %v", err)
	}
}

func TestQ1SessionHistoryRoundTripIsolation(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	names := []string{"anna_b", "anna-b", "Anna", "anna"}
	for _, name := range names {
		if _, err := Save(DirFor(name), "same-conversation", "chat", []llm.Message{{Role: "user", Content: name}}); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range names {
		got, err := Load(DirFor(name), "same-conversation")
		if err != nil || len(got.Convo) != 1 || got.Convo[0].Content != name {
			t.Errorf("%q loaded foreign history: %+v %v", name, got, err)
		}
	}
	if err := Delete(DirFor(names[0]), "same-conversation"); err != nil {
		t.Fatal(err)
	}
	for _, name := range names[1:] {
		if _, err := Load(DirFor(name), "same-conversation"); err != nil {
			t.Errorf("deleting other identity history affected %q: %v", name, err)
		}
	}
}
