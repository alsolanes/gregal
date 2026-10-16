package llm

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// Peces comunes dels protocols natius (anthropic.go, responses.go).

// campNatiu és un camp opcional d'una petició nativa: si el proveïdor el
// rebutja amb un 400 que l'anomena (alguna de les claus), es treu, es torna
// a provar i, per a aquell endpoint i model, ja no s'envia més. És la
// mateixa idea que compat.go: un camp opcional (pensament, esforç, cache…)
// mai no pot trencar la crida, i models diferents del mateix proveïdor
// accepten coses diferents.
type campNatiu struct {
	nom   string
	claus []string
	// efimer: el rebuig només val per a aquesta petició (no es recorda).
	efimer bool
}

// peticioNativa construeix el cos JSON amb els camps de off apagats i diu
// quins camps opcionals hi ha posat.
type peticioNativa func(off map[string]bool) (body []byte, presents []string, err error)

// postNatiu envia la petició a baseURL+path amb reintents (postRetryWith) i
// negociació dels camps opcionals. Torna la resposta tal com arriba (2xx o
// un 4xx definitiu amb el cos intacte).
func (c *Client) postNatiu(ctx context.Context, baseURL, model, path string, malformed bool, camps []campNatiu, headers func(*http.Request), build peticioNativa) (*http.Response, error) {
	url := endpointURL(baseURL, path)
	clau := normBase(baseURL) + "#" + strings.TrimSpace(model)
	off := map[string]bool{}
	for _, cp := range camps {
		if c.campOff(clau, cp.nom) {
			off[cp.nom] = true
		}
	}
	for intent := 0; intent <= len(camps); intent++ {
		body, presents, err := build(off)
		if err != nil {
			return nil, err
		}
		resp, err := c.postRetryWith(ctx, baseURL, malformed, func() (*http.Response, error) {
			req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
			if err != nil {
				return nil, err
			}
			req.Header.Set("Content-Type", "application/json")
			headers(req)
			return c.hc.Do(req)
		})
		if err != nil || (resp.StatusCode != http.StatusBadRequest && resp.StatusCode != http.StatusUnprocessableEntity) {
			return resp, err
		}
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
		resp.Body.Close()
		resp.Body = io.NopCloser(bytes.NewReader(raw))
		rebutjats := campsNatiusRebutjats(string(raw), camps, presents)
		if len(rebutjats) == 0 || intent == len(camps) {
			return resp, nil
		}
		for _, nom := range rebutjats {
			off[nom] = true
			if !campEfimer(camps, nom) {
				c.setCampOff(clau, nom)
			}
		}
	}
	return nil, fmt.Errorf("provider %s: negociació de camps esgotada", baseURL)
}

func campEfimer(camps []campNatiu, nom string) bool {
	for _, cp := range camps {
		if cp.nom == nom {
			return cp.efimer
		}
	}
	return false
}

func campsNatiusRebutjats(body string, camps []campNatiu, presents []string) []string {
	b := strings.ToLower(body)
	hi := map[string]bool{}
	for _, p := range presents {
		hi[p] = true
	}
	var out []string
	for _, cp := range camps {
		if !hi[cp.nom] {
			continue
		}
		for _, k := range cp.claus {
			if strings.Contains(b, k) {
				out = append(out, cp.nom)
				break
			}
		}
	}
	return out
}

// errHTTP és el format d'error comú (IsRetryable i el TUI el reconeixen).
func errHTTP(baseURL string, resp *http.Response) error {
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4000))
	return fmt.Errorf("provider %s: HTTP %d: %s", baseURL, resp.StatusCode, truncate(string(raw), 2000))
}

// esStream diu si la resposta és SSE (alguns proxies ignoren stream:true).
func esStream(resp *http.Response) bool {
	return strings.Contains(resp.Header.Get("Content-Type"), "text/event-stream")
}

// thinkMode torna el mode de raonament del context ("" si no se n'ha dit res).
func thinkMode(ctx context.Context) string {
	v, _ := ctx.Value(thinkKey{}).(string)
	return strings.ToLower(strings.TrimSpace(v))
}

// nativeChat és el punt d'entrada comú dels protocols natius per a les
// quatre vies de Chat*. Torna ok=false si l'endpoint parla chat completions.
func (c *Client) nativeChat(ctx context.Context, baseURL, apiKey, model string, msgs []Message, temp float64, maxTokens int, specs []ToolSpec, stream bool, onToken, onReason func(string)) (string, []ToolCall, bool, error) {
	switch ProviderTypeFor(baseURL) {
	case ProviderAnthropic:
		text, calls, err := c.anthropicChat(ctx, baseURL, apiKey, model, msgs, maxTokens, specs, stream, onToken, onReason)
		return text, calls, true, err
	case ProviderOpenAIResponses:
		text, calls, err := c.responsesChat(ctx, baseURL, apiKey, model, msgs, temp, maxTokens, specs, stream, onToken, onReason)
		return text, calls, true, err
	case ProviderOllama:
		text, calls, err := c.ollamaChat(ctx, baseURL, apiKey, model, msgs, temp, maxTokens, specs, stream, onToken, onReason)
		if err == nil {
			// Models que Ollama no sap traduir a tool_calls (plantilla sense
			// eines): la crida ve dins del text (textcalls.go).
			text, calls = RecoverCalls(text, calls, specs)
		}
		return text, calls, true, err
	}
	return "", nil, false, nil
}
