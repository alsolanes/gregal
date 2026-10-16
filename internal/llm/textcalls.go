package llm

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Crides d'eina escrites com a text (models locals).
//
// Molts models oberts, quan la plantilla de xat del servidor no tradueix
// la seva sintaxi a tool_calls (llama.cpp sense --jinja, plantilles
// «Generic», proxies), escriuen la crida dins del contingut:
//
//	Hermes/Qwen2.5:  <tool_call>{"name": "read", "arguments": {"path": "a.go"}}</tool_call>
//	Qwen3-Coder:     <tool_call><function=read><parameter=path>a.go</parameter></function></tool_call>
//	JSON pelat:      {"name": "read", "arguments": {"path": "a.go"}}
//
// Abans es treien del text i es demanava al model que ho repetís, cosa que
// en local costa un torn sencer (un altre prefill). Ara es converteixen en
// ToolCall normals, igual que el DSML de DeepSeek: passen per la mateixa
// política, permisos i aprovacions que qualsevol crida estructurada. Només
// s'accepten noms d'eines que s'han ofert en aquesta petició.

var (
	hermesBlock = regexp.MustCompile(`(?s)<tool_call>\s*(.*?)\s*(?:</tool_call>|$)`)
	qwenFunc    = regexp.MustCompile(`(?s)<function\s*=\s*([\w.\-]+)\s*>(.*?)(?:</function>|$)`)
	qwenParam   = regexp.MustCompile(`(?s)<parameter\s*=\s*([\w.\-]+)\s*>\n?(.*?)\n?</parameter>`)
	jsonFence   = regexp.MustCompile("(?s)^\\s*```(?:json)?\\s*(\\{.*\\})\\s*```\\s*$")
)

// RecoverCalls aplica totes les recuperacions a una resposta: DSML, crides
// en text (si s'han ofert eines) i reparació d'arguments.
func RecoverCalls(text string, calls []ToolCall, specs []ToolSpec) (string, []ToolCall) {
	if len(calls) == 0 && hasDSML(text) {
		text, calls = parseDSML(text)
	}
	if len(calls) == 0 && len(specs) > 0 {
		text, calls = parseTextCalls(text, specs)
	}
	return text, RepairCalls(calls, specs)
}

func specByName(specs []ToolSpec) map[string]ToolSpec {
	m := make(map[string]ToolSpec, len(specs))
	for _, s := range specs {
		m[s.Name] = s
	}
	return m
}

// parseTextCalls extreu crides escrites en text. Torna el text net i les
// crides; si no n'hi ha cap de vàlida, el text queda intacte.
func parseTextCalls(text string, specs []ToolSpec) (string, []ToolCall) {
	known := specByName(specs)
	var calls []ToolCall
	add := func(name string, args map[string]any) {
		if _, ok := known[name]; !ok {
			return
		}
		raw, _ := json.Marshal(args)
		tc := ToolCall{ID: fmt.Sprintf("txt-%d", len(calls)+1), Type: "function"}
		tc.Function.Name = name
		tc.Function.Arguments = string(raw)
		calls = append(calls, tc)
	}
	clean := text
	if strings.Contains(text, "<tool_call>") || strings.Contains(text, "<function=") || strings.Contains(text, "<function =") {
		bodies := []string{}
		for _, m := range hermesBlock.FindAllStringSubmatch(text, -1) {
			bodies = append(bodies, m[1])
		}
		if len(bodies) == 0 {
			bodies = []string{text}
		}
		for _, body := range bodies {
			if fs := qwenFunc.FindAllStringSubmatch(body, -1); len(fs) > 0 {
				for _, f := range fs {
					args := map[string]any{}
					for _, p := range qwenParam.FindAllStringSubmatch(f[2], -1) {
						args[p[1]] = paramValue(p[2])
					}
					add(f[1], args)
				}
				continue
			}
			if name, args, ok := jsonCall(body); ok {
				add(name, args)
			}
		}
		if len(calls) > 0 {
			clean = hermesBlock.ReplaceAllString(clean, "")
			clean = qwenFunc.ReplaceAllString(clean, "")
		}
	} else if name, args, ok := jsonCall(text); ok {
		// JSON pelat: només si és TOT el missatge (no un exemple dins d'una
		// explicació).
		add(name, args)
		if len(calls) > 0 {
			clean = ""
		}
	}
	if len(calls) == 0 {
		return text, nil
	}
	return strings.TrimSpace(clean), calls
}

// paramValue: els paràmetres XML són text; objectes, llistes i booleans en
// JSON es descodifiquen. Els números es deixen com a text: RepairCalls els
// converteix si l'esquema diu que ho són (un «007» pot ser un codi).
func paramValue(s string) any {
	t := strings.TrimSpace(s)
	if strings.HasPrefix(t, "{") || strings.HasPrefix(t, "[") || t == "true" || t == "false" {
		var v any
		if json.Unmarshal([]byte(t), &v) == nil {
			return v
		}
	}
	return s
}

// jsonCall interpreta {"name": …, "arguments"|"parameters"|"input": {…}}.
func jsonCall(s string) (string, map[string]any, bool) {
	t := strings.TrimSpace(s)
	if m := jsonFence.FindStringSubmatch(t); m != nil {
		t = m[1]
	}
	if !strings.HasPrefix(t, "{") || !strings.HasSuffix(t, "}") {
		return "", nil, false
	}
	var obj map[string]any
	if err := json.Unmarshal([]byte(t), &obj); err != nil {
		fixed := fixJSON(t)
		if json.Unmarshal([]byte(fixed), &obj) != nil {
			return "", nil, false
		}
	}
	name, _ := obj["name"].(string)
	if name == "" {
		if fn, ok := obj["function"].(map[string]any); ok {
			name, _ = fn["name"].(string)
			obj = fn
		}
	}
	if name == "" {
		return "", nil, false
	}
	for _, k := range []string{"arguments", "parameters", "input", "args"} {
		switch a := obj[k].(type) {
		case map[string]any:
			return name, a, true
		case string:
			var m map[string]any
			if json.Unmarshal([]byte(a), &m) == nil {
				return name, m, true
			}
		}
	}
	return name, map[string]any{}, true
}

// ---------- reparació d'arguments ----------

var trailingComma = regexp.MustCompile(`,\s*([}\]])`)

// fixJSON arregla els errors típics dels models: blocs de codi, comes
// finals i text abans o després de l'objecte.
func fixJSON(s string) string {
	t := strings.TrimSpace(s)
	if m := jsonFence.FindStringSubmatch(t); m != nil {
		t = m[1]
	}
	if i := strings.Index(t, "{"); i > 0 {
		t = t[i:]
	}
	if j := strings.LastIndex(t, "}"); j >= 0 && j < len(t)-1 {
		t = t[:j+1]
	}
	return trailingComma.ReplaceAllString(t, "$1")
}

// RepairCalls corregeix arguments que el model ha codificat malament quan
// l'esquema deixa clara la intenció: JSON amb comes finals, objectes o
// llistes passats com a cadena, «"20"» per a un enter, «"false"» per a un
// booleà. Un argument que no es pot entendre es deixa tal qual (l'eina
// donarà un error útil).
func RepairCalls(calls []ToolCall, specs []ToolSpec) []ToolCall {
	if len(calls) == 0 {
		return calls
	}
	known := specByName(specs)
	for i := range calls {
		raw := strings.TrimSpace(calls[i].Function.Arguments)
		var args map[string]any
		if json.Unmarshal([]byte(raw), &args) != nil {
			fixed := fixJSON(raw)
			if json.Unmarshal([]byte(fixed), &args) != nil {
				continue
			}
		}
		spec, ok := known[calls[i].Function.Name]
		changed := false
		if ok {
			props, _ := spec.Parameters["properties"].(map[string]any)
			for k, v := range args {
				p, _ := props[k].(map[string]any)
				if p == nil {
					continue
				}
				if nv, ok := coerce(v, p["type"]); ok {
					args[k] = nv
					changed = true
				}
			}
		}
		if changed || !json.Valid([]byte(raw)) {
			b, _ := json.Marshal(args)
			calls[i].Function.Arguments = string(b)
		}
	}
	return calls
}

func coerce(v any, typ any) (any, bool) {
	t, _ := typ.(string)
	s, isStr := v.(string)
	switch t {
	case "integer", "number":
		if isStr {
			if n, err := strconv.ParseFloat(strings.TrimSpace(s), 64); err == nil {
				return n, true
			}
		}
	case "boolean":
		if isStr {
			switch strings.ToLower(strings.TrimSpace(s)) {
			case "true", "yes", "1":
				return true, true
			case "false", "no", "0":
				return false, true
			}
		}
	case "object", "array":
		if isStr {
			var x any
			if json.Unmarshal([]byte(strings.TrimSpace(s)), &x) == nil {
				switch x.(type) {
				case map[string]any:
					if t == "object" {
						return x, true
					}
				case []any:
					if t == "array" {
						return x, true
					}
				}
			}
		}
	case "string":
		switch x := v.(type) {
		case float64:
			return strconv.FormatFloat(x, 'f', -1, 64), true
		case bool:
			return strconv.FormatBool(x), true
		}
	}
	return nil, false
}
