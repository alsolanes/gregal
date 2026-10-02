package telegram

import (
	"os"
	"testing"
	"time"

	"gregal/internal/llm"
)

// Els reintents del client LLM (1 s, 2 s, 4 s… fins a 50 s) són per a
// proveïdors de debò que arrenquen sota demanda; als tests els servidors
// falsos que tornen 5xx farien esperar minuts. Aquí s'escurcen.
func TestMain(m *testing.M) {
	llm.RetryBase, llm.RetryMax, llm.RetryJitter = 5*time.Millisecond, 20*time.Millisecond, time.Millisecond
	os.Exit(m.Run())
}
