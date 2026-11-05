package companion

import (
	"strings"
	"testing"
)

func TestDetectLang(t *testing.T) {
	cases := map[string]string{
		"Què podem sopar avui que sigui ràpid? Doneu-me idees.":                   "ca",
		"Quick question for everyone: what's one habit that made your week better?": "en",
		"¿Qué podemos cenar hoy que sea rápido? Dadme ideas.":                      "es",
		"Hola!":           "",
		"ok":              "",
		"@Migjorn quin llibre curt em recomanes per aquest cap de setmana?": "ca",
		"Hey! What's a good quick workout for today?":                       "en",
	}
	for msg, want := range cases {
		if got := DetectLang(msg); got != want {
			t.Errorf("%q → %q, volia %q", msg, got, want)
		}
	}
}

func TestReplyLanguageBrief(t *testing.T) {
	if b := ReplyLanguageBrief("What's one habit that made your week better?"); !strings.Contains(b, "English") || !strings.Contains(b, "quick-reply") {
		t.Fatalf("brief: %q", b)
	}
	if b := ReplyLanguageBrief("ok"); b != "" {
		t.Fatalf("sense prou text no ha de fixar res: %q", b)
	}
}
