// Package mcpimport llegeix els servidors MCP que ja hi ha configurats a
// Claude Code, Codex i opencode i els proposa a Gregal (`gregal mcp import`,
// `/mcp import`). Només llegeix: res no s'escriu fins que la persona diu que
// sí, i els secrets (variables d'entorn, capçaleres, claus als arguments) no
// surten mai dels textos que es mostren o es registren.
package mcpimport

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"

	"gregal/internal/config"
)

// Orígens que sap llegir.
const (
	ClaudeCode = "claude"
	Codex      = "codex"
	OpenCode   = "opencode"
)

// Found és un servidor trobat en un fitxer d'un altre programa.
type Found struct {
	Name   string
	Cfg    config.MCPServerCfg
	Origin string // claude | codex | opencode
	File   string
	// Disabled: l'altre programa el té desactivat; no s'importa per defecte.
	Disabled bool
	// Notes diuen què no s'ha pogut traduir (p. ex. un secret OAuth).
	Notes []string
}

// Result és el que Discover ha trobat. Si dos fitxers defineixen el mateix
// nom, MakePlan es queda amb el primer (el del projecte abans que el
// d'usuari, i Claude Code abans que Codex i opencode).
type Result struct {
	Found   []Found
	Scanned []string // fitxers que existien i s'han llegit
	Errors  []string // fitxers que existien però no s'han pogut llegir
}

// Discover llegeix els fitxers de configuració de home i del projecte cwd.
// from filtra per origen (claude, codex, opencode; buit o "all" = tots).
func Discover(home, cwd string, from ...string) Result {
	want := map[string]bool{}
	for _, f := range from {
		want[strings.ToLower(strings.TrimSpace(f))] = true
	}
	use := func(o string) bool { return len(want) == 0 || want["all"] || want[o] }
	var res Result
	read := func(path, origin string, parse func(raw []byte) ([]Found, error)) {
		raw, err := os.ReadFile(path)
		if err != nil {
			if !os.IsNotExist(err) {
				res.Errors = append(res.Errors, fmt.Sprintf("%s: %v", path, err))
			}
			return
		}
		found, err := parse(raw)
		if err != nil {
			res.Errors = append(res.Errors, fmt.Sprintf("%s: %v", path, err))
			return
		}
		res.Scanned = append(res.Scanned, path)
		for i := range found {
			found[i].Origin, found[i].File = origin, path
		}
		res.Found = append(res.Found, found...)
	}
	if use(ClaudeCode) {
		if cwd != "" {
			read(filepath.Join(cwd, ".mcp.json"), ClaudeCode, func(b []byte) ([]Found, error) { return parseClaude(b, "") })
		}
		read(filepath.Join(home, ".claude.json"), ClaudeCode, func(b []byte) ([]Found, error) { return parseClaude(b, cwd) })
	}
	if use(Codex) {
		codexHome := os.Getenv("CODEX_HOME")
		if codexHome == "" {
			codexHome = filepath.Join(home, ".codex")
		}
		read(filepath.Join(codexHome, "config.toml"), Codex, parseCodex)
	}
	if use(OpenCode) {
		cfgHome := os.Getenv("XDG_CONFIG_HOME")
		if cfgHome == "" {
			cfgHome = filepath.Join(home, ".config")
		}
		for _, name := range []string{"opencode.json", "opencode.jsonc"} {
			if cwd != "" {
				read(filepath.Join(cwd, name), OpenCode, parseOpenCode)
			}
			read(filepath.Join(cfgHome, "opencode", name), OpenCode, parseOpenCode)
		}
	}
	return res
}

var nameRe = regexp.MustCompile(`[^A-Za-z0-9_-]+`)

// CleanName adapta un nom d'un altre programa als de Gregal (sense espais
// ni barres).
func CleanName(n string) string {
	return strings.Trim(nameRe.ReplaceAllString(strings.TrimSpace(n), "-"), "-")
}

// ---- Claude Code -------------------------------------------------------

func parseClaude(raw []byte, cwd string) ([]Found, error) {
	var doc struct {
		MCPServers map[string]claudeServer `json:"mcpServers"`
		Projects   map[string]struct {
			MCPServers map[string]claudeServer `json:"mcpServers"`
		} `json:"projects"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, err
	}
	var out []Found
	add := func(m map[string]claudeServer) {
		names := make([]string, 0, len(m))
		for n := range m {
			names = append(names, n)
		}
		sort.Strings(names)
		for _, name := range names {
			if f, ok := m[name].found(name); ok {
				out = append(out, f)
			}
		}
	}
	// El projecte primer: és més específic que el d'usuari.
	for path, p := range doc.Projects {
		if cwd != "" && samePath(path, cwd) {
			add(p.MCPServers)
		}
	}
	add(doc.MCPServers)
	return out, nil
}

type claudeServer struct {
	Type    string            `json:"type"`
	Command string            `json:"command"`
	Args    []string          `json:"args"`
	Env     map[string]string `json:"env"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers"`
}

func (s claudeServer) found(name string) (Found, bool) {
	cfg := config.MCPServerCfg{Command: s.Command, Args: s.Args, Env: s.Env, URL: s.URL, Headers: s.Headers}
	switch strings.ToLower(s.Type) {
	case "http", "streamable-http":
		cfg.Transport = "http"
	case "sse":
		cfg.Transport = "sse"
	}
	if cfg.URL != "" {
		cfg.Command, cfg.Args, cfg.Env = "", nil, nil
	} else {
		cfg.Headers, cfg.Transport = nil, ""
	}
	return mk(name, cfg, false, nil)
}

func samePath(a, b string) bool {
	a, b = filepath.Clean(filepath.FromSlash(a)), filepath.Clean(filepath.FromSlash(b))
	return strings.EqualFold(a, b)
}

// ---- Codex -------------------------------------------------------------

func parseCodex(raw []byte) ([]Found, error) {
	root := parseTOML(string(raw))
	servers, _ := root["mcp_servers"].(tomlTable)
	names := make([]string, 0, len(servers))
	for n := range servers {
		names = append(names, n)
	}
	sort.Strings(names)
	var out []Found
	for _, name := range names {
		t, ok := servers[name].(tomlTable)
		if !ok {
			continue
		}
		cfg := config.MCPServerCfg{Env: tomlStringMap(t["env"])}
		cfg.Command, _ = t["command"].(string)
		cfg.Args = tomlStrings(t["args"])
		cfg.URL, _ = t["url"].(string)
		var notes []string
		if cfg.URL != "" {
			cfg.Command, cfg.Args, cfg.Env = "", nil, nil
			cfg.Headers = tomlStringMap(t["http_headers"])
			setHeader := func(k, v string) {
				if cfg.Headers == nil {
					cfg.Headers = map[string]string{}
				}
				cfg.Headers[k] = v
			}
			// Capçaleres que Codex llegeix d'una variable d'entorn: aquí
			// també queden com a referència ${VAR}, sense cap valor.
			for h, envName := range tomlStringMap(t["env_http_headers"]) {
				setHeader(h, "${"+envName+"}")
			}
			if v, _ := t["bearer_token_env_var"].(string); v != "" {
				setHeader("Authorization", "Bearer ${"+v+"}")
			}
		} else if vars := tomlStrings(t["env_vars"]); len(vars) > 0 {
			notes = append(notes, "env_vars ("+strings.Join(vars, ", ")+") come from the environment of the process")
		}
		disabled := false
		if en, ok := t["enabled"].(bool); ok && !en {
			disabled = true
		}
		if f, ok := mk(name, cfg, disabled, notes); ok {
			out = append(out, f)
		}
	}
	return out, nil
}

// ---- opencode ----------------------------------------------------------

func parseOpenCode(raw []byte) ([]Found, error) {
	var doc struct {
		MCP map[string]json.RawMessage `json:"mcp"`
	}
	if err := json.Unmarshal(stripJSONC(raw), &doc); err != nil {
		return nil, err
	}
	names := make([]string, 0, len(doc.MCP))
	for n := range doc.MCP {
		names = append(names, n)
	}
	sort.Strings(names)
	var out []Found
	for _, name := range names {
		var s struct {
			Type        string            `json:"type"`
			Command     []string          `json:"command"`
			Environment map[string]string `json:"environment"`
			URL         string            `json:"url"`
			Headers     map[string]string `json:"headers"`
			Enabled     *bool             `json:"enabled"`
			OAuth       json.RawMessage   `json:"oauth"`
		}
		if err := json.Unmarshal(doc.MCP[name], &s); err != nil {
			continue
		}
		cfg := config.MCPServerCfg{}
		var notes []string
		if s.Type == "remote" || (s.Type == "" && s.URL != "") {
			cfg.URL, cfg.Headers = s.URL, s.Headers
			var oa struct {
				ClientID     string `json:"clientId"`
				ClientSecret string `json:"clientSecret"`
				Scope        string `json:"scope"`
			}
			if len(s.OAuth) > 0 && json.Unmarshal(s.OAuth, &oa) == nil && oa.ClientID != "" {
				cfg.OAuth = &config.MCPOAuthCfg{ClientID: oa.ClientID, Scopes: strings.Fields(oa.Scope)}
				if oa.ClientSecret != "" {
					notes = append(notes, "the OAuth client secret is not imported; set oauth.client_secret yourself")
				}
			}
		} else if len(s.Command) > 0 {
			cfg.Command, cfg.Args, cfg.Env = s.Command[0], s.Command[1:], s.Environment
		}
		if f, ok := mk(name, cfg, s.Enabled != nil && !*s.Enabled, notes); ok {
			out = append(out, f)
		}
	}
	return out, nil
}

var trailingComma = regexp.MustCompile(`,(\s*[}\]])`)

// stripJSONC treu comentaris (// i /* */) i comes finals d'un JSONC, sense
// tocar el que hi ha dins de cadenes.
func stripJSONC(src []byte) []byte {
	var out []byte
	inStr, esc := false, false
	for i := 0; i < len(src); i++ {
		c := src[i]
		if inStr {
			out = append(out, c)
			switch {
			case esc:
				esc = false
			case c == '\\':
				esc = true
			case c == '"':
				inStr = false
			}
			continue
		}
		switch {
		case c == '"':
			inStr = true
			out = append(out, c)
		case c == '/' && i+1 < len(src) && src[i+1] == '/':
			for i < len(src) && src[i] != '\n' {
				i++
			}
			out = append(out, '\n')
		case c == '/' && i+1 < len(src) && src[i+1] == '*':
			i += 2
			for i+1 < len(src) && !(src[i] == '*' && src[i+1] == '/') {
				i++
			}
			i++
		default:
			out = append(out, c)
		}
	}
	return trailingComma.ReplaceAll(out, []byte("$1"))
}

// ---- comú --------------------------------------------------------------

func mk(name string, cfg config.MCPServerCfg, disabled bool, notes []string) (Found, bool) {
	clean := CleanName(name)
	if clean == "" || config.ValidaMCP(clean, cfg) != nil {
		return Found{}, false
	}
	if clean != name {
		notes = append(notes, fmt.Sprintf("renamed from %q", name))
	}
	return Found{Name: clean, Cfg: cfg, Disabled: disabled, Notes: notes}, true
}

// ---- pla ---------------------------------------------------------------

// Plan és què passaria si s'importés: què s'afegeix, què ja hi és, i què
// no pot entrar (nom ocupat per una altra definició, desactivat, repetit).
type Plan struct {
	Add       []Found
	Present   []Found // ja existeix igual
	Conflicts []Found // el nom existeix amb una altra definició
	Disabled  []Found // desactivat a l'origen
	Duplicate []Found // el mateix nom trobat abans en un altre fitxer
}

// MakePlan classifica el que s'ha trobat respecte del que Gregal ja té.
func MakePlan(existing map[string]config.MCPServerCfg, found []Found) Plan {
	var p Plan
	seen := map[string]bool{}
	for _, f := range found {
		if seen[f.Name] {
			p.Duplicate = append(p.Duplicate, f)
			continue
		}
		seen[f.Name] = true
		if cur, ok := existing[f.Name]; ok {
			if sameServer(cur, f.Cfg) {
				p.Present = append(p.Present, f)
			} else {
				p.Conflicts = append(p.Conflicts, f)
			}
			continue
		}
		if f.Disabled {
			p.Disabled = append(p.Disabled, f)
			continue
		}
		p.Add = append(p.Add, f)
	}
	return p
}

func sameServer(a, b config.MCPServerCfg) bool {
	if a.URL != "" || b.URL != "" {
		return a.URL == b.URL
	}
	return a.Command == b.Command && reflect.DeepEqual(nz(a.Args), nz(b.Args)) && reflect.DeepEqual(nzm(a.Env), nzm(b.Env))
}

func nz(s []string) []string {
	if len(s) == 0 {
		return nil
	}
	return s
}

func nzm(m map[string]string) map[string]string {
	if len(m) == 0 {
		return nil
	}
	return m
}

// Apply afegeix a cfg els servidors de plan.Add (només els de only, si
// se'n dóna cap) i torna els noms afegits. No desa: ho fa qui crida
// (Config.Save).
func Apply(cfg *config.Config, plan Plan, only []string) []string {
	pick := map[string]bool{}
	for _, n := range only {
		pick[n] = true
	}
	var added []string
	for _, f := range plan.Add {
		if len(pick) > 0 && !pick[f.Name] {
			continue
		}
		if cfg.MCP == nil {
			cfg.MCP = map[string]config.MCPServerCfg{}
		}
		cfg.MCP[f.Name] = f.Cfg
		added = append(added, f.Name)
	}
	return added
}

// ---- text segur --------------------------------------------------------

var secretName = regexp.MustCompile(`(?i)(token|secret|passw|pwd|key|auth|bearer|credential)`)

// Describe és la descripció d'una línia d'un servidor, amb els valors
// secrets emmascarats: és l'únic text que es mostra o es registra.
func Describe(f Found) string {
	c := f.Cfg
	var b strings.Builder
	if c.URL != "" {
		b.WriteString("url " + maskURL(c.URL))
		if c.Transport != "" {
			b.WriteString(" [" + c.Transport + "]")
		}
		if len(c.Headers) > 0 {
			b.WriteString("  headers: " + maskMap(c.Headers))
		}
		return b.String()
	}
	b.WriteString(c.Command)
	for _, a := range maskArgs(c.Args) {
		b.WriteString(" " + a)
	}
	if len(c.Env) > 0 {
		b.WriteString("  env: " + maskMap(c.Env))
	}
	return b.String()
}

// maskMap mostra les claus i amaga els valors, llevat de les referències
// ${VAR} (que no són cap secret).
func maskMap(m map[string]string) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		if v := m[k]; strings.Contains(v, "${") {
			parts = append(parts, k+"="+v)
		} else {
			parts = append(parts, k+"=***")
		}
	}
	return strings.Join(parts, ", ")
}

func maskArgs(args []string) []string {
	out := make([]string, len(args))
	hide := false
	for i, a := range args {
		switch {
		case hide:
			out[i], hide = "***", false
		case strings.HasPrefix(a, "-") && !strings.Contains(a, "=") && secretName.MatchString(a):
			out[i], hide = a, true
		case strings.Contains(a, "=") && !strings.Contains(a, "://") && secretName.MatchString(a[:strings.Index(a, "=")]):
			out[i] = a[:strings.Index(a, "=")] + "=***"
		case strings.Contains(a, "://"):
			out[i] = maskURL(a)
		default:
			out[i] = a
		}
	}
	return out
}

func maskURL(s string) string {
	u, err := url.Parse(s)
	if err != nil {
		return "***"
	}
	if u.User != nil {
		u.User = url.User("***")
	}
	q := u.Query()
	hidden := false
	for k := range q {
		if secretName.MatchString(k) {
			q.Set(k, "XXX")
			hidden = true
		}
	}
	if hidden {
		u.RawQuery = q.Encode()
	}
	return strings.ReplaceAll(strings.ReplaceAll(u.String(), "XXX", "***"), "%2A%2A%2A", "***")
}

// Report és la vista prèvia (sense secrets).
func Report(res Result, plan Plan) string {
	var b strings.Builder
	if len(res.Scanned) == 0 && len(res.Errors) == 0 {
		b.WriteString("No Claude Code, Codex or opencode MCP configuration found.\n")
	}
	for _, f := range res.Scanned {
		b.WriteString("read: " + f + "\n")
	}
	for _, e := range res.Errors {
		b.WriteString("could not read: " + e + "\n")
	}
	section := func(title string, list []Found) {
		if len(list) == 0 {
			return
		}
		b.WriteString("\n" + title + "\n")
		for _, f := range list {
			fmt.Fprintf(&b, "  %-18s %s  (%s)\n", f.Name, Describe(f), f.Origin)
			for _, n := range f.Notes {
				b.WriteString("      note: " + n + "\n")
			}
		}
	}
	section("Would add:", plan.Add)
	section("Already in Gregal (same definition):", plan.Present)
	section("Skipped, the name is taken by a different server (remove it first to import):", plan.Conflicts)
	section("Skipped, disabled in the source:", plan.Disabled)
	section("Skipped, the same name was found earlier in another file:", plan.Duplicate)
	return b.String()
}
