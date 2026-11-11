package agent

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"

	"gregal/internal/llm"
)

// Eines MCP del gestor global per compte (B5, auditoria del token store):
// el gestor és del procés i els seus servidors remots porten el token OAuth
// de l'administrador. En un servidor amb diversos comptes, el servidor web
// posa al context del torn quines d'aquestes eines pot fer servir aquest
// compte; la resta ni surten a les specs ni s'executen.

type extrasGateKey struct{}

// ErrExtraNotForAccount: l'eina és d'un connector d'un altre compte.
var ErrExtraNotForAccount = errors.New("this connector belongs to the server administrator's account; connect your own account in Connections")

// WithExtrasGate lliga al torn la funció que diu quines eines MCP globals
// pot fer servir aquest compte. nil = totes (un sol usuari, l'admin).
func WithExtrasGate(ctx context.Context, allow func(name string) bool) context.Context {
	if allow == nil {
		return ctx
	}
	return context.WithValue(ctx, extrasGateKey{}, allow)
}

// sessionGates: el mateix filtre per sessió (el servidor web el posa quan
// crea la sessió d'un compte; totes les eines d'aquesta sessió hi passen).
var sessionGates sync.Map // sessió → func(string) bool

// SetSessionExtrasGate fixa (o treu, amb nil) el filtre d'una sessió.
func SetSessionExtrasGate(session string, allow func(name string) bool) {
	if allow == nil {
		sessionGates.Delete(session)
		return
	}
	sessionGates.Store(session, allow)
}

func gateOf(ctx context.Context, session string) func(string) bool {
	if ctx != nil {
		if allow, _ := ctx.Value(extrasGateKey{}).(func(string) bool); allow != nil {
			return allow
		}
	}
	if session != "" {
		if v, ok := sessionGates.Load(session); ok {
			return v.(func(string) bool)
		}
	}
	return nil
}

// ExtraAllowed diu si el torn (ctx o sessió) pot fer servir l'eina MCP
// global name.
func ExtraAllowed(ctx context.Context, session, name string) bool {
	if !isMCP(name) {
		return true
	}
	allow := gateOf(ctx, session)
	return allow == nil || allow(name)
}

// GateSpecs treu de specs les eines MCP globals que el torn no pot fer servir.
func GateSpecs(ctx context.Context, session string, specs []llm.ToolSpec) []llm.ToolSpec {
	allow := gateOf(ctx, session)
	if allow == nil {
		return specs
	}
	out := make([]llm.ToolSpec, 0, len(specs))
	for _, s := range specs {
		if isMCP(s.Name) && !allow(s.Name) {
			continue
		}
		out = append(out, s)
	}
	return out
}

// ConnectorAction és l'acció d'una crida a l'eina connector (minúscules).
func ConnectorAction(argsJSON string) string {
	var a struct {
		Action string `json:"action"`
	}
	_ = json.Unmarshal([]byte(argsJSON), &a)
	return strings.ToLower(strings.TrimSpace(a.Action))
}
