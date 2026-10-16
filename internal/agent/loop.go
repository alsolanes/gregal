// Package agent implementa l'agent loop: el model demana eines (format
// OpenAI, verificat contra llama.cpp local) i el loop les executa fins a
// resposta final o límit de passos.
package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"gregal/internal/knowledge"
	"gregal/internal/llm"
	"gregal/internal/tools"
)

// Specs retorna les eines natives exposades al model.
func Specs() []llm.ToolSpec {
	str := map[string]any{"type": "string"}
	return []llm.ToolSpec{
		{Name: "preview", Description: "Open or update the user's side preview while working. Use html for a complete standalone HTML prototype, markdown for a complex plan or document, or url for a running localhost development server. Call again with updated content after revisions. This displays content only: it does not inspect the rendered page or verify correctness. HTML runs in an isolated frame without network access. Do not include secrets.",
			Parameters: map[string]any{"type": "object", "properties": map[string]any{
				"kind": map[string]any{"type": "string", "enum": []string{"html", "markdown", "url"}}, "content": str, "title": str,
			}, "required": []string{"kind", "content"}}},
		{Name: "dashboard_save", Description: "Save charts to a named dashboard INSIDE the Gregal app (its Plots & dashboard section). Use this when the user asks for a dashboard or saved charts in the app: no HTML needed. Each plot uses the gregal-plot JSON shape {title, type: line|bar|scatter, labels: [strings], series: [{label, values: [numbers]}]}. Creates the dashboard if it does not exist; replace=true clears its previous charts first. Only available in the web/desktop app.",
			Parameters: map[string]any{"type": "object", "properties": map[string]any{
				"dashboard": str,
				"plots": map[string]any{"type": "array", "items": map[string]any{"type": "object", "properties": map[string]any{
					"title": str, "type": map[string]any{"type": "string", "enum": []string{"line", "bar", "scatter"}},
					"labels": map[string]any{"type": "array", "items": str},
					"series": map[string]any{"type": "array", "items": map[string]any{"type": "object", "properties": map[string]any{
						"label": str, "values": map[string]any{"type": "array", "items": map[string]any{"type": "number"}},
					}}},
				}, "required": []string{"title", "type", "labels", "series"}}},
				"replace": map[string]any{"type": "boolean"},
			}, "required": []string{"dashboard", "plots"}}},
		{Name: "read", Description: "Read a file from disk. Returns numbered lines. If you need several files, request them all in the same step: reads run in parallel.",
			Parameters: map[string]any{"type": "object", "properties": map[string]any{
				"path":   str,
				"offset": map[string]any{"type": "integer"},
				"limit":  map[string]any{"type": "integer"},
			}, "required": []string{"path"}}},
		{Name: "bash", Description: "Run a shell command with a 2-minute timeout and return its output. Building and running one package's tests fits. Do NOT use it for anything long by nature (starting a server, the whole test suite): it gets cut off halfway. Use bash_background for that.",
			Parameters: map[string]any{"type": "object", "properties": map[string]any{
				"command": str,
				"bet":     betParam,
			}, "required": []string{"command"}}},
		{Name: "write", Description: "Write a whole file with the given content: for NEW files. To change an existing file use edit or patch: rewriting it whole is slow (every line is written again) and you can lose code you have not read.",
			Parameters: map[string]any{"type": "object", "properties": map[string]any{
				"path": str, "content": str,
			}, "required": []string{"path", "content"}}},
		{Name: "edit", Description: "Replace ONE exact block (it must be unique) in a file. For several changes to the same file use patch: with separate edits, if one fails the file is left half-changed.",
			Parameters: map[string]any{"type": "object", "properties": map[string]any{
				"path": str, "old_string": str, "new_string": str,
			}, "required": []string{"path", "old_string", "new_string"}}},
		{Name: "grep", Description: "Search for a literal text in the files under a directory. Skips .git and binaries.",
			Parameters: map[string]any{"type": "object", "properties": map[string]any{
				"pattern": str, "dir": str, "include": str,
			}, "required": []string{"pattern"}}},
		{Name: "glob", Description: "List the files matching a pattern (e.g. *.go, dir/*.txt).",
			Parameters: map[string]any{"type": "object", "properties": map[string]any{
				"pattern": str, "dir": str,
			}, "required": []string{"pattern"}}},
		symbolsToolSpec(),
		{Name: "web_search", Description: "Search the web (several engines at once, no local service needed). Returns title+URL+snippet per result; then open the useful ones with web_fetch. Several searches or pages in the same step run in parallel.",
			Parameters: map[string]any{"type": "object", "properties": map[string]any{
				"query": str, "count": map[string]any{"type": "integer"},
			}, "required": []string{"query"}}},
		{Name: "web_fetch", Description: "Read an http(s) page and return its main content as markdown (no menus, ads or images), with title, date and URL. Options: find=text to see only the paragraphs that contain it (long pages), max_chars to enlarge the excerpt (up to 30000), raw=true for the whole page as plain text. Prefer one read with a sufficient max_chars over repeated narrow find calls on the same URL, and reuse what you already fetched. Local hosts are blocked.",
			Parameters: map[string]any{"type": "object", "properties": map[string]any{
				"url":       str,
				"find":      str,
				"max_chars": map[string]any{"type": "integer"},
				"raw":       map[string]any{"type": "boolean"},
			}, "required": []string{"url"}}},
		{Name: "patch", Description: "Apply several edits to one file at once (atomic: all or nothing). Each edit is old→new (unique block) or after_line+new (insertion).",
			Parameters: map[string]any{"type": "object", "properties": map[string]any{
				"path": str,
				"edits": map[string]any{"type": "array", "items": map[string]any{"type": "object", "properties": map[string]any{
					"old":        str,
					"new":        str,
					"after_line": map[string]any{"type": "integer"},
				}}},
			}, "required": []string{"path", "edits"}}},
		{Name: "read_image", Description: "Read an image from disk (png/jpg/webp/gif, max 8MB) and attach it to the message so the model can see it.",
			Parameters: map[string]any{"type": "object", "properties": map[string]any{
				"path": str,
			}, "required": []string{"path"}}},
		{Name: "gh_issue", Description: "Query GitHub issues with the gh CLI (read-only): action=list|view, optional number, optional repo (owner/name), optional query.",
			Parameters: map[string]any{"type": "object", "properties": map[string]any{
				"action": map[string]any{"type": "string", "enum": []string{"list", "view"}},
				"number": map[string]any{"type": "integer"},
				"query":  str, "repo": str,
			}, "required": []string{"action"}}},
		{Name: "gh_pr", Description: "Query GitHub pull requests with the gh CLI (read-only): action=list|view|diff|checks, number required except for list, optional repo.",
			Parameters: map[string]any{"type": "object", "properties": map[string]any{
				"action": map[string]any{"type": "string", "enum": []string{"list", "view", "diff", "checks"}},
				"number": map[string]any{"type": "integer"},
				"query":  str, "repo": str,
			}, "required": []string{"action"}}},
		{Name: "git_commit", Description: "Commit changes in the workspace's git repository (needs the user's approval). By default it commits only the files you changed in this session (shell changes included); pass paths to choose files, or all=true for every change in the workspace. Other files the user has staged are left out. Write a concise message: a summary line, a blank line, then details if useful.",
			Parameters: map[string]any{"type": "object", "properties": map[string]any{
				"message": str,
				"paths":   map[string]any{"type": "array", "items": str},
				"all":     map[string]any{"type": "boolean"},
			}, "required": []string{"message"}}},
		{Name: "gh_pr_create", Description: "Open a GitHub pull request with the gh CLI from the current branch (needs the user's approval). Commit first with git_commit. On the default branch it first creates a new branch (branch, or gregal/<title>) so the PR never comes from the default branch; then it pushes (never forced) and creates the PR. base defaults to the repository's default branch.",
			Parameters: map[string]any{"type": "object", "properties": map[string]any{
				"title": str, "body": str, "base": str, "branch": str,
				"draft": map[string]any{"type": "boolean"},
			}, "required": []string{"title"}}},
		{Name: "office_read", Description: "Read Word/Excel/PowerPoint files (.docx/.xlsx/.pptx and templates .dotx/.potx/.xltx). docx: paragraphs with styles, lists, tables cell by cell, tracked changes, comments, headers/footers. xlsx: values with formulas (=B2*2 → 20). pptx: text per slide and notes. detail=layout (pptx) lists the layouts and every shape with id, placeholder, position and overflow warnings: use it before filling a template. The old .doc/.xls/.ppt formats are NOT supported.",
			Parameters: map[string]any{"type": "object", "properties": map[string]any{
				"path":   str,
				"detail": map[string]any{"type": "string", "enum": []string{"text", "layout", "full"}},
			}, "required": []string{"path"}}},
		{Name: "office_render", Description: "Render an office document to images (LibreOffice or Microsoft Office) and run automatic QA (overflowing text, overlaps, shapes off the slide, leftover placeholder text, formula errors, broken structure). Returns a numbered thumbnail grid; pass pages=\"3\" or \"2,5\" to see pages/slides at full size. ALWAYS render after creating or editing a document and fix what you see before delivering it.",
			Parameters: map[string]any{"type": "object", "properties": map[string]any{
				"path": str, "pages": str,
			}, "required": []string{"path"}}},
		{Name: "office_open", Description: "Open a document with the system's associated program (Word/Excel/PowerPoint). It does not block: it returns immediately. While the document is open it cannot be written with office_edit (file locked): ask the user to close it to keep editing.",
			Parameters: map[string]any{"type": "object", "properties": map[string]any{
				"path": str,
			}, "required": []string{"path"}}},
		{Name: "bash_background", Description: "Start a long command in the background (server, build, test suite) and return an id. It does not wait: read its output with bash_output and stop it with bash_kill. Use it when a command may take longer than the 2 minutes of plain bash.",
			Parameters: map[string]any{"type": "object", "properties": map[string]any{
				"command": str,
			}, "required": []string{"command"}}},
		{Name: "bash_output", Description: "Read the new output of a background process (id from bash_background). Also says whether it has finished and with which exit code.",
			Parameters: map[string]any{"type": "object", "properties": map[string]any{
				"id": str, "lines": map[string]any{"type": "integer"},
			}, "required": []string{"id"}}},
		{Name: "bash_kill", Description: "Stop a background process by its id. Without an id, list the running processes.",
			Parameters: map[string]any{"type": "object", "properties": map[string]any{
				"id": str,
			}}},
		{Name: "office_edit", Description: "Edit an existing office file, keeping its formatting (journaled: /rewind undoes it; an edit that would corrupt the file is refused). " +
			"docx ops: replace (find, replace; regex, ignore_case; also headers/footers/notes), insert (anchor = text of a paragraph, position before|after|start|end, content in markdown: becomes real styles, lists, tables), append (content), delete (anchor, all), comment (anchor, text), fill_vars (vars {{name}}), accept_changes. Add track=true to replace/insert/delete for tracked changes (author). " +
			"pptx ops: replace, add_slide (spec = one slide JSON or content markdown; layout; to = position), duplicate_slide (slide, to), delete_slide (slide), move_slide (slide, to), fill (slide, shapes {\"title\":\"…\",\"body\":\"- point\",\"body2\":…, or shape name / id=N}), notes (slide, text), fill_vars. " +
			"xlsx ops: set_cell (sheet, cell, value; \"=…\" is a formula), set_cells (cells {\"B2\":\"10\",\"C2\":\"=B2*2\"}), set_range (cell = top-left, content: one row per line, cells separated by |), format (spec JSON: number_formats, roles input|formula|link|key|total|header, widths, freeze, conditional, charts, merge, comments), recalc (lists formula errors). Slides are 1-based.",
			Parameters: map[string]any{"type": "object", "properties": map[string]any{
				"path": str,
				"op": map[string]any{"type": "string", "enum": []string{"replace", "insert", "append", "delete", "comment", "fill_vars", "accept_changes",
					"add_slide", "duplicate_slide", "delete_slide", "move_slide", "fill", "notes",
					"set_cell", "set_cells", "set_range", "format", "recalc"}},
				"find": str, "replace": str, "content": str, "anchor": str, "text": str, "author": str,
				"position":    map[string]any{"type": "string", "enum": []string{"before", "after", "start", "end"}},
				"regex":       map[string]any{"type": "boolean"},
				"ignore_case": map[string]any{"type": "boolean"},
				"track":       map[string]any{"type": "boolean"},
				"all":         map[string]any{"type": "boolean"},
				"slide":       map[string]any{"type": "integer"},
				"to":          map[string]any{"type": "integer"},
				"layout":      str,
				"spec":        map[string]any{"description": "JSON (object or string)"},
				"shapes":      map[string]any{"type": "object", "additionalProperties": str},
				"cells":       map[string]any{"type": "object", "additionalProperties": str},
				"vars":        map[string]any{"type": "object", "additionalProperties": str},
				"sheet":       str, "cell": str, "value": str,
			}, "required": []string{"path", "op"}}},
		{Name: "office_create", Description: "Create a new, professionally formatted office document. Fails if the file exists unless overwrite=true (use it to regenerate your own draft, never to replace a user file). Load the skill office-docx / office-pptx / office-xlsx first for the rules. " +
			"docx: content in markdown (headings, lists, numbered lists, tables, quotes, code, links, images ![alt](path), [TOC], \\pagebreak) becomes real Word styles; title/subtitle make a cover; template = a .dotx/.docx whose styles, header, footer and page setup are kept ({{name}} markers are filled from vars, {{content}} marks where content goes). " +
			"pptx: spec JSON {\"theme\":…,\"slides\":[{\"layout\":\"title|section|content|two_columns|comparison|big_number|image|title_only|blank|closing\",\"title\",\"subtitle\",\"bullets\":[…],\"body\",\"left\",\"right\",\"left_title\",\"right_title\",\"number\",\"caption\",\"image\",\"table\":[[…]],\"chart\":{\"type\":\"column|bar|line|pie|doughnut\",\"categories\":[…],\"series\":[{\"name\",\"values\":[…]}]},\"notes\"}]} or content markdown (one # per slide); template = .potx/.pptx (its layouts are reused; its sample slides are removed unless keep_template_slides). " +
			"xlsx: spec JSON {\"sheets\":[{\"name\",\"rows\":[[…]] (\"=…\" = formula),\"number_formats\":{\"B2:D9\":\"#,##0 €\"},\"roles\":{\"B2:B5\":\"input\"},\"widths\",\"freeze\",\"table\",\"conditional\",\"charts\",\"comments\",\"validation\"}]} or content rows separated by |. " +
			"theme: default, modern, nature, elegant, mono (optionally \"default:primary=1F4E79,accent=F18F01,font=Arial\").",
			Parameters: map[string]any{"type": "object", "properties": map[string]any{
				"path": str, "title": str, "subtitle": str, "content": str, "template": str, "theme": str, "author": str,
				"spec":                 map[string]any{"description": "JSON (object or string)"},
				"vars":                 map[string]any{"type": "object", "additionalProperties": str},
				"page":                 map[string]any{"type": "string", "enum": []string{"a4", "letter"}},
				"landscape":            map[string]any{"type": "boolean"},
				"page_numbers":         map[string]any{"type": "boolean"},
				"keep_template_slides": map[string]any{"type": "boolean"},
				"overwrite":            map[string]any{"type": "boolean"},
			}, "required": []string{"path"}}},
		{Name: "question", Description: "Ask the user when you need a decision to continue (at most 4 short options; free text is also allowed). Use it instead of guessing ambiguous requirements.",
			Parameters: map[string]any{"type": "object", "properties": map[string]any{
				"query": str,
				"options": map[string]any{"type": "array", "items": map[string]any{"type": "object", "properties": map[string]any{
					"label": str, "description": str,
				}, "required": []string{"label"}}},
			}, "required": []string{"query", "options"}}},
		{Name: "todowrite", Description: "Publish the task's list of steps (at most 20) so the UI shows progress with checks. Optional: only for multi-part tasks. Mark exactly one step as working and the rest pending/done. EXACT statuses: pending, working, done (nothing else).",
			Parameters: map[string]any{"type": "object", "properties": map[string]any{
				"items": map[string]any{"type": "array", "items": map[string]any{"type": "object", "properties": map[string]any{
					"title": str, "status": map[string]any{"type": "string", "enum": []string{"pending", "working", "done"}},
				}, "required": []string{"title", "status"}}},
			}, "required": []string{"items"}}},
		{Name: "browser", Description: "Real browser for pages with JavaScript, local apps (localhost) and sites with a logged-in session. action=open (url) loads and reads; read returns the page as markdown + numbered interactive elements [n]; click (target: [n], CSS selector or visible text); type (target, text, enter=true to submit); press (key: Enter|Tab|Escape|ArrowDown…); screenshot attaches a capture; back; eval (js) runs JavaScript; close. In the desktop app it drives the app's built-in browser pane, which the user watches live; elsewhere it is headless Chrome/Edge with gregal's own profile (browser.mode: window shows a visible window). For static pages web_fetch is faster.",
			Parameters: map[string]any{"type": "object", "properties": map[string]any{
				"action":    map[string]any{"type": "string", "enum": []string{"open", "read", "click", "type", "press", "screenshot", "back", "eval", "close"}},
				"url":       str,
				"target":    str,
				"text":      str,
				"key":       str,
				"js":        str,
				"find":      str,
				"enter":     map[string]any{"type": "boolean"},
				"raw":       map[string]any{"type": "boolean"},
				"max_chars": map[string]any{"type": "integer"},
			}, "required": []string{"action"}}},
		{Name: "todoread", Description: "Read the published list of steps (progress checks).",
			Parameters: map[string]any{"type": "object", "properties": map[string]any{}}},
		{Name: "skill", Description: "Open a skill by name and return its full content. Skills are knowledge about gregal itself (sessions, configuration, modes) and about the project. The index with the names and what each one covers is in your prompt: open the relevant one BEFORE saying you don't know something or guessing it.",
			Parameters: map[string]any{"type": "object", "properties": map[string]any{
				"name": str,
			}, "required": []string{"name"}}},
		{Name: "knowledge_search", Description: "Search the user's knowledge collections (documents indexed from folders: markdown, text, code, PDF, Word, Excel, PowerPoint, HTML, plus saved notes) by keywords. Returns the best passages with their source path and location (page, sheet, slide, heading or lines). Use it when a question may be answered by the collections listed in your prompt, before answering from memory, and cite the sources you rely on as path · location. collection (name or id) narrows the search; k is the number of passages (default 6, max 20). If nothing relevant comes back, retry once with the exact terms the documents would use.",
			Parameters: map[string]any{"type": "object", "properties": map[string]any{
				"query": str, "collection": str, "k": map[string]any{"type": "integer"},
			}, "required": []string{"query"}}},
		{Name: "knowledge_note", Description: "Save a short note (a fact, decision or summary worth finding again) to a knowledge collection, so knowledge_search finds it later. collection is a name or id; without it the note goes to this session's notes. Keep notes factual and self-contained. Needs the user's approval, except for project or session collections in autonomous mode.",
			Parameters: map[string]any{"type": "object", "properties": map[string]any{
				"text": str, "title": str, "collection": str,
			}, "required": []string{"text"}}},
	}
}

// PolicyFor retorna allow|ask|deny amb la política per defecte.
func PolicyFor(name, argsJSON string) (string, string) {
	return DefaultPolicy().For(name, argsJSON)
}

// Policy és una política de permisos (del config o per defecte).
type Policy struct {
	Tools     map[string]string
	BashAllow []string
	BashDeny  []string
	// ProjectDir habilita el pas automàtic d'escriptures (write/edit) en
	// mode code o autònom quan la ruta queda dins. Buit = ask.
	ProjectDir string
}

// DefaultPolicy retorna la política base (read allow, bash classify, write/edit ask).
func DefaultPolicy() *Policy { return &Policy{} }

// Les eines conegudes viuen a tools.Noms(): un sol registre per a
// l'agent, la validació del config i el menú de permisos del TUI.

// isMCP diu si és una eina d'un servidor MCP (prefix mcp_).
func isMCP(name string) bool { return tools.EsMCP(name) }

// extraSpecs i extraExec són eines externes (MCP) registrades a l'arrencada
// o en (re)connectar un servidor des de la pàgina de connectors.
var (
	extraMu    sync.RWMutex
	extraSpecs []llm.ToolSpec
	extraExec  = map[string]func(ctx context.Context, argsJSON string) (string, error){}
)

// RegisterExtra registra una eina externa. Amb un nom ja registrat
// n'actualitza l'spec i la funció (un servidor reconnectat).
func RegisterExtra(spec llm.ToolSpec, fn func(argsJSON string) (string, error)) {
	RegisterExtraCtx(spec, func(_ context.Context, argsJSON string) (string, error) { return fn(argsJSON) })
}

// RegisterExtraCtx és RegisterExtra amb el context del torn: Esc talla la
// crida (una eina MCP remota que no respon).
func RegisterExtraCtx(spec llm.ToolSpec, fn func(ctx context.Context, argsJSON string) (string, error)) {
	extraMu.Lock()
	defer extraMu.Unlock()
	extraExec[spec.Name] = fn
	for i, s := range extraSpecs {
		if s.Name == spec.Name {
			extraSpecs[i] = spec
			return
		}
	}
	extraSpecs = append(extraSpecs, spec)
}

// UnregisterExtra treu les eines externes que compleixen drop (un
// servidor MCP esborrat o que ja no les ofereix).
func UnregisterExtra(drop func(name string) bool) {
	extraMu.Lock()
	defer extraMu.Unlock()
	kept := extraSpecs[:0]
	for _, s := range extraSpecs {
		if drop(s.Name) {
			delete(extraExec, s.Name)
			continue
		}
		kept = append(kept, s)
	}
	extraSpecs = kept
}

func extraFn(name string) (func(ctx context.Context, argsJSON string) (string, error), bool) {
	extraMu.RLock()
	defer extraMu.RUnlock()
	fn, ok := extraExec[name]
	return fn, ok
}

// SpecsAll retorna les natives + delegate + les externes registrades.
func SpecsAll() []llm.ToolSpec {
	all := append(append([]llm.ToolSpec{}, Specs()...), delegateToolSpec())
	extraMu.RLock()
	defer extraMu.RUnlock()
	return append(all, extraSpecs...)
}

// ToolMsg construeix el missatge "tool" per l'historial. El contingut mai
// és buit: alguns providers (400) rebutgen tool sense 'content'.
// images (data URLs) viatgen com a parts image_url (Fase A v1.0).
func ToolMsg(c llm.ToolCall, out string, images ...string) llm.Message {
	if strings.TrimSpace(out) == "" {
		out = "(sense sortida)"
	}
	return llm.Message{Role: "tool", Content: out, Images: images, ToolCallID: c.ID, Name: c.Function.Name}
}

// bashPermissionOverride shares explicit shell grants across the synchronous
// and background aliases. Switching tools must not loosen a shell decision.
func bashPermissionOverride(p *Policy) string {
	if p == nil {
		return ""
	}
	decision := ""
	for _, name := range []string{"bash", "bash_background"} {
		switch p.Tools[name] {
		case "deny":
			return "deny"
		case "ask":
			decision = "ask"
		case "allow":
			if decision == "" {
				decision = "allow"
			}
		}
	}
	return decision
}

// For aplica overrides del config i després la base.
func (p *Policy) For(name, argsJSON string) (string, string) {
	if !tools.EsNativa(name) && !isMCP(name) {
		return "deny", "eina desconeguda: " + name
	}
	if p != nil && name != "bash" && name != "bash_background" {
		if d, ok := p.Tools[name]; ok {
			switch d {
			case "allow", "ask", "deny":
				return d, "permissions del config"
			}
		}
	}
	if isMCP(name) {
		if IsReadOnlyMCP(name) {
			return "allow", "" // servidor MCP marcat readonly
		}
		return "ask", "eina MCP: confirma"
	}
	switch name {
	case "read", "grep", "glob", "symbols", "web_search", "web_fetch", "delegate", "read_image", "gh_issue", "gh_pr", "office_read", "office_render", "todoread", "todowrite", "question", "skill", "preview", "knowledge_search", "dashboard_save":
		return "allow", ""
	case "knowledge_note":
		// Escriu a una base de coneixement que dura més que el torn.
		return "ask", "knowledge: confirm saving a note"
	case "bash_output", "bash_kill":
		// Llegir o aturar un procés que ja s'ha aprovat no torna a preguntar.
		return "allow", ""
	case "browser":
		// Obrir, llegir i capturar és com web_fetch; clicar, escriure i
		// executar JavaScript actua sobre la pàgina i demana permís.
		var a struct {
			Action string `json:"action"`
		}
		_ = json.Unmarshal([]byte(argsJSON), &a)
		if tools.BrowserActionSegura(a.Action) {
			return "allow", ""
		}
		return "ask", "navegador: actua sobre la pàgina (" + a.Action + ")"
	case "bash", "bash_background":
		var a struct {
			Command string `json:"command"`
		}
		override := bashPermissionOverride(p)
		if err := json.Unmarshal([]byte(argsJSON), &a); err != nil {
			if override != "" {
				return override, "permissions del config"
			}
			return "ask", "arguments il·legibles"
		}
		var allow, deny []string
		if p != nil {
			allow, deny = p.BashAllow, p.BashDeny
		}
		decision, reason := tools.ClassifyWith(a.Command, allow, deny)
		if decision == "deny" {
			return decision, reason
		}
		if override != "" {
			return override, "permissions del config"
		}
		return decision, reason
	case "git_commit", "gh_pr_create":
		// Commits i PRs queden a la història i al remot: sempre es pregunta
		// (llevat d'un permissions.tools explícit, que ja s'ha mirat a dalt).
		return "ask", "git: confirma (" + name + ")"
	default: // write, edit
		return "ask", "escriptura: confirma"
	}
}

// Decide aplica el mode i després la política.
// mode "chat" = només lectura (estil Codex read-only): write/edit denegats,
// bash només si és a la llista segura. mode "goal" (objectiu) també és només
// lectura: primer es concreta l'objectiu, després s'executa en mode code.
// mode "code" = política normal.
//
// En mode autònom no es demana permís mai: tot el que els altres modes
// preguntarien («ask») passa sol. El que està prohibit continua prohibit:
// el classificador de shell (sudo, rm -rf /, curl | sh…), les regles «deny»
// del config i l'eina question. Demanat el 2026-10-16: «si estem en autònom
// … que decideixi ell les coses i que no demani permisos i ho faci tot».
func (p *Policy) Decide(mode, name, argsJSON string) (string, string) {
	d, reason := p.decide(mode, name, argsJSON)
	if mode == ModeAutonomous && d == "ask" {
		return "allow", reason
	}
	return d, reason
}

func (p *Policy) decide(mode, name, argsJSON string) (string, string) {
	if mode == ModeInspect || mode == ModeChat || mode == ModeGoal {
		switch name {
		case "office_edit", "office_create", "bash_kill":
			return "deny", "read-only mode: cannot modify documents or stop processes"
		case "git_commit", "gh_pr_create":
			return "deny", "read-only mode: cannot commit or open pull requests"
		case "knowledge_note":
			return "deny", "read-only mode: cannot save knowledge notes (search is allowed)"
		}
	}
	if (mode == ModeInspect || mode == ModeChat || mode == ModeGoal) && (name == "bash" || name == "browser") {
		// Read-only modes cannot inherit broader shell or browser grants.
		if d, reason := DefaultPolicy().For(name, argsJSON); d != "allow" {
			return "deny", "read-only mode: " + reason
		}
	}
	// Consulta és l'alternativa sense popups: permet totes les eines natives
	// de lectura i només shell classificat com a lectura. Qualsevol altra cosa
	// es denega explícitament (mai es converteix en una aprovació).
	if mode == ModeInspect {
		switch name {
		case "write", "edit", "patch":
			return "deny", "mode consulta: no modifica fitxers"
		case "bash_background", "office_open":
			return "deny", "mode consulta: no engega processos ni programes"
		case "browser":
			if d, _ := p.For(name, argsJSON); d == "allow" {
				return "allow", ""
			}
			return "deny", "mode consulta: el navegador només mira (open, read, screenshot)"
		case "bash":
			d, reason := p.For(name, argsJSON)
			if d == "allow" {
				return "allow", ""
			}
			return "deny", "mode consulta: comanda no és lectura segura (" + reason + ")"
		default:
			if isMCP(name) && !IsReadOnlyMCP(name) {
				return "deny", "mode consulta: MCP desactivat"
			}
		}
	}
	if mode == ModeChat || mode == ModeGoal {
		què := "mode xat"
		if mode == ModeGoal {
			què = "mode objectiu"
		}
		switch name {
		case "write", "edit", "patch":
			return "deny", què + ": només lectura (passa a /mode code per editar)"
		case "bash_background", "office_open":
			return "deny", què + ": només lectura (no engega processos ni programes)"
		case "browser":
			if d, _ := p.For(name, argsJSON); d == "allow" {
				return "allow", ""
			}
			return "deny", què + ": el navegador només mira (open, read, screenshot)"
		case "bash":
			if d, reason := p.For(name, argsJSON); d != "allow" {
				return "deny", què + ": només lectura (" + reason + ")"
			}
			return "allow", ""
		default:
			if isMCP(name) && !IsReadOnlyMCP(name) {
				return "deny", què + ": només lectura (MCP desactivat)"
			}
		}
	}
	// Autònom: treballa sol i només pregunta pel que és perillós
	// (autonomous_policy.go).
	if mode == ModeAutonomous {
		if d, reason, ok := p.decideAutonomous(name, argsJSON); ok {
			return d, reason
		}
	}
	// En code i autònom, escriure dins del projecte passa sol. Un override
	// explícit del config sempre preval; tota altra acció «ask» continua
	// requerint aprovació, també en mode autònom.
	if (mode == ModeCode || mode == ModeAutonomous || mode == ModeDebug) && (name == "write" || name == "edit" || name == "patch") {
		if p != nil {
			if d, ok := p.Tools[name]; ok {
				switch d {
				case "allow", "ask", "deny":
					return d, "permissions del config"
				}
			}
			if InsideProject(p.ProjectDir, toolPath(name, argsJSON)) {
				return "allow", ""
			}
		}
	}
	d, reason := p.For(name, argsJSON)
	return d, reason
}

// Exec executa una eina amb l'etiqueta de sessió per defecte (TUI,
// headless, Telegram: allà només n'hi ha una).
func Exec(name, argsJSON string) (out string, images []string, err error) {
	return ExecSess(procSession, name, argsJSON)
}

// ExecSess és el mateix dient de quina sessió ve. Importa per als
// processos de segon pla: queden etiquetats amb ella, i és el que fa que
// surtin al Terminal de la pestanya que els ha engegat i que es matin en
// tancar-la. Abans tots duien "agent" i cap de les dues coses passava.
func ExecSess(session, name, argsJSON string) (out string, images []string, err error) {
	return ExecIn(session, "", name, argsJSON)
}

// ExecIn és el mateix dient també ON: el workspace de la sessió. Amb dir
// buit es treballa al directori del procés, que és el que volen el TUI i
// el headless. Vegeu resolArgs.
func ExecIn(session, dir, name, argsJSON string) (out string, images []string, err error) {
	return ExecCtx(context.Background(), session, dir, name, argsJSON)
}

// execCtxEina és el cos d'ExecCtx (lifecycle.go), sense els hooks
// post_tool: executa l'eina amb el context del torn.
func execCtxEina(ctx context.Context, session, dir, name, argsJSON string) (out string, images []string, err error) {
	if err := ctx.Err(); err != nil {
		return "", nil, err
	}
	// Foto git abans d'una ordre de shell que pot escriure (gitcheckpoint.go):
	// el propietari és el de MarkConvoFor ("" al TUI i al Telegram).
	checkpointAbansDe(session, dir, name, argsJSON)
	owner := session
	if session == "" {
		session = procSession
	}
	argsJSON = resolArgs(dir, argsJSON)
	// Què ha llegit i editat el model (tools/seguiment.go): la guarda de
	// lectura abans d'editar i el «sense canvis» de les relectures.
	seg := tools.SeguimentDe(session)
	seg.NouPas()
	switch name {
	case "preview":
		return previewRequest(argsJSON)
	case "dashboard_save":
		return "", nil, fmt.Errorf("dashboard_save is only available in the Gregal web/desktop app; show the charts as gregal-plot blocks instead")
	case "read":
		var a struct {
			Path   string `json:"path"`
			Offset int    `json:"offset"`
			Limit  int    `json:"limit"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &a); err != nil {
			return "", nil, err
		}
		out, err := seg.Read(a.Path, a.Offset, a.Limit)
		if err != nil {
			return "", nil, err
		}
		return "llegit " + a.Path + "\n" + out, nil, nil
	case "office_read":
		var oa struct {
			Path   string `json:"path"`
			Detail string `json:"detail"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &oa); err != nil {
			return "", nil, err
		}
		oout, oerr := tools.OfficeReadDetail(oa.Path, oa.Detail)
		if oerr != nil {
			return "", nil, oerr
		}
		limit := 12000
		if oa.Detail == "full" {
			limit = 40000
		}
		if len(oout) > limit {
			oout = oout[:limit] + "\n… (retallat; demana detail=full o llegeix per parts)"
		}
		return "llegit " + oa.Path + "\n" + oout, nil, nil
	case "office_render":
		var orr struct {
			Path  string `json:"path"`
			Pages string `json:"pages"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &orr); err != nil {
			return "", nil, err
		}
		out, imgs, err := tools.OfficeRender(ctx, tools.RenderOpts{Path: orr.Path, Pages: orr.Pages})
		if err != nil {
			return "", nil, err
		}
		return out, imgs, nil
	case "office_open":
		var oo struct {
			Path string `json:"path"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &oo); err != nil {
			return "", nil, err
		}
		oout, oerr := tools.OfficeOpen(oo.Path)
		if oerr != nil {
			return "", nil, oerr
		}
		return oout, nil, nil
	case "office_edit":
		var oe struct {
			Path       string          `json:"path"`
			Op         string          `json:"op"`
			Sheet      string          `json:"sheet"`
			Cell       string          `json:"cell"`
			Value      json.RawMessage `json:"value"`
			Find       string          `json:"find"`
			Replace    string          `json:"replace"`
			Content    string          `json:"content"`
			Anchor     string          `json:"anchor"`
			Position   string          `json:"position"`
			Text       string          `json:"text"`
			Author     string          `json:"author"`
			Regex      bool            `json:"regex"`
			IgnoreCase bool            `json:"ignore_case"`
			Track      bool            `json:"track"`
			All        bool            `json:"all"`
			Slide      int             `json:"slide"`
			To         int             `json:"to"`
			Layout     string          `json:"layout"`
			Spec       json.RawMessage `json:"spec"`
			Shapes     map[string]any  `json:"shapes"`
			Cells      map[string]any  `json:"cells"`
			Vars       map[string]any  `json:"vars"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &oe); err != nil {
			return "", nil, err
		}
		out, err := tools.OfficeEdit(tools.EditOpts{
			Path: oe.Path, Op: oe.Op, Sheet: oe.Sheet, Cell: oe.Cell, Value: rawText(oe.Value),
			Find: oe.Find, Replace: oe.Replace, Content: oe.Content, Anchor: oe.Anchor, Position: oe.Position,
			Text: oe.Text, Author: oe.Author, Regex: oe.Regex, IgnoreCase: oe.IgnoreCase, Track: oe.Track, All: oe.All,
			Slide: oe.Slide, To: oe.To, Layout: oe.Layout, Spec: rawText(oe.Spec),
			Shapes: strMap(oe.Shapes), Cells: strMap(oe.Cells), Vars: strMap(oe.Vars),
		})
		if err != nil {
			return "", nil, err
		}
		return out, nil, nil
	case "read_image":
		var ia struct {
			Path string `json:"path"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &ia); err != nil {
			return "", nil, err
		}
		dataURL, err := tools.ReadImageDataURL(ia.Path)
		if err != nil {
			return "", nil, err
		}
		return "imatge adjunta: " + ia.Path, []string{dataURL}, nil
	case "bash":
		var a struct {
			Command string `json:"command"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &a); err != nil {
			return "", nil, err
		}
		out, err := tools.BashCtx(ctx, dir, a.Command, tools.DefaultTimeout)
		return out, nil, err
	case "bash_background":
		var a struct {
			Command string `json:"command"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &a); err != nil {
			return "", nil, err
		}
		p, err := Procs().StartCtx(ctx, session, procDirOr(dir), a.Command)
		if err != nil {
			return "", nil, err
		}
		// Un procés que peta de seguida (ordre inexistent) ha de dir-ho
		// ara, no d'aquí a tres passos.
		if p.Wait(400 * time.Millisecond) {
			return p.Describe() + "\n" + p.Tail(40), nil, nil
		}
		return "engegat en segon pla amb id " + p.ID + " (" + a.Command + "). Llegeix-ne la sortida amb bash_output.", nil, nil
	case "bash_output":
		var a struct {
			ID    string `json:"id"`
			Lines int    `json:"lines"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &a); err != nil {
			return "", nil, err
		}
		p := Procs().Get(a.ID)
		if p == nil {
			return "", nil, fmt.Errorf("procés %q desconegut", a.ID)
		}
		n := a.Lines
		if n <= 0 || n > 400 {
			n = 120
		}
		return p.Describe() + "\n" + p.Tail(n), nil, nil
	case "bash_kill":
		var a struct {
			ID string `json:"id"`
		}
		_ = json.Unmarshal([]byte(argsJSON), &a)
		if strings.TrimSpace(a.ID) == "" {
			list := Procs().List(session)
			if len(list) == 0 {
				return "cap procés en segon pla", nil, nil
			}
			var b strings.Builder
			for _, sn := range list {
				estat := "acabat"
				if sn.Running {
					estat = "en marxa"
				}
				fmt.Fprintf(&b, "%s · %s · %s\n", sn.ID, estat, sn.Cmd)
			}
			return b.String(), nil, nil
		}
		if !Procs().Kill(a.ID) {
			return "", nil, fmt.Errorf("procés %q desconegut", a.ID)
		}
		return "aturat " + a.ID, nil, nil
	case "write":
		var a struct {
			Path    string `json:"path"`
			Content string `json:"content"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &a); err != nil {
			return "", nil, err
		}
		n, err := tools.Write(a.Path, []byte(a.Content))
		if err != nil {
			return "", nil, err
		}
		apendix := apendixEdicio(ctx, a.Path)
		seg.Coneix(a.Path)
		return fmt.Sprintf("escrit %s (%d bytes)", a.Path, n) + apendix, nil, nil
	case "edit":
		var a struct {
			Path string `json:"path"`
			Old  string `json:"old_string"`
			New  string `json:"new_string"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &a); err != nil {
			return "", nil, err
		}
		nota, err := seg.Edit(a.Path, a.Old, a.New)
		if err != nil {
			return "", nil, err
		}
		apendix := apendixEdicio(ctx, a.Path)
		seg.Coneix(a.Path)
		return "edit aplicat a " + a.Path + nota + apendix, nil, nil
	case "patch":
		var pa struct {
			Path  string `json:"path"`
			Edits []struct {
				Old       string `json:"old"`
				New       string `json:"new"`
				AfterLine int    `json:"after_line"`
			} `json:"edits"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &pa); err != nil {
			return "", nil, err
		}
		ops := make([]tools.PatchOp, 0, len(pa.Edits))
		for _, e := range pa.Edits {
			ops = append(ops, tools.PatchOp{Old: e.Old, New: e.New, AfterLine: e.AfterLine})
		}
		nota, err := seg.Patch(pa.Path, ops)
		if err != nil {
			return "", nil, err
		}
		apendix := apendixEdicio(ctx, pa.Path)
		seg.Coneix(pa.Path)
		return fmt.Sprintf("patch aplicat a %s (%d edicions)", pa.Path, len(ops)) + nota + apendix, nil, nil
	case "grep":
		var a struct {
			Pattern string `json:"pattern"`
			Dir     string `json:"dir"`
			Include string `json:"include"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &a); err != nil {
			return "", nil, err
		}
		out, err := tools.Grep(a.Pattern, a.Dir, a.Include, 50)
		return out, nil, err
	case "glob":
		var a struct {
			Pattern string `json:"pattern"`
			Dir     string `json:"dir"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &a); err != nil {
			return "", nil, err
		}
		out, err := tools.Glob(a.Pattern, a.Dir, 100)
		return out, nil, err
	case "symbols":
		out, err := symbolsTool(dir, argsJSON)
		return out, nil, err
	case "web_search":
		var a struct {
			Query string `json:"query"`
			Count int    `json:"count"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &a); err != nil {
			return "", nil, err
		}
		out, err := tools.WebSearchCtx(ctx, a.Query, a.Count)
		return out, nil, err
	case "web_fetch":
		var a struct {
			URL      string `json:"url"`
			Find     string `json:"find"`
			MaxChars int    `json:"max_chars"`
			Raw      bool   `json:"raw"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &a); err != nil {
			return "", nil, err
		}
		out, err := tools.WebFetchOpts_Ctx(ctx, a.URL, tools.WebFetchOpts{MaxChars: a.MaxChars, Find: a.Find, Raw: a.Raw})
		return out, nil, err
	case "gh_issue", "gh_pr":
		out, err := tools.GitHubCtx(ctx, name, argsJSON)
		return out, nil, err
	case "git_commit":
		out, err := tools.GitCommitTool(owner, procDirOr(dir), argsJSON)
		return out, nil, err
	case "gh_pr_create":
		out, err := tools.GHPRCreateTool(ctx, procDirOr(dir), argsJSON)
		return out, nil, err
	case "browser":
		var ba map[string]any
		if err := json.Unmarshal([]byte(argsJSON), &ba); err != nil {
			return "", nil, err
		}
		action, _ := ba["action"].(string)
		return tools.BrowserAction(ctx, action, ba)
	case "office_create":
		var oc struct {
			Path               string          `json:"path"`
			Title              string          `json:"title"`
			Subtitle           string          `json:"subtitle"`
			Content            string          `json:"content"`
			Template           string          `json:"template"`
			Theme              string          `json:"theme"`
			Author             string          `json:"author"`
			Spec               json.RawMessage `json:"spec"`
			Vars               map[string]any  `json:"vars"`
			Page               string          `json:"page"`
			Landscape          bool            `json:"landscape"`
			PageNumbers        *bool           `json:"page_numbers"`
			KeepTemplateSlides bool            `json:"keep_template_slides"`
			Overwrite          bool            `json:"overwrite"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &oc); err != nil {
			return "", nil, err
		}
		tpl := oc.Template
		if tpl != "" && !filepath.IsAbs(tpl) && dir != "" {
			tpl = filepath.Join(dir, tpl)
		}
		out, err := tools.OfficeCreateOpts(tools.CreateOpts{
			Path: oc.Path, Title: oc.Title, Subtitle: oc.Subtitle, Content: oc.Content, Template: tpl,
			Theme: oc.Theme, Author: oc.Author, Spec: rawText(oc.Spec), Vars: strMap(oc.Vars), Page: oc.Page,
			Landscape: oc.Landscape, NoPageNumbers: oc.PageNumbers != nil && !*oc.PageNumbers,
			KeepTemplateSlides: oc.KeepTemplateSlides, Overwrite: oc.Overwrite, BaseDir: dir,
		})
		if err != nil {
			return "", nil, err
		}
		return out, nil, nil
	case "question":
		// Sense UI interactiva (headless/loop) no es pot triar: es guia
		// el model perquè demani per text en comptes de bloquejar-se.
		q, opts, err := tools.ParseQuestion(argsJSON)
		if err != nil {
			return "", nil, err
		}
		_ = q
		_ = opts
		return PreguntaSenseUI, nil, nil
	case "todowrite":
		var tw struct {
			Items []struct {
				Title  string `json:"title"`
				Status string `json:"status"`
			} `json:"items"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &tw); err != nil {
			return "", nil, err
		}
		items := make([]tools.TodoItem, 0, len(tw.Items))
		for _, it := range tw.Items {
			items = append(items, tools.TodoItem{Title: it.Title, Status: it.Status})
		}
		tools.TodoSet(items)
		return tools.TodoRender(), nil, nil
	case "todoread":
		return tools.TodoRender(), nil, nil
	case "skill":
		// El paràmetre es diu `name`; `nom` era el d'abans i encara arriba
		// de models i sessions que el van aprendre.
		var a struct {
			Name string `json:"name"`
			Nom  string `json:"nom"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &a); err != nil {
			return "", nil, err
		}
		return LlegeixSkill(dir, firstOf(a.Name, a.Nom))
	case "knowledge_search":
		out, err := knowledge.SearchTool(ctx, knowledge.Where{Project: procDirOr(dir), Session: session}, argsJSON)
		return out, nil, err
	case "knowledge_note":
		out, err := knowledge.NoteTool(knowledge.Where{Project: procDirOr(dir), Session: session}, argsJSON)
		return out, nil, err
	case "delegate":
		out, err := delegateExec(withDelegateScope(ctx, session, dir), argsJSON)
		return out, nil, err
	default:
		if fn, ok := extraFn(name); ok {
			out, err := fn(ctx, argsJSON)

			return out, nil, err
		}
		return "", nil, fmt.Errorf("eina desconeguda: %s", name)
	}
}

func firstOf(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// rawText accepta un camp JSON que pot ser text o un objecte/llista (els
// models passen spec de les dues maneres) i el torna com a text.
func rawText(r json.RawMessage) string {
	t := strings.TrimSpace(string(r))
	if t == "" || t == "null" {
		return ""
	}
	if strings.HasPrefix(t, `"`) {
		var s string
		if json.Unmarshal(r, &s) == nil {
			return s
		}
	}
	return t
}

// strMap converteix valors JSON qualssevol (números, booleans) a text.
func strMap(m map[string]any) map[string]string {
	if len(m) == 0 {
		return nil
	}
	out := make(map[string]string, len(m))
	for k, v := range m {
		switch x := v.(type) {
		case string:
			out[k] = x
		case nil:
			out[k] = ""
		case float64:
			out[k] = strconv.FormatFloat(x, 'f', -1, 64)
		default:
			b, _ := json.Marshal(x)
			out[k] = string(b)
		}
	}
	return out
}

// Stepper fa un pas del loop: historial → (contingut, tool_calls).
type Stepper func(ctx context.Context, hist []llm.Message) (string, []llm.ToolCall, error)

// Loop orquestra passos fins a resposta final (per tests i headless;
// el TUI fa servir les peces per separat per poder demanar confirmacions).
//
// Mai acaba buit per pressupost (vegeu pressupost.go): dos passos abans del
// límit avisa el model, i en arribar-hi fa una última crida sense eines
// demanant la millor resposta amb el que té. La resposta torna sense error
// i amb Parcial = true; Continua la reprèn amb pressupost nou.
type Loop struct {
	MaxSteps int
	Step     Stepper
	RunTool  func(ctx context.Context, name, argsJSON string) (string, []string, error)
	Decide   func(name, argsJSON string) (string, string)
	// Finalize genera la resposta final SENSE eines quan s'esgota el
	// pressupost (o el model falla a mig torn). Sense, es fa servir Step i
	// les crides que torni es descarten: cap eina no s'executa a la síntesi.
	Finalize func(ctx context.Context, hist []llm.Message) (string, error)
	// FinalPrompt és la consigna de la crida final ("" =
	// PromptFinalExploracio en la llengua Lang).
	FinalPrompt string
	Lang        string

	// Sortides de l'última Run/Continua.
	// Parcial diu que la resposta és la síntesi forçada (pressupost o
	// error), no una resposta que el model hagi donat per acabada.
	Parcial bool
	// Motiu és per què és parcial: "steps" o el text de l'error.
	Motiu string
	// Hist és l'historial sencer, amb la resposta final (per continuar).
	Hist []llm.Message
}

// MotiuPassos és el Motiu d'una resposta parcial per pressupost exhaurit.
const MotiuPassos = "steps"

// Run executa el loop sencer. Les ask es tracten com allow (headless).
func (l *Loop) Run(ctx context.Context, task string) (string, []llm.Message, error) {
	return l.corre(ctx, []llm.Message{{Role: "user", Content: task}})
}

// Continua reprèn una exploració anterior (l.Hist, o hist si se'n dona)
// amb pressupost nou i la conversa intacta.
func (l *Loop) Continua(ctx context.Context, hist []llm.Message) (string, []llm.Message, error) {
	if hist == nil {
		hist = l.Hist
	}
	h := append(append([]llm.Message(nil), hist...), llm.Message{Role: "user", Content: PromptContinuaExploracio})
	return l.corre(ctx, h)
}

func (l *Loop) corre(ctx context.Context, hist []llm.Message) (string, []llm.Message, error) {
	max := l.MaxSteps
	if max <= 0 {
		max = 10
	}
	l.Parcial, l.Motiu, l.Hist = false, "", nil
	doom := &DoomTracker{}
	for i := 0; i < max; i++ {
		hist = AmbAvisPassos(hist, i+1, max)
		content, calls, err := l.Step(ctx, hist)
		if err != nil {
			// Cancel·lat per l'usuari, o sense res fet encara: no hi ha
			// feina a salvar i l'error és la resposta honesta.
			if errors.Is(ctx.Err(), context.Canceled) || i == 0 {
				l.Hist = hist
				return "", hist, err
			}
			return l.tanca(ctx, hist, err.Error())
		}
		hist = append(hist, llm.Message{Role: "assistant", Content: content, ToolCalls: calls})
		if len(calls) == 0 {
			l.Hist = hist
			return content, hist, nil
		}
		// Primer es decideix (permís i repetició, en ordre); després les
		// lectures s'executen en paral·lel i la resta en seqüència.
		type resultat struct {
			out  string
			imgs []string
		}
		fixos := make([]string, len(calls))
		for i, c := range calls {
			// El doom es comprova abans del permís: repetir una crida
			// denegada també és encallar-se.
			repetida := doom.Note(DoomSig(c.Function.Name, c.Function.Arguments))
			dec, reason := l.Decide(c.Function.Name, c.Function.Arguments)
			if dec == "deny" {
				fixos[i] = "EINA BLOQUEJADA: " + reason
			} else if repetida {
				fixos[i] = DoomGuide
			}
		}
		res := RunCalls(calls, func(i int) bool { return fixos[i] != "" }, func(_ int, c llm.ToolCall) resultat {
			r, ri, rerr := l.RunTool(ctx, c.Function.Name, c.Function.Arguments)
			if rerr != nil {
				return resultat{out: "ERROR: " + rerr.Error()}
			}
			return resultat{out: r, imgs: ri}
		})
		for i, c := range calls {
			out, outImgs := res[i].out, res[i].imgs
			if fixos[i] != "" {
				out, outImgs = fixos[i], nil
			}
			hist = append(hist, llm.Message{
				Role: "tool", Content: out, Images: outImgs,
				ToolCallID: c.ID, Name: c.Function.Name,
			})
		}
	}
	return l.tanca(ctx, hist, MotiuPassos)
}

// tanca fa la crida final sense eines i torna la resposta parcial. Si la
// crida falla o surt buida, el resum mecànic (ResumDeFeina): mai un error
// per haver treballat.
func (l *Loop) tanca(ctx context.Context, hist []llm.Message, motiu string) (string, []llm.Message, error) {
	l.Parcial, l.Motiu = true, motiu
	prompt := l.FinalPrompt
	if prompt == "" {
		prompt = PromptFinalExploracio(l.Lang)
	}
	fh := append(append([]llm.Message(nil), hist...), llm.Message{Role: "user", Content: prompt})
	// Si el que ha vençut és el temps del torn, la síntesi té el seu.
	fctx := ctx
	if ctx.Err() != nil {
		var cancel context.CancelFunc
		fctx, cancel = context.WithTimeout(context.WithoutCancel(ctx), TimeoutDecisio)
		defer cancel()
	}
	var final string
	var ferr error
	if l.Finalize != nil {
		final, ferr = l.Finalize(fctx, fh)
	} else {
		// Les crides que torni el pas es descarten: no s'executen.
		final, _, ferr = l.Step(fctx, fh)
	}
	final = NetejaFinal(final)
	if ferr != nil || final == "" {
		m := motiu
		if ferr != nil {
			m += "; " + ferr.Error()
		}
		final = ResumDeFeina(hist, m, l.Lang)
	}
	hist = append(hist, llm.Message{Role: "assistant", Content: final})
	l.Hist = hist
	return final, hist, nil
}

// resolArgs resol les rutes relatives dels arguments d'una eina contra el
// directori de la sessió.
//
// El bug que això tanca: `bash` i les eines de fitxers treballaven al
// directori del PROCÉS, no al workspace de la sessió. A l'escriptori, una
// pestanya que havia canviat de projecte seguia llegint i executant a
// l'altre —el panell de fitxers sí que el respectava, i les eines no—, i
// amb dues pestanyes en projectes diferents la cosa era pitjor encara.
//
// Es fa aquí, sobre el JSON, i no a cada cas: són dotze llocs i n'hi
// hauria hagut prou que un s'oblidés. Les claus són sempre rutes a les
// nostres eines ("path" a read/write/edit/patch/office/read_image, "dir" a
// grep/glob); "url", "repo" i companyia no s'hi toquen.
func resolArgs(dir, argsJSON string) string {
	if strings.TrimSpace(dir) == "" || strings.TrimSpace(argsJSON) == "" {
		return argsJSON
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(argsJSON), &m); err != nil {
		return argsJSON // que falli on toca, amb el seu error
	}
	canviat := false
	if p, ok := m["path"].(string); ok && strings.TrimSpace(p) != "" && !tools.IsRooted(p) {
		m["path"] = filepath.Join(dir, p)
		canviat = true
	}
	// "dir" buit o absent vol dir «el projecte»: abans era el del procés.
	if d, ok := m["dir"].(string); !ok || strings.TrimSpace(d) == "" {
		if _, hiEs := m["dir"]; hiEs || esDeDirectori(m) {
			m["dir"] = dir
			canviat = true
		}
	} else if !tools.IsRooted(d) {
		m["dir"] = filepath.Join(dir, d)
		canviat = true
	}
	if !canviat {
		return argsJSON
	}
	raw, err := json.Marshal(m)
	if err != nil {
		return argsJSON
	}
	return string(raw)
}

// esDeDirectori diu si els arguments són d'una eina que cerca dins d'un
// directori (grep/glob): tenen "pattern" i, si no diuen "dir", han de
// començar pel projecte.
func esDeDirectori(m map[string]any) bool {
	_, te := m["pattern"]
	return te
}

// resolArgsProva exposa resolArgs als tests d'altres paquets.
func ResolArgsProva(dir, argsJSON string) string { return resolArgs(dir, argsJSON) }
