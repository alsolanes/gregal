// Package companion: el mode company de Gregal (estil Dot). Memòria personal
// persistent + check-ins proactius.
//
// La memòria NO és nova: usa la memòria d'usuari de config (~/.config/gregal/
// memory.md), que ja s'injecta a cada system prompt. Aquí només hi afegim:
//  1. Extracció de fets personals: després d'un torn de xat amb contingut
//     personal, una crida lateral proposa (màx. 2) fets duradors. La UI els
//     ensenya i només els acceptats es desen (mateixa higiene que automem:
//     res d'escriptura automàtica).
//  2. Check-in: un resum curt generat a partir de la memòria + la darrera
//     sessió, pensat per a una notificació programada (job daily).
package companion

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"gregal/internal/llm"
)

// MaxFactSuggestions topa els fets proposats per torn (com automem: pocs i bons).
const MaxFactSuggestions = 2

// maxFactChars per fet: la memòria d'usuari té límit de 2000 caràcters
// globals; cada fet ha de ser dens.
const maxFactChars = 200

// CheckinMaxChars topa el check-in: és una notificació, no un informe.
const CheckinMaxChars = 500

// extractSystem és la consigna de la crida lateral d'extracció.
const extractSystem = `You maintain the personal memory of a companion app (like Dot). From the conversation below, propose AT MOST 2 stable facts about the USER worth remembering across sessions: who they are, their ongoing projects, stable preferences, relevant life facts. Rules: each fact is one standalone sentence under 200 characters, third person ("Fa servir..."), factual and durable — not temporary state, not secrets/tokens/keys/passwords, not things already in the existing memory, not facts about you (the assistant). Most turns teach nothing personal: return an empty list. Reply with exactly one JSON object: {"facts": ["..."]}.`

// checkinSystem és la consigna del check-in proactiu.
const checkinSystem = `You are the companion of a busy person. Write their daily check-in: 1-2 short sentences in the user's language, warm and direct (no greeting filler, no "as your AI"). Ground it ONLY in the memory and the last session digest provided. Pick the single most relevant thing: follow up on an ongoing project, surface a pending question, or note progress. If nothing is grounded to say, say so in one honest sentence. Never invent facts. Maximum 500 characters.`

// secretRe enxampa secrets evidents: claus, tokens, contrasenyes.
var secretRe = regexp.MustCompile(`(?i)(sk-[a-z0-9]{4,}|api[_-]?key|token|password|contrasenya|clau d'access|clau api|\bbearer\b)`)

// ShouldExtract diu si un torn de xat mereix una proposta de fets:
// prou contingut personal (no una salutació de dues línies).
func ShouldExtract(hist []llm.Message) bool {
	userChars := 0
	for _, m := range hist {
		if m.Role == "user" {
			userChars += len([]rune(m.Content))
		}
	}
	// 100+ caràcters d'usuari al torn: una salutació no hi arriba, una
	// anècdota real sí. (Mesurat: "Resulta que m'he comprat…" ~100 cars.)
	return userChars >= 100
}

// ParseExtract valida i neteja les propostes del model:
// - JSON {"facts": [...]}, màx MaxFactSuggestions,
// - una frase per fet, sense secrets, sense duplicats (entre ells i amb la
// memòria existent, comparació normalitzada com fa automem).
func ParseExtract(raw string, existing []string) []string {
	var parsed struct {
		Facts []string `json:"facts"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &parsed); err != nil {
		return nil
	}
	seen := map[string]bool{}
	for _, e := range existing {
		if k := factKey(e); k != "" {
			seen[k] = true
		}
	}
	var out []string
	for _, f := range parsed.Facts {
		f = strings.Join(strings.Fields(f), " ")
		if f == "" || len([]rune(f)) > maxFactChars {
			continue
		}
		if secretRe.MatchString(f) {
			continue
		}
		k := factKey(f)
		dup := seen[k]
		for s := range seen {
			if len(k) >= 12 && len(s) >= 12 && (strings.Contains(s, k) || strings.Contains(k, s)) {
				dup = true
			}
		}
		if dup {
			continue
		}
		seen[k] = true
		out = append(out, f)
		if len(out) == MaxFactSuggestions {
			break
		}
	}
	return out
}

// factKey normalitza un fet per comparar-lo: sense prefix de data,
// minúscules, sense puntuació repetida (mateixa idea que noteKey d'automem).
func factKey(fact string) string {
	fact = strings.TrimSpace(fact)
	if i := strings.Index(fact, ": "); i > 0 && isDatePrefix(fact[:i]) {
		fact = fact[i+2:]
	}
	fact = strings.TrimPrefix(fact, "- ")
	return strings.ToLower(strings.Join(strings.Fields(fact), " "))
}

func isDatePrefix(s string) bool {
	if len(s) != 10 {
		return false
	}
	return s[4] == '-' && s[7] == '-'
}

// ExtractMessage compon les missatges de la crida lateral d'extracció.
func ExtractMessage(digest, existing string) []llm.Message {
	if strings.TrimSpace(existing) == "" {
		existing = "(none)"
	}
	return []llm.Message{
		{Role: "system", Content: extractSystem},
		{Role: "user", Content: "EXISTING MEMORY:\n" + existing + "\n\nCONVERSATION:\n" + digest},
	}
}

// CheckinPrompt compon el prompt del check-in diari amb la memòria i el
// digest de la darrera sessió. La crida la fa el job existent; aquí només
// hi ha el text, testejable sense xarxa.
func CheckinPrompt(memory, lastSessionDigest string) string {
	if strings.TrimSpace(memory) == "" {
		memory = "(empty: this is a fresh start, greet them and invite one line about what they are up to)"
	}
	if strings.TrimSpace(lastSessionDigest) == "" {
		lastSessionDigest = "(no recent session)"
	}
	return fmt.Sprintf("PERSONAL MEMORY:\n%s\n\nLAST SESSION DIGEST:\n%s\n\nWrite the check-in now (max %d characters).", memory, lastSessionDigest, CheckinMaxChars)
}

// CheckinSystem torna la consigna del check-in (per si el caller vol
// sobreescriure-la amb l'idioma de l'usuari).
func CheckinSystem() string { return checkinSystem }

// TruncateCheckin retalla un check-in a CheckinMaxChars per límits de
// notificació, sense partir paraula.
func TruncateCheckin(s string) string {
	s = strings.TrimSpace(s)
	r := []rune(s)
	if len(r) <= CheckinMaxChars {
		return s
	}
	cut := string(r[:CheckinMaxChars])
	if i := strings.LastIndex(cut, " "); i > CheckinMaxChars*2/3 {
		cut = cut[:i]
	}
	return cut + "…"
}
