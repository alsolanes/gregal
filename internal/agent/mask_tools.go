package agent

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"gregal/internal/llm"
)

const (
	maskKeepRecentTools = 4
	maskToolMinChars    = 3000
	maskPreviewMaxBytes = 360
	maskPreviewLines    = 3
	maskArchiveMarker   = "[sortida arxivada:"
)

// MaskOldToolOutputs redueix la mida de les sortides d'eina antigues i molt
// llargues que el model ja ha vist en passos anteriors, deixant intactes les
// darreres sortides recents perquè mantingui tot el context immediat.
//
// Això accelera el prefill en torns amb moltes lectures o execucions i evita
// que els models ràpids perdin el focus entre milers de línies antigues.
// No modifica h: torna una còpia només si cal emmascarar algun missatge.
func MaskOldToolOutputs(h []llm.Message) []llm.Message {
	lastAssistant := -1
	for i := len(h) - 1; i >= 0; i-- {
		if h[i].Role == "assistant" {
			lastAssistant = i
			break
		}
	}
	if lastAssistant < 0 {
		return h
	}

	recentCutoff := len(h)
	kept := 0
	for i := len(h) - 1; i >= 0; i-- {
		if h[i].Role == "tool" {
			kept++
			if kept == maskKeepRecentTools {
				recentCutoff = i
				break
			}
		}
	}

	var out []llm.Message
	for i, m := range h {
		if m.Role != "tool" || i >= lastAssistant || i >= recentCutoff {
			continue
		}
		if len(m.Content) <= maskToolMinChars || strings.Contains(m.Content, maskArchiveMarker) {
			continue
		}
		if out == nil {
			out = append([]llm.Message{}, h...)
		}
		lines := strings.Count(m.Content, "\n") + 1
		kb := float64(len(m.Content)) / 1024.0
		preview := toolOutputPreview(m.Content)
		note := fmt.Sprintf("%s %d línies (%.1f KB) ja vistes en passos anteriors; torna a cridar l'eina si et cal el detall]", maskArchiveMarker, lines, kb)
		if preview != "" {
			m.Content = preview + "\n\n" + note
		} else {
			m.Content = note
		}
		out[i] = m
	}
	if out == nil {
		return h
	}
	return out
}

func toolOutputPreview(s string) string {
	var b strings.Builder
	count := 0
	for _, line := range strings.Split(s, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if b.Len() > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(trimmed)
		count++
		if count >= maskPreviewLines || b.Len() >= maskPreviewMaxBytes {
			break
		}
	}
	res := b.String()
	if len(res) > maskPreviewMaxBytes {
		cut := maskPreviewMaxBytes
		for cut > 0 && !utf8.RuneStart(res[cut]) {
			cut--
		}
		res = res[:cut] + "…"
	}
	return res
}
