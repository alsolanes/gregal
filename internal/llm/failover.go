package llm

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// Target és un endpoint concret (provider + model).
type Target struct {
	BaseURL string
	APIKey  string
	Model   string
}

// IsRetryable diu si l'error justifica provar el fallback: errors de xarxa,
// 429 i 5xx. Els 400/401/403/404 són errors de configuració: reintentar
// amb un altre provider els amagaria, així que fallen directe.
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}
	var lengthErr *LengthError
	var streamErr *StreamInterruptedError
	if errors.As(err, &lengthErr) || errors.As(err, &streamErr) {
		return false
	}
	msg := err.Error()
	if strings.Contains(msg, "HTTP 429") || strings.Contains(msg, "HTTP 5") {
		return true
	}
	if strings.Contains(msg, "HTTP 400") || strings.Contains(msg, "HTTP 401") ||
		strings.Contains(msg, "HTTP 403") || strings.Contains(msg, "HTTP 404") {
		return false
	}
	// Error de xarxa (connection refused, timeout, DNS…): el missatge no
	// és "provider %s: HTTP N" sinó l'error cru.
	if strings.HasPrefix(msg, "provider ") && strings.Contains(msg, "HTTP") {
		return false
	}
	return true
}

// ChatWithToolsFO prova prim i, si falla amb error recuperable i hi ha
// fallback, ho torna a provar allà. Torna usedFb=true si ha respost el
// fallback (cal mostrar-ho: la resposta ve d'un altre model); a més crida
// notify(model) en aquell moment perquè cada front-end ho anunciï a la
// seva manera (nil = sense avís).
func (c *Client) ChatWithToolsFO(ctx context.Context, prim Target, fb *Target, msgs []Message, temp float64, maxTokens int, specs []ToolSpec, notify func(model string)) (string, []ToolCall, bool, error) {
	content, calls, err := c.ChatWithTools(ctx, prim.BaseURL, prim.APIKey, prim.Model, msgs, temp, maxTokens, specs)
	if err == nil || fb == nil || !IsRetryable(err) {
		return content, calls, false, err
	}
	content2, calls2, err2 := c.ChatWithTools(ctx, fb.BaseURL, fb.APIKey, fb.Model, msgs, temp, maxTokens, specs)
	if err2 != nil {
		return "", nil, false, fmt.Errorf("primari: %v; fallback %s: %v", err, fb.Model, err2)
	}
	if notify != nil {
		notify(fb.Model)
	}
	return content2, calls2, true, nil
}

// ChatStreamWithToolsFO és ChatWithToolsFO amb streaming. Si el primari
// falla a mig stream, els tokens ja emesos no es poden retirar: el fallback
// només s'intenta quan encara no n'ha sortit cap (els errors recuperables
// —429, 5xx, xarxa— arriben quasi sempre abans del cos).
func (c *Client) ChatStreamWithToolsFO(ctx context.Context, prim Target, fb *Target, msgs []Message, temp float64, maxTokens int, specs []ToolSpec, onToken, onReason func(string), notify func(model string)) (string, []ToolCall, bool, error) {
	emitted := false
	wrap := func(t string) {
		emitted = true
		if onToken != nil {
			onToken(t)
		}
	}
	content, calls, err := c.ChatStreamWithTools(ctx, prim.BaseURL, prim.APIKey, prim.Model, msgs, temp, maxTokens, specs, wrap, onReason)
	if err == nil || fb == nil || emitted || !IsRetryable(err) {
		return content, calls, false, err
	}
	content2, calls2, err2 := c.ChatStreamWithTools(ctx, fb.BaseURL, fb.APIKey, fb.Model, msgs, temp, maxTokens, specs, onToken, onReason)
	if err2 != nil {
		return "", nil, false, fmt.Errorf("primari: %v; fallback %s: %v", err, fb.Model, err2)
	}
	if notify != nil {
		notify(fb.Model)
	}
	return content2, calls2, true, nil
}

// ChatFO és el mateix per a crides sense eines.
func (c *Client) ChatFO(ctx context.Context, prim Target, fb *Target, msgs []Message, temp float64, maxTokens int, notify func(model string)) (string, bool, error) {
	out, err := c.Chat(ctx, prim.BaseURL, prim.APIKey, prim.Model, msgs, temp, maxTokens)
	if err == nil || fb == nil || !IsRetryable(err) {
		return out, false, err
	}
	out2, err2 := c.Chat(ctx, fb.BaseURL, fb.APIKey, fb.Model, msgs, temp, maxTokens)
	if err2 != nil {
		return "", false, fmt.Errorf("primari: %v; fallback %s: %v", err, fb.Model, err2)
	}
	if notify != nil {
		notify(fb.Model)
	}
	return out2, true, nil
}
