package web

import (
	"context"
	"encoding/json"
	"net/http"
	"regexp"
	"sort"
	"strings"

	"gregal/internal/connexions"
	"gregal/internal/llm"
	"gregal/internal/tools"
)

// Formulari segur per a una contrasenya o una clau (com Grok): quan un vent
// necessita un secret, crida l'eina «secret» i al fil surt una targeta amb un
// camp amagat. El que la persona hi escriu va directe al magatzem xifrat
// (connexions.Vault): no passa pel fil, ni pel registre, ni pel model, que
// només rep la referència {{secret:id}}. En una ordre de bash, la referència
// es torna una variable d'entorn en executar-la, i si el valor surt a la
// sortida es tapa abans de tornar al model. Al navegador no s'escriu: les
// contrasenyes de les webs les escriu la persona (handoff, B3).

const secretRefPrefix = "secret:"

var secretIDUnsafe = regexp.MustCompile(`[^a-z0-9_]+`)

// secretID fa un identificador curt i estable a partir del que proposa el vent.
func secretID(raw string) string {
	s := strings.ToLower(strings.TrimSpace(raw))
	s = secretIDUnsafe.ReplaceAllString(s, "_")
	s = strings.Trim(s, "_")
	if len(s) > 40 {
		s = strings.Trim(s[:40], "_")
	}
	return s
}

var secretPlaceholder = regexp.MustCompile(`\{\{\s*secret:([A-Za-z0-9_ -]{1,60})\s*\}\}`)

// secretEnvName és la variable on arriba el valor: GREGAL_SECRET_<ID>.
func secretEnvName(id string) string { return "GREGAL_SECRET_" + strings.ToUpper(id) }

func windSecretSpec() llm.ToolSpec {
	return llm.ToolSpec{
		Name: "secret",
		Description: "Ask the person for a password, API key or token through a secure form in the chat. " +
			"The value goes straight to an encrypted store and you NEVER see it: you get back a placeholder {{secret:<id>}}. " +
			"Use the placeholder inside bash commands (at run time it becomes an environment variable and the value is masked in the output). " +
			"Never ask for passwords or keys in plain chat, never repeat them, and never type passwords into web pages (logins in the browser are done by the person). " +
			"If a secret with that id was already given, it is reused without asking again.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"id":   map[string]any{"type": "string", "description": "Short stable id, e.g. openweather_key"},
				"name": map[string]any{"type": "string", "description": "What it is, in the person's language, e.g. «Clau de l'API d'OpenWeather»"},
				"why":  map[string]any{"type": "string", "description": "One sentence: what you need it for"},
			},
			"required": []string{"id", "name"},
		},
	}
}

type secretArgs struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Why  string `json:"why"`
}

func parseSecretArgs(argsJSON string) (secretArgs, bool) {
	var a secretArgs
	if json.Unmarshal([]byte(argsJSON), &a) != nil {
		return a, false
	}
	a.ID = secretID(firstNonEmpty(a.ID, a.Name))
	a.Name = clipRuleRunes(strings.TrimSpace(a.Name), 80)
	a.Why = clipRuleRunes(strings.TrimSpace(a.Why), 200)
	if a.Name == "" {
		a.Name = a.ID
	}
	return a, a.ID != ""
}

func (s *Server) secretVault() *connexions.Vault { return connexions.DefaultVault() }

func secretUseText(id string) string {
	return "Saved securely. Use {{secret:" + id + "}} in bash commands where the value is needed (it becomes $" + secretEnvName(id) +
		" at run time). You do not know the value and must not ask for it in chat."
}

// windSecretCall demana el secret amb la targeta (o el reaprofita si ja hi és).
func (s *Server) windSecretCall(ctx context.Context, emit func(kind, text string), argsJSON string) (string, bool) {
	a, ok := parseSecretArgs(argsJSON)
	if !ok {
		return "Invalid secret request: give an id and a name.", false
	}
	if v, err := s.secretVault().Get(s.remindOwner(), secretRefPrefix+a.ID); err == nil && len(v) > 0 {
		return secretUseText(a.ID), false
	}
	req := &windAskReq{Type: "secret", Name: a.ID, Query: a.Name, Reason: a.Why}
	ans, out := s.windAskWait(ctx, emit, req, argsJSON)
	if out == askParked {
		return "", true
	}
	if out != askAnswered || !ans.Approve {
		return "The person did not give it. Do not ask for it in chat; say what you could not do without it and carry on with the rest.", false
	}
	return secretUseText(a.ID), false
}

// storeSecretAnswer desa el valor d'una targeta «secret» pendent d'aquesta
// persona. Torna fals si la decisió no és seva, no és un secret o ja no espera.
func (s *Server) storeSecretAnswer(key, value string) (bool, error) {
	var id string
	_ = s.withDecisions(func(st *decisionDir) error {
		cur, err := st.load(key)
		if err != nil || cur.Owner != s.remindOwner() || cur.Type != "secret" {
			return nil
		}
		st.sweep(cur) // una targeta caducada no desa res
		if cur.State != "pending" {
			return nil
		}
		id = cur.Name
		return nil
	})
	if id == "" {
		return false, nil
	}
	return true, s.secretVault().Put(s.remindOwner(), secretRefPrefix+id, []byte(value))
}

// withSecrets prepara una ordre de bash: cada {{secret:id}} es torna
// $GREGAL_SECRET_ID i el valor va al context com a variable d'entorn. Torna
// també els valors, per tapar-los a la sortida. Un secret que no existeix és
// un error clar per al model.
func (s *Server) withSecrets(ctx context.Context, name, argsJSON string) (context.Context, string, []string, error) {
	if (name != "bash" && name != "bash_background") || !strings.Contains(argsJSON, "{{") {
		return ctx, argsJSON, nil, nil
	}
	var args map[string]any
	if json.Unmarshal([]byte(argsJSON), &args) != nil {
		return ctx, argsJSON, nil, nil
	}
	cmd, _ := args["command"].(string)
	if !secretPlaceholder.MatchString(cmd) {
		return ctx, argsJSON, nil, nil
	}
	if name == "bash_background" {
		return ctx, argsJSON, nil, errSecretBackground
	}
	var env, values []string
	var missing []string
	cmd = secretPlaceholder.ReplaceAllStringFunc(cmd, func(m string) string {
		id := secretID(secretPlaceholder.FindStringSubmatch(m)[1])
		v, err := s.secretVault().Get(s.remindOwner(), secretRefPrefix+id)
		if err != nil || len(v) == 0 {
			missing = append(missing, id)
			return m
		}
		env = append(env, secretEnvName(id)+"="+string(v))
		values = append(values, string(v))
		return "$" + secretEnvName(id)
	})
	if len(missing) > 0 {
		return ctx, argsJSON, nil, &secretMissingError{missing}
	}
	args["command"] = cmd
	b, _ := json.Marshal(args)
	return tools.WithExtraEnv(ctx, env), string(b), values, nil
}

type secretMissingError struct{ ids []string }

func (e *secretMissingError) Error() string {
	return "no secret saved with id " + strings.Join(e.ids, ", ") + ": ask the person for it with the secret tool first"
}

type secretBackgroundError struct{}

func (secretBackgroundError) Error() string {
	return "secrets can only be used in bash (not bash_background)"
}

var errSecretBackground error = secretBackgroundError{}

// maskSecrets tapa els valors a la sortida (els llargs primer).
func maskSecrets(out string, values []string) string {
	sort.Slice(values, func(i, j int) bool { return len(values[i]) > len(values[j]) })
	for _, v := range values {
		if len(v) >= 4 {
			out = strings.ReplaceAll(out, v, "[secret]")
		}
	}
	return out
}

// Secrets desats (per a la pantalla de regles): només els noms i quan, mai el valor.
type savedSecret struct {
	ID string `json:"id"`
}

func (s *Server) handleWindSecrets(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		ids := s.secretVault().Refs(s.remindOwner(), secretRefPrefix)
		out := []savedSecret{}
		for _, id := range ids {
			out = append(out, savedSecret{ID: strings.TrimPrefix(id, secretRefPrefix)})
		}
		writeJSON(w, map[string]any{"secrets": out})
	case http.MethodDelete:
		id := secretID(r.URL.Query().Get("id"))
		if id == "" {
			http.Error(w, "id", http.StatusBadRequest)
			return
		}
		if err := s.secretVault().Delete(s.remindOwner(), secretRefPrefix+id); err != nil {
			http.Error(w, "could not delete", http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]any{"ok": true})
	default:
		http.Error(w, "GET or DELETE", http.StatusMethodNotAllowed)
	}
}

