package agent

// Finestra de context per model, detectada del proveïdor.
//
// Abans la finestra era el context_window del rol o, si valia 0, un
// defecte (32K, o 8K si el proveïdor semblava local). Amb models de 64K
// o 128K era llençar la meitat del context, i amb models més petits era
// un 400 a mig torn. Ara `context_window: 0` vol dir «la que declari el
// model»: es demana a <base>/models a l'arrencada (i en canviar de
// proveïdor) i es recorda per (URL, model). Un valor explícit al config
// continua manant. Si el proveïdor no ho diu (OpenAI, zen), queda el
// defecte de sempre, i si el model es queixa a mig torn amb la mida real
// (ParseContextExceeded), s'aprèn (LearnWindow) i no cal tornar-hi.

import (
	"context"
	"strings"
	"sync"
	"time"

	"gregal/internal/config"
	"gregal/internal/llm"
)

var (
	windowMu    sync.Mutex
	windowCache = map[string]int{}       // clau: base|model → tokens
	windowProbe = map[string]time.Time{} // clau: base → últim intent
)

func windowKey(base, model string) string {
	return strings.TrimRight(strings.TrimSpace(base), "/") + "|" + strings.TrimSpace(model)
}

// LearnWindow desa la finestra real d'un model (del proveïdor o d'un
// error de context excedit).
func LearnWindow(base, model string, n int) {
	if n <= 0 {
		return
	}
	windowMu.Lock()
	windowCache[windowKey(base, model)] = n
	windowMu.Unlock()
}

// KnownWindow torna la finestra apresa d'un model, si n'hi ha.
func KnownWindow(base, model string) (int, bool) {
	windowMu.Lock()
	defer windowMu.Unlock()
	n, ok := windowCache[windowKey(base, model)]
	return n, ok
}

// ResetWindows buida la cache (tests).
func ResetWindows() {
	windowMu.Lock()
	windowCache = map[string]int{}
	windowProbe = map[string]time.Time{}
	windowMu.Unlock()
}

// DetectWindows consulta /models de cada proveïdor que fa servir algun rol
// (una vegada cada deu minuts com a molt) i desa les finestres. És
// bloquejant però curt (4 s per proveïdor, en paral·lel): el TUI ho corre
// en un tea.Cmd; el headless i el web, abans del primer pas.
func DetectWindows(ctx context.Context, cfg *config.Config) {
	if cfg == nil {
		return
	}
	bases := map[string]*baseProbe{}
	for _, r := range cfg.Roles {
		for _, pm := range [][2]string{{r.Provider, r.Model}, {r.FallbackProvider, r.FallbackModel}} {
			if p, ok := cfg.Providers[pm[0]]; ok && strings.TrimSpace(p.BaseURL) != "" {
				afegeixBase(bases, p, pm[1])
			}
		}
	}
	detectWindowBases(ctx, bases)
}

// DetectRoleWindows detecta les finestres que pot fer servir aquest torn:
// el model actiu i el fallback. No espera per providers d'altres rols, i no
// fa cap consulta si el config ja fixa la finestra o els models ja són coneguts.
// La detecció global continua disponible per a la inicialització del TUI.
func DetectRoleWindows(ctx context.Context, cfg *config.Config, role config.Role) {
	if cfg == nil || role.ContextWindow > 0 {
		return
	}
	bases := map[string]*baseProbe{}
	add := func(providerName, model string) {
		p, ok := cfg.Providers[providerName]
		if !ok || strings.TrimSpace(p.BaseURL) == "" || strings.TrimSpace(model) == "" {
			return
		}
		if _, known := KnownWindow(strings.TrimRight(p.BaseURL, "/"), model); !known {
			afegeixBase(bases, p, model)
		}
	}
	add(role.Provider, role.Model)
	add(role.FallbackProvider, role.FallbackModel)
	detectWindowBases(ctx, bases)
}

// baseProbe és el que cal per sondejar un endpoint: la clau i els models
// dels rols (per a la finestra real dels servidors locals).
type baseProbe struct {
	key    string
	models []string
}

func afegeixBase(bases map[string]*baseProbe, p config.Provider, model string) {
	base := strings.TrimRight(p.BaseURL, "/")
	b, ok := bases[base]
	if !ok {
		b = &baseProbe{key: p.APIKey}
		bases[base] = b
	}
	if m := strings.TrimSpace(model); m != "" {
		b.models = append(b.models, m)
	}
}

// esServidorLocal diu si cal preguntar la finestra al servidor local
// (/props, /api/ps, /api/v0/models): els tipus locals declarats o una
// adreça local.
func esServidorLocal(base string) bool {
	switch llm.ProviderTypeFor(base) {
	case llm.ProviderOllama, llm.ProviderLlamaCpp:
		return true
	case llm.ProviderAnthropic, llm.ProviderOpenAIResponses:
		return false
	}
	return esLocal(base, "")
}

func detectWindowBases(ctx context.Context, bases map[string]*baseProbe) {
	var wg sync.WaitGroup
	for base, bp := range bases {
		windowMu.Lock()
		if t, ok := windowProbe[base]; ok && time.Since(t) < 10*time.Minute {
			windowMu.Unlock()
			continue
		}
		windowProbe[base] = time.Now()
		windowMu.Unlock()
		wg.Add(1)
		go func(base string, bp *baseProbe) {
			defer wg.Done()
			if ws, err := llm.ProbeModelWindows(base, bp.key); err == nil {
				windowMu.Lock()
				for id, n := range ws {
					windowCache[windowKey(base, id)] = n
				}
				windowMu.Unlock()
			}
			if !esServidorLocal(base) {
				return
			}
			// Un servidor local té la finestra que se li ha configurat, que
			// no és la d'entrenament que declara /v1/models (llama.cpp hi
			// posa n_ctx_train): sense això, la compactació arribava tard i
			// el servidor truncava (Ollama) o desplaçava el context en silenci.
			srv := llm.ProbeLocalServer(ctx, base, bp.key)
			for _, m := range bp.models {
				if n, ok := llm.LocalWindow(ctx, srv, base, bp.key, m); ok {
					LearnWindow(base, m, n)
				}
			}
		}(base, bp)
	}
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-ctx.Done():
	}
}

// Window resol la finestra efectiva d'un rol: config explícit > detectada
// o apresa > defecte (8K si el proveïdor és local i no ho diu, 32K si no).
func Window(cfg *config.Config, r config.Role) int {
	n, _ := WindowSource(cfg, r)
	return n
}

// Budget és la finestra efectiva d'un rol per al PROMPT: la finestra menys
// la reserva de resposta (max_tokens). És el valor que han de fer servir
// NeedsCompact i MakeRoom; Window és el límit que es mostra a l'usuari.
func Budget(cfg *config.Config, r config.Role) int {
	return PromptBudget(Window(cfg, r), r.MaxTokens)
}

// finestresConegudes són finestres de context de famílies de models que els
// proveïdors no declaren a /models (OpenAI, opencode zen…). Sense això tot
// queia al defecte de 32K i una tasca llarga es compactava a ~18K tokens
// amb models de 128K o més. Els valors són conservadors (el mínim de la
// família, arrodonit avall): sobreestimar fa que el proveïdor rebutgi el
// prompt, i llavors el motor aprèn la mida real de l'error
// (ParseContextExceeded → LearnWindow). Només s'apliquen a proveïdors
// remots: un servidor local té la finestra que se li ha configurat. L'ordre
// importa (el patró més específic primer); context_window al rol mana sempre.
var finestresConegudes = []struct {
	patro  string
	tokens int
}{
	{"claude", 200000},
	{"gemini-1.5", 1000000},
	{"gemini-2", 1000000},
	{"gemini-3", 1000000},
	{"gpt-4.1", 1000000},
	{"gpt-4o", 128000},
	{"gpt-5", 272000},
	{"deepseek-v3.1", 128000},
	{"deepseek-v3.2", 128000},
	{"deepseek-v4", 128000},
	{"deepseek-chat", 128000},
	{"deepseek-reasoner", 128000},
	{"deepseek", 64000},
	{"kimi-k2", 128000},
	{"glm-4.5", 128000},
	{"glm-4.6", 128000},
	{"qwen3-coder", 128000},
}

// finestraConeguda busca el model a finestresConegudes (0 si no hi és).
func finestraConeguda(model string) int {
	m := strings.ToLower(strings.TrimSpace(model))
	if i := strings.LastIndex(m, "/"); i >= 0 {
		m = m[i+1:] // «openrouter/anthropic/claude-…» → «claude-…»
	}
	for _, f := range finestresConegudes {
		if strings.Contains(m, f.patro) {
			return f.tokens
		}
	}
	return 0
}

// WindowSource és Window dient d'on surt: "config", "model", "coneguda"
// (taula finestresConegudes) o "defecte".
func WindowSource(cfg *config.Config, r config.Role) (int, string) {
	if r.ContextWindow > 0 {
		return r.ContextWindow, "config"
	}
	base := ""
	if cfg != nil {
		if p, ok := cfg.Providers[r.Provider]; ok {
			base = p.BaseURL
		}
	}
	if n, ok := KnownWindow(base, r.Model); ok {
		return n, "model"
	}
	if esLocal(base, r.Provider) {
		return 8192, "defecte"
	}
	if n := finestraConeguda(r.Model); n > 0 {
		return n, "coneguda"
	}
	return DefaultWindow, "defecte"
}

func esLocal(base, provider string) bool {
	return strings.Contains(base, "localhost") || strings.Contains(base, "127.0.0.1") || strings.Contains(provider, "local")
}
