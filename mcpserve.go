package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"gregal/internal/agent"
	"gregal/internal/brand"
	"gregal/internal/config"
	"gregal/internal/demodb"
	"gregal/internal/llm"
	"gregal/internal/mcp"
	"gregal/internal/session"
)

// runMCP implementa `gregal mcp serve`: Gregal com a servidor MCP per
// stdio, perquè un altre client (Claude Code, Cursor, un altre agent) li
// pugui encarregar feina. stdout és el canal del protocol: tota la resta
// (passos, avisos) va a stderr.
func runMCP(args []string) {
	if len(args) > 0 && args[0] == "demo" {
		runMCPDemo()
		return
	}
	if len(args) > 0 && args[0] == "import" {
		runMCPImport(args[1:])
		return
	}
	if len(args) == 0 || args[0] != "serve" {
		fmt.Fprintln(os.Stderr, "usage: gregal mcp serve [--config path] [--workspace dir] | gregal mcp demo | gregal mcp import [--from claude,codex,opencode] [--only names] [--yes] [--dry-run]")
		os.Exit(2)
	}
	fs := flag.NewFlagSet("mcp serve", flag.ContinueOnError)
	cfgPath := fs.String("config", "", "configuration path (default ~/.config/gregal/config.yaml)")
	workspace := fs.String("workspace", "", "default workspace for gregal_run (default: current directory)")
	if err := fs.Parse(args[1:]); err != nil {
		os.Exit(2)
	}
	cfg, path, err := config.Load(*cfgPath)
	if err != nil {
		fatalCfg(err, configLanguage(path))
	}
	brand.Setup(cfg.Brand)
	ws := *workspace
	if ws == "" {
		ws, _ = os.Getwd()
	}
	if abs, err := filepath.Abs(ws); err == nil {
		ws = abs
	}
	// El protocol es queda el stdout de debò; qualsevol Print perdut del
	// codi de l'agent anirà a stderr i no corromprà el JSON-RPC.
	proto := os.Stdout
	os.Stdout = os.Stderr

	mcp.Setup(toMCPServers(cfg))
	agent.SetPostEditHook(cfg.Hooks.PostEdit)
	agent.SetDiagMode(cfg.Hooks.Diag)
	agent.SetLifecycleHooks(cfg.Hooks)
	agent.SetupPrices(cfg)
	cli := llm.New()
	cfg.ConfigureClient(cli)
	agent.SetupDelegate(cfg, cli)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	logf := func(format string, a ...any) { fmt.Fprintf(os.Stderr, "gregal mcp: "+format+"\n", a...) }
	srv := &gregalMCP{cfg: cfg, defaultWS: ws, sessionsDir: session.DefaultDir(), log: logf,
		run: func(ctx context.Context, o agent.RunOpts, verify string) (agent.RunResult, error) {
			return agent.RunAmbVerificacio(ctx, cli, cfg, o, verify, 2)
		}}
	logf("serving on stdio (workspace %s)", ws)
	if err := mcp.Serve(ctx, os.Stdin, proto, mcp.ServeOptions{Name: "gregal", Version: version, Tools: srv.tools(), Log: logf}); err != nil && err != io.EOF && ctx.Err() == nil {
		logf("%v", err)
		os.Exit(1)
	}
}

// runMCPDemo implementa `gregal mcp demo`: la base de dades de
// demostració (internal/demodb) com a servidor MCP de només lectura per
// stdio. No llegeix cap config ni toca el disc.
func runMCPDemo() {
	proto := os.Stdout
	os.Stdout = os.Stderr
	t0 := time.Now()
	db := demodb.Generate()
	logf := func(format string, a ...any) { fmt.Fprintf(os.Stderr, "gregal mcp demo: "+format+"\n", a...) }
	logf("dataset ready in %s; serving on stdio", time.Since(t0).Round(time.Millisecond))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := mcp.Serve(ctx, os.Stdin, proto, demodb.ServeOptions(db, version, logf)); err != nil && err != io.EOF && ctx.Err() == nil {
		logf("%v", err)
		os.Exit(1)
	}
}

// gregalMCP són les eines que Gregal exposa com a servidor MCP.
type gregalMCP struct {
	cfg         *config.Config
	defaultWS   string
	sessionsDir string
	log         func(string, ...any)
	run         func(ctx context.Context, o agent.RunOpts, verify string) (agent.RunResult, error)
	// runMu: un torn alhora (l'agent comparteix estat de procés: checklist,
	// checkpoints, preus).
	runMu sync.Mutex
}

func (g *gregalMCP) tools() []mcp.ServerTool {
	str := func(desc string) map[string]any { return map[string]any{"type": "string", "description": desc} }
	return []mcp.ServerTool{
		{
			Name: "gregal_run",
			Description: "Run a Gregal coding-agent turn headlessly in a workspace and return the final answer and the files it changed. " +
				"Gregal reads, edits and runs commands in the workspace like a developer. Tools that need approval are rejected unless auto_approve is true.",
			InputSchema: map[string]any{"type": "object", "properties": map[string]any{
				"task":         str("What Gregal should do (natural language)."),
				"mode":         map[string]any{"type": "string", "enum": []string{"code", "inspect", "chat", "goal", "autonomous", "debug"}, "description": "Agent mode (default: the configuration's mode)."},
				"workspace":    str("Absolute path of the project directory (default: where gregal mcp serve was started)."),
				"auto_approve": map[string]any{"type": "boolean", "description": "Approve ask-policy tools (writes outside the project, risky commands) without a human. Not allowed with mode autonomous."},
				"max_steps":    map[string]any{"type": "integer", "minimum": 1, "maximum": 200, "description": "Hard step limit (default: configuration)."},
				"verify":       str("Optional verification command (e.g. 'go test ./...'); Gregal retries until it passes (2 rounds)."),
			}, "required": []string{"task"}},
			Handler: g.handleRun,
		},
		{
			Name:        "gregal_sessions_list",
			Description: "List saved Gregal conversations (most recent first).",
			InputSchema: map[string]any{"type": "object", "properties": map[string]any{
				"limit":     map[string]any{"type": "integer", "minimum": 1, "maximum": 200, "description": "Maximum sessions (default 20)."},
				"workspace": str("Only sessions of this workspace."),
			}},
			Handler: g.handleSessionsList,
		},
		{
			Name:        "gregal_session_get",
			Description: "Get a saved Gregal conversation: metadata and its messages (long contents truncated).",
			InputSchema: map[string]any{"type": "object", "properties": map[string]any{
				"name":         str("Session name from gregal_sessions_list."),
				"max_messages": map[string]any{"type": "integer", "minimum": 1, "maximum": 500, "description": "Last N messages (default 50)."},
			}, "required": []string{"name"}},
			Handler: g.handleSessionGet,
		},
	}
}

type mcpRunResult struct {
	Answer       string   `json:"answer"`
	ChangedFiles []string `json:"changed_files"`
	Workspace    string   `json:"workspace"`
	Mode         string   `json:"mode"`
	Steps        int      `json:"steps"`
	ToolsUsed    []string `json:"tools_used"`
	Exhausted    bool     `json:"exhausted,omitempty"`
	Verified     *bool    `json:"verified,omitempty"`
	CostUSD      *float64 `json:"cost_usd,omitempty"`
	Session      string   `json:"session,omitempty"`
	Error        string   `json:"error,omitempty"`
}

func (g *gregalMCP) handleRun(ctx context.Context, args map[string]any) (string, error) {
	task := mcp.ArgString(args, "task")
	if task == "" {
		return "", fmt.Errorf("task is required")
	}
	mode := mcp.ArgString(args, "mode")
	if mode == "" {
		mode = g.cfg.Mode
	}
	ws := mcp.ArgString(args, "workspace")
	if ws == "" {
		ws = g.defaultWS
	}
	abs, err := filepath.Abs(ws)
	if err != nil {
		return "", err
	}
	if st, err := os.Stat(abs); err != nil || !st.IsDir() {
		return "", fmt.Errorf("workspace %q is not a directory", ws)
	}
	autoApprove := mcp.ArgBool(args, "auto_approve")
	maxSteps := mcp.ArgInt(args, "max_steps")
	if err := validaHeadless(g.cfg, "json", mode, autoApprove, maxSteps, cliLanguage(g.cfg)); err != nil {
		return "", err
	}
	hard := maxSteps > 0
	if maxSteps == 0 {
		maxSteps = g.cfg.Agent.MaxSteps
	}
	g.runMu.Lock()
	defer g.runMu.Unlock()
	before, isGit := gitSnapshot(abs)
	g.log("gregal_run in %s (%s): %s", abs, mode, retallaMCP(task, 120))
	res, runErr := g.run(ctx, agent.RunOpts{
		Task: task, Mode: mode, MaxSteps: maxSteps, HardStepLimit: hard, AutoApprove: autoApprove, Dir: abs,
		OnStep: func(step int, tools []string) {
			if len(tools) > 0 {
				g.log("step %d: %s", step, strings.Join(tools, ", "))
			}
		},
	}, mcp.ArgString(args, "verify"))
	out := mcpRunResult{
		Answer: res.Answer, Workspace: abs, Mode: mode, Steps: res.Steps, ToolsUsed: res.Tools,
		Exhausted: res.Exhausted, Verified: res.Verified, CostUSD: res.CostUSD,
	}
	if out.ToolsUsed == nil {
		out.ToolsUsed = []string{}
	}
	out.ChangedFiles = changedFiles(abs, before, isGit, res.Trace)
	if runErr != nil {
		out.Error = runErr.Error()
	}
	if strings.TrimSpace(res.Answer) != "" {
		if name, err := g.saveRun(abs, task, res.Answer); err == nil {
			out.Session = name
		}
	}
	raw, _ := json.MarshalIndent(out, "", "  ")
	if runErr != nil {
		return string(raw), runErr
	}
	return string(raw), nil
}

// saveRun desa la tasca i la resposta com una conversa (gregal_session_get
// la torna a trobar; la web i el TUI també la poden reprendre).
func (g *gregalMCP) saveRun(ws, task, answer string) (string, error) {
	name := "mcp-" + time.Now().Format("20060102-150405")
	path, err := session.SaveSession(g.sessionsDir, session.Session{
		Name: name, Role: "code", Workspace: ws,
		Convo: []llm.Message{{Role: "user", Content: task}, {Role: "assistant", Content: answer}},
	})
	if err != nil {
		return "", err
	}
	return strings.TrimSuffix(filepath.Base(path), ".json"), nil
}

func (g *gregalMCP) handleSessionsList(_ context.Context, args map[string]any) (string, error) {
	limit := mcp.ArgInt(args, "limit")
	if limit <= 0 {
		limit = 20
	}
	wsFilter := mcp.ArgString(args, "workspace")
	infos, err := session.List(g.sessionsDir)
	if err != nil {
		return "", err
	}
	type row struct {
		Name      string    `json:"name"`
		Title     string    `json:"title"`
		SavedAt   time.Time `json:"saved_at"`
		Messages  int       `json:"messages"`
		Workspace string    `json:"workspace,omitempty"`
		Pinned    bool      `json:"pinned,omitempty"`
	}
	out := []row{}
	for _, in := range infos {
		if wsFilter != "" && !mateixCami(in.Workspace, wsFilter) {
			continue
		}
		out = append(out, row{Name: in.Name, Title: in.Title, SavedAt: in.SavedAt, Messages: in.Msgs, Workspace: in.Workspace, Pinned: in.Pinned})
		if len(out) >= limit {
			break
		}
	}
	raw, _ := json.MarshalIndent(map[string]any{"sessions": out}, "", "  ")
	return string(raw), nil
}

func (g *gregalMCP) handleSessionGet(_ context.Context, args map[string]any) (string, error) {
	name := mcp.ArgString(args, "name")
	if name == "" {
		return "", fmt.Errorf("name is required")
	}
	s, err := session.Load(g.sessionsDir, name)
	if err != nil {
		return "", err
	}
	max := mcp.ArgInt(args, "max_messages")
	if max <= 0 {
		max = 50
	}
	msgs := s.Convo
	if len(msgs) > max {
		msgs = msgs[len(msgs)-max:]
	}
	type msg struct {
		Role    string   `json:"role"`
		Content string   `json:"content,omitempty"`
		Tools   []string `json:"tool_calls,omitempty"`
	}
	list := make([]msg, 0, len(msgs))
	for _, m := range msgs {
		limit := 4000
		if m.Role == "tool" {
			limit = 600
		}
		one := msg{Role: m.Role, Content: retallaMCP(m.Content, limit)}
		for _, c := range m.ToolCalls {
			one.Tools = append(one.Tools, c.Function.Name)
		}
		list = append(list, one)
	}
	raw, _ := json.MarshalIndent(map[string]any{
		"name": s.Name, "title": s.Title, "workspace": s.Workspace, "saved_at": s.SavedAt,
		"total_messages": len(s.Convo), "messages": list,
	}, "", "  ")
	return string(raw), nil
}

func mateixCami(a, b string) bool {
	ca, cb := filepath.Clean(a), filepath.Clean(b)
	if filepath.Separator == '\\' {
		return strings.EqualFold(ca, cb)
	}
	return ca == cb
}

func retallaMCP(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "… (truncated)"
}

// gitSnapshot desa l'estat dels fitxers bruts d'un repo git (camí → hash
// del contingut) per saber després què ha canviat el torn. ok=false si el
// directori no és un repo.
func gitSnapshot(dir string) (map[string]string, bool) {
	out, err := exec.Command("git", "-C", dir, "status", "--porcelain", "-z", "--untracked-files=all").Output()
	if err != nil {
		return nil, false
	}
	snap := map[string]string{}
	entries := strings.Split(string(out), "\x00")
	for i := 0; i < len(entries); i++ {
		e := entries[i]
		if len(e) < 4 {
			continue
		}
		xy, p := e[:2], e[3:]
		if xy[0] == 'R' || xy[0] == 'C' {
			i++ // el camí original ve a la següent entrada
		}
		snap[p] = hashFile(filepath.Join(dir, filepath.FromSlash(p)))
	}
	return snap, true
}

func hashFile(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return "missing"
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "unreadable"
	}
	return hex.EncodeToString(h.Sum(nil))
}

var pathArg = regexp.MustCompile(`"(?:path|file_path)"\s*:\s*"((?:[^"\\]|\\.)*)"`)

// changedFiles compara l'estat de git d'abans i d'ara; fora de git, en
// treu els camins de les eines d'escriptura del trace.
func changedFiles(dir string, before map[string]string, isGit bool, trace []agent.TraceCall) []string {
	set := map[string]bool{}
	if isGit {
		after, ok := gitSnapshot(dir)
		if ok {
			for p, h := range after {
				if before[p] != h {
					set[p] = true
				}
			}
			for p := range before {
				if _, still := after[p]; !still {
					set[p] = true
				}
			}
		}
	} else {
		for _, tc := range trace {
			switch tc.Tool {
			case "write", "edit", "patch", "office_edit":
			default:
				continue
			}
			if tc.Failed {
				continue
			}
			if m := pathArg.FindStringSubmatch(tc.Args); m != nil {
				var p string
				if json.Unmarshal([]byte(`"`+m[1]+`"`), &p) == nil && p != "" {
					if rel, err := filepath.Rel(dir, p); err == nil && !strings.HasPrefix(rel, "..") && filepath.IsAbs(p) {
						p = filepath.ToSlash(rel)
					}
					set[p] = true
				}
			}
		}
	}
	out := make([]string, 0, len(set))
	for p := range set {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}
