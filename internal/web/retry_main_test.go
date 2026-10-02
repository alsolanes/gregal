package web

import (
	"os"
	"testing"
	"time"

	"gregal/internal/llm"
)

// TestMain fa dues coses: escurça els reintents del client LLM (1 s, 2 s,
// 4 s… fins a 50 s; són per a proveïdors de debò que arrenquen sota demanda,
// i amb els servidors falsos dels tests farien esperar minuts) i aparta les
// dades de debò amb GREGAL_DATA_DIR. Sense això les proves desaven
// converses, tokens i contrasenyes a ~/.local/share/gregal: milers de
// fitxers al cap del temps.
func TestMain(m *testing.M) {
	llm.RetryBase, llm.RetryMax, llm.RetryJitter = 5*time.Millisecond, 20*time.Millisecond, time.Millisecond
	dir, err := os.MkdirTemp("", "gregal-web-test-")
	if err != nil {
		panic(err)
	}
	os.Setenv("GREGAL_DATA_DIR", dir)
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}
