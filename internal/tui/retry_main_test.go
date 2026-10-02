package tui

import (
	"os"
	"testing"
	"time"

	"gregal/internal/llm"
)

// TestMain fa dues coses: escurça els reintents del client LLM (1 s, 2 s,
// 4 s… fins a 50 s; són per a proveïdors de debò que arrenquen sota demanda,
// i amb els servidors falsos dels tests farien esperar minuts) i aparta les
// dades de debò amb GREGAL_DATA_DIR, com fa el paquet web. Sense això cada
// passada de la suite desava converses «Fet.» a ~/.local/share/gregal, i
// la llista de converses de la web i el TUI se n'omplia (n'hi havia
// centenars el 2026-09-28).
func TestMain(m *testing.M) {
	// Existing snapshots and text assertions explicitly exercise Catalan.
	SetIdioma("ca")
	llm.RetryBase, llm.RetryMax, llm.RetryJitter = 5*time.Millisecond, 20*time.Millisecond, time.Millisecond
	dir, err := os.MkdirTemp("", "gregal-tui-test-")
	if err != nil {
		panic(err)
	}
	os.Setenv("GREGAL_DATA_DIR", dir)
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}
