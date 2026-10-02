package config

import "strings"

// Catàleg de proveïdors coneguts.
//
// Donar d'alta un endpoint volia dir saber-se la URL de memòria i
// escriure `/provider add <nom> <url>`. Qui arrenca Gregal per primera
// vegada es troba un config d'exemple que apunta a un llama.cpp a
// localhost:8089 que no té, el primer missatge peta amb un error HTTP i
// no hi ha res que digui per on començar. Amb el catàleg, triar
// «OpenRouter» ja porta la URL bona i el nom de la variable d'entorn
// habitual; només falta enganxar la clau.
//
// Tots són endpoints OpenAI-compatibles: és l'únic protocol que parla el
// client (internal/llm). Si te'n falta un, `/provider add` a mà continua
// existint.

// ProveidorConegut és una entrada del catàleg.
type ProveidorConegut struct {
	// Nom és el que tindrà al config (clau de `providers:`).
	Nom string `json:"name"`
	// Etiqueta és com es diu de cara a la persona.
	Etiqueta string `json:"label"`
	// BaseURL és l'endpoint OpenAI-compatible, ja amb /v1 si en porta.
	BaseURL string `json:"url"`
	// EnvVar és la variable d'entorn on la gent sol tenir la clau.
	EnvVar       string `json:"env_var"`
	DefaultModel string `json:"default_model"`
	// Local diu que no necessita clau (corre a la teva màquina).
	Local bool `json:"local"`
	// Nota és una línia d'ajuda (on es treu la clau, què cal tenir viu).
	Nota string `json:"note"`
}

// Cataleg retorna els proveïdors coneguts, primer els locals (no
// demanen clau ni compte) i després els de pagament per ordre d'ús.
func Cataleg() []ProveidorConegut {
	return []ProveidorConegut{
		{Nom: "ollama", Etiqueta: "Ollama", BaseURL: "http://localhost:11434/v1", Local: true,
			Nota: "cal tenir `ollama serve` en marxa"},
		{Nom: "lmstudio", Etiqueta: "LM Studio", BaseURL: "http://localhost:1234/v1", Local: true,
			Nota: "engega el servidor local des de la pestanya Developer"},
		{Nom: "llamacpp", Etiqueta: "llama.cpp", BaseURL: "http://localhost:8080/v1", Local: true,
			Nota: "llama-server --port 8080"},
		{Nom: "openrouter", Etiqueta: "OpenRouter", BaseURL: "https://openrouter.ai/api/v1", EnvVar: "OPENROUTER_API_KEY", DefaultModel: "openai/gpt-4.1-mini",
			Nota: "clau a openrouter.ai/keys"},
		{Nom: "openai", Etiqueta: "OpenAI", BaseURL: "https://api.openai.com/v1", EnvVar: "OPENAI_API_KEY", DefaultModel: "gpt-4.1-mini",
			Nota: "clau a platform.openai.com/api-keys"},
		{Nom: "groq", Etiqueta: "Groq", BaseURL: "https://api.groq.com/openai/v1", EnvVar: "GROQ_API_KEY", DefaultModel: "openai/gpt-oss-120b",
			Nota: "clau a console.groq.com/keys"},
		{Nom: "deepseek", Etiqueta: "DeepSeek", BaseURL: "https://api.deepseek.com", EnvVar: "DEEPSEEK_API_KEY", DefaultModel: "deepseek-flash",
			Nota: "clau a platform.deepseek.com"},
		{Nom: "mistral", Etiqueta: "Mistral", BaseURL: "https://api.mistral.ai/v1", EnvVar: "MISTRAL_API_KEY", DefaultModel: "mistral-small-latest",
			Nota: "clau a console.mistral.ai"},
		{Nom: "together", Etiqueta: "Together AI", BaseURL: "https://api.together.xyz/v1", EnvVar: "TOGETHER_API_KEY", DefaultModel: "Qwen/Qwen3-Coder-480B-A35B-Instruct-FP8",
			Nota: "clau a api.together.ai/settings/api-keys"},
		{Nom: "cerebras", Etiqueta: "Cerebras", BaseURL: "https://api.cerebras.ai/v1", EnvVar: "CEREBRAS_API_KEY", DefaultModel: "gpt-oss-120b",
			Nota: "clau a cloud.cerebras.ai"},
		{Nom: "zen", Etiqueta: "OpenCode Go", BaseURL: "https://opencode.ai/zen/go/v1", EnvVar: "ZEN_API_KEY", DefaultModel: "glm-5.3-flash",
			Nota: "el pla Go d'opencode.ai"},
	}
}

// ProveidorPerNom busca una entrada del catàleg (ok=false si no hi és).
func ProveidorPerNom(nom string) (ProveidorConegut, bool) {
	for _, p := range Cataleg() {
		if strings.EqualFold(p.Nom, nom) {
			return p, true
		}
	}
	return ProveidorConegut{}, false
}

// NomLliure retorna un nom que encara no és al config: «openai»,
// «openai-2»… Així afegir dues vegades el mateix proveïdor no en
// sobreescriu la configuració anterior.
func (c *Config) NomLliure(base string) string {
	if c == nil || c.Providers == nil {
		return base
	}
	if _, ok := c.Providers[base]; !ok {
		return base
	}
	for i := 2; i < 100; i++ {
		nom := base + "-" + string(rune('0'+i/10)) + string(rune('0'+i%10))
		if i < 10 {
			nom = base + "-" + string(rune('0'+i))
		}
		if _, ok := c.Providers[nom]; !ok {
			return nom
		}
	}
	return base
}
