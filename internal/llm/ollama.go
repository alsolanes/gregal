package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Proveïdor natiu Ollama (POST <arrel>/api/chat).
//
// Per què no n'hi ha prou amb l'endpoint OpenAI-compatible (/v1): el
// num_ctx per defecte d'Ollama és de 2k-4k tokens i, quan el prompt no hi
// cap, Ollama en descarta el principi en silenci, és a dir, el system prompt
// i les eines. /v1 no pot enviar num_ctx ni keep_alive. Aquí:
//   - options.num_ctx és fix per (endpoint, model) durant tot el procés:
//     canviar-lo d'una petició a l'altra torna a carregar el model. Es tria
//     una vegada: el del config; si no, el del model si ja està carregat amb
//     prou context (/api/ps, així no es recarrega); si no, 32k (o el màxim
//     del model, si és més petit: /api/show);
//   - keep_alive llarg (30m per defecte): amb els 5 min d'Ollama el model es
//     descarrega entre torns i es perd la caché;
//   - /api/show diu si el model sap cridar eines, pensar o veure imatges.
//
// El cable: missatges amb images (base64 pelat), tool_calls amb arguments
// com a objecte i resultats amb tool_name; el streaming és NDJSON (un
// objecte per línia) i l'últim porta done:true i les estadístiques.

// OllamaDefaultNumCtx és el num_ctx si el config no en diu res i el model
// no està carregat amb prou context. La documentació d'Ollama recomana
// ≥64k per a agents; 32k és el compromís per a màquines de 16-24 GB.
const OllamaDefaultNumCtx = 32768

// OllamaMinNumCtx és el mínim per sota del qual un context ja carregat no
// es reaprofita (i gregal doctor avisa).
const OllamaMinNumCtx = 16384

// OllamaDefaultKeepAlive és el keep_alive si el config no en diu res.
const OllamaDefaultKeepAlive = "30m"

// OllamaModel és el que diu /api/show d'un model.
type OllamaModel struct {
	Capabilities []string
	// ContextLength és el màxim del model (model_info.<arq>.context_length).
	ContextLength int
	// NumCtxParam és el num_ctx del Modelfile, si en té.
	NumCtxParam int
}

// Te diu si el model declara una capacitat (tools, thinking, vision…).
// Sense capacitats (Ollama antic) es dona per bo.
func (m OllamaModel) Te(capacitat string) bool {
	if len(m.Capabilities) == 0 {
		return true
	}
	for _, c := range m.Capabilities {
		if strings.EqualFold(c, capacitat) {
			return true
		}
	}
	return false
}

// ProbeOllamaShow fa POST /api/show per a un model.
func ProbeOllamaShow(ctx context.Context, baseURL, apiKey, model string) (OllamaModel, error) {
	var raw struct {
		Capabilities []string       `json:"capabilities"`
		ModelInfo    map[string]any `json:"model_info"`
		Parameters   string         `json:"parameters"`
	}
	if err := doJSON(ctx, http.MethodPost, serverRoot(baseURL)+"/api/show", apiKey, map[string]any{"model": model}, &raw); err != nil {
		return OllamaModel{}, err
	}
	out := OllamaModel{Capabilities: raw.Capabilities}
	for k, v := range raw.ModelInfo {
		if strings.HasSuffix(k, ".context_length") {
			out.ContextLength = numAny(v)
		}
	}
	for _, l := range strings.Split(raw.Parameters, "\n") {
		f := strings.Fields(l)
		if len(f) == 2 && f[0] == "num_ctx" {
			out.NumCtxParam = numAny(f[1])
		}
	}
	return out, nil
}

// OllamaLoaded és un model carregat segons /api/ps.
type OllamaLoaded struct {
	Name          string
	ContextLength int
	ExpiresAt     string
}

// ProbeOllamaPS fa GET /api/ps: models carregats i amb quin context.
func ProbeOllamaPS(ctx context.Context, baseURL, apiKey string) ([]OllamaLoaded, error) {
	var raw struct {
		Models *[]struct {
			Name          string `json:"name"`
			Model         string `json:"model"`
			ContextLength int    `json:"context_length"`
			ExpiresAt     string `json:"expires_at"`
		} `json:"models"`
	}
	if err := getJSON(ctx, serverRoot(baseURL)+"/api/ps", apiKey, &raw); err != nil {
		return nil, err
	}
	if raw.Models == nil {
		return nil, fmt.Errorf("/api/ps sense models: no és Ollama")
	}
	var out []OllamaLoaded
	for _, m := range *raw.Models {
		n := m.Name
		if n == "" {
			n = m.Model
		}
		out = append(out, OllamaLoaded{Name: n, ContextLength: m.ContextLength, ExpiresAt: m.ExpiresAt})
	}
	return out, nil
}

// OllamaSameModel compara noms d'Ollama: «qwen3» és «qwen3:latest».
func OllamaSameModel(a, b string) bool {
	norm := func(s string) string {
		s = strings.ToLower(strings.TrimSpace(s))
		if !strings.Contains(s, ":") {
			s += ":latest"
		}
		return s
	}
	return norm(a) == norm(b)
}

// ollamaEstat és el que es decideix una sola vegada per (endpoint, model).
type ollamaEstat struct {
	once   sync.Once
	numCtx int
	model  OllamaModel
	show   bool // /api/show ha respost
}

var ollamaEstats sync.Map // normBase#model → *ollamaEstat

// resetOllama oblida les decisions preses (tests).
func resetOllama() { ollamaEstats = sync.Map{} }

func ollamaEstatDe(ctx context.Context, baseURL, apiKey, model string) *ollamaEstat {
	v, _ := ollamaEstats.LoadOrStore(normBase(baseURL)+"#"+strings.TrimSpace(model), &ollamaEstat{})
	e := v.(*ollamaEstat)
	e.once.Do(func() {
		pctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 4*time.Second)
		defer cancel()
		if m, err := ProbeOllamaShow(pctx, baseURL, apiKey, model); err == nil {
			e.model, e.show = m, true
		}
		e.numCtx = triaNumCtx(pctx, baseURL, apiKey, model, e.model)
	})
	return e
}

// triaNumCtx: config > carregat amb prou context > 32k (fitat al màxim del model).
func triaNumCtx(ctx context.Context, baseURL, apiKey, model string, m OllamaModel) int {
	if n := localOptionsFor(baseURL).NumCtx; n > 0 {
		return n
	}
	if ps, err := ProbeOllamaPS(ctx, baseURL, apiKey); err == nil {
		for _, l := range ps {
			if OllamaSameModel(l.Name, model) && l.ContextLength >= OllamaMinNumCtx {
				return l.ContextLength
			}
		}
	}
	n := OllamaDefaultNumCtx
	if m.NumCtxParam > n {
		n = m.NumCtxParam
	}
	if m.ContextLength > 0 && m.ContextLength < n {
		n = m.ContextLength
	}
	return n
}

// OllamaNumCtx torna el num_ctx que Gregal fa servir (o farà servir) amb
// aquest model. La primera crida el decideix; les següents el repeteixen.
func OllamaNumCtx(ctx context.Context, baseURL, apiKey, model string) int {
	return ollamaEstatDe(ctx, baseURL, apiKey, model).numCtx
}

func OllamaKeepAliveFor(baseURL string) any {
	ka := strings.TrimSpace(localOptionsFor(baseURL).KeepAlive)
	if ka == "" {
		return OllamaDefaultKeepAlive
	}
	// Un número pelat són segons (o -1, per sempre): Ollama el vol com a número.
	var n int
	if _, err := fmt.Sscanf(ka, "%d", &n); err == nil && fmt.Sprint(n) == ka {
		return n
	}
	return ka
}

var campsOllama = []campNatiu{
	{nom: "think", claus: []string{"think"}},
}

type ollamaCall struct {
	ID       string `json:"id,omitempty"`
	Function struct {
		Index     *int            `json:"index,omitempty"`
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	} `json:"function"`
}

type ollamaMsg struct {
	Role      string       `json:"role"`
	Content   string       `json:"content"`
	Thinking  string       `json:"thinking,omitempty"`
	Images    []string     `json:"images,omitempty"`
	ToolCalls []ollamaCall `json:"tool_calls,omitempty"`
	ToolName  string       `json:"tool_name,omitempty"`
}

// ollamaImages separa les imatges base64 (les que Ollama accepta) de les
// URL (que no pot baixar: es diuen al text).
func ollamaImages(imgs []string) (b64 []string, nota string) {
	var urls []string
	for _, u := range imgs {
		if _, data, ok := splitDataURL(u); ok {
			b64 = append(b64, data)
		} else if strings.TrimSpace(u) != "" {
			urls = append(urls, u)
		}
	}
	if len(urls) > 0 {
		nota = "\n[image URLs not sent to the local model: " + strings.Join(urls, ", ") + "]"
	}
	return b64, nota
}

func ollamaMessages(msgs []Message) []ollamaMsg {
	msgs = normalitzaOrdreEines(msgs)
	noms := map[string]string{} // id de crida → nom d'eina
	out := make([]ollamaMsg, 0, len(msgs))
	for _, m := range msgs {
		om := ollamaMsg{Role: m.Role, Content: m.Content}
		imgs, nota := ollamaImages(m.Images)
		om.Images, om.Content = imgs, om.Content+nota
		switch m.Role {
		case "assistant":
			om.Content = senseThink(m.Content)
			for _, tc := range m.ToolCalls {
				var oc ollamaCall
				oc.Function.Name = tc.Function.Name
				oc.Function.Arguments = validArgs(tc.Function.Arguments)
				om.ToolCalls = append(om.ToolCalls, oc)
				noms[tc.ID] = tc.Function.Name
			}
		case "tool":
			om.ToolName = noms[m.ToolCallID]
		case "system", "user":
		default:
			om.Role = "user"
		}
		out = append(out, om)
	}
	return out
}

func (c *Client) ollamaBody(ctx context.Context, baseURL, model string, msgs []Message, temp float64, maxTokens int, specs []ToolSpec, stream bool, e *ollamaEstat) peticioNativa {
	conv := ollamaMessages(msgs)
	mode := thinkMode(ctx)
	return func(off map[string]bool) ([]byte, []string, error) {
		var presents []string
		opts := map[string]any{"num_ctx": e.numCtx}
		if temp > 0 {
			opts["temperature"] = temp
		}
		if maxTokens > 0 {
			opts["num_predict"] = maxTokens
		}
		body := map[string]any{
			"model":      model,
			"messages":   conv,
			"stream":     stream,
			"options":    opts,
			"keep_alive": OllamaKeepAliveFor(baseURL),
		}
		if len(specs) > 0 {
			body["tools"] = toolsWire(specs)
		}
		if think, ok := ollamaThink(mode, model); ok && !off["think"] && (!e.show || e.model.Te("thinking")) {
			body["think"] = think
			presents = append(presents, "think")
		}
		raw, err := json.Marshal(body)
		return raw, presents, err
	}
}

// ollamaThink tradueix el mode think del rol al camp think d'Ollama: bool,
// o low/medium/high als models que ho admeten (gpt-oss).
func ollamaThink(mode, model string) (any, bool) {
	gptOSS := strings.Contains(strings.ToLower(model), "gpt-oss")
	switch mode {
	case "no", "none":
		if gptOSS {
			return "low", true // gpt-oss no es pot apagar
		}
		return false, true
	case "low", "medium", "high":
		if gptOSS {
			return mode, true
		}
		return true, true
	case "max":
		if gptOSS {
			return "high", true
		}
		return true, true
	}
	return nil, false
}

type ollamaResp struct {
	Message struct {
		Content   string       `json:"content"`
		Thinking  string       `json:"thinking"`
		ToolCalls []ollamaCall `json:"tool_calls"`
	} `json:"message"`
	Done               bool    `json:"done"`
	DoneReason         string  `json:"done_reason"`
	PromptEvalCount    int     `json:"prompt_eval_count"`
	PromptEvalDuration float64 `json:"prompt_eval_duration"` // ns
	EvalCount          int     `json:"eval_count"`
	EvalDuration       float64 `json:"eval_duration"` // ns
	LoadDuration       float64 `json:"load_duration"` // ns
	Error              string  `json:"error"`
}

var ollamaCallSeq atomic.Int64

func (c *Client) ollamaChat(ctx context.Context, baseURL, apiKey, model string, msgs []Message, temp float64, maxTokens int, specs []ToolSpec, stream bool, onToken, onReason func(string)) (string, []ToolCall, error) {
	c.clearUsage()
	e := ollamaEstatDe(ctx, baseURL, apiKey, model)
	if len(specs) > 0 && e.show && !e.model.Te("tools") {
		return "", nil, fmt.Errorf("provider %s: Ollama says model %q does not support tools (capabilities: %s); pick a model with tool calling", baseURL, model, strings.Join(e.model.Capabilities, ", "))
	}
	headers := func(req *http.Request) {
		if k := strings.TrimSpace(apiKey); k != "" {
			req.Header.Set("Authorization", "Bearer "+k)
		}
	}
	root := serverRoot(baseURL)
	resp, err := c.postNatiu(ctx, root, model, "/api/chat", hasMalformedToolArguments(msgs), campsOllama, headers, c.ollamaBody(ctx, baseURL, model, msgs, temp, maxTokens, specs, stream, e))
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
	var text, reason strings.Builder
	var final *ollamaResp
	dec := json.NewDecoder(resp.Body)
	for {
		var ev ollamaResp
		if err := dec.Decode(&ev); err != nil {
			if err == io.EOF {
				break
			}
			if cerr := ctx.Err(); cerr != nil {
				return "", nil, cerr
			}
			if text.Len() > 0 || reason.Len() > 0 || len(res.calls) > 0 {
				return "", nil, &StreamInterruptedError{}
			}
			return "", nil, fmt.Errorf("resposta no JSON: %w", err)
		}
		prog.tros()
		if ev.Error != "" {
			return "", nil, fmt.Errorf("provider %s: %s", baseURL, ev.Error)
		}
		if t := ev.Message.Thinking; t != "" {
			reason.WriteString(t)
			prog.delta(ProgressReasoning, t)
			if onReason != nil && stream {
				onReason(t)
			}
		}
		if t := ev.Message.Content; t != "" {
			text.WriteString(t)
			prog.delta(ProgressText, t)
			if onToken != nil && stream {
				onToken(t)
			}
		}
		for _, oc := range ev.Message.ToolCalls {
			id := oc.ID
			if id == "" {
				id = fmt.Sprintf("call_ollama_%d", ollamaCallSeq.Add(1))
			}
			args := string(oc.Function.Arguments)
			prog.delta(ProgressTool, args)
			res.calls = append(res.calls, antCall(id, oc.Function.Name, args))
		}
		if ev.Done {
			f := ev
			final = &f
			break
		}
	}
	if final == nil {
		if err := ctx.Err(); err != nil {
			return "", nil, err
		}
		return "", nil, &StreamInterruptedError{}
	}
	rec.local(ollamaTimings(final))
	rec.see(ollamaUsage(final, msgs, specs))
	res.text = text.String()
	res.reasoned = len([]rune(reason.String()))
	if strings.Contains(res.text, "think>") {
		cl, th := extractThinkTags(res.text)
		res.text = cl
		res.reasoned += len([]rune(th))
	}
	if !stream && onReason != nil && reason.Len() > 0 {
		onReason(reason.String())
	}
	switch {
	case final.DoneReason == "length":
		res.finish = "length"
	case len(res.calls) > 0:
		res.finish = "tool_calls"
	default:
		res.finish = "stop"
	}
	return c.acabaNatiu(res, maxTokens, stream && onToken != nil)
}

func ollamaTimings(r *ollamaResp) *LocalTimings {
	t := &LocalTimings{PromptN: r.PromptEvalCount, PredictedN: r.EvalCount, LoadMs: r.LoadDuration / 1e6}
	if r.PromptEvalDuration > 0 {
		t.PromptPerSecond = float64(r.PromptEvalCount) / (r.PromptEvalDuration / 1e9)
	}
	if r.EvalDuration > 0 {
		t.PredictedPerSecond = float64(r.EvalCount) / (r.EvalDuration / 1e9)
	}
	return t
}

// ollamaUsage: prompt_eval_count són els tokens que Ollama ha processat de
// nou; amb la caché encertada no compta el prefix reaprofitat. Si queda
// clarament per sota de l'estimació del prompt, la diferència s'atribueix a
// la caché (estimació: Ollama no ho declara), perquè el comptador i la
// calibració no creguin que el prompt fa cent tokens.
func ollamaUsage(r *ollamaResp, msgs []Message, specs []ToolSpec) Usage {
	var u Usage
	u.PromptTokens = r.PromptEvalCount
	u.CompletionTokens = r.EvalCount
	est := EstimateTokens(msgs)
	if raw, err := json.Marshal(toolsWire(specs)); err == nil && len(specs) > 0 {
		est += len(raw) / 4
	}
	if r.PromptEvalCount > 0 && r.PromptEvalCount < est/2 {
		u.PromptTokensDetails.CachedTokens = est - r.PromptEvalCount
		u.PromptTokens = est
	}
	u.TotalTokens = u.PromptTokens + u.CompletionTokens
	return u
}
