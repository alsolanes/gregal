package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"gregal/internal/mcp"
)

// /mcp al TUI: estat dels servidors i, per als remots, connectar amb OAuth.
// El TUI no té servidor web: el callback el rep un servidor efímer al
// loopback i l'URL d'autorització es pinta perquè l'usuari l'obri.

// mcpAuthMsg arriba quan l'autorització d'un servidor acaba (o falla).
type mcpAuthMsg struct {
	server string
	url    string
	err    error
	done   bool
	wait   *mcp.Loopback
}

// mcpPromptMsg porta un prompt MCP ja expandit (/mcp__servidor__prompt).
type mcpPromptMsg struct {
	text string
	err  error
}

func (m Model) mcpCommand(args []string) (tea.Model, tea.Cmd) {
	if len(args) >= 2 {
		sub, name := args[0], args[1]
		if m.mcp == nil {
			m.push(systemLine(T("app.mcpInactiu")))
			return m, nil
		}
		switch sub {
		case "connect", "reconnect":
			m.push(systemLine("mcp: connecting " + name + "…"))
			return m, mcpConnectCmd(m.mcp, name)
		case "disconnect", "logout":
			_ = m.mcp.Disconnect(name)
			m.push(systemLine("mcp: " + name + " disconnected (token forgotten)"))
			return m, nil
		}
	}
	if m.mcp == nil || m.mcp.Summary() == "inactiu" {
		m.push(systemLine(T("app.mcpInactiu")))
		return m, nil
	}
	m.push(systemLine("mcp: " + m.mcp.Summary()))
	for _, row := range m.mcp.Status() {
		line := fmt.Sprintf("  %v [%v] %v", row["name"], row["transport"], row["status"])
		if a, ok := row["auth"]; ok {
			line += fmt.Sprintf(" · auth %v", a)
		}
		if row["status"] == mcp.StatusNeedsAuth {
			line += fmt.Sprintf(" → /mcp connect %v", row["name"])
		}
		m.push(dimStyle.Render(line))
	}
	for _, t := range m.mcp.ToolsSnapshot() {
		m.push(dimStyle.Render("  " + t.Spec.Name + " — " + shortArgs(t.Spec.Description, 100)))
	}
	for _, p := range m.mcp.Prompts() {
		m.push(dimStyle.Render("  /" + p.Command() + " — " + shortArgs(p.Description, 100)))
	}
	for srv, msg := range m.mcp.ErrorsSnapshot() {
		m.push(badStyle.Render("  ✗ " + srv + ": " + msg))
	}
	return m, nil
}

// mcpConnectCmd connecta un servidor. Si demana OAuth, obre el loopback,
// torna primer l'URL (es pinta) i després espera el callback.
func mcpConnectCmd(mgr *mcp.Manager, name string) tea.Cmd {
	return func() tea.Msg {
		lb, err := mgr.StartLoopback()
		if err != nil {
			return mcpAuthMsg{server: name, err: err, done: true}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		u, err := mgr.Connect(ctx, name, lb.RedirectURI)
		cancel()
		if err != nil || u == "" {
			lb.Close()
			return mcpAuthMsg{server: name, err: err, done: true}
		}
		return mcpAuthMsg{server: name, url: u, done: false, err: nil, wait: lb}
	}
}

// mcpWaitCmd espera el callback del navegador (com a molt 10 minuts).
func mcpWaitCmd(name string, lb *mcp.Loopback) tea.Cmd {
	return func() tea.Msg {
		defer lb.Close()
		select {
		case r := <-lb.Done:
			if r.Server == "" {
				r.Server = name
			}
			return mcpAuthMsg{server: r.Server, err: r.Err, done: true}
		case <-time.After(10 * time.Minute):
			return mcpAuthMsg{server: name, err: fmt.Errorf("authorization timed out"), done: true}
		}
	}
}

// onMCPAuth pinta l'avenç de l'autorització.
func (m Model) onMCPAuth(msg mcpAuthMsg) (tea.Model, tea.Cmd) {
	if !msg.done {
		m.push(systemLine("mcp: open this URL to authorize " + msg.server + ":"))
		m.push(msg.url)
		return m, mcpWaitCmd(msg.server, msg.wait)
	}
	if msg.err != nil {
		m.push(badStyle.Render("✗ mcp " + msg.server + ": " + msg.err.Error()))
		return m, nil
	}
	m.push(systemLine("mcp: " + msg.server + " connected · " + m.mcp.Summary()))
	return m, nil
}

// expandMCPPromptCmd resol un /mcp__servidor__prompt en segon pla.
func expandMCPPromptCmd(mgr *mcp.Manager, text string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		out, _, err := mgr.ExpandPromptCommand(ctx, text)
		return mcpPromptMsg{text: out, err: err}
	}
}

// isMCPPrompt diu si el text és una ordre de prompt MCP coneguda.
func (m Model) isMCPPrompt(text string) bool {
	if m.mcp == nil || !strings.HasPrefix(text, "/mcp__") {
		return false
	}
	cmd, _, _ := strings.Cut(strings.TrimSpace(text), " ")
	_, ok := m.mcp.FindPrompt(cmd)
	return ok
}

func (m Model) onMCPPrompt(msg mcpPromptMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.push(badStyle.Render("✗ " + msg.err.Error()))
		return m, nil
	}
	if strings.TrimSpace(msg.text) == "" {
		m.push(warnStyle.Render("mcp: empty prompt"))
		return m, nil
	}
	return m.submitText(msg.text)
}
