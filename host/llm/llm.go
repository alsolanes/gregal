// Package llm reexporta el client de models de Gregal per a productes externs
// (vegeu gregal/host).
package llm

import illm "gregal/internal/llm"

type (
	Client     = illm.Client
	Message    = illm.Message
	UsageMeter = illm.UsageMeter
)

var (
	IsLocalBase = illm.IsLocalBase
	WithThink   = illm.WithThink
	WithUsage   = illm.WithUsage
)
