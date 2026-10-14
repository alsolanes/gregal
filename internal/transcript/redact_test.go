package transcript

import (
	"strings"
	"testing"

	"gregal/internal/config"
)

func TestRedactPatronsDeClaus(t *testing.T) {
	cases := map[string]string{
		"openai":      "key sk-proj-abcdefghijklmnopqrstuvwxyz0123 end",
		"openai-curt": "key sk-abcdefghij0123456789ABCD end",
		"anthropic":   "key sk-ant-api03-AbCdEfGhIj_KlMnOp-1234 end",
		"ghp":         "key ghp_abcdefghijklmnopqrstuvwxyz0123456789 end",
		"gho":         "key gho_abcdefghijklmnopqrstuvwxyz0123456789 end",
		"github_pat":  "key github_pat_11ABCDEFG0123456789_abcdefghijklmnop end",
		"slack":       "key xoxb-1234567890-abcdefghij end",
		"slack-p":     "key xoxp-1234567890-abcdefghij end",
		"aws":         "key AKIAIOSFODNN7EXAMPLE end",
		"google":      "key AIzaSyA-1234567890abcdefghijklmnopqrstu end",
		"jwt":         "key eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.dozjgNryP4J3jVmNHl0w5N_XgL0n3I9PlFUP0THsR8U end",
	}
	for name, in := range cases {
		got := Redact(in, RedactOptions{})
		if got != "key "+Redacted+" end" {
			t.Errorf("%s: %q", name, got)
		}
	}
}

func TestRedactBearerIAssignacions(t *testing.T) {
	got := Redact("Authorization: Bearer abcdef0123456789abcdef", RedactOptions{})
	if got != "Authorization: Bearer "+Redacted {
		t.Fatalf("bearer: %q", got)
	}
	for _, in := range []string{
		"api_key=abc123def456ghi",
		"OPENAI_API_KEY=x9y8z7w6v5u4",
		`"token": "a1b2c3d4e5f6g7"`,
		"password: hunter2hunter2",
		"client_secret = 'Zx9Qw8Er7Ty6'",
		"export GITHUB_TOKEN=abcd1234efgh5678",
	} {
		got := Redact(in, RedactOptions{})
		if !strings.Contains(got, Redacted) {
			t.Errorf("no amagat: %q → %q", in, got)
		}
		for _, secret := range []string{"abc123def456ghi", "x9y8z7w6v5u4", "a1b2c3d4e5f6g7", "hunter2hunter2", "Zx9Qw8Er7Ty6", "abcd1234efgh5678"} {
			if strings.Contains(got, secret) {
				t.Errorf("%q encara conté %q", got, secret)
			}
		}
	}
}

func TestRedactPEM(t *testing.T) {
	in := "abans\n-----BEGIN RSA PRIVATE KEY-----\nMIIEow\nAAAA\n-----END RSA PRIVATE KEY-----\ndesprés"
	got := Redact(in, RedactOptions{})
	if got != "abans\n[REDACTED PRIVATE KEY]\ndesprés" {
		t.Fatalf("%q", got)
	}
	got = Redact("x -----BEGIN OPENSSH PRIVATE KEY-----\nb3Blbn (retallat)", RedactOptions{})
	if strings.Contains(got, "b3Blbn") {
		t.Fatalf("bloc PEM sense final: %q", got)
	}
}

func TestRedactSecretsDelConfig(t *testing.T) {
	t.Setenv("GREGAL_TEST_PROVIDER_KEY", "envvalue-0123456789")
	cfg := &config.Config{
		Providers: map[string]config.Provider{
			"a": {APIKey: "literal-key-XYZ-0001"},
			"b": {APIKey: "${GREGAL_TEST_PROVIDER_KEY}"},
			"c": {APIKey: "none"},
		},
		Telegram: config.TelegramCfg{Token: "123456:telegram-token-abc"},
		MCP: map[string]config.MCPServerCfg{
			"m": {Env: map[string]string{"X": "mcp-env-secret-77"}, Headers: map[string]string{"Authorization": "Bearer mcp-header-token-99"}},
		},
	}
	secrets := SecretsFromConfig(cfg)
	text := "a literal-key-XYZ-0001 b envvalue-0123456789 c none t 123456:telegram-token-abc m mcp-env-secret-77 h mcp-header-token-99"
	got := Redact(text, RedactOptions{Secrets: secrets})
	for _, s := range []string{"literal-key-XYZ-0001", "envvalue-0123456789", "telegram-token-abc", "mcp-env-secret-77", "mcp-header-token-99"} {
		if strings.Contains(got, s) {
			t.Errorf("secret %q visible: %q", s, got)
		}
	}
	if !strings.Contains(got, "c none") {
		t.Errorf("un valor curt no s'ha de tocar: %q", got)
	}
}

func TestSecretsFromEnv(t *testing.T) {
	got := SecretsFromEnv([]string{"PATH=/usr/bin:/bin", "MY_API_KEY=abcdefgh123", "GH_TOKEN=short", "SECRET_THING=0123456789"})
	if len(got) != 2 || got[0] != "abcdefgh123" || got[1] != "0123456789" {
		t.Fatalf("%v", got)
	}
}

func TestRedactCarpetesPersonals(t *testing.T) {
	cases := map[string]string{
		`C:\Users\aleix\dev\gregal\main.go`:      `~\dev\gregal\main.go`,
		`c:/users/Aleix/dev`:                     `~/dev`,
		`"C:\\Users\\aleix\\dev"`:                `"~\\dev"`,
		`/home/aleix/src/x.go`:                   `~/src/x.go`,
		`cd /Users/aleix/proj && ls`:             `cd ~/proj && ls`,
		`file:///home/aleix/a.txt`:               `file://~/a.txt`,
		`see https://example.com/home/page here`: `see https://example.com/home/page here`,
		`/srv/home/x`:                            `/srv/home/x`,
	}
	for in, want := range cases {
		if got := Redact(in, RedactOptions{}); got != want {
			t.Errorf("%q → %q, volia %q", in, got, want)
		}
	}
	// Carpeta personal concreta fora de les genèriques.
	o := RedactOptions{HomeDirs: []string{`D:\Perfils\aleix`}}
	if got := Redact(`D:\Perfils\aleix\x i d:/perfils/ALEIX/y i D:\Perfils\aleix.solanes`, o); got != `~\x i ~/y i D:\Perfils\aleix.solanes` {
		t.Errorf("home concret: %q", got)
	}
}

func TestRedactEmailsOpcional(t *testing.T) {
	in := "contact aleix@example.com please"
	if got := Redact(in, RedactOptions{}); got != in {
		t.Fatalf("sense flag no s'han de tocar: %q", got)
	}
	if got := Redact(in, RedactOptions{Emails: true}); got != "contact [email] please" {
		t.Fatalf("amb flag: %q", got)
	}
}

// El codi normal no es pot desfigurar: res d'això és un secret.
func TestRedactSenseFalsosPositius(t *testing.T) {
	for _, in := range []string{
		"token := lexer.Next()",
		"password = getPassword()",
		"type Config struct { Token string `yaml:\"token\"` }",
		"func (t *Token) String() string { return t.raw }",
		"const tokenKind = 3",
		"let secret: string = readSecret(path);",
		"the task-runner and risk-assessment-framework-v2 modules",
		"apiKey: process.env.OPENAI_API_KEY",
		"auth_header = os.Getenv(\"AUTH\")",
		"go test ./internal/... -run TestX",
		"Bearer token",
		"sk-learn is not a key",
		"https://github.com/user/repo/blob/main/README.md",
	} {
		if got := Redact(in, RedactOptions{}); got != in {
			t.Errorf("fals positiu: %q → %q", in, got)
		}
	}
}
