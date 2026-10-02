package llm

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Message és un missatge de chat (system, user, assistant, tool).
// Images (URLs o data: URLs) només tenen sentit a user/tool: en el wire
// OpenAI-compat el content passa a ser array de parts (text + image_url).
type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content,omitempty"`
	Images     []string   `json:"-"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
	Name       string     `json:"-"`
}

// MarshalJSON manté el format string clàssic sense imatges (cap canvi
// de wire per als providers actuals) i emet parts amb imatges quan n'hi ha.
// En l'especificació d'OpenAI i en proveïdors com Console Go/Zen, el camp
// "name" és rebutjat amb HTTP 400 invalid_request_error si s'envia a
// missatges de tool o endpoints estrictes. Amb json:"-" Name no s'emet mai.
func (m Message) MarshalJSON() ([]byte, error) {
	return marshalMessage(m, false)
}

// marshalMessage construeix el missatge wire. sanitize només s'activa per a
// la petició al provider; la persistència de sessions ha de conservar els
// arguments originals encara que siguin invàlids.
func marshalMessage(m Message, sanitize bool) ([]byte, error) {
	type wire Message
	w := wire(m)
	if sanitize {
		w.ToolCalls = safeToolCalls(m.ToolCalls)
	}
	if len(m.Images) == 0 {
		return json.Marshal(w)
	}
	parts := make([]any, 0, len(m.Images)+1)
	if m.Content != "" {
		parts = append(parts, map[string]any{"type": "text", "text": m.Content})
	}
	for _, u := range m.Images {
		parts = append(parts, map[string]any{
			"type":      "image_url",
			"image_url": map[string]string{"url": u},
		})
	}
	type imgWire struct {
		Role       string     `json:"role"`
		Content    any        `json:"content"`
		ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
		ToolCallID string     `json:"tool_call_id,omitempty"`
	}
	iw := imgWire{
		Role: m.Role, Content: parts,
		ToolCalls: m.ToolCalls, ToolCallID: m.ToolCallID,
	}
	if sanitize {
		iw.ToolCalls = safeToolCalls(m.ToolCalls)
	}
	return json.Marshal(iw)
}

// safeToolCalls protegeix la conversa que es torna a enviar al provider.
// Alguns models retornen una cadena de arguments inacabada (sobretot quan
// conté un heredoc Python). L'execució local encara rep la cadena original i
// pot produir un error útil, però mai no enviem al provider un
// function.arguments que no sigui JSON vàlid: molts proxies responen 500 i
// deixen el torn atrapat en reintents.
func safeToolCalls(in []ToolCall) []ToolCall {
	if len(in) == 0 {
		return nil
	}
	out := append([]ToolCall(nil), in...)
	for i := range out {
		raw := strings.TrimSpace(out[i].Function.Arguments)
		if raw == "" || !json.Valid([]byte(raw)) || !strings.HasPrefix(raw, "{") {
			out[i].Function.Arguments = "{}"
		}
	}
	return out
}

func hasMalformedToolArguments(messages []Message) bool {
	for _, msg := range messages {
		for _, call := range msg.ToolCalls {
			raw := strings.TrimSpace(call.Function.Arguments)
			if raw == "" || !json.Valid([]byte(raw)) || !strings.HasPrefix(raw, "{") {
				return true
			}
		}
	}
	return false
}

// ToolCall és una crida d'eina del model (format OpenAI).
type ToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

// ToolSpec descriu una eina disponible (format OpenAI).
type ToolSpec struct {
	Name        string
	Description string
	Parameters  map[string]any
}

type chatRequest struct {
	Model                string    `json:"model"`
	Messages             []Message `json:"messages"`
	Temperature          float64   `json:"temperature,omitempty"`
	MaxTokens            int       `json:"max_tokens,omitempty"`
	Stream               bool      `json:"stream"`
	Tools                []any     `json:"tools,omitempty"`
	PromptCacheKey       string    `json:"prompt_cache_key,omitempty"`
	PromptCacheRetention string    `json:"prompt_cache_retention,omitempty"`
	EnableThinking       *bool     `json:"enable_thinking,omitempty"`
	// ChatTemplateKwargs va on el passador local espera l'ordre de pensar:
	// el prompt-injector escriu chat_template_kwargs.enable_thinking i amb
	// setdefault respecta el que porti el client. El camp del nivell de dalt
	// (enable_thinking) serveix per als endpoints directes.
	ChatTemplateKwargs map[string]any `json:"chat_template_kwargs,omitempty"`
	// StreamOptions només el posa postStream (vegeu usage.go).
	StreamOptions *streamOptions `json:"stream_options,omitempty"`
	// ReasoningEffort és l'ordre de no pensar de les API que segueixen
	// OpenAI (opencode zen): vegeu compat.go.
	ReasoningEffort string `json:"reasoning_effort,omitempty"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content          string     `json:"content"`
			ReasoningContent string     `json:"reasoning_content"`
			Reasoning        string     `json:"reasoning"`
			Thought          string     `json:"thought"`
			Thinking         string     `json:"thinking"`
			ToolCalls        []ToolCall `json:"tool_calls"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage Usage `json:"usage"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// Usage és el consum real que declaren els proveïdors (OpenAI-compatibles).
// No tots l'envien: quan hi és, val més que l'heurístic EstimateTokens.
type Usage struct {
	PromptTokens        int `json:"prompt_tokens"`
	CompletionTokens    int `json:"completion_tokens"`
	TotalTokens         int `json:"total_tokens"`
	PromptTokensDetails struct {
		CachedTokens int `json:"cached_tokens"`
	} `json:"prompt_tokens_details"`
}

// Buida diu si el proveïdor no ha declarat consum en aquesta resposta.
func (u Usage) Buida() bool {
	return u.PromptTokens == 0 && u.CompletionTokens == 0 && u.TotalTokens == 0
}

type streamEvent struct {
	Choices []struct {
		Delta struct {
			Content          string `json:"content"`
			ReasoningContent string `json:"reasoning_content"`
			Reasoning        string `json:"reasoning"`
			Thought          string `json:"thought"`
			Thinking         string `json:"thinking"`
			// ToolCalls arriba a trossos: el primer delta d'un índex porta id
			// i nom, els següents afegeixen bocins d'arguments. S'acumula per
			// Index, no per posició, que alguns proveïdors intercalen crides.
			ToolCalls []struct {
				Index    int    `json:"index"`
				ID       string `json:"id"`
				Type     string `json:"type"`
				Function struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"delta"`
		// FinishReason arriba a l'últim event. Sense mirar-lo, un torn que
		// s'ha quedat sense tokens enmig del raonament torna una resposta
		// buida i cap error: la interfície pinta un blanc i no saps per què.
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	// Alguns proveïdors envien el consum a l'últim event (choices buides).
	Usage Usage `json:"usage"`
}

// Client és un client HTTP minimalista OpenAI-compatible (/chat/completions).
// Serveix per llama.cpp, proxies OpenAI-compatibles i passarel·les cloud.
type Client struct {
	hc *http.Client

	sessionOnce sync.Once
	session     string

	// usageMu protegeix lastUsage: l'últim consum declarat pel proveïdor.
	// Cada crida el neteja i, si la resposta en porta, el desa: així
	// LastUsage no arrossega el consum d'una crida anterior quan el
	// proveïdor no en declara cap.
	usageMu   sync.Mutex
	lastUsage Usage
	// campsOff: «endpoint|camp» dels camps opcionals que un proveïdor ha
	// rebutjat (compat.go). També sota usageMu.
	campsOff map[string]bool
	cacheMu  sync.RWMutex
	cache    map[string]promptCacheConfig
}

type promptCacheConfig struct {
	key       string
	retention string
}

// LastUsage torna el consum real de l'última crida (ok=false si el
// proveïdor no el declara). Serveix per calibrar l'estimació heurística.
func (c *Client) LastUsage() (Usage, bool) {
	c.usageMu.Lock()
	defer c.usageMu.Unlock()
	if c.lastUsage.Buida() {
		return Usage{}, false
	}
	return c.lastUsage, true
}

func (c *Client) clearUsage() {
	c.usageMu.Lock()
	c.lastUsage = Usage{}
	c.usageMu.Unlock()
}

func (c *Client) setUsage(u Usage) {
	if u.Buida() {
		return
	}
	c.usageMu.Lock()
	c.lastUsage = u
	c.usageMu.Unlock()
}

// New crea un client amb timeout de 600s (els models locals que raonen triguen).
func New() *Client {
	// 600 s i no 180: un model local que raona pot trigar molt més que un de
	// núvol (el pensament compta dins del mateix temps), i amb 180 s el client
	// avortava a mig generar i el reintent tornava a començar de zero, de
	// manera que la tasca no avançava mai. El que ha de tallar una petició
	// penjada és /stop, no el rellotge.
	// 1200 s i no 600: una sola generació llarga (un write de 32k tokens a
	// ~35-40 t/s) pot durar 10-13 min. Amb 600 el client avortava el pas
	// just quan el write gegant anava ple i el reintent començava de zero.
	return &Client{hc: &http.Client{Timeout: 1200 * time.Second}, cache: map[string]promptCacheConfig{}}
}

// SetPromptCache activa els camps opcionals de prompt caching per a un
// provider/model. No s'envien per defecte perquè els endpoints locals i els
// proxies antics poden rebutjar camps desconeguts.
// WithThink marca al context si el model ha de raonar. Si el rol té think: no,
// la petició s'envia amb enable_thinking: false i el model no gasta tokens ni
// temps pensant. És la mateixa tècnica que els avisos de reintent: el valor
// viatja amb el context i cap firma canvia.
func WithThink(ctx context.Context, mode string) context.Context {
	if mode == "" {
		return ctx
	}
	return context.WithValue(ctx, thinkKey{}, mode)
}

type thinkKey struct{}

func (c *Client) chatRequestExtra(ctx context.Context) *bool {
	if v, ok := ctx.Value(thinkKey{}).(string); ok && v == "no" {
		fals := false
		return &fals
	}
	return nil
}

// thinkKwargs és la via que el passador local respecta: chat_template_kwargs.
func (c *Client) thinkKwargs(ctx context.Context) map[string]any {
	if v, ok := ctx.Value(thinkKey{}).(string); ok && v == "no" {
		return map[string]any{"enable_thinking": false}
	}
	return nil
}

// SetPromptCache activa els camps opcionals de prompt caching per a un
// provider/model. No s'envien per defecte perquè els endpoints locals i els
// proxies antics poden rebutjar camps desconeguts.
func (c *Client) SetPromptCache(baseURL, model, key, retention string) {
	key = strings.TrimSpace(key)
	if c == nil || key == "" || strings.TrimSpace(baseURL) == "" || strings.TrimSpace(model) == "" {
		return
	}
	c.cacheMu.Lock()
	if c.cache == nil {
		c.cache = map[string]promptCacheConfig{}
	}
	c.cache[cacheKey(baseURL, model)] = promptCacheConfig{key: key, retention: strings.TrimSpace(retention)}
	c.cacheMu.Unlock()
}

func (c *Client) promptCache(baseURL, model string) promptCacheConfig {
	if c == nil {
		return promptCacheConfig{}
	}
	c.cacheMu.RLock()
	defer c.cacheMu.RUnlock()
	return c.cache[cacheKey(baseURL, model)]
}

func cacheKey(baseURL, model string) string {
	return strings.TrimSuffix(strings.TrimSpace(baseURL), "/") + "\x00" + strings.TrimSpace(model)
}

func (c *Client) post(ctx context.Context, baseURL, apiKey string, reqBody chatRequest) (*http.Response, error) {
	url := strings.TrimSuffix(baseURL, "/") + "/chat/completions"
	if cache := c.promptCache(baseURL, reqBody.Model); cache.key != "" {
		reqBody.PromptCacheKey = cache.key
		reqBody.PromptCacheRetention = cache.retention
	}
	body, err := marshalChatRequest(reqBody)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	// opencode.ai/zen/go encamina per sessió: sense aquesta capçalera respon
	// "MissingSessionID". Hermes fa el mateix amb totes les seves crides.
	if strings.Contains(baseURL, "opencode.ai") {
		req.Header.Set("x-opencode-session", c.sessionID())
	}
	return c.hc.Do(req)
}

// marshalChatRequest és el wire específic del provider. No usa directament
// json.Marshal(chatRequest), perquè això passaria per Message.MarshalJSON i
// faria que la sanitització d'arguments afectés també la persistència.
func marshalChatRequest(req chatRequest) ([]byte, error) {
	messages := make([]json.RawMessage, len(req.Messages))
	for i, msg := range req.Messages {
		raw, err := marshalMessage(msg, true)
		if err != nil {
			return nil, err
		}
		messages[i] = raw
	}
	type requestWire struct {
		Model                string            `json:"model"`
		Messages             []json.RawMessage `json:"messages"`
		Temperature          float64           `json:"temperature,omitempty"`
		MaxTokens            int               `json:"max_tokens,omitempty"`
		Stream               bool              `json:"stream"`
		Tools                []any             `json:"tools,omitempty"`
		PromptCacheKey       string            `json:"prompt_cache_key,omitempty"`
		PromptCacheRetention string            `json:"prompt_cache_retention,omitempty"`
		StreamOptions        *streamOptions    `json:"stream_options,omitempty"`
		// L'ordre de no pensar (rol amb think: no). Sense aquests dos camps
		// aquí, chatRequest els portava però mai sortien pel cable: el
		// think: no del commit d423b12 no tenia cap efecte.
		EnableThinking     *bool          `json:"enable_thinking,omitempty"`
		ChatTemplateKwargs map[string]any `json:"chat_template_kwargs,omitempty"`
		ReasoningEffort    string         `json:"reasoning_effort,omitempty"`
	}
	return json.Marshal(requestWire{
		Model: req.Model, Messages: messages, Temperature: req.Temperature,
		MaxTokens: req.MaxTokens, Stream: req.Stream, Tools: req.Tools,
		PromptCacheKey: req.PromptCacheKey, PromptCacheRetention: req.PromptCacheRetention,
		StreamOptions:  req.StreamOptions,
		EnableThinking: req.EnableThinking, ChatTemplateKwargs: req.ChatTemplateKwargs,
		ReasoningEffort: req.ReasoningEffort,
	})
}

// sessionID retorna l'identificador de sessió estable del client (una sola
// vegada per procés, perquè l'encaminament del provider sigui coherent).
func (c *Client) sessionID() string {
	c.sessionOnce.Do(func() {
		var b [12]byte
		if _, err := rand.Read(b[:]); err == nil {
			c.session = hex.EncodeToString(b[:])
		} else {
			c.session = fmt.Sprintf("gregal-%d", time.Now().UnixNano())
		}
	})
	return c.session
}

// Chat fa una petició no-streaming i retorna el contingut del primer choice.
// Si el torn queda buit per length, ho torna a provar una vegada amb el
// doble de pressupost (retryLength) abans de donar l'error per bo.
func (c *Client) Chat(ctx context.Context, baseURL, apiKey, model string, msgs []Message, temp float64, maxTokens int) (string, error) {
	var out string
	err := retryFit(ctx, maxTokens, func(budget int) error {
		var err error
		out, err = c.chatInner(ctx, baseURL, apiKey, model, msgs, temp, budget)
		return err
	})
	return out, err
}

func (c *Client) chatInner(ctx context.Context, baseURL, apiKey, model string, msgs []Message, temp float64, maxTokens int) (string, error) {
	c.clearUsage()
	resp, err := c.postCompat(ctx, baseURL, apiKey, chatRequest{
		Model: model, Messages: msgs,
		Temperature: temp, MaxTokens: maxTokens,
		EnableThinking: c.chatRequestExtra(ctx), ChatTemplateKwargs: c.thinkKwargs(ctx), ReasoningEffort: c.reasoningEffort(ctx), Stream: false,
	})
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("provider %s: HTTP %d: %s%s", baseURL, resp.StatusCode, truncate(string(raw), 2000), zenHint(baseURL, resp.StatusCode, string(raw)))
	}
	// Una resposta 2xx ja és una crida feta (i facturada): compta al
	// comptador del context encara que després sigui buida o tallada.
	rec := c.usageCall(ctx)
	defer rec.done()
	var out chatResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", fmt.Errorf("resposta no JSON: %w", err)
	}
	rec.see(out.Usage)
	if out.Error != nil {
		return "", fmt.Errorf("provider: %s", out.Error.Message)
	}
	if len(out.Choices) == 0 {
		return "", fmt.Errorf("provider: sense choices a la resposta")
	}
	ch := out.Choices[0]
	// Buit inclou només-espais: un model que torna "\n\n" amb finish=stop
	// passava com a resposta bona i el TUI pintava un GREGAL › en blanc
	// sense cap error. Retallat: si no queda res, és torn buit.
	ch.Message.Content = senseThink(ch.Message.Content)
	if strings.TrimSpace(ch.Message.Content) == "" {
		return "", emptyReplyErr(ch.FinishReason, len([]rune(ch.Message.ReasoningContent+ch.Message.Reasoning+ch.Message.Thought+ch.Message.Thinking)), maxTokens)
	}
	if ch.FinishReason == "length" {
		return "", &LengthError{MaxTokens: maxTokens, Partial: true}
	}
	return ch.Message.Content, nil
}

// ChatWithTools crida amb specs d'eines i retorna contingut + tool_calls.
// Amb el mateix reintent per length que Chat.
func (c *Client) ChatWithTools(ctx context.Context, baseURL, apiKey, model string, msgs []Message, temp float64, maxTokens int, specs []ToolSpec) (string, []ToolCall, error) {
	var text string
	var calls []ToolCall
	err := retryFit(ctx, maxTokens, func(budget int) error {
		var err error
		text, calls, err = c.chatWithToolsInner(ctx, baseURL, apiKey, model, msgs, temp, budget, specs)
		return err
	})
	return text, calls, err
}

func (c *Client) chatWithToolsInner(ctx context.Context, baseURL, apiKey, model string, msgs []Message, temp float64, maxTokens int, specs []ToolSpec) (string, []ToolCall, error) {
	c.clearUsage()
	var tos []any
	for _, s := range specs {
		tos = append(tos, map[string]any{
			"type": "function",
			"function": map[string]any{
				"name": s.Name, "description": s.Description, "parameters": s.Parameters,
			},
		})
	}
	resp, err := c.postCompat(ctx, baseURL, apiKey, chatRequest{
		Model: model, Messages: msgs,
		Temperature: temp, MaxTokens: maxTokens,
		EnableThinking: c.chatRequestExtra(ctx), ChatTemplateKwargs: c.thinkKwargs(ctx), ReasoningEffort: c.reasoningEffort(ctx), Stream: false, Tools: tos,
	})
	if err != nil {
		return "", nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", nil, err
	}
	if resp.StatusCode >= 300 {
		return "", nil, fmt.Errorf("provider %s: HTTP %d: %s%s", baseURL, resp.StatusCode, truncate(string(raw), 2000), zenHint(baseURL, resp.StatusCode, string(raw)))
	}
	rec := c.usageCall(ctx)
	defer rec.done()
	var out chatResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", nil, fmt.Errorf("resposta no JSON: %w", err)
	}
	rec.see(out.Usage)
	if out.Error != nil {
		return "", nil, fmt.Errorf("provider: %s", out.Error.Message)
	}
	if len(out.Choices) == 0 {
		return "", nil, fmt.Errorf("provider: sense choices a la resposta")
	}
	m := out.Choices[0].Message
	for i := range m.ToolCalls {
		if m.ToolCalls[i].Type == "" {
			m.ToolCalls[i].Type = "function"
		}
	}
	// Proxy que no tradueix el DSML de DeepSeek o el format <tool_call> de models locals a tool_calls:
	// la crida ve dins de content. La recuperem en comptes d'ensenyar-la a l'usuari.
	if len(m.ToolCalls) == 0 && hasDSML(m.Content) {
		m.Content, m.ToolCalls = parseDSML(m.Content)
	}
	// Aquí el contingut buit és normal si el torn és una crida d'eina; el que
	// no pot ser és acabar sense text ni eines i fer com si res (inclòs
	// només-espais: vegeu chatInner).
	if strings.TrimSpace(m.Content) == "" && len(m.ToolCalls) == 0 {
		return "", nil, emptyReplyErr(out.Choices[0].FinishReason, len([]rune(m.ReasoningContent+m.Reasoning+m.Thought+m.Thinking)), maxTokens)
	}
	if out.Choices[0].FinishReason == "length" {
		return "", nil, &LengthError{MaxTokens: maxTokens, Partial: true}
	}
	return senseThink(m.Content), m.ToolCalls, nil
}

// ChatStream crida amb stream:true, envia cada token a onToken (i el
// raonament a onReason) i retorna el text sencer. Amb el mateix reintent
// per length que Chat (només si no s'ha emès ni una lletra).
func (c *Client) ChatStream(ctx context.Context, baseURL, apiKey, model string, msgs []Message, temp float64, maxTokens int, onToken, onReason func(string)) (string, error) {
	var out string
	err := retryFit(ctx, maxTokens, func(budget int) error {
		var err error
		out, err = c.chatStreamInner(ctx, baseURL, apiKey, model, msgs, temp, budget, onToken, onReason)
		return err
	})
	return out, err
}

func (c *Client) chatStreamInner(ctx context.Context, baseURL, apiKey, model string, msgs []Message, temp float64, maxTokens int, onToken, onReason func(string)) (string, error) {
	c.clearUsage()
	resp, err := c.postStream(ctx, baseURL, apiKey, chatRequest{
		Model: model, Messages: msgs,
		Temperature: temp, MaxTokens: maxTokens,
		EnableThinking: c.chatRequestExtra(ctx), ChatTemplateKwargs: c.thinkKwargs(ctx), ReasoningEffort: c.reasoningEffort(ctx), Stream: true,
	})
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 2000))
		return "", fmt.Errorf("provider %s: HTTP %d: %s%s", baseURL, resp.StatusCode, truncate(string(raw), 2000), zenHint(baseURL, resp.StatusCode, string(raw)))
	}
	rec := c.usageCall(ctx)
	defer rec.done()
	var full strings.Builder
	reasoned, finish := 0, ""
	sawDone := false
	br := bufio.NewReader(resp.Body)
	for {
		line, rerr := br.ReadString('\n')
		if t := strings.TrimSpace(line); strings.HasPrefix(t, "data: ") {
			payload := strings.TrimPrefix(t, "data: ")
			if payload == "[DONE]" {
				sawDone = true
				break
			}
			var ev streamEvent
			if jerr := json.Unmarshal([]byte(payload), &ev); jerr == nil {
				rec.see(ev.Usage)
				for _, ch := range ev.Choices {
					if d := ch.Delta.Content; d != "" {
						full.WriteString(d)
						if onToken != nil {
							onToken(d)
						}
					}
					if r := ch.Delta.ReasoningContent + ch.Delta.Reasoning + ch.Delta.Thought + ch.Delta.Thinking; r != "" {
						reasoned += len([]rune(r))
						if onReason != nil {
							onReason(r)
						}
					}
					if ch.FinishReason != "" {
						finish = ch.FinishReason
					}
				}
			}
		}
		if rerr != nil {
			break
		}
	}
	if !sawDone && finish == "" {
		return "", &StreamInterruptedError{}
	}
	text := full.String()
	if strings.Contains(text, "think>") {
		cl, th := extractThinkTags(text)
		text = cl
		reasoned += len([]rune(th))
		if onReason != nil && th != "" {
			onReason(th)
		}
	}
	// Aquesta via no porta eines. Si el model hi ha respost amb una crida en
	// DSML (DeepSeek sense esquema de tools), no s'imprimeix el markup: es
	// treu del text i s'explica què ha passat.
	if hasDSML(text) {
		clean, calls := parseDSML(text)
		if len(calls) > 0 {
			return clean, dsmlLeakErr(calls)
		}
		text = clean
	}
	if strings.TrimSpace(text) == "" {
		return "", emptyReplyErr(finish, reasoned, maxTokens)
	}
	if finish == "length" {
		return "", lengthTallat(reasoned, maxTokens, len([]rune(strings.TrimSpace(text))))
	}
	return text, nil
}

// emptyReplyErr explica un torn que acaba sense ni una lletra de resposta.
// El cas típic és un model de raonament amb max_tokens curt: el pensament
// es menja tot el pressupost i la resposta ja no hi cap. Abans això era una
// línia en blanc i cap pista. Els casos de length tornen *LengthError
// (recuperable amb més pressupost: vegeu retryLength); la resta, error pla.
func emptyReplyErr(finish string, reasoned, maxTokens int) error {
	if finish == "length" {
		return &LengthError{Reasoned: reasoned, MaxTokens: maxTokens}
	}
	if reasoned > 0 {
		return fmt.Errorf("el model ha pensat %d caràcters però no ha escrit cap resposta (finish_reason %q)", reasoned, finish)
	}
	return fmt.Errorf("el model ha tornat una resposta buida (finish_reason %q)", finish)
}

// toolsWire converteix els specs al format OpenAI de `tools`.
func toolsWire(specs []ToolSpec) []any {
	var tos []any
	for _, s := range specs {
		tos = append(tos, map[string]any{
			"type": "function",
			"function": map[string]any{
				"name": s.Name, "description": s.Description, "parameters": s.Parameters,
			},
		})
	}
	return tos
}

// ChatStreamWithTools és ChatWithTools amb stream:true: el text arriba en
// directe a onToken (i el raonament a onReason) i les tool_calls es
// reconstrueixen dels deltes. Amb això totes les vies porten eines i cap
// prompt pot prometre'n unes que la crida no dona (1.1.3): abans el xat
// anava sense `tools` per poder fer streaming, i DeepSeek escrivia la crida
// en DSML com a text.
func (c *Client) ChatStreamWithTools(ctx context.Context, baseURL, apiKey, model string, msgs []Message, temp float64, maxTokens int, specs []ToolSpec, onToken, onReason func(string)) (string, []ToolCall, error) {
	var text string
	var calls []ToolCall
	err := retryFit(ctx, maxTokens, func(budget int) error {
		var err error
		text, calls, err = c.chatStreamWithToolsInner(ctx, baseURL, apiKey, model, msgs, temp, budget, specs, onToken, onReason)
		return err
	})
	return text, calls, err
}

func (c *Client) chatStreamWithToolsInner(ctx context.Context, baseURL, apiKey, model string, msgs []Message, temp float64, maxTokens int, specs []ToolSpec, onToken, onReason func(string)) (string, []ToolCall, error) {
	c.clearUsage()
	resp, err := c.postStream(ctx, baseURL, apiKey, chatRequest{
		Model: model, Messages: msgs,
		Temperature: temp, MaxTokens: maxTokens,
		EnableThinking: c.chatRequestExtra(ctx), ChatTemplateKwargs: c.thinkKwargs(ctx), ReasoningEffort: c.reasoningEffort(ctx), Stream: true, Tools: toolsWire(specs),
	})
	if err != nil {
		return "", nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 2000))
		return "", nil, fmt.Errorf("provider %s: HTTP %d: %s%s", baseURL, resp.StatusCode, truncate(string(raw), 2000), zenHint(baseURL, resp.StatusCode, string(raw)))
	}
	rec := c.usageCall(ctx)
	defer rec.done()
	// Alguns proxies ignoren stream:true i tornen el JSON sencer d'una vegada
	// (i els fakes dels tests també). Es llegeix com a resposta normal: el
	// text arriba a onToken d'un sol cop i les tool_calls ja vénen muntades.
	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "text/event-stream") {
		raw, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", nil, err
		}
		var out chatResponse
		if err := json.Unmarshal(raw, &out); err != nil {
			return "", nil, fmt.Errorf("resposta no JSON ni SSE: %w", err)
		}
		if out.Error != nil {
			return "", nil, fmt.Errorf("provider: %s", out.Error.Message)
		}
		if len(out.Choices) == 0 {
			return "", nil, fmt.Errorf("provider: sense choices a la resposta")
		}
		rec.see(out.Usage)
		m := out.Choices[0].Message
		for i := range m.ToolCalls {
			if m.ToolCalls[i].Type == "" {
				m.ToolCalls[i].Type = "function"
			}
		}
		text, calls := m.Content, m.ToolCalls
		if len(calls) == 0 && hasDSML(text) {
			text, calls = parseDSML(text)
		}
		if strings.TrimSpace(text) == "" && len(calls) == 0 {
			return "", nil, emptyReplyErr(out.Choices[0].FinishReason, len([]rune(m.ReasoningContent+m.Reasoning+m.Thought+m.Thinking)), maxTokens)
		}
		if out.Choices[0].FinishReason == "length" {
			// El retorn és abans de cridar onToken: la resposta no s'ha
			// ensenyat, així que un tall aquí encara es pot reintentar.
			return "", nil, lengthTallat(len([]rune(m.ReasoningContent+m.Reasoning+m.Thought+m.Thinking)), maxTokens, 0)
		}
		if text != "" && onToken != nil {
			onToken(text)
		}
		return text, calls, nil
	}
	var full strings.Builder
	reasoned, finish := 0, ""
	sawDone := false
	// Acumulació per índex: l'ordre d'arribada no és l'ordre de les crides.
	byIndex := map[int]*ToolCall{}
	order := []int{}
	br := bufio.NewReader(resp.Body)
	for {
		line, rerr := br.ReadString('\n')
		if t := strings.TrimSpace(line); strings.HasPrefix(t, "data: ") {
			payload := strings.TrimPrefix(t, "data: ")
			if payload == "[DONE]" {
				sawDone = true
				break
			}
			var ev streamEvent
			if jerr := json.Unmarshal([]byte(payload), &ev); jerr == nil {
				rec.see(ev.Usage)
				for _, ch := range ev.Choices {
					if d := ch.Delta.Content; d != "" {
						full.WriteString(d)
						if onToken != nil {
							onToken(d)
						}
					}
					if r := ch.Delta.ReasoningContent + ch.Delta.Reasoning + ch.Delta.Thought + ch.Delta.Thinking; r != "" {
						reasoned += len([]rune(r))
						if onReason != nil {
							onReason(r)
						}
					}
					for _, tc := range ch.Delta.ToolCalls {
						cur, ok := byIndex[tc.Index]
						if !ok {
							cur = &ToolCall{Type: "function"}
							byIndex[tc.Index] = cur
							order = append(order, tc.Index)
						}
						if tc.ID != "" {
							cur.ID = tc.ID
						}
						if tc.Type != "" {
							cur.Type = tc.Type
						}
						if tc.Function.Name != "" {
							cur.Function.Name += tc.Function.Name
						}
						cur.Function.Arguments += tc.Function.Arguments
					}
					if ch.FinishReason != "" {
						finish = ch.FinishReason
					}
				}
			}
		}
		if rerr != nil {
			break
		}
	}
	if !sawDone && finish == "" {
		return "", nil, &StreamInterruptedError{}
	}
	calls := make([]ToolCall, 0, len(order))
	for _, i := range order {
		calls = append(calls, *byIndex[i])
	}
	text := full.String()
	if strings.Contains(text, "think>") {
		cl, th := extractThinkTags(text)
		text = cl
		reasoned += len([]rune(th))
		if onReason != nil && th != "" {
			onReason(th)
		}
	}
	// Proxy que no tradueix el DSML: la crida ve dins del text.
	if len(calls) == 0 && hasDSML(text) {
		text, calls = parseDSML(text)
	}
	if strings.TrimSpace(text) == "" && len(calls) == 0 {
		return "", nil, emptyReplyErr(finish, reasoned, maxTokens)
	}
	if finish == "length" {
		return "", nil, &LengthError{Reasoned: reasoned, MaxTokens: maxTokens, Partial: true}
	}
	return text, calls, nil
}

// zenHint afegeix guia quan el Zen rebutja amb 500 genèric: el model
// consta a /models però el workspace el té desactivat (Model access,
// típic en models sense zero data retention). És consentiment de
// l'admin al dashboard i cap client ho pot activar per API.
func zenHint(baseURL string, status int, body string) string {
	if status == 500 && strings.Contains(baseURL, "opencode.ai") &&
		strings.Contains(body, "Internal server error") {
		return " — el model consta però el workspace el rebutja: activa'l a Model access al dashboard (cal admin)"
	}
	return ""
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// EstimateTokens aproxima tokens (caràcters/4 + 8 per missatge). És una
// estimació (~), no un recompte del tokenizer. Cada imatge compta com a
// 1000 tokens (conservador, estil OpenAI vision de baixa resolució).
// MARGE: l'error típic és ±30%. No feu servir aquest nombre per retallar al
// límit de la finestra: CompactThreshold (agent/compact.go, 0.75) deixa un
// 25% de coixí precisament per aquest error.
func EstimateTokens(msgs []Message) int {
	n := 0
	for _, m := range msgs {
		n += len([]rune(m.Role)) / 4
		n += len([]rune(m.Content)) / 4
		n += 1000 * len(m.Images)
		for _, c := range m.ToolCalls {
			n += len([]rune(c.Function.Arguments)) / 4
		}
		n += 8
	}
	return n
}

// FmtCount humanitza: 999 → "999", 1500 → "1.5k".
func FmtCount(n int) string {
	if n < 1000 {
		return itoa(n)
	}
	k := float64(n) / 1000
	if k >= 100 {
		return itoa(n/1000) + "k"
	}
	return trimZero(k) + "k"
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}

func trimZero(k float64) string {
	s := fmt.Sprintf("%.1f", k)
	return strings.TrimSuffix(s, ".0")
}

func extractThinkTags(s string) (clean, think string) {
	var cleanB, thinkB strings.Builder
	// «</think>» sense cap «<think>» al davant: la plantilla del model ja
	// obre el bloc de raonament dins del prompt, i el que arriba és el
	// final del pensament i el tancament. Amb think: no, glm-5.3-flash
	// (opencode zen) tornava «</think>Arreglat…» i l'etiqueta sortia a la
	// resposta de l'usuari.
	if end := strings.Index(s, "</think>"); end >= 0 {
		if start := strings.Index(s, "<think>"); start == -1 || end < start {
			thinkB.WriteString(s[:end])
			s = s[end+len("</think>"):]
		}
	}
	for {
		start := strings.Index(s, "<think>")
		if start == -1 {
			cleanB.WriteString(s)
			break
		}
		cleanB.WriteString(s[:start])
		s = s[start+len("<think>"):]
		end := strings.Index(s, "</think>")
		if end == -1 {
			thinkB.WriteString(s)
			break
		}
		thinkB.WriteString(s[:end])
		s = s[end+len("</think>"):]
	}
	return cleanB.String(), thinkB.String()
}

// senseThink és el text sense les etiquetes de raonament (les crides sense
// streaming, que no tenen on enviar el pensament).
func senseThink(s string) string {
	if !strings.Contains(s, "think>") {
		return s
	}
	cl, _ := extractThinkTags(s)
	return strings.TrimLeft(cl, "\r\n")
}
