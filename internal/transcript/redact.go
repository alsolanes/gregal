// Package transcript converteix una conversa desada en un document que es
// pot llegir fora de l'app (Markdown o HTML autocontingut) i, si cal, la
// sanitza abans: claus i tokens, valors secrets del config, camins de la
// carpeta personal i, opcionalment, adreces de correu.
//
// Tot és pur (sense disc ni xarxa): el servidor web, el TUI i els tests en
// fan servir les mateixes funcions.
package transcript

import (
	"os"
	"regexp"
	"sort"
	"strings"

	"gregal/internal/config"
)

// Redacted és el text que substitueix un secret.
const Redacted = "[REDACTED]"

// RedactOptions diu què s'ha d'amagar a més dels patrons fixos.
type RedactOptions struct {
	// Secrets són valors literals que no poden sortir mai (claus del config,
	// tokens de variables d'entorn). Els curts (<8) s'ignoren: amagar
	// «ollama» o «none» trencaria el text sense protegir res.
	Secrets []string
	// Emails amaga també les adreces de correu.
	Emails bool
	// HomeDirs són carpetes personals concretes (p. ex. os.UserHomeDir) a
	// més de les genèriques C:\Users\nom, /home/nom i /Users/nom.
	HomeDirs []string
}

// Redactor aplica unes RedactOptions; es construeix una vegada per document.
type Redactor struct {
	opt     RedactOptions
	secrets []string
	homes   []*regexp.Regexp
}

// minSecret és la longitud mínima d'un secret literal.
const minSecret = 8

var (
	rePEM = regexp.MustCompile(`-----BEGIN [A-Z0-9 ]*PRIVATE KEY-----[\s\S]*?(?:-----END [A-Z0-9 ]*PRIVATE KEY-----|$)`)
	// Patrons de claus conegudes. L'ordre importa poc: tots substitueixen
	// el valor sencer per Redacted.
	reTokens = []*regexp.Regexp{
		// Anthropic (sk-ant-…), OpenAI (sk-…, sk-proj-…) i compatibles.
		regexp.MustCompile(`\bsk-ant-[A-Za-z0-9_\-]{10,}`),
		regexp.MustCompile(`\bsk-[A-Za-z0-9_\-]{20,}`),
		// GitHub: tokens clàssics i fine-grained.
		regexp.MustCompile(`\bgh[pousr]_[A-Za-z0-9]{20,}`),
		regexp.MustCompile(`\bgithub_pat_[A-Za-z0-9_]{20,}`),
		// Slack.
		regexp.MustCompile(`\bxox[baprs]-[A-Za-z0-9\-]{10,}`),
		// AWS access key id.
		regexp.MustCompile(`\b(?:AKIA|ASIA)[0-9A-Z]{16}\b`),
		// Google API key.
		regexp.MustCompile(`\bAIza[0-9A-Za-z_\-]{35}`),
		// JWT (capçalera i cos comencen sempre per eyJ).
		regexp.MustCompile(`\beyJ[A-Za-z0-9_\-]{8,}\.eyJ[A-Za-z0-9_\-]{8,}\.[A-Za-z0-9_\-]{8,}`),
	}
	reBearer = regexp.MustCompile(`(?i)\b(bearer)(\s+)[A-Za-z0-9._~+/=\-]{16,}`)
	// Assignacions genèriques: api_key=…, "token": "…", PASSWORD: …
	// El valor es valida a part (assignacioSecreta) per no tocar codi
	// normal com `token := lexer.Next()` o `password: string`.
	reAssign = regexp.MustCompile(`(?i)\b([A-Za-z0-9_.\-]*(?:api[_\-]?key|apikey|secret|token|passwd|password|access[_\-]?key|private[_\-]?key|auth)[A-Za-z0-9_\-]*)(["']?[ \t]*[:=][ \t]*["']?)([^\s"'` + "`" + `,;{}<>]{6,})`)
	reEmail  = regexp.MustCompile(`[A-Za-z0-9._%+\-]+@[A-Za-z0-9\-]+(?:\.[A-Za-z0-9\-]+)*\.[A-Za-z]{2,}`)
	// Carpetes personals genèriques. A Windows, majúscules indiferents i
	// qualsevol barra (també la doble del JSON escapat).
	reHomeWin  = regexp.MustCompile(`(?i)\b[a-z]:(?:\\\\|\\|/)+users(?:\\\\|\\|/)+[^\\/\s"'<>|:*?]+`)
	reHomeUnix = regexp.MustCompile(`(^|[^\w.~\-])/(?:home|Users)/[^/\s"'<>:]+`)
	reDigit    = regexp.MustCompile(`[0-9]`)
)

// NewRedactor prepara un Redactor.
func NewRedactor(o RedactOptions) *Redactor {
	r := &Redactor{opt: o}
	seen := map[string]bool{}
	for _, s := range o.Secrets {
		s = strings.TrimSpace(s)
		if len(s) < minSecret || seen[s] {
			continue
		}
		seen[s] = true
		r.secrets = append(r.secrets, s)
	}
	// Els llargs primer: si un secret en conté un altre, que no en quedi un tros.
	sort.Slice(r.secrets, func(i, j int) bool { return len(r.secrets[i]) > len(r.secrets[j]) })
	for _, h := range o.HomeDirs {
		if re := homeRegexp(h); re != nil {
			r.homes = append(r.homes, re)
		}
	}
	return r
}

// Redact és NewRedactor(o).Redact(s).
func Redact(s string, o RedactOptions) string { return NewRedactor(o).Redact(s) }

// Redact aplica totes les regles a s.
func (r *Redactor) Redact(s string) string {
	if r == nil || s == "" {
		return s
	}
	s = rePEM.ReplaceAllString(s, "[REDACTED PRIVATE KEY]")
	for _, sec := range r.secrets {
		s = strings.ReplaceAll(s, sec, Redacted)
	}
	for _, re := range reTokens {
		s = re.ReplaceAllString(s, Redacted)
	}
	s = reBearer.ReplaceAllString(s, "${1}${2}"+Redacted)
	s = reAssign.ReplaceAllStringFunc(s, func(m string) string {
		sub := reAssign.FindStringSubmatch(m)
		if !assignacioSecreta(sub[3]) {
			return m
		}
		return sub[1] + sub[2] + Redacted
	})
	for _, re := range r.homes {
		s = re.ReplaceAllString(s, "~${1}")
	}
	s = reHomeWin.ReplaceAllString(s, "~")
	s = reHomeUnix.ReplaceAllString(s, "${1}~")
	if r.opt.Emails {
		s = reEmail.ReplaceAllString(s, "[email]")
	}
	return s
}

// assignacioSecreta decideix si el valor d'una assignació sembla un secret.
// Els secrets de debò porten xifres i no són crides ni camins de codi:
// `password = getPassword()` o `token: string` no s'han de tocar.
func assignacioSecreta(v string) bool {
	if strings.Contains(v, Redacted) || strings.ContainsAny(v, "()[]$") {
		return false
	}
	if len(v) < 8 || !reDigit.MatchString(v) {
		return false
	}
	return true
}

// homeRegexp construeix el patró d'una carpeta personal concreta: igual
// amb barres d'un tipus o de l'altre (i escapades), i sense majúscules a
// Windows. Torna nil per a camins massa curts (/, C:\).
func homeRegexp(dir string) *regexp.Regexp {
	dir = strings.TrimRight(strings.TrimSpace(dir), `\/`)
	parts := strings.FieldsFunc(dir, func(r rune) bool { return r == '/' || r == '\\' })
	if len(parts) < 2 {
		return nil
	}
	quoted := make([]string, len(parts))
	for i, p := range parts {
		quoted[i] = regexp.QuoteMeta(p)
	}
	sep := `(?:\\\\|\\|/)+`
	pat := strings.Join(quoted, sep)
	if !strings.Contains(parts[0], ":") {
		pat = sep + pat // camí absolut unix: comença per barra
	}
	// El nom ha d'acabar aquí: «aleix» no pot tapar mig «aleix.solanes».
	return regexp.MustCompile(`(?i)` + pat + `($|[^\w.\-])`)
}

// SecretsFromConfig recull els valors que no poden sortir mai d'aquest
// config: les api_key dels providers (ja expandides, siguin literals o
// ${VAR}), el token de Telegram i els valors d'entorn i capçaleres dels
// servidors MCP. Hi afegeix les variables d'entorn amb nom de secret.
func SecretsFromConfig(cfg *config.Config) []string {
	var out []string
	add := func(v string) {
		v = strings.TrimSpace(os.ExpandEnv(v))
		if v == "" || strings.HasPrefix(v, "${") {
			return
		}
		out = append(out, v)
		// "Bearer xyz": també el token sol.
		if f := strings.Fields(v); len(f) == 2 && strings.EqualFold(f[0], "bearer") {
			out = append(out, f[1])
		}
	}
	if cfg != nil {
		for _, p := range cfg.Providers {
			add(p.APIKey)
		}
		add(cfg.Telegram.Token)
		for _, m := range cfg.MCP {
			for _, v := range m.Env {
				add(v)
			}
			for _, v := range m.Headers {
				add(v)
			}
		}
	}
	return append(out, SecretsFromEnv(os.Environ())...)
}

// reEnvSecret reconeix noms de variables d'entorn que porten secrets.
var reEnvSecret = regexp.MustCompile(`(?i)(KEY|TOKEN|SECRET|PASSWORD|PASSWD|CREDENTIAL)`)

// SecretsFromEnv torna els valors de les variables (format NOM=valor) amb
// nom de secret: OPENAI_API_KEY, GITHUB_TOKEN, GREGAL_API_TOKEN…
func SecretsFromEnv(env []string) []string {
	var out []string
	for _, kv := range env {
		k, v, ok := strings.Cut(kv, "=")
		if !ok || !reEnvSecret.MatchString(k) {
			continue
		}
		if v = strings.TrimSpace(v); len(v) >= minSecret {
			out = append(out, v)
		}
	}
	return out
}

// DefaultHomeDirs és la carpeta personal d'aquest procés (si n'hi ha).
func DefaultHomeDirs() []string {
	if h, err := os.UserHomeDir(); err == nil && strings.TrimSpace(h) != "" {
		return []string{h}
	}
	return nil
}
