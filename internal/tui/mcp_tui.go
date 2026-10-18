package tui

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"gregal/internal/mcp"
	"gregal/internal/mcpimport"
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
	if len(args) > 0 && args[0] == "import" {
		return m.mcpImportCommand(args[1:])
	}
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

// mcpReadyMsg arriba quan el primer intent de connexió d'un servidor MCP
// (fet en segon pla des d'Init) acaba, bé o malament.
type mcpReadyMsg struct {
	server string
	err    error
}

// mcpSetupCmds connecta cada servidor en una ordre pròpia: el TUI pinta
// de seguida i les eines de cada servidor apareixen quan aquell acaba,
// sense esperar el més lent.
func mcpSetupCmds(mgr *mcp.Manager) []tea.Cmd {
	var cmds []tea.Cmd
	for _, name := range mgr.Connecting() {
		name := name
		cmds = append(cmds, func() tea.Msg {
			return mcpReadyMsg{server: name, err: mgr.ConnectServer(name)}
		})
	}
	return cmds
}

// onMCPReady avisa dels servidors que no han pogut connectar en arrencar
// (abans només ho deia /mcp); els connectats no fan soroll: la barra
// deixa de dir «connectant».
func (m Model) onMCPReady(msg mcpReadyMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil && m.mcp != nil {
		if rows := m.mcp.Status(); statusOf(rows, msg.server) == mcp.StatusNeedsAuth {
			m.push(dimStyle.Render("mcp: " + msg.server + " needs authorization → /mcp connect " + msg.server))
		} else {
			m.push(badStyle.Render("✗ mcp " + msg.server + ": " + msg.err.Error()))
		}
	}
	return m, nil
}

func statusOf(rows []map[string]any, name string) any {
	for _, r := range rows {
		if r["name"] == name {
			return r["status"]
		}
	}
	return nil
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

// mcpImportCommand és «/mcp import»: ensenya què s'importaria de Claude
// Code, Codex i opencode (sense secrets) i, amb «/mcp import yes [noms]»,
// ho desa al config i connecta els servidors afegits.
func (m Model) mcpImportCommand(args []string) (tea.Model, tea.Cmd) {
	home, _ := os.UserHomeDir()
	cwd, _ := os.Getwd()
	res := mcpimport.Discover(home, cwd)
	plan := mcpimport.MakePlan(m.cfg.MCP, res.Found)
	confirm := len(args) > 0 && strings.EqualFold(args[0], "yes")
	if !confirm {
		for _, l := range strings.Split(strings.TrimRight(mcpimport.Report(res, plan), "\n"), "\n") {
			m.push(dimStyle.Render(l))
		}
		if len(plan.Add) == 0 {
			m.push(systemLine("mcp import: nothing to import"))
		} else {
			m.push(systemLine("mcp import: /mcp import yes [names] adds them to the config (env values and headers are copied as they are)"))
		}
		return m, nil
	}
	if m.cfgPath == "" {
		m.push(badStyle.Render("mcp import: no config file to write to"))
		return m, nil
	}
	added := mcpimport.Apply(m.cfg, plan, args[1:])
	if len(added) == 0 {
		m.push(systemLine("mcp import: nothing added"))
		return m, nil
	}
	if err := m.cfg.Save(m.cfgPath); err != nil {
		for _, n := range added {
			delete(m.cfg.MCP, n)
		}
		m.push(badStyle.Render("mcp import: not saved: " + err.Error()))
		return m, nil
	}
	m.push(systemLine("mcp import: added " + strings.Join(added, ", ")))
	if m.mcp == nil {
		m.push(dimStyle.Render("  they connect the next time Gregal starts"))
		return m, nil
	}
	var cmds []tea.Cmd
	for _, name := range added {
		name, srv := name, mcp.ServerFromConfig(m.cfg.MCP[name])
		mgr := m.mcp
		cmds = append(cmds, func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
			defer cancel()
			return mcpReadyMsg{server: name, err: mgr.Add(ctx, name, srv)}
		})
	}
	return m, tea.Batch(cmds...)
}
