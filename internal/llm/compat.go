package llm

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"regexp"
	"strings"
)

// Camps opcionals i proveïdors estrictes.
//
// Hi ha camps que només entenen alguns proveïdors: stream_options (el
// consum en streaming), enable_thinking i chat_template_kwargs (l'ordre de
// no pensar dels servidors locals, llama.cpp i el passador de halogen) i
// reasoning_effort (la mateixa ordre a l'API d'OpenAI i a opencode zen).
// Els servidors permissius ignoren el que no coneixen; els estrictes
// (opencode zen, vLLM amb pydantic, alguns proxies) tornen un 400 «unknown
// field». Mesurat el 2026-09-29: zen rebutja enable_thinking,
// chat_template_kwargs i thinking, i accepta reasoning_effort.
//
// postCompat envia tots els camps opcionals que la petició porti i, si el
// proveïdor en rebutja algun, el treu, torna a provar i se'n recorda per a
// aquell endpoint la resta del procés. Un camp opcional mai no pot trencar
// una crida: no saber el consum o que el model pensi és molt millor.
const (
	campStreamOptions   = "stream_options"
	campEnableThinking  = "enable_thinking"
	campTemplateKwargs  = "chat_template_kwargs"
	campReasoningEffort = "reasoning_effort"
	// cache_prompt i id_slot només van a llama-server (llamacpp.go), però
	// un proxy estricte al davant els podria rebutjar.
	campCachePrompt = "cache_prompt"
	campIDSlot      = "id_slot"
	campSampling    = "top_k"
)

var campsOpcionals = []string{campStreamOptions, campEnableThinking, campTemplateKwargs, campReasoningEffort, campCachePrompt, campIDSlot, campSampling}

// postCompat és postRetry amb els camps opcionals negociats (vegeu dalt).
// A llama-server hi afegeix cache_prompt i id_slot; el slot s'allibera quan
// es tanca el cos de la resposta.
func (c *Client) postCompat(ctx context.Context, baseURL, apiKey string, req chatRequest) (*http.Response, error) {
	allibera := c.llamaExtras(ctx, baseURL, apiKey, &req)
	resp, err := c.postCompatNeg(ctx, baseURL, apiKey, req)
	if err != nil || resp == nil || resp.Body == nil {
		allibera()
		return resp, err
	}
	resp.Body = &cosAllibera{ReadCloser: resp.Body, allibera: allibera}
	return resp, nil
}

type cosAllibera struct {
	io.ReadCloser
	allibera func()
}

func (b *cosAllibera) Close() error {
	b.allibera()
	return b.ReadCloser.Close()
}

func (c *Client) postCompatNeg(ctx context.Context, baseURL, apiKey string, req chatRequest) (*http.Response, error) {
	// GLM-5.3 only accepts enabled reasoning. `none` is rejected by the
	// provider, so translate an explicit request to disable thinking into its
	// lowest supported effort before negotiating optional fields.
	if glmForcesReasoning(req.Model) && req.ReasoningEffort == "none" {
		req.ReasoningEffort = "low"
		req.EnableThinking = nil
		req.ChatTemplateKwargs = nil
	}
	base := normBase(baseURL)
	req = c.aplicaAjustos(base, req)
	req = applyProfile(base, req)
	for _, camp := range campsOpcionals {
		if c.campOff(campKey(base, req.Model, camp), camp) {
			req = senseCamp(req, camp)
		}
	}
	for intent := 0; intent < len(campsOpcionals)+len(ajustosModel)+3; intent++ {
		resp, err := c.postRetry(ctx, baseURL, apiKey, req)
		if err != nil || (resp.StatusCode != http.StatusBadRequest && resp.StatusCode != http.StatusUnprocessableEntity) {
			return resp, err
		}
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
		resp.Body.Close()
		// Paràmetres que el model no admet però que tenen equivalent (va
		// abans dels camps opcionals: «unsupported parameter: max_tokens»
		// també semblaria un camp desconegut i se'n traurien uns altres).
		if nou, ajust := ajustaParametres(string(raw), req); ajust != "" {
			req = nou
			c.setCampOff(base+"|"+req.Model, ajust)
			continue
		}
		// reasoning_effort amb un valor que el model no admet: el més proper
		// dels que diu que admet (o «none» si el conflicte és amb les eines).
		if nou, fix, ok := ajustaEsforc(string(raw), req); ok {
			req = nou
			c.setEffortFix(base+"|"+req.Model, fix)
			continue
		}
		rebutjats := campsRebutjats(string(raw), req)
		if len(rebutjats) == 0 {
			// Un 400 d'una altra mena (context massa llarg, model
			// inexistent): es torna tal qual perquè el caller l'interpreti.
			resp.Body = io.NopCloser(bytes.NewReader(raw))
			return resp, nil
		}
		for _, camp := range rebutjats {
			req = senseCamp(req, camp)
			c.setCampOff(campKey(base, req.Model, camp), camp)
		}
	}
	return c.postRetry(ctx, baseURL, apiKey, req)
}

func glmForcesReasoning(model string) bool {
	model = strings.ToLower(strings.TrimSpace(model))
	if i := strings.LastIndexByte(model, '/'); i >= 0 {
		model = model[i+1:]
	}
	switch model {
	case "glm-5.3", "glm-5.3-flash", "glm-5.3-flashx":
		return true
	default:
		return false
	}
}

// campsRebutjats diu quins camps opcionals de la petició ha rebutjat el
// servidor. Si el missatge els anomena, aquells; si només diu que hi ha un
// camp desconegut sense dir quin, tots els opcionals que porta.
func campsRebutjats(body string, req chatRequest) []string {
	b := strings.ToLower(body)
	presents := campsPresents(req)
	var anomenats []string
	for _, camp := range presents {
		if strings.Contains(b, camp) || (camp == campStreamOptions && strings.Contains(b, "include_usage")) || (camp == campSampling && mentionsSampling(b)) {
			anomenats = append(anomenats, camp)
		}
	}
	if len(anomenats) > 0 {
		return anomenats
	}
	if rebutjaCampDesconegut(body) {
		return presents
	}
	return nil
}

func campsPresents(req chatRequest) []string {
	var out []string
	if req.StreamOptions != nil {
		out = append(out, campStreamOptions)
	}
	if req.EnableThinking != nil {
		out = append(out, campEnableThinking)
	}
	if req.TopP != nil || req.TopK != 0 || req.MinP != nil || req.PresencePenalty != nil || req.RepetitionPenalty != nil {
		out = append(out, campSampling)
	}
	if req.ChatTemplateKwargs != nil {
		out = append(out, campTemplateKwargs)
	}
	if req.ReasoningEffort != "" {
		out = append(out, campReasoningEffort)
	}
	if req.CachePrompt != nil {
		out = append(out, campCachePrompt)
	}
	if req.IDSlot != nil {
		out = append(out, campIDSlot)
	}
	return out
}

func senseCamp(req chatRequest, camp string) chatRequest {
	switch camp {
	case campStreamOptions:
		req.StreamOptions = nil
	case campEnableThinking:
		req.EnableThinking = nil
	case campTemplateKwargs:
		req.ChatTemplateKwargs = nil
	case campReasoningEffort:
		req.ReasoningEffort = ""
	case campCachePrompt:
		req.CachePrompt = nil
	case campIDSlot:
		req.IDSlot = nil
	case campSampling:
		req.TopP, req.TopK, req.MinP, req.PresencePenalty, req.RepetitionPenalty, req.RepeatPenalty = nil, 0, nil, nil, nil, nil
	}
	return req
}

func normBase(baseURL string) string {
	return strings.TrimSuffix(strings.TrimSpace(baseURL), "/")
}

func (c *Client) campOff(base, camp string) bool {
	c.usageMu.Lock()
	defer c.usageMu.Unlock()
	return c.campsOff[base+"|"+camp]
}

func (c *Client) setCampOff(base, camp string) {
	c.usageMu.Lock()
	defer c.usageMu.Unlock()
	if c.campsOff == nil {
		c.campsOff = map[string]bool{}
	}
	c.campsOff[base+"|"+camp] = true
}

// reasoningEffort tradueix think a la intensitat de raonament OpenAI.
func (c *Client) reasoningEffort(ctx context.Context) string {
	v, ok := ctx.Value(thinkKey{}).(string)
	if !ok {
		return ""
	}
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "no", "none":
		return "none"
	case "low", "medium", "high", "max":
		return strings.ToLower(strings.TrimSpace(v))
	default: // Buit, auto i valors antics/desconeguts: comportament del model.
		return ""
	}
}

// Ajustos per model: paràmetres estàndard que alguns models rebutgen i que
// tenen una alternativa. Vist el 2026-10-14 amb Azure OpenAI: «Unsupported
// parameter: 'max_tokens' is not supported with this model. Use
// 'max_completion_tokens' instead.» Els models de raonament d'OpenAI/Azure
// (o-series, gpt-5…) tampoc no accepten una temperatura que no sigui la de
// per defecte. Es descobreix amb el primer 400 i es recorda per a aquell
// endpoint i model la resta del procés.
const (
	ajustMaxCompletion = "max_completion_tokens"
	ajustSenseTemp     = "no_temperature"
)

var ajustosModel = []string{ajustMaxCompletion, ajustSenseTemp}

// ajustaParametres torna la petició corregida i quin ajust ha calgut, o ""
// si el 400 no és d'aquesta mena (o la petició ja estava ajustada).
func ajustaParametres(body string, req chatRequest) (chatRequest, string) {
	b := strings.ToLower(body)
	if req.MaxTokens > 0 && strings.Contains(b, "max_tokens") && strings.Contains(b, "max_completion_tokens") {
		return ambAjust(req, ajustMaxCompletion), ajustMaxCompletion
	}
	if req.Temperature != 0 && strings.Contains(b, "temperature") &&
		(strings.Contains(b, "unsupported") || strings.Contains(b, "not support") || strings.Contains(b, "only the default")) {
		return ambAjust(req, ajustSenseTemp), ajustSenseTemp
	}
	return req, ""
}

func ambAjust(req chatRequest, ajust string) chatRequest {
	switch ajust {
	case ajustMaxCompletion:
		if req.MaxTokens > 0 {
			req.MaxCompletionTokens, req.MaxTokens = req.MaxTokens, 0
		}
	case ajustSenseTemp:
		req.Temperature = 0 // omitempty: el model fa servir la seva
	}
	return req
}

// aplicaAjustos aplica els ajustos ja descoberts per a aquest endpoint i model.
func (c *Client) aplicaAjustos(base string, req chatRequest) chatRequest {
	for _, a := range ajustosModel {
		if c.campOff(base+"|"+req.Model, a) {
			req = ambAjust(req, a)
		}
	}
	return ambEsforc(req, c.effortFix(base+"|"+req.Model))
}

// campKey: reasoning_effort depèn del model (a Foundry, la mateixa URL
// serveix models que l'admeten i models que no); els altres camps opcionals
// depenen del servidor.
func campKey(base, model, camp string) string {
	if camp == campReasoningEffort {
		return base + "|" + model
	}
	return base
}

// Esforç de raonament per model (Azure OpenAI, documentació 2026-10-14):
// none (gpt-5.1 i posteriors), minimal (només el gpt-5 original), low,
// medium, high, xhigh (gpt-5.4+ i codex-max), max (GPT-6/5.6, només a
// Responses). o1-mini no l'admet. I els gpt-5.6 a Chat Completions no
// combinen raonament i eines: amb tools, només «none» (ni tan sols val no
// enviar-lo, perquè per defecte és medium).
var nivellsEsforc = []string{"none", "minimal", "low", "medium", "high", "xhigh", "max"}

const esforcEinesNone = "tools:none"

var reValorsAdmesos = regexp.MustCompile(`'([a-z]+)'`)

// ajustaEsforc interpreta un 400 sobre reasoning_effort. Torna la petició
// corregida i l'ajust a recordar: un valor, o esforcEinesNone.
func ajustaEsforc(body string, req chatRequest) (chatRequest, string, bool) {
	b := strings.ToLower(body)
	if !strings.Contains(b, "reasoning_effort") {
		return req, "", false
	}
	// «Function tools with reasoning_effort are not supported for gpt-5.6…
	// set reasoning_effort to 'none'.»
	if len(req.Tools) > 0 && strings.Contains(b, "tools") && strings.Contains(b, "'none'") && req.ReasoningEffort != "none" {
		return ambEsforc(req, esforcEinesNone), esforcEinesNone, true
	}
	// «Unsupported value: 'reasoning_effort' does not support 'max' with
	// this model. Supported values are: 'low', 'medium', and 'high'.»
	i := strings.Index(b, "supported values")
	if req.ReasoningEffort == "" || i < 0 {
		return req, "", false
	}
	var admesos []string
	for _, m := range reValorsAdmesos.FindAllStringSubmatch(b[i:], -1) {
		if posEsforc(m[1]) >= 0 {
			admesos = append(admesos, m[1])
		}
	}
	tria := esforcMesProper(req.ReasoningEffort, admesos)
	if tria == "" || tria == req.ReasoningEffort {
		return req, "", false
	}
	req.ReasoningEffort = tria
	return req, tria, true
}

func posEsforc(v string) int {
	for i, n := range nivellsEsforc {
		if n == v {
			return i
		}
	}
	return -1
}

// esforcMesProper tria l'admès més proper al demanat; en cas d'empat, el
// més alt (qui demana raonament el vol, més que no pas menys).
func esforcMesProper(volgut string, admesos []string) string {
	p := posEsforc(volgut)
	millor, dist := "", 1<<30
	for _, a := range admesos {
		d := posEsforc(a) - p
		if d < 0 {
			d = -d
		}
		if d < dist || (d == dist && posEsforc(a) > posEsforc(millor)) {
			millor, dist = a, d
		}
	}
	return millor
}

// ambEsforc aplica un ajust d'esforç ja conegut a la petició.
func ambEsforc(req chatRequest, fix string) chatRequest {
	switch {
	case fix == "":
	case fix == esforcEinesNone:
		if len(req.Tools) > 0 {
			req.ReasoningEffort = "none"
		}
	case req.ReasoningEffort != "":
		req.ReasoningEffort = fix
	}
	return req
}

func (c *Client) effortFix(key string) string {
	c.usageMu.Lock()
	defer c.usageMu.Unlock()
	return c.effortFixes[key]
}

func (c *Client) setEffortFix(key, fix string) {
	c.usageMu.Lock()
	defer c.usageMu.Unlock()
	if c.effortFixes == nil {
		c.effortFixes = map[string]string{}
	}
	c.effortFixes[key] = fix
}

// mentionsSampling diu si un error parla d'algun camp de mostreig.
func mentionsSampling(b string) bool {
	for _, k := range []string{"top_p", "top_k", "min_p", "presence_penalty", "repetition_penalty", "repeat_penalty"} {
		if strings.Contains(b, k) {
			return true
		}
	}
	return false
}
