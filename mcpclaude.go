package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"gregal/internal/mcp"
)

// runMCPClaude implementa `gregal mcp claude`: un servidor MCP per stdio que
// deixa Gregal fer servir els models de Claude a través del CLI `claude`
// (la sessió de subscripció que l'usuari ja té iniciada). No és un proveïdor
// del motor: és una eina que l'agent crida per als casos concrets en què es
// vol Claude (una segona opinió, una revisió, una tasca difícil). Es registra
// com qualsevol altre servidor MCP: command = gregal, args = [mcp, claude].
func runMCPClaude(args []string) {
	fs := flag.NewFlagSet("mcp claude", flag.ContinueOnError)
	bin := fs.String("bin", "", "claude executable (default: $GREGAL_CLAUDE_BIN or claude on PATH)")
	workspace := fs.String("workspace", "", "default working directory for claude (default: current directory)")
	model := fs.String("model", "", "default model alias or id (default: the CLI's own default)")
	timeout := fs.Duration("timeout", 15*time.Minute, "maximum duration of one claude call")
	if err := fs.Parse(args); err != nil {
		os.Exit(2)
	}
	ws := *workspace
	if ws == "" {
		ws, _ = os.Getwd()
	}
	if abs, err := filepath.Abs(ws); err == nil {
		ws = abs
	}
	proto := os.Stdout
	os.Stdout = os.Stderr
	logf := func(format string, a ...any) { fmt.Fprintf(os.Stderr, "gregal mcp claude: "+format+"\n", a...) }
	c := &claudeCLI{bin: resolveClaudeBin(*bin), defaultWS: ws, defaultModel: *model, timeout: *timeout, log: logf}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	logf("serving on stdio (claude: %s, workspace %s)", c.bin, ws)
	if err := mcp.Serve(ctx, os.Stdin, proto, mcp.ServeOptions{Name: "gregal-claude", Version: version, Tools: c.tools(), Log: logf}); err != nil && err != io.EOF && ctx.Err() == nil {
		logf("%v", err)
		os.Exit(1)
	}
}

func resolveClaudeBin(flagVal string) string {
	if flagVal != "" {
		return flagVal
	}
	if v := strings.TrimSpace(os.Getenv("GREGAL_CLAUDE_BIN")); v != "" {
		return v
	}
	return "claude"
}

// claudeCLI executa `claude -p` com a subprocés.
type claudeCLI struct {
	bin          string
	defaultWS    string
	defaultModel string
	timeout      time.Duration
	log          func(string, ...any)
}

func (c *claudeCLI) tools() []mcp.ServerTool {
	str := func(desc string) map[string]any { return map[string]any{"type": "string", "description": desc} }
	return []mcp.ServerTool{{
		Name: "claude_ask",
		Description: "Ask a Claude model (through the user's own Claude Code subscription login) and return its answer. " +
			"Use it for a second opinion, a review or a hard sub-task. By default Claude gets no tools and sees only the prompt; " +
			"access=read lets it read files of the workspace, access=edit lets it also edit and write them.",
		InputSchema: map[string]any{"type": "object", "properties": map[string]any{
			"prompt":         str("What to ask Claude. Include all the context it needs."),
			"system":         str("Optional system prompt (replaces Claude Code's default one)."),
			"model":          str("Model alias (sonnet, opus, haiku) or full id. Default: the server's default."),
			"effort":         map[string]any{"type": "string", "enum": []string{"low", "medium", "high"}, "description": "Reasoning effort."},
			"access":         map[string]any{"type": "string", "enum": []string{"none", "read", "edit"}, "description": "none (default): no tools; read: Read/Grep/Glob in the workspace; edit: read + Edit/Write (no shell)."},
			"workspace":      str("Absolute directory Claude works in (default: where the server was started). Only matters with access read/edit."),
			"session":        str("Session id from a previous call, to continue that conversation."),
			"keep_session":   map[string]any{"type": "boolean", "description": "Save the conversation so its session id can be resumed later (default false: nothing is stored)."},
			"max_budget_usd": map[string]any{"type": "number", "description": "Spending cap for this call, in USD (API-billed accounts only)."},
		}, "required": []string{"prompt"}},
		Handler: c.handleAsk,
	}}
}

type claudeAnswer struct {
	Answer     string  `json:"answer"`
	Session    string  `json:"session,omitempty"`
	Model      string  `json:"model,omitempty"`
	Turns      int     `json:"turns"`
	DurationMS int64   `json:"duration_ms"`
	CostUSD    float64 `json:"cost_usd,omitempty"`
}

// claudeArgs construeix la línia d'ordres. Pura: es prova sense executar res.
func (c *claudeCLI) claudeArgs(args map[string]any) ([]string, error) {
	access := mcp.ArgString(args, "access")
	if access == "" {
		access = "none"
	}
	out := []string{"-p", "--output-format", "json", "--strict-mcp-config", "--disable-slash-commands"}
	switch access {
	case "none":
		// Sense eines ni configuració de l'usuari: una consulta pura.
		out = append(out, "--tools", "", "--setting-sources", "")
	case "read":
		out = append(out, "--tools", "Read,Grep,Glob", "--allowedTools", "Read,Grep,Glob")
	case "edit":
		out = append(out, "--tools", "Read,Grep,Glob,Edit,Write", "--allowedTools", "Read,Grep,Glob,Edit,Write", "--permission-mode", "acceptEdits")
	default:
		return nil, fmt.Errorf("access must be none, read or edit")
	}
	model := mcp.ArgString(args, "model")
	if model == "" {
		model = c.defaultModel
	}
	if model != "" {
		out = append(out, "--model", model)
	}
	if e := mcp.ArgString(args, "effort"); e != "" {
		switch e {
		case "low", "medium", "high":
			out = append(out, "--effort", e)
		default:
			return nil, fmt.Errorf("effort must be low, medium or high")
		}
	}
	if s := mcp.ArgString(args, "system"); s != "" {
		out = append(out, "--system-prompt", s)
	}
	if v, ok := args["max_budget_usd"].(float64); ok && v > 0 {
		out = append(out, "--max-budget-usd", fmt.Sprintf("%g", v))
	}
	if sess := mcp.ArgString(args, "session"); sess != "" {
		out = append(out, "--resume", sess)
	} else if !mcp.ArgBool(args, "keep_session") {
		out = append(out, "--no-session-persistence")
	}
	return out, nil
}

func (c *claudeCLI) handleAsk(ctx context.Context, args map[string]any) (string, error) {
	prompt := mcp.ArgString(args, "prompt")
	if prompt == "" {
		return "", fmt.Errorf("prompt is required")
	}
	cliArgs, err := c.claudeArgs(args)
	if err != nil {
		return "", err
	}
	dir := mcp.ArgString(args, "workspace")
	if dir == "" {
		dir = c.defaultWS
	}
	if st, err := os.Stat(dir); err != nil || !st.IsDir() {
		return "", fmt.Errorf("workspace %q is not a directory", dir)
	}
	if c.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.timeout)
		defer cancel()
	}
	c.log("claude_ask (%s): %s", mcp.ArgString(args, "access"), retallaMCP(prompt, 100))
	cmd := exec.CommandContext(ctx, c.bin, cliArgs...)
	cmd.Dir = dir
	cmd.Stdin = strings.NewReader(prompt)
	// El CLI es nega a córrer dins d'un altre Claude Code: aquesta crida és
	// un procés no interactiu i sense sessió desada, així que no hi comparteix res.
	cmd.Env = withoutEnv(os.Environ(), "CLAUDECODE")
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	runErr := cmd.Run()
	res, perr := parseClaudeResult(stdout.Bytes())
	if perr != nil {
		if runErr != nil {
			if ctx.Err() != nil {
				return "", fmt.Errorf("claude timed out or was cancelled: %w", ctx.Err())
			}
			return "", fmt.Errorf("claude failed: %v: %s", runErr, retallaMCP(strings.TrimSpace(stderr.String()), 600))
		}
		return "", perr
	}
	if res.IsError {
		return "", fmt.Errorf("claude: %s", retallaMCP(res.Result, 800))
	}
	raw, _ := json.MarshalIndent(claudeAnswer{
		Answer: res.Result, Session: res.SessionID, Model: firstModel(res.ModelUsage),
		Turns: res.NumTurns, DurationMS: res.DurationMS, CostUSD: res.TotalCostUSD,
	}, "", "  ")
	return string(raw), nil
}

type claudeResult struct {
	IsError      bool                       `json:"is_error"`
	Result       string                     `json:"result"`
	SessionID    string                     `json:"session_id"`
	NumTurns     int                        `json:"num_turns"`
	DurationMS   int64                      `json:"duration_ms"`
	TotalCostUSD float64                    `json:"total_cost_usd"`
	ModelUsage   map[string]json.RawMessage `json:"modelUsage"`
}

func parseClaudeResult(b []byte) (claudeResult, error) {
	var r claudeResult
	b = bytes.TrimSpace(b)
	if len(b) == 0 {
		return r, fmt.Errorf("claude returned no output")
	}
	if err := json.Unmarshal(b, &r); err != nil {
		return r, fmt.Errorf("unexpected claude output: %s", retallaMCP(string(b), 300))
	}
	return r, nil
}

func firstModel(m map[string]json.RawMessage) string {
	for k := range m {
		return k
	}
	return ""
}

func withoutEnv(env []string, name string) []string {
	out := make([]string, 0, len(env))
	for _, e := range env {
		if strings.HasPrefix(strings.ToUpper(e), strings.ToUpper(name)+"=") {
			continue
		}
		out = append(out, e)
	}
	return out
}
