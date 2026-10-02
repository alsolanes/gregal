// Package usercmd carrega slash commands personalitzades (estil Claude Code):
// fitxers .md a ~/.config/gregal/commands (personals) i a
// <projecte>/.gregal/commands (de projecte; guanyen per nom).
// El cos és una plantilla: {{args}} (o $ARGUMENTS) s'expandeix amb els
// arguments. Són fitxers teus i de confiança, com els hooks.
package usercmd

import (
	"os"
	"path/filepath"
	"strings"
)

// Cmd és una ordre carregada.
type Cmd struct {
	Name   string // nom del fitxer sense .md
	Desc   string // primera línia útil (títol o text)
	Body   string // plantilla
	Source string // "personal" o "projecte"
}

// PersonalDir és el directori d'ordres personals.
func PersonalDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "gregal", "commands")
}

// ProjectDir és el directori d'ordres del projecte.
func ProjectDir(cwd string) string {
	if cwd == "" {
		return ""
	}
	return filepath.Join(cwd, ".gregal", "commands")
}

// List carrega totes les ordres (projecte guanya per nom).
func List(cwd string) []Cmd {
	seen := map[string]bool{}
	var out []Cmd
	for _, dir := range []struct {
		path, src string
	}{{ProjectDir(cwd), "projecte"}, {PersonalDir(), "personal"}} {
		ents, err := os.ReadDir(dir.path)
		if err != nil || dir.path == "" {
			continue
		}
		for _, e := range ents {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
				continue
			}
			name := strings.TrimSuffix(e.Name(), ".md")
			if !validName(name) || seen[name] {
				continue
			}
			seen[name] = true
			raw, err := os.ReadFile(filepath.Join(dir.path, e.Name()))
			if err != nil || strings.TrimSpace(string(raw)) == "" {
				continue
			}
			body := string(raw)
			out = append(out, Cmd{Name: name, Desc: DescOf(body), Body: body, Source: dir.src})
		}
	}
	return out
}

// Find busca una ordre per nom (projecte primer).
func Find(cwd, name string) (Cmd, bool) {
	for _, c := range List(cwd) {
		if c.Name == name {
			return c, true
		}
	}
	return Cmd{}, false
}

// Resolve mira si el text és una ordre personalitzada (/nom args…).
// Torna l'ordre, els args i cert si existeix.
func Resolve(cwd, text string) (Cmd, string, bool) {
	fields := strings.Fields(text)
	if len(fields) == 0 || !strings.HasPrefix(fields[0], "/") {
		return Cmd{}, "", false
	}
	name := strings.TrimPrefix(fields[0], "/")
	if !Valid(name) {
		return Cmd{}, "", false
	}
	c, ok := Find(cwd, name)
	if !ok {
		return Cmd{}, "", false
	}
	return c, strings.TrimSpace(strings.TrimPrefix(text, fields[0])), true
}

// Expand substitueix {{args}} (o $ARGUMENTS) pels arguments. Si el cos comença
// amb un títol "# …", es considera descripció i no entra al prompt.
func Expand(c Cmd, args string) string {
	body := c.Body
	if i := strings.Index(body, "\n"); i >= 0 {
		if strings.HasPrefix(strings.TrimSpace(body[:i]), "#") {
			body = body[i+1:]
		}
	} else if strings.HasPrefix(strings.TrimSpace(body), "#") {
		body = ""
	}
	out := strings.ReplaceAll(body, "{{args}}", args)
	out = strings.ReplaceAll(out, "$ARGUMENTS", args)
	return strings.TrimSpace(out)
}

// DescOf extreu la descripció: primer títol "# …" o primera línia útil.
func DescOf(body string) string {
	for _, ln := range strings.Split(body, "\n") {
		ln = strings.TrimSpace(ln)
		if ln == "" || strings.HasPrefix(ln, "<!--") {
			continue
		}
		ln = strings.TrimPrefix(ln, "#")
		ln = strings.TrimSpace(ln)
		if len(ln) > 80 {
			ln = ln[:77] + "…"
		}
		if ln != "" {
			return ln
		}
	}
	return "(sense descripció)"
}

func validName(s string) bool {
	if s == "" || len(s) > 40 {
		return false
	}
	for _, r := range s {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-' || r == '_' {
			continue
		}
		return false
	}
	return true
}

// Valid diu si un nom és apte per a ordre (/nom).
func Valid(name string) bool { return validName(name) }
