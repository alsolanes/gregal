package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
)

// Proveïdor natiu OpenAI Responses (POST <base>/responses).
//
// Traducció del format comú:
//   - system/user/assistant passen a items de missatge (input_text,
//     input_image, output_text);
//   - les crides d'eina són items function_call (call_id, name, arguments) i
//     els resultats, items function_call_output; les imatges d'un resultat
//     van en un missatge user just després (output només admet text);
//   - els items de raonament es demanen xifrats (include
//     reasoning.encrypted_content, store:false: cap estat al servidor) i es
//     tornen intactes davant de les crides que van produir (blocsNatius).
//
// think: low/medium/high/max → reasoning.effort (max = high); «no» → none;
// buit/auto → sense effort. Sempre es demana el resum del raonament perquè
// arribi a onReason. temperature, reasoning i include són negociables: els
// models que no els accepten els rebutgen amb un 400 i ja no s'envien més.

var campsResponses = []campNatiu{
	{nom: "temperature", claus: []string{"temperature"}},
	{nom: "reasoning", claus: []string{"reasoning.effort", "reasoning.summary", "'reasoning'", "\"reasoning\"", "reasoning is not supported", "effort"}},
	{nom: "include", claus: []string{"include", "encrypted_content"}},
}

type respItem = map[string]any

func respImage(u string) respItem {
	return respItem{"type": "input_image", "image_url": u}
}

func (c *Client) responsesInput(msgs []Message) []respItem {
	msgs = normalitzaOrdreEines(msgs)
	var out []respItem
	pendents := map[string]bool{}
	ordre := []string{}
	tanca := func() {
		for _, id := range ordre {
			if pendents[id] {
				out = append(out, respItem{"type": "function_call_output", "call_id": id, "output": "(no result recorded)"})
			}
		}
		pendents, ordre = map[string]bool{}, nil
	}
	for _, m := range msgs {
		switch m.Role {
		case "system":
			if strings.TrimSpace(m.Content) == "" {
				continue
			}
			tanca()
			out = append(out, respItem{"role": "system", "content": m.Content})
		case "tool":
			if !pendents[m.ToolCallID] {
				parts := []respItem{{"type": "input_text", "text": "Tool result (" + m.ToolCallID + "):\n" + m.Content}}
				for _, img := range m.Images {
					parts = append(parts, respImage(img))
				}
				out = append(out, respItem{"role": "user", "content": parts})
				continue
			}
			pendents[m.ToolCallID] = false
			output := m.Content
			if strings.TrimSpace(output) == "" {
				output = "(empty)"
			}
			out = append(out, respItem{"type": "function_call_output", "call_id": m.ToolCallID, "output": output})
			if len(m.Images) > 0 {
				parts := []respItem{{"type": "input_text", "text": "Images returned by tool call " + m.ToolCallID + ":"}}
				for _, img := range m.Images {
					parts = append(parts, respImage(img))
				}
				// Les imatges no poden anar entre resultats del mateix lot:
				// s'afegeixen quan el lot es tanca (vegeu sota).
				out = append(out, respItem{"role": "user", "content": parts, "_despres": true})
			}
		case "assistant":
			tanca()
			if natius, ok := c.natius.troba(m.ToolCalls, m.Content); ok {
				for _, raw := range natius {
					var it respItem
					if json.Unmarshal(raw, &it) == nil {
						out = append(out, it)
					}
				}
			}
			if txt := senseThink(m.Content); strings.TrimSpace(txt) != "" {
				out = append(out, respItem{"type": "message", "role": "assistant", "content": []respItem{{"type": "output_text", "text": txt}}})
			}
			for _, tc := range m.ToolCalls {
				out = append(out, respItem{"type": "function_call", "call_id": tc.ID, "name": tc.Function.Name, "arguments": string(validArgs(tc.Function.Arguments))})
				if _, ja := pendents[tc.ID]; !ja {
					ordre = append(ordre, tc.ID)
				}
				pendents[tc.ID] = true
			}
		default:
			tanca()
			var parts []respItem
			if strings.TrimSpace(m.Content) != "" {
				parts = append(parts, respItem{"type": "input_text", "text": m.Content})
			}
			for _, img := range m.Images {
				parts = append(parts, respImage(img))
			}
			if len(parts) == 0 {
				continue
			}
			out = append(out, respItem{"role": "user", "content": parts})
		}
	}
	tanca()
	// Mou els missatges d'imatges de resultats darrere del seu lot de
	// function_call_output (l'API vol els resultats seguits de les crides).
	final := make([]respItem, 0, len(out))
	var diferits []respItem
	for _, it := range out {
		if it["_despres"] == true {
			delete(it, "_despres")
			diferits = append(diferits, it)
			continue
		}
		if it["type"] != "function_call_output" && len(diferits) > 0 {
			final = append(final, diferits...)
			diferits = nil
		}
		final = append(final, it)
	}
	return append(final, diferits...)
}

func (c *Client) responsesBody(ctx context.Context, model string, msgs []Message, temp float64, maxTokens int, specs []ToolSpec, stream bool) peticioNativa {
	input := c.responsesInput(msgs)
	mode := thinkMode(ctx)
	effort := ""
	switch mode {
	case "no", "none":
		effort = "none"
	case "low", "medium", "high":
		effort = mode
	case "max":
		effort = "high"
	}
	return func(off map[string]bool) ([]byte, []string, error) {
		var presents []string
		body := map[string]any{"model": model, "input": input, "store": false}
		if stream {
			body["stream"] = true
		}
		if maxTokens > 0 {
			body["max_output_tokens"] = maxTokens
		}
		if temp > 0 && !off["temperature"] {
			body["temperature"] = temp
			presents = append(presents, "temperature")
		}
		if !off["reasoning"] {
			r := map[string]any{"summary": "auto"}
			if effort != "" {
				r["effort"] = effort
			}
			body["reasoning"] = r
			presents = append(presents, "reasoning")
		}
		if !off["include"] {
			body["include"] = []string{"reasoning.encrypted_content"}
			presents = append(presents, "include")
		}
		if len(specs) > 0 {
			var tools []respItem
			for _, s := range specs {
				params := s.Parameters
				if params == nil {
					params = map[string]any{"type": "object", "properties": map[string]any{}}
				}
				tools = append(tools, respItem{"type": "function", "name": s.Name, "description": s.Description, "parameters": params, "strict": false})
			}
			body["tools"] = tools
		}
		raw, err := json.Marshal(body)
		return raw, presents, err
	}
}

type respUsage struct {
	InputTokens        int `json:"input_tokens"`
	OutputTokens       int `json:"output_tokens"`
	TotalTokens        int `json:"total_tokens"`
	InputTokensDetails struct {
		CachedTokens int `json:"cached_tokens"`
	} `json:"input_tokens_details"`
}

func (u respUsage) comu() Usage {
	var out Usage
	out.PromptTokens, out.CompletionTokens, out.TotalTokens = u.InputTokens, u.OutputTokens, u.TotalTokens
	if out.TotalTokens == 0 {
		out.TotalTokens = u.InputTokens + u.OutputTokens
	}
	out.PromptTokensDetails.CachedTokens = u.InputTokensDetails.CachedTokens
	return out
}

type respObject struct {
	Status            string            `json:"status"`
	Output            []json.RawMessage `json:"output"`
	Usage             respUsage         `json:"usage"`
	IncompleteDetails *struct {
		Reason string `json:"reason"`
	} `json:"incomplete_details"`
	Error *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

type respOutItem struct {
	Type      string `json:"type"`
	ID        string `json:"id"`
	CallID    string `json:"call_id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
	Content   []struct {
		Type    string `json:"type"`
		Text    string `json:"text"`
		Refusal string `json:"refusal"`
	} `json:"content"`
	Summary []struct {
		Text string `json:"text"`
	} `json:"summary"`
}

// interpretaItems omple text, crides, raonament i blocs a partir dels items
// finals de la resposta.
func interpretaItems(items []json.RawMessage, res *resultatNatiu, ambText bool) (reason string) {
	var text, rs strings.Builder
	for _, raw := range items {
		var it respOutItem
		if json.Unmarshal(raw, &it) != nil {
			continue
		}
		switch it.Type {
		case "message":
			for _, p := range it.Content {
				if p.Type == "output_text" {
					text.WriteString(p.Text)
				}
				if p.Type == "refusal" && p.Refusal != "" {
					res.finish, res.refusal = "refusal", p.Refusal
				}
			}
		case "reasoning":
			for _, s := range it.Summary {
				rs.WriteString(s.Text)
			}
			res.blocs = append(res.blocs, raw)
		case "function_call":
			id := it.CallID
			if id == "" {
				id = it.ID
			}
			res.calls = append(res.calls, antCall(id, it.Name, it.Arguments))
		}
	}
	if ambText {
		res.text = text.String()
	}
	return rs.String()
}

func respFinish(o respObject, calls int) string {
	if o.Status == "incomplete" {
		if o.IncompleteDetails != nil && o.IncompleteDetails.Reason == "content_filter" {
			return "refusal"
		}
		return "length"
	}
	if calls > 0 {
		return "tool_calls"
	}
	return "stop"
}

func (c *Client) responsesChat(ctx context.Context, baseURL, apiKey, model string, msgs []Message, temp float64, maxTokens int, specs []ToolSpec, stream bool, onToken, onReason func(string)) (string, []ToolCall, error) {
	c.clearUsage()
	headers := func(req *http.Request) {
		setAuth(req, baseURL, apiKey) // Bearer, o api-key a Azure (azure.go)
		if stream {
			req.Header.Set("Accept", "text/event-stream")
		}
	}
	resp, err := c.postNatiu(ctx, baseURL, model, "/responses", hasMalformedToolArguments(msgs), campsResponses, headers, c.responsesBody(ctx, model, msgs, temp, maxTokens, specs, stream))
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
		res, err = responsesStream(ctx, baseURL, resp.Body, rec, prog, onToken, onReason)
	} else {
		res, err = responsesWhole(baseURL, resp.Body, rec, prog, onReason)
		if err == nil && stream && onToken != nil && res.finish != "length" && res.finish != "refusal" && res.text != "" {
			onToken(res.text)
		}
	}
	if err != nil {
		return "", nil, err
	}
	return c.acabaNatiu(res, maxTokens, live && onToken != nil)
}

func responsesWhole(baseURL string, body io.Reader, rec *usageCall, prog *progresResposta, onReason func(string)) (resultatNatiu, error) {
	raw, err := io.ReadAll(body)
	if err != nil {
		return resultatNatiu{}, err
	}
	var o respObject
	if err := json.Unmarshal(raw, &o); err != nil {
		return resultatNatiu{}, fmt.Errorf("resposta no JSON: %w", err)
	}
	if o.Error != nil && o.Error.Message != "" {
		return resultatNatiu{}, fmt.Errorf("provider %s: %s: %s", baseURL, o.Error.Code, o.Error.Message)
	}
	rec.see(o.Usage.comu())
	prog.tros()
	var res resultatNatiu
	reason := interpretaItems(o.Output, &res, true)
	res.reasoned = len([]rune(reason))
	prog.delta(ProgressReasoning, reason)
	prog.delta(ProgressText, res.text)
	for _, tc := range res.calls {
		prog.delta(ProgressTool, tc.Function.Arguments)
	}
	if onReason != nil && reason != "" {
		onReason(reason)
	}
	if res.finish == "" {
		res.finish = respFinish(o, len(res.calls))
	}
	return res, nil
}

type respStreamEvent struct {
	Type        string          `json:"type"`
	Delta       string          `json:"delta"`
	OutputIndex int             `json:"output_index"`
	Item        json.RawMessage `json:"item"`
	Response    *respObject     `json:"response"`
	Code        string          `json:"code"`
	Message     string          `json:"message"`
}

func responsesStream(ctx context.Context, baseURL string, body io.Reader, rec *usageCall, prog *progresResposta, onToken, onReason func(string)) (resultatNatiu, error) {
	var res resultatNatiu
	var text strings.Builder
	items := map[int]json.RawMessage{}
	args := map[int]*strings.Builder{}
	var final *respObject
	var errStream error
	readSSE(body, func(event, data string) bool {
		if data == "" || data == "[DONE]" {
			return data != "[DONE]"
		}
		prog.tros()
		var ev respStreamEvent
		if json.Unmarshal([]byte(data), &ev) != nil {
			return true
		}
		if ev.Type == "" {
			ev.Type = event
		}
		switch ev.Type {
		case "response.output_text.delta":
			text.WriteString(ev.Delta)
			prog.delta(ProgressText, ev.Delta)
			if onToken != nil && ev.Delta != "" {
				onToken(ev.Delta)
			}
		case "response.reasoning_summary_text.delta", "response.reasoning_text.delta":
			res.reasoned += len([]rune(ev.Delta))
			prog.delta(ProgressReasoning, ev.Delta)
			if onReason != nil && ev.Delta != "" {
				onReason(ev.Delta)
			}
		case "response.function_call_arguments.delta":
			b := args[ev.OutputIndex]
			if b == nil {
				b = &strings.Builder{}
				args[ev.OutputIndex] = b
			}
			b.WriteString(ev.Delta)
			prog.delta(ProgressTool, ev.Delta)
		case "response.output_item.added":
			if _, ok := items[ev.OutputIndex]; !ok && len(ev.Item) > 0 {
				items[ev.OutputIndex] = ev.Item
			}
		case "response.output_item.done":
			if len(ev.Item) > 0 {
				items[ev.OutputIndex] = ev.Item
			}
		case "response.completed", "response.incomplete":
			final = ev.Response
			return false
		case "response.failed":
			msg := "response failed"
			if ev.Response != nil && ev.Response.Error != nil {
				msg = ev.Response.Error.Code + ": " + ev.Response.Error.Message
				if ev.Response.Error.Code == "server_error" {
					errStream = &StreamInterruptedError{}
					return false
				}
			}
			errStream = fmt.Errorf("provider %s: %s", baseURL, msg)
			return false
		case "error":
			errStream = fmt.Errorf("provider %s: %s: %s", baseURL, ev.Code, ev.Message)
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
	if final == nil {
		if err := ctx.Err(); err != nil {
			return res, err
		}
		return res, &StreamInterruptedError{}
	}
	rec.see(final.Usage.comu())
	// Els items finals: els de output_item.done (amb els arguments
	// acumulats si l'item no els porta); si no n'hi ha, els de la resposta.
	var llista []json.RawMessage
	if len(items) > 0 {
		idx := make([]int, 0, len(items))
		for i := range items {
			idx = append(idx, i)
		}
		sort.Ints(idx)
		for _, i := range idx {
			raw := items[i]
			if b := args[i]; b != nil {
				var it map[string]any
				if json.Unmarshal(raw, &it) == nil && it["type"] == "function_call" {
					if a, _ := it["arguments"].(string); a == "" {
						it["arguments"] = b.String()
						raw, _ = json.Marshal(it)
					}
				}
			}
			llista = append(llista, raw)
		}
	} else {
		llista = final.Output
	}
	interpretaItems(llista, &res, false)
	res.text = text.String()
	if res.text == "" {
		// Proxy que no envia deltes de text: el text surt dels items.
		var tmp resultatNatiu
		interpretaItems(llista, &tmp, true)
		res.text = tmp.text
		if res.text != "" && onToken != nil && final.Status != "incomplete" {
			onToken(res.text)
		}
	}
	if res.finish == "" {
		res.finish = respFinish(*final, len(res.calls))
	}
	return res, nil
}
