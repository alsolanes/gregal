package agent

import (
	"os"
	"testing"
	"time"

	"gregal/internal/llm"
	"gregal/internal/lsp"
)

// Els reintents del client LLM (1 s, 2 s, 4 s… fins a 50 s) són per a
// proveïdors de debò que arrenquen sota demanda; als tests els servidors
// falsos que tornen 5xx farien esperar minuts. Aquí s'escurcen.
func TestMain(m *testing.M) {
	llm.RetryBase, llm.RetryMax, llm.RetryJitter = 5*time.Millisecond, 20*time.Millisecond, time.Millisecond
	// L'eina lsp només s'ofereix si hi ha un servidor de llenguatge al PATH
	// (els runners de GitHub porten gopls): els tests no han de dependre de
	// què hi ha instal·lat a la màquina.
	lsp.Configure(nil, true)
	os.Exit(m.Run())
}
