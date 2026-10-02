package mcp

import (
	"fmt"
	"sort"

	"gregal/internal/agent"
	"gregal/internal/llm"
)

// Manager arrenca N servidors i registra les seves tools a l'agent.
type Manager struct {
	clients map[string]*Client
	Tools   []Tool
	Errors  map[string]string // servidor → error d'arrencada
}

// Status returns the configured server names and their current connection
// state without exposing process handles or environment secrets.
func (m *Manager) Status() []map[string]any {
	if m == nil {
		return nil
	}
	names := make(map[string]string, len(m.clients)+len(m.Errors))
	for name := range m.clients {
		names[name] = "connected"
	}
	for name := range m.Errors {
		names[name] = "error"
	}
	keys := make([]string, 0, len(names))
	for name := range names {
		keys = append(keys, name)
	}
	sort.Strings(keys)
	out := make([]map[string]any, 0, len(keys))
	for _, name := range keys {
		row := map[string]any{"name": name, "status": names[name]}
		if err := m.Errors[name]; err != "" {
			row["error"] = err
		}
		out = append(out, row)
	}
	return out
}

// Setup arrenca els servidors del config i registra les tools a l'agent.
// No falla mai: els errors queden a Errors (consultables amb /mcp).
func Setup(servers map[string]Server) *Manager {
	m := &Manager{clients: map[string]*Client{}, Errors: map[string]string{}}
	for name, srv := range servers {
		c, err := Dial(name, srv)
		if err != nil {
			m.Errors[name] = err.Error()
			continue
		}
		m.clients[name] = c
		tools, err := c.Tools()
		if err != nil {
			m.Errors[name] = err.Error()
			c.Close()
			delete(m.clients, name)
			continue
		}
		m.Tools = append(m.Tools, tools...)
	}
	for _, t := range m.Tools {
		t := t
		agent.RegisterExtra(t.Spec, func(args string) (string, error) { return t.Call(args) })
	}
	return m
}

// Summary retorna "2 servidors, 7 eines" o "inactiu".
func (m *Manager) Summary() string {
	if m == nil || (len(m.clients) == 0 && len(m.Errors) == 0) {
		return "inactiu"
	}
	s := fmt.Sprintf("%d servidor(s), %d eina(es)", len(m.clients), len(m.Tools))
	if len(m.Errors) > 0 {
		s += fmt.Sprintf(" (%d error(s))", len(m.Errors))
	}
	return s
}

// Close atura tots els servidors.
func (m *Manager) Close() {
	if m == nil {
		return
	}
	for _, c := range m.clients {
		c.Close()
	}
}

// SpecsOf retorna les specs registrades (per tests).
func (m *Manager) SpecsOf() []llm.ToolSpec {
	var out []llm.ToolSpec
	for _, t := range m.Tools {
		out = append(out, t.Spec)
	}
	return out
}
