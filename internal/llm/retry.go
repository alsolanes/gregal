package llm

import (
	"context"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Retry: quants intents per crida (1 inicial + reintents) i esperes.
// S'aplica a totes les crides (Chat, ChatWithTools, ChatStream abans del
// primer byte): només es reintenten errors de xarxa i HTTP 429/5xx.
// Els 400/401/403/404 fallen directe (configuració, no transient).
//
// Les esperes són exponencials des d'un segon fins a un topall de vint
// (1, 2, 4, 8, 16, 20 → uns cinquanta segons en total). Abans eren tres
// intents amb mig segon: bé per a un tall de xarxa, inútil amb un model
// que arrenca sota demanda i torna 502 durant un minut (vist en viu amb
// localhost). Un 429 amb Retry-After s'espera el que digui el
// servidor (fins a 30 s). Qui crida pot saber que s'està reintentant amb
// WithRetryHook: el TUI i la finestra ho diuen a la barra en comptes de
// quedar-se muts.
const RetryAttempts = 6

// Esperes com a variables perquè els tests les escurcin.
var (
	RetryBase   = 1 * time.Second
	RetryMax    = 20 * time.Second
	RetryJitter = 400 * time.Millisecond
)

// RetryHook rep cada reintent: intent que ha fallat, total d'intents,
// espera fins al següent i l'error.
type RetryHook func(attempt, total int, wait time.Duration, err error)

type retryHookKey struct{}

// WithRetryHook adjunta al context un avisador de reintents.
func WithRetryHook(ctx context.Context, fn RetryHook) context.Context {
	if fn == nil {
		return ctx
	}
	return context.WithValue(ctx, retryHookKey{}, fn)
}

func retryHookFrom(ctx context.Context) RetryHook {
	if fn, ok := ctx.Value(retryHookKey{}).(RetryHook); ok {
		return fn
	}
	return nil
}

// backoffFor calcula l'espera abans de l'intent següent.
func backoffFor(attempt int, resp *http.Response) time.Duration {
	if resp != nil && resp.StatusCode == 429 {
		if ra := resp.Header.Get("Retry-After"); ra != "" {
			if s, err := strconv.Atoi(ra); err == nil && s > 0 {
				if s > 30 {
					s = 30
				}
				return time.Duration(s) * time.Second
			}
		}
	}
	b := RetryBase * time.Duration(1<<(attempt-1))
	if b > RetryMax {
		b = RetryMax
	}
	return b + time.Duration(rand.Int63n(int64(RetryJitter)))
}

// postRetry fa POST amb reintents. Torna la resposta amb 2xx o amb un 4xx
// definitiu (el caller n'extreu l'error); els 429/5xx esgotats tornen error.
func (c *Client) postRetry(ctx context.Context, baseURL, apiKey string, reqBody chatRequest) (*http.Response, error) {
	var last error
	hook := retryHookFrom(ctx)
	for attempt := 1; attempt <= RetryAttempts; attempt++ {
		resp, err := c.post(ctx, baseURL, apiKey, reqBody)
		var failed *http.Response
		if err != nil {
			last = fmt.Errorf("provider %s: %w", baseURL, err)
			if ctx.Err() != nil {
				return nil, last
			}
		} else if resp.StatusCode == 429 || resp.StatusCode >= 500 {
			raw, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
			resp.Body.Close()
			failed = resp
			last = fmt.Errorf("provider %s: HTTP %d: %s%s", baseURL, resp.StatusCode, truncate(string(raw), 2000), zenHint(baseURL, resp.StatusCode, string(raw)))
			// Un error de parseig dels arguments és determinista: repetir exactament
			// el mateix historial només allarga el torn i pot arribar a dos minuts.
			if isPermanentProviderError(string(raw)) && hasMalformedToolArguments(reqBody.Messages) {
				return nil, last
			}
		} else {
			return resp, nil
		}
		if attempt < RetryAttempts {
			backoff := backoffFor(attempt, failed)
			if hook != nil {
				hook(attempt, RetryAttempts, backoff, last)
			}
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
		}
	}
	return nil, last
}

// isPermanentProviderError identifica errors que el proxy etiqueta com a 5xx
// però que no es poden arreglar reintentant la mateixa petició. En particular,
// alguns models deixen a mig escriure function.arguments quan una eina conté
// un heredoc o una cadena amb salts de línia.
func isPermanentProviderError(body string) bool {
	lower := strings.ToLower(body)
	return strings.Contains(lower, "failed to parse tool call arguments")
}

// RetryNote és el text curt per a una barra d'estat: «proveïdor HTTP 502 ·
// reintent 2/6 d'aquí a 4 s».
func RetryNote(attempt, total int, wait time.Duration, err error) string {
	motiu := "sense resposta"
	if err != nil {
		msg := err.Error()
		if i := indexHTTP(msg); i >= 0 && i+8 <= len(msg) {
			motiu = msg[i : i+8]
		} else if len(msg) > 40 {
			motiu = msg[:40] + "…"
		} else {
			motiu = msg
		}
	}
	return fmt.Sprintf("proveïdor %s · reintent %d/%d d'aquí a %ds", motiu, attempt+1, total, int(wait.Round(time.Second)/time.Second))
}

func indexHTTP(s string) int {
	for i := 0; i+4 <= len(s); i++ {
		if s[i:i+4] == "HTTP" {
			return i
		}
	}
	return -1
}
