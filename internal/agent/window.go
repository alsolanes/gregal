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
	bases := map[string]string{} // base → clau
	for _, r := range cfg.Roles {
		for _, pn := range []string{r.Provider, r.FallbackProvider} {
			if p, ok := cfg.Providers[pn]; ok && strings.TrimSpace(p.BaseURL) != "" {
				bases[strings.TrimRight(p.BaseURL, "/")] = p.APIKey
			}
		}
	}
	var wg sync.WaitGroup
	for base, key := range bases {
		windowMu.Lock()
		if t, ok := windowProbe[base]; ok && time.Since(t) < 10*time.Minute {
			windowMu.Unlock()
			continue
		}
		windowProbe[base] = time.Now()
		windowMu.Unlock()
		wg.Add(1)
		go func(base, key string) {
			defer wg.Done()
			ws, err := llm.ProbeModelWindows(base, key)
			if err != nil {
				return
			}
			windowMu.Lock()
			for id, n := range ws {
				windowCache[windowKey(base, id)] = n
			}
			windowMu.Unlock()
		}(base, key)
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

// WindowSource és Window dient d'on surt: "config", "model" o "defecte".
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
	return DefaultWindow, "defecte"
}

func esLocal(base, provider string) bool {
	return strings.Contains(base, "localhost") || strings.Contains(base, "127.0.0.1") || strings.Contains(provider, "local")
}
