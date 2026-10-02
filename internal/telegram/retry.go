package telegram

import (
	"bytes"
	"io"
	"net/http"
	"strconv"
	"time"
)

// retryTransport reintenta les crides a Telegram que fallen per coses
// passatgeres: 429 (massa crides seguides — el streaming n'és el principal
// culpable) i 5xx. Telegram diu quants segons esperar a
// `parameters.retry_after`; l'honorem, però mai més d'enfora de maxRetryWait,
// i com a molt maxAttempts intents.
//
// Per què una capa de transport i no reintents a cada crida: així val per a
// TOTES les crides (missatges, edicions, getFile, descàrregues) i no hi ha
// cap camí que se n'oblidi. Si un reintent no és possible (cos no replicable)
// o el temps d'espera és absurd, es retorna la resposta tal com ha arribat:
// mai s'amaga un error, només s'hi dona una segona oportunitat.
type retryTransport struct {
	base http.RoundTripper
}

const (
	maxAttempts  = 3
	maxRetryWait = 10 * time.Second
)

func (t retryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	base := t.base
	if base == nil {
		base = http.DefaultTransport
	}
	var body []byte
	if req.Body != nil {
		// Sense GetBody no podem repetir la petició: millor no reintentar que
		// enviar un cos a mitges.
		if req.GetBody == nil {
			return base.RoundTrip(req)
		}
		b, err := io.ReadAll(req.Body)
		req.Body.Close()
		if err != nil {
			return nil, err
		}
		body = b
	}
	for intent := 1; ; intent++ {
		// El cos s'ha de tornar a posar a CADA intent, tambe al primer: si
		// nomes es reposa al segon, la primera peticio surt buida (i Telegram
		// respon 400 «Bad Request», que va ser exactament el que va fer
		// caure el bot en bucle).
		if body != nil {
			rc, err := req.GetBody()
			if err != nil {
				return nil, err
			}
			req.Body = rc
		}
		resp, err := base.RoundTrip(req)
		if err != nil {
			// Error de xarxa: val la pena tornar-hi, amb una espera curta.
			if intent < maxAttempts {
				time.Sleep(time.Duration(intent) * 500 * time.Millisecond)
				continue
			}
			return nil, err
		}
		if !retriable(resp.StatusCode) || intent >= maxAttempts {
			return resp, nil
		}
		wait := retryAfter(resp)
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		time.Sleep(wait)
	}
}

// retriable diu si val la pena tornar-ho a provar.
func retriable(code int) bool {
	return code == http.StatusTooManyRequests || code >= 500
}

// retryAfter llegeix el temps d'espera que demana Telegram (capçalera
// Retry-After o el JSON d'error) i el limita.
func retryAfter(resp *http.Response) time.Duration {
	wait := time.Second
	if v := resp.Header.Get("Retry-After"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			wait = time.Duration(n) * time.Second
		}
	} else if b, err := io.ReadAll(io.LimitReader(resp.Body, 4096)); err == nil {
		if i := bytes.Index(b, []byte(`"retry_after"`)); i >= 0 {
			if n, err := strconv.Atoi(digitsAfter(b[i:])); err == nil && n > 0 {
				wait = time.Duration(n) * time.Second
			}
		}
	}
	if wait > maxRetryWait {
		wait = maxRetryWait
	}
	return wait
}

// digitsAfter extreu el primer nombre que apareix al text.
func digitsAfter(b []byte) string {
	start := -1
	for i, c := range b {
		if c >= '0' && c <= '9' {
			if start < 0 {
				start = i
			}
			continue
		}
		if start >= 0 {
			return string(b[start:i])
		}
	}
	if start >= 0 {
		return string(b[start:])
	}
	return ""
}
