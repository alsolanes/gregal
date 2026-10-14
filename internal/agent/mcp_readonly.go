package agent

import "sync"

// Eines MCP de només lectura: les dels servidors marcats `readonly: true`
// al config (la base de dades de demostració, per exemple). La política
// les tracta com una lectura: passen sense demanar permís, també en els
// modes de només lectura. Un permissions.tools explícit continua manant.

var (
	readOnlyMu  sync.RWMutex
	readOnlyMCP = map[string]bool{}
)

// SetReadOnlyMCPTools substitueix el conjunt d'eines MCP de només lectura
// (el gestor MCP el refà cada cop que canvien els servidors).
func SetReadOnlyMCPTools(names []string) {
	set := make(map[string]bool, len(names))
	for _, n := range names {
		set[n] = true
	}
	readOnlyMu.Lock()
	readOnlyMCP = set
	readOnlyMu.Unlock()
}

// IsReadOnlyMCP diu si una eina MCP ve d'un servidor de només lectura.
func IsReadOnlyMCP(name string) bool {
	readOnlyMu.RLock()
	defer readOnlyMu.RUnlock()
	return readOnlyMCP[name]
}
