package agent

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"gregal/internal/config"
	"gregal/internal/llm"
)

// CompactThreshold és la fracció de finestra a partir de la qual es compacta.
// El 25% restant és el marge d'error d'EstimateTokens (llm/client.go): mai
// compacteu al 100% amb una estimació heurística.
const CompactThreshold = 0.75

// DefaultWindow quan el rol no en declara (els locals són 32K natius).
const DefaultWindow = 32768

// NeedsCompact diu si l'estimació supera el llindar de la finestra.
func NeedsCompact(estTokens, window int) bool {
	if window <= 0 {
		window = DefaultWindow
	}
	return estTokens > int(float64(window)*CompactThreshold)
}

// WindowOf resol la finestra d'un rol amb el defecte sa.
func WindowOf(window int) int {
	if window <= 0 {
		return DefaultWindow
	}
	return window
}

// PromptBudget és l'espai real per al prompt: la finestra menys la reserva
// de resposta. Sense descomptar max_tokens, amb finestres petites el prompt
// pot omplir-la tota i el proveïdor respon error perquè no hi cap la
// sortida. La reserva es limita a un quart de la finestra: un max_tokens
// desproporcionat (més gran que la finestra) no ha de deixar el prompt a
// zero.
func PromptBudget(window, maxTokens int) int {
	window = WindowOf(window)
	if maxTokens <= 0 {
		return window
	}
	reserve := maxTokens
	if lim := window / 4; reserve > lim {
		reserve = lim
	}
	return window - reserve
}

var (
	reCtxTokens    = regexp.MustCompile(`(?i)(?:context size \(|"n_ctx"\s*:\s*|maximum context length is |limit is |context is )(\d+)`)
	rePromptTokens = regexp.MustCompile(`(?i)the prompt does not fit:\s*(\d+)\s*tokens`)
)

// ParseContextExceeded detecta si un error d'un provider indica que s'ha superat
// la finestra de context del model i extreu la mida real (n_ctx) si hi consta.
func ParseContextExceeded(err error) (bool, int) {
	ok, _, window := ParseContextUsage(err)
	return ok, window
}

// ParseContextUsage extreu les mides del missatge del provider quan són
// disponibles. promptTokens permet calibrar l'estimació local del context.
func ParseContextUsage(err error) (exceeded bool, promptTokens, window int) {
	if err == nil {
		return false, 0, 0
	}
	msg := err.Error()
	lower := strings.ToLower(msg)
	if !strings.Contains(lower, "exceed_context_size_error") &&
		!strings.Contains(lower, "exceeds the available context size") &&
		!strings.Contains(lower, "maximum context length") &&
		!strings.Contains(lower, "the prompt does not fit") &&
		!strings.Contains(lower, "context_length_exceeded") &&
		!strings.Contains(lower, "prompt is too long") &&
		!strings.Contains(lower, "too many tokens") {
		return false, 0, 0
	}
	if m := rePromptTokens.FindStringSubmatch(msg); len(m) > 1 {
		promptTokens, _ = strconv.Atoi(m[1])
	}
	if m := reCtxTokens.FindStringSubmatch(msg); len(m) > 1 {
		if val, e := strconv.Atoi(m[1]); e == nil && val > 0 {
			window = val
		}
	}
	return true, promptTokens, window
}

// TrimKeep retorna els darrers keep missatges (continuïtat recent).
func TrimKeep(convo []llm.Message, keep int) []llm.Message {
	if keep < 0 {
		keep = 0
	}
	if len(convo) <= keep {
		return convo
	}
	return append([]llm.Message(nil), convo[len(convo)-keep:]...)
}

// Summarize demana al model un resum dens de la conversa per compactar.
// És una crida sense eines (amb failover si hi ha fallback).
//
// Si la conversa no cap en una sola crida, es resumeix per blocs (map) i es
// combinen els resums parcials (reduce). Abans, quan hi havia més de 25
// missatges, es descartava el mig («missatges intermedis omesos») i les
// decisions que hi vivien es perdien abans de resumir.
//
// lang és la llengua de la sessió (cfg.Lang(), o config.LangOfPrompt del
// system prompt): el resum torna al model i a vegades a la pantalla, i un
// resum en una altra llengua feia canviar la de les respostes.
func Summarize(ctx context.Context, client *llm.Client, prim llm.Target, fb *llm.Target, convo []llm.Message, lang string) (string, error) {
	// Hook pre_compact: abans de perdre el detall (lifecycle.go).
	hookPreCompact(ctx, len(convo), llm.EstimateTokens(convo))
	text := convoText(convo)
	// Mida de bloc: l'objectiu, eixamplat si cal perquè el nombre de crides
	// no esclati en una conversa enorme.
	size := summarizeChunkRunes
	if n := (len([]rune(text)) + summarizeMaxParts - 1) / summarizeMaxParts; n > size {
		size = n
	}
	parts := splitText(text, size)
	// splitText pot fer un tros de més si els talls cauen a la meitat; els
	// fusionem per no superar el nombre de crides promès.
	for len(parts) > summarizeMaxParts {
		last := parts[len(parts)-1]
		parts = parts[:len(parts)-1]
		parts[len(parts)-1] += last
	}
	instr := instrResum(summarizeInstr, lang, "Conversation")
	if len(parts) == 1 {
		return summarizeText(ctx, client, prim, fb, instr, parts[0])
	}
	resums := make([]string, 0, len(parts))
	nOK := 0
	for i, p := range parts {
		capçalera := fmt.Sprintf("(part %d of %d)\n", i+1, len(parts))
		s, err := summarizeText(ctx, client, prim, fb, instr, capçalera+p)
		if err != nil {
			// Una part que falla no ha de fer perdre la resta: es marca i
			// es continua; el resum combinat encara serveix.
			resums = append(resums, fmt.Sprintf("(part %d not summarized: %v)", i+1, err))
			continue
		}
		nOK++
		resums = append(resums, s)
	}
	if nOK == 0 {
		return "", fmt.Errorf("resum: cap bloc s'ha pogut resumir")
	}
	cos := capRunes(strings.Join(resums, "\n---\n"), 16000)
	return summarizeText(ctx, client, prim, fb, instrResum(combineInstr, lang, "Partial summaries"), cos)
}

const (
	// summarizeChunkRunes és la mida objectiu de cada bloc que es resumeix
	// per separat; summarizeMaxParts, el nombre màxim de crides (map) abans
	// de fusionar blocs, perquè una conversa enorme no faci 80 crides.
	summarizeChunkRunes = 12000
	summarizeMaxParts   = 6
	summarizeMaxTokens  = 1024
)

const summarizeInstr = "Summarize this working conversation with a coding agent. I want a DENSE, short summary (at most 30 lines) that lets us continue exactly where we were without rereading anything. Include, when present:\n" +
	"- Current goal and status (what is left).\n- Decisions made and why.\n- Files touched/created and what changes in each.\n- Errors found and how they were solved.\n- Exact relevant names, paths and commands.\nDo not add any assessment or questions. Reply ONLY with the summary."

const combineInstr = "Combine these partial summaries of the same working conversation into a single DENSE, short summary (at most 30 lines), without losing any decision, file, error or exact command. Do not repeat sections or add assessments. Reply ONLY with the combined summary."

// instrResum tanca una consigna de resum amb la directiva d'idioma i la
// capçalera del text que ve a continuació.
func instrResum(base, lang, capçalera string) string {
	return base + "\n" + config.LanguageDirective(lang) + "\n\n" + capçalera + ":\n"
}

// convoText renderitza la conversa a text pla amb capçalera de rol i les
// eines, retallant cada missatge per no menjar-se el context del resumidor.
func convoText(msgs []llm.Message) string {
	var b strings.Builder
	for _, m := range msgs {
		text := m.Content
		if len(m.ToolCalls) > 0 {
			var tcs []string
			for _, tc := range m.ToolCalls {
				tcs = append(tcs, tc.Function.Name+" "+tc.Function.Arguments)
			}
			text += "\n[tools: " + strings.Join(tcs, " · ") + "]"
		}
		if r := []rune(text); len(r) > 2000 {
			text = string(r[:2000]) + "…(truncated)"
		}
		b.WriteString("\n[" + m.Role + "]\n" + text + "\n")
	}
	return b.String()
}

// splitText parteix un text en trossos de ~size runes, procurant tallar en
// un salt de línia per no partir un missatge pel mig.
func splitText(s string, size int) []string {
	r := []rune(s)
	if size <= 0 || len(r) <= size {
		return []string{s}
	}
	var out []string
	for len(r) > size {
		cut := size
		for i := cut; i > size/2; i-- {
			if r[i] == '\n' {
				cut = i
				break
			}
		}
		out = append(out, string(r[:cut]))
		r = r[cut:]
	}
	if len(r) > 0 {
		out = append(out, string(r))
	}
	return out
}

// summarizeText fa una crida de resum amb un cos de prompt ja construït.
func summarizeText(ctx context.Context, client *llm.Client, prim llm.Target, fb *llm.Target, instr, cos string) (string, error) {
	cctx, cancel := context.WithTimeout(ctx, 180*time.Second)
	defer cancel()
	out, _, err := client.ChatFO(cctx, prim, fb,
		[]llm.Message{{Role: "user", Content: instr + cos}}, 0.3, summarizeMaxTokens, nil)
	if err != nil {
		return "", fmt.Errorf("resum: %w", err)
	}
	if strings.TrimSpace(out) == "" {
		return "", fmt.Errorf("resum buit")
	}
	return strings.TrimSpace(out), nil
}
