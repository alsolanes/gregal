package agent

import (
	"sync"

	"gregal/internal/llm"
)

// readOnlyParallel és l'allowlist d'eines paral·lelitzables: només lectura
// pura, sense escriptura ni interacció. Són les mateixes que la policy marca
// com a allow (loop.go) menys delegate (fan-out de subagents: recursió i
// recursos propis, mai dins d'un paral·lel) i question (interactiva).
// bash/edit/write/patch/todowrite/office_* (excepte office_read),
// bash_background/bash_output/bash_kill són mutables o amb efectes.
var readOnlyParallel = map[string]bool{
	"read":        true,
	"grep":        true,
	"glob":        true,
	"todoread":    true,
	"web_search":  true,
	"web_fetch":   true,
	"read_image":  true,
	"office_read": true,
	"gh_issue":    true,
	"gh_pr":       true,
}

// allReadOnlyCalls diu si TOTES les crides del bloc són read-only (i n'hi ha
// alguna). Només aleshores el bloc pot executar-se en paral·lel.
func allReadOnlyCalls(calls []llm.ToolCall) bool {
	if len(calls) == 0 {
		return false
	}
	for _, c := range calls {
		if !readOnlyParallel[c.Function.Name] {
			return false
		}
	}
	return true
}

// execFn executa una eina: (sortida, imatges, error).
type execFn func(name, argsJSON string) (string, []string, error)

// execParallel executa totes les crides amb exec en goroutines (màxim 4
// concurrents) i retorna sortides, imatges i errors en l'ordre original.
// Els errors no avorten: es retornen per índex i qui crida els converteix en
// "ERROR: ..." com en la via seqüencial.
func execParallel(calls []llm.ToolCall, exec execFn) ([]string, [][]string, []error) {
	n := len(calls)
	outs := make([]string, n)
	allImgs := make([][]string, n)
	errs := make([]error, n)
	sem := make(chan struct{}, 4)
	var wg sync.WaitGroup
	for i, c := range calls {
		wg.Add(1)
		go func(i int, c llm.ToolCall) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			o, imgs, err := exec(c.Function.Name, c.Function.Arguments)
			outs[i], allImgs[i], errs[i] = o, imgs, err
		}(i, c)
	}
	wg.Wait()
	return outs, allImgs, errs
}
