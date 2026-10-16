package main

import (
	"context"
	"fmt"
	"net"
	neturl "net/url"
	"strings"
	"time"

	"gregal/internal/config"
	"gregal/internal/llm"
)

// Comprovacions de gregal doctor per als servidors locals (llama.cpp,
// Ollama, LM Studio): el que trenca la caché de KV o trunca el context en
// silenci, i els flags que acceleren el prefill. Vegeu docs/local-models.md.

var doctorLocalMessages = map[string]cliMessage{
	"doctor.status_warn":          {"WARN", "AVÍS"},
	"doctor.status_info":          {"INFO", "INFO"},
	"doctor.local":                {"local server", "servidor local"},
	"doctor.local_llamacpp":       {"%s: llama.cpp, n_ctx %s per slot, %d slot(s)", "%s: llama.cpp, n_ctx %s per slot, %d slot(s)"},
	"doctor.local_set_type":       {"%s: set `type: %s` on the provider (%s)", "%s: posa `type: %s` al provider (%s)"},
	"doctor.local_llama_why":      {"prompt cache, a fixed slot per session and cache timings", "caché de prompt, slot fix per sessió i timings de caché"},
	"doctor.local_ollama_why":     {"through /v1 Ollama uses its default num_ctx (2-4k) and silently drops the start of the prompt", "per /v1 Ollama fa servir el num_ctx per defecte (2-4k) i descarta en silenci el principi del prompt"},
	"doctor.local_ctx_shift":      {"%s: --context-shift is on: llama.cpp silently drops old tokens when the context fills; start llama-server with --no-context-shift", "%s: --context-shift és actiu: llama.cpp descarta tokens vells en silenci quan el context s'omple; engega llama-server amb --no-context-shift"},
	"doctor.local_generic":        {"%s: the chat template cannot render tools (llama.cpp falls back to the Generic format: less reliable tool calls, more tokens); use --jinja and the model's own template (--chat-template-file)", "%s: la plantilla de xat no sap pintar eines (llama.cpp cau al format Generic: crides menys fiables i més tokens); fes servir --jinja i la plantilla del model (--chat-template-file)"},
	"doctor.local_small_ctx":      {"%s: context %s is below %s tokens: the system prompt and tools alone fill most of it", "%s: context de %s, per sota de %s tokens: el system prompt i les eines ja n'omplen gairebé tot"},
	"doctor.local_llama_tips":     {"%s: speed-ups: %s", "%s: per anar més ràpid: %s"},
	"doctor.local_ollama":         {"%s: Ollama, %s: num_ctx %s (%s), keep_alive %s", "%s: Ollama, %s: num_ctx %s (%s), keep_alive %s"},
	"doctor.local_ollama_fixed":   {"fixed by Gregal", "fix, el posa Gregal"},
	"doctor.local_ollama_loaded":  {"loaded", "carregat"},
	"doctor.local_ollama_v1":      {"%s: Ollama through /v1, %s loaded with num_ctx %s", "%s: Ollama per /v1, %s carregat amb num_ctx %s"},
	"doctor.local_no_tools":       {"%s: Ollama says %s does not support tools (capabilities: %s)", "%s: Ollama diu que %s no admet eines (capacitats: %s)"},
	"doctor.local_ollama_tips":    {"%s: server env: OLLAMA_FLASH_ATTENTION=1 OLLAMA_KV_CACHE_TYPE=q8_0; OLLAMA_CONTEXT_LENGTH and OLLAMA_KEEP_ALIVE for other clients", "%s: entorn del servidor: OLLAMA_FLASH_ATTENTION=1 OLLAMA_KV_CACHE_TYPE=q8_0; OLLAMA_CONTEXT_LENGTH i OLLAMA_KEEP_ALIVE per a altres clients"},
	"doctor.local_lmstudio":       {"%s: LM Studio, %s: context %s (%s)", "%s: LM Studio, %s: context %s (%s)"},
	"doctor.local_lmstudio_state": {"%s: LM Studio, %s is %s: the first call loads it (raise the idle TTL so it stays loaded)", "%s: LM Studio, %s és %s: la primera crida el carrega (puja el TTL d'inactivitat perquè no es descarregui)"},
}

func init() {
	for k, v := range doctorLocalMessages {
		cliMessages[k] = v
	}
}

// liniaDoctor és una línia del doctor: ok, warn (no compta com a fallada) o info.
type liniaDoctor struct {
	estat  string // "ok", "warn", "info"
	detall string
}

// localTarget és un endpoint local que fan servir els rols.
type localTarget struct {
	nom, base, key, tipus string
	models                []string
}

func localTargets(cfg *config.Config) []localTarget {
	var out []localTarget
	idx := map[string]int{}
	for _, name := range strings.Split(cfg.RoleNames(), ", ") {
		r, ok := cfg.Roles[strings.TrimSpace(name)]
		if !ok {
			continue
		}
		for _, pm := range [][2]string{{r.Provider, r.Model}, {r.FallbackProvider, r.FallbackModel}} {
			p, ok := cfg.Providers[pm[0]]
			if !ok || strings.TrimSpace(p.BaseURL) == "" {
				continue
			}
			tipus, _ := llm.NormalizeProviderType(p.Type)
			if tipus == llm.ProviderAnthropic || tipus == llm.ProviderOpenAIResponses {
				continue
			}
			if tipus == "" && !esAdrecaLocal(p.BaseURL) {
				continue
			}
			base := strings.TrimRight(p.BaseURL, "/")
			i, ja := idx[base]
			if !ja {
				i = len(out)
				idx[base] = i
				out = append(out, localTarget{nom: pm[0], base: base, key: p.APIKey, tipus: tipus})
			}
			if m := strings.TrimSpace(pm[1]); m != "" && !conte(out[i].models, m) {
				out[i].models = append(out[i].models, m)
			}
		}
	}
	return out
}

func conte(xs []string, x string) bool {
	for _, y := range xs {
		if y == x {
			return true
		}
	}
	return false
}

// esAdrecaLocal diu si el base_url apunta a la màquina o a la xarxa local.
func esAdrecaLocal(base string) bool {
	u, err := neturl.Parse(strings.TrimSpace(base))
	if err != nil {
		return false
	}
	h := strings.ToLower(u.Hostname())
	if h == "localhost" || strings.HasSuffix(h, ".local") || strings.HasSuffix(h, ".localhost") {
		return true
	}
	ip := net.ParseIP(h)
	return ip != nil && (ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified())
}

// checksLocal sondeja un endpoint local i torna les línies del doctor.
func checksLocal(ctx context.Context, lang string, t localTarget) []liniaDoctor {
	text := func(key string, args ...any) string { return cliText(lang, key, args...) }
	srv := llm.ProbeLocalServer(ctx, t.base, t.key)
	var out []liniaDoctor
	add := func(estat, detall string) { out = append(out, liniaDoctor{estat, detall}) }
	switch srv.Kind {
	case llm.ServerLlamaCpp:
		p := srv.Props
		add("ok", text("doctor.local_llamacpp", t.nom, llm.FmtCount(p.NCtx), p.TotalSlots))
		if t.tipus != llm.ProviderLlamaCpp {
			add("warn", text("doctor.local_set_type", t.nom, llm.ProviderLlamaCpp, text("doctor.local_llama_why")))
		}
		if p.ContextShift != nil && *p.ContextShift {
			add("warn", text("doctor.local_ctx_shift", t.nom))
		}
		if !p.ToolsTemplate {
			add("warn", text("doctor.local_generic", t.nom))
		}
		if p.NCtx > 0 && p.NCtx < llm.OllamaMinNumCtx {
			add("warn", text("doctor.local_small_ctx", t.nom, llm.FmtCount(p.NCtx), llm.FmtCount(llm.OllamaMinNumCtx)))
		}
		var tips []string
		if p.CacheReuse == nil || *p.CacheReuse == 0 {
			tips = append(tips, "--cache-reuse 256")
		}
		tips = append(tips, "-ctk q8_0 -ctv q8_0 (not q4_0 on K)", "-fa on", "-ub 1024")
		if p.Speculative == nil || !*p.Speculative {
			tips = append(tips, "--spec-type ngram-mod")
		}
		add("info", text("doctor.local_llama_tips", t.nom, strings.Join(tips, ", ")))
	case llm.ServerOllama:
		if t.tipus != llm.ProviderOllama {
			add("warn", text("doctor.local_set_type", t.nom, llm.ProviderOllama, text("doctor.local_ollama_why")))
			for _, l := range srv.Loaded {
				if l.ContextLength > 0 {
					add("info", text("doctor.local_ollama_v1", t.nom, l.Name, llm.FmtCount(l.ContextLength)))
					if l.ContextLength < llm.OllamaMinNumCtx {
						add("warn", text("doctor.local_small_ctx", t.nom, llm.FmtCount(l.ContextLength), llm.FmtCount(llm.OllamaMinNumCtx)))
					}
				}
			}
		} else {
			for _, m := range t.models {
				n := llm.OllamaNumCtx(ctx, t.base, t.key, m)
				font := text("doctor.local_ollama_fixed")
				for _, l := range srv.Loaded {
					if llm.OllamaSameModel(l.Name, m) && l.ContextLength == n {
						font = text("doctor.local_ollama_loaded")
					}
				}
				ka := fmt.Sprint(llm.OllamaKeepAliveFor(t.base))
				estat := "ok"
				if n < llm.OllamaMinNumCtx {
					estat = "warn"
				}
				add(estat, text("doctor.local_ollama", t.nom, m, llm.FmtCount(n), font, ka))
				if estat == "warn" {
					add("warn", text("doctor.local_small_ctx", t.nom, llm.FmtCount(n), llm.FmtCount(llm.OllamaMinNumCtx)))
				}
			}
		}
		for _, m := range t.models {
			if show, err := llm.ProbeOllamaShow(ctx, t.base, t.key, m); err == nil && !show.Te("tools") {
				add("warn", text("doctor.local_no_tools", t.nom, m, strings.Join(show.Capabilities, ", ")))
			}
		}
		add("info", text("doctor.local_ollama_tips", t.nom))
	case llm.ServerLMStudio:
		for _, m := range t.models {
			for _, lm := range srv.LMStudio {
				if lm.ID != m {
					continue
				}
				if lm.State != "loaded" {
					add("info", text("doctor.local_lmstudio_state", t.nom, m, lm.State))
				}
				if w := lm.Window(); w > 0 {
					estat := "ok"
					if w < llm.OllamaMinNumCtx {
						estat = "warn"
					}
					add(estat, text("doctor.local_lmstudio", t.nom, m, llm.FmtCount(w), lm.State))
				}
			}
		}
	}
	return out
}

// doctorLocal imprimeix les comprovacions locals. Els avisos no compten
// com a fallades: el servidor funciona, però pot anar millor.
func doctorLocal(cfg *config.Config, lang string, ok func(name, detail string)) {
	text := func(key string, args ...any) string { return cliText(lang, key, args...) }
	for _, t := range localTargets(cfg) {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		linies := checksLocal(ctx, lang, t)
		cancel()
		for _, l := range linies {
			switch l.estat {
			case "ok":
				ok(text("doctor.local"), l.detall)
			case "warn":
				fmt.Printf("%-4s%-12s %s\n", text("doctor.status_warn"), text("doctor.local"), l.detall)
			default:
				fmt.Printf("%-4s%-12s %s\n", text("doctor.status_info"), text("doctor.local"), l.detall)
			}
		}
	}
}
