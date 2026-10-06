package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
	"gregal/internal/llm"
	"gregal/internal/tema"
	"gregal/internal/tools"
)

// Provider és un backend OpenAI-compatible (llama.cpp local, proxy, cloud...).
type Provider struct {
	BaseURL string `yaml:"base_url"`
	APIKey  string `yaml:"api_key"`
	// PromptCacheKey és opcional: els providers OpenAI-compatible que ho
	// suporten el fan servir per agrupar prefixes iguals. Buit = wire clàssic.
	PromptCacheKey       string `yaml:"prompt_cache_key"`
	PromptCacheRetention string `yaml:"prompt_cache_retention"`
}

// MaskedKey resumeix la clau sense exposar-la: "(buida)", "${VAR}" o "sí".
func (p Provider) MaskedKey() string {
	switch {
	case strings.TrimSpace(p.APIKey) == "":
		return "(buida)"
	case strings.HasPrefix(strings.TrimSpace(p.APIKey), "${"):
		return strings.TrimSpace(p.APIKey)
	default:
		return "sí (oculta)"
	}
}

// Role lliga una feina (chat, codi, revisió...) a un provider + model + paràmetres.
type Role struct {
	Provider      string  `yaml:"provider"`
	Model         string  `yaml:"model"`
	LongRun       *bool   `yaml:"long_run,omitempty"`
	Temperature   float64 `yaml:"temperature"`
	MaxTokens     int     `yaml:"max_tokens"`
	ContextWindow int     `yaml:"context_window"`
	// Fallback: si el provider primari falla (xarxa, 429, 5xx), ho torna
	// a provar aquí. Buit = sense failover. P. ex. primari local-swap,
	// fallback cloud per quan el Strix és apagat (o a l'inrevés).
	FallbackProvider string `yaml:"fallback_provider,omitempty"`
	FallbackModel    string `yaml:"fallback_model"`
	// Think apaga el raonament del model quan el rol no el necessita. Un agent
	// de codi fa moltes peticions per tasca, i el pensament compta dins del
	// mateix temps: mesurat amb Halogen, cada pas amb raonament OF gastava
	// 2-3x més temps i tokens, i els passos llargs arribaven a 8 minuts
	// de pensament abans d'escriure res. «no» desactiva el raonament i low,
	// medium, high i max fixen la intensitat en proveïdors compatibles.
	// «auto» raona on aporta (planificar, decidir després de llegir, un
	// vermell) i no als passos mecànics (després d'una edició aplicada o
	// d'un verd, l'ampliació i la síntesi): agent.ThinkPerPas.
	// Buit = el que el model tingui per defecte.
	Think string `yaml:"think,omitempty"`
}

// LongRunsAllowed opta aquest rol a les tasques llargues. Nil conserva el
// comportament de configs antigues; l'usuari pot desactivar-lo explícitament.
func (r Role) LongRunsAllowed() bool { return r.LongRun == nil || *r.LongRun }

func (c *Config) LongRunsAllowed(provider, model string) bool {
	if c == nil {
		return true
	}
	for _, r := range c.Roles {
		if r.Provider == provider && r.Model == model {
			return r.LongRunsAllowed()
		}
	}
	return true
}

// FallbackTarget resol el fallback d'un rol: nil si no n'hi ha configurat
// o el provider no existeix (aleshores no hi ha failover, no error).
func (c *Config) FallbackTarget(r Role) *llm.Target {
	if strings.TrimSpace(r.FallbackProvider) == "" || strings.TrimSpace(r.FallbackModel) == "" {
		return nil
	}
	p, ok := c.Providers[r.FallbackProvider]
	if !ok {
		return nil
	}
	return &llm.Target{BaseURL: p.BaseURL, APIKey: p.APIKey, Model: r.FallbackModel}
}

// PrimTarget és el target primari d'un rol amb el seu provider.
func (c *Config) PrimTarget(p Provider, r Role) llm.Target {
	return llm.Target{BaseURL: p.BaseURL, APIKey: p.APIKey, Model: r.Model}
}

// ConfigureClient registra les opcions de cache dels providers al client
// compartit. Els camps només s'envien quan s'han configurat explícitament:
// així un endpoint local o un proxy antic no rep camps nous que no entengui.
func (c *Config) ConfigureClient(client *llm.Client) {
	if c == nil || client == nil {
		return
	}
	for name, p := range c.Providers {
		if strings.TrimSpace(p.PromptCacheKey) == "" {
			continue
		}
		models := map[string]bool{}
		for _, r := range c.Roles {
			if r.Provider == name && r.Model != "" {
				models[r.Model] = true
			}
			if r.FallbackProvider == name && r.FallbackModel != "" {
				models[r.FallbackModel] = true
			}
		}
		for model := range models {
			client.SetPromptCache(p.BaseURL, model, p.PromptCacheKey, p.PromptCacheRetention)
		}
	}
}

// VerifyCfg controla el mode de verificació.
// mode: off | manual | auto | both | strict
// strict = auto + teeth: un CAL REVISAR bloqueja el pròxim torn d'agent
// web fins a aprovar-lo explícitament.
type VerifyCfg struct {
	Mode string `yaml:"mode"`
}

// AutonomousCfg controla el mode autònom de llarg abast. Els valors zero es
// completen amb defaults quan es carrega el config.
type AutonomousCfg struct {
	CheckpointEvery int      `yaml:"checkpoint_every"`
	ReviewEvery     int      `yaml:"review_every"`
	MaxMinutes      int      `yaml:"max_minutes"`
	MaxToolSteps    int      `yaml:"max_tool_steps"`
	MaxCostUSD      float64  `yaml:"max_cost_usd"`
	ReviewerRole    string   `yaml:"reviewer_role"`
	Verify          []string `yaml:"verify"`
}

// AgentCfg controla l'agent loop (/agent).
type AgentCfg struct {
	MaxSteps   int           `yaml:"max_steps"`
	Autonomous AutonomousCfg `yaml:"autonomous"`
	// ApprovalTimeoutS és quants segons espera la web una aprovació o una
	// pregunta abans de donar-la per denegada. 0 (sense posar) = 30 minuts;
	// negatiu = sense límit, fins que l'usuari respon o atura el torn.
	ApprovalTimeoutS int `yaml:"approval_timeout_s,omitempty"`
}

// ApprovalTimeout és l'espera efectiva d'una aprovació (0 = sense límit).
// Abans eren 120 s fixos: qui s'aixecava a fer un cafè tornava i trobava
// l'acció denegada i el model provant una altra cosa. El TUI i Telegram
// ja esperaven sense límit.
func (c *Config) ApprovalTimeout() time.Duration {
	switch s := c.Agent.ApprovalTimeoutS; {
	case s < 0:
		return 0
	case s == 0:
		return 30 * time.Minute
	default:
		return time.Duration(s) * time.Second
	}
}

// AutonomousConfig retorna els valors efectius encara que el Config s'hagi
// construït directament en un test o per una integració, sense passar per
// Load.
func (c *Config) AutonomousConfig() AutonomousCfg {
	a := c.Agent.Autonomous
	if a.CheckpointEvery <= 0 {
		a.CheckpointEvery = 10
	}
	if a.ReviewEvery <= 0 {
		a.ReviewEvery = 2
	}
	if a.MaxMinutes <= 0 {
		a.MaxMinutes = 600
	}
	if a.MaxToolSteps <= 0 {
		a.MaxToolSteps = 2000
	}
	if strings.TrimSpace(a.ReviewerRole) == "" {
		a.ReviewerRole = "reviewer"
	}
	return a
}

// CostPrice és USD per milió de tokens (clau: nom de model o provider/model).
type CostPrice struct {
	In  float64 `yaml:"in"`
	Out float64 `yaml:"out"`
}

// HooksCfg controla els hooks post-eina (estil PostToolUse).
// post_edit: plantilla amb {file} que corre (sh -c, 30s) després de cada
// write/edit amb èxit; la sortida s'anota al resultat de l'eina.
// Exemples: "gofmt -w {file}", "ruff check {file}".
// Buit = desactivat. És config teva i de confiança: no hi posis res que no
// executaries tu mateix al terminal.
type HooksCfg struct {
	PostEdit string `yaml:"post_edit"`
	// Diag és la comprovació després d'editar: "full" (defecte: sintaxi
	// en procés + go vet / py_compile / node --check / bash -n si són al
	// PATH) o "syntax" (només la sintaxi en procés, com abans).
	Diag string `yaml:"diag"`
}

// PermissionsCfg controla permisos d'eines (allow|ask|deny).
type PermissionsCfg struct {
	Tools     map[string]string `yaml:"tools"`
	BashAllow []string          `yaml:"bash_allow"`
	BashDeny  []string          `yaml:"bash_deny"`
}

// MCPServerCfg és un servidor MCP (transport stdio).
type MCPServerCfg struct {
	Command string            `yaml:"command"`
	Args    []string          `yaml:"args"`
	Env     map[string]string `yaml:"env"`
}

// Config és l'arrel del config.yaml.
type Config struct {
	Providers   map[string]Provider  `yaml:"providers"`
	Roles       map[string]Role      `yaml:"roles"`
	Verify      VerifyCfg            `yaml:"verify"`
	Agent       AgentCfg             `yaml:"agent"`
	Router      RouterCfg            `yaml:"router"`
	Budget      BudgetCfg            `yaml:"budget"`
	Cost        map[string]CostPrice `yaml:"cost"`
	Hooks       HooksCfg             `yaml:"hooks"`
	System      string               `yaml:"system"`
	Permissions PermissionsCfg       `yaml:"permissions"`
	Mode        string               `yaml:"mode"`
	// Language és l'idioma de la interfície i de les respostes: ca o en.
	// Buit = en. Els comentaris i els tests del codi continuen en català;
	// això només és el que llegeix qui fa servir el programa.
	Language string `yaml:"lang"`
	// Theme és el tema del TUI: qualsevol de TemesSuportats (per
	// defecte, fosc). La finestra ja en tenia un, de tema, i el TUI era
	// fosc i prou: en un terminal de fons clar, els grisos apagats de la
	// paleta fosca no es llegeixen.
	Theme string `yaml:"theme"`
	// Mouse és el ratolí del TUI: on (roda per desplaçar; bloqueja la
	// selecció nativa) o off (selecció nativa; sense roda). Buit = on.
	// Es commuta en calent amb /mouse i es desa.
	Mouse string `yaml:"mouse"`
	// Animacions són els moviments del TUI que no informen de res: la mar
	// de la capçalera, la mascota que respira i la marea del mesurador.
	// Buit o off = quietes (una eina on es passen hores no pot tenir res
	// que es mogui sol); on = com abans. El spinner de feina no compta:
	// aquell sí que diu una cosa, i es queda sempre.
	Animacions string `yaml:"animacions"`
	// Cockpit és la columna de la dreta del TUI (tasca, canvis, cua,
	// context, validació) quan el terminal fa 120 columnes o més. Buit o
	// on = hi és; off = amagada. Ctrl+T la commuta i ho desa aquí.
	Cockpit  string                  `yaml:"cockpit"`
	MCP      map[string]MCPServerCfg `yaml:"mcp"`
	Telegram TelegramCfg             `yaml:"telegram"`
	// Users són els comptes que poden entrar al servidor web i a l'app
	// (nom → contrasenya i carpetes permeses). Sense secció `users:` tot
	// queda com abans: només el token únic de sempre.
	Users map[string]UserCfg `yaml:"users"`

	// rawKeys conserva les api_key TAL COM al fitxer (sense expandir ${VAR}),
	// perquè Save no congeli secrets ni variables d'entorn al disc.
	rawKeys map[string]string `yaml:"-"`
	// rawToken conserva el token de Telegram tal com és al fitxer (o ${VAR}).
	rawToken string `yaml:"-"`
}

// UserCfg és un compte del servidor web i de l'app.
//
// La contrasenya mai s'escriu en clar: es desa com
// `pbkdf2$sha256$iters$salt$hash` (vegeu web.HashPassword). `roots` són les
// carpetes on aquest usuari pot navegar, obrir i editar; a fora, l'API de
// fitxers diu que no. Un usuari sense `roots` només pot xatejar.
type UserCfg struct {
	Password string   `yaml:"password"`
	Roots    []string `yaml:"roots"`
	// Home és on comencen les sessions de codi d'aquest usuari (les seves
	// carpetes de treball). Buit = la primera de `roots`.
	Home  string `yaml:"home"`
	Admin bool   `yaml:"admin"`
	// TelegramID lliga aquest compte amb un usuari de Telegram: el bot li
	// dona sessió i carpeta pròpies (la seva `home`). 0 = sense lligam
	// (l'usuari entra per `telegram.allowed_users`, amb el projecte global).
	TelegramID int64 `yaml:"telegram_id"`
	// Unattended permet a aquest compte entrar al mode autònom (eines sense
	// cap aprovació). Per defecte només ho pot fer qui és a allowed_users.
	Unattended bool `yaml:"unattended"`
}

// TelegramCfg configura el bot de Telegram.
//
// El token s'escriu al config (0600) o s'hi posa ${GREGAL_TELEGRAM_TOKEN}.
// allowed_users és la llista d'usuaris que poden fer servir el bot i aprovar
// eines; allow_groups permet fer-lo servir en grups on el bot sigui present.
type TelegramCfg struct {
	Token        string  `yaml:"token"`
	AllowedUsers []int64 `yaml:"allowed_users"`
	AllowGroups  bool    `yaml:"allow_groups"`
}

// defaultSystem és la identitat del gregal quan el config no en defineix.
const defaultSystem = `Ets el gregal, un agent de codi que viu en un TUI. Parles SEMPRE en català (informal i directe, sense palla). Respostes curtes: primer la solució, després l'explicació mínima. Quan faràs servir eines, avisa en una línia què faràs.`

// defaultSystemEN és el mateix amb l'anglès com a llengua de resposta. Es
// tria amb `lang: en`. No és una traducció automàtica del de dalt: el
// prompt és el que decideix com respon el model i val més tenir-lo escrit
// a mà en cada llengua que passar-lo per una plantilla.
const defaultSystemEN = `You are gregal, a coding agent living in a TUI. You ALWAYS speak English (informal and direct, no filler). Short answers: the solution first, then the minimum explanation. When you are about to use tools, say in one line what you will do.`

// LangsSuportats són els idiomes de la interfície i de les respostes.
var LangsSuportats = []string{"ca", "en"}

var TemesSuportats = tema.Noms()

// Tema torna el tema efectiu del TUI (per defecte, fosc).
func (c *Config) Tema() string {
	for _, t := range TemesSuportats {
		if c.Theme == t {
			return t
		}
	}
	return "fosc"
}

// Lang torna l'idioma efectiu (per defecte, anglès).
func (c *Config) Lang() string {
	for _, l := range LangsSuportats {
		if c.Language == l {
			return l
		}
	}
	return "en"
}

// MouseOn diu si el ratolí captura (roda per desplaçar) o deixa la
// selecció nativa del terminal. Per defecte activat; "off" el desactiva.
// Es commuta en calent amb /mouse.
func (c *Config) MouseOn() bool {
	return c.Mouse != "off"
}

// AnimacionsOn diu si els moviments decoratius del TUI estan encesos.
// Per defecte no: es demanen explícitament amb `animacions: on`.
func (c *Config) AnimacionsOn() bool {
	return c.Animacions == "on"
}

// CockpitOn diu si el cockpit va en columna quan el terminal és prou ample.
// Per defecte sí: s'amaga amb Ctrl+T o amb `cockpit: off`.
func (c *Config) CockpitOn() bool {
	return c.Cockpit != "off"
}

const defaultYAML = `# Gregal configuration. Choose any OpenAI-compatible provider and model.
lang: en
# Public provider presets. Supply only your own API key, then select a provider.
# Keys are read from environment variables; no credentials are bundled.

providers:
  ollama:
    base_url: http://localhost:11434/v1
    api_key: ""
  lmstudio:
    base_url: http://localhost:1234/v1
    api_key: ""
  llamacpp:
    base_url: http://localhost:8080/v1
    api_key: ""
  openai:
    base_url: https://api.openai.com/v1
    api_key: ${OPENAI_API_KEY}
  openrouter:
    base_url: https://openrouter.ai/api/v1
    api_key: ${OPENROUTER_API_KEY}
  groq:
    base_url: https://api.groq.com/openai/v1
    api_key: ${GROQ_API_KEY}
  deepseek:
    base_url: https://api.deepseek.com
    api_key: ${DEEPSEEK_API_KEY}
  mistral:
    base_url: https://api.mistral.ai/v1
    api_key: ${MISTRAL_API_KEY}
  together:
    base_url: https://api.together.xyz/v1
    api_key: ${TOGETHER_API_KEY}
  cerebras:
    base_url: https://api.cerebras.ai/v1
    api_key: ${CEREBRAS_API_KEY}
  zen:
    base_url: https://opencode.ai/zen/go/v1
    api_key: ${ZEN_API_KEY}

# context_window: 0 = la que declari el model a GET /models (es detecta a
# l'arrencada i s'aprèn dels errors de context). Posa un nombre fix només si
# el proveïdor no ho diu i te la saps.
roles:
  chat:
    provider: openai
    model: gpt-4.1-mini
    temperature: 0.7
    max_tokens: 1024
  think:
    provider: openai
    model: gpt-4.1-mini
    temperature: 0.6
    max_tokens: 8192
  code:
    provider: openai
    model: gpt-4.1-mini
    temperature: 0.4
    max_tokens: 8192
    context_window: 0
    long_run: true
  reviewer:
    provider: openai
    model: gpt-4.1-mini
    temperature: 0.2
    max_tokens: 1024

verify:
  mode: both

# router: tria de rol per tasca (auto = pregunta→chat barat, pesada→strong_role).
router:
  mode: auto
  strong_role: ""
  escalate_after: 0

# budget: avís quan la sessió web supera N USD (0 = apagat).
budget:
  session_usd: 0

agent:
  max_steps: 10
  # Mode autònom: treballa per fites, verifica i reprèn mentre hi hagi progrés.
  autonomous:
    checkpoint_every: 10
    review_every: 2
    max_minutes: 600
    max_tool_steps: 2000
    max_cost_usd: 0
    reviewer_role: reviewer
    verify: []

# hooks: post_edit corre ({file}) després de cada write/edit amb èxit.
# diag: full (defecte: sintaxi + go vet / py_compile / node --check / bash -n
# sobre el fitxer editat, si l'eina és al PATH) o syntax (només sintaxi).
# Exemples: "gofmt -w {file}" · "ruff check --fix {file}" · "" = desactivat.
hooks:
  post_edit: ""

# cost: USD per milió de tokens (in/out) per model cloud. Sense entrada =
# sense cost (locals sempre $0 implícit: no cal llistar-los). Exemple amb
# preus orientatius — revisa els vigents del teu provider:
#cost:
#  kimi-k2.7-code: {in: 0.6, out: 2.5}

# system: custom instructions (empty = built-in prompt in the selected language).
system: ""

# permissions: allow|ask|deny per eina + llistes extra per bash. patch és
# edició atòmica; delegate/read_image són eines locals i gh_issue/gh_pr són
# consultes GitHub read-only (requereixen la CLI gh).
permissions:
  tools: {write: ask, edit: ask}
  bash_allow: []
  bash_deny: []

# mode: code (accés total amb confirmacions) o chat (només lectura).
mode: code

# mouse: on (roda per desplaçar; bloqueja la selecció nativa) o off
# (selecció nativa del terminal; sense roda). Buit = on. També amb /mouse.
#mouse: "on"

# animacions: on encén la mar de la capçalera, la mascota i la marea del
# mesurador. Per defecte són quietes; el spinner de feina hi és sempre.
#animacions: "off"

# cockpit: la columna de la dreta del TUI (tasca, canvis, cua, context,
# validació) quan el terminal fa 120 columnes o més. Ctrl+T la commuta.
#cockpit: "on"

# mcp: servidors Model Context Protocol (stdio). Les seves tools arriben a
# l'agent com mcp_<servidor>_<eina> (per defecte demanen confirmació).
#mcp:
#  recordatoris:
#    command: /usr/bin/python3
#    args: [/path/to/mcp-server.py]
`

// DefaultPath retorna ~/.config/gregal/config.yaml.
func DefaultPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "config.yaml"
	}
	return filepath.Join(home, ".config", "gregal", "config.yaml")
}

// DefaultYAML retorna la plantilla de configuració per defecte.
func DefaultYAML() string { return defaultYAML }

// Load carrega el config; si no existeix, el crea amb valors per defecte.
func Load(path string) (*Config, string, error) {
	if path == "" {
		path = DefaultPath()
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			return nil, path, err
		}
		if err := os.WriteFile(path, []byte(defaultYAML), 0o600); err != nil {
			return nil, path, err
		}
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, path, err
	}
	var c Config
	if err := yaml.Unmarshal(raw, &c); err != nil {
		return nil, path, fmt.Errorf("yaml: %w", err)
	}
	// Expandeix ${VAR} a les api_keys (conserva l'original per Save).
	c.rawKeys = map[string]string{}
	for name, p := range c.Providers {
		c.rawKeys[name] = p.APIKey
		p.APIKey = os.ExpandEnv(p.APIKey)
		c.Providers[name] = p
	}
	// Claus desades fora del config (~/.config/gregal/auth.yaml): només
	// omplen les que el config deixa buides.
	c.aplicaAuth()
	c.rawToken = c.Telegram.Token
	c.Telegram.Token = os.ExpandEnv(c.Telegram.Token)
	if c.Agent.MaxSteps <= 0 {
		// 10 passos és poc per a una tasca de codi real: llegir quatre
		// fitxers, fer un grep i editar-ne dos ja te'ls menja, i el torn
		// s'acabava a mitges. Cap agent de referència posa un sostre tan
		// baix. 40 és marge per treballar; qui vulgui frenar-ho ho baixa al
		// config, i Esc atura el torn en qualsevol moment.
		c.Agent.MaxSteps = 40
	}
	if c.Agent.Autonomous.CheckpointEvery <= 0 {
		c.Agent.Autonomous.CheckpointEvery = 10
	}
	if c.Agent.Autonomous.ReviewEvery <= 0 {
		c.Agent.Autonomous.ReviewEvery = 2
	}
	if c.Agent.Autonomous.MaxMinutes <= 0 {
		c.Agent.Autonomous.MaxMinutes = 600
	}
	if c.Agent.Autonomous.MaxToolSteps <= 0 {
		c.Agent.Autonomous.MaxToolSteps = 2000
	}
	if strings.TrimSpace(c.Agent.Autonomous.ReviewerRole) == "" {
		c.Agent.Autonomous.ReviewerRole = "reviewer"
	}
	if err := c.Validate(); err != nil {
		return nil, path, err
	}
	return &c, path, nil
}

// Save escriu el config al fitxer (restaura les api_key originals sense
// expandir, valida abans de guardar i conserva permisos 0600).
func (c *Config) Save(path string) error {
	if path == "" {
		path = DefaultPath()
	}
	cp := *c
	if c.rawToken != "" {
		cp.Telegram.Token = c.rawToken
	}
	cp.Providers = map[string]Provider{}
	for name, p := range c.Providers {
		if raw, ok := c.rawKeys[name]; ok {
			p.APIKey = raw
		} else if strings.TrimSpace(p.APIKey) != "" && !strings.HasPrefix(strings.TrimSpace(p.APIKey), "${") {
			// Clau nova escrita en calent: no la congelem al disc.
			p.APIKey = ""
		}
		cp.Providers[name] = p
	}
	if err := cp.Validate(); err != nil {
		return err
	}
	raw, err := yaml.Marshal(&cp)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		return err
	}
	// El config pot portar secrets: força 0600 encara que el
	// fitxer ja existís amb permisos més oberts.
	return os.Chmod(path, 0o600)
}

// SetRawKey registra l'api_key original (sense expandir) d'un provider.
func (c *Config) SetRawKey(name, key string) {
	if c.rawKeys == nil {
		c.rawKeys = map[string]string{}
	}
	c.rawKeys[name] = key
}

// Validate comprova rols obligatoris, providers i mode de verificació.
func (c *Config) Validate() error {
	for _, need := range []string{"chat", "code", "reviewer"} {
		r, ok := c.Roles[need]
		if !ok {
			return fmt.Errorf("falta el rol obligatori %q", need)
		}
		if _, ok := c.Providers[r.Provider]; !ok {
			return fmt.Errorf("el rol %q apunta al provider desconegut %q", need, r.Provider)
		}
		if strings.TrimSpace(r.Model) == "" {
			return fmt.Errorf("el rol %q no té model", need)
		}
		if r.ContextWindow < 0 {
			return fmt.Errorf("el rol %q té context_window negatiu", need)
		}
	}
	switch c.Verify.Mode {
	case "off", "manual", "auto", "both", "strict":
	default:
		return fmt.Errorf("verify.mode %q invàlid (off|manual|auto|both|strict)", c.Verify.Mode)
	}
	if c.Language != "" && c.Lang() != c.Language {
		return fmt.Errorf("lang %q invàlid (%s)", c.Language, strings.Join(LangsSuportats, "|"))
	}
	if c.Mouse != "" && c.Mouse != "on" && c.Mouse != "off" {
		return fmt.Errorf("mouse %q invàlid (on|off)", c.Mouse)
	}
	if c.Animacions != "" && c.Animacions != "on" && c.Animacions != "off" {
		return fmt.Errorf("animacions %q invàlid (on|off)", c.Animacions)
	}
	if c.Cockpit != "" && c.Cockpit != "on" && c.Cockpit != "off" {
		return fmt.Errorf("cockpit %q invàlid (on|off)", c.Cockpit)
	}
	switch c.Router.Mode {
	case "", "auto", "off":
		if c.Router.Mode == "" {
			c.Router.Mode = "auto"
		}
	default:
		return fmt.Errorf("router.mode %q invàlid (auto|off)", c.Router.Mode)
	}
	if c.Router.EscalateAfter < 0 {
		return fmt.Errorf("router.escalate_after %d invàlid (>=0)", c.Router.EscalateAfter)
	}
	if c.Budget.SessionUSD < 0 {
		return fmt.Errorf("budget.session_usd %v invàlid (>=0)", c.Budget.SessionUSD)
	}
	if s := c.Router.StrongRole; s != "" {
		if _, ok := c.Roles[s]; !ok {
			return fmt.Errorf("router.strong_role %q no és cap rol", s)
		}
	}
	if c.Mode == "" {
		c.Mode = "code"
	}
	switch c.Mode {
	case "code", "chat", "inspect", "goal", "autonomous":
	default:
		return fmt.Errorf("mode %q invàlid (code|inspect|chat|goal|autonomous)", c.Mode)
	}
	if c.Agent.MaxSteps < 1 || c.Agent.MaxSteps > 200 {
		return fmt.Errorf("agent.max_steps %d fora de rang (1-200)", c.Agent.MaxSteps)
	}
	a := c.AutonomousConfig()
	if a.CheckpointEvery < 1 || a.CheckpointEvery > 200 {
		return fmt.Errorf("agent.autonomous.checkpoint_every %d fora de rang (1-200)", a.CheckpointEvery)
	}
	if a.ReviewEvery < 1 || a.ReviewEvery > 50 {
		return fmt.Errorf("agent.autonomous.review_every %d fora de rang (1-50)", a.ReviewEvery)
	}
	if a.MaxMinutes < 1 || a.MaxMinutes > 7*24*60 {
		return fmt.Errorf("agent.autonomous.max_minutes %d fora de rang (1-10080)", a.MaxMinutes)
	}
	if a.MaxToolSteps < 1 || a.MaxToolSteps > 10000 {
		return fmt.Errorf("agent.autonomous.max_tool_steps %d fora de rang (1-10000)", a.MaxToolSteps)
	}
	if a.MaxCostUSD < 0 {
		return fmt.Errorf("agent.autonomous.max_cost_usd %v invàlid (>=0)", a.MaxCostUSD)
	}
	for name, d := range c.Permissions.Tools {
		// El registre canònic és tools.Noms(): abans aquí hi havia una
		// còpia curta, i posar-hi office_edit o bash_background feia
		// fallar la càrrega d'un config que l'agent executava sense cap
		// problema.
		if !tools.EsMCP(name) && !tools.EsNativa(name) {
			return fmt.Errorf("permissions.tools.%s: eina desconeguda", name)
		}
		switch d {
		case "allow", "ask", "deny":
		default:
			return fmt.Errorf("permissions.tools.%s=%q invàlid (allow|ask|deny)", name, d)
		}
	}
	for name, s := range c.MCP {
		if strings.TrimSpace(s.Command) == "" {
			return fmt.Errorf("mcp.%s: falta command", name)
		}
	}
	return nil
}

// SystemPrompt retorna el prompt d'identitat (custom o per defecte).
func (c *Config) SystemPrompt() string {
	base := defaultSystem
	if c.Lang() == "en" {
		base = defaultSystemEN
	}
	if strings.TrimSpace(c.System) != "" {
		base = c.System
	}
	return base + userMemoryBlock()
}

// RoleNames retorna els noms de rol ordenats, per --check-config.
func (c *Config) RoleNames() string {
	names := make([]string, 0, len(c.Roles))
	for n := range c.Roles {
		names = append(names, n)
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}

// InitialRole tria el rol que correspon al mode inicial en tots els clients.
// Evita que una configuració en mode code/inspect acabi consultant el model
// barat de xat per defecte; si la plantilla és mínima, fa un fallback segur.
func (c *Config) InitialRole() string {
	if c == nil {
		return "chat"
	}
	preferred := []string{"chat"}
	switch c.Mode {
	case "code", "autonomous":
		preferred = []string{"code", "chat", "think"}
	case "inspect":
		preferred = []string{"think", "chat", "code"}
	case "goal":
		preferred = []string{"chat", "think", "code"}
	}
	for _, name := range preferred {
		if r, ok := c.Roles[name]; ok && strings.TrimSpace(r.Model) != "" {
			return name
		}
	}
	names := make([]string, 0, len(c.Roles))
	for name, r := range c.Roles {
		if name != "reviewer" && strings.TrimSpace(r.Model) != "" {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	if len(names) > 0 {
		return names[0]
	}
	return "chat"
}
