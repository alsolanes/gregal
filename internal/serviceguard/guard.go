// Package serviceguard prevents agent shell commands from stopping Gregal.
// It is a command guard, not a sandbox for arbitrary scripts or interpreters.
package serviceguard

import (
	"fmt"
	"os"
	"path"
	"strings"

	"mvdan.cc/sh/v3/expand"
	"mvdan.cc/sh/v3/syntax"
)

// Check rejects destructive systemctl actions on the runtime's own service
// and the Gregal service namespace (fallback/sibling frontend protection).
// Call it before permission overrides and again before spawning a shell.
func Check(command string) error { return (scope{ownUnit: OwnUnit()}).check(command, 0) }

// OwnUnit returns the innermost actual systemd service of this process, or
// empty when unavailable. Never trusts a shell/caller-supplied unit name.
func OwnUnit() string {
	raw, _ := os.ReadFile("/proc/self/cgroup")
	return scopeFromCgroup(string(raw)).ownUnit
}

// CheckRestart is for the trusted updater, not agent shell permissions: an
// external updater may restart any Gregal service except its own runtime unit.
// No Gregal namespace fallback applies when runtime membership is unavailable.
func CheckRestart(unit string) error {
	own := OwnUnit()
	if own != "" && strings.TrimSuffix(unit, ".service") == strings.TrimSuffix(own, ".service") {
		return fmt.Errorf("self-stop protection: cannot restart own runtime unit %q from inside it; use an external updater", own)
	}
	return nil
}

type scope struct{ ownUnit string }

// Read actual process membership, not shell assignments or caller-supplied
// environment variables. cgroup v1 and v2 both end in a slash-delimited path.
func scopeFromCgroup(cgroup string) scope {
	var s scope
	for _, line := range strings.Split(cgroup, "\n") {
		fields := strings.SplitN(line, ":", 3)
		if len(fields) != 3 {
			continue
		}
		for _, component := range strings.Split(fields[2], "/") {
			if strings.HasSuffix(component, ".service") && !strings.HasPrefix(component, "user@") {
				s.ownUnit = component
			}
		}
	}
	return s
}

func (s scope) check(command string, depth int) error {
	if depth > 16 {
		return fmt.Errorf("self-stop protection: shell wrapper nesting exceeds inspection limit")
	}
	file, err := syntax.NewParser().Parse(strings.NewReader(command), "")
	if err != nil {
		return nil // Ordinary policy/shell syntax validation remain unchanged.
	}
	var blocked error
	syntax.Walk(file, func(node syntax.Node) bool {
		if blocked != nil {
			return false
		}
		if call, ok := node.(*syntax.CallExpr); ok {
			blocked = s.checkCall(call.Args, depth)
		}
		return blocked == nil
	})
	return blocked
}

// literal decodes only static shell words. Never executes substitutions or
// consults the environment; Walk separately inspects substitution commands.
func literal(word *syntax.Word) (string, bool) { return literalParts(word.Parts, false) }

func literalParts(parts []syntax.WordPart, quoted bool) (string, bool) {
	var result strings.Builder
	for _, part := range parts {
		switch p := part.(type) {
		case *syntax.Lit:
			for i := 0; i < len(p.Value); i++ {
				c := p.Value[i]
				if c == '\\' && i+1 < len(p.Value) {
					next := p.Value[i+1]
					if !quoted || strings.ContainsRune("$`\"\\\n", rune(next)) {
						i++
						if next != '\n' {
							result.WriteByte(next)
						}
						continue
					}
				}
				result.WriteByte(c)
			}
		case *syntax.SglQuoted:
			if p.Dollar {
				value, err := expand.Literal(nil, &syntax.Word{Parts: []syntax.WordPart{p}})
				if err != nil {
					return "", false
				}
				result.WriteString(value)
				continue
			}
			result.WriteString(p.Value)
		case *syntax.DblQuoted:
			value, ok := literalParts(p.Parts, true)
			if !ok || p.Dollar {
				return "", false
			}
			result.WriteString(value)
		default:
			return "", false
		}
	}
	return result.String(), true
}

func (s scope) checkCall(args []*syntax.Word, depth int) error {
	if len(args) == 0 {
		return nil
	}
	executable, ok := literal(args[0])
	if !ok {
		return nil // Computed executable names require an actual sandbox.
	}
	name := path.Base(executable)
	args = args[1:]
	switch name {
	case "systemctl":
		return s.checkSystemctl(args)
	case "sh", "bash", "dash", "zsh", "ksh":
		for i, arg := range args {
			word, _ := literal(arg)
			if strings.HasPrefix(word, "-") && !strings.HasPrefix(word, "--") && strings.Contains(word, "c") && i+1 < len(args) {
				if script, ok := literal(args[i+1]); ok {
					return s.check(script, depth+1)
				}
				break
			}
		}
	case "eval":
		var words []string
		for _, arg := range args {
			word, ok := literal(arg)
			if !ok {
				return nil
			}
			words = append(words, word)
		}
		return s.check(strings.Join(words, " "), depth+1)
	case "env", "command", "exec", "timeout", "nice", "nohup", "setsid", "stdbuf", "xargs", "sudo", "doas", "pkexec", "systemd-run":
		i := 0
		for i < len(args) {
			word, ok := literal(args[i])
			if !ok {
				return nil
			}
			if word == "--" {
				i++
				break
			}
			if name == "env" && (word == "-S" || word == "--split-string" || strings.HasPrefix(word, "--split-string=")) {
				script := strings.TrimPrefix(word, "--split-string=")
				tailStart := i + 1
				if word == "-S" || word == "--split-string" {
					if i+1 >= len(args) {
						return nil
					}
					var ok bool
					script, ok = literal(args[i+1])
					if !ok {
						return nil
					}
					tailStart = i + 2
				}
				// env appends remaining argv after splitting -S. Preserve their
				// original shell syntax rather than dropping the actual target.
				var combined strings.Builder
				combined.WriteString(script)
				for _, arg := range args[tailStart:] {
					combined.WriteByte(' ')
					if err := syntax.NewPrinter().Print(&combined, arg); err != nil {
						return fmt.Errorf("self-stop protection: cannot inspect env arguments: %w", err)
					}
				}
				return s.check(combined.String(), depth+1)
			}
			if name == "command" && (word == "-v" || word == "-V") {
				return nil // command lookup, not execution.
			}
			if strings.HasPrefix(word, "-") {
				i++
				if wrapperOptionValue(name, word) {
					i++
				}
				continue
			}
			if name == "env" && strings.Contains(word, "=") {
				i++
				continue
			}
			break
		}
		if name == "timeout" {
			i++ // duration precedes the executable.
		}
		if i < len(args) {
			return s.checkCall(args[i:], depth+1)
		}
	}
	return nil
}

func wrapperOptionValue(name, option string) bool {
	switch name {
	case "sudo":
		return option == "-u" || option == "--user" || option == "-g" || option == "--group" || option == "-h" || option == "--host" || option == "-D" || option == "--chdir" || option == "-r" || option == "-t" || option == "-p" || option == "-C"
	case "doas", "pkexec":
		return option == "-u" || option == "--user"
	case "systemd-run":
		return option == "--unit" || option == "--description" || option == "--property" || option == "-p" || option == "--working-directory" || option == "--uid" || option == "--gid" || option == "--setenv"
	case "env":
		return option == "-u" || option == "--unset" || option == "-C" || option == "--chdir"
	case "exec":
		return option == "-a"
	case "timeout":
		return option == "-s" || option == "--signal" || option == "-k" || option == "--kill-after"
	case "nice":
		return option == "-n" || option == "--adjustment"
	case "stdbuf":
		return option == "-i" || option == "-o" || option == "-e" || option == "--input" || option == "--output" || option == "--error"
	case "xargs":
		return option == "-n" || option == "-P" || option == "-I" || option == "-d" || option == "-s" || option == "--max-args" || option == "--max-procs" || option == "--replace" || option == "--delimiter"
	}
	return false
}

func systemctlOptionValue(option string) bool {
	switch option {
	case "--host", "-H", "--machine", "-M", "--root", "--image", "--signal", "-s", "--kill-whom", "--kill-who", "--job-mode", "--type", "-t", "--state", "--property", "-p", "--output", "-o", "--lines", "-n", "--preset-mode", "--legend":
		return true
	}
	return false
}

func (s scope) checkSystemctl(args []*syntax.Word) error {
	action := ""
	options := true
	for i := 0; i < len(args); i++ {
		word, static := literal(args[i])
		if static && options && word == "--" {
			options = false
			continue
		}
		if static && options && strings.HasPrefix(word, "-") {
			if systemctlOptionValue(word) {
				i++
			}
			continue
		}
		if action == "" {
			if !static {
				return fmt.Errorf("self-stop protection: cannot inspect computed systemctl action")
			}
			action = word
			if !destructive(action) {
				return nil
			}
			continue
		}
		if !static || s.protectedUnit(word) {
			return fmt.Errorf("self-stop protection: systemctl %s targets protected or computed unit %q; use an external operator", action, word)
		}
	}
	return nil
}

func destructive(action string) bool {
	switch action {
	case "stop", "restart", "try-restart", "condrestart", "reload-or-restart", "reload-or-try-restart", "force-reload", "disable", "kill", "mask":
		return true
	}
	return false
}

func (s scope) protectedUnit(unit string) bool {
	if s.ownUnit != "" {
		for _, own := range []string{s.ownUnit, strings.TrimSuffix(s.ownUnit, ".service")} {
			if match, _ := path.Match(unit, own); match {
				return true
			}
		}
	}
	if strings.ContainsAny(unit, "*?[") {
		// Conservatively protect selectors whose fixed prefix can overlap the
		// Gregal namespace; explicit unrelated selectors keep their policy.
		prefix := unit[:strings.IndexAny(unit, "*?[")]
		return strings.HasPrefix("gregal", prefix) || strings.HasPrefix(prefix, "gregal-") || strings.HasPrefix(prefix, "gregal@")
	}
	base := strings.TrimSuffix(unit, ".service")
	if strings.Contains(base, ".") { // Other unit types are not Gregal services.
		return false
	}
	return base == "gregal" || strings.HasPrefix(base, "gregal-") || strings.HasPrefix(base, "gregal@")
}
