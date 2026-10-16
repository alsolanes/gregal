package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	neturl "net/url"
	"sort"
	"strings"
	"time"
)

// ProbeModels fa GET <base>/models amb la clau (Bearer) i torna els IDs
// ordenats. Serveix per provar/comprovar un provider sense xat.
// (Abans hi havia 3 còpies: web, telegram, tui — ara una de sola.)
func ProbeModels(base, key string) ([]string, error) {
	url := modelsURL(base)
	// 4s, no 10: llistar models és un GET a un endpoint que ha de ser
	// instantani. Amb 10s, un provider apagat que no rebutja sinó que
	// s'empassa la connexió feia esperar deu segons tota la llista.
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	setModelsAuth(req, base, key)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, netAmable(url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %d (revisa URL/clau)", resp.StatusCode)
	}
	var v struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&v); err != nil {
		return nil, err
	}
	ids := []string{}
	for _, d := range v.Data {
		if d.ID != "" {
			ids = append(ids, d.ID)
		}
	}
	sort.Strings(ids)
	return ids, nil
}

// ShortIDs resumeix una llista d'IDs (8 + "… +N").
func ShortIDs(ids []string) string {
	if len(ids) > 8 {
		ids = append(ids[:8], fmt.Sprintf("… +%d", len(ids)-8))
	}
	return strings.Join(ids, ", ")
}

// netAmable tradueix l'error de xarxa a alguna cosa que digui què fer. El
// de Go ("dial tcp [::1]:8089: connectex: No connection could be made
// because the target machine actively refused it") és exacte i no serveix
// de res a qui només vol saber si el servidor està engegat.
func netAmable(url string, err error) error {
	host := url
	if u, e := neturl.Parse(url); e == nil && u.Host != "" {
		host = u.Host
	}
	s := err.Error()
	switch {
	case errors.Is(err, context.DeadlineExceeded) || strings.Contains(s, "context deadline exceeded"):
		return fmt.Errorf("%s no respon a temps", host)
	case strings.Contains(s, "refused"):
		return fmt.Errorf("%s no accepta connexions (el servidor no està engegat?)", host)
	case strings.Contains(s, "no such host"):
		return fmt.Errorf("%s no existeix (revisa l'adreça)", host)
	}
	return fmt.Errorf("%s: %w", host, err)
}

// ProbeModelWindows fa GET <base>/models i torna, per a cada model que ho
// declari, la finestra de context en tokens. Els servidors OpenAI-compatibles
// ho diuen amb noms diferents: context_length (llama-swap, OpenRouter,
// strix), context_window, max_model_len (vLLM), max_context_length,
// n_ctx / meta.n_ctx_train (llama.cpp), max_input_tokens. Els que no en
// diuen res (OpenAI, opencode zen) simplement no surten al mapa.
func ProbeModelWindows(base, key string) (map[string]int, error) {
	url := modelsURL(base)
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	setModelsAuth(req, base, key)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, netAmable(url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %d (revisa URL/clau)", resp.StatusCode)
	}
	var v struct {
		Data []map[string]any `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&v); err != nil {
		return nil, err
	}
	out := map[string]int{}
	for _, d := range v.Data {
		id, _ := d["id"].(string)
		if id == "" {
			continue
		}
		if n := windowFromModel(d); n > 0 {
			out[id] = n
		}
	}
	return out, nil
}

// windowFromModel busca la finestra als camps coneguts (també dins de
// meta i top_provider).
func windowFromModel(d map[string]any) int {
	num := func(v any) int {
		switch x := v.(type) {
		case float64:
			return int(x)
		case int:
			return x
		case string:
			var n int
			fmt.Sscanf(x, "%d", &n)
			return n
		}
		return 0
	}
	for _, k := range []string{"context_length", "context_window", "max_model_len", "max_context_length", "n_ctx", "max_input_tokens", "input_token_limit"} {
		if n := num(d[k]); n > 0 {
			return n
		}
	}
	for _, sub := range []string{"meta", "top_provider", "capabilities", "limits"} {
		if m, ok := d[sub].(map[string]any); ok {
			for _, k := range []string{"n_ctx_train", "n_ctx", "context_length", "context_window", "max_context_length", "context"} {
				if n := num(m[k]); n > 0 {
					return n
				}
			}
		}
	}
	return 0
}

// modelsURL és <base>/models; a Ollama natiu, <arrel>/v1/models (el
// base_url pot no portar /v1).
func modelsURL(base string) string {
	if ProviderTypeFor(base) == ProviderOllama {
		return serverRoot(base) + "/v1/models"
	}
	return endpointURL(base, "/models")
}

// LMStudioModel és el que diu GET /api/v0/models d'un model.
type LMStudioModel struct {
	ID string
	// MaxContextLength és el màxim del model.
	MaxContextLength int
	// LoadedContextLength és el context amb què està carregat (0 = no ho diu).
	LoadedContextLength int
	// State és «loaded» o «not-loaded».
	State string
	Type  string
}

// ProbeLMStudio fa GET <arrel>/api/v0/models (API REST de LM Studio).
func ProbeLMStudio(ctx context.Context, baseURL, apiKey string) ([]LMStudioModel, error) {
	var raw struct {
		Data []map[string]any `json:"data"`
	}
	if err := getJSON(ctx, serverRoot(baseURL)+"/api/v0/models", apiKey, &raw); err != nil {
		return nil, err
	}
	var out []LMStudioModel
	for _, d := range raw.Data {
		m := LMStudioModel{
			ID:                  strAny(d["id"]),
			MaxContextLength:    numAny(d["max_context_length"]),
			LoadedContextLength: numAny(d["loaded_context_length"]),
			State:               strAny(d["state"]),
			Type:                strAny(d["type"]),
		}
		if m.ID == "" || m.Type == "embeddings" {
			continue
		}
		out = append(out, m)
	}
	if out == nil && raw.Data == nil {
		return nil, fmt.Errorf("/api/v0/models sense data: no és LM Studio")
	}
	return out, nil
}

// Finestra efectiva d'un model segons LM Studio: la carregada si ho diu.
func (m LMStudioModel) Window() int {
	if m.State == "loaded" && m.LoadedContextLength > 0 {
		return m.LoadedContextLength
	}
	return m.MaxContextLength
}

// Tipus de servidor local detectats per ProbeLocalServer.
const (
	ServerLlamaCpp = "llama.cpp"
	ServerOllama   = "ollama"
	ServerLMStudio = "lmstudio"
)

// LocalServer és el que se sap d'un servidor local.
type LocalServer struct {
	Kind     string // ServerLlamaCpp, ServerOllama, ServerLMStudio o ""
	Props    LlamaProps
	Loaded   []OllamaLoaded
	LMStudio []LMStudioModel
}

// ProbeLocalServer endevina quin servidor local hi ha darrere d'un
// base_url: el tipus declarat primer i, si no, /props (llama.cpp), /api/ps
// (Ollama) i /api/v0/models (LM Studio). Kind buit = cap dels tres.
func ProbeLocalServer(ctx context.Context, baseURL, apiKey string) LocalServer {
	try := []string{ServerLlamaCpp, ServerOllama, ServerLMStudio}
	switch ProviderTypeFor(baseURL) {
	case ProviderOllama:
		try = []string{ServerOllama}
	case ProviderLlamaCpp:
		try = []string{ServerLlamaCpp}
	case ProviderAnthropic, ProviderOpenAIResponses:
		return LocalServer{}
	}
	for _, k := range try {
		switch k {
		case ServerLlamaCpp:
			if p, err := ProbeLlamaProps(ctx, baseURL, apiKey); err == nil {
				return LocalServer{Kind: k, Props: p}
			}
		case ServerOllama:
			if l, err := ProbeOllamaPS(ctx, baseURL, apiKey); err == nil {
				return LocalServer{Kind: k, Loaded: l}
			}
		case ServerLMStudio:
			if m, err := ProbeLMStudio(ctx, baseURL, apiKey); err == nil {
				return LocalServer{Kind: k, LMStudio: m}
			}
		}
	}
	return LocalServer{}
}

// LocalWindow torna la finestra real d'un model en un servidor local, la
// que el servidor fa servir i no la d'entrenament (que és el que /v1/models
// de llama.cpp declara a meta.n_ctx_train): llama.cpp /props n_ctx (per
// slot), Ollama el num_ctx que Gregal hi envia (natiu) o el del model
// carregat (/v1), LM Studio el context carregat. ok=false si no se sap.
func LocalWindow(ctx context.Context, srv LocalServer, baseURL, apiKey, model string) (int, bool) {
	switch srv.Kind {
	case ServerLlamaCpp:
		return srv.Props.NCtx, srv.Props.NCtx > 0
	case ServerOllama:
		if ProviderTypeFor(baseURL) == ProviderOllama {
			n := OllamaNumCtx(ctx, baseURL, apiKey, model)
			return n, n > 0
		}
		for _, l := range srv.Loaded {
			if OllamaSameModel(l.Name, model) && l.ContextLength > 0 {
				return l.ContextLength, true
			}
		}
	case ServerLMStudio:
		for _, m := range srv.LMStudio {
			if m.ID == model && m.Window() > 0 {
				return m.Window(), true
			}
		}
	}
	return 0, false
}
