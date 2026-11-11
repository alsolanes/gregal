package web

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// Les fixtures del contracte (docs/assistant-contract/fixtures) són el que
// fan servir els paquets d'Android i del servidor per construir pantalles i
// proves sense esperar la infraestructura. Aquest test garanteix que porten
// els camps imprescindibles, estats vàlids i cap secret.
func TestAssistantContractFixtures(t *testing.T) {
	dir := filepath.Join("..", "..", "docs", "assistant-contract", "fixtures")
	files, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil || len(files) == 0 {
		t.Fatalf("sense fixtures a %s: %v", dir, err)
	}
	required := map[string][]string{
		"encarrec":  {"id", "owner", "conversationId", "runId", "objective", "state", "allowedScope"},
		"decisio":   {"id", "owner", "runId", "proposedEffect", "revision", "state"},
		"connexio":  {"id", "owner", "method", "accountLabel", "status", "capabilities", "authRef"},
		"ordinador": {"id", "owner", "health", "capabilities", "controlOwner"},
		"missatge":  {"id", "conversationId", "source", "text", "createdAt"},
	}
	states := map[string]struct {
		field string
		valid []string
	}{
		"encarrec":  {"state", encarrecStates},
		"decisio":   {"state", decisioStates},
		"connexio":  {"status", connexioStates},
		"ordinador": {"health", ordinadorHealth},
	}
	seen := map[string]bool{}
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		var obj map[string]any
		if err := json.Unmarshal(raw, &obj); err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		kind, _ := obj["kind"].(string)
		req, ok := required[kind]
		if !ok {
			t.Fatalf("%s: kind desconegut %q", f, kind)
		}
		seen[kind] = true
		for _, k := range req {
			if _, ok := obj[k]; !ok {
				t.Errorf("%s: falta %q", filepath.Base(f), k)
			}
		}
		if st, ok := states[kind]; ok {
			if v, _ := obj[st.field].(string); !slices.Contains(st.valid, v) {
				t.Errorf("%s: %s=%q no és cap de %v", filepath.Base(f), st.field, v, st.valid)
			}
		}
		if kind == "ordinador" {
			if v, _ := obj["controlOwner"].(string); !slices.Contains(ordinadorControl, v) {
				t.Errorf("%s: controlOwner=%q", filepath.Base(f), v)
			}
		}
		if kind == "connexio" {
			if ref, _ := obj["authRef"].(string); !strings.HasPrefix(ref, "auth:") {
				t.Errorf("%s: authRef ha de ser una referència opaca «auth:…», no %q", filepath.Base(f), ref)
			}
		}
		low := strings.ToLower(string(raw))
		for _, secret := range []string{"password", "token\"", "cookie", "access_token", "refresh_token"} {
			if strings.Contains(low, secret) {
				t.Errorf("%s: conté un camp de secret (%s)", filepath.Base(f), secret)
			}
		}
	}
	for kind := range required {
		if !seen[kind] {
			t.Errorf("falta una fixture de %q", kind)
		}
	}
}
