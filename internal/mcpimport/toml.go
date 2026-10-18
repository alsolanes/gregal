package mcpimport

import (
	"fmt"
	"strconv"
	"strings"
)

// Un lector de TOML mínim, només per a ~/.codex/config.toml: taules
// ([a.b."c d"]), claus amb cometes, cadenes (bàsiques i literals), booleans,
// enters, llistes (també en diverses línies) i taules en línia. No és un
// parser complet (dates, cadenes multilínia, taules de taules: tot allò que
// la secció mcp_servers no fa servir queda ignorat), i prefereix saltar-se
// una línia estranya que fallar: l'import ha de poder llegir un fitxer de
// Codex que té moltes més coses.

type tomlTable map[string]any

// parseTOML retorna l'arbre del document.
func parseTOML(src string) tomlTable {
	root := tomlTable{}
	cur := root
	lines := strings.Split(strings.ReplaceAll(src, "\r\n", "\n"), "\n")
	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(stripComment(lines[i]))
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "[[") { // taula d'arrays: no ens interessa
			cur = tomlTable{}
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			path, ok := splitKey(line[1 : len(line)-1])
			if !ok {
				cur = tomlTable{}
				continue
			}
			cur = descend(root, path)
			continue
		}
		eq := indexOutsideQuotes(line, '=')
		if eq < 0 {
			continue
		}
		path, ok := splitKey(strings.TrimSpace(line[:eq]))
		if !ok {
			continue
		}
		val := strings.TrimSpace(line[eq+1:])
		// Llistes i taules en línia poden continuar a les línies següents.
		for unbalanced(val) && i+1 < len(lines) {
			i++
			val += "\n" + stripComment(lines[i])
		}
		v, ok := parseValue(val)
		if !ok {
			continue
		}
		t := cur
		if len(path) > 1 {
			t = descend(cur, path[:len(path)-1])
		}
		t[path[len(path)-1]] = v
	}
	return root
}

func descend(t tomlTable, path []string) tomlTable {
	for _, k := range path {
		next, ok := t[k].(tomlTable)
		if !ok {
			next = tomlTable{}
			t[k] = next
		}
		t = next
	}
	return t
}

// splitKey parteix `a.b."c.d"` en parts.
func splitKey(s string) ([]string, bool) {
	var parts []string
	var b strings.Builder
	var quote rune
	for _, r := range s {
		switch {
		case quote != 0:
			if r == quote {
				quote = 0
			} else {
				b.WriteRune(r)
			}
		case r == '"' || r == '\'':
			quote = r
		case r == '.':
			parts = append(parts, strings.TrimSpace(b.String()))
			b.Reset()
		default:
			b.WriteRune(r)
		}
	}
	if quote != 0 {
		return nil, false
	}
	parts = append(parts, strings.TrimSpace(b.String()))
	for _, p := range parts {
		if p == "" {
			return nil, false
		}
	}
	return parts, true
}

func stripComment(s string) string {
	var quote rune
	esc := false
	for i, r := range s {
		switch {
		case esc:
			esc = false
		case quote == '"' && r == '\\':
			esc = true
		case quote != 0:
			if r == quote {
				quote = 0
			}
		case r == '"' || r == '\'':
			quote = r
		case r == '#':
			return s[:i]
		}
	}
	return s
}

func indexOutsideQuotes(s string, c rune) int {
	var quote rune
	for i, r := range s {
		switch {
		case quote != 0:
			if r == quote {
				quote = 0
			}
		case r == '"' || r == '\'':
			quote = r
		case r == c:
			return i
		}
	}
	return -1
}

func unbalanced(s string) bool {
	depth := 0
	var quote rune
	esc := false
	for _, r := range s {
		switch {
		case esc:
			esc = false
		case quote == '"' && r == '\\':
			esc = true
		case quote != 0:
			if r == quote {
				quote = 0
			}
		case r == '"' || r == '\'':
			quote = r
		case r == '[' || r == '{':
			depth++
		case r == ']' || r == '}':
			depth--
		}
	}
	return depth > 0
}

func parseValue(s string) (any, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, false
	}
	switch {
	case s[0] == '"':
		v, rest, ok := readBasicString(s)
		return v, ok && strings.TrimSpace(rest) == ""
	case s[0] == '\'':
		end := strings.IndexByte(s[1:], '\'')
		if end < 0 {
			return nil, false
		}
		return s[1 : 1+end], true
	case s[0] == '[':
		if s[len(s)-1] != ']' {
			return nil, false
		}
		items := splitTop(s[1 : len(s)-1])
		out := []any{}
		for _, it := range items {
			if v, ok := parseValue(it); ok {
				out = append(out, v)
			}
		}
		return out, true
	case s[0] == '{':
		if s[len(s)-1] != '}' {
			return nil, false
		}
		t := tomlTable{}
		for _, it := range splitTop(s[1 : len(s)-1]) {
			eq := indexOutsideQuotes(it, '=')
			if eq < 0 {
				continue
			}
			path, ok := splitKey(strings.TrimSpace(it[:eq]))
			if !ok {
				continue
			}
			v, ok := parseValue(it[eq+1:])
			if !ok {
				continue
			}
			descend(t, path[:len(path)-1])[path[len(path)-1]] = v
		}
		return t, true
	case s == "true":
		return true, true
	case s == "false":
		return false, true
	}
	if n, err := strconv.ParseInt(strings.ReplaceAll(s, "_", ""), 10, 64); err == nil {
		return n, true
	}
	if f, err := strconv.ParseFloat(strings.ReplaceAll(s, "_", ""), 64); err == nil {
		return f, true
	}
	return nil, false
}

func readBasicString(s string) (val, rest string, ok bool) {
	var b strings.Builder
	for i := 1; i < len(s); i++ {
		switch c := s[i]; c {
		case '"':
			return b.String(), s[i+1:], true
		case '\\':
			if i+1 >= len(s) {
				return "", "", false
			}
			i++
			switch s[i] {
			case 'n':
				b.WriteByte('\n')
			case 't':
				b.WriteByte('\t')
			case 'r':
				b.WriteByte('\r')
			case '"', '\\':
				b.WriteByte(s[i])
			case 'u', 'U':
				n := 4
				if s[i] == 'U' {
					n = 8
				}
				if i+n >= len(s) {
					return "", "", false
				}
				cp, err := strconv.ParseInt(s[i+1:i+1+n], 16, 32)
				if err != nil {
					return "", "", false
				}
				b.WriteRune(rune(cp))
				i += n
			default:
				b.WriteByte('\\')
				b.WriteByte(s[i])
			}
		default:
			b.WriteByte(c)
		}
	}
	return "", "", false
}

// splitTop parteix el cos d'una llista o d'una taula en línia per comes de
// primer nivell.
func splitTop(s string) []string {
	var out []string
	depth := 0
	var quote rune
	esc := false
	start := 0
	for i, r := range s {
		switch {
		case esc:
			esc = false
		case quote == '"' && r == '\\':
			esc = true
		case quote != 0:
			if r == quote {
				quote = 0
			}
		case r == '"' || r == '\'':
			quote = r
		case r == '[' || r == '{':
			depth++
		case r == ']' || r == '}':
			depth--
		case r == ',' && depth == 0:
			out = append(out, strings.TrimSpace(s[start:i]))
			start = i + 1
		}
	}
	out = append(out, strings.TrimSpace(s[start:]))
	clean := out[:0]
	for _, o := range out {
		if o != "" {
			clean = append(clean, o)
		}
	}
	return clean
}

func tomlStrings(v any) []string {
	var out []string
	if l, ok := v.([]any); ok {
		for _, x := range l {
			out = append(out, fmt.Sprint(x))
		}
	}
	return out
}

func tomlStringMap(v any) map[string]string {
	t, ok := v.(tomlTable)
	if !ok || len(t) == 0 {
		return nil
	}
	out := make(map[string]string, len(t))
	for k, x := range t {
		out[k] = fmt.Sprint(x)
	}
	return out
}
