package web

import (
	"fmt"
	"log"
	"strings"
	"sync"
)

// promptBlock és una part del system d'un torn de xat amb el seu nom.
type promptBlock struct {
	Name string
	Text string
}

func joinPromptBlocks(blocks []promptBlock) string {
	var b strings.Builder
	for _, p := range blocks {
		b.WriteString(p.Text)
	}
	return b.String()
}

// promptStats és el que pesa el system d'un torn: total, tokens estimats
// (caràcters/4, com llm.EstimateTokens), quant del principi és igual al torn
// anterior del mateix vent (el prefix que un servidor local pot reaprofitar) i
// el bloc que el trenca.
type promptStats struct {
	Chars        int
	Tokens       int
	StableChars  int
	BreaksAt     string
	BlockChars   []promptBlockSize
	PreviousSeen bool
}

type promptBlockSize struct {
	Name  string
	Chars int
}

var promptHistory = struct {
	sync.Mutex
	last map[string]string
}{last: map[string]string{}}

// measurePrompt calcula les mides i les compara amb el system anterior.
func measurePrompt(blocks []promptBlock, previous string, hadPrevious bool) promptStats {
	full := joinPromptBlocks(blocks)
	st := promptStats{Chars: len(full), Tokens: len(full) / 4, PreviousSeen: hadPrevious}
	for _, b := range blocks {
		st.BlockChars = append(st.BlockChars, promptBlockSize{b.Name, len(b.Text)})
	}
	if !hadPrevious {
		return st
	}
	n := 0
	for n < len(full) && n < len(previous) && full[n] == previous[n] {
		n++
	}
	st.StableChars = n
	if n == len(full) && n == len(previous) {
		return st
	}
	pos := 0
	for _, b := range blocks {
		if n < pos+len(b.Text) {
			st.BreaksAt = b.Name
			return st
		}
		pos += len(b.Text)
	}
	return st
}

func (st promptStats) String() string {
	var parts []string
	for _, b := range st.BlockChars {
		if b.Chars > 0 {
			parts = append(parts, fmt.Sprintf("%s=%d", b.Name, b.Chars))
		}
	}
	cache := "first turn"
	if st.PreviousSeen {
		cache = fmt.Sprintf("stable_prefix=%d/%d", st.StableChars, st.Chars)
		if st.BreaksAt != "" {
			cache += " breaks_at=" + st.BreaksAt
		}
	}
	return fmt.Sprintf("total=%d chars (~%d tok) %s | %s", st.Chars, st.Tokens, cache, strings.Join(parts, " "))
}

// recordPromptStats mesura el system del torn, el compara amb l'anterior del
// mateix vent i en deixa una línia al registre.
func recordPromptStats(windID string, blocks []promptBlock) promptStats {
	full := joinPromptBlocks(blocks)
	promptHistory.Lock()
	prev, ok := promptHistory.last[windID]
	promptHistory.last[windID] = full
	promptHistory.Unlock()
	st := measurePrompt(blocks, prev, ok)
	log.Printf("prompt wind=%s %s", windID, st)
	return st
}
