package llm

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// DeepSeek (i algun proxy que hi passa per sobre) té un format natiu de
// crida d'eina, DSML: <｜DSML｜calls><｜DSML｜invoke name="glob"><｜DSML｜parameter
// name="pattern" string="true">**/*</｜DSML｜parameter></｜DSML｜invoke></｜DSML｜calls>.
// Quan la petició porta `tools` el model el tradueix a tool_calls; quan no en
// porta —o el proxy no ho tradueix— el markup arriba dins de content i, sense
// aquesta capa, la UI l'imprimia tal qual a l'usuari. La barra és U+FF5C i
// segons la ruta arriba simple o doblada.

var (
	dsmlBlock  = regexp.MustCompile(`(?s)<｜+DSML｜+\s*calls\s*>.*?</｜+DSML｜+\s*calls\s*>`)
	dsmlInvoke = regexp.MustCompile(`(?s)<｜+DSML｜+\s*invoke\s+name="([^"]+)"\s*>(.*?)</｜+DSML｜+\s*invoke\s*>`)
	dsmlParam  = regexp.MustCompile(`(?s)<｜+DSML｜+\s*parameter\s+name="([^"]+)"([^>]*)>(.*?)</｜+DSML｜+\s*parameter\s*>`)
	dsmlAny    = regexp.MustCompile(`<｜+DSML｜+`)
)

// hasDSML diu si content porta markup DSML.
func hasDSML(content string) bool { return dsmlAny.MatchString(content) }

// parseDSML separa el text visible de les crides d'eina codificades en DSML.
// Les crides surten com a ToolCall normals (ids "dsml-1", "dsml-2"…) i el
// text torna sense cap rastre del markup. Si no hi ha DSML, torna el text tal
// qual i cap crida.
func parseDSML(content string) (string, []ToolCall) {
	if !hasDSML(content) {
		return content, nil
	}
	var calls []ToolCall
	for _, inv := range dsmlInvoke.FindAllStringSubmatch(content, -1) {
		args := map[string]any{}
		for _, p := range dsmlParam.FindAllStringSubmatch(inv[2], -1) {
			name, attrs, val := p[1], p[2], strings.TrimSpace(p[3])
			// string="true" vol dir literal; sense, DeepSeek hi posa JSON
			// (números, booleans, llistes). Si no parseja, queda com a text.
			if strings.Contains(attrs, `string="true"`) {
				args[name] = val
				continue
			}
			var v any
			if err := json.Unmarshal([]byte(val), &v); err == nil {
				args[name] = v
			} else {
				args[name] = val
			}
		}
		raw, _ := json.Marshal(args)
		tc := ToolCall{ID: fmt.Sprintf("dsml-%d", len(calls)+1), Type: "function"}
		tc.Function.Name = inv[1]
		tc.Function.Arguments = string(raw)
		calls = append(calls, tc)
	}
	clean := dsmlBlock.ReplaceAllString(content, "")
	// Invokes fora d'un bloc calls (o blocs mal tancats): fora igualment.
	clean = dsmlInvoke.ReplaceAllString(clean, "")
	clean = dsmlAny.ReplaceAllString(clean, "")
	return strings.TrimSpace(clean), calls
}

// dsmlLeakErr és l'error d'una via sense eines que rep una crida DSML: no es
// pot executar i no es pot ensenyar; s'explica.
func dsmlLeakErr(calls []ToolCall) error {
	names := make([]string, 0, len(calls))
	for _, c := range calls {
		names = append(names, c.Function.Name)
	}
	return fmt.Errorf("el model ha intentat cridar %s però aquesta via no porta eines: passa a un mode amb eines (code/consulta) o repeteix la pregunta", strings.Join(names, ", "))
}
