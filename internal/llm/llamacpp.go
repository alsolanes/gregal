package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Servidors locals: opcions per endpoint, slots de llama.cpp i timings.
//
// En local el temps el domina el prefill: si la caché de KV no encerta, una
// conversa de 30k tokens triga desenes de segons abans de començar a
// respondre. Dues coses la trenquen sense que ningú se n'adoni:
//   - llama-server reparteix les peticions entre slots per semblança de
//     prompt; una crida lateral (títol, revisió, missatge de commit) pot caure
//     al slot de la sessió i expulsar-ne el prefix. Amb id_slot la sessió té
//     el seu slot i les crides laterals un altre (si n'hi ha més d'un);
//   - cache_prompt és per defecte a les versions actuals, però no a totes:
//     s'envia explícit.
//
// I els timings (cache_n, prompt_n, prompt_per_second) que torna llama.cpp
// a cada resposta diuen quants tokens han sortit de la caché: passen al
// comptador de consum com a cached_tokens.

// LocalOptions són les opcions d'un servidor local (providers.<nom>).
type LocalOptions struct {
	// NumCtx és el num_ctx fix que s'envia a Ollama (0 = decideix Gregal:
	// el del model carregat o 32k, vegeu ollama.go).
	NumCtx int
	// KeepAlive és el keep_alive d'Ollama ("" = 30m; "-1" = sempre).
	KeepAlive string
	// Slot és l'id_slot de la sessió a llama.cpp (nil = 0; -1 = no fixar).
	Slot *int
	// SideSlot és l'id_slot de les crides laterals (nil = l'últim slot si
	// el servidor en té més d'un; -1 = no fixar).
	SideSlot *int
}

var localOptions sync.Map // normBase → LocalOptions

// RegisterLocalOptions desa les opcions d'un endpoint local (es crida en
// carregar el config, com RegisterProviderType).
func RegisterLocalOptions(baseURL string, o LocalOptions) {
	base := normBase(baseURL)
	if base == "" {
		return
	}
	localOptions.Store(base, o)
}

func localOptionsFor(baseURL string) LocalOptions {
	if v, ok := localOptions.Load(normBase(baseURL)); ok {
		return v.(LocalOptions)
	}
	return LocalOptions{}
}

type sideCallKey struct{}

// WithSideCall marca una crida lateral (títol, revisió, missatge de commit,
// resum): a llama.cpp va a un altre slot perquè no expulsi el prefix de la
// sessió de la caché de KV.
func WithSideCall(ctx context.Context) context.Context {
	return context.WithValue(ctx, sideCallKey{}, true)
}

func isSideCall(ctx context.Context) bool {
	v, _ := ctx.Value(sideCallKey{}).(bool)
	return v
}

// serverRoot treu el /v1 (o /api) del final: /props, /api/chat i
// /api/v0/models viuen a l'arrel del servidor.
func serverRoot(baseURL string) string {
	b := normBase(baseURL)
	if i := strings.IndexByte(b, '?'); i >= 0 {
		b = b[:i]
	}
	for _, suf := range []string{"/v1", "/api"} {
		if strings.HasSuffix(b, suf) {
			return strings.TrimSuffix(b, suf)
		}
	}
	return b
}

// LlamaProps és el que interessa de GET /props de llama-server.
type LlamaProps struct {
	NCtx         int    // finestra per slot (default_generation_settings.n_ctx)
	TotalSlots   int    // --parallel
	ChatTemplate string // plantilla Jinja carregada
	ModelPath    string
	// ToolsTemplate diu si la plantilla sap pintar eines (chat_template_caps
	// si el servidor ho declara; si no, si la plantilla esmenta «tool»).
	ToolsTemplate bool
	// ContextShift és el --context-shift si el servidor el declara (nil = no ho diu).
	ContextShift *bool
	// CacheReuse és n_cache_reuse si el servidor el declara (nil = no ho diu).
	CacheReuse *int
	// Speculative diu si hi ha descodificació especulativa (nil = no ho diu).
	Speculative *bool
	Build       string
}

// ProbeLlamaProps fa GET <arrel>/props. Torna error si no és llama-server.
func ProbeLlamaProps(ctx context.Context, baseURL, apiKey string) (LlamaProps, error) {
	var raw map[string]any
	if err := getJSON(ctx, serverRoot(baseURL)+"/props", apiKey, &raw); err != nil {
		return LlamaProps{}, err
	}
	gen, _ := raw["default_generation_settings"].(map[string]any)
	if gen == nil {
		if _, ok := raw["total_slots"]; !ok {
			return LlamaProps{}, fmt.Errorf("/props sense default_generation_settings: no és llama-server")
		}
	}
	p := LlamaProps{
		NCtx:         numAny(gen["n_ctx"]),
		TotalSlots:   numAny(raw["total_slots"]),
		ChatTemplate: strAny(raw["chat_template"]),
		ModelPath:    strAny(raw["model_path"]),
		Build:        strAny(raw["build_info"]),
	}
	if p.NCtx == 0 {
		p.NCtx = numAny(raw["n_ctx"])
	}
	p.ToolsTemplate = strings.Contains(strings.ToLower(p.ChatTemplate), "tool")
	if caps, ok := raw["chat_template_caps"].(map[string]any); ok {
		for _, k := range []string{"supports_tool_calls", "supports_tools"} {
			if b, ok := caps[k].(bool); ok {
				p.ToolsTemplate = b
				break
			}
		}
	}
	// Els flags del servidor no surten tots a /props i els noms han canviat
	// entre versions: es busquen a tot l'arbre.
	visitJSON(raw, func(k string, v any) {
		lk := strings.ToLower(k)
		switch {
		case strings.Contains(lk, "context_shift") || strings.Contains(lk, "ctx_shift"):
			if b, ok := v.(bool); ok && p.ContextShift == nil {
				p.ContextShift = &b
			}
		case strings.Contains(lk, "cache_reuse"):
			if _, ok := v.(float64); ok && p.CacheReuse == nil {
				n := numAny(v)
				p.CacheReuse = &n
			}
		case lk == "speculative":
			if b, ok := v.(bool); ok && p.Speculative == nil {
				p.Speculative = &b
			}
		}
	})
	return p, nil
}

func visitJSON(v any, fn func(k string, v any)) {
	switch x := v.(type) {
	case map[string]any:
		for k, val := range x {
			fn(k, val)
			visitJSON(val, fn)
		}
	case []any:
		for _, val := range x {
			visitJSON(val, fn)
		}
	}
}

func numAny(v any) int {
	switch x := v.(type) {
	case float64:
		return int(x)
	case int:
		return x
	case json.Number:
		n, _ := x.Int64()
		return int(n)
	case string:
		var n int
		fmt.Sscanf(x, "%d", &n)
		return n
	}
	return 0
}

func strAny(v any) string {
	s, _ := v.(string)
	return s
}

// getJSON és un GET curt (4 s) amb la clau, que descodifica a out.
func getJSON(ctx context.Context, url, apiKey string, out any) error {
	return doJSON(ctx, http.MethodGet, url, apiKey, nil, out)
}

func doJSON(ctx context.Context, method, url, apiKey string, body any, out any) error {
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	var rd *strings.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return err
		}
		rd = strings.NewReader(string(raw))
	}
	var req *http.Request
	var err error
	if rd != nil {
		req, err = http.NewRequestWithContext(ctx, method, url, rd)
	} else {
		req, err = http.NewRequestWithContext(ctx, method, url, nil)
	}
	if err != nil {
		return err
	}
	if rd != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if k := strings.TrimSpace(apiKey); k != "" {
		req.Header.Set("Authorization", "Bearer "+k)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return netAmable(url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("%s: HTTP %d", url, resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// Slots de llama.cpp.
//
// Els /props es demanen una vegada per endpoint i procés. La sessió va al
// seu slot (0 per defecte) i les crides laterals a l'últim, si n'hi ha més
// d'un. Si el slot que tocaria ja té una petició en vol d'aquest mateix
// procés (subagents en paral·lel), no es fixa: amb id_slot la petició
// faria cua darrere de l'altra, i sense, el servidor tria un slot lliure.
type slotsEndpoint struct {
	once  sync.Once
	total int
	mu    sync.Mutex
	vol   map[int]int // slot → peticions en vol
}

var slotsPerBase sync.Map // normBase → *slotsEndpoint

func slotsFor(ctx context.Context, baseURL, apiKey string) *slotsEndpoint {
	v, _ := slotsPerBase.LoadOrStore(normBase(baseURL), &slotsEndpoint{vol: map[int]int{}})
	s := v.(*slotsEndpoint)
	s.once.Do(func() {
		// Un context propi: la crida pot ser cancel·lada i el resultat
		// s'ha de poder reaprofitar.
		pctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
		defer cancel()
		if p, err := ProbeLlamaProps(pctx, baseURL, apiKey); err == nil {
			s.total = p.TotalSlots
		}
	})
	return s
}

// resetSlots oblida els slots coneguts (tests).
func resetSlots() { slotsPerBase = sync.Map{} }

// prenSlot tria l'id_slot d'una petició (ok=false = no se n'envia) i torna
// la funció que l'allibera quan la resposta s'ha llegit.
func prenSlot(ctx context.Context, baseURL, apiKey string) (slot int, ok bool, allibera func()) {
	s := slotsFor(ctx, baseURL, apiKey)
	o := localOptionsFor(baseURL)
	want := 0
	if o.Slot != nil {
		want = *o.Slot
	}
	if isSideCall(ctx) {
		switch {
		case o.SideSlot != nil:
			want = *o.SideSlot
		case s.total > 1:
			want = s.total - 1
		default:
			want = -1
		}
	}
	if want < 0 || s.total < 2 || want >= s.total {
		// Amb un sol slot (o sense saber-ne res) fixar-lo no aporta res i
		// un id fora de rang és un error del servidor.
		return 0, false, func() {}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.vol[want] > 0 {
		return 0, false, func() {}
	}
	s.vol[want]++
	var un sync.Once
	return want, true, func() {
		un.Do(func() {
			s.mu.Lock()
			s.vol[want]--
			s.mu.Unlock()
		})
	}
}

// LocalTimings és el rendiment declarat pel servidor local a l'última crida.
type LocalTimings struct {
	// CacheN són els tokens del prompt que han sortit de la caché de KV.
	CacheN int
	// PromptN són els tokens del prompt processats de nou (prefill).
	PromptN int
	// PromptPerSecond és la velocitat de prefill.
	PromptPerSecond float64
	// PredictedN i PredictedPerSecond són la generació.
	PredictedN         int
	PredictedPerSecond float64
	// LoadMs és el temps de càrrega del model (Ollama: ha de ser ~0 a
	// partir del segon torn; si no, el model s'està tornant a carregar).
	LoadMs float64
}

// llamaTimings és l'objecte timings de llama-server.
type llamaTimings struct {
	CacheN             int     `json:"cache_n"`
	PromptN            int     `json:"prompt_n"`
	PromptMs           float64 `json:"prompt_ms"`
	PromptPerSecond    float64 `json:"prompt_per_second"`
	PredictedN         int     `json:"predicted_n"`
	PredictedMs        float64 `json:"predicted_ms"`
	PredictedPerSecond float64 `json:"predicted_per_second"`
}

func (t *llamaTimings) local() LocalTimings {
	return LocalTimings{CacheN: t.CacheN, PromptN: t.PromptN, PromptPerSecond: t.PromptPerSecond, PredictedN: t.PredictedN, PredictedPerSecond: t.PredictedPerSecond}
}

// ambTimings completa l'usage amb els timings: cache_n passa a
// cached_tokens si el servidor no l'ha posat, i si no hi ha usage (versions
// antigues en streaming) se'n fa un dels timings.
func ambTimings(u Usage, t *LocalTimings) Usage {
	if t == nil {
		return u
	}
	if u.Buida() {
		u.PromptTokens = t.PromptN + t.CacheN
		u.CompletionTokens = t.PredictedN
		u.TotalTokens = u.PromptTokens + u.CompletionTokens
	}
	if u.PromptTokensDetails.CachedTokens == 0 && t.CacheN > 0 {
		u.PromptTokensDetails.CachedTokens = t.CacheN
	}
	return u
}

// LastTimings torna els timings de l'última crida a un servidor local
// (ok=false si el servidor no n'ha donat).
func (c *Client) LastTimings() (LocalTimings, bool) {
	c.usageMu.Lock()
	defer c.usageMu.Unlock()
	if c.lastTimings == nil {
		return LocalTimings{}, false
	}
	return *c.lastTimings, true
}

func (c *Client) setTimings(t *LocalTimings) {
	c.usageMu.Lock()
	c.lastTimings = t
	c.usageMu.Unlock()
}

// llamaExtras posa cache_prompt i id_slot a una petició cap a llama-server.
// Torna la funció que allibera el slot.
func (c *Client) llamaExtras(ctx context.Context, baseURL, apiKey string, req *chatRequest) func() {
	if ProviderTypeFor(baseURL) != ProviderLlamaCpp {
		return func() {}
	}
	t := true
	req.CachePrompt = &t
	slot, ok, allibera := prenSlot(ctx, baseURL, apiKey)
	if ok {
		req.IDSlot = &slot
	}
	return allibera
}
