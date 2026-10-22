package web

import (
	"strings"
	"testing"
)

// Els 6 moods de la UI han de ser personae vàlides; els antics continuen
// funcionant; qualsevol altra cosa es rebutja.
func TestRetroMoodsNorm(t *testing.T) {
	for _, p := range []string{"neutral", "sarcastic", "playful", "sharp", "flirty", "serious", "friendly", "mentor", " Serious "} {
		if got := retroPersonaNorm(p); got == "" || got != strings.ToLower(strings.TrimSpace(p)) {
			t.Errorf("persona %q hauria de ser vàlida, tinc %q", p, got)
		}
	}
	for _, p := range []string{"", "evil", "borde", "romantic", "sarcàstic"} {
		if got := retroPersonaNorm(p); got != "" {
			t.Errorf("persona %q s'hauria de rebutjar, tinc %q", p, got)
		}
	}
}

// Cada mood té veu pròpia i tots mantenen l'ancoratge.
func TestRetroMoodDirectivesDiffer(t *testing.T) {
	seen := map[string]string{}
	for _, p := range []string{"sarcastic", "playful", "sharp", "flirty", "serious"} {
		d := retroPersonaDirective(p)
		if !strings.HasPrefix(d, "PERSONA (tone only)") || !strings.Contains(d, "STRICT GROUNDING RULES above still apply") {
			t.Fatalf("%s: directiva sense capçalera o sense ancoratge: %q", p, d)
		}
		for q, od := range seen {
			if od == d {
				t.Fatalf("%s i %s comparteixen directiva: no hi ha veu pròpia", p, q)
			}
		}
		seen[p] = d
	}
	if d := retroPersonaDirective("sharp"); strings.Contains(strings.ToLower(d), "insult") && !strings.Contains(strings.ToLower(d), "never") {
		t.Errorf("borde sense límit: %q", d)
	}
	if d := retroPersonaDirective("flirty"); !strings.Contains(strings.ToLower(d), "explicit") {
		t.Error("flirty ha de prohibir explícitament el contingut explícit")
	}
}

// POST /api/retro/rewrite: canvia el to i reescriu els missatges d'agent
// amb el model; els de l'usuari no es toquen.
func TestRetroRewriteRegeneratesTexts(t *testing.T) {
	fake := &retroFake{}
	s := retroTestServer(t, fake)

	c := retroChannelFrom(t, retroPost(t, s, "/api/retro/start", `{}`))
	before := map[string]string{}
	for _, m := range c.Messages {
		before[m.ID] = m.Text
	}
	if len(before) == 0 {
		t.Fatal("el canal de prova no té missatges")
	}

	rec := retroPost(t, s, "/api/retro/rewrite", `{"key":"`+c.Key+`","persona":"sarcastic"}`)
	out := retroChannelFrom(t, rec)
	if out.Persona != "sarcastic" {
		t.Fatalf("persona no aplicada: %q", out.Persona)
	}
	changed, kept := 0, 0
	for _, m := range out.Messages {
		old, ok := before[m.ID]
		if !ok {
			t.Fatalf("missatge nou %s: la reescriptura no pot afegir ni treure", m.ID)
		}
		if m.Author == "user" || m.Author == "system" {
			if m.Text != old {
				t.Errorf("missatge %s (%s) tocat: només es reescriuen veus d'agent", m.ID, m.Author)
			}
			kept++
			continue
		}
		if m.Text == old {
			t.Errorf("missatge %s d'agent sense reescriure", m.ID)
		} else {
			changed++
		}
		if len(m.Receipts) == 0 && len(c.Messages[0].Receipts) > 0 {
			t.Errorf("missatge %s ha perdut els receipts", m.ID)
		}
	}
	if changed == 0 {
		t.Error("cap missatge d'agent reescrit")
	}
	_ = kept
}
