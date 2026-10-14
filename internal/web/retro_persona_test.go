package web

import (
	"os"
	"strings"
	"testing"
)

func TestRetroPersonaDirectiveKeepsGrounding(t *testing.T) {
	if retroPersonaDirective("neutral") != "" || retroPersonaDirective("bogus") != "" {
		t.Fatal("neutral and unknown personas add nothing to the prompt")
	}
	for _, p := range []string{"friendly", "sarcastic", "mentor", " Mentor "} {
		d := retroPersonaDirective(p)
		if !strings.HasPrefix(d, "PERSONA (tone only)") || !strings.Contains(d, "STRICT GROUNDING RULES above still apply") {
			t.Fatalf("%q: %q", p, d)
		}
	}
	c := &retroChannel{Title: "t", Persona: "sarcastic"}
	sys, _ := retroStandupPrompt(c)
	if strings.Index(sys, "STRICT GROUNDING") > strings.Index(sys, "PERSONA") || !strings.Contains(sys, "sarcastic") {
		t.Fatalf("persona must follow the grounding rules: %s", sys)
	}
	sys, _ = retroReplyPrompt(c, retroModeMessage, []string{"coordinator"}, "", nil, "hi")
	if !strings.Contains(sys, "PERSONA (tone only)") || !strings.Contains(sys, "STRICT GROUNDING") {
		t.Fatalf("reply prompt lacks persona or rules: %s", sys)
	}
}

func TestRetroPersonaFromConfigRequestAndChannel(t *testing.T) {
	fake := &retroFake{}
	s := retroTestServer(t, fake)
	if err := os.WriteFile(s.cfgPath, []byte("retro:\n  persona: mentor\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := s.retroDefaultPersona(); got != "mentor" {
		t.Fatalf("config default: %q", got)
	}
	c := retroChannelFrom(t, retroPost(t, s, "/api/retro/start", `{}`))
	if c.Persona != "mentor" {
		t.Fatalf("new channel uses retro.persona: %+v", c.Persona)
	}
	fake.mu.Lock()
	first := fake.bodies[0]
	fake.mu.Unlock()
	if !strings.Contains(first, "patient senior mentor") {
		t.Fatal("stand-up prompt lacks the mentor tone")
	}
	// The header picker: an open channel changes tone without a model call.
	before := fake.calls.Load()
	c = retroChannelFrom(t, retroPost(t, s, "/api/retro/start", `{"run_id":5,"persona":"friendly"}`))
	if c.Persona != "friendly" || fake.calls.Load() != before {
		t.Fatalf("persona switch: %q calls %d→%d", c.Persona, before, fake.calls.Load())
	}
	if c := retroChannelFrom(t, retroPost(t, s, "/api/retro/start", `{"run_id":5,"persona":"evil"}`)); c.Persona != "friendly" {
		t.Fatalf("unknown persona must be ignored: %q", c.Persona)
	}
	rec := retroPost(t, s, "/api/retro/message", `{"text":"why?","persona":"sarcastic"}`)
	if c := retroChannelFrom(t, rec); c.Persona != "sarcastic" {
		t.Fatalf("message persona: %q", c.Persona)
	}
	fake.mu.Lock()
	last := fake.bodies[len(fake.bodies)-1]
	fake.mu.Unlock()
	if !strings.Contains(last, "lightly sarcastic") || !strings.Contains(last, "STRICT GROUNDING") {
		t.Fatal("reply prompt must carry the new tone and the same rules")
	}
}
