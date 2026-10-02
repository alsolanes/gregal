package llm

import (
	"bytes"
	"context"
	"io"
	"net/http"
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
)

var campsOpcionals = []string{campStreamOptions, campEnableThinking, campTemplateKwargs, campReasoningEffort}

// postCompat és postRetry amb els camps opcionals negociats (vegeu dalt).
func (c *Client) postCompat(ctx context.Context, baseURL, apiKey string, req chatRequest) (*http.Response, error) {
	base := normBase(baseURL)
	for _, camp := range campsOpcionals {
		if c.campOff(base, camp) {
			req = senseCamp(req, camp)
		}
	}
	for intent := 0; intent < len(campsOpcionals)+1; intent++ {
		resp, err := c.postRetry(ctx, baseURL, apiKey, req)
		if err != nil || (resp.StatusCode != http.StatusBadRequest && resp.StatusCode != http.StatusUnprocessableEntity) {
			return resp, err
		}
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
		resp.Body.Close()
		rebutjats := campsRebutjats(string(raw), req)
		if len(rebutjats) == 0 {
			// Un 400 d'una altra mena (context massa llarg, model
			// inexistent): es torna tal qual perquè el caller l'interpreti.
			resp.Body = io.NopCloser(bytes.NewReader(raw))
			return resp, nil
		}
		for _, camp := range rebutjats {
			req = senseCamp(req, camp)
			c.setCampOff(base, camp)
		}
	}
	return c.postRetry(ctx, baseURL, apiKey, req)
}

// campsRebutjats diu quins camps opcionals de la petició ha rebutjat el
// servidor. Si el missatge els anomena, aquells; si només diu que hi ha un
// camp desconegut sense dir quin, tots els opcionals que porta.
func campsRebutjats(body string, req chatRequest) []string {
	b := strings.ToLower(body)
	presents := campsPresents(req)
	var anomenats []string
	for _, camp := range presents {
		if strings.Contains(b, camp) || (camp == campStreamOptions && strings.Contains(b, "include_usage")) {
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
	if req.ChatTemplateKwargs != nil {
		out = append(out, campTemplateKwargs)
	}
	if req.ReasoningEffort != "" {
		out = append(out, campReasoningEffort)
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

// reasoningEffort és l'ordre de no pensar per a les API que segueixen
// OpenAI (zen l'accepta; «none» hi va passar el raonament d'uns 2.000-
// 10.000 caràcters a 362).
func (c *Client) reasoningEffort(ctx context.Context) string {
	if v, ok := ctx.Value(thinkKey{}).(string); ok && v == "no" {
		return "none"
	}
	return ""
}
