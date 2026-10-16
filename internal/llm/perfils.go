package llm

import (
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

// Perfils de família de model.
//
// Les model cards recomanen un mostreig concret i alguns models en
// depenen molt (Qwen3 amb greedy entra en bucles infinits; Devstral vol
// temperatura 0,15). Als servidors locals (llama.cpp, Ollama, LM Studio,
// vLLM) ningú no posa aquests valors si no els posa el client, així que
// s'apliquen allà. Als proveïdors remots no: ja serveixen el model amb els
// seus valors. La temperatura del rol, si n'hi ha, sempre mana.

// Profile descriu una família.
type Profile struct {
	Family            string
	Temperature       float64 // 0 = no s'envia
	TopP              float64
	TopK              int
	MinP              *float64
	PresencePenalty   float64
	RepetitionPenalty float64
}

type profileRule struct {
	re *regexp.Regexp
	p  Profile
}

func f64(v float64) *float64 { return &v }

// L'ordre importa: el primer que coincideix guanya (qwen3-coder abans de qwen3).
var profileRules = []profileRule{
	{regexp.MustCompile(`qwen3[\w.\-]*coder|qwen-?3-?coder`), Profile{Family: "qwen3-coder", Temperature: 0.7, TopP: 0.8, TopK: 20, RepetitionPenalty: 1.05}},
	{regexp.MustCompile(`qwen2\.5[\w.\-]*coder|qwen-?2\.5-?coder`), Profile{Family: "qwen2.5-coder", Temperature: 0.7, TopP: 0.8, TopK: 20, RepetitionPenalty: 1.05}},
	{regexp.MustCompile(`qwq|qwen3`), Profile{Family: "qwen3", Temperature: 0.6, TopP: 0.95, TopK: 20, MinP: f64(0), PresencePenalty: 0.5}},
	{regexp.MustCompile(`qwen`), Profile{Family: "qwen", Temperature: 0.7, TopP: 0.8, TopK: 20, RepetitionPenalty: 1.05}},
	{regexp.MustCompile(`deepseek[\w.\-]*r1|r1-distill`), Profile{Family: "deepseek-r1", Temperature: 0.6, TopP: 0.95}},
	{regexp.MustCompile(`deepseek`), Profile{Family: "deepseek", Temperature: 0.3}},
	{regexp.MustCompile(`gpt-oss`), Profile{Family: "gpt-oss", Temperature: 1.0, TopP: 1.0}},
	{regexp.MustCompile(`glm`), Profile{Family: "glm", Temperature: 0.7, TopP: 0.95, TopK: 40}},
	{regexp.MustCompile(`devstral`), Profile{Family: "devstral", Temperature: 0.15}},
	{regexp.MustCompile(`magistral`), Profile{Family: "magistral", Temperature: 0.7, TopP: 0.95}},
	{regexp.MustCompile(`mistral|codestral|ministral`), Profile{Family: "mistral", Temperature: 0.15}},
	{regexp.MustCompile(`gemma`), Profile{Family: "gemma", Temperature: 1.0, TopP: 0.95, TopK: 64, MinP: f64(0)}},
	{regexp.MustCompile(`llama`), Profile{Family: "llama", Temperature: 0.6, TopP: 0.9}},
	{regexp.MustCompile(`phi-?4|phi4`), Profile{Family: "phi", Temperature: 0.6, TopP: 0.95}},
	{regexp.MustCompile(`kimi`), Profile{Family: "kimi", Temperature: 0.6}},
}

// ProfileFor torna el perfil d'un model (Family "" si no se'n coneix cap).
func ProfileFor(model string) Profile {
	m := strings.ToLower(model)
	for _, r := range profileRules {
		if r.re.MatchString(m) {
			return r.p
		}
	}
	return Profile{}
}

var sizeRe = regexp.MustCompile(`(?:^|[^a-z0-9.])(\d+(?:\.\d+)?)\s*b(?:$|[^a-z0-9])`)

// ModelSizeB torna la mida (milers de milions de paràmetres) si el nom la
// porta («qwen2.5-coder:7b», «Qwen3-30B-A3B» → 30), o 0.
func ModelSizeB(model string) float64 {
	m := strings.ToLower(model)
	best := 0.0
	for _, g := range sizeRe.FindAllStringSubmatch(m, -1) {
		if v, err := strconv.ParseFloat(g[1], 64); err == nil && v > best {
			best = v
		}
	}
	return best
}

// IsLocalBase diu si l'URL apunta a un servidor de la mateixa màquina o de
// la xarxa local (llama.cpp, Ollama, LM Studio, vLLM casolà).
func IsLocalBase(base string) bool {
	u, err := url.Parse(strings.TrimSpace(base))
	if err != nil || u.Host == "" {
		return false
	}
	h := strings.ToLower(u.Hostname())
	switch {
	case h == "localhost", h == "::1", strings.HasPrefix(h, "127."), strings.HasSuffix(h, ".local"), strings.HasSuffix(h, ".localhost"), h == "host.docker.internal":
		return true
	case strings.HasPrefix(h, "10."), strings.HasPrefix(h, "192.168."):
		return true
	case strings.HasPrefix(h, "172."):
		parts := strings.Split(h, ".")
		if len(parts) > 1 {
			if n, err := strconv.Atoi(parts[1]); err == nil && n >= 16 && n <= 31 {
				return true
			}
		}
	}
	return false
}

// CompactFor diu si convé el mode compacte (prompt curt i menys eines):
// models locals petits (≤ 14B) o finestres de context petites (< 32k).
func CompactFor(base, model string, window int) bool {
	if !IsLocalBase(base) {
		return false
	}
	if window > 0 && window < 32768 {
		return true
	}
	if s := ModelSizeB(model); s > 0 && s <= 14 {
		return true
	}
	return false
}

// applyProfile posa el mostreig recomanat a les peticions cap a servidors
// locals, sense trepitjar el que ja porti la petició.
func applyProfile(base string, req chatRequest) chatRequest {
	if !IsLocalBase(base) {
		return req
	}
	p := ProfileFor(req.Model)
	if p.Family == "" {
		return req
	}
	if req.Temperature == 0 && p.Temperature > 0 {
		req.Temperature = p.Temperature
	}
	if req.TopP == nil && p.TopP > 0 {
		req.TopP = f64(p.TopP)
	}
	if req.TopK == 0 && p.TopK > 0 {
		req.TopK = p.TopK
	}
	if req.MinP == nil && p.MinP != nil {
		req.MinP = p.MinP
	}
	if req.PresencePenalty == nil && p.PresencePenalty > 0 {
		req.PresencePenalty = f64(p.PresencePenalty)
	}
	if req.RepetitionPenalty == nil && p.RepetitionPenalty > 0 {
		req.RepetitionPenalty = f64(p.RepetitionPenalty)
		req.RepeatPenalty = f64(p.RepetitionPenalty) // nom de llama.cpp
	}
	return req
}
