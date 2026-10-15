package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strings"
)

// Proveïdor natiu Anthropic (Messages API, POST <base>/messages).
//
// Traducció del format comú (Message/ToolCall, estil OpenAI) al d'Anthropic:
//   - els missatges system del principi van al camp system (blocs de text);
//     els que arriben a mitja conversa passen a text d'usuari, perquè la
//     posició dels system intermedis té restriccions que el motor no coneix;
//   - les crides d'eina són blocs tool_use dins del missatge assistant i els
//     resultats, blocs tool_result dins del missatge user següent (primer de
//     tot, abans de qualsevol text);
//   - els missatges seguits del mateix rol es fusionen (l'API alterna);
//   - els blocs de pensament (thinking / redacted_thinking) es tornen intactes,
//     amb la signatura, al missatge assistant d'on van sortir (blocsNatius);
//   - prompt caching: cache_control al darrer bloc del system, a la darrera
//     eina i al darrer bloc de la conversa (tres punts de tall de quatre).
//
// El pensament segueix el mode think del rol: low/medium/high/max envien
// thinking adaptive (amb resum visible) i output_config.effort; «no» demana
// effort low sense configurar thinking (els models on el pensament és
// obligatori no accepten desactivar-lo); buit/auto envia adaptive sense
// effort. temperature no s'envia: els models actuals la rebutgen.

var campsAnthropic = []campNatiu{
	// Claus estretes: un error dels blocs reenviats (signatura) no ha
	// d'apagar el pensament per a tot el procés (vegeu thinking_blocks).
	{nom: "thinking", claus: []string{"thinking.type", "thinking.display", "adaptive", "does not support thinking", "thinking is not supported", "thinking: extra"}},
	{nom: "output_config", claus: []string{"output_config", "effort"}},
	{nom: "eager_input_streaming", claus: []string{"eager_input_streaming"}},
	// Els blocs de pensament reenviats: si l'API els rebutja (historial
	// editat per la compactació, signatura d'un altre model…) es reintenta
	// sense, però només aquella petició: a la següent es tornen a provar.
	{nom: "thinking_blocks", claus: []string{"thinking", "signature"}, efimer: true},
}

type antBlock = map[string]any

type antMsg struct {
	Role    string     `json:"role"`
	Content []antBlock `json:"content"`
}

var antIDInvalid = regexp.MustCompile(`[^a-zA-Z0-9_-]`)

// antID adapta un id de crida al patró que accepta l'API (^[a-zA-Z0-9_-]+$).
func antID(id string) string {
	id = antIDInvalid.ReplaceAllString(id, "_")
	if id == "" {
		return "call_0"
	}
	return id
}

func antImage(u string) antBlock {
	if mt, data, ok := splitDataURL(u); ok {
		return antBlock{"type": "image", "source": map[string]any{"type": "base64", "media_type": mt, "data": data}}
	}
	return antBlock{"type": "image", "source": map[string]any{"type": "url", "url": u}}
}

// anthropicMessages tradueix l'historial. Torna també si el darrer missatge
// assistant amb crides d'eina té els seus blocs de pensament a la memòria
// (sense, cal no demanar pensament: l'API exigiria els blocs).
func (c *Client) anthropicMessages(msgs []Message, ambPensament bool) (system []antBlock, out []antMsg, pensamentOK bool) {
	msgs = normalitzaOrdreEines(msgs)
	pensamentOK = true
	pendents := map[string]bool{} // tool_use sense resultat encara
	ordrePendents := []string{}
	tancaPendents := func() {
		if len(pendents) == 0 {
			return
		}
		var blocs []antBlock
		for _, id := range ordrePendents {
			if pendents[id] {
				blocs = append(blocs, antBlock{"type": "tool_result", "tool_use_id": id, "content": "(no result recorded)", "is_error": true})
			}
		}
		pendents, ordrePendents = map[string]bool{}, nil
		if len(blocs) > 0 {
			out = afegeixUser(out, blocs, true)
		}
	}
	vistNoSystem := false
	for _, m := range msgs {
		switch m.Role {
		case "system":
			if strings.TrimSpace(m.Content) == "" {
				continue
			}
			if !vistNoSystem {
				system = append(system, antBlock{"type": "text", "text": m.Content})
				continue
			}
			tancaPendents()
			out = afegeixUser(out, []antBlock{{"type": "text", "text": "[system] " + m.Content}}, false)
		case "tool":
			vistNoSystem = true
			id := antID(m.ToolCallID)
			var content []antBlock
			if strings.TrimSpace(m.Content) != "" {
				content = append(content, antBlock{"type": "text", "text": m.Content})
			}
			for _, img := range m.Images {
				content = append(content, antImage(img))
			}
			if len(content) == 0 {
				content = append(content, antBlock{"type": "text", "text": "(empty)"})
			}
			if !pendents[id] {
				// Resultat orfe (sense tool_use al davant): l'API el
				// rebutjaria; passa a text perquè el model el vegi igual.
				out = afegeixUser(out, append([]antBlock{{"type": "text", "text": "Tool result (" + m.ToolCallID + "):"}}, content...), false)
				continue
			}
			pendents[id] = false
			out = afegeixUser(out, []antBlock{{"type": "tool_result", "tool_use_id": id, "content": content}}, true)
		case "assistant":
			vistNoSystem = true
			tancaPendents()
			var blocs []antBlock
			natius, ok := c.natius.troba(m.ToolCalls, m.Content)
			if !ambPensament {
				natius, ok = nil, false
			}
			if ok {
				for _, raw := range natius {
					var b antBlock
					if json.Unmarshal(raw, &b) == nil {
						blocs = append(blocs, b)
					}
				}
			}
			if txt := senseThink(m.Content); strings.TrimSpace(txt) != "" {
				blocs = append(blocs, antBlock{"type": "text", "text": txt})
			}
			for _, tc := range m.ToolCalls {
				id := antID(tc.ID)
				blocs = append(blocs, antBlock{"type": "tool_use", "id": id, "name": tc.Function.Name, "input": validArgs(tc.Function.Arguments)})
				if _, ja := pendents[id]; !ja {
					ordrePendents = append(ordrePendents, id)
				}
				pendents[id] = true
			}
			if len(blocs) == 0 {
				continue
			}
			if len(m.ToolCalls) > 0 {
				pensamentOK = ok
			}
			if n := len(out); n > 0 && out[n-1].Role == "assistant" {
				out[n-1].Content = append(out[n-1].Content, blocs...)
			} else {
				out = append(out, antMsg{Role: "assistant", Content: blocs})
			}
		default: // user
			vistNoSystem = true
			tancaPendents()
			var blocs []antBlock
			for _, img := range m.Images {
				blocs = append(blocs, antImage(img))
			}
			if strings.TrimSpace(m.Content) != "" {
				blocs = append(blocs, antBlock{"type": "text", "text": m.Content})
			}
			if len(blocs) == 0 {
				continue
			}
			out = afegeixUser(out, blocs, false)
		}
	}
	tancaPendents()
	if len(out) == 0 || out[0].Role != "user" {
		out = append([]antMsg{{Role: "user", Content: []antBlock{{"type": "text", "text": "(continue)"}}}}, out...)
	}
	// Els models actuals no accepten prefill: la conversa ha d'acabar en user.
	if out[len(out)-1].Role == "assistant" {
		out = append(out, antMsg{Role: "user", Content: []antBlock{{"type": "text", "text": "Continue."}}})
	}
	return system, out, pensamentOK
}

// afegeixUser afegeix blocs al darrer missatge user (o en crea un). Els
// tool_result han d'anar davant de qualsevol altre bloc del missatge.
func afegeixUser(out []antMsg, blocs []antBlock, resultats bool) []antMsg {
	n := len(out)
	if n == 0 || out[n-1].Role != "user" {
		return append(out, antMsg{Role: "user", Content: blocs})
	}
	if !resultats {
		out[n-1].Content = append(out[n-1].Content, blocs...)
		return out
	}
	cur := out[n-1].Content
	i := 0
	for i < len(cur) && cur[i]["type"] == "tool_result" {
		i++
	}
	merged := make([]antBlock, 0, len(cur)+len(blocs))
	merged = append(merged, cur[:i]...)
	merged = append(merged, blocs...)
	merged = append(merged, cur[i:]...)
	out[n-1].Content = merged
	return out
}

func antTools(specs []ToolSpec, stream bool, off map[string]bool) []antBlock {
	var tools []antBlock
	for _, s := range specs {
		schema := s.Parameters
		if schema == nil {
			schema = map[string]any{"type": "object", "properties": map[string]any{}}
		}
		t := antBlock{"name": s.Name, "description": s.Description, "input_schema": schema}
		if stream && !off["eager_input_streaming"] {
			t["eager_input_streaming"] = true
		}
		tools = append(tools, t)
	}
	return tools
}

func cacheEphemeral() map[string]any { return map[string]any{"type": "ephemeral"} }

// marcaCache posa cache_control al darrer bloc que el pot portar.
func marcaCache(blocs []antBlock) {
	for i := len(blocs) - 1; i >= 0; i-- {
		switch blocs[i]["type"] {
		case "thinking", "redacted_thinking":
			continue
		}
		blocs[i]["cache_control"] = cacheEphemeral()
		return
	}
}

// copyBlocks fa una còpia superficial dels blocs per no tocar els que
// s'han reutilitzat de la memòria entre intents de negociació.
func copyBlocks(in []antBlock) []antBlock {
	out := make([]antBlock, len(in))
	for i, b := range in {
		nb := make(antBlock, len(b)+1)
		for k, v := range b {
			nb[k] = v
		}
		out[i] = nb
	}
	return out
}

func (c *Client) anthropicBody(ctx context.Context, model string, msgs []Message, maxTokens int, specs []ToolSpec, stream bool) peticioNativa {
	system, conv, pensamentOK := c.anthropicMessages(msgs, true)
	teBlocs := false
	for _, m := range conv {
		for _, b := range m.Content {
			if b["type"] == "thinking" || b["type"] == "redacted_thinking" {
				teBlocs = true
			}
		}
	}
	if maxTokens <= 0 {
		maxTokens = 8192
	}
	mode := thinkMode(ctx)
	return func(off map[string]bool) ([]byte, []string, error) {
		var presents []string
		conv, pensamentOK := conv, pensamentOK
		if teBlocs {
			if off["thinking_blocks"] {
				_, conv, pensamentOK = c.anthropicMessages(msgs, false)
			} else {
				presents = append(presents, "thinking_blocks")
			}
		}
		body := map[string]any{"model": model, "max_tokens": maxTokens}
		if stream {
			body["stream"] = true
		}
		if len(system) > 0 {
			sys := copyBlocks(system)
			marcaCache(sys)
			body["system"] = sys
		}
		if len(specs) > 0 {
			tools := antTools(specs, stream, off)
			if stream && !off["eager_input_streaming"] {
				presents = append(presents, "eager_input_streaming")
			}
			tools[len(tools)-1]["cache_control"] = cacheEphemeral()
			body["tools"] = tools
		}
		msgsOut := make([]antMsg, len(conv))
		copy(msgsOut, conv)
		last := len(msgsOut) - 1
		msgsOut[last] = antMsg{Role: msgsOut[last].Role, Content: copyBlocks(msgsOut[last].Content)}
		marcaCache(msgsOut[last].Content)
		body["messages"] = msgsOut
		effort := ""
		switch mode {
		case "no", "none":
			effort = "low"
		case "low", "medium", "high", "max":
			effort = mode
		}
		if mode != "no" && mode != "none" && pensamentOK && !off["thinking"] {
			body["thinking"] = map[string]any{"type": "adaptive", "display": "summarized"}
			presents = append(presents, "thinking")
		}
		if effort != "" && !off["output_config"] {
			body["output_config"] = map[string]any{"effort": effort}
			presents = append(presents, "output_config")
		}
		raw, err := json.Marshal(body)
		return raw, presents, err
	}
}

// antUsage és el consum que declara Anthropic.
type antUsage struct {
	InputTokens              int `json:"input_tokens"`
	OutputTokens             int `json:"output_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens"`
}

func (u antUsage) comu() Usage {
	var out Usage
	out.PromptTokens = u.InputTokens + u.CacheCreationInputTokens + u.CacheReadInputTokens
	out.CompletionTokens = u.OutputTokens
	out.TotalTokens = out.PromptTokens + out.CompletionTokens
	out.PromptTokensDetails.CachedTokens = u.CacheReadInputTokens
	out.CacheWriteTokens = u.CacheCreationInputTokens
	return out
}

// suma combina el consum de message_start (entrada) amb el de message_delta
// (sortida acumulada; alguns proxies hi repeteixen l'entrada).
func (u *antUsage) suma(d antUsage) {
	if d.InputTokens > 0 {
		u.InputTokens = d.InputTokens
	}
	if d.CacheCreationInputTokens > 0 {
		u.CacheCreationInputTokens = d.CacheCreationInputTokens
	}
	if d.CacheReadInputTokens > 0 {
		u.CacheReadInputTokens = d.CacheReadInputTokens
	}
	if d.OutputTokens > 0 {
		u.OutputTokens = d.OutputTokens
	}
}

type antRespBlock struct {
	Type      string          `json:"type"`
	Text      string          `json:"text"`
	Thinking  string          `json:"thinking"`
	Signature string          `json:"signature"`
	Data      string          `json:"data"`
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Input     json.RawMessage `json:"input"`
}

type antResponse struct {
	Type        string         `json:"type"`
	Content     []antRespBlock `json:"content"`
	StopReason  string         `json:"stop_reason"`
	StopDetails *antStop       `json:"stop_details"`
	Usage       antUsage       `json:"usage"`
	Error       *antError      `json:"error"`
	Message     *antResponse   `json:"message"` // message_start
}

type antStop struct {
	Category    string `json:"category"`
	Explanation string `json:"explanation"`
}

type antError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

// resultatNatiu és el que en surt de qualsevol protocol natiu.
type resultatNatiu struct {
	text     string
	reasoned int
	calls    []ToolCall
	blocs    []json.RawMessage // blocs per tornar (pensament / raonament)
	finish   string            // "stop", "length", "tool_calls", "refusal"
	refusal  string
}

func blocPensament(b antRespBlock) json.RawMessage {
	var raw []byte
	if b.Type == "redacted_thinking" {
		raw, _ = json.Marshal(map[string]any{"type": "redacted_thinking", "data": b.Data})
	} else {
		raw, _ = json.Marshal(map[string]any{"type": "thinking", "thinking": b.Thinking, "signature": b.Signature})
	}
	return raw
}

func antFinish(stop string) string {
	switch stop {
	case "max_tokens", "model_context_window_exceeded":
		return "length"
	case "tool_use":
		return "tool_calls"
	case "refusal":
		return "refusal"
	}
	return "stop"
}

func refusalText(d *antStop) string {
	if d == nil {
		return ""
	}
	s := d.Category
	if d.Explanation != "" {
		if s != "" {
			s += ": "
		}
		s += d.Explanation
	}
	return s
}

func (c *Client) anthropicChat(ctx context.Context, baseURL, apiKey, model string, msgs []Message, maxTokens int, specs []ToolSpec, stream bool, onToken, onReason func(string)) (string, []ToolCall, error) {
	c.clearUsage()
	headers := func(req *http.Request) {
		req.Header.Set("anthropic-version", AnthropicVersion)
		if k := strings.TrimSpace(apiKey); k != "" {
			req.Header.Set("x-api-key", k)
		}
		if stream {
			req.Header.Set("Accept", "text/event-stream")
		}
	}
	resp, err := c.postNatiu(ctx, baseURL, model, "/messages", hasMalformedToolArguments(msgs), campsAnthropic, headers, c.anthropicBody(ctx, model, msgs, maxTokens, specs, stream))
	if err != nil {
		return "", nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return "", nil, errHTTP(baseURL, resp)
	}
	rec := c.usageCall(ctx)
	defer rec.done()
	prog := nouProgres(ctx)
	var res resultatNatiu
	live := stream && esStream(resp)
	if live {
		res, err = c.anthropicStream(ctx, baseURL, resp.Body, rec, prog, onToken, onReason)
	} else {
		res, err = anthropicWhole(baseURL, resp.Body, rec, prog, onReason)
		if err == nil && stream && onToken != nil && res.finish != "length" && res.text != "" {
			onToken(res.text)
		}
	}
	if err != nil {
		return "", nil, err
	}
	return c.acabaNatiu(res, maxTokens, live && onToken != nil)
}

// acabaNatiu aplica les mateixes regles que la via chat completions: torn
// buit, tall per pressupost, rebuig; i desa els blocs per a la tornada.
func (c *Client) acabaNatiu(res resultatNatiu, maxTokens int, liveText bool) (string, []ToolCall, error) {
	if res.finish == "refusal" {
		msg := "the model declined the request (stop_reason refusal)"
		if res.refusal != "" {
			msg += ": " + res.refusal
		}
		return "", nil, fmt.Errorf("provider: %s", msg)
	}
	if strings.TrimSpace(res.text) == "" && len(res.calls) == 0 {
		return "", nil, emptyReplyErr(res.finish, res.reasoned, maxTokens)
	}
	if res.finish == "length" {
		visible := 0
		if liveText {
			visible = len([]rune(res.text))
		}
		return "", nil, lengthTallat(res.reasoned, maxTokens, visible).amb(res.text, res.calls)
	}
	c.natius.desa(res.calls, res.text, res.blocs)
	return res.text, res.calls, nil
}

func anthropicWhole(baseURL string, body io.Reader, rec *usageCall, prog *progresResposta, onReason func(string)) (resultatNatiu, error) {
	raw, err := io.ReadAll(body)
	if err != nil {
		return resultatNatiu{}, err
	}
	var out antResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return resultatNatiu{}, fmt.Errorf("resposta no JSON: %w", err)
	}
	if out.Error != nil {
		return resultatNatiu{}, fmt.Errorf("provider %s: %s: %s", baseURL, out.Error.Type, out.Error.Message)
	}
	rec.see(out.Usage.comu())
	prog.tros()
	var res resultatNatiu
	var text, reason strings.Builder
	for _, b := range out.Content {
		switch b.Type {
		case "text":
			text.WriteString(b.Text)
		case "thinking", "redacted_thinking":
			reason.WriteString(b.Thinking)
			res.blocs = append(res.blocs, blocPensament(b))
		case "tool_use":
			res.calls = append(res.calls, antCall(b.ID, b.Name, string(b.Input)))
		}
	}
	res.text = text.String()
	res.reasoned = len([]rune(reason.String()))
	prog.delta(ProgressReasoning, reason.String())
	prog.delta(ProgressText, res.text)
	for _, tc := range res.calls {
		prog.delta(ProgressTool, tc.Function.Arguments)
	}
	if onReason != nil && reason.Len() > 0 {
		onReason(reason.String())
	}
	res.finish = antFinish(out.StopReason)
	res.refusal = refusalText(out.StopDetails)
	return res, nil
}

func antCall(id, name, args string) ToolCall {
	var tc ToolCall
	tc.ID, tc.Type = id, "function"
	tc.Function.Name = name
	if strings.TrimSpace(args) == "" {
		args = "{}"
	}
	tc.Function.Arguments = args
	return tc
}

type antStreamEvent struct {
	Type         string       `json:"type"`
	Index        int          `json:"index"`
	Message      *antResponse `json:"message"`
	ContentBlock antRespBlock `json:"content_block"`
	Delta        struct {
		Type        string   `json:"type"`
		Text        string   `json:"text"`
		Thinking    string   `json:"thinking"`
		Signature   string   `json:"signature"`
		PartialJSON string   `json:"partial_json"`
		StopReason  string   `json:"stop_reason"`
		StopDetails *antStop `json:"stop_details"`
	} `json:"delta"`
	Usage *antUsage `json:"usage"`
	Error *antError `json:"error"`
}

type antBlocStream struct {
	tipus     string
	text      strings.Builder
	thinking  strings.Builder
	signature strings.Builder
	args      strings.Builder
	data      string
	id, name  string
	inputIni  json.RawMessage
}

func (c *Client) anthropicStream(ctx context.Context, baseURL string, body io.Reader, rec *usageCall, prog *progresResposta, onToken, onReason func(string)) (resultatNatiu, error) {
	var res resultatNatiu
	var usage antUsage
	blocs := map[int]*antBlocStream{}
	stopReason := ""
	acabat := false
	var errStream error
	var text strings.Builder
	readSSE(body, func(event, data string) bool {
		if data == "" {
			return true
		}
		prog.tros()
		var ev antStreamEvent
		if json.Unmarshal([]byte(data), &ev) != nil {
			return true
		}
		if ev.Type == "" {
			ev.Type = event
		}
		switch ev.Type {
		case "message_start":
			if ev.Message != nil {
				usage.suma(ev.Message.Usage)
				rec.see(usage.comu())
			}
		case "content_block_start":
			b := &antBlocStream{tipus: ev.ContentBlock.Type, id: ev.ContentBlock.ID, name: ev.ContentBlock.Name, data: ev.ContentBlock.Data}
			b.text.WriteString(ev.ContentBlock.Text)
			b.thinking.WriteString(ev.ContentBlock.Thinking)
			b.signature.WriteString(ev.ContentBlock.Signature)
			if len(ev.ContentBlock.Input) > 0 && string(ev.ContentBlock.Input) != "{}" {
				b.inputIni = ev.ContentBlock.Input
			}
			blocs[ev.Index] = b
			if t := ev.ContentBlock.Text; t != "" {
				text.WriteString(t)
				prog.delta(ProgressText, t)
				if onToken != nil {
					onToken(t)
				}
			}
		case "content_block_delta":
			b := blocs[ev.Index]
			if b == nil {
				b = &antBlocStream{}
				blocs[ev.Index] = b
			}
			switch ev.Delta.Type {
			case "text_delta":
				if b.tipus == "" {
					b.tipus = "text"
				}
				b.text.WriteString(ev.Delta.Text)
				text.WriteString(ev.Delta.Text)
				prog.delta(ProgressText, ev.Delta.Text)
				if onToken != nil && ev.Delta.Text != "" {
					onToken(ev.Delta.Text)
				}
			case "thinking_delta":
				if b.tipus == "" {
					b.tipus = "thinking"
				}
				b.thinking.WriteString(ev.Delta.Thinking)
				res.reasoned += len([]rune(ev.Delta.Thinking))
				prog.delta(ProgressReasoning, ev.Delta.Thinking)
				if onReason != nil && ev.Delta.Thinking != "" {
					onReason(ev.Delta.Thinking)
				}
			case "signature_delta":
				b.signature.WriteString(ev.Delta.Signature)
			case "input_json_delta":
				b.args.WriteString(ev.Delta.PartialJSON)
				prog.delta(ProgressTool, ev.Delta.PartialJSON)
			}
		case "message_delta":
			if ev.Delta.StopReason != "" {
				stopReason = ev.Delta.StopReason
				res.refusal = refusalText(ev.Delta.StopDetails)
			}
			if ev.Usage != nil {
				usage.suma(*ev.Usage)
				rec.see(usage.comu())
			}
		case "message_stop":
			acabat = true
			return false
		case "error":
			msg := "stream error"
			if ev.Error != nil {
				msg = ev.Error.Type + ": " + ev.Error.Message
				if ev.Error.Type == "overloaded_error" || ev.Error.Type == "api_error" {
					errStream = &StreamInterruptedError{}
					return false
				}
			}
			errStream = fmt.Errorf("provider %s: %s", baseURL, msg)
			return false
		}
		return true
	})
	if errStream != nil {
		if err := ctx.Err(); err != nil {
			return res, err
		}
		return res, errStream
	}
	if !acabat && stopReason == "" {
		if err := ctx.Err(); err != nil {
			return res, err
		}
		return res, &StreamInterruptedError{}
	}
	idx := make([]int, 0, len(blocs))
	for i := range blocs {
		idx = append(idx, i)
	}
	sort.Ints(idx)
	for _, i := range idx {
		b := blocs[i]
		switch b.tipus {
		case "thinking", "redacted_thinking":
			res.blocs = append(res.blocs, blocPensament(antRespBlock{Type: b.tipus, Thinking: b.thinking.String(), Signature: b.signature.String(), Data: b.data}))
		case "tool_use":
			args := b.args.String()
			if args == "" && len(b.inputIni) > 0 {
				args = string(b.inputIni)
			}
			res.calls = append(res.calls, antCall(b.id, b.name, args))
		}
	}
	res.text = text.String()
	res.finish = antFinish(stopReason)
	return res, nil
}
