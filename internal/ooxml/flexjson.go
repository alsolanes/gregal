package ooxml

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// Formes de JSON que els models fan servir de manera natural i que
// s'accepten igual (sobretot els models petits): una taula com a llista
// d'objectes, números com a text, cel·les numèriques.

// FlexTable és una taula: [["a","b"],[1,2]] o [{"a":1,"b":2}, …].
type FlexTable [][]string

// UnmarshalJSON accepta files de valors qualssevol o una llista d'objectes
// (les claus del primer objecte fan de capçalera).
func (t *FlexTable) UnmarshalJSON(b []byte) error {
	var raw []json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		return fmt.Errorf("table ha de ser una llista de files")
	}
	var out [][]string
	var header []string
	for _, r := range raw {
		var row []any
		if json.Unmarshal(r, &row) == nil {
			var cells []string
			for _, v := range row {
				cells = append(cells, cellText(v))
			}
			out = append(out, cells)
			continue
		}
		var obj map[string]any
		if err := json.Unmarshal(r, &obj); err != nil {
			return fmt.Errorf("fila de taula no vàlida: %s", string(r))
		}
		if header == nil {
			for k := range obj {
				header = append(header, k)
			}
			// L'ordre de les claus es perd al JSON de Go: es manté el del text.
			header = keyOrder(string(r), header)
			out = append(out, header)
		}
		var cells []string
		for _, k := range header {
			cells = append(cells, cellText(obj[k]))
		}
		out = append(out, cells)
	}
	*t = out
	return nil
}

func cellText(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(x)
	default:
		b, _ := json.Marshal(x)
		return string(b)
	}
}

// keyOrder ordena les claus segons on apareixen al JSON original.
func keyOrder(src string, keys []string) []string {
	sort.SliceStable(keys, func(i, j int) bool {
		return strings.Index(src, `"`+keys[i]+`"`) < strings.Index(src, `"`+keys[j]+`"`)
	})
	return keys
}

// FlexNumbers és una llista de números que també accepta text («14», «9,5»).
type FlexNumbers []float64

// UnmarshalJSON accepta números o text numèric.
func (n *FlexNumbers) UnmarshalJSON(b []byte) error {
	var raw []any
	if err := json.Unmarshal(b, &raw); err != nil {
		return fmt.Errorf("values ha de ser una llista de números")
	}
	out := make([]float64, 0, len(raw))
	for _, v := range raw {
		switch x := v.(type) {
		case float64:
			out = append(out, x)
		case string:
			s := strings.TrimSpace(strings.NewReplacer("€", "", "$", "", "%", "", " ", "").Replace(x))
			if f, err := strconv.ParseFloat(s, 64); err == nil {
				out = append(out, f)
			} else if f, err := strconv.ParseFloat(strings.Replace(strings.ReplaceAll(s, ".", ""), ",", ".", 1), 64); err == nil {
				out = append(out, f)
			} else {
				return fmt.Errorf("valor no numèric: %q", x)
			}
		default:
			return fmt.Errorf("valor no numèric: %v", x)
		}
	}
	*n = out
	return nil
}
