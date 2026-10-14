package tui

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"gregal/internal/tools"
)

// /worktree: la sessió del TUI passa a treballar en un git worktree aïllat
// (<repo>/.gregal/worktrees/<nom>, branca gregal/<nom>). El repo principal
// no es toca fins que es porta la feina amb /worktree apply (pegat o merge).
func (m *Model) worktreeCommand(arg string) (tea.Model, tea.Cmd) {
	fields := strings.Fields(arg)
	sub := "list"
	if len(fields) > 0 {
		sub = strings.ToLower(fields[0])
	}
	name := ""
	if len(fields) > 1 {
		name = fields[1]
	}
	switch sub {
	case "list", "ls":
		list, err := tools.WorktreeList(m.cwd)
		if err != nil {
			m.push(systemLine("⚠ " + err.Error()))
			return m, nil
		}
		if len(list) == 0 {
			m.push(systemLine(T("wt.cap")))
			return m, nil
		}
		var b strings.Builder
		fmt.Fprintf(&b, "worktrees (%d):\n", len(list))
		for _, wt := range list {
			marca := "  "
			if samePathTUI(wt.Path, m.cwd) {
				marca = "▸ "
			}
			estat := ""
			if wt.Dirty {
				estat = " · *"
			}
			fmt.Fprintf(&b, "%s%s · %s · +%d%s\n", marca, wt.Name, wt.Branch, wt.Ahead, estat)
		}
		m.push(systemLine(strings.TrimRight(b.String(), "\n")))
		return m, nil
	case "new", "add", "create":
		if m.agentActive || m.busy {
			m.push(systemLine(T("wt.noEnMarxa")))
			return m, nil
		}
		if name == "" {
			m.push(systemLine(T("wt.us")))
			return m, nil
		}
		wt, err := tools.WorktreeCreate(m.cwd, name)
		if err != nil {
			m.push(systemLine("⚠ " + err.Error()))
			return m, nil
		}
		m.switchCwd(wt.Path)
		m.push(systemLine(fmt.Sprintf(T("wt.dins"), wt.Name, wt.Path, wt.Branch)))
		return m, nil
	case "exit", "leave", "main":
		if m.agentActive || m.busy {
			m.push(systemLine(T("wt.noEnMarxa")))
			return m, nil
		}
		main, err := tools.MainRepoRoot(m.cwd)
		if err != nil {
			m.push(systemLine("⚠ " + err.Error()))
			return m, nil
		}
		m.switchCwd(main)
		m.push(systemLine(fmt.Sprintf(T("wt.fora"), main)))
		return m, nil
	case "diff":
		if name == "" {
			name = tools.WorktreeNameOf(m.cwd)
		}
		patch, _, err := tools.WorktreeDiff(m.cwd, name)
		if err != nil {
			m.push(systemLine("⚠ " + err.Error()))
			return m, nil
		}
		if strings.TrimSpace(patch) == "" {
			m.push(systemLine(fmt.Sprintf(T("wt.senseCanvis"), name)))
			return m, nil
		}
		m.push(indentTool(capText(patch, 4000)))
		return m, nil
	case "apply", "merge":
		if name == "" {
			name = tools.WorktreeNameOf(m.cwd)
		}
		how := "patch"
		if sub == "merge" {
			how = "merge"
		}
		if len(fields) > 2 {
			how = strings.ToLower(fields[2])
		}
		if how != "patch" && how != "merge" {
			m.push(systemLine(T("wt.us")))
			return m, nil
		}
		// Toca el repo principal: es confirma com un rewind.
		nm, dir := name, m.cwd
		m.confirmIdx = 0
		m.pending = &pendingOp{
			desc: "worktree " + nm + " → " + how,
			run: func(ctx context.Context) (string, error) {
				return tools.WorktreeApply(dir, nm, how)
			},
		}
		m.push(systemLine("worktree " + nm + " → " + how + "?"))
		return m, nil
	case "remove", "rm":
		if name == "" {
			m.push(systemLine(T("wt.us")))
			return m, nil
		}
		if wtName := tools.WorktreeNameOf(m.cwd); wtName == name {
			m.push(systemLine(T("wt.noEnMarxa") + " (/worktree exit)"))
			return m, nil
		}
		delBranch := len(fields) > 2 && (fields[2] == "--branch" || fields[2] == "-b")
		out, err := tools.WorktreeRemove(m.cwd, name, delBranch)
		if err != nil {
			m.push(systemLine("⚠ " + err.Error()))
			return m, nil
		}
		m.push(systemLine(out))
		return m, nil
	default:
		m.push(systemLine(T("wt.us")))
		return m, nil
	}
}

// switchCwd canvia la carpeta de treball de la sessió (com en reprendre
// una sessió d'un altre directori): eines, permisos i barra la segueixen.
func (m *Model) switchCwd(dir string) {
	m.cwd = dir
	if m.policy != nil {
		m.policy.ProjectDir = dir
	}
	m.refreshGitBranch()
}

func samePathTUI(a, b string) bool {
	return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
}
