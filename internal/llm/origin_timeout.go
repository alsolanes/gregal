package llm

import (
	"sync"
)

// Proxies que tallen les respostes lentes (HTTP 524).
//
// llm.solanes.xyz (el qwen flash local, ~38 tokens/s) és darrere de
// Cloudflare, que torna 524 si l'origen no envia la resposta en ~100 s.
// Una crida SENSE streaming no envia res fins que el model acaba: tota
// resposta de més de ~3.800 tokens (o un prompt gran amb una de mitjana)
// hi topa sempre. Abans es tractava com un 5xx transitori: sis intents de
// 100-125 s cadascun (uns 11 minuts en silenci, el «pas que no torna mai»
// del 2026-10-28) i el motor encara hi tornava dues rondes més, mentre el
// servidor seguia generant les respostes abandonades. Reintentar la
// mateixa petició no ho arregla mai.
//
// Ara un 524 no es reintenta: les crides sense streaming d'aquell endpoint
// passen a fer-se en streaming (els bytes arriben de seguida i el proxy no
// talla) i se'n recorda per a la resta del procés.

// OriginTimeoutError és un 524: el proxy ha tallat esperant l'origen.
// L'error porta el text «HTTP 524» de sempre (IsServerError, IsRetryable
// per al fallback).
type OriginTimeoutError struct{ Err error }

func (e *OriginTimeoutError) Error() string { return e.Err.Error() }
func (e *OriginTimeoutError) Unwrap() error { return e.Err }

var nomesStream sync.Map // normBase(baseURL) → true

// streamObligat diu si l'endpoint ja ha tallat una crida sense streaming.
func streamObligat(baseURL string) bool {
	_, ok := nomesStream.Load(normBase(baseURL))
	return ok
}

func marcaStreamObligat(baseURL string) {
	nomesStream.Store(normBase(baseURL), true)
}

// oblidaStreamObligat és per als tests.
func oblidaStreamObligat() {
	nomesStream.Range(func(k, _ any) bool {
		nomesStream.Delete(k)
		return true
	})
}
