package llm

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
)

// Protocols natius.
//
// El client parla per defecte el protocol OpenAI-compatible de chat
// completions. Alguns proveïdors tenen una API pròpia més rica: Anthropic
// (Messages API: blocs tool_use/tool_result, blocs de pensament amb
// signatura, prompt caching amb cache_control) i OpenAI (Responses API:
// items de raonament que s'han de tornar). El tipus es declara al config
// (providers.<nom>.type) i es registra aquí per base_url, de manera que cap
// firma de Chat* canvia: el motor, el failover, els reintents i els avisos de
// progrés continuen igual i el client tria el cable segons l'endpoint.

// Tipus de proveïdor admesos a providers.<nom>.type.
const (
	// ProviderOpenAI és el protocol per defecte (chat completions).
	ProviderOpenAI = "openai"
	// ProviderAnthropic és l'API Messages d'Anthropic (/v1/messages).
	ProviderAnthropic = "anthropic"
	// ProviderOpenAIResponses és l'API Responses d'OpenAI (/v1/responses).
	ProviderOpenAIResponses = "openai-responses"
)

// NormalizeProviderType torna el tipus canònic ("" = chat completions) i
// ok=false si el valor no és cap dels coneguts.
func NormalizeProviderType(t string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(t)) {
	case "", ProviderOpenAI, "openai-compatible", "chat", "chat-completions":
		return "", true
	case ProviderAnthropic, "anthropic-messages", "messages":
		return ProviderAnthropic, true
	case ProviderOpenAIResponses, "responses", "openai_responses":
		return ProviderOpenAIResponses, true
	}
	return "", false
}

var providerTypes sync.Map // normBase(baseURL) → tipus canònic

// RegisterProviderType lliga un base_url a un protocol. Amb un tipus buit o
// desconegut l'endpoint torna al protocol per defecte. És global (no per
// Client) perquè hi ha fronts que creen el seu propi client (Telegram,
// servidor MCP) i tots han de parlar el mateix cable amb el mateix endpoint.
func RegisterProviderType(baseURL, typ string) {
	base := normBase(baseURL)
	if base == "" {
		return
	}
	t, _ := NormalizeProviderType(typ)
	if t == "" {
		providerTypes.Delete(base)
		return
	}
	providerTypes.Store(base, t)
}

// ProviderTypeFor torna el protocol registrat per a l'endpoint ("" = chat
// completions).
func ProviderTypeFor(baseURL string) string {
	if v, ok := providerTypes.Load(normBase(baseURL)); ok {
		return v.(string)
	}
	return ""
}

// AnthropicVersion és la versió d'API que s'envia a anthropic-version.
const AnthropicVersion = "2023-06-01"

// setModelsAuth posa la credencial adequada al protocol en un GET /models.
func setModelsAuth(req *http.Request, base, key string) {
	key = strings.TrimSpace(key)
	if ProviderTypeFor(base) == ProviderAnthropic {
		req.Header.Set("anthropic-version", AnthropicVersion)
		if key != "" {
			req.Header.Set("x-api-key", key)
		}
		return
	}
	if key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}
}

// Memòria de blocs natius d'anada i tornada.
//
// Els blocs de pensament d'Anthropic (amb la seva signatura) i els items de
// raonament xifrats de Responses s'han de tornar intactes dins de la mateixa
// cadena de crides d'eina. El Message del motor no els pot portar (és el
// format comú de tots els protocols i es desa a les sessions), així que es
// guarden aquí, al client, indexats per la primera crida d'eina de la
// resposta (o pel text, si la resposta no cridava eines). Quan el cable
// reconstrueix un missatge assistant, hi torna a posar els blocs.
const maxBlocsNatius = 512

type blocsNatius struct {
	mu    sync.Mutex
	m     map[string][]json.RawMessage
	ordre []string
}

// clauNativa identifica un missatge assistant: id de la primera crida o
// resum del text.
func clauNativa(calls []ToolCall, text string) string {
	if len(calls) > 0 && calls[0].ID != "" {
		return "call:" + calls[0].ID
	}
	if strings.TrimSpace(text) == "" {
		return ""
	}
	h := sha256.Sum256([]byte(text))
	return "txt:" + hex.EncodeToString(h[:12])
}

func (b *blocsNatius) desa(calls []ToolCall, text string, blocs []json.RawMessage) {
	k := clauNativa(calls, text)
	if k == "" || len(blocs) == 0 {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.m == nil {
		b.m = map[string][]json.RawMessage{}
	}
	if _, ok := b.m[k]; !ok {
		b.ordre = append(b.ordre, k)
	}
	b.m[k] = blocs
	for len(b.ordre) > maxBlocsNatius {
		delete(b.m, b.ordre[0])
		b.ordre = b.ordre[1:]
	}
}

func (b *blocsNatius) troba(calls []ToolCall, text string) ([]json.RawMessage, bool) {
	k := clauNativa(calls, text)
	if k == "" {
		return nil, false
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	v, ok := b.m[k]
	return v, ok
}

// validArgs torna els arguments d'una crida com a objecte JSON vàlid ("{}"
// si el model els ha deixat a mitges): els protocols natius els envien com a
// objecte i un JSON trencat faria fallar tota la petició.
func validArgs(raw string) json.RawMessage {
	raw = strings.TrimSpace(raw)
	if raw == "" || !json.Valid([]byte(raw)) || !strings.HasPrefix(raw, "{") {
		return json.RawMessage("{}")
	}
	return json.RawMessage(raw)
}

// splitDataURL separa un data URL en media type i dades base64.
func splitDataURL(u string) (mediaType, data string, ok bool) {
	if !strings.HasPrefix(u, "data:") {
		return "", "", false
	}
	i := strings.Index(u, ";base64,")
	if i < 0 {
		return "", "", false
	}
	return u[len("data:"):i], u[i+len(";base64,"):], true
}

// readSSE llegeix un stream d'events del servidor (event:/data:) i crida fn
// per cada event complet. Torna quan el cos s'acaba o fn diu prou (false).
func readSSE(body io.Reader, fn func(event, data string) bool) error {
	br := bufio.NewReader(body)
	var event string
	var data strings.Builder
	flush := func() bool {
		if data.Len() == 0 && event == "" {
			return true
		}
		cont := fn(event, data.String())
		event = ""
		data.Reset()
		return cont
	}
	for {
		line, err := br.ReadString('\n')
		t := strings.TrimRight(line, "\r\n")
		switch {
		case t == "":
			if line != "" || err != nil {
				if !flush() {
					return nil
				}
			}
		case strings.HasPrefix(t, ":"):
			// comentari / keep-alive
		case strings.HasPrefix(t, "event:"):
			event = strings.TrimSpace(strings.TrimPrefix(t, "event:"))
		case strings.HasPrefix(t, "data:"):
			if data.Len() > 0 {
				data.WriteByte('\n')
			}
			data.WriteString(strings.TrimPrefix(strings.TrimPrefix(t, "data:"), " "))
		}
		if err != nil {
			flush()
			return err
		}
	}
}
