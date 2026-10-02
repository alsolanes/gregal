package agent

// Execució en paral·lel de les eines d'un mateix pas.
//
// Quan el model demana quatre lectures i dues cerques web en un sol pas,
// no hi ha cap motiu per fer-les una darrere l'altra: cap escriu res, i
// la més lenta (una pàgina web) marcava el temps de totes. Aquí les eines
// de només lectura que van seguides s'executen alhora (fins a
// MaxParallelTools) i les que modifiquen l'estat (write, edit, patch,
// bash, office_*, MCP) es fan d'una en una i en l'ordre que el model les
// ha demanat. Els resultats tornen sempre en l'ordre original: l'historial
// que veu el model no canvia.
//
// Ho fan servir el TUI, el headless (-p), el backend web/escriptori, el
// Loop de proves i els subagents.

import (
	"sync"

	"gregal/internal/llm"
)

// MaxParallelTools és el màxim d'eines de lectura alhora dins d'un pas.
const MaxParallelTools = 6

// ParallelSafe diu si una eina es pot executar alhora amb d'altres del
// mateix pas: no escriu al disc ni engega processos. bash no hi és encara
// que sigui de lectura (ClassifyWith) perquè un `go test` i un `go build`
// alhora es trepitgen la cache.
func ParallelSafe(name string) bool {
	switch name {
	case "read", "grep", "glob", "web_search", "web_fetch", "gh_issue", "gh_pr",
		"office_read", "read_image", "todoread", "bash_output", "delegate":
		return true
	}
	return false
}

// RunCalls executa les crides: les seguides que són ParallelSafe alhora,
// la resta seqüencialment; torna els resultats en l'ordre de calls.
// skip(i) == true vol dir que la crida i no s'executa (ja té resposta:
// bloquejada, repetida, ajornada) i el seu resultat queda al zero value.
func RunCalls[R any](calls []llm.ToolCall, skip func(i int) bool, run func(i int, c llm.ToolCall) R) []R {
	out := make([]R, len(calls))
	salta := func(i int) bool { return skip != nil && skip(i) }
	i := 0
	for i < len(calls) {
		c := calls[i]
		if salta(i) {
			i++
			continue
		}
		if !ParallelSafe(c.Function.Name) {
			out[i] = run(i, c)
			i++
			continue
		}
		// Lot de crides segures seguides (saltant les que no s'executen).
		j := i
		var lot []int
		for j < len(calls) && (salta(j) || ParallelSafe(calls[j].Function.Name)) {
			if !salta(j) {
				lot = append(lot, j)
			}
			j++
		}
		if len(lot) == 1 {
			out[lot[0]] = run(lot[0], calls[lot[0]])
		} else {
			var wg sync.WaitGroup
			sem := make(chan struct{}, MaxParallelTools)
			for _, k := range lot {
				wg.Add(1)
				sem <- struct{}{}
				go func(k int) {
					defer wg.Done()
					defer func() { <-sem }()
					out[k] = run(k, calls[k])
				}(k)
			}
			wg.Wait()
		}
		i = j
	}
	return out
}
